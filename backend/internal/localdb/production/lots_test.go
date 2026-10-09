package production

import (
	"strings"
	"testing"
)

func TestProductionLotCalendarAndMetadataBounds(t *testing.T) {
	for _, test := range []struct {
		code, made, expiry string
		valid              bool
	}{
		{"Lote A", "2024-02-29", "2024-02-29", true},
		{"Lote A", "2024-02-29", "", true},
		{"Lote A", "2023-02-29", "", false},
		{"Lote A", "2024-02-30", "2024-03-01", false},
		{"Lote A", "2024-02-29", "2024-02-28", false},
		{"Lote A", "0000-01-01", "", false},
		{"Lote A", "2024-2-29", "", false},
		{"Lote A", "2024-02-29T00:00:00Z", "", false},
		{" A", "2024-02-29", "", false},
		{"A\nB", "2024-02-29", "", false},
		{strings.Repeat("á", 32), "2024-02-29", "", true},
		{strings.Repeat("á", 33), "2024-02-29", "", false},
	} {
		if got := validLotMetadata(test.code, test.made, test.expiry); got != test.valid {
			t.Fatalf("%+v got=%t", test, got)
		}
	}
}
