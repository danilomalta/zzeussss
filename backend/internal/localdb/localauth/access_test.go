package localauth

import (
	"context"
	"errors"
	"testing"

	"titansystem-backend/internal/localdb/identity"
)

func TestAdminResetAuditFailureRollsBackAndLostReplyRetryDoesNotRevokeNewSessions(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	for _, id := range []string{"owner", "cashier"} {
		if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a',?,?,'now')`, id, hash); err != nil {
			t.Fatal(err)
		}
	}
	owner, err := Login(context.Background(), db, device, "owner", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	cashier, err := Login(context.Background(), db, device, "cashier", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	input := AdminInput{OperationID: "reset", CurrentPassword: ownerPassword, NewPassword: recoveredPassword, Reason: "Requested by employee"}
	if _, err := db.Exec(`CREATE TRIGGER fail_admin BEFORE INSERT ON account_access_operations BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := AdministerAccess(context.Background(), db, device, owner.Token, "cashier", "password_reset", input); err == nil {
		t.Fatal("missing audit accepted")
	}
	if _, err := Resolve(context.Background(), db, cashier.Token); err != nil {
		t.Fatal("session escaped rollback")
	}
	if _, err := Login(context.Background(), db, device, "cashier", ownerPassword); err != nil {
		t.Fatal("password escaped rollback")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_admin`); err != nil {
		t.Fatal(err)
	}
	first, err := AdministerAccess(context.Background(), db, device, owner.Token, "cashier", "password_reset", input)
	if err != nil || first.Repeated {
		t.Fatal("reset failed")
	}
	newSession, err := Login(context.Background(), db, device, "cashier", recoveredPassword)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := AdministerAccess(context.Background(), db, device, owner.Token, "cashier", "password_reset", input)
	if err != nil || !retry.Repeated {
		t.Fatal("retry failed")
	}
	if _, err := Resolve(context.Background(), db, newSession.Token); err != nil {
		t.Fatal("retry revoked a new session")
	}
	input.Reason = "different reason"
	if _, err := AdministerAccess(context.Background(), db, device, owner.Token, "cashier", "password_reset", input); !errors.Is(err, ErrAccessConflict) {
		t.Fatal("conflicting operation accepted")
	}
}

func TestManagerCanManageCashierButCannotManageOwnerPeerOrForeignDevice(t *testing.T) {
	db, device := fixture(t)
	for _, q := range []string{
		`INSERT INTO identities VALUES ('manager','Manager','now')`,
		`INSERT INTO identities VALUES ('peer','Peer','now')`,
		`INSERT INTO memberships VALUES ('a','manager','manager','active','now')`,
		`INSERT INTO memberships VALUES ('a','peer','manager','active','now')`,
		`INSERT INTO membership_stores VALUES ('a','manager','s')`,
		`INSERT INTO membership_stores VALUES ('a','peer','s')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	hash, _ := HashPassword(ownerPassword)
	for _, id := range []string{"manager", "cashier", "peer"} {
		if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a',?,?,'now')`, id, hash); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := Login(context.Background(), db, device, "manager", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	input := AdminInput{OperationID: "manager-reset", CurrentPassword: ownerPassword, NewPassword: recoveredPassword, Reason: "Employee request"}
	for _, id := range []string{"owner", "peer", "manager"} {
		if _, err := AdministerAccess(context.Background(), db, device, manager.Token, id, "password_reset", input); !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("manager authorized for %s", id)
		}
	}
	foreign := identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "other"}
	if _, err := AdministerAccess(context.Background(), db, foreign, manager.Token, "cashier", "password_reset", input); !errors.Is(err, ErrDenied) {
		t.Fatal("foreign device accepted")
	}
	if _, err := AdministerAccess(context.Background(), db, device, manager.Token, "cashier", "password_reset", input); err != nil {
		t.Fatal(err)
	}
	input.OperationID = "manager-revoke"
	input.NewPassword = ""
	if _, err := db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='manager'`); err != nil {
		t.Fatal(err)
	}
	if _, err := AdministerAccess(context.Background(), db, device, manager.Token, "cashier", "sessions_revoked", input); !errors.Is(err, ErrDenied) {
		t.Fatal("revoked manager accepted")
	}
}

func TestAdminResetDoesNotChangeSameIdentityCredentialInAnotherTenant(t *testing.T) {
	db, device := fixture(t)
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('b','Other','now')`,
		`INSERT INTO stores VALUES ('b','s','Other store')`,
		`INSERT INTO memberships VALUES ('b','cashier','cashier','active','now')`,
		`INSERT INTO membership_stores VALUES ('b','cashier','s')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	hash, _ := HashPassword(ownerPassword)
	for _, id := range []string{"owner", "cashier"} {
		if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a',?,?,'now')`, id, hash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('b','cashier',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	owner, err := Login(context.Background(), db, device, "owner", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	input := AdminInput{OperationID: "reset", CurrentPassword: ownerPassword, NewPassword: recoveredPassword, Reason: "Local request"}
	if _, err := AdministerAccess(context.Background(), db, device, owner.Token, "cashier", "password_reset", input); err != nil {
		t.Fatal(err)
	}
	var unchanged []byte
	if err := db.QueryRow(`SELECT password_hash FROM local_passwords WHERE tenant_id='b' AND identity_id='cashier'`).Scan(&unchanged); err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(hash) {
		t.Fatal("foreign tenant password changed")
	}
}

func TestExplicitRecoveryRevocationKeepsOwnerSession(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, make([]byte, 32), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	owner, err := Login(context.Background(), db, device, "owner", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := RevokeOwnRecovery(context.Background(), db, device, owner.Token, "wrong"); !errors.Is(err, ErrDenied) {
		t.Fatal("wrong password accepted")
	}
	if err := RevokeOwnRecovery(context.Background(), db, device, owner.Token, ownerPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), db, owner.Token); err != nil {
		t.Fatal("owner session revoked")
	}
	var active int
	if err := db.QueryRow(`SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NULL`).Scan(&active); err != nil || active != 0 {
		t.Fatal("recovery key still active")
	}
}
