package localapi

import (
	"bytes"
	"encoding/json"
	"testing"

	"titansystem-backend/internal/localdb/accesspolicy"
)

func TestHTTPAuditExtendedSourcesNeverExposeKeysOrPayloads(t *testing.T) {
	db, app, o := fixture(t)
	token := loginToken(t, app, o.OwnerID)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO identities VALUES ('candidate','Candidate','now')`, nil},
		{`INSERT INTO membership_invites VALUES ('invite',?,?,'candidate',?,'employee',printf('%064s','PRIVATE_INVITE_HASH'),9999999999,NULL,NULL)`, []any{o.TenantID, o.StoreID, o.OwnerID}},
		{`INSERT INTO membership_invite_events VALUES ('invite1','invite',?,'issued',123)`, []any{o.OwnerID}},
		{`INSERT INTO membership_invite_events VALUES ('invite2','invite',?,'revoked',123)`, []any{o.OwnerID}},
		{`INSERT INTO device_pairing_events VALUES ('pair1',?,?,?,'approved',123)`, []any{o.TenantID, o.DeviceID, o.OwnerID}},
		{`INSERT INTO devices VALUES (?,?,'sender','Sender')`, []any{o.TenantID, o.StoreID}},
		{`INSERT INTO sync_incoming_peers VALUES (?, ?,?,'sender','stock.movement',zeroblob(32),'active',?)`, []any{o.TenantID, o.StoreID, o.DeviceID, o.OwnerID}},
		{`INSERT INTO sync_peer_audit VALUES ('peer1',?, ?,?,'sender','stock.movement','grant',?,'1970-01-01T00:02:03Z')`, []any{o.TenantID, o.StoreID, o.DeviceID, o.OwnerID}},
		{`INSERT INTO device_encryption_keys VALUES (?,?,?,1,zeroblob(32),zeroblob(32),zeroblob(64),?)`, []any{o.TenantID, o.StoreID, o.DeviceID, o.OwnerID}},
		{`INSERT INTO device_encryption_key_audit VALUES ('enc1',?,?,?,1,'PRIVATE_KEY_FINGERPRINT',?,'1970-01-01T00:02:03Z')`, []any{o.TenantID, o.StoreID, o.DeviceID, o.OwnerID}},
		{`INSERT INTO stores VALUES (?,'other','Other')`, []any{o.TenantID}},
		{`INSERT INTO devices VALUES (?,'other','other-receiver','Other')`, []any{o.TenantID}},
		{`INSERT INTO devices VALUES (?,'other','other-sender','Other')`, []any{o.TenantID}},
		{`INSERT INTO membership_invites VALUES ('other-invite',?,'other','candidate',?,'employee',printf('%064s','OTHER_PRIVATE_HASH'),9999999999,NULL,NULL)`, []any{o.TenantID, o.OwnerID}},
		{`INSERT INTO membership_invite_events VALUES ('other-invite-event','other-invite',?,'issued',124)`, []any{o.OwnerID}},
		{`INSERT INTO device_pairings VALUES (?,'other','other-receiver',zeroblob(32),zeroblob(32),9999999999,'approved',?,1,?,1,NULL,NULL)`, []any{o.TenantID, o.OwnerID, o.OwnerID}},
		{`INSERT INTO device_pairing_events VALUES ('other-pair-event',?,'other-receiver',?,'approved',124)`, []any{o.TenantID, o.OwnerID}},
		{`INSERT INTO sync_incoming_peers VALUES (?,'other','other-receiver','other-sender','stock.movement',zeroblob(32),'active',?)`, []any{o.TenantID, o.OwnerID}},
		{`INSERT INTO sync_peer_audit VALUES ('other-peer-event',?,'other','other-receiver','other-sender','stock.movement','grant',?,'1970-01-01T00:02:04Z')`, []any{o.TenantID, o.OwnerID}},
		{`INSERT INTO device_encryption_keys VALUES (?,'other','other-receiver',1,zeroblob(32),zeroblob(32),zeroblob(64),?)`, []any{o.TenantID, o.OwnerID}},
		{`INSERT INTO device_encryption_key_audit VALUES ('other-enc-event',?,'other','other-receiver',1,'PRIVATE_KEY_FINGERPRINT',?,'1970-01-01T00:02:04Z')`, []any{o.TenantID, o.OwnerID}},
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt.query, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	status, body := request(t, app, "GET", "/local/v1/access/audit", "", token)
	var all accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &all) != nil || len(all.Items) != 5 {
		t.Fatalf("sources %d %s", status, body)
	}
	if bytes.Contains(body, []byte("other-")) {
		t.Fatal("extended audit crossed store")
	}
	for _, secret := range []string{"PRIVATE_INVITE_HASH", "PRIVATE_KEY_FINGERPRINT", "token_hash", "public_key", "signature", "payload_json", token} {
		if bytes.Contains(body, []byte(secret)) {
			t.Fatal("unsafe audit field")
		}
	}
	for source, count := range map[string]int{"invite": 2, "pairing": 1, "peer": 1, "encryption": 1, "policy": 0, "account": 0, "staff": 0, "security": 0} {
		status, body = request(t, app, "GET", "/local/v1/access/audit?source="+source, "", token)
		var page accesspolicy.AuditPage
		if status != 200 || json.Unmarshal(body, &page) != nil || len(page.Items) != count {
			t.Fatalf("source %s: %d %s", source, status, body)
		}
	}
	// Ties in time are paginated by unique event reference, not by timestamp alone.
	status, body = request(t, app, "GET", "/local/v1/access/audit?source=invite&limit=1", "", token)
	var first accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &first) != nil || first.NextCursor == "" {
		t.Fatal("filtered cursor missing")
	}
	status, body = request(t, app, "GET", "/local/v1/access/audit?source=invite&limit=1&cursor="+first.NextCursor, "", token)
	var second accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &second) != nil || len(second.Items) != 1 || second.Items[0].Reference == first.Items[0].Reference || second.NextCursor != "" {
		t.Fatal("filtered pagination duplicate")
	}
	status, _ = request(t, app, "GET", "/local/v1/access/audit?source=pairing&cursor="+first.NextCursor, "", token)
	if status != 400 {
		t.Fatal("cursor crossed source filter")
	}
	for _, path := range []string{"/access/audit?source=raw", "/access/audit?source=invite&source=peer", "/access/audit?tenant_id=foreign"} {
		status, _ = request(t, app, "GET", "/local/v1"+path, "", token)
		if status != 400 {
			t.Fatalf("invalid audit query %s: %d", path, status)
		}
	}
}

func TestHTTPAuditDelegateCannotSelectAdministrativeSources(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	policyPerson(t, db, o, "rh", "employee")
	postPolicy(t, app, owner, policyInput("dept", "department", "rh", "", "", "active", 0), 200)
	postPolicy(t, app, owner, policyInput("delegate", "delegation", "rh", "rh", "", "active", 0), 200)
	token := loginToken(t, app, "rh")
	status, body := request(t, app, "GET", "/local/v1/access/audit?department_id=rh&source=policy", "", token)
	if status != 200 || !bytes.Contains(body, []byte("policy:")) {
		t.Fatal("delegate policy audit unavailable")
	}
	for _, source := range []string{"account", "security", "staff", "invite", "pairing", "peer", "encryption"} {
		status, _ = request(t, app, "GET", "/local/v1/access/audit?department_id=rh&source="+source, "", token)
		if status != 403 {
			t.Fatalf("delegate source %s: %d", source, status)
		}
	}
}
