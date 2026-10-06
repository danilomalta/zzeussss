package localauth

import (
	"context"
	"testing"
)

func TestCashierCanChangeOnlyOwnPasswordAndForeignDeviceCannot(t *testing.T) {
	db, device := fixture(t)
	hash, err := HashPassword("uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"owner", "cashier"} {
		if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a',?,?,'now')`, id, hash); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	owner, err := Login(ctx, db, device, "owner", "uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	cashier, err := Login(ctx, db, device, "cashier", "uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	foreign := device
	foreign.StoreID = "another-store"
	if err := ChangeOwnPassword(ctx, db, foreign, cashier.Token, "uma-senha-bem-longa-1", "outra-senha-bem-longa-2"); err == nil {
		t.Fatal("foreign device accepted")
	}
	if err := ChangeOwnPassword(ctx, db, device, cashier.Token, "uma-senha-bem-longa-1", "outra-senha-bem-longa-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(ctx, db, owner.Token); err != nil {
		t.Fatal("other user session revoked")
	}
	if _, err := Login(ctx, db, device, "owner", "uma-senha-bem-longa-1"); err != nil {
		t.Fatal("other user password modified")
	}
	if _, err := Login(ctx, db, device, "cashier", "outra-senha-bem-longa-2"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRevokedBeforeSecurityOperationCannotMutate(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword("uma-senha-bem-longa-1")
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	session, err := Login(context.Background(), db, device, "owner", "uma-senha-bem-longa-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := Logout(context.Background(), db, session.Token); err != nil {
		t.Fatal(err)
	}
	if err := ChangeOwnPassword(context.Background(), db, device, session.Token, "uma-senha-bem-longa-1", "outra-senha-bem-longa-2"); err == nil {
		t.Fatal("revoked session accepted")
	}
	var events int
	if err := db.QueryRow(`SELECT count(*) FROM account_security_events`).Scan(&events); err != nil || events != 0 {
		t.Fatal("unauthorized audit event")
	}
}
