package fiscalproduction

import (
	"context"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestHomologationFiscalOwnerRecheckedAfterRoleChange(t *testing.T) {
	registry, a, _ := customerFixture(t)
	ctx := context.Background()
	if _, e := registry.SaveCustomerCredentials(ctx, a, "profile", CustomerCredentialInput{OperationID: "owner-guard-fixture", ProfileRevision: 1, Token: []byte("SyntheticOwnerOnlyToken")}); e != nil {
		t.Fatal(e)
	}
	provider, _, e := registry.Resolve(a)
	if e != nil {
		t.Fatal(e)
	}
	focus := provider.(*FocusProvider)
	if e = focus.scope(ctx, a, identity.Sell); e != nil {
		t.Fatal("owner", e)
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		// Simulates a role change while an existing session/request is in flight.
		if _, e = a.DB.Exec(`UPDATE memberships SET role=? WHERE tenant_id=? AND identity_id=?`, role, a.Actor.TenantID, a.Actor.IdentityID); e != nil {
			t.Fatal(e)
		}
		if focus.scope(ctx, a, identity.Sell) == nil || focus.scope(ctx, a, identity.CancelSale) == nil {
			t.Fatal("non-owner fiscal authority", role)
		}
		if (&Runtime{}).ValidateCheckoutHomologation(ctx, a, 2) == nil {
			t.Fatal("native checkout accepted non-owner", role)
		}
	}
}
