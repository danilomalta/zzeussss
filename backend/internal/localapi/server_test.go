package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

func fixture(t *testing.T) (*sql.DB, *fiber.App, localsetup.Result) {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "server.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	result, err := localsetup.Initialize(context.Background(), db, localsetup.Input{
		TenantName: "Mercado", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Caixa", Password: "senha-forte-de-teste-1", PublicKey: public,
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(db, identity.DeviceContext{TenantID: result.TenantID, StoreID: result.StoreID, DeviceID: result.DeviceID})
	if err != nil {
		t.Fatal(err)
	}
	return db, app, result
}

func request(t *testing.T, app *fiber.App, method, path, body, token string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var responseBody bytes.Buffer
	if _, err = responseBody.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, responseBody.Bytes()
}

func loginToken(t *testing.T, app *fiber.App, ownerID string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]string{"identity_id": ownerID, "password": "senha-forte-de-teste-1"})
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, app, "POST", "/local/v1/login", string(encoded), "")
	if status != 200 {
		t.Fatalf("login: %d %s", status, body)
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Token == "" {
		t.Fatalf("token: %s %v", body, err)
	}
	return response.Token
}

func TestLocalAPIRequiresHumanSessionAndRevocation(t *testing.T) {
	db, app, owner := fixture(t)
	status, _ := request(t, app, "GET", "/local/v1/me", "", "")
	if status != 401 {
		t.Fatalf("sem sessão: %d", status)
	}
	token := loginToken(t, app, owner.OwnerID)
	status, body := request(t, app, "GET", "/local/v1/me", "", token)
	if status != 200 || !bytes.Contains(body, []byte(owner.TenantID)) {
		t.Fatalf("sessão: %d %s", status, body)
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, owner.TenantID, owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", token)
	if status != 401 {
		t.Fatalf("aparelho revogado: %d", status)
	}
}
