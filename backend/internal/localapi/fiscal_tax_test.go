package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"errors"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func taxPolicyFixture(f *stockHTTPFixture) fiscal.TaxPolicyInput {
	return fiscal.TaxPolicyInput{OperationID: "tax-policy-one", ProfileID: "issuer-one", ProfileRevision: 1, Active: true,
		Rule: fiscalcore.TaxRule{ID: "policy-one", ProductID: f.product, CFOP: "5102", NCM: "19059090", OriginUF: "SP", DestinationUF: "SP", CRT: "1", Model: "65", ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", LegalReference: "Synthetic calculation fixture, not tax advice", ApprovalReference: "test-owner-review", BaseRounding: "tax_final", Components: []fiscalcore.TaxComponent{{Code: "ICMS", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "0"}}},
		Case: fiscalcore.TaxCase{Operation: "sale", Purpose: "final_consumption", OriginMunicipality: "3550308", DestinationMunicipality: "3550308", RecipientStatus: "noncontributor", FinalConsumer: "yes", ProductOrigin: "0"}, Sources: []string{"https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm"}}
}
func taxQuoteFixture(in fiscal.TaxPolicyInput) fiscal.TaxQuoteInput {
	return fiscal.TaxQuoteInput{OperationID: "quote-one", PolicyID: in.Rule.ID, PolicyRevision: 1, Context: fiscalcore.TaxContext{OriginUF: "SP", DestinationUF: "SP", CRT: "1", Model: "65", Date: "2026-10-29"}, Case: in.Case, Item: fiscalcore.TaxItem{ProductID: in.Rule.ProductID, Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}}
}
func taxRequest(t *testing.T, f *stockHTTPFixture, method, path string, in any, want int, out any) {
	t.Helper()
	var body []byte
	if in != nil {
		body, _ = json.Marshal(in)
	}
	code, b := request(t, f.app, method, path, string(body), f.token)
	if code != want {
		t.Fatalf("%s %s =%d want%d %s", method, path, code, want, b)
	}
	if out != nil && json.Unmarshal(b, out) != nil {
		t.Fatal("invalid response")
	}
}

func TestFiscalTaxMissingTaxTotalDecisionCannotPublish(t *testing.T) {
	f := fiscalFixture(t)
	raw, _ := json.Marshal(taxPolicyFixture(f))
	for _, body := range []string{
		strings.Replace(string(raw), `"adds_to_total":false`, `"adds_to_total":null`, 1),
		strings.Replace(string(raw), `,"adds_to_total":false`, "", 1),
		strings.Replace(string(raw), `"active":true`, `"active":null`, 1),
		strings.Replace(string(raw), `"active":true,`, "", 1),
	} {
		code, _ := request(t, f.app, "POST", "/local/v1/fiscal/tax-policies", body, f.token)
		if code != 400 {
			t.Fatal("missing explicit tax decision published", code)
		}
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_policies`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid decision persisted", count, err)
	}
}
func TestFiscalTaxPolicyQuoteScopeRevisionAndRecovery(t *testing.T) {
	f := fiscalFixture(t)
	in := taxPolicyFixture(f)
	var policy fiscal.TaxPolicy
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", in, 201, &policy)
	if policy.Revision != 1 || len(policy.SHA256) != 64 || policy.ActorID != f.owner.OwnerID {
		t.Fatal("missing approval evidence")
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", in, 200, &policy)
	if !policy.Repeated {
		t.Fatal("not idempotent")
	}
	changed := in
	changed.Active = false
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", changed, 409, nil)
	overlap := in
	overlap.OperationID = "overlap"
	overlap.Rule.ID = "other-policy"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", overlap, 409, nil)
	bad := in
	bad.OperationID = "bad-profile"
	bad.Rule.ID = "bad-policy"
	bad.Rule.CRT = "3"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", bad, 400, nil)
	quote := taxQuoteFixture(in)
	var original fiscal.TaxQuote
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 201, &original)
	if original.Result.Taxes[0].Amount != "18.00" || original.Result.Total != "100.00" || original.Result.OfficialAuthorization {
		t.Fatal("wrong calculation or invented authorization")
	}
	wrong := quote
	wrong.OperationID = "wrong-case"
	wrong.Case.FinalConsumer = "no"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", wrong, 400, nil)
	wrong = quote
	wrong.OperationID = "expired"
	wrong.Context.Date = "2027-01-01"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", wrong, 400, nil)
	wrong = quote
	wrong.Item.UnitPrice = "60"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", wrong, 409, nil)
	// A superseding inactive revision blocks new calculations but preserves
	// original receipts and exact replays, including after profile changes.
	changed.OperationID = "deactivate"
	changed.ExpectedRevision = 1
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", changed, 201, nil)
	wrong = quote
	wrong.OperationID = "new-after-inactive"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", wrong, 409, nil)
	var recovered fiscal.TaxQuote
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 200, &recovered)
	if !recovered.Repeated || recovered.ResultSHA256 != original.ResultSHA256 || recovered.PolicySHA256 != original.PolicySHA256 {
		t.Fatal("recovery changed originals")
	}
	taxRequest(t, f, "GET", "/local/v1/fiscal/tax-policies/policy-one?revision=1", nil, 200, &policy)
	if !policy.Input.Active || policy.Revision != 1 {
		t.Fatal("history replaced")
	}
	for _, query := range []string{"?revision=1&revision=2", "?revision=01", "?tenant_id=foreign", "?revision=0"} {
		taxRequest(t, f, "GET", "/local/v1/fiscal/tax-policies/policy-one"+query, nil, 400, nil)
	}
	for _, table := range []string{"fiscal_tax_policies", "fiscal_tax_quotes"} {
		for _, sql := range []string{"DELETE FROM " + table, "UPDATE " + table + " SET actor_id='other'"} {
			if _, err := f.db.Exec(sql); err == nil {
				t.Fatal("immutable evidence modified")
			}
		}
	}
	// A cashier may sell, but publication and calculation require accounting.
	policyPerson(t, f.db, f.owner, "tax-cashier", "cashier")
	f.token = loginToken(t, f.app, "tax-cashier")
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", in, 403, nil)
	taxRequest(t, f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 403, nil)
}
func TestFiscalTaxRevokedSessionCannotWrite(t *testing.T) {
	f := fiscalFixture(t)
	in := taxPolicyFixture(f)
	if err := localauth.Logout(context.Background(), f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if _, err := fiscal.PublishTaxPolicy(context.Background(), f.db, nil, identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}, f.device, in, f.token); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("revoked session reached write", err)
	}
	fresh := loginToken(t, f.app, f.owner.OwnerID)
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	for _, kind := range []string{"tenant", "store", "user", "device"} {
		a, d := actor, f.device
		switch kind {
		case "tenant":
			a.TenantID = "foreign"
		case "store":
			a.StoreID = "foreign"
		case "user":
			a.IdentityID = "foreign"
		case "device":
			d.DeviceID = "foreign"
		}
		if _, err := fiscal.PublishTaxPolicy(context.Background(), f.db, nil, a, d, in, fresh); !errors.Is(err, localauth.ErrDenied) {
			t.Fatal("context reached publication", kind, err)
		}
		if _, err := fiscal.CreateTaxQuote(context.Background(), f.db, nil, a, d, taxQuoteFixture(in), fresh); !errors.Is(err, localauth.ErrDenied) {
			t.Fatal("context reached calculation", kind, err)
		}
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_policies`).Scan(&n); err != nil || n != 0 {
		t.Fatal("rejected attempts persisted", n, err)
	}
}

func TestFiscalTaxConcurrentQuotesAndProfileChange(t *testing.T) {
	f := fiscalFixture(t)
	in := taxPolicyFixture(f)
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", in, 201, nil)
	quote := taxQuoteFixture(in)
	raw, _ := json.Marshal(quote)
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", "/local/v1/fiscal/tax-quotes", string(raw), f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	created := 0
	for code := range codes {
		if code == 201 {
			created++
		} else if code != 200 {
			t.Fatal("concurrent request failed", code)
		}
	}
	if created != 1 {
		t.Fatal("created multiple calculation records", created)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_quotes`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicated quote", count, err)
	}
	profile := strings.Replace(fiscalProfileBody, `"operation_id":"issuer-create"`, `"operation_id":"issuer-update"`, 1)
	profile = strings.Replace(profile, `"expected_revision":0`, `"expected_revision":1`, 1)
	code, body := request(t, f.app, "POST", "/local/v1/fiscal/profiles", profile, f.token)
	if code != 201 {
		t.Fatal(code, string(body))
	}
	quote.OperationID = "after-profile-change"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 409, nil)
	taxRequest(t, f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, nil)
	// Read-only accounting access must not publish an owner's approval.
	policyPerson(t, f.db, f.owner, "tax-accountant", "accountant")
	f.token = loginToken(t, f.app, "tax-accountant")
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", in, 403, nil)
	taxRequest(t, f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, nil)
}

// Real process HTTP, durable SQLite, restart, encrypted backup and clean restore.
// No external tax authority is contacted and fixture rates are not legal rules.
func TestFiscalTaxRealHTTPRestartAndRestore(t *testing.T) {
	f := fiscalFixture(t)
	station := pilotPrepareCLIStation(t, f)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	p := &pilotHTTP{t: t, service: pilotStart(t, path, f, station), client: &http.Client{Transport: transport, Timeout: 10 * time.Second}}
	defer func() { p.service.stop(t) }()
	p.token = p.login(f.owner.OwnerID)
	in := taxPolicyFixture(f)
	quote := taxQuoteFixture(in)
	var original, after fiscal.TaxQuote
	p.call("POST", "/local/v1/fiscal/tax-policies", in, 201, nil)
	p.call("POST", "/local/v1/fiscal/tax-quotes", quote, 201, &original)
	p.service.stop(t)
	p.service = pilotStart(t, path, f, station)
	p.token = p.login(f.owner.OwnerID)
	p.call("GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &after)
	if original.ResultSHA256 != after.ResultSHA256 || original.PolicySHA256 != after.PolicySHA256 {
		t.Fatal("restart lost evidence")
	}
	p.service.stop(t)
	restored := ""
	if *pilotBackupCLI != "" {
		restored = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		db, err := localdb.Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		archive := filepath.Join(dir, "tax.tytbak")
		restored = filepath.Join(dir, "restored.sqlite")
		if err = backup.Create(context.Background(), db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if err = backup.Verify(context.Background(), archive, f.device, key); err != nil {
			t.Fatal(err)
		}
		if err = backup.Restore(context.Background(), archive, restored, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	p.service = pilotStart(t, restored, f, station)
	p.call("GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 401, nil)
	p.token = p.login(f.owner.OwnerID)
	p.call("POST", "/local/v1/fiscal/tax-quotes", quote, 200, &after)
	if !after.Repeated || original.ResultSHA256 != after.ResultSHA256 || original.PolicySHA256 != after.PolicySHA256 || after.Result.OfficialAuthorization {
		t.Fatal("restore changed originals or invented authorization")
	}
	t.Logf("TAX CALCULATION ONLY: %d real HTTP calls, three service processes, exact policy/result hashes preserved through restart and encrypted restore; no SEFAZ transmission", p.steps)
}
