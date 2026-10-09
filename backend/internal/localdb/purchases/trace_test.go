package purchases

import (
	"context"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestTraceRejectsInvalidIDsBeforeDatabaseAccess(t *testing.T) {
	for _, id := range []string{"", " order", "order ", "bad\x00", "bad\r", "bad\n", strings.Repeat("x", 129)} {
		_, err := Trace(context.Background(), nil, identity.Scope{}, identity.DeviceContext{}, id)
		if err != ErrInvalid {
			t.Fatal(id, err)
		}
	}
}
