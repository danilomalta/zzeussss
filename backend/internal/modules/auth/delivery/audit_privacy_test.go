package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/security"
)

func TestLoginFailureAuditNeverCopiesClientHeadersOrBody(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })
	var recorded []byte
	// Observe the exact record sent to the existing audit sink. Abort before SQL;
	// this test does not claim persistence or invent another logging pipeline.
	if err := db.Callback().Create().Before("gorm:create").Register("p09-observe-audit", func(tx *gorm.DB) {
		if record, ok := tx.Statement.Dest.(*security.AuditLog); ok {
			recorded, _ = json.Marshal(record)
		}
		tx.AddError(errors.New("synthetic sink stop"))
	}); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	limiter := NovoLimitadorTentativasLogin(ConfiguracaoTentativasLoginPadrao())
	app.Use(limiter.Middleware())
	app.Post("/api/v1/autenticacao/login", func(c *fiber.Ctx) error { return c.SendStatus(401) })
	raw := `{"password":"p09-private-password","xml":"p09-private-xml"}`
	req := httptest.NewRequest("POST", "/api/v1/autenticacao/login", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "p09-private-token")
	req.Header.Set("X-Correlacao-Id", "00000000000")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 || len(recorded) == 0 {
		t.Fatal("audit path not exercised")
	}
	for i, v := range []string{"p09-private-password", "p09-private-xml", "p09-private-token", "00000000000"} {
		if bytes.Contains(recorded, []byte(v)) || bytes.Contains(body, []byte(v)) {
			t.Errorf("audit/HTTP leaked synthetic field %d", i)
		}
	}
	if !bytes.Contains(recorded, []byte("login_falho")) {
		t.Fatal("security event lost")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal("unexpected DB access")
	}
}
