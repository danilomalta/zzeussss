package production

import (
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestLotSearchValidationAndBoundFilters(t *testing.T) {
	for _, in := range []LotSearch{{Offset: -1}, {Offset: MaxQuantity + 1}, {Status: "completed"}, {Quality: "approved"}, {Expiry: "expired"}, {ExpiresFrom: "2025-02-29"}, {ExpiresTo: "0000-01-01"}, {ExpiresFrom: "2024-2-01"}, {ExpiresFrom: "2024-03-01", ExpiresTo: "2024-02-29"}, {Expiry: "undated", ExpiresTo: "2024-02-29"}, {ProductID: "\x00"}, {LocationID: strings.Repeat("x", 129)}} {
		if validLotSearch(in) {
			t.Fatal("accepted", in)
		}
	}
	in := LotSearch{Status: "recorded", Quality: "not_assessed", ProductID: "x' OR 1=1 --", LocationID: "room", ExpiresFrom: "2024-02-29", ExpiresTo: "2024-02-29", Offset: MaxQuantity}
	if !validLotSearch(in) {
		t.Fatal(in)
	}
	where, args := lotSearchPredicate(identity.Scope{TenantID: "tenant", StoreID: "store"}, in)
	if strings.Contains(where, in.ProductID) || len(args) != 8 || args[3] != in.ProductID || !strings.Contains(where, "COALESCE") || !strings.Contains(where, "expires_on<>''") {
		t.Fatal(where, args)
	}
}
