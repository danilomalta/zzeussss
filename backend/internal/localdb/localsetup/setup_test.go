package localsetup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func TestInitializeEmptyDatabaseAndLogin(t *testing.T) {
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "first.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{TenantName: "Mercado", StoreName: "Matriz", OwnerName: "Dono", DeviceName: "Caixa", Password: "senha-forte-pessoal-1", PublicKey: public}
	result, err := Initialize(ctx, db, in)
	if err != nil || result.TenantID == "" || result.OwnerID == "" {
		t.Fatalf("instalação: %+v %v", result, err)
	}
	device := identity.DeviceContext{TenantID: result.TenantID, StoreID: result.StoreID, DeviceID: result.DeviceID}
	challenge, err := identity.IssueDeviceChallenge(ctx, db, device)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(private, identity.DeviceAuthMessage(device, challenge))
	if _, err = identity.CompleteDeviceChallenge(ctx, db, device, challenge.ID, signature); err != nil {
		t.Fatal(err)
	}
	if _, err = localauth.Login(ctx, db, device, result.OwnerID, in.Password); err != nil {
		t.Fatal(err)
	}
	if _, err = Initialize(ctx, db, in); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("segunda instalação: %v", err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicou empresa: %d %v", count, err)
	}
}
