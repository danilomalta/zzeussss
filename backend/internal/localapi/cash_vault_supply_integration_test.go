package localapi

import (
	"context"
	"crypto/ed25519"
	"reflect"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func TestHTTPVaultSupplyPreservesCustodyEvidenceAndRejectsCrossContext(t *testing.T) {
	f, receiver := vaultSetup(t)
	reviewHTTP(t, f, f.token, "POST", reviewBody("supply-investigate", "investigate", 0, 290), 200)
	reviewHTTP(t, f, f.token, "POST", reviewBody("supply-recount", "recount", 1, 300), 200)
	reviewHTTP(t, f, f.token, "POST", reviewBody("supply-resolve", "resolve", 2, 300), 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	before := vaultCounts(t, f)
	for _, role := range []string{"cashier", "accountant"} {
		id := "supply-" + role
		policyPerson(t, f.db, f.owner, id, role)
		token := loginToken(t, f.app, id)
		vaultResult(t, f, token, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 403)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	contracts, err := entitlementstore.New(f.db, verifier, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "receiver"}
	for _, kind := range []string{"tenant", "store", "device"} {
		actor, device := scope, f.device
		switch kind {
		case "tenant":
			actor.TenantID = "foreign-tenant"
		case "store":
			actor.StoreID = "foreign-store"
		case "device":
			device.DeviceID = "foreign-device"
		}
		_, err := cashvault.Apply(context.Background(), f.db, contracts, actor, device, "vault-one", "supply", cashvault.Input{OperationID: "cross-" + kind, CashSessionID: "turn-one", AmountCents: 50, Reason: "Context must be rejected"})
		if err == nil {
			t.Fatalf("cross %s supply permitted", kind)
		}
	}
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("denied role or context changed money")
	}
	result := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	repeated := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	if result.Vaults[0].BalanceCents != 240 || !repeated.Repeated || !reflect.DeepEqual(result.Entries, repeated.Entries) {
		t.Fatal("supply did not preserve original effect")
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", strings.Replace(supplyVaultBody, `"amount_cents":50`, `"amount_cents":51`, 1), 409)
	var target int64
	if err = f.db.QueryRow(`SELECT opening_cents+(SELECT coalesce(sum(amount_cents),0) FROM cash_movements WHERE cash_session_id=s.id AND tenant_id=s.tenant_id AND store_id=s.store_id) FROM cash_sessions s WHERE id='turn-one'`).Scan(&target); err != nil || target != 750 {
		t.Fatal("cash target amount", target, err)
	}
	if target+readVault(t, f, receiver, "vault-one", 200).BalanceCents != 990 {
		t.Fatal("custody shortfall was silently reimbursed")
	}
	review := reviewHTTP(t, f, f.token, "GET", "", 200).Review
	if review.CountedCents != 300 || review.OriginalCountedCents != 290 || review.State != "resolved" {
		t.Fatal("supply altered investigation")
	}
	if bag := bagRead(t, f, receiver, "bag-one", 200); *bag.DifferenceCents != -10 {
		t.Fatal("supply erased original bag variance")
	}
}
