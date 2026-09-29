package security

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"titansystem-backend/internal/core/database"
)

func TestActiveSessionRejectsRevokedOrForeignContext(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	for _, tc := range []struct {
		name   string
		tenant string
		role   string
		active bool
	}{
		{"vínculo ativo", "empresa-a", "admin", true},
		{"outra empresa", "empresa-b", "admin", false},
		{"papel antigo", "empresa-a", "manager", false},
		{"empresa suspensa ou usuário ausente", "empresa-a", "admin", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("operador", tc.tenant, tc.role).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.active))
			active, err := ActiveSession("operador", tc.tenant, tc.role)
			if err != nil || active != tc.active {
				t.Fatalf("ativo=%v erro=%v; esperado=%v", active, err, tc.active)
			}
		})
	}
	if active, err := ActiveSession("", "empresa-a", "admin"); active || err != nil {
		t.Fatalf("identidade vazia: ativo=%v erro=%v", active, err)
	}
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("operador", "empresa-a", "admin").
		WillReturnError(errors.New("conexão perdida"))
	if active, err := ActiveSession("operador", "empresa-a", "admin"); active || err == nil {
		t.Fatalf("falha de conexão deve negar: ativo=%v erro=%v", active, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
