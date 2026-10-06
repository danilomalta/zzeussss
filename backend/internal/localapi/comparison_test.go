package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/comparison"
)

func comparisonInput() comparison.Input {
	return comparison.Input{OperationID: "site-op", ID: "site-1", Name: "Netshoes", Origin: "https://www.netshoes.com.br", SearchTemplate: "https://www.netshoes.com.br/busca?q={query}", Status: "active"}
}
func postSite(t *testing.T, f *httpContractFixture, in comparison.Input, want int) comparison.Result {
	t.Helper()
	body, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	status, reply := request(t, f.app, "POST", "/local/v1/comparison-sites", string(body), f.token)
	if status != want {
		t.Fatalf("save: %d want %d %s", status, want, reply)
	}
	var out comparison.Result
	if status == 200 || status == 201 {
		if e = json.Unmarshal(reply, &out); e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func licensedComparison(t *testing.T) *httpContractFixture {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory}), 200)
	return f
}
func siteCount(t *testing.T, f *httpContractFixture, table string) int {
	t.Helper()
	var n int
	if e := f.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestHTTPComparisonSitesCreateReplayRevisionAndStableReceipt(t *testing.T) {
	f := licensedComparison(t)
	in := comparisonInput()
	v := postSite(t, f, in, 201)
	if v.Site.Revision != 1 || v.Site.Mode != "external_link" {
		t.Fatal(v)
	}
	again := postSite(t, f, in, 200)
	if !again.Repeated || again.Site.ChangedAt != v.Site.ChangedAt {
		t.Fatal("unstable retry")
	}
	update := in
	update.OperationID = "site-edit"
	update.ExpectedRevision = 1
	update.Name = "Netshoes consulta"
	update.Status = "inactive"
	edited := postSite(t, f, update, 200)
	if edited.Site.Revision != 2 {
		t.Fatal(edited)
	}
	postSite(t, f, in, 200)
	status, body := request(t, f.app, "GET", "/local/v1/comparison-site-operations/site-op", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"revision":1`)) {
		t.Fatalf("old operation %d %s", status, body)
	}
	stale := update
	stale.OperationID = "stale"
	postSite(t, f, stale, 409)
	forged := in
	forged.Name = "Changed payload"
	postSite(t, f, forged, 409)
	duplicate := in
	duplicate.ID = "other-id"
	duplicate.OperationID = "other-op"
	postSite(t, f, duplicate, 409)
	if siteCount(t, f, "comparison_sites") != 1 || siteCount(t, f, "comparison_site_operations") != 2 {
		t.Fatal("duplicated site or audit")
	}
	status, body = request(t, f.app, "GET", "/local/v1/comparison-sites", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"automatic_state":"not_configured"`)) || !bytes.Contains(body, []byte(`"status":"inactive"`)) {
		t.Fatalf("list %d %s", status, body)
	}
}
func TestHTTPComparisonSitesRejectUnsafeURLsAndAmbiguousPayload(t *testing.T) {
	f := licensedComparison(t)
	for _, origin := range []string{"http://www.netshoes.com.br", "https://127.0.0.1", "https://localhost", "https://store.local", "https://user:pass@www.netshoes.com.br", "https://www.netshoes.com.br:443", "https://www.netshoes.com.br/busca", "https://example.123", "https://www.netshoes.com.br#x"} {
		in := comparisonInput()
		in.Origin = origin
		postSite(t, f, in, 400)
	}
	for _, template := range []string{"javascript:alert(1)", "https://evil.com/busca?q={query}", "https://www.netshoes.com.br/busca?q={query}&q={query}", "https://user:pass@www.netshoes.com.br/?q={query}", "https://www.netshoes.com.br/#q={query}", "https://www.netshoes.com.br/{unknown}"} {
		in := comparisonInput()
		in.SearchTemplate = template
		postSite(t, f, in, 400)
	}
	raw, _ := json.Marshal(comparisonInput())
	for _, body := range []string{strings.TrimSuffix(string(raw), "}") + `,"tenant_id":"other"}`, strings.TrimSuffix(string(raw), "}") + `,"name":"duplicate"}`, strings.Replace(string(raw), `"expected_revision":0`, `"expected_revision":0.5`, 1)} {
		status, _ := request(t, f.app, "POST", "/local/v1/comparison-sites", body, f.token)
		if status != 400 {
			t.Fatalf("ambiguous: %d", status)
		}
	}
	if siteCount(t, f, "comparison_sites") != 0 {
		t.Fatal("invalid input wrote data")
	}
}
func TestHTTPComparisonSitesReadPermissionWriteLicenseAndRevocation(t *testing.T) {
	f := httpContractSetup(t)
	postSite(t, f, comparisonInput(), 403)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff}), 200)
	postSite(t, f, comparisonInput(), 403)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
	postSite(t, f, comparisonInput(), 201)
	if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	status, _ := request(t, f.app, "GET", "/local/v1/comparison-sites", "", f.token)
	if status != 200 {
		t.Fatalf("cashier read %d", status)
	}
	postSite(t, f, comparisonInput(), 403)
	status, _ = request(t, f.app, "GET", "/local/v1/comparison-site-operations/site-op", "", f.token)
	if status != 403 {
		t.Fatalf("cashier operation %d", status)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/comparison-sites", "", "")
	if status != 401 {
		t.Fatal("anonymous")
	}
	if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/comparison-sites", "", f.token)
	if status != 401 {
		t.Fatal("revoked")
	}
}
func TestHTTPComparisonSiteAuditFailureRollsBackWholeConfiguration(t *testing.T) {
	for _, failure := range []string{"ABORT,'test'", "IGNORE"} {
		t.Run(failure, func(t *testing.T) {
			f := licensedComparison(t)
			in := comparisonInput()
			postSite(t, f, in, 201)
			if _, e := f.db.Exec(`CREATE TRIGGER fail_comparison BEFORE INSERT ON comparison_site_operations BEGIN SELECT RAISE(` + failure + `); END`); e != nil {
				t.Fatal(e)
			}
			in.OperationID = "failed-edit"
			in.ExpectedRevision = 1
			in.Status = "inactive"
			body, _ := json.Marshal(in)
			status, _ := request(t, f.app, "POST", "/local/v1/comparison-sites", string(body), f.token)
			if status == 200 || status == 201 {
				t.Fatal("audit failed but success")
			}
			var revision int
			var state string
			if e := f.db.QueryRow(`SELECT revision,status FROM comparison_sites`).Scan(&revision, &state); e != nil {
				t.Fatal(e)
			}
			if revision != 1 || state != "active" || siteCount(t, f, "comparison_site_operations") != 1 {
				t.Fatal("configuration not rolled back")
			}
		})
	}
}
func TestHTTPComparisonSitesAreTenantScopedAndLimitIsEnforced(t *testing.T) {
	f := licensedComparison(t)
	if _, e := f.db.Exec(`INSERT INTO tenants VALUES('foreign','Outra','now')`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO memberships VALUES('foreign',?,'owner','active','now')`, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO comparison_sites VALUES('foreign','secret','Outro site','https://www.netshoes.com.br','','active',1,?,'now')`, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	status, body := request(t, f.app, "GET", "/local/v1/comparison-sites", "", f.token)
	if status != 200 || bytes.Contains(body, []byte("secret")) {
		t.Fatalf("foreign %d %s", status, body)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/comparison-sites?tenant_id=foreign", "", f.token)
	if status != 400 {
		t.Fatal("query tenant accepted")
	}
	for i := 0; i < 32; i++ {
		in := comparisonInput()
		in.OperationID = fmt.Sprintf("op-%d", i)
		in.ID = fmt.Sprintf("site-%d", i)
		in.Origin = fmt.Sprintf("https://shop%d.com.br", i)
		in.SearchTemplate = ""
		postSite(t, f, in, 201)
	}
	postSite(t, f, comparisonInput(), 409)
}
func TestHTTPComparisonConcurrentIdenticalWritesDoNotDuplicate(t *testing.T) {
	f := licensedComparison(t)
	body, _ := json.Marshal(comparisonInput())
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", "/local/v1/comparison-sites", string(body), f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 && status != 201 {
			t.Fatalf("concurrent %d", status)
		}
	}
	if siteCount(t, f, "comparison_sites") != 1 || siteCount(t, f, "comparison_site_operations") != 1 {
		t.Fatal("duplicate")
	}
}

func TestHTTPComparisonConfigurationSurvivesDatabaseReopen(t *testing.T) {
	f := licensedComparison(t)
	postSite(t, f, comparisonInput(), 201)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	f.db = reopened
	f.app, err = New(reopened, f.device)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, f.app, "GET", "/local/v1/comparison-sites", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte("Netshoes")) {
		t.Fatalf("reopened %d %s", status, body)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/comparison-site-operations/site-op", "", f.token)
	if status != 200 {
		t.Fatalf("operation after reopen %d", status)
	}
	postSite(t, f, comparisonInput(), 503)
}
