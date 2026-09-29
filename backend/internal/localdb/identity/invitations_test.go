package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

func TestInvitationIsBoundToRecipientAndSingleUse(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	token, err := IssueInvite(ctx, db, Scope{IdentityID: "dono", TenantID: "market"},
		"fornecedor", "stock", "m1")
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, "SELECT token_hash FROM membership_invites").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(token))
	if stored != fmt.Sprintf("%x", hash) || stored == token {
		t.Fatal("código em texto puro ou hash incorreto no banco")
	}
	if err := RedeemInvite(ctx, db, "caixa", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("identidade errada foi aceita: %v", err)
	}
	if err := RedeemInvite(ctx, db, "fornecedor", token); err != nil {
		t.Fatalf("convite do destinatário deveria funcionar: %v", err)
	}
	if err := Can(ctx, db, Scope{"fornecedor", "market", "m1"}, ManageStock); err != nil {
		t.Fatalf("vínculo não aplicado: %v", err)
	}
	if err := Can(ctx, db, Scope{"fornecedor", "market", "m2"}, ManageStock); !errors.Is(err, ErrDenied) {
		t.Fatalf("acesso à loja não concedida: %v", err)
	}
	if err := Can(ctx, db, Scope{"fornecedor", "supplier", "f1"}, ManageProduction); err != nil {
		t.Fatalf("vínculo anterior foi alterado: %v", err)
	}
	if err := RedeemInvite(ctx, db, "fornecedor", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("reuso deveria falhar: %v", err)
	}
	var events int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM membership_invite_events").Scan(&events); err != nil || events != 2 {
		t.Fatalf("auditoria incompleta: count=%d err=%v", events, err)
	}
}

func TestInvitationRespectsIssuerAndExpiration(t *testing.T) {
	ctx := context.Background()
	manager := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	for _, tc := range []struct{ role, store string }{
		{"manager", "m1"}, {"accountant", "m1"}, {"cashier", "m2"},
	} {
		db := setup(t)
		if _, err := IssueInvite(ctx, db, manager, "fornecedor", tc.role, tc.store); !errors.Is(err, ErrDenied) {
			t.Fatalf("gerente conseguiu convidar %s na loja %s: %v", tc.role, tc.store, err)
		}
	}
	db := setup(t)
	token, err := IssueInvite(ctx, db, manager, "fornecedor", "cashier", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE membership_invites SET expires_unix = 1"); err != nil {
		t.Fatal(err)
	}
	if err := RedeemInvite(ctx, db, "fornecedor", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("convite expirado foi aceito: %v", err)
	}
}

func TestRevokedIssuerInvalidatesPendingInvitation(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	token, err := IssueInvite(ctx, db, Scope{IdentityID: "gerente", TenantID: "market"},
		"fornecedor", "cashier", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE memberships SET status = 'revoked' WHERE tenant_id = ? AND identity_id = ?",
		"market", "gerente"); err != nil {
		t.Fatal(err)
	}
	if err := RedeemInvite(ctx, db, "fornecedor", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("convite de emissor revogado foi aceito: %v", err)
	}
}

func TestOwnerRevokesInvitationWithAudit(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	token, err := IssueInvite(ctx, db, Scope{IdentityID: "dono", TenantID: "market"},
		"fornecedor", "stock", "m1")
	if err != nil {
		t.Fatal(err)
	}
	var inviteID string
	if err := db.QueryRowContext(ctx, "SELECT id FROM membership_invites").Scan(&inviteID); err != nil {
		t.Fatal(err)
	}
	if err := RevokeInvite(ctx, db, Scope{IdentityID: "gerente", TenantID: "market"}, inviteID); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("gerente cancelou convite alheio: %v", err)
	}
	if err := RevokeInvite(ctx, db, Scope{IdentityID: "dono", TenantID: "market"}, inviteID); err != nil {
		t.Fatalf("dono não conseguiu cancelar: %v", err)
	}
	if err := RedeemInvite(ctx, db, "fornecedor", token); !errors.Is(err, ErrInviteUnavailable) {
		t.Fatalf("convite cancelado foi aceito: %v", err)
	}
	var events int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM membership_invite_events").Scan(&events); err != nil || events != 2 {
		t.Fatalf("auditoria incompleta: %d, %v", events, err)
	}
}
