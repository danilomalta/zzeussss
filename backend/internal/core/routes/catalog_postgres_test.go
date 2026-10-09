package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/db/migrations"
	"titansystem-backend/internal/core/database"
	posUsecase "titansystem-backend/internal/modules/pos/usecase"
)

// Opt-in only. Schema, products and identities are disposable fixtures.
// No .env, application database or historical initial migration is used.
func TestPostgresCatalogCreationAndDiscountAtomicity(t *testing.T) {
	raw := os.Getenv("TITAN_CATALOG_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("PostgreSQL isolado não configurado")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("URL de teste inválida")
	}
	host := cfg.ConnConfig.Host
	ip := net.ParseIP(host)
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		t.Fatal("teste exige banco *_test em loopback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("pool de teste indisponível")
	}
	defer pool.Close()
	schema := "titan_catalog_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("schema isolado indisponível")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("pool isolado indisponível")
	}
	defer isolated.Close()
	sqlDB := stdlib.OpenDBFromPool(isolated)
	defer sqlDB.Close()
	exec := func(statement string, args ...interface{}) {
		t.Helper()
		if _, e := sqlDB.ExecContext(ctx, statement, args...); e != nil {
			t.Fatal("fixture SQL falhou")
		}
	}
	exec(`CREATE TABLE tenants(id UUID PRIMARY KEY,status TEXT NOT NULL); CREATE TABLE users(id UUID PRIMARY KEY,tenant_id UUID NOT NULL REFERENCES tenants(id),role TEXT NOT NULL)`)
	exec(migrations.OnlineSessionsSQL)
	// The original 000003 qualifies public; route all its references to this new
	// schema, never to the application's public schema. DDL structure is unchanged.
	exec(migrations.CatalogProductsSQL)
	exec(strings.ReplaceAll(migrations.CatalogDiscountsSQL, "public.", ""))
	tenantA, tenantB, userA, userB, sessionA, sessionB := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, actor := range []struct{ tenant, user, session string }{{tenantA, userA, sessionA}, {tenantB, userB, sessionB}} {
		exec(`INSERT INTO tenants(id,status) VALUES($1,'active')`, actor.tenant)
		exec(`INSERT INTO users(id,tenant_id,role) VALUES($1,$2,'owner')`, actor.user, actor.tenant)
		exec(`INSERT INTO online_sessions(id,tenant_id,user_id,role,created_at,expires_at) VALUES($1,$2,$3,'owner',clock_timestamp(),clock_timestamp()+interval '1 hour')`, actor.session, actor.tenant, actor.user)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("GORM de teste indisponível")
	}
	previous := database.DB
	database.DB = db
	defer func() { database.DB = previous }()
	t.Setenv("JWT_SECRET", "catalog-fixture-only-secret")
	token := func(tenant, user, session string) string {
		signed, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "tenant_id": tenant, "role": "owner", "sid": session, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("catalog-fixture-only-secret"))
		if e != nil {
			t.Fatal("token de fixture indisponível")
		}
		return signed
	}
	tokenA, tokenB := token(tenantA, userA, sessionA), token(tenantB, userB, sessionB)
	app := fiber.New()
	Registrar(app)
	request := func(method, path, body, access string, want int) map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+access)
		resp, e := app.Test(req, 10000)
		if e != nil {
			t.Fatal("requisição de teste falhou")
		}
		defer resp.Body.Close()
		data, e := io.ReadAll(resp.Body)
		if e != nil {
			t.Fatal("leitura da resposta falhou")
		}
		if resp.StatusCode != want {
			t.Fatalf("status recebido %d esperado %d", resp.StatusCode, want)
		}
		var result map[string]interface{}
		if json.Unmarshal(data, &result) != nil {
			t.Fatal("resposta JSON inválida")
		}
		return result
	}
	count := func(table, tenant string) int {
		t.Helper()
		if table != "products" && table != "discount_suggestions" {
			t.Fatal("fixture de contagem inválida")
		}
		var n int
		if sqlDB.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE tenant_id=$1", tenant).Scan(&n) != nil {
			t.Fatal("contagem de fixture falhou")
		}
		return n
	}
	product := request("POST", "/api/v1/produtos/", `{"nome":"Item","sku":"same-sku","preco":2.50,"estoque":100}`, tokenA, 201)
	id, ok := product["ID"].(float64)
	if !ok || id <= 0 {
		t.Fatal("cadastro sem ID real")
	}
	var price string
	if sqlDB.QueryRowContext(ctx, `SELECT preco::text FROM products WHERE id=$1 AND tenant_id=$2`, int64(id), tenantA).Scan(&price) != nil || price != "2.50" {
		t.Fatal("preço decimal não persistiu")
	}
	if _, exists := product["tenant_id"]; exists {
		t.Fatal("resposta expôs tenant_id")
	}
	request("POST", "/api/v1/produtos/", `{"nome":"Item","sku":"same-sku","preco":2.50}`, tokenA, 409)
	request("POST", "/api/v1/produtos/", `{"nome":"Item","sku":"same-sku","preco":2.50}`, tokenB, 201)
	request("POST", "/api/v1/produtos/", `{"nome":"Item","sku":"bad-price","preco":0.001}`, tokenA, 400)
	if count("products", tenantA) != 1 || count("products", tenantB) != 1 {
		t.Fatal("unicidade ou validação não protegeu cadastro")
	}
	maximum := request("POST", "/api/v1/produtos/", `{"nome":"Maximum","sku":"maximum-price","preco":9999999999.99}`, tokenA, 201)
	maximumID, ok := maximum["ID"].(float64)
	if !ok || maximumID <= 0 {
		t.Fatal("preço máximo sem cadastro")
	}
	if sqlDB.QueryRowContext(ctx, `SELECT preco::text FROM products WHERE id=$1 AND tenant_id=$2`, int64(maximumID), tenantA).Scan(&price) != nil || price != "9999999999.99" {
		t.Fatal("limite decimal não persistiu exatamente")
	}
	// Run the real transaction and partial unique index concurrently.
	results := make(chan struct {
		created int
		failed  bool
	}, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			records, e := posUsecase.RunDiscountEngine(tenantA)
			results <- struct {
				created int
				failed  bool
			}{len(records), e != nil}
		}()
	}
	wg.Wait()
	close(results)
	created := 0
	for result := range results {
		if result.failed {
			t.Fatal("geração concorrente falhou")
		}
		created += result.created
	}
	if created != 1 || count("discount_suggestions", tenantA) != 1 || count("discount_suggestions", tenantB) != 0 {
		t.Fatal("índice pendente ou isolamento falhou")
	}
	replay := request("POST", "/api/v1/discounts/suggest", "", tokenA, 200)
	if replay["items_gerados"] != float64(0) {
		t.Fatal("repetição contou conflito como criação")
	}
	// Cross-company FK must fail in PostgreSQL, independently of HTTP guards.
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO discount_suggestions(tenant_id,product_id,suggested_discount,suggested_range,reason,criteria) VALUES($1,$2,15,'test','test','test')`, tenantB, int64(id))
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23503" {
		t.Fatal("FK composta não rejeitou contexto estrangeiro")
	}
	request("POST", "/api/v1/produtos/", `{"nome":"First","sku":"rollback-first","preco":1,"estoque":100}`, tokenA, 201)
	last := request("POST", "/api/v1/produtos/", `{"nome":"Last","sku":"rollback-last","preco":1,"estoque":100}`, tokenA, 201)
	failID, ok := last["ID"].(float64)
	if !ok || failID <= id {
		t.Fatal("ordem de fixture inválida")
	}
	exec(fmt.Sprintf(`CREATE FUNCTION reject_catalog_suggestion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_catalog_suggestion BEFORE INSERT ON discount_suggestions FOR EACH ROW WHEN (NEW.product_id=%d) EXECUTE FUNCTION reject_catalog_suggestion()`, int64(failID)))
	request("POST", "/api/v1/discounts/suggest", "", tokenA, 500)
	if count("discount_suggestions", tenantA) != 1 {
		t.Fatal("erro após primeiro INSERT não reverteu lote")
	}
	exec(`INSERT INTO products(tenant_id,nome,sku,estoque) SELECT $1,'bulk','bulk-'||i,0 FROM generate_series(1,1000) AS i`, tenantB)
	request("POST", "/api/v1/discounts/suggest", "", tokenB, 409)
	if count("discount_suggestions", tenantB) != 0 {
		t.Fatal("limite publicou lote parcial")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionB)
	request("POST", "/api/v1/produtos/", `{"nome":"Revoked","sku":"revoked"}`, tokenB, 401)
	if count("products", tenantB) != 1001 {
		t.Fatal("sessão revogada gravou produto")
	}
	// Schema and data remain for inspection. No DROP, TRUNCATE or cleanup SQL.
}
