package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/outgoing"
)

func approvedEncryption(t *testing.T, f *fixture) EncryptionBinding {
	t.Helper()
	binding, err := SignEncryptionBinding(f.receiver, 1, encryptionKey(t), f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, err := TrustEncryptionBinding(context.Background(), f.senderDB, f.actor, f.sender, binding); err != nil || repeated {
		t.Fatalf("aprovação: %t %v", repeated, err)
	}
	return binding
}

func TestPersistentEncryptionKeysDeliverAfterReopen(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "private.json")
	key, err := CreateEncryptionKey(path, f.receiver)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := SignEncryptionBinding(f.receiver, 1, key, f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, binding); err != nil {
		t.Fatal(err)
	}
	var seq int
	var name, dbPath string
	if err := f.senderDB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
		t.Fatal(err)
	}
	if err := f.senderDB.Close(); err != nil {
		t.Fatal(err)
	}
	f.senderDB, err = localdb.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	public, err := TrustedEncryptionPublic(ctx, f.senderDB, f.sender, f.receiver)
	if err != nil || !bytes.Equal(public.Bytes(), key.PublicKey().Bytes()) {
		t.Fatal("vínculo público não persistiu")
	}
	loaded, err := LoadEncryptionKey(path, f.receiver)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), public)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReceiveSealed(ctx, f.receiverDB, f.receiver, f.receiverKey, loaded, envelope)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, result.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	rows, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(rows) != 0 {
		t.Fatal("recibo não confirmou fila")
	}
	receivedCount(t, f, 1)
}

func TestEncryptionBindingRevisionAndAuditAreAtomic(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	first := approvedEncryption(t, f)
	if repeated, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, first); err != nil || !repeated {
		t.Fatalf("repetição: %t %v", repeated, err)
	}
	var count int
	if err := f.senderDB.QueryRow("SELECT COUNT(*) FROM device_encryption_key_audit").Scan(&count); err != nil || count != 1 {
		t.Fatalf("auditoria=%d erro=%v", count, err)
	}
	conflict, err := SignEncryptionBinding(f.receiver, 1, encryptionKey(t), f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("revisão conflitante: %v", err)
	}
	next, err := SignEncryptionBinding(f.receiver, 2, encryptionKey(t), f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.senderDB.Exec(`CREATE TRIGGER fail_encryption_audit BEFORE INSERT ON device_encryption_key_audit BEGIN SELECT RAISE(ABORT,'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, next); err == nil {
		t.Fatal("falha de auditoria ignorada")
	}
	public, err := TrustedEncryptionPublic(ctx, f.senderDB, f.sender, f.receiver)
	if err != nil || !bytes.Equal(public.Bytes(), first.PublicKey) {
		t.Fatal("falha não preservou chave anterior")
	}
	// Remove only the trigger in this disposable test database.
	if _, err := f.senderDB.Exec("DROP TRIGGER fail_encryption_audit"); err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, next); err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, first); !errors.Is(err, ErrConflict) {
		t.Fatalf("revisão antiga: %v", err)
	}
	public, err = TrustedEncryptionPublic(ctx, f.senderDB, f.sender, f.receiver)
	if err != nil || !bytes.Equal(public.Bytes(), next.PublicKey) {
		t.Fatal("revisão nova não ficou ativa")
	}
}

func TestEncryptionBindingRejectsForgedKeysScopesAndManager(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	binding, err := SignEncryptionBinding(f.receiver, 1, encryptionKey(t), f.senderKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, binding); !errors.Is(err, ErrDenied) {
		t.Fatalf("assinante errado: %v", err)
	}
	binding, err = SignEncryptionBinding(f.receiver, 1, encryptionKey(t), f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*EncryptionBinding){
		func(b *EncryptionBinding) { b.Device.TenantID = "foreign" },
		func(b *EncryptionBinding) { b.Device.StoreID = "foreign" },
		func(b *EncryptionBinding) { b.Revision = 2 },
		func(b *EncryptionBinding) { b.PublicKey = make([]byte, 32) },
	} {
		changed := binding
		change(&changed)
		if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, changed); err == nil {
			t.Fatal("vínculo alterado aceito")
		}
	}
	if _, err := f.senderDB.Exec("UPDATE memberships SET role='manager' WHERE identity_id='owner'"); err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, binding); err == nil {
		t.Fatal("gerente aprovou chave")
	}
}

func TestTrustedEncryptionRechecksRevocationAndStoredSignature(t *testing.T) {
	for _, query := range []string{
		"UPDATE memberships SET status='revoked' WHERE identity_id='owner'",
		"UPDATE device_pairings SET status='revoked' WHERE device_id='sender'",
		"UPDATE device_pairings SET status='revoked' WHERE device_id='receiver'",
		"UPDATE device_pairings SET public_key=zeroblob(32) WHERE device_id='receiver'",
		"UPDATE device_encryption_keys SET signature=zeroblob(64)",
	} {
		t.Run(query, func(t *testing.T) {
			f := setup(t)
			approvedEncryption(t, f)
			if _, err := f.senderDB.Exec(query); err != nil {
				t.Fatal(err)
			}
			if _, err := TrustedEncryptionPublic(context.Background(), f.senderDB, f.sender, f.receiver); err == nil {
				t.Fatal("confiança revogada ou adulterada aceita")
			}
		})
	}
}
