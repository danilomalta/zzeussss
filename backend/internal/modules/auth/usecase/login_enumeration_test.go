package usecase_test

import (
	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/auth/usecase"
)

func TestLoginUnknownAndWrongPasswordHaveSameFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	orm, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = orm
	defer func() { database.DB = old }()
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-account-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	var failure string
	for _, exists := range []bool{false, true} {
		rows := sqlmock.NewRows([]string{"id", "tenant_id", "name", "email", "password_hash", "role"})
		if exists {
			rows.AddRow("user", "tenant", "User", "unit@example.invalid", string(hash), "owner")
		}
		mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1.*`).WithArgs("unit@example.invalid", 1).WillReturnRows(rows)
		result, err := usecase.NewLoginUseCase().Execute(usecase.LoginInput{Email: "unit@example.invalid", Password: "incorrect-password"})
		if err == nil || result != nil {
			t.Fatal("failed login accepted")
		}
		if failure == "" {
			failure = err.Error()
		} else if err.Error() != failure {
			t.Fatal("account existence exposed")
		}
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
