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
	"titansystem-backend/internal/onlinecatalog"
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
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	if onlinecatalog.Migrate(ctx, sqlDB) != nil || onlinecatalog.Migrate(ctx, sqlDB) != nil || onlinecatalog.CheckSchema(ctx, sqlDB) != nil {
		t.Fatal("migração incremental do catálogo falhou")
	}
	if onlinecatalog.MigrateCreation(ctx, sqlDB) != nil || onlinecatalog.MigrateCreation(ctx, sqlDB) != nil || onlinecatalog.CheckCreation(ctx, sqlDB) != nil {
		t.Fatal("migração do cadastro falhou")
	}
	if onlinecatalog.MigrateBatches(ctx, sqlDB) != nil || onlinecatalog.MigrateBatches(ctx, sqlDB) != nil || onlinecatalog.CheckBatches(ctx, sqlDB) != nil {
		t.Fatal("migração de lotes falhou")
	}
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
	request := func(method, path, body, access string, want int, operation ...string) map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if method == "POST" && path == "/api/v1/produtos/" {
			req.Header.Set("Idempotency-Key", uuid.NewString())
			if len(operation) > 0 {
				req.Header.Set("Idempotency-Key", operation[0])
			}
		}
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
	// Management operations use the same mounted HTTP routes and real PostgreSQL.
	path := fmt.Sprintf("/api/v1/produtos/%d", int64(id))
	snapshot := request("GET", path, "", tokenA, 200)
	if snapshot["version"] != float64(1) || snapshot["price_cents"] != float64(250) {
		t.Fatal("snapshot inicial inexato")
	}
	request("GET", path, "", tokenB, 404)
	op := uuid.NewString()
	body := fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"price_cents":375}`, op)
	first := request("POST", path+"/price", body, tokenA, 200)
	replayed := request("POST", path+"/price", body, tokenA, 200)
	if first["operation_id"] != replayed["operation_id"] {
		t.Fatal("replay mudou identidade")
	}
	request("GET", "/api/v1/catalog/operations/"+op, "", tokenA, 200)
	request("GET", "/api/v1/catalog/operations/"+op, "", tokenB, 404)
	request("POST", path+"/price", strings.Replace(body, "375", "376", 1), tokenA, 409)
	request("POST", path+"/price", strings.Replace(body, op, uuid.NewString(), 1), tokenA, 409)
	request("POST", path+"/price", strings.Replace(body, op, uuid.NewString(), 1), tokenB, 404)
	otherUser, otherSession := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO users(id,tenant_id,role) VALUES($1,$2,'owner')`, otherUser, tenantA)
	exec(`INSERT INTO online_sessions(id,tenant_id,user_id,role,created_at,expires_at) VALUES($1,$2,$3,'owner',clock_timestamp(),clock_timestamp()+interval '1 hour')`, otherSession, tenantA, otherUser)
	otherToken := token(tenantA, otherUser, otherSession)
	request("GET", "/api/v1/catalog/operations/"+op, "", otherToken, 404)
	request("POST", path+"/price", body, otherToken, 409)
	request("POST", path+"/details", fmt.Sprintf(`{"operation_id":%q,"expected_version":2,"nome":"Duplicate","descricao":"","sku":"maximum-price"}`, uuid.NewString()), tokenA, 409)
	details := fmt.Sprintf(`{"operation_id":%q,"expected_version":2,"nome":"Renamed","descricao":"Checked","sku":"renamed-sku"}`, uuid.NewString())
	request("POST", path+"/details", details, tokenA, 200)
	request("POST", path+"/active", fmt.Sprintf(`{"operation_id":%q,"expected_version":3,"ativo":false}`, uuid.NewString()), tokenA, 200)
	request("POST", path+"/active", fmt.Sprintf(`{"operation_id":%q,"expected_version":4,"ativo":true}`, uuid.NewString()), tokenA, 200)
	// Concurrent same operation: one version increment and one event.
	store := onlinecatalog.Store{DB: sqlDB}
	actor := onlinecatalog.Actor{Tenant: tenantA, User: userA, Session: sessionA, Role: "owner"}
	concurrent := onlinecatalog.Change{OperationID: uuid.NewString(), ExpectedVersion: 5, PriceCents: 401}
	changes := make(chan error, 2)
	var mutationWG sync.WaitGroup
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.Mutate(ctx, actor, int64(id), "price", concurrent)
			changes <- e
		}()
	}
	mutationWG.Wait()
	close(changes)
	for e := range changes {
		if e != nil {
			t.Fatal("replay concorrente falhou")
		}
	}
	current, e := store.Product(ctx, actor, int64(id))
	if e != nil || current.Version != 6 || current.PriceCents != 401 {
		t.Fatal("replay duplicou versão")
	}
	// Two independent operations on the same version must not overwrite each other.
	independent := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.Mutate(ctx, actor, int64(id), "price", onlinecatalog.Change{OperationID: uuid.NewString(), ExpectedVersion: 6, PriceCents: 402})
			independent <- e
		}()
	}
	mutationWG.Wait()
	close(independent)
	wins, conflicts := 0, 0
	for e := range independent {
		if e == nil {
			wins++
		} else if e == onlinecatalog.ErrConflict {
			conflicts++
		} else {
			t.Fatal("concorrência indisponível")
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("versão não protegeu disputa")
	}
	var receipts, events int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND product_id=$2`, tenantA, int64(id)).Scan(&receipts) != nil || receipts != 6 {
		t.Fatal("histórico duplicado")
	}
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_outbox WHERE tenant_id=$1`, tenantA).Scan(&events) != nil || events != receipts {
		t.Fatal("evento fora da transação")
	}
	failEvent := uuid.NewString()
	exec(fmt.Sprintf(`CREATE FUNCTION reject_catalog_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_catalog_event BEFORE INSERT ON online_catalog_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_catalog_event()`, failEvent))
	request("POST", path+"/price", fmt.Sprintf(`{"operation_id":%q,"expected_version":7,"price_cents":999999999999}`, failEvent), tokenA, 503)
	request("GET", "/api/v1/catalog/operations/"+failEvent, "", tokenA, 404)
	current, e = store.Product(ctx, actor, int64(id))
	if e != nil || current.Version != 7 || current.PriceCents != 402 {
		t.Fatal("falha de evento gravou alteração")
	}
	history := request("GET", path+"/history?limit=2&offset=0", "", tokenA, 200)
	if len(history["items"].([]interface{})) != 2 {
		t.Fatal("histórico não paginado")
	}
	// Audit failure must roll back price/version and leave no outbox intent.
	exec(`CREATE FUNCTION reject_catalog_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_catalog_audit BEFORE INSERT ON online_catalog_operations FOR EACH ROW WHEN (NEW.action='details') EXECUTE FUNCTION reject_catalog_audit()`)
	request("POST", path+"/details", fmt.Sprintf(`{"operation_id":%q,"expected_version":7,"nome":"Must rollback","descricao":"","sku":"must-rollback"}`, uuid.NewString()), tokenA, 503)
	current, e = store.Product(ctx, actor, int64(id))
	if e != nil || current.Version != 7 || current.Name != "Renamed" {
		t.Fatal("falha de auditoria gravou produto")
	}
	// Authorization is checked again under row locks inside the transaction.
	stockUser, stockSession := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO users(id,tenant_id,role) VALUES($1,$2,'stock')`, stockUser, tenantA)
	exec(`INSERT INTO online_sessions(id,tenant_id,user_id,role,created_at,expires_at) VALUES($1,$2,$3,'stock',clock_timestamp(),clock_timestamp()+interval '1 hour')`, stockSession, tenantA, stockUser)
	stockActor := onlinecatalog.Actor{Tenant: tenantA, User: stockUser, Session: stockSession, Role: "stock"}
	if _, e = store.Mutate(ctx, stockActor, int64(id), "price", onlinecatalog.Change{OperationID: uuid.NewString(), ExpectedVersion: 7, PriceCents: 1}); e != onlinecatalog.ErrDenied {
		t.Fatal("stock alterou preço")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, stockSession)
	if _, e = store.Mutate(ctx, stockActor, int64(id), "details", onlinecatalog.Change{OperationID: uuid.NewString(), ExpectedVersion: 7, Name: "Invalid", SKU: "invalid"}); e != onlinecatalog.ErrDenied {
		t.Fatal("revogação não bloqueou transação")
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
	// Creation replay/concurrency and search use the newly protected legacy route.
	creationOp := uuid.NewString()
	creationBody := `{"nome":"Search 50%_item","sku":"repeat-sku","preco":0.01,"estoque":3}`
	createdProduct := request("POST", "/api/v1/produtos/", creationBody, tokenA, 201, creationOp)
	createdAgain := request("POST", "/api/v1/produtos/", creationBody, tokenA, 201, creationOp)
	if createdProduct["ID"] != createdAgain["ID"] || createdProduct["price_cents"] != float64(1) {
		t.Fatal("cadastro repetido ou preço inexato")
	}
	request("POST", "/api/v1/produtos/", strings.Replace(creationBody, "0.01", "0.02", 1), tokenA, 409, creationOp)
	request("POST", "/api/v1/produtos/", creationBody, otherToken, 409, creationOp)
	request("POST", "/api/v1/produtos/", creationBody, tokenA, 428, "")
	request("GET", "/api/v1/catalog/creations/"+creationOp, "", tokenA, 200)
	request("GET", "/api/v1/catalog/creations/"+creationOp, "", otherToken, 404)
	request("GET", "/api/v1/catalog/creations/"+creationOp, "", tokenB, 401)
	request("GET", fmt.Sprintf("/api/v1/produtos/%d/creation", int64(createdProduct["ID"].(float64))), "", tokenA, 200)
	search := request("GET", "/api/v1/produtos/search?q=50%25_item&active=active&limit=2", "", tokenA, 200)
	if len(search["items"].([]interface{})) != 1 {
		t.Fatal("busca literal não isolou produto")
	}
	empty := request("GET", "/api/v1/produtos/search?q=not-found&active=inactive", "", tokenA, 200)
	if len(empty["items"].([]interface{})) != 0 {
		t.Fatal("estado vazio fabricado")
	}
	request("GET", "/api/v1/produtos/search?q=a&q=b", "", tokenA, 400)
	beforeCount := count("products", tenantA)
	newInput := onlinecatalog.NewProduct{Name: "Concurrent", SKU: "concurrent-create", PriceCents: onlinecatalog.MaxPrice}
	newOp := uuid.NewString()
	createResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() { defer mutationWG.Done(); _, e := store.Create(ctx, actor, newOp, newInput); createResults <- e }()
	}
	mutationWG.Wait()
	close(createResults)
	for e := range createResults {
		if e != nil {
			t.Fatal("cadastro concorrente falhou")
		}
	}
	if count("products", tenantA) != beforeCount+1 {
		t.Fatal("cadastro concorrente duplicou produto")
	}
	failedCreation := uuid.NewString()
	exec(fmt.Sprintf(`CREATE FUNCTION reject_creation_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_creation_event BEFORE INSERT ON online_catalog_creation_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_creation_event()`, failedCreation))
	request("POST", "/api/v1/produtos/", `{"nome":"Rollback","sku":"creation-rollback"}`, tokenA, 503, failedCreation)
	request("GET", "/api/v1/catalog/creations/"+failedCreation, "", tokenA, 404)
	if count("products", tenantA) != beforeCount+1 {
		t.Fatal("outbox não reverteu cadastro")
	}
	// Batch HTTP flow and the same SQL transaction protect two different items.
	p1 := request("POST", "/api/v1/produtos/", `{"nome":"Batch one","sku":"batch-one","preco":2.50}`, tokenA, 201)
	p2 := request("POST", "/api/v1/produtos/", `{"nome":"Batch two","sku":"batch-two","preco":3.50}`, tokenA, 201)
	id1, id2 := int64(p1["ID"].(float64)), int64(p2["ID"].(float64))
	batchOp := uuid.NewString()
	batchBody := fmt.Sprintf(`{"operation_id":%q,"items":[{"product_id":%d,"action":"active","expected_version":1,"ativo":false},{"product_id":%d,"action":"price","expected_version":1,"price_cents":475}]}`, batchOp, id2, id1)
	preview := request("POST", "/api/v1/catalog/batches/preview", batchBody, tokenA, 200)
	checkProduct := func(productID, version, price int64, active bool) {
		t.Helper()
		p, e := store.Product(ctx, actor, productID)
		if e != nil || p.Version != version || p.PriceCents != price || p.Active != active {
			t.Fatal("lote alterou snapshot inesperadamente")
		}
	}
	checkProduct(id1, 1, 250, true)
	checkProduct(id2, 1, 350, true)
	applyBody := strings.TrimSuffix(batchBody, "}") + fmt.Sprintf(`,"preview_hash":%q}`, preview["preview_hash"])
	request("POST", "/api/v1/catalog/batches/apply", batchBody, tokenA, 400)
	batchResult := request("POST", "/api/v1/catalog/batches/apply", applyBody, tokenA, 200)
	if len(batchResult["items"].([]interface{})) != 2 {
		t.Fatal("lote sem todos os recibos")
	}
	request("POST", "/api/v1/catalog/batches/apply", applyBody, tokenA, 200)
	request("POST", "/api/v1/catalog/batches/apply", strings.Replace(applyBody, "475", "476", 1), tokenA, 409)
	request("POST", "/api/v1/catalog/batches/apply", applyBody, otherToken, 409)
	request("GET", "/api/v1/catalog/batches/"+batchOp, "", tokenA, 200)
	request("GET", "/api/v1/catalog/batches/"+batchOp, "", otherToken, 404)
	historyBatch := request("GET", "/api/v1/catalog/batches?limit=1&offset=0", "", tokenA, 200)
	if len(historyBatch["items"].([]interface{})) != 1 {
		t.Fatal("histórico de lote não paginado")
	}
	checkProduct(id1, 2, 475, true)
	checkProduct(id2, 2, 350, false)
	// A later individual change makes the original preview stale; nothing else changes.
	staleOp := uuid.NewString()
	staleBody := fmt.Sprintf(`{"operation_id":%q,"items":[{"product_id":%d,"action":"price","expected_version":2,"price_cents":500},{"product_id":%d,"action":"active","expected_version":2,"ativo":true}]}`, staleOp, id1, id2)
	stalePreview := request("POST", "/api/v1/catalog/batches/preview", staleBody, tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", id1), fmt.Sprintf(`{"operation_id":%q,"expected_version":2,"price_cents":476}`, uuid.NewString()), tokenA, 200)
	request("POST", "/api/v1/catalog/batches/apply", strings.TrimSuffix(staleBody, "}")+fmt.Sprintf(`,"preview_hash":%q}`, stalePreview["preview_hash"]), tokenA, 409)
	checkProduct(id2, 2, 350, false)
	// Fault on the SECOND item's outbox must roll back both versions and all receipts.
	failureBatch := uuid.NewString()
	failureChild := uuid.NewSHA1(uuid.MustParse(failureBatch), []byte(fmt.Sprintf("catalog-batch:%d", id2))).String()
	exec(fmt.Sprintf(`CREATE FUNCTION reject_batch_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_batch_event BEFORE INSERT ON online_catalog_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_batch_event()`, failureChild))
	failureBody := fmt.Sprintf(`{"operation_id":%q,"items":[{"product_id":%d,"action":"price","expected_version":3,"price_cents":500},{"product_id":%d,"action":"active","expected_version":2,"ativo":true}]}`, failureBatch, id1, id2)
	failurePreview := request("POST", "/api/v1/catalog/batches/preview", failureBody, tokenA, 200)
	request("POST", "/api/v1/catalog/batches/apply", strings.TrimSuffix(failureBody, "}")+fmt.Sprintf(`,"preview_hash":%q}`, failurePreview["preview_hash"]), tokenA, 503)
	request("GET", "/api/v1/catalog/batches/"+failureBatch, "", tokenA, 404)
	checkProduct(id1, 3, 476, true)
	checkProduct(id2, 2, 350, false)
	var rolledBackChildren int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND operation_id=$2`, tenantA, failureChild).Scan(&rolledBackChildren) != nil || rolledBackChildren != 0 {
		t.Fatal("lote parcial deixou auditoria")
	}
	// Same request concurrently replays once, even with products submitted out of order.
	price500 := int64(500)
	activeTrue := true
	concurrentBatch := onlinecatalog.BatchInput{OperationID: uuid.NewString(), Items: []onlinecatalog.BatchItem{{ProductID: id2, Action: "active", ExpectedVersion: 2, Active: &activeTrue}, {ProductID: id1, Action: "price", ExpectedVersion: 3, PriceCents: &price500}}}
	concurrentPreview, e := store.PreviewBatch(ctx, actor, concurrentBatch)
	if e != nil {
		t.Fatal("prévia concorrente falhou")
	}
	concurrentBatch.PreviewHash = concurrentPreview.PreviewHash
	batchErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.ApplyBatch(ctx, actor, concurrentBatch)
			batchErrors <- e
		}()
	}
	mutationWG.Wait()
	close(batchErrors)
	for e := range batchErrors {
		if e != nil {
			t.Fatal("replay concorrente do lote falhou")
		}
	}
	checkProduct(id1, 4, 500, true)
	checkProduct(id2, 3, 350, true)
	// Independent batches competing for the same versions cannot partly apply.
	independentBatches := make([]onlinecatalog.BatchInput, 2)
	for i := range independentBatches {
		p := int64(600 + i)
		activeFalse := false
		independentBatches[i] = onlinecatalog.BatchInput{OperationID: uuid.NewString(), Items: []onlinecatalog.BatchItem{{ProductID: id1, Action: "price", ExpectedVersion: 4, PriceCents: &p}, {ProductID: id2, Action: "active", ExpectedVersion: 3, Active: &activeFalse}}}
		preview, e := store.PreviewBatch(ctx, actor, independentBatches[i])
		if e != nil {
			t.Fatal("prévia de disputa falhou")
		}
		independentBatches[i].PreviewHash = preview.PreviewHash
	}
	competition := make(chan error, 2)
	for _, b := range independentBatches {
		mutationWG.Add(1)
		go func(b onlinecatalog.BatchInput) {
			defer mutationWG.Done()
			_, e := store.ApplyBatch(ctx, actor, b)
			competition <- e
		}(b)
	}
	mutationWG.Wait()
	close(competition)
	wins, conflicts = 0, 0
	for e := range competition {
		if e == nil {
			wins++
		} else if e == onlinecatalog.ErrConflict {
			conflicts++
		} else {
			t.Fatal("disputa de lotes falhou")
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("disputa de lotes publicou resultados parciais")
	}
	pAfter, e := store.Product(ctx, actor, id1)
	if e != nil || pAfter.Version != 5 || (pAfter.PriceCents != 600 && pAfter.PriceCents != 601) {
		t.Fatal("primeiro item não protegeu disputa")
	}
	checkProduct(id2, 4, 350, false)
	// Tenant and live session checks also apply inside the batch service.
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionB)
	request("POST", "/api/v1/catalog/batches/preview", batchBody, tokenB, 404)
	if _, e := store.PreviewBatch(ctx, stockActor, concurrentBatch); e != onlinecatalog.ErrDenied {
		t.Fatal("stock criou prévia de preço")
	}
	var batchChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_batch_migrations WHERE version=10`).Scan(&batchChecksum) != nil {
		t.Fatal("checksum de lote ausente")
	}
	exec(`UPDATE online_catalog_batch_migrations SET version=11 WHERE version=10`)
	if onlinecatalog.MigrateBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("versão futura de lote aceita")
	}
	exec(`UPDATE online_catalog_batch_migrations SET version=10,checksum='tampered' WHERE version=11`)
	if onlinecatalog.MigrateBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("checksum de lote adulterado aceito")
	}
	exec(`UPDATE online_catalog_batch_migrations SET checksum=$1 WHERE version=10`, batchChecksum)
	var catalogChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_migrations WHERE version=8`).Scan(&catalogChecksum) != nil {
		t.Fatal("checksum ausente")
	}
	exec(`UPDATE online_catalog_migrations SET checksum='tampered' WHERE version=8`)
	if onlinecatalog.CheckSchema(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.Migrate(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("histórico adulterado aceito")
	}
	exec(`UPDATE online_catalog_migrations SET checksum=$1,version=9 WHERE version=8`, catalogChecksum)
	if onlinecatalog.CheckSchema(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.Migrate(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("histórico futuro aceito")
	}
	exec(`UPDATE online_catalog_migrations SET version=8 WHERE version=9`)
	if onlinecatalog.CheckSchema(ctx, sqlDB) != nil {
		t.Fatal("fixture restaurada incompatível")
	}
	// Schema and data remain for inspection. No DROP, TRUNCATE or cleanup SQL.
}
