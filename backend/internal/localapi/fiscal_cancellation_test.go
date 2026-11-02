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

const cancellationSQLEndpoint = "https://homologacao.example.gov.br/NFeRecepcaoEvento4"
const cancellationApproval = "synthetic-owner-decision-not-legal-deadline-approval"

func cancellationSQLFixture(t *testing.T) (*stockHTTPFixture, fiscal.Intent, fiscalcore.Cancellation) {
	t.Helper()
	f, intent, number, original := queryOriginalFixture(t)
	query := querySQLRecorder(t, f, f.db, intent, "query-authorized", f.token)
	if err := query.Reserve(context.Background(), querySQLRequest(t, query, "query-authorized")); err != nil {
		t.Fatal(err)
	}
	response := querySQLResult(t, original, "query-authorized", "100")
	when := time.Now().Add(time.Second).Truncate(time.Second)
	response.Body = bytes.ReplaceAll(response.Body, []byte("2026-10-27T12:00:00-03:00"), []byte(when.Format("2006-01-02T15:04:05-07:00")))
	sum := sha256.Sum256(response.Body)
	response.SHA256 = hex.EncodeToString(sum[:])
	if err := query.Complete(context.Background(), response); err != nil {
		t.Fatal(err)
	}
	return f, intent, fiscalcore.Cancellation{Key: number.AccessKey, CNPJ: number.AccessKey[6:20], Environment: 2, Protocol: "135260000000001", Time: when.Add(time.Second), Reason: "Synthetic cancellation decision for tests"}
}
func cancellationSQLRecorder(t *testing.T, f *stockHTTPFixture, db *sql.DB, intent fiscal.Intent, op, token string) *fiscal.HomologationCancellationRecorder {
	t.Helper()
	r, err := fiscal.NewHomologationCancellationRecorder(context.Background(), db, journalLicense(t, f, db), numberingActor(f), f.device, intent.ID, "query-authorized", op, cancellationSQLEndpoint, cancellationApproval, token)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func cancellationSQLRequest(t *testing.T, r *fiscal.HomologationCancellationRecorder, f *stockHTTPFixture, op string, in fiscalcore.Cancellation) fiscalcore.RecordedRequest {
	t.Helper()
	raw, err := fiscalcore.BuildCancellation(in)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := querySyntheticCredential(t).SignXML("nfe_event", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	event := etree.NewDocument()
	if err = event.ReadFromBytes(signed); err != nil {
		t.Fatal(err)
	}
	d := etree.NewDocument()
	envelope := d.CreateElement("soap:Envelope")
	envelope.CreateAttr("xmlns:soap", "http://www.w3.org/2003/05/soap-envelope")
	body := envelope.CreateElement("soap:Body")
	msg := body.CreateElement("nfeDadosMsg")
	msg.CreateAttr("xmlns", fiscalcore.NFeNS+"/wsdl/NFeRecepcaoEvento4")
	batch := msg.CreateElement("envEvento")
	batch.CreateAttr("xmlns", fiscalcore.NFeNS)
	batch.CreateAttr("versao", "1.00")
	batch.CreateElement("idLote").SetText("1")
	batch.AddChild(event.Root().Copy())
	wire, _ := d.WriteToBytes()
	sum := sha256.Sum256(wire)
	return fiscalcore.RecordedRequest{OperationID: op, Scope: r.Scope(), URL: cancellationSQLEndpoint, Issuer: in.CNPJ, Environment: "2", Key: r.Key(), RequestSHA256: hex.EncodeToString(sum[:]), Body: wire}
}
func cancellationSQLResult(op string, in fiscalcore.Cancellation) fiscalcore.ExchangeResult {
	raw := []byte(fmt.Sprintf(`<retEnvEvento xmlns="%s" versao="1.00"><idLote>1</idLote><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>128</cStat><xMotivo>Synthetic batch</xMotivo><retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>135</cStat><xMotivo>Synthetic cancellation result</xMotivo><chNFe>%s</chNFe><tpEvento>110111</tpEvento><xEvento>Cancelamento registrado</xEvento><nSeqEvento>1</nSeqEvento><dhRegEvento>%s</dhRegEvento><nProt>135260000000002</nProt></infEvento></retEvento></retEnvEvento>`, fiscalcore.NFeNS, in.Key[:2], in.Key[:2], in.Key, in.Time.Add(time.Second).Format("2006-01-02T15:04:05-07:00")))
	sum := sha256.Sum256(raw)
	return fiscalcore.ExchangeResult{OperationID: op, HTTPStatus: 200, State: "response_received", SHA256: hex.EncodeToString(sum[:]), Body: raw}
}
func cancellationBusinessDigest(t *testing.T, db *sql.DB) string {
	t.Helper()
	h := sha256.New()
	encode := json.NewEncoder(h)
	for _, table := range []string{"sales", "sale_items", "sale_payments", "cash_sessions", "cash_movements", "stock_movements"} {
		rows, err := db.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			if err = rows.Scan(targets...); err != nil {
				t.Fatal(err)
			}
			if err = encode.Encode(values); err != nil {
				t.Fatal(err)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
func TestFiscalCancellationSQLConcurrentReservationDoesNotRefundOrRestock(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	req := cancellationSQLRequest(t, r, f, "cancel-original", in)
	before := cancellationBusinessDigest(t, f.db)
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out <- r.Reserve(ctx, req) }()
	}
	wg.Wait()
	close(out)
	success := 0
	for err := range out {
		if err == nil {
			success++
		} else if !errors.Is(err, fiscalcore.ErrRecorded) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("reservation count", success)
	}
	if err := r.BeforeSend(ctx, req.OperationID); err != nil {
		t.Fatal(err)
	}
	response := cancellationSQLResult(req.OperationID, in)
	if err := r.Complete(ctx, response); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, response); err != nil {
		t.Fatal("idempotent callback", err)
	}
	divergent := response
	divergent.HTTPStatus = 201
	if err := r.Complete(ctx, divergent); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("overwritten result", err)
	}
	proof, err := fiscalcore.RecoverHomologationCancellation(ctx, r)
	if err != nil || proof.State != "cancellation_indicated_homologation" || len(proof.ProcessedXML) == 0 {
		t.Fatal(proof.State, err)
	}
	if after := cancellationBusinessDigest(t, f.db); after != before {
		t.Fatal("fiscal event changed sale, payment, cash or stock")
	}
	other := cancellationSQLRecorder(t, f, f.db, intent, "cancel-new-id", f.token)
	if err = other.Reserve(ctx, cancellationSQLRequest(t, other, f, "cancel-new-id", in)); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("new ID repeated event", err)
	}
	old, err := fiscalcore.RecoverHomologationCancellation(ctx, other)
	if err != nil || old.Exchange.OperationID != req.OperationID || !bytes.Equal(old.ProcessedXML, proof.ProcessedXML) {
		t.Fatal("new ID replaced original recovery", err)
	}
	for _, statement := range []string{`UPDATE fiscal_cancellation_requests SET operation_id='changed'`, `DELETE FROM fiscal_cancellation_requests`, `UPDATE fiscal_cancellation_results SET result_json='{}'`, `DELETE FROM fiscal_cancellation_results`} {
		if _, err = f.db.Exec(statement); err == nil {
			t.Fatal("mutable journal", statement)
		}
	}
}
func TestFiscalCancellationSQLLogoutAndNewSessionCannotResend(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, "cancel-original"); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, cancellationSQLResult("cancel-original", in)); err != nil {
		t.Fatal("lost late response", err)
	}
	token := loginToken(t, f.app, f.owner.OwnerID)
	reader := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", token)
	if _, err := fiscalcore.RecoverHomologationCancellation(ctx, reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.BeforeSend(ctx, "cancel-original"); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("new session resumed old event", err)
	}
	if err := reader.Reserve(ctx, cancellationSQLRequest(t, reader, f, "cancel-original", in)); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal(err)
	}
}
func TestFiscalCancellationSQLRejectsWrongProtocolAndContextBeforeReservation(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	for name, mutate := range map[string]func(*fiscalcore.Cancellation){"protocol": func(i *fiscalcore.Cancellation) { i.Protocol = "135260000000999" }, "issuer": func(i *fiscalcore.Cancellation) { i.CNPJ = "11222333000181" }, "production": func(i *fiscalcore.Cancellation) { i.Environment = 1 }, "old_time": func(i *fiscalcore.Cancellation) { i.Time = i.Time.Add(-time.Hour) }, "future_time": func(i *fiscalcore.Cancellation) { i.Time = time.Now().Add(time.Hour) }, "reason": func(i *fiscalcore.Cancellation) { i.Reason = "short" }} {
		t.Run(name, func(t *testing.T) {
			candidate := in
			mutate(&candidate)
			if err := r.ValidateCancellation(ctx, candidate); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	req := cancellationSQLRequest(t, r, f, "cancel-original", in)
	for name, mutate := range map[string]func(*fiscalcore.RecordedRequest){"scope": func(q *fiscalcore.RecordedRequest) { q.Scope = "foreign" }, "operation": func(q *fiscalcore.RecordedRequest) { q.OperationID = "foreign" }, "endpoint": func(q *fiscalcore.RecordedRequest) { q.URL = "https://foreign.gov.br/event" }, "body": func(q *fiscalcore.RecordedRequest) {
		q.Body = bytes.Replace(q.Body, []byte("decision"), []byte("tampered"), 1)
		sum := sha256.Sum256(q.Body)
		q.RequestSHA256 = hex.EncodeToString(sum[:])
	}} {
		t.Run(name, func(t *testing.T) {
			q := req
			q.Body = append([]byte(nil), req.Body...)
			mutate(&q)
			if err := r.Reserve(ctx, q); err == nil {
				t.Fatal("unbound request accepted")
			}
		})
	}
	for _, dimension := range []string{"tenant", "store", "actor", "device"} {
		t.Run(dimension, func(t *testing.T) {
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
			if _, err := fiscal.NewHomologationCancellationRecorder(ctx, f.db, journalLicense(t, f, f.db), a, d, intent.ID, "query-authorized", "cancel-original", cancellationSQLEndpoint, cancellationApproval, f.token); err == nil {
				t.Fatal("foreign context accepted")
			}
		})
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_cancellation_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid request reserved", count, err)
	}
}
func TestFiscalCancellationSQLNewerLookupInvalidatesUnsentAuthority(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	newest := querySQLRecorder(t, f, f.db, intent, "query-new-pending", f.token)
	if err := newest.Reserve(ctx, querySQLRequest(t, newest, "query-new-pending")); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, "cancel-original"); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("stale authorization used", err)
	}
	// Reserving the new lookup preserves uncertainty of the event already stored.
	req, result, err := r.Read(ctx)
	if err != nil || req.OperationID != "cancel-original" || result.State != "uncertain" {
		t.Fatal("old uncertain event lost", err)
	}
}
func TestFiscalCancellationSQLAccountingReadDoesNotGrantCancellation(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, cancellationSQLResult("cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"cashier", "accountant", "manager"} {
		t.Run(role, func(t *testing.T) {
			id := "cancel-" + role
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"create-`+id+`","identity_id":"`+id+`","name":"Synthetic cancellation staff","role":"`+role+`","password":"synthetic-cancel-password-only"}`, f.token)
			if code != 201 {
				t.Fatal(code)
			}
			code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+id+`","password":"synthetic-cancel-password-only"}`, "")
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
			reader, err := fiscal.NewHomologationCancellationRecorder(ctx, f.db, journalLicense(t, f, f.db), a, f.device, intent.ID, "query-authorized", "cancel-original", cancellationSQLEndpoint, cancellationApproval, auth.Token)
			if role == "cashier" {
				if err == nil {
					t.Fatal("cashier read cancellation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = fiscalcore.RecoverHomologationCancellation(ctx, reader); err != nil {
				t.Fatal(err)
			}
			err = reader.ValidateCancellation(ctx, in)
			if role == "accountant" && err == nil {
				t.Fatal("accountant gained cancellation authority")
			}
			if role == "manager" && err != nil {
				t.Fatal("authorized manager denied", err)
			}
			if err = reader.BeforeSend(ctx, "cancel-original"); err == nil {
				t.Fatal("different actor resumed owner event")
			}
		})
	}
}
func TestFiscalCancellationSQLContractRevocationBlocksSendButKeepsCallback(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	// Reuse the signed-revision fixture helper pattern, never a database mutation.
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	if err := r.BeforeSend(ctx, "cancel-original"); err == nil {
		t.Fatal("revoked contract sent")
	}
	if err := r.Complete(ctx, cancellationSQLResult("cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	if _, err := fiscalcore.RecoverHomologationCancellation(ctx, r); err != nil {
		t.Fatal("revocation hid archive", err)
	}
}
func TestFiscalCancellationSQLRestartKeepsOriginalEvidence(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, cancellationSQLResult("cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	before, err := fiscalcore.RecoverHomologationCancellation(ctx, r)
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
	reader := cancellationSQLRecorder(t, f, db, intent, "cancel-original", f.token)
	after, err := fiscalcore.RecoverHomologationCancellation(ctx, reader)
	if err != nil || !bytes.Equal(before.ProcessedXML, after.ProcessedXML) || before.EventID != after.EventID || before.Exchange.OperationID != after.Exchange.OperationID {
		t.Fatal("restart changed evidence", err)
	}
}
func TestFiscalCancellationSQLRestorePreservesProcessAndBlocksNewEvent(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, cancellationSQLResult("cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	before, err := fiscalcore.RecoverHomologationCancellation(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	var dest string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, f)
		var seq int
		var name, path string
		if err = f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
			t.Fatal(err)
		}
		dest = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		dir := t.TempDir()
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(dir, "cancel.tytbak")
		dest = filepath.Join(dir, "restored.sqlite")
		if err = backup.Create(ctx, f.db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err = backup.Restore(ctx, archive, dest, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	db, err := localdb.Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	reader := cancellationSQLRecorder(t, f, db, intent, "cancel-original", token)
	after, err := fiscalcore.RecoverHomologationCancellation(ctx, reader)
	if err != nil || !bytes.Equal(before.ProcessedXML, after.ProcessedXML) || before.Exchange.OperationID != after.Exchange.OperationID {
		t.Fatal("restored process changed", err)
	}
	if err = reader.BeforeSend(ctx, "cancel-original"); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("restore resumed event", err)
	}
	fresh := cancellationSQLRecorder(t, f, db, intent, "cancel-different", token)
	if err = fresh.Reserve(ctx, cancellationSQLRequest(t, fresh, f, "cancel-different", in)); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("restore allowed new event", err)
	}
}

func TestFiscalCancellationSQLRequiresRecordedAuthorization(t *testing.T) {
	for _, scenario := range []string{"missing", "uncertain", "not_found", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			f, intent, _, original := queryOriginalFixture(t)
			ctx := context.Background()
			if scenario != "missing" {
				query := querySQLRecorder(t, f, f.db, intent, "query-source", f.token)
				if err := query.Reserve(ctx, querySQLRequest(t, query, "query-source")); err != nil {
					t.Fatal(err)
				}
				if scenario != "uncertain" {
					status := "217"
					if scenario == "canceled" {
						status = "101"
					}
					if err := query.Complete(ctx, querySQLResult(t, original, "query-source", status)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := fiscal.NewHomologationCancellationRecorder(ctx, f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, "query-source", "cancel-original", cancellationSQLEndpoint, cancellationApproval, f.token); err == nil {
				t.Fatal("missing authorization accepted")
			}
			var count int
			if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_cancellation_requests`).Scan(&count); err != nil || count != 0 {
				t.Fatal("invalid source reserved cancellation", err)
			}
		})
	}
}
func TestFiscalCancellationSQLMalformedCallbackKeepsOriginalRequest(t *testing.T) {
	f, intent, in := cancellationSQLFixture(t)
	ctx := context.Background()
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-original", f.token)
	if err := r.Reserve(ctx, cancellationSQLRequest(t, r, f, "cancel-original", in)); err != nil {
		t.Fatal(err)
	}
	base := cancellationSQLResult("cancel-original", in)
	for _, mutate := range []func(*fiscalcore.ExchangeResult){func(q *fiscalcore.ExchangeResult) { q.OperationID = "foreign" }, func(q *fiscalcore.ExchangeResult) { q.State = "canceled" }, func(q *fiscalcore.ExchangeResult) { q.HTTPStatus = 0 }, func(q *fiscalcore.ExchangeResult) { q.HTTPStatus = 600 }, func(q *fiscalcore.ExchangeResult) { q.SHA256 = "wrong" }, func(q *fiscalcore.ExchangeResult) { q.State = "uncertain" }} {
		candidate := base
		mutate(&candidate)
		if err := r.Complete(ctx, candidate); err == nil {
			t.Fatal("malformed callback accepted")
		}
	}
	req, result, err := r.Read(ctx)
	if err != nil || req.OperationID != "cancel-original" || result.State != "uncertain" {
		t.Fatal("request discarded", err)
	}
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out <- r.Complete(ctx, base) }()
	}
	wg.Wait()
	close(out)
	for err := range out {
		if err != nil {
			t.Fatal("concurrent identical callback", err)
		}
	}
	if _, err := fiscalcore.RecoverHomologationCancellation(ctx, r); err != nil {
		t.Fatal(err)
	}
}
