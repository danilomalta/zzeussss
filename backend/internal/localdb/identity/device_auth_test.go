package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"testing"
)

func approvedDevice(t *testing.T) (DeviceContext, ed25519.PrivateKey, *sql.DB) {
	t.Helper()
	db := setup(t)
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	device := DeviceContext{TenantID: "market", StoreID: "m1", DeviceID: "phone-one"}
	manager := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	challenge, err := RequestPairing(ctx, db, manager, device.DeviceID, "Celular", public)
	if err != nil {
		t.Fatal(err)
	}
	proof := ed25519.Sign(private, PairingMessage(device.TenantID, device.StoreID, device.DeviceID, challenge))
	if err := ProvePairing(ctx, db, device.TenantID, device.DeviceID, proof); err != nil {
		t.Fatal(err)
	}
	if err := ApprovePairing(ctx, db, manager, device.DeviceID); err != nil {
		t.Fatal(err)
	}
	return device, private, db
}

func TestDeviceChallengeSingleUseAndScope(t *testing.T) {
	device, private, db := approvedDevice(t)
	ctx := context.Background()
	challenge, err := IssueDeviceChallenge(ctx, db, device)
	if err != nil {
		t.Fatal(err)
	}
	wrong := DeviceContext{TenantID: "supplier", StoreID: "f1", DeviceID: device.DeviceID}
	if _, err := CompleteDeviceChallenge(ctx, db, wrong, challenge.ID,
		ed25519.Sign(private, DeviceAuthMessage(device, challenge))); !errors.Is(err, ErrDenied) {
		t.Fatalf("outra empresa aceitou desafio: %v", err)
	}
	proof := ed25519.Sign(private, DeviceAuthMessage(device, challenge))
	got, err := CompleteDeviceChallenge(ctx, db, device, challenge.ID, proof)
	if err != nil || got != device {
		t.Fatalf("desafio válido falhou: dispositivo=%+v erro=%v", got, err)
	}
	if _, err := CompleteDeviceChallenge(ctx, db, device, challenge.ID, proof); !errors.Is(err, ErrDenied) {
		t.Fatalf("replay aceito: %v", err)
	}
}

func TestRevocationAndExpirationBlockDeviceAuthentication(t *testing.T) {
	device, private, db := approvedDevice(t)
	ctx := context.Background()
	challenge, err := IssueDeviceChallenge(ctx, db, device)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE device_auth_challenges SET expires_unix = 1 WHERE id = ?", challenge.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteDeviceChallenge(ctx, db, device, challenge.ID,
		ed25519.Sign(private, DeviceAuthMessage(device, challenge))); !errors.Is(err, ErrDenied) {
		t.Fatalf("desafio expirado aceito: %v", err)
	}
	challenge, err = IssueDeviceChallenge(ctx, db, device)
	if err != nil {
		t.Fatal(err)
	}
	manager := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	if err := RevokeDevice(ctx, db, manager, device.DeviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteDeviceChallenge(ctx, db, device, challenge.ID,
		ed25519.Sign(private, DeviceAuthMessage(device, challenge))); !errors.Is(err, ErrDenied) {
		t.Fatalf("desafio emitido antes da revogação foi aceito: %v", err)
	}
	if _, err := IssueDeviceChallenge(ctx, db, device); !errors.Is(err, ErrDenied) {
		t.Fatalf("dispositivo revogado recebeu desafio: %v", err)
	}
}
