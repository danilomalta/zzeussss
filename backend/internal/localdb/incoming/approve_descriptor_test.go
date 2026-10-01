package incoming

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"testing"
)

func peerApprovalDescriptor(t *testing.T, f *fixture) ([]byte, PublicFingerprints) {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := CreatePublicDescriptor(f.sender, 1, key, f.senderKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	_, prints, err := ParsePublicDescriptor(raw)
	if err != nil {
		t.Fatal(err)
	}
	return raw, prints
}
func approvalRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPublicPeerApprovalIsAtomicIdempotentAndEventScoped(t *testing.T) {
	f := setup(t)
	raw, prints := peerApprovalDescriptor(t, f)
	ctx := context.Background()
	events := []string{"stock.operation", "sale.committed"}
	for i := 0; i < 2; i++ {
		if err := ApprovePublicPeer(ctx, f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, events); err != nil {
			t.Fatal(err)
		}
	}
	if events[0] != "stock.operation" {
		t.Fatal("caller input changed")
	}
	if approvalRows(t, f.receiverDB, "device_encryption_key_audit") != 1 || approvalRows(t, f.receiverDB, "sync_peer_audit") != 2 || approvalRows(t, f.receiverDB, "sync_incoming_peers") != 2 {
		t.Fatal("duplicate audit or unexpected grants")
	}
	if _, err := TrustedEncryptionPublic(ctx, f.receiverDB, f.receiver, f.sender); err != nil {
		t.Fatal(err)
	}
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, f.message(t)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := RevokePublicPeerEvents(ctx, f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, []string{"sale.committed"}); err != nil {
			t.Fatal(err)
		}
	}
	if approvalRows(t, f.receiverDB, "sync_peer_audit") != 3 {
		t.Fatal("repeated revoke duplicated audit")
	}
	var status string
	if err := f.receiverDB.QueryRow("SELECT status FROM sync_incoming_peers WHERE event_type='stock.operation'").Scan(&status); err != nil || status != "active" {
		t.Fatal("unselected grant revoked")
	}
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, f.message(t)); err == nil {
		t.Fatal("revoked grant still accepts replay")
	}
}

func TestPublicPeerTrustAloneDoesNotGrantEvents(t *testing.T) {
	f := setup(t)
	raw, prints := peerApprovalDescriptor(t, f)
	if err := ApprovePublicPeer(context.Background(), f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, nil); err != nil {
		t.Fatal(err)
	}
	if approvalRows(t, f.receiverDB, "device_encryption_keys") != 1 || approvalRows(t, f.receiverDB, "sync_incoming_peers") != 0 {
		t.Fatal("trust granted incoming events")
	}
}

func TestPublicPeerApprovalRejectsIdentityScopeAndFingerprintFailures(t *testing.T) {
	for _, scenario := range []string{"fingerprint", "unpaired", "foreign-key", "owner-revoked", "local-revoked", "manager", "unknown-event", "duplicate-event", "foreign-company"} {
		t.Run(scenario, func(t *testing.T) {
			f := setup(t)
			raw, prints := peerApprovalDescriptor(t, f)
			events := []string{"sale.committed"}
			var query string
			switch scenario {
			case "fingerprint":
				prints.SigningSHA256 = "wrong"
			case "unpaired":
				query = "UPDATE device_pairings SET status='pending' WHERE device_id='sender'"
			case "foreign-key":
				query = "UPDATE device_pairings SET public_key=zeroblob(32) WHERE device_id='sender'"
			case "owner-revoked":
				query = "UPDATE memberships SET status='revoked'"
			case "local-revoked":
				query = "UPDATE device_pairings SET status='revoked' WHERE device_id='receiver'"
			case "manager":
				query = "UPDATE memberships SET role='manager'"
			case "unknown-event":
				events = []string{"unknown"}
			case "duplicate-event":
				events = []string{"sale.committed", "sale.committed"}
			case "foreign-company":
				descriptor, _ := ParseDescriptorForTest(t, raw)
				descriptor.Binding.Device.TenantID = "foreign"
				descriptor.Binding.Signature = ed25519.Sign(f.senderKey, bindingBytes(descriptor.Binding))
				raw, _ = json.Marshal(descriptor)
				_, prints, _ = ParsePublicDescriptor(raw)
			}
			if query != "" {
				if _, err := f.receiverDB.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			if err := ApprovePublicPeer(context.Background(), f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, events); err == nil {
				t.Fatal("unauthorized approval")
			}
			if approvalRows(t, f.receiverDB, "device_encryption_keys") != 0 || approvalRows(t, f.receiverDB, "sync_incoming_peers") != 0 {
				t.Fatal("failed approval changed state")
			}
		})
	}
}

func ParseDescriptorForTest(t *testing.T, raw []byte) (PublicDescriptor, PublicFingerprints) {
	t.Helper()
	descriptor, prints, err := ParsePublicDescriptor(raw)
	if err != nil {
		t.Fatal(err)
	}
	return descriptor, prints
}

func TestPublicPeerAuditFailureRollsBackKeyAndAllGrants(t *testing.T) {
	f := setup(t)
	raw, prints := peerApprovalDescriptor(t, f)
	if _, err := f.receiverDB.Exec("CREATE TRIGGER reject_peer_audit BEFORE INSERT ON sync_peer_audit WHEN NEW.event_type='stock.operation' BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := ApprovePublicPeer(context.Background(), f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, []string{"sale.committed", "stock.operation"}); err == nil {
		t.Fatal("audit failure hidden")
	}
	for _, table := range []string{"device_encryption_keys", "device_encryption_key_audit", "sync_incoming_peers", "sync_peer_audit"} {
		if approvalRows(t, f.receiverDB, table) != 0 {
			t.Fatal("partial approval: " + table)
		}
	}
}

func TestPublicPeerRejectsSilentRotationAndAllowsRevokingRevokedPair(t *testing.T) {
	f := setup(t)
	raw, prints := peerApprovalDescriptor(t, f)
	ctx := context.Background()
	if err := ApprovePublicPeer(ctx, f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, []string{"sale.committed"}); err != nil {
		t.Fatal(err)
	}
	changed, changedPrints := peerApprovalDescriptor(t, f)
	if err := ApprovePublicPeer(ctx, f.receiverDB, f.actor, f.receiver, changed, changedPrints.SigningSHA256, changedPrints.EncryptionSHA256, nil); err == nil {
		t.Fatal("silent rotation accepted")
	}
	if _, err := f.receiverDB.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id='sender'"); err != nil {
		t.Fatal(err)
	}
	if err := RevokePublicPeerEvents(ctx, f.receiverDB, f.actor, f.receiver, raw, prints.SigningSHA256, prints.EncryptionSHA256, []string{"sale.committed"}); err != nil {
		t.Fatal(err)
	}
	if approvalRows(t, f.receiverDB, "sync_peer_audit") != 2 {
		t.Fatal("missing revocation audit")
	}
}
