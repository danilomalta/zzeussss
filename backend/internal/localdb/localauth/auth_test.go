package localauth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func fixture(t *testing.T) (*sql.DB, identity.DeviceContext) {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "auth.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('a','Mercado','now')`,
		`INSERT INTO stores VALUES ('a','s','Loja')`,
		`INSERT INTO devices VALUES ('a','s','d','Caixa')`,
		`INSERT INTO identities VALUES ('owner','Dono','now')`,
		`INSERT INTO identities VALUES ('cashier','Caixa','now')`,
		`INSERT INTO memberships VALUES ('a','owner','owner','active','now')`,
		`INSERT INTO memberships VALUES ('a','cashier','cashier','active','now')`,
		`INSERT INTO membership_stores VALUES ('a','cashier','s')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES ('a','s','d',?,?,9999999999,'approved','owner')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return db, identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "d"}
}

func TestOfflineLoginRevocationAndLogout(t *testing.T) {
	db, device := fixture(t)
	ctx := context.Background()
	hash, err := HashPassword("uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = Login(ctx, db, device, "owner", "errada"); !errors.Is(err, ErrDenied) {
		t.Fatalf("senha incorreta: %v", err)
	}
	session, err := Login(ctx, db, device, "owner", "uma-senha-bem-longa-1")
	if err != nil || session.Token == "" {
		t.Fatalf("login: %+v %v", session, err)
	}
	if _, err = Resolve(ctx, db, session.Token); err != nil {
		t.Fatalf("sessão: %v", err)
	}
	if err = Logout(ctx, db, session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = Resolve(ctx, db, session.Token); !errors.Is(err, ErrDenied) {
		t.Fatalf("logout: %v", err)
	}
	session, err = Login(ctx, db, device, "owner", "uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE memberships SET status='revoked' WHERE tenant_id='a' AND identity_id='owner'`); err != nil {
		t.Fatal(err)
	}
	if _, err = Resolve(ctx, db, session.Token); !errors.Is(err, ErrDenied) {
		t.Fatalf("dono revogado: %v", err)
	}
}

func TestPasswordChangeRevokesSessionsAndDevice(t *testing.T) {
	db, device := fixture(t)
	ctx := context.Background()
	owner := identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "owner"}
	hash, err := HashPassword("uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	session, err := Login(ctx, db, device, "owner", "uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = SetPassword(ctx, db, owner, device, "owner", "outra-senha-bem-longa-2"); err != nil {
		t.Fatal(err)
	}
	if _, err = Resolve(ctx, db, session.Token); !errors.Is(err, ErrDenied) {
		t.Fatalf("sessão antiga: %v", err)
	}
	if _, err = Login(ctx, db, device, "owner", "uma-senha-bem-longa-1"); !errors.Is(err, ErrDenied) {
		t.Fatalf("senha antiga: %v", err)
	}
	newSession, err := Login(ctx, db, device, "owner", "outra-senha-bem-longa-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='a' AND device_id='d'`); err != nil {
		t.Fatal(err)
	}
	if _, err = Resolve(ctx, db, newSession.Token); !errors.Is(err, ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
}
