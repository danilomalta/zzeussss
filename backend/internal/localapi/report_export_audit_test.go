package localapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func TestReportExportAuditBindsArtifactScopeAndRevocation(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	path := accountingPath + "/report.csv" + accountingQuery(closedFirst, periodEnd)
	code, data := request(t, f.app, "GET", path, "", accountant)
	if code != 200 {
		t.Fatal("export", code)
	}
	var raw, tenant, store string
	if err := f.db.QueryRow(`SELECT tenant_id,store_id,payload_json FROM outbox WHERE event_type='audit.report_export'`).Scan(&tenant, &store, &raw); err != nil {
		t.Fatal("missing export audit", err)
	}
	var audit map[string]any
	if json.Unmarshal([]byte(raw), &audit) != nil {
		t.Fatal("audit JSON")
	}
	h := sha256.Sum256(data)
	if tenant != f.owner.TenantID || store != f.owner.StoreID || audit["sha256"] != hex.EncodeToString(h[:]) || audit["from"] != closedFirst || audit["to"] != periodEnd {
		t.Fatal("audit does not identify actual artifact/context")
	}
	if bytes.Contains([]byte(raw), []byte(accountant)) || bytes.Contains([]byte(raw), []byte("Troco")) || bytes.Contains([]byte(raw), []byte("password")) {
		t.Fatal("audit contains token or business text")
	}
	if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE tenant_id=? AND identity_id='period-accountant'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if code, _ = request(t, f.app, "GET", path, "", accountant); code != 401 {
		t.Fatal("revoked access", code)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='audit.report_export'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("revocation generated export", err)
	}
}

func TestReportExportAuditManagementAndFiscalArtifacts(t *testing.T) {
	check := func(t *testing.T, f *httpContractFixture, path, kind string) {
		t.Helper()
		code, data := request(t, f.app, "GET", path, "", f.token)
		if code != 200 {
			t.Fatal("artifact export", code)
		}
		var raw string
		if err := f.db.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='audit.report_export'`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		h := sha256.Sum256(data)
		if json.Unmarshal([]byte(raw), &record) != nil || record["kind"] != kind || record["sha256"] != hex.EncodeToString(h[:]) || bytes.Contains([]byte(raw), []byte(f.token)) {
			t.Fatal("artifact audit mismatch")
		}
	}
	t.Run("management-page", func(t *testing.T) {
		f := httpContractSetup(t)
		check(t, f, "/local/v1/management/sales/products.csv?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z", "management_sales_page")
	})
	t.Run("fiscal-archive", func(t *testing.T) {
		f, intent, _ := danfeSQLFixture(t)
		querySQLRecorder(t, f, f.db, intent, "danfe-authorized", f.token)
		q := url.Values{"from": {time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}, "until": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "lang": {"pt-BR"}}
		check(t, f.httpContractFixture, "/local/v1/fiscal/accounting-export?"+q.Encode(), "fiscal_homologation_period")
	})
}

func TestReportExportAuditConcurrentDownloadsNeverMutateCash(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	var before, after int
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_journal_entries`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	path := accountingPath + "/report.csv" + accountingQuery(closedFirst, periodEnd)
	var wg sync.WaitGroup
	status := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); code, _ := request(t, f.app, "GET", path, "", accountant); status <- code }()
	}
	wg.Wait()
	close(status)
	for code := range status {
		if code != 200 {
			t.Fatal("concurrent export", code)
		}
	}
	var count, distinct int
	if err := f.db.QueryRow(`SELECT count(*),count(DISTINCT operation_id) FROM outbox WHERE event_type='audit.report_export'`).Scan(&count, &distinct); err != nil || count != 4 || distinct != 4 {
		t.Fatal("missing or duplicate audit", err)
	}
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_journal_entries`).Scan(&after); err != nil || before != after {
		t.Fatal("export changed ledger", err)
	}
}

func TestReportExportAuditRevalidatesOriginalSessionAndScope(t *testing.T) {
	for _, kind := range []string{"revoked-session", "revoked-permission", "tenant", "device"} {
		t.Run(kind, func(t *testing.T) {
			f := httpContractSetup(t)
			ss, err := localauth.Resolve(context.Background(), f.db, f.token)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "revoked-session":
				_, err = f.db.Exec(`UPDATE local_sessions SET revoked_unix=1`)
			case "revoked-permission":
				// Owner recovery deliberately cannot be locked out by access rules.
				// Exercise revocation of an explicitly store-scoped accountant instead.
				if _, err = f.db.Exec(`UPDATE memberships SET role='accountant' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
				if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				_, err = f.db.Exec(`INSERT INTO access_rules VALUES(?,?,'member',?,'view_accounting','deny',1)`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
			case "tenant":
				ss.Actor.TenantID = "foreign"
			case "device":
				ss.Device.DeviceID = "foreign"
			}
			if err != nil {
				t.Fatal(err)
			}
			s := Server{DB: f.db, Device: f.device}
			app := fiber.New()
			app.Get("/probe", func(c *fiber.Ctx) error {
				c.Locals("session", ss)
				if err := s.recordReportExport(c, "closed_cash_period", closedFirst, periodEnd, []byte("synthetic CSV")); !errors.Is(err, identity.ErrDenied) {
					t.Error("stale/foreign export accepted", err)
				}
				return c.SendStatus(403)
			})
			request(t, app, "GET", "/probe", "", "")
			var n int
			if err = f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='audit.report_export'`).Scan(&n); err != nil || n != 0 {
				t.Fatal("unauthorized audit persisted", err)
			}
		})
	}
}

func TestReportExportAuditFailureNeverReleasesCSV(t *testing.T) {
	for _, action := range []string{"IGNORE", "ABORT,'synthetic audit write fault'"} {
		t.Run(action, func(t *testing.T) {
			f, accountant, _ := accountingSetup(t)
			if _, err := f.db.Exec(`CREATE TRIGGER reject_export BEFORE INSERT ON outbox WHEN NEW.event_type='audit.report_export' BEGIN SELECT RAISE(` + action + `); END`); err != nil {
				t.Fatal(err)
			}
			path := accountingPath + "/report.csv" + accountingQuery(closedFirst, periodEnd)
			code, body := request(t, f.app, "GET", path, "", accountant)
			if code != 500 || bytes.Contains(body, []byte("format_version")) {
				t.Fatal("unrecorded artifact released", code)
			}
			var count int
			if err := f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='audit.report_export'`).Scan(&count); err != nil || count != 0 {
				t.Fatal("partial audit", err)
			}
		})
	}
}
