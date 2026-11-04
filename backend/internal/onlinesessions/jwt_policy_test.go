package onlinesessions

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInadequateJWTKeyRefusesCreateAndRefreshBeforeDatabase(t *testing.T) {
	for i, key := range []string{"short", strings.Repeat("x", 64), strings.Repeat("A", 43), jwtFixtureKey + "="} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal("fixture database failed")
		}
		s := New(db, key)
		pair, err := s.Create(context.Background(), Scope{User: testUser, Tenant: testTenant, Role: "owner"}, "fixture hash")
		if err != ErrUnavailable || pair.Access != "" || pair.Refresh != "" || strings.Contains(err.Error(), key) {
			t.Fatalf("create case %d accepted or disclosed", i)
		}
		refresh, err := refreshSecret(testID)
		if err != nil {
			t.Fatal("fixture refresh failed")
		}
		pair, err = s.Rotate(context.Background(), refresh)
		if err != ErrUnavailable || pair.Access != "" || pair.Refresh != "" || strings.Contains(err.Error(), key) {
			t.Fatalf("refresh case %d accepted or disclosed", i)
		}
		if err = mock.ExpectationsWereMet(); err != nil {
			t.Fatal("database reached with invalid signing key")
		}
		db.Close()
	}
}
