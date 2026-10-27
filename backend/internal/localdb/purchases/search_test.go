package purchases

import (
	"context"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestSearchRejectsInvalidBoundsBeforeDatabaseAccess(t *testing.T) {
	for _, test := range []struct {
		filter SearchFilter
		offset int64
	}{
		{SearchFilter{}, -1}, {SearchFilter{}, MaxQuantity + 1},
		{SearchFilter{SupplierID: " supplier"}, 0}, {SearchFilter{ProductID: "bad\x00"}, 0},
		{SearchFilter{SupplierID: "bad\n"}, 0}, {SearchFilter{ProductID: strings.Repeat("x", 129)}, 0},
	} {
		_, err := Search(context.Background(), nil, identity.Scope{}, identity.DeviceContext{}, test.filter, test.offset)
		if err != ErrInvalid {
			t.Fatal(test, err)
		}
	}
}
