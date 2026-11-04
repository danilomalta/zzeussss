package onlinesessions

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestStoreRefusesUnsafeSigningKeyBeforeTransaction(t *testing.T) {
	for i, key := range []string{"", " \t", "super_secret_key_change_in_prod", "titanpass"} {
		db, m, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		s := New(db, key)
		pair, err := s.Create(context.Background(), Scope{User: testUser, Tenant: testTenant, Role: "owner"}, "fixture password hash")
		if err != ErrUnavailable || pair.Access != "" || pair.Refresh != "" {
			t.Fatalf("unsafe store case %d accepted", i)
		}
		if err = m.ExpectationsWereMet(); err != nil {
			t.Fatal("database reached before configuration refusal")
		}
		db.Close()
	}
}
