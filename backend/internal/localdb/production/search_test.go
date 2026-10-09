package production

import (
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestOrderSearchValidationAndBoundPredicates(t *testing.T) {
	for _, in := range []OrderSearch{{Offset: -1}, {Offset: MaxQuantity + 1}, {Status: "consumed"}, {LocationID: " "}, {ResponsibleID: strings.Repeat("x", 129)}, {VersionID: "\x00"}} {
		if validOrderSearch(in) {
			t.Fatal("accepted", in)
		}
	}
	in := OrderSearch{Status: "completed", LocationID: "x' OR 1=1 --", ResponsibleID: "owner", VersionID: "version", Offset: MaxQuantity}
	if !validOrderSearch(in) {
		t.Fatal(in)
	}
	where, args := orderSearchPredicate(identity.Scope{TenantID: "tenant", StoreID: "store"}, in)
	if strings.Contains(where, in.LocationID) || len(args) != 5 || args[2] != in.LocationID || !strings.Contains(where, "EXISTS") {
		t.Fatal(where, args)
	}
	in.Status = "approved"
	where, args = orderSearchPredicate(identity.Scope{TenantID: "tenant", StoreID: "store"}, in)
	if !strings.Contains(where, "AND NOT EXISTS") || args[2] != "approved" {
		t.Fatal(where, args)
	}
}
