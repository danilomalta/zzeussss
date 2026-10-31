package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beevik/etree"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

const invalidationEndpoint = "https://homologacao.example.gov.br/NFeInutilizacao4"

func invalidationSQLFixture(t *testing.T) (*stockHTTPFixture, fiscalcore.Invalidation) {
	t.Helper()
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	numberingPrepare(t, f, intent, 1)
	return f, fiscalcore.Invalidation{UF: "SP", CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Year: 2026, First: 2, Last: 4, Environment: 2, Reason: "Synthetic range reviewed by owner only"}
}
func invalidationSQLRecorder(t *testing.T, f *stockHTTPFixture, db *sql.DB, op, token string, in fiscalcore.Invalidation) *fiscal.HomologationInvalidationRecorder {
	t.Helper()
	r, err := fiscal.NewHomologationInvalidationRecorder(context.Background(), db, journalLicense(t, f, db), numberingActor(f), f.device, "issuer-one", 1, op, invalidationEndpoint, "synthetic-range-human-review", token, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func invalidationSQLRequest(t *testing.T, r *fiscal.HomologationInvalidationRecorder, op string, in fiscalcore.Invalidation) fiscalcore.RecordedRequest {
	t.Helper()
	raw, _, err := fiscalcore.BuildInvalidation(in)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := querySyntheticCredential(t).SignXML("nfe_invalidation", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	d := etree.NewDocument()
	root := d.CreateElement("soap:Envelope")
	root.CreateAttr("xmlns:soap", "http://www.w3.org/2003/05/soap-envelope")
	body := root.CreateElement("soap:Body")
	msg := body.CreateElement("nfeDadosMsg")
	msg.CreateAttr("xmlns", fiscalcore.NFeNS+"/wsdl/NFeInutilizacao4")
	doc := etree.NewDocument()
	if err = doc.ReadFromBytes(signed); err != nil {
		t.Fatal(err)
	}
	msg.AddChild(doc.Root().Copy())
	wire, _ := d.WriteToBytes()
	sum := sha256.Sum256(wire)
	return fiscalcore.RecordedRequest{OperationID: op, Scope: r.Scope(), URL: invalidationEndpoint, Issuer: in.CNPJ, Environment: "2", Key: r.ID(), RequestSHA256: hex.EncodeToString(sum[:]), Body: wire}
}
func invalidationSQLResult(op string, in fiscalcore.Invalidation, status string) fiscalcore.ExchangeResult {
	raw := []byte(fmt.Sprintf(`<retInutNFe xmlns="%s" versao="4.00"><infInut><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cStat>%s</cStat><xMotivo>Synthetic invalidation only</xMotivo><cUF>35</cUF><ano>%02d</ano><CNPJ>%s</CNPJ><mod>%s</mod><serie>%d</serie><nNFIni>%d</nNFIni><nNFFin>%d</nNFFin><dhRecbto>%s</dhRecbto><nProt>135260000000003</nProt></infInut></retInutNFe>`, fiscalcore.NFeNS, status, in.Year%100, in.CNPJ, in.Model, in.Series, in.First, in.Last, time.Now().Format("2006-01-02T15:04:05-07:00")))
	sum := sha256.Sum256(raw)
	return fiscalcore.ExchangeResult{OperationID: op, State: "response_received", HTTPStatus: 200, Body: raw, SHA256: hex.EncodeToString(sum[:])}
}
func TestFiscalInvalidationSQLConcurrentReservationKeepsOriginalAndBusiness(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	req := invalidationSQLRequest(t, r, "invalidate-one", in)
	before := cancellationBusinessDigest(t, f.db)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Reserve(ctx, req) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, fiscalcore.ErrRecorded) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("reservations", success)
	}
	if err := r.BeforeSend(ctx, "invalidate-one"); err != nil {
		t.Fatal(err)
	}
	response := invalidationSQLResult("invalidate-one", in, "102")
	results = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Complete(ctx, response) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	proof, err := fiscalcore.RecoverHomologationInvalidation(ctx, r)
	if err != nil || proof.State != "invalidation_indicated_homologation" || len(proof.ProcessedXML) == 0 {
		t.Fatal(proof.State, err)
	}
	if after := cancellationBusinessDigest(t, f.db); after != before {
		t.Fatal("invalidation moved commercial data")
	}
	if err = r.Reserve(ctx, req); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("duplicate resent", err)
	}
	different := response
	different.HTTPStatus = 201
	if err = r.Complete(ctx, different); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("overwritten response", err)
	}
	for _, statement := range []string{`UPDATE fiscal_invalidation_requests SET first_number=3`, `DELETE FROM fiscal_invalidation_requests`, `UPDATE fiscal_invalidation_results SET result_json='{}'`, `DELETE FROM fiscal_invalidation_results`} {
		if _, err = f.db.Exec(statement); err == nil {
			t.Fatal("mutable invalidation evidence")
		}
	}
}
func TestFiscalInvalidationSQLBlocksAllocationAndLegacyTransmission(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
		t.Fatal(err)
	}
	adjacent := in
	adjacent.First = 5
	adjacent.Last = 7
	other := invalidationSQLRecorder(t, f, f.db, "invalidate-adjacent", f.token, adjacent)
	if err := other.Reserve(ctx, invalidationSQLRequest(t, other, "invalidate-adjacent", adjacent)); err != nil {
		t.Fatal(err)
	}
	intent := numberingNextIntent(t, f, 2)
	assigned, err := fiscal.AllocateHomologationNumber(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
	if err != nil || assigned.Number != 8 {
		t.Fatal("reserved ranges reused", assigned.Number, err)
	}
	// Even a legacy unnumbered journal cannot use3. The trigger is defense in depth.
	legacyIntent := numberingNextIntent(t, f, 3)
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: in.CNPJ, Model: in.Model, Series: in.Series, Number: 3, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := fiscal.NewHomologationRecorder(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, legacyIntent.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", key, f.token)
	if err != nil {
		t.Fatal(err)
	}
	legacyRequest := fiscalRecorded(t, legacy)
	legacyRequest.Key = key
	if err = legacy.Reserve(ctx, legacyRequest); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("legacy path used excluded range", err)
	}
	if _, err = f.db.Exec(`INSERT INTO fiscal_number_allocations SELECT tenant_id,store_id,?,issuer,model,series,3,?,issued_at,device_id,actor_id,?,created_at FROM fiscal_number_allocations LIMIT 1`, legacyIntent.ID, key, "raw-blocked-number"); err == nil {
		t.Fatal("SQL guard bypassed")
	}
	//563 never releases the exclusion, even if its human-readable reason mentions a protocol.
	if err = r.Complete(ctx, invalidationSQLResult("invalidate-one", in, "563")); err != nil {
		t.Fatal(err)
	}
	out, err := fiscalcore.RecoverHomologationInvalidation(ctx, r)
	if err != nil || len(out.ProcessedXML) > 0 || out.State != "response_requires_review" {
		t.Fatal("563 promoted/released", out.State, err)
	}
}
func TestFiscalInvalidationSQLCannotTakeAssignedNumberOrOverlap(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
		t.Fatal(err)
	}
	for _, bounds := range [][2]int64{{1, 1}, {3, 6}, {2, 4}} {
		candidate := in
		candidate.First = bounds[0]
		candidate.Last = bounds[1]
		next := invalidationSQLRecorder(t, f, f.db, fmt.Sprintf("other-%d-%d", bounds[0], bounds[1]), f.token, candidate)
		if err := next.Reserve(ctx, invalidationSQLRequest(t, next, fmt.Sprintf("other-%d-%d", bounds[0], bounds[1]), candidate)); !errors.Is(err, fiscal.ErrConflict) {
			t.Fatal("occupied range accepted", bounds, err)
		}
	}
	changed := in
	changed.Reason = "Different explicit reason for the same operation"
	sameID := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, changed)
	if err := sameID.Reserve(ctx, invalidationSQLRequest(t, sameID, "invalidate-one", changed)); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("operation replaced", err)
	}
}
func TestFiscalInvalidationSQLAllocationRaceNeverOverlaps(t *testing.T) {
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f, in := invalidationSQLFixture(t)
			ctx := context.Background()
			intent := numberingNextIntent(t, f, 2)
			r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
			req := invalidationSQLRequest(t, r, "invalidate-one", in)
			var assigned fiscal.NumberAssignment
			var reserveErr, allocateErr error
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); reserveErr = r.Reserve(ctx, req) }()
			go func() {
				defer wg.Done()
				assigned, allocateErr = fiscal.AllocateHomologationNumber(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
			}()
			wg.Wait()
			if allocateErr != nil {
				t.Fatal(allocateErr)
			}
			if reserveErr == nil {
				if assigned.Number != 5 {
					t.Fatal("overlapping allocation", assigned.Number)
				}
			} else if !errors.Is(reserveErr, fiscal.ErrConflict) || assigned.Number != 2 {
				t.Fatal("unexpected outcome", assigned.Number, reserveErr)
			}
		})
	}
}
func TestFiscalInvalidationSQLFutureGapDoesNotSkipAvailableNumbers(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	in.First = 10
	in.Last = 12
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-future", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-future", in)); err != nil {
		t.Fatal(err)
	}
	intent := numberingNextIntent(t, f, 2)
	number, err := fiscal.AllocateHomologationNumber(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
	if err != nil || number.Number != 2 {
		t.Fatal("unrelated gap advanced sequence", number.Number, err)
	}
}
func TestFiscalInvalidationSQLLogoutAndNewSessionNeverResume(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
		t.Fatal(err)
	}
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, "invalidate-one"); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, invalidationSQLResult("invalidate-one", in, "102")); err != nil {
		t.Fatal("late evidence lost", err)
	}
	token := loginToken(t, f.app, f.owner.OwnerID)
	reader := invalidationSQLRecorder(t, f, f.db, "invalidate-one", token, in)
	if _, err := fiscalcore.RecoverHomologationInvalidation(ctx, reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.BeforeSend(ctx, "invalidate-one"); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("new session resumed", err)
	}
}
func TestFiscalInvalidationSQLProfileContractAndWrongContextBlockSend(t *testing.T) {
	for _, scenario := range []string{"profile", "contract", "context"} {
		t.Run(scenario, func(t *testing.T) {
			f, in := invalidationSQLFixture(t)
			ctx := context.Background()
			r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
			if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "profile":
				body := strings.Replace(fiscalProfileBody, "issuer-create", "issuer-update", 1)
				body = strings.Replace(body, `"expected_revision":0`, `"expected_revision":1`, 1)
				body = strings.Replace(body, "Synthetic test issuer", "Revised synthetic issuer", 1)
				code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", body, f.token)
				if code != 201 {
					t.Fatal(code)
				}
			case "contract":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
			case "context":
				for _, dimension := range []string{"tenant", "store", "actor", "device"} {
					a, d := numberingActor(f), f.device
					switch dimension {
					case "tenant":
						a.TenantID = "foreign"
						d.TenantID = "foreign"
					case "store":
						a.StoreID = "foreign"
						d.StoreID = "foreign"
					case "actor":
						a.IdentityID = "foreign"
					case "device":
						d.DeviceID = "foreign"
					}
					if _, err := fiscal.NewHomologationInvalidationRecorder(ctx, f.db, journalLicense(t, f, f.db), a, d, "issuer-one", 1, "invalidate-one", invalidationEndpoint, "synthetic-range-human-review", f.token, in); err == nil {
						t.Fatal("foreign context accepted", dimension)
					}
				}
				return
			}
			if err := r.BeforeSend(ctx, "invalidate-one"); err == nil {
				t.Fatal("old authority sent", scenario)
			}
			if err := r.Complete(ctx, invalidationSQLResult("invalidate-one", in, "102")); err != nil {
				t.Fatal(err)
			}
			if _, err := fiscalcore.RecoverHomologationInvalidation(ctx, r); err != nil {
				t.Fatal("archive lost", err)
			}
		})
	}
}
func TestFiscalInvalidationSQLRestartAndRestorePreserveRanges(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, invalidationSQLResult("invalidate-one", in, "102")); err != nil {
		t.Fatal(err)
	}
	before, err := fiscalcore.RecoverHomologationInvalidation(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	var seq int
	var name, path string
	if err = f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err = f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reader := invalidationSQLRecorder(t, f, db, "invalidate-one", f.token, in)
	after, err := fiscalcore.RecoverHomologationInvalidation(ctx, reader)
	if err != nil || !bytes.Equal(before.ProcessedXML, after.ProcessedXML) {
		t.Fatal("restart changed process", err)
	}
	f.db = db
	var dest string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, f)
		dest = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		dir := t.TempDir()
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(dir, "range.tytbak")
		dest = filepath.Join(dir, "restored.sqlite")
		if err = backup.Create(ctx, db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err = backup.Restore(ctx, archive, dest, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := localdb.Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(restored, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	recovered := invalidationSQLRecorder(t, f, restored, "invalidate-one", token, in)
	proof, err := fiscalcore.RecoverHomologationInvalidation(ctx, recovered)
	if err != nil || !bytes.Equal(proof.ProcessedXML, before.ProcessedXML) || proof.Exchange.OperationID != before.Exchange.OperationID {
		t.Fatal("restore changed range evidence", err)
	}
	if err = recovered.BeforeSend(ctx, "invalidate-one"); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("restore resumed invalidation", err)
	}
	fresh := in
	fresh.First = 5
	fresh.Last = 6
	newRecorder := invalidationSQLRecorder(t, f, restored, "invalidate-new", token, fresh)
	if err = newRecorder.Reserve(ctx, invalidationSQLRequest(t, newRecorder, "invalidate-new", fresh)); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("new ID released restore barrier", err)
	}
}

func TestFiscalInvalidationSQLReadPermissionNeverGrantsOwnerAuthority(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-one", in)); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, invalidationSQLResult("invalidate-one", in, "102")); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"cashier", "accountant", "manager"} {
		t.Run(role, func(t *testing.T) {
			id := "invalidation-" + role
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"create-`+id+`","identity_id":"`+id+`","name":"Synthetic range staff","role":"`+role+`","password":"synthetic-range-password-only"}`, f.token)
			if code != 201 {
				t.Fatal(code)
			}
			code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+id+`","password":"synthetic-range-password-only"}`, "")
			if code != 200 {
				t.Fatal(code)
			}
			var auth struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(body, &auth); err != nil {
				t.Fatal(err)
			}
			a := numberingActor(f)
			a.IdentityID = id
			reader, err := fiscal.NewHomologationInvalidationRecorder(ctx, f.db, journalLicense(t, f, f.db), a, f.device, "issuer-one", 1, "invalidate-one", invalidationEndpoint, "synthetic-range-human-review", auth.Token, in)
			if role == "cashier" {
				if err == nil {
					t.Fatal("cashier read accounting evidence")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = fiscalcore.RecoverHomologationInvalidation(ctx, reader); err != nil {
				t.Fatal(err)
			}
			if err = reader.ValidateInvalidation(ctx, in); err == nil {
				t.Fatal("non-owner gained series authority")
			}
			if err = reader.BeforeSend(ctx, "invalidate-one"); err == nil {
				t.Fatal("another actor resumed owner request")
			}
		})
	}
}
func TestFiscalInvalidationSQLMalformedRequestAndCallbackPreserveUncertainty(t *testing.T) {
	f, in := invalidationSQLFixture(t)
	ctx := context.Background()
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-one", f.token, in)
	req := invalidationSQLRequest(t, r, "invalidate-one", in)
	for name, mutate := range map[string]func(*fiscalcore.RecordedRequest){"scope": func(q *fiscalcore.RecordedRequest) { q.Scope = "foreign" }, "endpoint": func(q *fiscalcore.RecordedRequest) { q.URL = "https://foreign.gov.br/inut" }, "operation": func(q *fiscalcore.RecordedRequest) { q.OperationID = "foreign" }, "body": func(q *fiscalcore.RecordedRequest) {
		q.Body = bytes.Replace(q.Body, []byte("owner only"), []byte("tampered!!"), 1)
		sum := sha256.Sum256(q.Body)
		q.RequestSHA256 = hex.EncodeToString(sum[:])
	}} {
		t.Run(name, func(t *testing.T) {
			q := req
			q.Body = append([]byte(nil), q.Body...)
			mutate(&q)
			if err := r.Reserve(ctx, q); err == nil {
				t.Fatal("unbound request accepted")
			}
		})
	}
	if err := r.Reserve(ctx, req); err != nil {
		t.Fatal(err)
	}
	base := invalidationSQLResult("invalidate-one", in, "102")
	for _, mutate := range []func(*fiscalcore.ExchangeResult){func(q *fiscalcore.ExchangeResult) { q.OperationID = "foreign" }, func(q *fiscalcore.ExchangeResult) { q.State = "invalidated" }, func(q *fiscalcore.ExchangeResult) { q.HTTPStatus = 0 }, func(q *fiscalcore.ExchangeResult) { q.HTTPStatus = 600 }, func(q *fiscalcore.ExchangeResult) { q.SHA256 = "wrong" }, func(q *fiscalcore.ExchangeResult) { q.State = "uncertain" }} {
		q := base
		mutate(&q)
		if err := r.Complete(ctx, q); err == nil {
			t.Fatal("bad callback accepted")
		}
	}
	original, result, err := r.Read(ctx)
	if err != nil || original.OperationID != "invalidate-one" || result.State != "uncertain" {
		t.Fatal("uncertain original lost", err)
	}
	if err = r.Complete(ctx, base); err != nil {
		t.Fatal(err)
	}
}
func TestFiscalInvalidationSQLExhaustedTailNeverWraps(t *testing.T) {
	f := fiscalFixture(t)
	ctx := context.Background()
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, numberingOrigin(999999998), f.token); err != nil {
		t.Fatal(err)
	}
	in := fiscalcore.Invalidation{UF: "SP", CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Year: 2026, First: 999999998, Last: 999999999, Environment: 2, Reason: "Synthetic explicit exhausted tail only"}
	r := invalidationSQLRecorder(t, f, f.db, "invalidate-tail", f.token, in)
	if err := r.Reserve(ctx, invalidationSQLRequest(t, r, "invalidate-tail", in)); err != nil {
		t.Fatal(err)
	}
	if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token); !errors.Is(err, fiscal.ErrNumberExhausted) {
		t.Fatal("wrapped or reused excluded tail", err)
	}
}
