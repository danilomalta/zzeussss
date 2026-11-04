package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
)

const productionCancelReason = "Synthetic explicit cancellation requested before circulation"

func (p *productionFixture) cancel(t *testing.T, op, query string, want int) fiscalcore.CancellationEvidence {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"operation_id": op, "query_operation_id": query, "reason": productionCancelReason, "approval_reference": "SYNTHETIC-MANAGER-DECISION"})
	code, body := request(t, p.f.app, "POST", p.path("/cancel"), string(raw), p.f.token)
	if code != want {
		t.Fatalf("cancel %d want%d %s", code, want, body)
	}
	var out fiscalcore.CancellationEvidence
	if code == 200 || code == 202 {
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func TestHTTPFiscalProductionCancellationRegisteredAndReplay(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) {
			p := newProductionFixture(t, model)
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.emit(t, prep, "issue-one", 200)
			p.cancel(t, "cancel-one", "missing-query", 409)
			p.query(t, prep, "confirm-before-cancel", 200)
			before := productionBusinessHash(t, p)
			result := p.cancel(t, "cancel-one", "confirm-before-cancel", 200)
			if result.State != "cancellation_indicated_production" || len(result.ProcessedXML) == 0 || p.cancels.Load() != 1 {
				t.Fatalf("not registered: %s", result.State)
			}
			if err := fiscalcore.ValidateXML("nfe_event_process", result.ProcessedXML); err != nil {
				t.Fatal(err)
			}
			// Explicit cancellation is fiscal only; no cash refund/stock mutation.
			if before != productionBusinessHash(t, p) {
				t.Fatal("commercial effects changed")
			}
			p.cancel(t, "cancel-one", "confirm-before-cancel", 200)
			p.cancel(t, "different-operation", "confirm-before-cancel", 409)
			if p.cancels.Load() != 1 {
				t.Fatal("reposted cancellation")
			}
			if code, _ := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 409 {
				t.Fatal("active XML after cancellation", code)
			}
			if code, body := request(t, p.f.app, "GET", p.path("/cancellation"), "", p.f.token); code != 200 || !bytes.Contains(body, []byte("cancellation_indicated_production")) {
				t.Fatal("missing recovery", code)
			}
			p.prepare(t, "new-revision", 1, 409)
			raw, _ := json.Marshal(map[string]string{"operation_id": "cancel-one", "query_operation_id": "confirm-before-cancel", "reason": "Different intent on same operation ID", "approval_reference": "SYNTHETIC-MANAGER-DECISION"})
			if code, _ := request(t, p.f.app, "POST", p.path("/cancel"), string(raw), p.f.token); code != 409 {
				t.Fatal("changed replay accepted", code)
			}
		})
	}
}
func TestHTTPFiscalProductionCancellationNegativeAndUncertainNeverRetry(t *testing.T) {
	for _, mode := range []string{"cancel-drop", "cancel-status-136", "cancel-status-573", "cancel-status-155"} {
		t.Run(mode, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.emit(t, prep, "issue-one", 200)
			p.query(t, prep, "query-one", 200)
			p.mode.Store(mode)
			want := 200
			if mode == "cancel-drop" {
				want = 202
			}
			out := p.cancel(t, "cancel-one", "query-one", want)
			if mode == "cancel-status-155" {
				if out.State != "cancellation_indicated_production" {
					t.Fatal("155 not registered")
				}
			} else if out.State == "cancellation_indicated_production" {
				t.Fatal("false registration", mode)
			}
			p.cancel(t, "cancel-one", "query-one", 200)
			p.cancel(t, "cancel-two", "query-one", 409)
			if code, _ := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 409 {
				t.Fatal("uncertain/rejected cancellation exposed active XML")
			}
			if p.cancels.Load() != 1 {
				t.Fatal("duplicate POST")
			}
		})
	}
}
func TestHTTPFiscalProductionCancellationBackupRestoreHistoricalEvidence(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.emit(t, prep, "issue-one", 200)
	p.query(t, prep, "query-one", 200)
	original := p.cancel(t, "cancel-one", "query-one", 200)
	var seq int
	var name, source string
	if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &source); err != nil {
		t.Fatal(err)
	}
	var destination string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, p.f)
		destination = pilotCLIBackupRoundTrip(t, p.f, source, station)
	} else {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		defer clear(key)
		dir := t.TempDir()
		archive := filepath.Join(dir, "event.tytbak")
		destination = filepath.Join(dir, "restored.sqlite")
		if err := backup.Create(context.Background(), p.f.db, p.f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err := backup.Restore(context.Background(), archive, destination, p.f.device, key); err != nil {
			t.Fatal(err)
		}
	}

	db, err := localdb.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p.f.db = db
	p.installApp(t, nil)
	if code, _ := request(t, p.f.app, "GET", p.path("/cancellation"), "", p.f.token); code != 401 {
		t.Fatal("restored session active")
	}
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	code, raw := request(t, p.f.app, "GET", p.path("/cancellation"), "", p.f.token)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	var got fiscalcore.CancellationEvidence
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.ProcessedXML, original.ProcessedXML) || got.Exchange.OperationID != original.Exchange.OperationID {
		t.Fatal("restored event changed")
	}
	if code, _ = request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 409 {
		t.Fatal("restored active copy")
	}
	for _, table := range []string{"fiscal_production_cancellation_requests", "fiscal_production_cancellation_results"} {
		if _, err = db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err == nil {
			t.Fatal("immutable event deleted")
		}
	}
}

func TestHTTPFiscalProductionCancellationStaleQueryAndAuthority(t *testing.T) {
	for _, scenario := range []string{"stale-query", "accountant", "logout-at-send", "revocation-at-send"} {
		t.Run(scenario, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.emit(t, prep, "issue-one", 200)
			p.query(t, prep, "query-one", 200)
			switch scenario {
			case "stale-query":
				p.query(t, prep, "query-two", 200)
				p.cancel(t, "cancel-one", "query-one", 409)
			case "accountant":
				p.f.token = periodAccountant(t, p.f)
				p.cancel(t, "cancel-one", "query-one", 403)
			case "logout-at-send":
				p.logoutGuard.Store(true)
				p.crlGuard.Store(true)
				p.crlCalls.Store(0)
				p.cancel(t, "cancel-one", "query-one", 401)
			case "revocation-at-send":
				p.crlGuard.Store(true)
				p.crlCalls.Store(0)
				p.cancel(t, "cancel-one", "query-one", 503)
			}
			if p.cancels.Load() != 0 {
				t.Fatal("unauthorized POST", scenario)
			}
		})
	}
}

func TestHTTPFiscalProductionCancellationInvalidResponsesAndDownloads(t *testing.T) {
	for _, mode := range []string{"cancel-invalid-xml", "cancel-http500", "cancel-redirect", "authorize"} {
		t.Run(mode, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.emit(t, prep, "issue-one", 200)
			p.query(t, prep, "query-one", 200)
			p.mode.Store(mode)
			want := 202
			if mode == "authorize" {
				want = 200
			}
			out := p.cancel(t, "cancel-one", "query-one", want)
			code, raw := request(t, p.f.app, "GET", p.path("/cancellation/xml"), "", p.f.token)
			if want == 200 {
				if code != 200 || !bytes.Equal(raw, out.ProcessedXML) {
					t.Fatal("event download differs", code)
				}
			} else if code != 409 {
				t.Fatal("uncertain event downloadable", code)
			}
			p.cancel(t, "cancel-two", "query-one", 409)
			if p.cancels.Load() != 1 {
				t.Fatal("response induced resend")
			}
		})
	}
}

func TestHTTPFiscalProductionCancellationLogoutKeepsInflightResult(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.emit(t, prep, "issue-one", 200)
	p.query(t, prep, "query-one", 200)
	p.mode.Store("cancel-logout-inflight")
	out := p.cancel(t, "cancel-one", "query-one", 200)
	if out.State != "cancellation_indicated_production" {
		t.Fatal("late response lost")
	}
	if code, _ := request(t, p.f.app, "GET", p.path("/cancellation"), "", p.f.token); code != 401 {
		t.Fatal("logout ignored")
	}
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	same := p.cancel(t, "cancel-one", "query-one", 200)
	if !bytes.Equal(out.ProcessedXML, same.ProcessedXML) || p.cancels.Load() != 1 {
		t.Fatal("new session lost or resent original")
	}
}

func TestHTTPFiscalProductionCancellationLostResponseRecoveredByExactQuery(t *testing.T) {
	for _, mode := range []string{"query-canceled-event", "query-other-event"} {
		t.Run(mode, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.emit(t, prep, "issue-one", 200)
			p.query(t, prep, "query-one", 200)
			p.mode.Store("cancel-drop")
			p.cancel(t, "cancel-one", "query-one", 202)
			p.mode.Store(mode)
			p.query(t, prep, "query-recover-cancellation", 200)
			code, body := request(t, p.f.app, "GET", p.path("/cancellation"), "", p.f.token)
			if code != 200 {
				t.Fatal(code, string(body))
			}
			var out fiscalcore.CancellationEvidence
			if err := json.Unmarshal(body, &out); err != nil {
				t.Fatal(err)
			}
			if mode == "query-canceled-event" {
				if out.State != "cancellation_indicated_production" || out.RecoveryQueryOperationID != "query-recover-cancellation" || out.Exchange.OperationID != "cancel-one" || out.Exchange.State != "uncertain" || len(out.ProcessedXML) == 0 {
					var event, invoice, query fiscalcore.RecordedRequest
					var result fiscalcore.ExchangeResult
					var raw string
					p.f.db.QueryRow(`SELECT request_json FROM fiscal_production_cancellation_requests`).Scan(&raw)
					json.Unmarshal([]byte(raw), &event)
					for kind, target := range map[string]*fiscalcore.RecordedRequest{"issue": &invoice, "query": &query} {
						p.f.db.QueryRow(`SELECT request_json FROM fiscal_production_requests WHERE kind=? ORDER BY rowid DESC LIMIT 1`, kind).Scan(&raw)
						json.Unmarshal([]byte(raw), target)
					}
					p.f.db.QueryRow(`SELECT result_json FROM fiscal_production_results WHERE operation_id=?`, query.OperationID).Scan(&raw)
					json.Unmarshal([]byte(raw), &result)
					proof, queryErr := fiscalcore.ReconcileProductionNFe(invoice, query, result)
					_, eventErr := fiscalcore.ReconcileProductionCancellationQuery(event, invoice, query, result)
					t.Fatalf("did not recover exact event: %s query=%s queryDisposition=%s queryErr=%v eventErr=%v", out.State, out.RecoveryQueryOperationID, proof.Disposition, queryErr, eventErr)
				}
			} else if out.State == "cancellation_indicated_production" {
				t.Fatal("changed signed event accepted")
			}
			if p.cancels.Load() != 1 {
				t.Fatal("recovery resent cancellation")
			}
		})
	}
}

func TestHTTPFiscalProductionCancellationReservationFailureDoesNotSend(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.emit(t, prep, "issue-one", 200)
	p.query(t, prep, "query-one", 200)
	if _, err := p.f.db.Exec(`CREATE TRIGGER fail_event_reservation BEFORE INSERT ON fiscal_production_cancellation_requests BEGIN SELECT RAISE(ABORT,'synthetic rollback');END`); err != nil {
		t.Fatal(err)
	}
	p.cancel(t, "cancel-one", "query-one", 500)
	if p.cancels.Load() != 0 {
		t.Fatal("POST before durable reservation")
	}
	var count int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_cancellation_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial reservation")
	}
	if _, err := p.f.db.Exec(`DROP TRIGGER fail_event_reservation`); err != nil {
		t.Fatal(err)
	}
	p.cancel(t, "cancel-one", "query-one", 200)
	if p.cancels.Load() != 1 {
		t.Fatal("explicit retry of unreserved decision failed")
	}
}
