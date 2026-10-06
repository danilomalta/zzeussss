package identity

import (
	"context"
	"errors"
	"testing"
)

func TestExplicitStaffDenialAlsoBlocksLegacyInviteIssueAndConsumption(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO identities VALUES ('candidate','Candidate','now')`); err != nil {
		t.Fatal(err)
	}
	actor := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	token, err := IssueInvite(ctx, db, actor, "candidate", "employee", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO access_rules VALUES ('market','m1','member','gerente','manage_staff','deny',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := IssueInvite(ctx, db, actor, "candidate", "employee", "m1"); !errors.Is(err, ErrDenied) {
		t.Fatal("blocked manager issued invitation")
	}
	if err := RedeemInvite(ctx, db, "candidate", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("blocked manager invitation consumed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM membership_invites WHERE consumed_unix IS NOT NULL`).Scan(&count); err != nil || count != 0 {
		t.Fatal("denied consumption was persisted")
	}
	if _, err := db.Exec(`UPDATE access_rules SET effect='inherit',revision=2`); err != nil {
		t.Fatal(err)
	}
	if err := RedeemInvite(ctx, db, "candidate", token); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitStaffDenialBlocksDeviceAdministrator(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO access_rules VALUES ('market','m1','member','gerente','manage_staff','deny',1)`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := requireDeviceAdmin(ctx, tx, Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}, "m1"); !errors.Is(err, ErrDenied) {
		t.Fatal("device administrator bypassed explicit denial")
	}
	if err := requireDeviceAdmin(ctx, tx, Scope{IdentityID: "dono", TenantID: "market", StoreID: "m1"}, "m1"); err != nil {
		t.Fatal("owner locked out")
	}
}
