package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func TestPairingRequiresProofAndManagerApproval(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manager := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	challenge, err := RequestPairing(ctx, db, manager, "device-one", "Celular caixa", public)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApprovePairing(ctx, db, manager, "device-one"); !errors.Is(err, ErrDenied) {
		t.Fatalf("aprovou sem prova: %v", err)
	}
	wrong := ed25519.Sign(private, PairingMessage("supplier", "f1", "device-one", challenge))
	if err := ProvePairing(ctx, db, "market", "device-one", wrong); !errors.Is(err, ErrDenied) {
		t.Fatalf("assinatura de outra empresa foi aceita: %v", err)
	}
	proof := ed25519.Sign(private, PairingMessage("market", "m1", "device-one", challenge))
	if err := ProvePairing(ctx, db, "market", "device-one", proof); err != nil {
		t.Fatalf("prova válida foi negada: %v", err)
	}
	if err := ApprovePairing(ctx, db, Scope{IdentityID: "caixa", TenantID: "market", StoreID: "m1"}, "device-one"); !errors.Is(err, ErrDenied) {
		t.Fatalf("operador aprovou dispositivo: %v", err)
	}
	if err := ApprovePairing(ctx, db, manager, "device-one"); err != nil {
		t.Fatalf("gerente não aprovou: %v", err)
	}
	if err := RevokeDevice(ctx, db, Scope{IdentityID: "dono", TenantID: "market", StoreID: "m2"}, "device-one"); !errors.Is(err, ErrDenied) {
		t.Fatalf("revogação de outra loja foi aceita: %v", err)
	}
	if err := RevokeDevice(ctx, db, manager, "device-one"); err != nil {
		t.Fatalf("revogação na loja falhou: %v", err)
	}
	if err := ProvePairing(ctx, db, "market", "device-one", proof); !errors.Is(err, ErrDenied) {
		t.Fatalf("prova reutilizada foi aceita: %v", err)
	}
	var status string
	if err := db.QueryRowContext(ctx, "SELECT status FROM device_pairings WHERE tenant_id = ? AND device_id = ?",
		"market", "device-one").Scan(&status); err != nil || status != "revoked" {
		t.Fatalf("estado inesperado: %s %v", status, err)
	}
	var events int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_pairing_events").Scan(&events); err != nil || events != 4 {
		t.Fatalf("auditoria incompleta: %d %v", events, err)
	}
}

func TestPairingExpiresAndRespectsStore(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RequestPairing(ctx, db,
		Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m2"}, "wrong-store", "Celular", public); !errors.Is(err, ErrDenied) {
		t.Fatalf("gerente abriu pareamento de outra loja: %v", err)
	}
	challenge, err := RequestPairing(ctx, db,
		Scope{IdentityID: "dono", TenantID: "market", StoreID: "m2"}, "device-two", "Celular", public)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE device_pairings SET challenge_expires_unix = 1 WHERE device_id = ?", "device-two"); err != nil {
		t.Fatal(err)
	}
	proof := ed25519.Sign(private, PairingMessage("market", "m2", "device-two", challenge))
	if err := ProvePairing(ctx, db, "market", "device-two", proof); !errors.Is(err, ErrDenied) {
		t.Fatalf("desafio expirado foi aceito: %v", err)
	}
}
