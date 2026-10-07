package security

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"titansystem-backend/internal/core/database"
)

func TestAccessSessionRequiresActiveFamilyMembershipAndExpiry(t *testing.T) {
	conn, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })
	id := "11111111-1111-4111-8111-111111111112"
	for _, active := range []bool{true, false} {
		m.ExpectQuery(`(?s)SELECT EXISTS.*s.id=\$4.*s.role=u.role.*t.status='active'.*s.revoked_at IS NULL.*s.expires_at > clock_timestamp`).WithArgs("user", "tenant", "owner", id).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(active))
		ok, err := ActiveOnlineSession("user", "tenant", "owner", id)
		if err != nil || ok != active {
			t.Fatal("barreira de sessão incorreta")
		}
	}
	if ok, err := ActiveOnlineSession("user", "tenant", "owner", ""); ok || err != nil {
		t.Fatal("sessão antiga aceita")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
