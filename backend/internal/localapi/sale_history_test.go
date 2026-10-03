package localapi

import (
	"bytes"
	"testing"
)

func TestHTTPSaleHistoryScopesPaginationAndRevocation(t *testing.T) {
	f := salesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	status, body := request(t, f.app, "GET", "/local/v1/sales?limit=1&offset=0", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"sale_id":"sale-one"`)) {
		t.Fatalf("history: %d %s", status, body)
	}
	status, body = request(t, f.app, "GET", "/local/v1/sales?limit=1&offset=1", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"items":[]`)) {
		t.Fatalf("page: %d %s", status, body)
	}
	for _, path := range []string{"/local/v1/sales?limit=51", "/local/v1/sales?offset=-1", "/local/v1/sales?limit=x"} {
		status, _ = request(t, f.app, "GET", path, "", f.token)
		if status != 400 {
			t.Fatalf("invalid pagination: %d", status)
		}
	}
	status, _ = request(t, f.app, "GET", "/local/v1/sales", "", "")
	if status != 401 {
		t.Fatalf("anonymous: %d", status)
	}
	if _, err := f.db.Exec(`INSERT INTO identities VALUES ('history-other','Outro','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO memberships VALUES (?,'history-other','cashier','active','now')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE sale_operations SET actor_identity_id='history-other' WHERE tenant_id=?`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, f.app, "GET", "/local/v1/sales", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"items":[]`)) {
		t.Fatalf("other actor exposed: %d %s", status, body)
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/sales", "", f.token)
	if status != 401 && status != 403 {
		t.Fatalf("revoked: %d", status)
	}
}
