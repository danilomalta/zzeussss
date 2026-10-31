package companyregistration

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
	"titansystem-backend/internal/localdb/localsetup"
)

func TestRegistrationBeforeFiscalOrContractAndAfterReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "customer.sqlite")
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Client", StoreName: "Store", OwnerName: "Owner", DeviceName: "PC", Password: "disposable-password-strong", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	a := identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID}
	d := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	session, err := localauth.Login(ctx, db, d, owner.OwnerID, "disposable-password-strong")
	if err != nil {
		t.Fatal(err)
	}
	in := Input{OperationID: "minimum", Stage: "pre_registration", CompanyName: "Client", Responsible: "Owner"}
	saved, err := Save(ctx, db, a, d, session.Token, in)
	if err != nil || saved.Revision != 1 {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	read, err := Read(ctx, db, a, d, session.Token, "minimum")
	if err != nil || read.Input != in || read.Revision != 1 {
		t.Fatal("reopen", err)
	}
	changed := in
	changed.CompanyName = "conflict"
	if _, err = Save(ctx, db, a, d, session.Token, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("overwrite accepted", err)
	}
	a.IdentityID = "foreign"
	if _, err = Read(ctx, db, a, d, session.Token, ""); err == nil {
		t.Fatal("foreign identity accepted")
	}
}
