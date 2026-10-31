package fiscal

import (
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestCheckoutOriginalRequestAddressIsolation(t *testing.T) {
	a := identity.Scope{TenantID: "tenant-a", StoreID: "store", IdentityID: "owner"}
	d := identity.DeviceContext{TenantID: "tenant-a", StoreID: "store", DeviceID: "device"}
	key := checkoutRequestOperation(a, d, "sale", "original-hash")
	if key != checkoutRequestOperation(a, d, "sale", "original-hash") {
		t.Fatal("unstable original address")
	}
	variants := []string{checkoutRequestOperation(identity.Scope{TenantID: "tenant-b", StoreID: "store"}, d, "sale", "original-hash"), checkoutRequestOperation(a, d, "other-sale", "original-hash"), checkoutRequestOperation(a, d, "sale", "another-attempt"), checkoutRequestOperation(a, identity.DeviceContext{DeviceID: "other-device"}, "sale", "original-hash")}
	for _, other := range variants {
		if key == other {
			t.Fatal("original request scope collision")
		}
	}
}
