package usecase_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/auth/delivery"
	"titansystem-backend/internal/modules/auth/usecase"
)

func TestLoginRealScenarios(t *testing.T) {
	// Configura o segredo do JWT temporariamente para o contexto de testes unitários
	os.Setenv("JWT_SECRET", "test_secret_for_auth_scenarios_unit_testing")

	// Prepara a hash Bcrypt correta para a senha de teste
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("falha ao gerar bcrypt hash para testes: %v", err)
	}

	// Inicializa o mock do banco de dados sqlmock
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("erro ao criar mock sqlmock: %v", err)
	}
	defer db.Close()

	// Conecta o GORM ao dialer mockado do Postgres
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("erro ao conectar GORM ao mock: %v", err)
	}

	// Sobrescreve a conexão global compartilhada para os testes
	database.DB = gormDB

	// Inicializa a aplicação Fiber para testes e registra a rota de login
	app := fiber.New(fiber.Config{
		ProxyHeader: "X-Forwarded-Proto",
	})
	loginUseCase := usecase.NewLoginUseCase()
	authHandler := delivery.NewAuthHandler(loginUseCase)
	app.Post("/api/v1/auth/login", authHandler.Login)

	// CENÁRIO A: Senha Incorreta (Retorna 401 Unauthorized)
	t.Run("Cenário A: Senha incorreta deve falhar e retornar 401", func(t *testing.T) {
		// Define o retorno mockado da query SELECT
		rows := sqlmock.NewRows([]string{"id", "client_id", "name", "email", "password_hash", "role", "created_at"}).
			AddRow("user-uuid-1", "client-uuid-1", "John Doe", "john@titan.com", string(hashedPassword), "admin", time.Now())

		// GORM executa uma busca por email
		mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1.*`).
			WithArgs("john@titan.com", 1).
			WillReturnRows(rows)

		// Payload com a senha errada
		body := map[string]string{
			"email":    "john@titan.com",
			"password": "wrong_password",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Valida resposta de erro amigável para evitar enumeração de usuários
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "e-mail ou senha inválidos", result["error"])
	})

	// CENÁRIO B: Credenciais Corretas (Retorna 200 OK com JWT e Cookie seguro)
	t.Run("Cenário B: Credenciais corretas deve retornar 200 e emitir JWT + Cookie", func(t *testing.T) {
		// Define o retorno mockado da query SELECT
		rows := sqlmock.NewRows([]string{"id", "client_id", "name", "email", "password_hash", "role", "created_at"}).
			AddRow("user-uuid-1", "client-uuid-1", "John Doe", "john@titan.com", string(hashedPassword), "admin", time.Now())

		mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1.*`).
			WithArgs("john@titan.com", 1).
			WillReturnRows(rows)

		// Payload com a senha correta
		body := map[string]string{
			"email":    "john@titan.com",
			"password": "correct_password",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "https://example.com/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-Proto", "https") // Habilita a detecção de HTTPS pelo ProxyHeader
		req.TLS = &tls.ConnectionState{} // Simula uma conexão TLS segura real no Fiber

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Valida que o corpo do JSON devolve o token JWT de acesso
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.NotEmpty(t, result["access_token"])
		assert.Equal(t, float64(900), result["expires_in"]) // 15 minutos em segundos

		// Valida a presença e as flags estritas do Cookie HttpOnly/Secure do Refresh Token
		cookieHeader := resp.Header.Get("Set-Cookie")
		assert.NotEmpty(t, cookieHeader)
		cookieLower := strings.ToLower(cookieHeader)
		assert.True(t, strings.Contains(cookieLower, "titan_session_rt="))
		assert.True(t, strings.Contains(cookieLower, "httponly"))
		assert.True(t, strings.Contains(cookieLower, "secure"))
		assert.True(t, strings.Contains(cookieLower, "samesite=strict"))
	})
}
