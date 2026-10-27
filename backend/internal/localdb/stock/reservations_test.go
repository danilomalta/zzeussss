package stock

import (
	"context"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestActiveReservationBoundsRejectBeforeOpeningDatabase(t *testing.T) {
	for _, test := range []struct {
		product, location string
		offset            int64
	}{{"", "room", 0}, {"product", "", 0}, {"product", "room", -1}, {"product", "room", MaxAvailabilityQuantity + 1}, {"product\x00", "room", 0}} {
		_, err := ActiveReservations(context.Background(), nil, identity.Scope{}, identity.DeviceContext{}, test.product, test.location, test.offset)
		if err != ErrInvalidOperation {
			t.Fatal(test, err)
		}
	}
}
