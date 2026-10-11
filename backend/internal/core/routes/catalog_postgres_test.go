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
	if onlinecatalog.MigrateUndo(ctx, sqlDB) != nil || onlinecatalog.MigrateUndo(ctx, sqlDB) != nil || onlinecatalog.CheckUndo(ctx, sqlDB) != nil {
		t.Fatal("migração de reversão falhou")
	}
	if onlinecatalog.MigrateImports(ctx, sqlDB) != nil || onlinecatalog.MigrateImports(ctx, sqlDB) != nil || onlinecatalog.CheckImports(ctx, sqlDB) != nil {
		t.Fatal("migração CSV falhou")
	}
	if onlinecatalog.MigrateAdjustments(ctx, sqlDB) != nil || onlinecatalog.MigrateAdjustments(ctx, sqlDB) != nil || onlinecatalog.CheckAdjustments(ctx, sqlDB) != nil {
		t.Fatal("migração reajuste falhou")
	}
	if onlinecatalog.MigrateBarcodes(ctx, sqlDB) != nil || onlinecatalog.MigrateBarcodes(ctx, sqlDB) != nil || onlinecatalog.CheckBarcodes(ctx, sqlDB) != nil {
		t.Fatal("migração de códigos falhou")
	}
	if onlinecatalog.MigrateBarcodeBatches(ctx, sqlDB) != nil || onlinecatalog.MigrateBarcodeBatches(ctx, sqlDB) != nil || onlinecatalog.CheckBarcodeBatches(ctx, sqlDB) != nil {
		t.Fatal("migração lote de códigos falhou")
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
	// Forced audit failure verified; restore normal execution for later scenarios.
	exec(`ALTER TABLE online_catalog_operations DISABLE TRIGGER reject_catalog_audit`)
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
	// Compensation restores values atomically while versions and history advance.
	u1 := request("POST", "/api/v1/produtos/", `{"nome":"Undo one","sku":"undo-one","preco":2.50}`, tokenA, 201)
	u2 := request("POST", "/api/v1/produtos/", `{"nome":"Undo two","sku":"undo-two","preco":3.50}`, tokenA, 201)
	uid1, uid2 := int64(u1["ID"].(float64)), int64(u2["ID"].(float64))
	makeUndoSource := func(version int64, price int64) string {
		t.Helper()
		op := uuid.NewString()
		body := fmt.Sprintf(`{"operation_id":%q,"items":[{"product_id":%d,"action":"price","expected_version":%d,"price_cents":%d},{"product_id":%d,"action":"active","expected_version":%d,"ativo":false}]}`, op, uid1, version, price, uid2, version)
		p := request("POST", "/api/v1/catalog/batches/preview", body, tokenA, 200)
		request("POST", "/api/v1/catalog/batches/apply", strings.TrimSuffix(body, "}")+fmt.Sprintf(`,"preview_hash":%q}`, p["preview_hash"]), tokenA, 200)
		return op
	}
	originalUndoBatch := makeUndoSource(1, 375)
	undoOp := uuid.NewString()
	undoBody := fmt.Sprintf(`{"operation_id":%q,"source_operation_id":%q,"reason":"Corrige reajuste"}`, undoOp, originalUndoBatch)
	undoPreviewHTTP := request("POST", "/api/v1/catalog/undos/preview", undoBody, otherToken, 200)
	checkProduct(uid1, 2, 375, true)
	checkProduct(uid2, 2, 350, false)
	undoApplyBody := strings.TrimSuffix(undoBody, "}") + fmt.Sprintf(`,"preview_hash":%q}`, undoPreviewHTTP["preview_hash"])
	request("POST", "/api/v1/catalog/undos/apply", undoApplyBody, otherToken, 200)
	request("POST", "/api/v1/catalog/undos/apply", undoApplyBody, otherToken, 200)
	request("POST", "/api/v1/catalog/undos/apply", strings.Replace(undoApplyBody, "Corrige reajuste", "Outro motivo", 1), otherToken, 409)
	request("GET", "/api/v1/catalog/undos/"+undoOp, "", otherToken, 200)
	request("GET", "/api/v1/catalog/undos/"+undoOp, "", tokenA, 404)
	undoStatus := request("GET", "/api/v1/catalog/batches/"+originalUndoBatch+"/undo", "", tokenA, 200)
	if undoStatus["source_operation_id"] != originalUndoBatch || undoStatus["operation_id"] != undoOp {
		t.Fatal("situação da reversão não vinculou original")
	}
	request("GET", "/api/v1/catalog/batches/"+originalUndoBatch, "", tokenA, 200)
	checkProduct(uid1, 3, 250, true)
	checkProduct(uid2, 3, 350, true)
	request("POST", "/api/v1/catalog/undos/preview", strings.Replace(undoBody, undoOp, uuid.NewString(), 1), tokenA, 409)
	chain := fmt.Sprintf(`{"operation_id":%q,"source_operation_id":%q,"reason":"chain"}`, uuid.NewString(), undoOp)
	request("POST", "/api/v1/catalog/undos/preview", chain, otherToken, 409)
	request("POST", "/api/v1/catalog/undos/preview", undoBody, tokenB, 404)
	// Changes after a source batch refuse compensation rather than overwriting them.
	request("POST", "/api/v1/catalog/undos/preview", fmt.Sprintf(`{"operation_id":%q,"source_operation_id":%q,"reason":"stale"}`, uuid.NewString(), batchOp), tokenA, 409)
	lateSource := makeUndoSource(3, 425)
	lateUndoOp := uuid.NewString()
	lateBody := fmt.Sprintf(`{"operation_id":%q,"source_operation_id":%q,"reason":"Late rollback"}`, lateUndoOp, lateSource)
	latePreview := request("POST", "/api/v1/catalog/undos/preview", lateBody, tokenA, 200)
	exec(fmt.Sprintf(`CREATE FUNCTION reject_undo_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_undo_event BEFORE INSERT ON online_catalog_undo_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_undo_event()`, lateUndoOp))
	request("POST", "/api/v1/catalog/undos/apply", strings.TrimSuffix(lateBody, "}")+fmt.Sprintf(`,"preview_hash":%q}`, latePreview["preview_hash"]), tokenA, 503)
	request("GET", "/api/v1/catalog/undos/"+lateUndoOp, "", tokenA, 404)
	request("GET", "/api/v1/catalog/batches/"+lateUndoOp, "", tokenA, 404)
	request("GET", "/api/v1/catalog/batches/"+lateSource+"/undo", "", tokenA, 404)
	checkProduct(uid1, 4, 425, true)
	checkProduct(uid2, 4, 350, false)
	failedChild1 := uuid.NewSHA1(uuid.MustParse(lateUndoOp), []byte(fmt.Sprintf("catalog-batch:%d", uid1))).String()
	failedChild2 := uuid.NewSHA1(uuid.MustParse(lateUndoOp), []byte(fmt.Sprintf("catalog-batch:%d", uid2))).String()
	var failedUndoAuditCount int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND operation_id IN ($2,$3)`, tenantA, failedChild1, failedChild2).Scan(&failedUndoAuditCount) != nil || failedUndoAuditCount != 0 {
		t.Fatal("reversão falha deixou recibos dos itens")
	}
	// Different reversal IDs racing for the same original can win only once.
	raceUndos := make([]onlinecatalog.UndoInput, 2)
	for i := range raceUndos {
		u := onlinecatalog.UndoInput{OperationID: uuid.NewString(), SourceOperationID: lateSource, Reason: "Authorized reversal"}
		p, e := store.PreviewUndo(ctx, actor, u)
		if e != nil {
			t.Fatal("prévia de reversão concorrente falhou")
		}
		u.PreviewHash = p.PreviewHash
		raceUndos[i] = u
	}
	undoErrors := make(chan error, 2)
	for _, u := range raceUndos {
		mutationWG.Add(1)
		go func(u onlinecatalog.UndoInput) {
			defer mutationWG.Done()
			_, e := store.ApplyUndo(ctx, actor, u)
			undoErrors <- e
		}(u)
	}
	mutationWG.Wait()
	close(undoErrors)
	wins, conflicts = 0, 0
	for e := range undoErrors {
		if e == nil {
			wins++
		} else if e == onlinecatalog.ErrConflict {
			conflicts++
		} else {
			t.Fatal("concorrência da reversão falhou")
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("reversão duplicou original")
	}
	checkProduct(uid1, 5, 250, true)
	checkProduct(uid2, 5, 350, true)
	var undoCount int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_undos WHERE tenant_id=$1 AND source_operation_id=$2`, tenantA, lateSource).Scan(&undoCount) != nil || undoCount != 1 {
		t.Fatal("reversão não foi única")
	}
	// Fresh HTTP fixture for this phase, keeping the real 100/min limiter enabled.
	app = fiber.New()
	Registrar(app)
	csvProduct1 := request("POST", "/api/v1/produtos/", `{"nome":"Import price","sku":"import-price","preco":2.50}`, tokenA, 201)
	csvProduct2 := request("POST", "/api/v1/produtos/", `{"nome":"Import active","sku":"import-active","preco":3.50}`, tokenA, 201)
	csvID1, csvID2 := int64(csvProduct1["ID"].(float64)), int64(csvProduct2["ID"].(float64))
	request("POST", "/api/v1/produtos/", `{"nome":"Other company","sku":"import-price","preco":9.99}`, tokenB, 201)
	csvOp := uuid.NewString()
	csvText := "sku;expected_version;preco;ativo\nimport-active;1;;false\nimport-price;1;4,75;\n"
	csvInput := onlinecatalog.ImportInput{OperationID: csvOp, Reason: "Planilha conferida", CSV: csvText}
	csvJSON, _ := json.Marshal(csvInput)
	csvPreview := request("POST", "/api/v1/catalog/imports/preview", string(csvJSON), tokenA, 200)
	checkProduct(csvID1, 1, 250, true)
	checkProduct(csvID2, 1, 350, true)
	if csvPreview["source_hash"] == "" || len(csvPreview["items"].([]interface{})) != 2 {
		t.Fatal("prévia CSV sem fingerprint ou itens")
	}
	var importCount int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_imports WHERE tenant_id=$1 AND operation_id=$2`, tenantA, csvOp).Scan(&importCount) != nil || importCount != 0 {
		t.Fatal("prévia CSV gravou resultado")
	}
	csvInput.PreviewHash = csvPreview["preview_hash"].(string)
	csvJSON, _ = json.Marshal(csvInput)
	csvApplied := request("POST", "/api/v1/catalog/imports/apply", string(csvJSON), tokenA, 200)
	checkProduct(csvID1, 2, 475, true)
	checkProduct(csvID2, 2, 350, false)
	request("POST", "/api/v1/catalog/imports/apply", string(csvJSON), tokenA, 200)
	request("GET", "/api/v1/catalog/imports/"+csvOp, "", tokenA, 200)
	request("GET", "/api/v1/catalog/imports/"+csvOp, "", otherToken, 404)
	request("GET", "/api/v1/catalog/imports/"+csvOp, "", tokenB, 404)
	csvAdministrative := request("GET", "/api/v1/catalog/batches/"+csvOp+"/import", "", otherToken, 200)
	if csvAdministrative["actor_id"] != userA || csvAdministrative["source_hash"] != csvApplied["source_hash"] {
		t.Fatal("consulta administrativa CSV perdeu ator ou origem")
	}
	changedImport := csvInput
	changedImport.Reason = "Different reason"
	changedJSON, _ := json.Marshal(changedImport)
	request("POST", "/api/v1/catalog/imports/apply", string(changedJSON), tokenA, 409)
	request("POST", "/api/v1/catalog/imports/apply", string(csvJSON), otherToken, 409)
	unknownImport := onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Unknown", CSV: "sku;expected_version;preco;ativo\nmissing-import;1;2;\n"}
	unknownJSON, _ := json.Marshal(unknownImport)
	request("POST", "/api/v1/catalog/imports/preview", string(unknownJSON), tokenA, 404)
	request("POST", "/api/v1/catalog/imports/preview", `{"operation_id":"x","reason":"test","csv":"x","tenant_id":"foreign"}`, tokenA, 400)
	// An imported batch can be compensated by the existing official undo flow.
	csvUndo := onlinecatalog.UndoInput{OperationID: uuid.NewString(), SourceOperationID: csvOp, Reason: "Correção da planilha"}
	csvUndoPreview, e := store.PreviewUndo(ctx, actor, csvUndo)
	if e != nil {
		t.Fatal("prévia de reversão CSV falhou")
	}
	csvUndo.PreviewHash = csvUndoPreview.PreviewHash
	if _, e = store.ApplyUndo(ctx, actor, csvUndo); e != nil {
		t.Fatal("reversão CSV falhou")
	}
	checkProduct(csvID1, 3, 250, true)
	checkProduct(csvID2, 3, 350, true)
	// Stable replay no longer depends on the current SKU or price/version.
	exec(`UPDATE products SET sku='import-renamed',catalog_version=catalog_version+1 WHERE tenant_id=$1 AND id=$2`, tenantA, csvID1)
	stableCSV := request("POST", "/api/v1/catalog/imports/apply", string(csvJSON), tokenA, 200)
	if stableCSV["source_hash"] != csvApplied["source_hash"] || stableCSV["batch"].(map[string]interface{})["items"].([]interface{})[0].(map[string]interface{})["after"].(map[string]interface{})["version"] != float64(2) {
		t.Fatal("replay CSV consultou cadastro atual")
	}
	checkProduct(csvID1, 4, 250, true)
	// Re-resolving SKU after a stale preview must refuse all rows.
	staleCSV := onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Stale CSV", CSV: "sku;expected_version;preco;ativo\nimport-renamed;4;5;\nimport-active;3;;false\n"}
	staleCSVPreview, e := store.PreviewImport(ctx, actor, staleCSV)
	if e != nil {
		t.Fatal("prévia CSV stale falhou")
	}
	staleCSV.PreviewHash = staleCSVPreview.PreviewHash
	exec(`UPDATE products SET sku='import-new-name',catalog_version=catalog_version+1 WHERE tenant_id=$1 AND id=$2`, tenantA, csvID1)
	if _, e = store.ApplyImport(ctx, actor, staleCSV); e != onlinecatalog.ErrMissing {
		t.Fatal("SKU antigo aceito")
	}
	checkProduct(csvID2, 3, 350, true)
	// Fault AFTER item mutation and batch insert rolls back product, audit and metadata.
	lateCSV := onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Rollback CSV", CSV: "sku;expected_version;preco;ativo\nimport-new-name;5;6;\nimport-active;3;;false\n"}
	lateCSVPreview, e := store.PreviewImport(ctx, actor, lateCSV)
	if e != nil {
		t.Fatal("prévia de rollback CSV falhou")
	}
	lateCSV.PreviewHash = lateCSVPreview.PreviewHash
	exec(fmt.Sprintf(`CREATE FUNCTION reject_import_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_import_event BEFORE INSERT ON online_catalog_import_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_import_event()`, lateCSV.OperationID))
	if _, e = store.ApplyImport(ctx, actor, lateCSV); e != onlinecatalog.ErrUnavailable {
		t.Fatal("evento CSV falho não recusou aplicação")
	}
	checkProduct(csvID1, 5, 250, true)
	checkProduct(csvID2, 3, 350, true)
	if _, e = store.Import(ctx, actor, lateCSV.OperationID, false); e != onlinecatalog.ErrMissing {
		t.Fatal("rollback CSV deixou recibo")
	}
	if _, e = store.Batch(ctx, actor, lateCSV.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("rollback CSV deixou lote")
	}
	lateCSVChild := uuid.NewSHA1(uuid.MustParse(lateCSV.OperationID), []byte(fmt.Sprintf("catalog-batch:%d", csvID1))).String()
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND operation_id=$2`, tenantA, lateCSVChild).Scan(&importCount) != nil || importCount != 0 {
		t.Fatal("rollback CSV deixou auditoria")
	}
	// Identical concurrent imports have one durable result and increment once.
	concurrentCSV := lateCSV
	concurrentCSV.OperationID = uuid.NewString()
	concurrentCSV.PreviewHash = ""
	concurrentCSVPreview, e := store.PreviewImport(ctx, actor, concurrentCSV)
	if e != nil {
		t.Fatal("prévia concorrente CSV falhou")
	}
	concurrentCSV.PreviewHash = concurrentCSVPreview.PreviewHash
	importErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.ApplyImport(ctx, actor, concurrentCSV)
			importErrors <- e
		}()
	}
	mutationWG.Wait()
	close(importErrors)
	for e := range importErrors {
		if e != nil {
			t.Fatal("replay concorrente CSV falhou")
		}
	}
	checkProduct(csvID1, 6, 600, true)
	checkProduct(csvID2, 4, 350, false)
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_import_outbox WHERE tenant_id=$1 AND operation_id=$2`, tenantA, concurrentCSV.OperationID).Scan(&importCount) != nil || importCount != 1 {
		t.Fatal("CSV duplicou evento")
	}
	// An unrelated regular batch identity cannot be re-labelled as an import.
	collisionCSV := concurrentCSV
	collisionCSV.OperationID = csvUndo.OperationID
	if _, e = store.ApplyImport(ctx, actor, collisionCSV); e != onlinecatalog.ErrConflict {
		t.Fatal("CSV aceitou identidade de lote já usada")
	}

	// Independent imports competing for the same versions cannot partly apply.
	importCompetition := make(chan error, 2)
	independentCSV := make([]onlinecatalog.ImportInput, 2)
	for i := range independentCSV {
		independentCSV[i] = onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Competing CSV", CSV: fmt.Sprintf("sku;expected_version;preco;ativo\nimport-new-name;6;%d;\nimport-active;4;;true\n", 7+i)}
		p, e := store.PreviewImport(ctx, actor, independentCSV[i])
		if e != nil {
			t.Fatal("prévia da disputa CSV falhou")
		}
		independentCSV[i].PreviewHash = p.PreviewHash
	}
	for _, v := range independentCSV {
		mutationWG.Add(1)
		go func(v onlinecatalog.ImportInput) {
			defer mutationWG.Done()
			_, e := store.ApplyImport(ctx, actor, v)
			importCompetition <- e
		}(v)
	}
	mutationWG.Wait()
	close(importCompetition)
	wins, conflicts = 0, 0
	for e := range importCompetition {
		if e == nil {
			wins++
		} else if e == onlinecatalog.ErrConflict {
			conflicts++
		} else {
			t.Fatal("disputa CSV falhou")
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("disputa CSV não preservou versão")
	}
	checkProduct(csvID2, 5, 350, true)
	var competingPrice int64
	var competingVersion int64
	if sqlDB.QueryRowContext(ctx, `SELECT (preco*100)::bigint,catalog_version FROM products WHERE tenant_id=$1 AND id=$2`, tenantA, csvID1).Scan(&competingPrice, &competingVersion) != nil || competingVersion != 7 || (competingPrice != 700 && competingPrice != 800) {
		t.Fatal("CSV parcialmente aplicado na disputa")
	}
	var otherImportPrice string
	if sqlDB.QueryRowContext(ctx, `SELECT preco::text FROM products WHERE tenant_id=$1 AND sku='import-price'`, tenantB).Scan(&otherImportPrice) != nil || otherImportPrice != "9.99" {
		t.Fatal("CSV alterou outra empresa")
	}
	// Export has its own fresh HTTP fixture; the actual limiter stays enabled.
	app = fiber.New()
	Registrar(app)
	exportOne := request("POST", "/api/v1/produtos/", `{"nome":"Export zero","sku":"export-round-001","preco":0}`, tokenA, 201)
	exportTwo := request("POST", "/api/v1/produtos/", `{"nome":"Export quoted","sku":"export-round;çafé","preco":9999999999.99}`, tokenA, 201)
	exportID1, exportID2 := int64(exportOne["ID"].(float64)), int64(exportTwo["ID"].(float64))
	request("POST", "/api/v1/produtos/", `{"nome":"Other export","sku":"export-round-001","preco":9.99}`, tokenB, 201)
	exportFirst := request("GET", "/api/v1/catalog/exports/preview?q=export-round&limit=1", "", tokenA, 200)
	if exportFirst["total"] != float64(2) || exportFirst["has_more"] != true || exportFirst["next_offset"] != float64(1) || len(exportFirst["items"].([]interface{})) != 1 {
		t.Fatal("exportação não paginou")
	}
	exportSecond := request("GET", "/api/v1/catalog/exports/preview?q=export-round&limit=1&offset=1", "", tokenA, 200)
	if exportSecond["has_more"] != false || len(exportSecond["items"].([]interface{})) != 1 {
		t.Fatal("segunda página CSV incorreta")
	}
	if _, present := exportSecond["next_offset"]; present {
		t.Fatal("exportação inventou próxima página")
	}
	exportAll := request("GET", "/api/v1/catalog/exports/preview?q=export-round&limit=100", "", tokenA, 200)
	csvBefore := exportAll["csv"].(string)
	if !strings.Contains(csvBefore, "0.00") || !strings.Contains(csvBefore, "9999999999.99") || !strings.Contains(csvBefore, `"export-round;çafé"`) {
		t.Fatal("CSV perdeu precisão ou escape")
	}
	exportOther := request("GET", "/api/v1/catalog/exports/preview?q=export-round", "", tokenB, 200)
	if exportOther["total"] != float64(1) || strings.Contains(exportOther["csv"].(string), "9999999999.99") {
		t.Fatal("exportação atravessou empresa")
	}
	request("GET", "/api/v1/catalog/exports/preview?tenant_id=foreign", "", tokenA, 400)
	request("GET", "/api/v1/catalog/exports/preview?action=price&action=active", "", tokenA, 400)
	// CSV download is raw text, so validate bytes/headers outside the JSON helper.
	exportReq := httptest.NewRequest("GET", "/api/v1/catalog/exports/csv?q=export-round&limit=100", nil)
	exportReq.Header.Set("Authorization", "Bearer "+tokenA)
	exportResp, e := app.Test(exportReq, 10000)
	if e != nil {
		t.Fatal("download CSV falhou")
	}
	exportBytes, e := io.ReadAll(exportResp.Body)
	exportResp.Body.Close()
	if e != nil || exportResp.StatusCode != 200 || string(exportBytes) != csvBefore || exportResp.Header.Get("X-Catalog-Source-Hash") != exportAll["source_hash"] || exportResp.Header.Get("Cache-Control") != "no-store" || exportResp.Header.Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatal("download CSV divergiu da prévia")
	}
	// Export itself leaves product versions intact, and unedited CSV is a no-op.
	checkProduct(exportID1, 1, 0, true)
	checkProduct(exportID2, 1, onlinecatalog.MaxPrice, true)
	roundInput := onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Round trip export/import", CSV: csvBefore}
	if _, e = store.PreviewImport(ctx, actor, roundInput); e != onlinecatalog.ErrInput {
		t.Fatal("CSV intacto inventou alteração")
	}
	roundInput.CSV = strings.ReplaceAll(strings.ReplaceAll(csvBefore, "0.00;", "1.25;"), "9999999999.99;", "2,75;")
	roundPreview, e := store.PreviewImport(ctx, actor, roundInput)
	if e != nil {
		t.Fatal("prévia do CSV editado falhou")
	}
	roundInput.PreviewHash = roundPreview.PreviewHash
	if _, e = store.ApplyImport(ctx, actor, roundInput); e != nil {
		t.Fatal("reimportação CSV editado falhou")
	}
	checkProduct(exportID1, 2, 125, true)
	checkProduct(exportID2, 2, 275, true)
	exportActive := request("GET", "/api/v1/catalog/exports/preview?q=export-round&action=active", "", tokenA, 200)
	activeText := exportActive["csv"].(string)
	if strings.Contains(activeText, "1.25") || !strings.Contains(activeText, ";2;;true") {
		t.Fatal("exportação de status misturou preço")
	}
	activeRound := onlinecatalog.ImportInput{OperationID: uuid.NewString(), Reason: "Status export/import", CSV: strings.ReplaceAll(activeText, ";;true", ";;false")}
	activeRoundPreview, e := store.PreviewImport(ctx, actor, activeRound)
	if e != nil {
		t.Fatal("prévia status exportado falhou")
	}
	activeRound.PreviewHash = activeRoundPreview.PreviewHash
	if _, e = store.ApplyImport(ctx, actor, activeRound); e != nil {
		t.Fatal("reimportação de status falhou")
	}
	checkProduct(exportID1, 3, 125, false)
	checkProduct(exportID2, 3, 275, false)
	exportEmpty := request("GET", "/api/v1/catalog/exports/preview?q=export-round&ativo=active", "", tokenA, 200)
	if exportEmpty["total"] != float64(0) || exportEmpty["has_more"] != false || exportEmpty["csv"] != "sku;expected_version;preco;ativo\n" || len(exportEmpty["items"].([]interface{})) != 0 {
		t.Fatal("exportação vazia inventou dados")
	}
	exportInactive := request("GET", "/api/v1/catalog/exports/preview?q=export-round&ativo=inactive&action=active", "", tokenA, 200)
	if exportInactive["total"] != float64(2) || !strings.Contains(exportInactive["csv"].(string), ";3;;false") {
		t.Fatal("filtro inativo não correspondeu estado real")
	}
	// Never export formulas or transform identifiers silently to suppress them.
	request("POST", "/api/v1/produtos/", `{"nome":"Unsafe export","sku":"=export-formula","preco":1}`, tokenA, 201)
	request("GET", "/api/v1/catalog/exports/preview?q=export-formula", "", tokenA, 409)
	var exportWrites int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND product_id IN ($2,$3)`, tenantA, exportID1, exportID2).Scan(&exportWrites) != nil || exportWrites != 4 {
		t.Fatal("exportação criou auditoria de alteração")
	}
	// Reajuste: fresh HTTP fixture preserves the actual per-app rate limiter.
	app = fiber.New()
	Registrar(app)
	adjustProduct1 := request("POST", "/api/v1/produtos/", `{"nome":"Adjust 101","sku":"adjust-101","preco":1.01}`, tokenA, 201)
	adjustProduct2 := request("POST", "/api/v1/produtos/", `{"nome":"Adjust 201","sku":"adjust-201","preco":2.01}`, tokenA, 201)
	adjustID1, adjustID2 := int64(adjustProduct1["ID"].(float64)), int64(adjustProduct2["ID"].(float64))
	adjust := onlinecatalog.AdjustmentInput{OperationID: uuid.NewString(), Reason: "Reajuste revisado", RateBasisPoints: 5000, Items: []onlinecatalog.AdjustmentItem{{ProductID: adjustID2, ExpectedVersion: 1}, {ProductID: adjustID1, ExpectedVersion: 1}}}
	adjustJSON, _ := json.Marshal(adjust)
	adjustPreview := request("POST", "/api/v1/catalog/adjustments/preview", string(adjustJSON), tokenA, 200)
	checkProduct(adjustID1, 1, 101, true)
	checkProduct(adjustID2, 1, 201, true)
	if adjustPreview["rounding"] != "half_up" || adjustPreview["items"].([]interface{})[0].(map[string]interface{})["after"].(map[string]interface{})["price_cents"] != float64(152) {
		t.Fatal("arredondamento reajuste inexato")
	}
	adjust.PreviewHash = adjustPreview["preview_hash"].(string)
	adjustJSON, _ = json.Marshal(adjust)
	request("POST", "/api/v1/catalog/adjustments/apply", string(adjustJSON), tokenA, 200)
	request("POST", "/api/v1/catalog/adjustments/apply", string(adjustJSON), tokenA, 200)
	checkProduct(adjustID1, 2, 152, true)
	checkProduct(adjustID2, 2, 302, true)
	request("GET", "/api/v1/catalog/adjustments/"+adjust.OperationID, "", tokenA, 200)
	request("GET", "/api/v1/catalog/adjustments/"+adjust.OperationID, "", otherToken, 404)
	request("GET", "/api/v1/catalog/adjustments/"+adjust.OperationID, "", tokenB, 404)
	adjustHistory := request("GET", "/api/v1/catalog/adjustments?limit=1&offset=0", "", otherToken, 200)
	if len(adjustHistory["items"].([]interface{})) != 1 || adjustHistory["items"].([]interface{})[0].(map[string]interface{})["reason"] != "Reajuste revisado" {
		t.Fatal("histórico reajuste perdeu motivo")
	}
	request("GET", "/api/v1/catalog/adjustments?limit=11", "", tokenA, 400)
	adjustForeignHistory := request("GET", "/api/v1/catalog/adjustments", "", tokenB, 200)
	if len(adjustForeignHistory["items"].([]interface{})) != 0 {
		t.Fatal("histórico reajuste atravessou empresa")
	}
	request("POST", "/api/v1/catalog/adjustments/preview", string(strings.ReplaceAll(string(adjustJSON), `,"preview_hash":`, `,"extra":`)), tokenA, 400)
	changedAdjust := adjust
	changedAdjust.RateBasisPoints = 4999
	changedAdjustJSON, _ := json.Marshal(changedAdjust)
	request("POST", "/api/v1/catalog/adjustments/apply", string(changedAdjustJSON), tokenA, 409)
	request("POST", "/api/v1/catalog/adjustments/apply", string(adjustJSON), otherToken, 409)
	foreignAdjust := adjust
	foreignAdjust.OperationID = uuid.NewString()
	foreignAdjust.PreviewHash = ""
	foreignJSON, _ := json.Marshal(foreignAdjust)
	request("POST", "/api/v1/catalog/adjustments/preview", string(foreignJSON), tokenB, 404)
	// Official batch compensation restores original prices with new versions.
	adjustUndo := onlinecatalog.UndoInput{OperationID: uuid.NewString(), SourceOperationID: adjust.OperationID, Reason: "Corrige reajuste"}
	adjustUndoPreview, e := store.PreviewUndo(ctx, actor, adjustUndo)
	if e != nil {
		t.Fatal("prévia reversão reajuste falhou")
	}
	adjustUndo.PreviewHash = adjustUndoPreview.PreviewHash
	if _, e = store.ApplyUndo(ctx, actor, adjustUndo); e != nil {
		t.Fatal("reversão reajuste falhou")
	}
	checkProduct(adjustID1, 3, 101, true)
	checkProduct(adjustID2, 3, 201, true)
	adjustReplay := request("POST", "/api/v1/catalog/adjustments/apply", string(adjustJSON), tokenA, 200)
	if adjustReplay["batch"].(map[string]interface{})["items"].([]interface{})[0].(map[string]interface{})["after"].(map[string]interface{})["price_cents"] != float64(152) {
		t.Fatal("replay reajuste recalculou preço atual")
	}
	lateAdjustment := onlinecatalog.AdjustmentInput{OperationID: uuid.NewString(), Reason: "Rollback reajuste", RateBasisPoints: 1000, Items: []onlinecatalog.AdjustmentItem{{ProductID: adjustID1, ExpectedVersion: 3}, {ProductID: adjustID2, ExpectedVersion: 3}}}
	lateAdjustmentPreview, e := store.PreviewAdjustment(ctx, actor, lateAdjustment)
	if e != nil {
		t.Fatal("prévia rollback reajuste falhou")
	}
	lateAdjustment.PreviewHash = lateAdjustmentPreview.PreviewHash
	exec(fmt.Sprintf(`CREATE FUNCTION reject_adjustment_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_adjustment_event BEFORE INSERT ON online_catalog_adjustment_outbox FOR EACH ROW WHEN (NEW.operation_id='%s'::uuid) EXECUTE FUNCTION reject_adjustment_event()`, lateAdjustment.OperationID))
	if _, e = store.ApplyAdjustment(ctx, actor, lateAdjustment); e != onlinecatalog.ErrUnavailable {
		t.Fatal("evento falho de reajuste aceito")
	}
	checkProduct(adjustID1, 3, 101, true)
	checkProduct(adjustID2, 3, 201, true)
	if _, e = store.Adjustment(ctx, actor, lateAdjustment.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("rollback reajuste deixou recibo")
	}
	if _, e = store.Batch(ctx, actor, lateAdjustment.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("rollback reajuste deixou lote")
	}
	lateAdjustmentChild := uuid.NewSHA1(uuid.MustParse(lateAdjustment.OperationID), []byte(fmt.Sprintf("catalog-batch:%d", adjustID1))).String()
	var adjustmentCount int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1 AND operation_id=$2`, tenantA, lateAdjustmentChild).Scan(&adjustmentCount) != nil || adjustmentCount != 0 {
		t.Fatal("rollback reajuste deixou auditoria")
	}
	concurrentAdjustment := lateAdjustment
	concurrentAdjustment.OperationID = uuid.NewString()
	concurrentAdjustment.PreviewHash = ""
	concurrentAdjustmentPreview, e := store.PreviewAdjustment(ctx, actor, concurrentAdjustment)
	if e != nil {
		t.Fatal("prévia concorrente reajuste falhou")
	}
	concurrentAdjustment.PreviewHash = concurrentAdjustmentPreview.PreviewHash
	adjustmentErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.ApplyAdjustment(ctx, actor, concurrentAdjustment)
			adjustmentErrors <- e
		}()
	}
	mutationWG.Wait()
	close(adjustmentErrors)
	for e := range adjustmentErrors {
		if e != nil {
			t.Fatal("replay concorrente reajuste falhou")
		}
	}
	checkProduct(adjustID1, 4, 111, true)
	checkProduct(adjustID2, 4, 221, true)
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_adjustment_outbox WHERE tenant_id=$1 AND operation_id=$2`, tenantA, concurrentAdjustment.OperationID).Scan(&adjustmentCount) != nil || adjustmentCount != 1 {
		t.Fatal("reajuste duplicou evento")
	}
	// Stale explicit selection refuses the complete adjustment.
	staleAdjustment := lateAdjustment
	staleAdjustment.OperationID = uuid.NewString()
	staleAdjustment.PreviewHash = ""
	if _, e = store.PreviewAdjustment(ctx, actor, staleAdjustment); e != onlinecatalog.ErrConflict {
		t.Fatal("reajuste stale aceito")
	}
	zeroAdjustment := onlinecatalog.AdjustmentInput{OperationID: uuid.NewString(), Reason: "Redução autorizada a zero", RateBasisPoints: -10000, Items: []onlinecatalog.AdjustmentItem{{ProductID: adjustID1, ExpectedVersion: 4}}}
	zeroAdjustmentPreview, e := store.PreviewAdjustment(ctx, actor, zeroAdjustment)
	if e != nil {
		t.Fatal("prévia redução a zero falhou")
	}
	zeroAdjustment.PreviewHash = zeroAdjustmentPreview.PreviewHash
	if _, e = store.ApplyAdjustment(ctx, actor, zeroAdjustment); e != nil {
		t.Fatal("redução exata a zero falhou")
	}
	checkProduct(adjustID1, 5, 0, true)
	noOpAdjustment := zeroAdjustment
	noOpAdjustment.OperationID = uuid.NewString()
	noOpAdjustment.PreviewHash = ""
	noOpAdjustment.RateBasisPoints = 100
	noOpAdjustment.Items[0].ExpectedVersion = 5
	if _, e = store.PreviewAdjustment(ctx, actor, noOpAdjustment); e != onlinecatalog.ErrInput {
		t.Fatal("reajuste sem efeito inventou versão")
	}
	// Historical adjustment export: read-only and independent from current prices.
	app = fiber.New()
	Registrar(app)
	var reportOperationCount, reportEventCount int64
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_adjustments WHERE tenant_id=$1`, tenantA).Scan(&reportOperationCount) != nil || sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_adjustment_outbox WHERE tenant_id=$1`, tenantA).Scan(&reportEventCount) != nil {
		t.Fatal("contagem relatório ausente")
	}
	if reportOperationCount != 3 {
		t.Fatal("operações inesperadas na fixture de reajuste")
	}
	report := request("GET", "/api/v1/catalog/adjustments/exports/preview?limit=1", "", tokenA, 200)
	if report["total"] != float64(reportOperationCount) || report["row_count"] != float64(1) || report["next_offset"] != float64(1) || report["text_prefix"] != "'" || !strings.Contains(report["csv"].(string), ";111;0;1.11;0.00;-10000;") {
		t.Fatal("relatório sem valores históricos exatos")
	}
	request("GET", "/api/v1/catalog/adjustments/exports/preview?limit=11", "", tokenA, 400)
	request("GET", "/api/v1/catalog/adjustments/exports/preview?actor_id=", "", tokenA, 400)
	request("GET", "/api/v1/catalog/adjustments/exports/preview?product_id=01", "", tokenA, 400)
	request("GET", "/api/v1/catalog/adjustments/exports/preview?tenant_id="+tenantA, "", tokenA, 400)
	productReport := request("GET", fmt.Sprintf("/api/v1/catalog/adjustments/exports/preview?product_id=%d", adjustID2), "", tokenA, 200)
	if productReport["total"] != float64(2) || productReport["row_count"] != float64(2) || !strings.Contains(productReport["csv"].(string), ";201;302;2.01;3.02;5000;") {
		t.Fatal("reajuste compensado perdeu histórico")
	}
	// Original receipt still contains both products; filtered CSV contains only ID2.
	if len(productReport["items"].([]interface{})[0].(map[string]interface{})["batch"].(map[string]interface{})["items"].([]interface{})) != 2 {
		t.Fatal("recibo foi mutilado")
	}
	otherActorReport := request("GET", "/api/v1/catalog/adjustments/exports/preview?actor_id="+otherUser, "", tokenA, 200)
	if otherActorReport["total"] != float64(0) || otherActorReport["items"] == nil {
		t.Fatal("responsável sem operações inventou linhas")
	}
	adminReport := request("GET", "/api/v1/catalog/adjustments/exports/preview?actor_id="+userA, "", otherToken, 200)
	if adminReport["total"] != float64(reportOperationCount) {
		t.Fatal("administrador não consultou relatório autorizado")
	}
	foreignReport := request("GET", fmt.Sprintf("/api/v1/catalog/adjustments/exports/preview?actor_id=%s&product_id=%d", userA, adjustID1), "", tokenB, 200)
	if foreignReport["total"] != float64(0) {
		t.Fatal("relatório vazou empresa")
	}
	beyondReport := request("GET", "/api/v1/catalog/adjustments/exports/preview?offset=99", "", tokenA, 200)
	if beyondReport["row_count"] != float64(0) || beyondReport["has_more"] != false || !strings.HasSuffix(beyondReport["csv"].(string), "rounding\n") {
		t.Fatal("página vazia incorreta")
	}
	var originalAdjustmentAt time.Time
	if sqlDB.QueryRowContext(ctx, `SELECT created_at FROM online_catalog_adjustments WHERE tenant_id=$1 AND operation_id=$2`, tenantA, adjust.OperationID).Scan(&originalAdjustmentAt) != nil {
		t.Fatal("timestamp histórico ausente")
	}
	originalReport, e := store.ExportAdjustments(ctx, actor, onlinecatalog.AdjustmentExportQuery{Limit: 10, From: originalAdjustmentAt.UTC().Format(time.RFC3339Nano), Until: originalAdjustmentAt.Add(time.Microsecond).UTC().Format(time.RFC3339Nano)})
	if e != nil || originalReport.Total != 1 || originalReport.RowCount != 2 || originalReport.Items[0].OperationID != adjust.OperationID {
		t.Fatal("limites inclusivo/exclusivo históricos")
	}
	excludedReport, e := store.ExportAdjustments(ctx, actor, onlinecatalog.AdjustmentExportQuery{Limit: 10, From: originalAdjustmentAt.Add(-time.Microsecond).UTC().Format(time.RFC3339Nano), Until: originalAdjustmentAt.UTC().Format(time.RFC3339Nano)})
	if e != nil || excludedReport.Total != 0 {
		t.Fatal("until não exclusivo")
	}
	downloadReport, e := store.ExportAdjustments(ctx, actor, onlinecatalog.AdjustmentExportQuery{Limit: 1, ProductID: adjustID2})
	if e != nil {
		t.Fatal("exportação filtrada indisponível")
	}
	downloadRequest := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/catalog/adjustments/exports/csv?limit=1&product_id=%d", adjustID2), nil)
	downloadRequest.Header.Set("Authorization", "Bearer "+tokenA)
	downloadResponse, e := app.Test(downloadRequest, 5000)
	if e != nil {
		t.Fatal("download reajuste falhou")
	}
	downloadBody, e := io.ReadAll(downloadResponse.Body)
	downloadResponse.Body.Close()
	if e != nil || downloadResponse.StatusCode != 200 || string(downloadBody) != downloadReport.CSV || downloadResponse.Header.Get("X-Catalog-Source-Hash") != downloadReport.SourceHash || downloadResponse.Header.Get("X-Catalog-Row-Count") != "1" || downloadResponse.Header.Get("X-Catalog-Text-Prefix") != "'" || downloadResponse.Header.Get("Content-Type") != "text/csv; charset=utf-8" || downloadResponse.Header.Get("Content-Disposition") != `attachment; filename="titan-catalog-adjustment-history.csv"` || downloadResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("download não correspondeu à página")
	}
	// Malformed durable receipt refuses the complete report; no partial data.
	exec(`UPDATE online_catalog_adjustments SET reason='fixture mismatch' WHERE tenant_id=$1 AND operation_id=$2`, tenantA, adjust.OperationID)
	if _, e = store.ExportAdjustments(ctx, actor, onlinecatalog.AdjustmentExportQuery{Limit: 10}); e != onlinecatalog.ErrUnavailable {
		t.Fatal("recibo malformado exportado")
	}
	exec(`UPDATE online_catalog_adjustments SET reason=$3 WHERE tenant_id=$1 AND operation_id=$2`, tenantA, adjust.OperationID, adjust.Reason)
	var reportOperationsAfter, reportEventsAfter int64
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_adjustments WHERE tenant_id=$1`, tenantA).Scan(&reportOperationsAfter) != nil || sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_adjustment_outbox WHERE tenant_id=$1`, tenantA).Scan(&reportEventsAfter) != nil || reportOperationsAfter != reportOperationCount || reportEventsAfter != reportEventCount {
		t.Fatal("relatório produziu escritas")
	}
	checkProduct(adjustID1, 5, 0, true)
	checkProduct(adjustID2, 4, 221, true)
	// Multiple online GTINs, versioned lifecycle, duplicate protection and recovery.
	app = fiber.New()
	Registrar(app)
	barcodeProductA := request("POST", "/api/v1/produtos/", `{"nome":"Leitura A","sku":"barcode-A","preco":5.25}`, tokenA, 201)
	barcodeProductB := request("POST", "/api/v1/produtos/", `{"nome":"Leitura B","sku":"barcode-B","preco":7.50}`, tokenA, 201)
	barcodeProductForeign := request("POST", "/api/v1/produtos/", `{"nome":"Leitura outra empresa","sku":"barcode-foreign","preco":9}`, tokenB, 201)
	barcodeProductID := int64(barcodeProductA["ID"].(float64))
	barcodeOtherID := int64(barcodeProductB["ID"].(float64))
	barcodeForeignID := int64(barcodeProductForeign["ID"].(float64))
	firstCodeInput := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 1, Code: "4006381333931", Reason: "Código da unidade conferido"}
	firstCodeJSON, _ := json.Marshal(firstCodeInput)
	firstCode := request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeProductID), string(firstCodeJSON), tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeProductID), string(firstCodeJSON), tokenA, 200)
	checkProduct(barcodeProductID, 2, 525, true)
	if firstCode["after"].(map[string]interface{})["canonical_code"] != "04006381333931" {
		t.Fatal("GTIN não normalizado")
	}
	barcodeLookup := request("GET", "/api/v1/catalog/barcodes/lookup?code=04006381333931", "", tokenA, 200)
	if barcodeLookup["product"].(map[string]interface{})["id"] != float64(barcodeProductID) || barcodeLookup["product"].(map[string]interface{})["price_cents"] != float64(525) {
		t.Fatal("leitura não resolveu produto/preço real")
	}
	secondCodeInput := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 2, Code: "036000291452", Reason: "Código alternativo conferido"}
	secondCodeJSON, _ := json.Marshal(secondCodeInput)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeProductID), string(secondCodeJSON), tokenA, 200)
	checkProduct(barcodeProductID, 3, 525, true)
	codesPage := request("GET", fmt.Sprintf("/api/v1/produtos/%d/barcodes?limit=1", barcodeProductID), "", tokenA, 200)
	if codesPage["total"] != float64(2) || codesPage["has_more"] != true || len(codesPage["items"].([]interface{})) != 1 {
		t.Fatal("lista de códigos não paginada")
	}
	dupCode := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 1, Code: "04006381333931", Reason: "Tentativa equivalente"}
	dupJSON, _ := json.Marshal(dupCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeOtherID), string(dupJSON), tokenA, 409)
	checkProduct(barcodeOtherID, 1, 750, true)
	dupCode.OperationID = uuid.NewString()
	dupJSON, _ = json.Marshal(dupCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeForeignID), string(dupJSON), tokenB, 200)
	foreignCode := request("GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", tokenB, 200)
	if foreignCode["product"].(map[string]interface{})["id"] != float64(barcodeForeignID) {
		t.Fatal("código resolveu empresa incorreta")
	}
	request("GET", "/api/v1/catalog/barcodes/operations/"+firstCodeInput.OperationID, "", tokenB, 404)
	request("GET", "/api/v1/catalog/barcodes/operations/"+firstCodeInput.OperationID, "", otherToken, 404)
	stateFalse := false
	disableCode := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 3, ExpectedCodeVersion: 1, Active: &stateFalse, Reason: "Etiqueta antiga"}
	disableJSON, _ := json.Marshal(disableCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeProductID, firstCodeInput.OperationID), string(disableJSON), tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeProductID, firstCodeInput.OperationID), string(disableJSON), tokenA, 200)
	checkProduct(barcodeProductID, 4, 525, true)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", tokenA, 404)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=00036000291452", "", tokenA, 200)
	inactiveCodes := request("GET", fmt.Sprintf("/api/v1/produtos/%d/barcodes?state=inactive", barcodeProductID), "", tokenA, 200)
	if inactiveCodes["total"] != float64(1) {
		t.Fatal("inativação não listada")
	}
	dupCode.OperationID = uuid.NewString()
	dupJSON, _ = json.Marshal(dupCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeOtherID), string(dupJSON), tokenA, 409)
	stateTrue := true
	reactivateCode := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 4, ExpectedCodeVersion: 2, Active: &stateTrue, Reason: "Etiqueta reconferida"}
	reactivateJSON, _ := json.Marshal(reactivateCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeProductID, firstCodeInput.OperationID), string(reactivateJSON), tokenA, 200)
	checkProduct(barcodeProductID, 5, 525, true)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", tokenA, 200)
	reactivateCode.OperationID = uuid.NewString()
	reactivateCode.ExpectedVersion = 5
	reactivateCode.ExpectedCodeVersion = 3
	reactivateJSON, _ = json.Marshal(reactivateCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeProductID, firstCodeInput.OperationID), string(reactivateJSON), tokenA, 409)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", barcodeProductID), fmt.Sprintf(`{"operation_id":%q,"expected_version":5,"price_cents":600}`, uuid.NewString()), tokenA, 200)
	checkProduct(barcodeProductID, 6, 600, true)
	originalCodeReceipt := request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", barcodeProductID), string(firstCodeJSON), tokenA, 200)
	if originalCodeReceipt["product_after"].(map[string]interface{})["price_cents"] != float64(525) || originalCodeReceipt["product_after"].(map[string]interface{})["version"] != float64(2) {
		t.Fatal("replay reescreveu recibo com estado atual")
	}
	barcodeHistory := request("GET", fmt.Sprintf("/api/v1/produtos/%d/barcodes/history?limit=2", barcodeProductID), "", otherToken, 200)
	if barcodeHistory["total"] != float64(4) || barcodeHistory["has_more"] != true {
		t.Fatal("auditoria administrativa incompleta")
	}
	wrongProductCode := disableCode
	wrongProductCode.OperationID = uuid.NewString()
	wrongProductCode.ExpectedVersion = 1
	wrongProductJSON, _ := json.Marshal(wrongProductCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeOtherID, firstCodeInput.OperationID), string(wrongProductJSON), tokenA, 404)
	staleCode := disableCode
	staleCode.OperationID = uuid.NewString()
	staleCode.ExpectedVersion = 6
	staleCode.ExpectedCodeVersion = 1
	staleJSON, _ := json.Marshal(staleCode)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", barcodeProductID, firstCodeInput.OperationID), string(staleJSON), tokenA, 409)
	exec(`CREATE FUNCTION refuse_barcode_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated fixture' USING ERRCODE='23514'; END $$; CREATE TRIGGER refuse_barcode_event BEFORE INSERT ON online_catalog_barcode_outbox FOR EACH ROW EXECUTE FUNCTION refuse_barcode_event()`)
	lateCode := onlinecatalog.BarcodeInput{OperationID: uuid.NewString(), ExpectedVersion: 6, Code: "96385074", Reason: "Rollback tardio conferido"}
	if _, e = store.ChangeBarcode(ctx, actor, barcodeProductID, "", "add", lateCode); e != onlinecatalog.ErrUnavailable {
		t.Fatal("falha outbox de código publicou sucesso")
	}
	checkProduct(barcodeProductID, 6, 600, true)
	var lateCodeCount int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND canonical_code='00000096385074'`, tenantA).Scan(&lateCodeCount) != nil || lateCodeCount != 0 {
		t.Fatal("rollback preservou código parcial")
	}
	if _, e = store.BarcodeOperation(ctx, actor, lateCode.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("rollback preservou recibo parcial")
	}
	exec(`ALTER TABLE online_catalog_barcode_outbox DISABLE TRIGGER refuse_barcode_event`)
	// Independent operations and products racing for the same code cannot both win.
	concurrentCodeProduct1 := request("POST", "/api/v1/produtos/", `{"nome":"Concorrência código 1","sku":"concurrent-code-1","preco":1}`, tokenA, 201)
	concurrentCodeProduct2 := request("POST", "/api/v1/produtos/", `{"nome":"Concorrência código 2","sku":"concurrent-code-2","preco":1}`, tokenA, 201)
	concurrentCodeProducts := []int64{int64(concurrentCodeProduct1["ID"].(float64)), int64(concurrentCodeProduct2["ID"].(float64))}
	codeErrors := make(chan error, 2)
	for _, id := range concurrentCodeProducts {
		mutationWG.Add(1)
		go func(product int64) {
			defer mutationWG.Done()
			input := lateCode
			input.OperationID = uuid.NewString()
			input.ExpectedVersion = 1
			_, e := store.ChangeBarcode(ctx, actor, product, "", "add", input)
			codeErrors <- e
		}(id)
	}
	mutationWG.Wait()
	close(codeErrors)
	codeWins, codeConflicts := 0, 0
	for e := range codeErrors {
		if e == nil {
			codeWins++
		} else if e == onlinecatalog.ErrConflict {
			codeConflicts++
		} else {
			t.Fatal("concorrência de código falhou")
		}
	}
	if codeWins != 1 || codeConflicts != 1 {
		t.Fatal("código atribuído a dois produtos")
	}
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND canonical_code='00000096385074'`, tenantA).Scan(&lateCodeCount) != nil || lateCodeCount != 1 {
		t.Fatal("duplicata concorrente persistiu")
	}
	// Inactive product blocks every code, even if the alias remains active.
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/active", barcodeProductID), fmt.Sprintf(`{"operation_id":%q,"expected_version":6,"ativo":false}`, uuid.NewString()), tokenA, 200)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", tokenA, 404)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=036000291452", "", tokenA, 404)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/active", barcodeProductID), fmt.Sprintf(`{"operation_id":%q,"expected_version":7,"ativo":true}`, uuid.NewString()), tokenA, 200)
	request("GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", tokenA, 200)
	// Atomic barcode batches: preview, apply, original receipt and rollback.
	app = fiber.New()
	Registrar(app)
	gtin33 := func(base string) string {
		total := 0
		weight := 3
		for i := len(base) - 1; i >= 0; i-- {
			total += int(base[i]-'0') * weight
			weight = 4 - weight
		}
		return base + fmt.Sprint((10-total%10)%10)
	}
	newBatchProduct33 := func(sku string) int64 {
		result := request("POST", "/api/v1/produtos/", fmt.Sprintf(`{"nome":"Lote de etiquetas","sku":%q,"preco":1.25}`, sku), tokenA, 201)
		return int64(result["ID"].(float64))
	}
	batchID33a, batchID33b := newBatchProduct33("gtin-batch-a"), newBatchProduct33("gtin-batch-b")
	batch33 := onlinecatalog.BarcodeBatchInput{OperationID: uuid.NewString(), Reason: "Etiquetas conferidas em lote", Items: []onlinecatalog.BarcodeBatchItem{{ProductID: batchID33b, ExpectedVersion: 1, Code: gtin33("789123450002")}, {ProductID: batchID33a, ExpectedVersion: 1, Code: gtin33("789123450001")}}}
	batchJSON33, _ := json.Marshal(batch33)
	preview33 := request("POST", "/api/v1/catalog/barcodes/batches/preview", string(batchJSON33), tokenA, 200)
	checkProduct(batchID33a, 1, 125, true)
	checkProduct(batchID33b, 1, 125, true)
	if _, e := store.BarcodeBatch(ctx, actor, batch33.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("prévia gravou recibo")
	}
	batch33.PreviewHash = preview33["preview_hash"].(string)
	batchJSON33, _ = json.Marshal(batch33)
	applied33 := request("POST", "/api/v1/catalog/barcodes/batches/apply", string(batchJSON33), tokenA, 200)
	if len(applied33["items"].([]interface{})) != 2 {
		t.Fatal("lote não registrou todos itens")
	}
	checkProduct(batchID33a, 2, 125, true)
	checkProduct(batchID33b, 2, 125, true)
	request("GET", "/api/v1/catalog/barcodes/batches/"+batch33.OperationID, "", tokenA, 200)
	request("GET", "/api/v1/catalog/barcodes/batches/"+batch33.OperationID, "", tokenB, 404)
	request("GET", "/api/v1/catalog/barcodes/batches/"+batch33.OperationID, "", otherToken, 404)
	request("POST", "/api/v1/catalog/barcodes/batches/apply", string(batchJSON33), otherToken, 409)
	request("GET", "/api/v1/catalog/barcodes/lookup?code="+gtin33("789123450001"), "", tokenA, 200)
	item33 := applied33["items"].([]interface{})[0].(map[string]interface{})
	request("GET", "/api/v1/catalog/barcodes/operations/"+item33["operation_id"].(string), "", tokenA, 200)
	request("GET", fmt.Sprintf("/api/v1/produtos/%d/barcodes/history", batchID33a), "", otherToken, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", batchID33a), fmt.Sprintf(`{"operation_id":%q,"expected_version":2,"price_cents":180}`, uuid.NewString()), tokenA, 200)
	original33 := request("POST", "/api/v1/catalog/barcodes/batches/apply", string(batchJSON33), tokenA, 200)
	if original33["items"].([]interface{})[0].(map[string]interface{})["product_after"].(map[string]interface{})["price_cents"] != float64(125) {
		t.Fatal("replay mudou snapshot")
	}
	checkProduct(batchID33a, 3, 180, true)
	changed33 := batch33
	changed33.Reason = "Outro motivo"
	changedJSON33, _ := json.Marshal(changed33)
	request("POST", "/api/v1/catalog/barcodes/batches/apply", string(changedJSON33), tokenA, 409)
	// Duplicate canonical code in a different product rejects the complete preview.
	duplicate33 := onlinecatalog.BarcodeBatchInput{OperationID: uuid.NewString(), Reason: "Duplicidade", Items: []onlinecatalog.BarcodeBatchItem{{ProductID: batchID33b, ExpectedVersion: 2, Code: gtin33("789123450001")}}}
	duplicateJSON33, _ := json.Marshal(duplicate33)
	request("POST", "/api/v1/catalog/barcodes/batches/preview", string(duplicateJSON33), tokenA, 409)
	// A intervening product edit invalidates the hash/version; no new code appears.
	staleID33 := newBatchProduct33("gtin-batch-stale")
	stale33 := onlinecatalog.BarcodeBatchInput{OperationID: uuid.NewString(), Reason: "Prévia obsoleta", Items: []onlinecatalog.BarcodeBatchItem{{ProductID: staleID33, ExpectedVersion: 1, Code: gtin33("789123450003")}}}
	stalePreview33, e := store.PreviewBarcodeBatch(ctx, actor, stale33)
	if e != nil {
		t.Fatal("prévia válida recusada")
	}
	stale33.PreviewHash = stalePreview33.PreviewHash
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", staleID33), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"price_cents":150}`, uuid.NewString()), tokenA, 200)
	if _, e = store.ApplyBarcodeBatch(ctx, actor, stale33); e != onlinecatalog.ErrConflict {
		t.Fatal("prévia obsoleta aplicada")
	}
	checkProduct(staleID33, 2, 150, true)
	// Failure after both item outboxes must undo both codes, versions and receipts.
	rollbackID33a, rollbackID33b := newBatchProduct33("gtin-batch-rollback-a"), newBatchProduct33("gtin-batch-rollback-b")
	rollback33 := onlinecatalog.BarcodeBatchInput{OperationID: uuid.NewString(), Reason: "Falha tardia", Items: []onlinecatalog.BarcodeBatchItem{{ProductID: rollbackID33a, ExpectedVersion: 1, Code: gtin33("789123450004")}, {ProductID: rollbackID33b, ExpectedVersion: 1, Code: gtin33("789123450005")}}}
	rollbackPreview33, e := store.PreviewBarcodeBatch(ctx, actor, rollback33)
	if e != nil {
		t.Fatal("prévia rollback indisponível")
	}
	rollback33.PreviewHash = rollbackPreview33.PreviewHash
	exec(`CREATE FUNCTION refuse_barcode_batch_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated fixture' USING ERRCODE='23514'; END $$; CREATE TRIGGER refuse_barcode_batch_event BEFORE INSERT ON online_catalog_barcode_batch_outbox FOR EACH ROW EXECUTE FUNCTION refuse_barcode_batch_event()`)
	if _, e = store.ApplyBarcodeBatch(ctx, actor, rollback33); e != onlinecatalog.ErrUnavailable {
		t.Fatal("outbox de lote falhou sem rollback")
	}
	checkProduct(rollbackID33a, 1, 125, true)
	checkProduct(rollbackID33b, 1, 125, true)
	var partial33 int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id IN ($2,$3)`, tenantA, rollbackID33a, rollbackID33b).Scan(&partial33) != nil || partial33 != 0 {
		t.Fatal("código parcial do lote")
	}
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcode_operations WHERE tenant_id=$1 AND product_id IN ($2,$3)`, tenantA, rollbackID33a, rollbackID33b).Scan(&partial33) != nil || partial33 != 0 {
		t.Fatal("auditoria parcial do lote")
	}
	if _, e = store.BarcodeBatch(ctx, actor, rollback33.OperationID); e != onlinecatalog.ErrMissing {
		t.Fatal("recibo parcial do lote")
	}
	exec(`ALTER TABLE online_catalog_barcode_batch_outbox DISABLE TRIGGER refuse_barcode_batch_event`)
	// Two concurrent retries must publish one complete batch, not duplicate aliases.
	batchResults33 := make(chan error, 2)
	for i := 0; i < 2; i++ {
		mutationWG.Add(1)
		go func() {
			defer mutationWG.Done()
			_, e := store.ApplyBarcodeBatch(ctx, actor, rollback33)
			batchResults33 <- e
		}()
	}
	mutationWG.Wait()
	close(batchResults33)
	for e := range batchResults33 {
		if e != nil {
			t.Fatal("replay concorrente do lote falhou")
		}
	}
	checkProduct(rollbackID33a, 2, 125, true)
	checkProduct(rollbackID33b, 2, 125, true)
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcode_batch_outbox WHERE tenant_id=$1 AND operation_id=$2`, tenantA, rollback33.OperationID).Scan(&partial33) != nil || partial33 != 1 {
		t.Fatal("outbox de lote duplicada")
	}
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id IN ($2,$3)`, tenantA, rollbackID33a, rollbackID33b).Scan(&partial33) != nil || partial33 != 2 {
		t.Fatal("replay duplicou códigos")
	}
	// GTIN CSV adapter reuses the exact atomic batch operation and receipt.
	app = fiber.New()
	Registrar(app)
	csvRequest34 := func(path string, access string, want int) string {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, e := app.Test(req, 10000)
		if e != nil {
			t.Fatal("download CSV indisponível")
		}
		defer resp.Body.Close()
		data, e := io.ReadAll(resp.Body)
		if e != nil || resp.StatusCode != want {
			t.Fatal("status download CSV incorreto")
		}
		if want == 200 && (!strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;")) {
			t.Fatal("download CSV sem proteção")
		}
		return string(data)
	}
	if csvRequest34("/api/v1/catalog/barcodes/import/template", tokenA, 200) != onlinecatalog.BarcodeCSVHeader {
		t.Fatal("modelo CSV não canônico")
	}
	csvID34a, csvID34b := newBatchProduct33("csv-gtin-a"), newBatchProduct33("csv-gtin-b")
	csv34 := onlinecatalog.BarcodeCSVInput{OperationID: uuid.NewString(), Reason: "Códigos da planilha conferidos", CSV: onlinecatalog.BarcodeCSVHeader + fmt.Sprintf("%d;1;%s\n%d;1;%s\n", csvID34b, gtin33("789123450011"), csvID34a, gtin33("789123450010"))}
	csvBody34, _ := json.Marshal(csv34)
	csvPreview34 := request("POST", "/api/v1/catalog/barcodes/import/preview", string(csvBody34), tokenA, 200)
	checkProduct(csvID34a, 1, 125, true)
	checkProduct(csvID34b, 1, 125, true)
	request("GET", "/api/v1/catalog/barcodes/batches/"+csv34.OperationID, "", tokenA, 404)
	csv34.PreviewHash = csvPreview34["preview_hash"].(string)
	csvBody34, _ = json.Marshal(csv34)
	csvApplied34 := request("POST", "/api/v1/catalog/barcodes/import/apply", string(csvBody34), tokenA, 200)
	checkProduct(csvID34a, 2, 125, true)
	checkProduct(csvID34b, 2, 125, true)
	request("GET", "/api/v1/catalog/barcodes/batches/"+csv34.OperationID, "", tokenA, 200)
	request("GET", "/api/v1/catalog/barcodes/batches/"+csv34.OperationID, "", tokenB, 404)
	request("POST", "/api/v1/catalog/barcodes/import/apply", string(csvBody34), tokenA, 200)
	// Identical normalized JSON items and CSV share an identity, not two imports.
	csvBatch34, e := onlinecatalog.DecodeBarcodeCSV("application/json", csvBody34, true)
	if e != nil {
		t.Fatal("CSV válido recusado")
	}
	csvBatchBody34, _ := json.Marshal(csvBatch34)
	request("POST", "/api/v1/catalog/barcodes/batches/apply", string(csvBatchBody34), tokenA, 200)
	csvVariant34 := csv34
	csvVariant34.CSV = "\ufeff" + strings.ReplaceAll(csv34.CSV, "\n", "\r\n")
	variantBody34, _ := json.Marshal(csvVariant34)
	request("POST", "/api/v1/catalog/barcodes/import/apply", string(variantBody34), tokenA, 200)
	changedCSV34 := csv34
	changedCSV34.CSV = strings.Replace(csv34.CSV, gtin33("789123450010"), gtin33("789123450012"), 1)
	changedBody34, _ := json.Marshal(changedCSV34)
	request("POST", "/api/v1/catalog/barcodes/import/apply", string(changedBody34), tokenA, 409)
	reportCSV34 := csvRequest34(fmt.Sprintf("/api/v1/produtos/%d/barcodes/export.csv", csvID34a), tokenA, 200)
	if !strings.HasPrefix(reportCSV34, onlinecatalog.BarcodeCSVReportHeader) || !strings.Contains(reportCSV34, ";"+gtin33("789123450010")+";") {
		t.Fatal("conferência não exportou código real")
	}
	csvRequest34(fmt.Sprintf("/api/v1/produtos/%d/barcodes/export.csv", csvID34a), tokenB, 404)
	codeID34 := csvApplied34["items"].([]interface{})[0].(map[string]interface{})["after"].(map[string]interface{})["id"].(string)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes/%s/active", csvID34a, codeID34), fmt.Sprintf(`{"operation_id":%q,"expected_version":2,"expected_code_version":1,"ativo":false,"reason":"Etiqueta antiga"}`, uuid.NewString()), tokenA, 200)
	reportCSV34 = csvRequest34(fmt.Sprintf("/api/v1/produtos/%d/barcodes/export.csv", csvID34a), tokenA, 200)
	if !strings.Contains(reportCSV34, ";false;true\n") {
		t.Fatal("exportação ocultou inativo ou confundiu estados")
	}
	// Invalid rows expose only controlled line/field/code diagnostics, no raw text.
	badCSV34 := onlinecatalog.BarcodeCSVInput{OperationID: uuid.NewString(), Reason: "Conferência", CSV: onlinecatalog.BarcodeCSVHeader + fmt.Sprintf("%d;2;private-invalid\n", csvID34b)}
	badBody34, _ := json.Marshal(badCSV34)
	badResponse34 := request("POST", "/api/v1/catalog/barcodes/import/preview", string(badBody34), tokenA, 400)
	diag34 := badResponse34["csv_error"].(map[string]interface{})
	if diag34["line"] != float64(2) || diag34["code"] != "gtin" {
		t.Fatal("diagnóstico CSV incorreto")
	}
	encodedDiag34, _ := json.Marshal(badResponse34)
	if strings.Contains(string(encodedDiag34), "private-invalid") {
		t.Fatal("diagnóstico vazou conteúdo")
	}
	checkProduct(csvID34b, 2, 125, true)
	// Late failure through the CSV HTTP path must leave zero partial batch effects.
	csvRollbackID34a, csvRollbackID34b := newBatchProduct33("csv-gtin-rollback-a"), newBatchProduct33("csv-gtin-rollback-b")
	csvRollback34 := onlinecatalog.BarcodeCSVInput{OperationID: uuid.NewString(), Reason: "Rollback CSV", CSV: onlinecatalog.BarcodeCSVHeader + fmt.Sprintf("%d;1;%s\n%d;1;%s\n", csvRollbackID34a, gtin33("789123450013"), csvRollbackID34b, gtin33("789123450014"))}
	rollbackCSVBody34, _ := json.Marshal(csvRollback34)
	rollbackCSVPreview34 := request("POST", "/api/v1/catalog/barcodes/import/preview", string(rollbackCSVBody34), tokenA, 200)
	csvRollback34.PreviewHash = rollbackCSVPreview34["preview_hash"].(string)
	rollbackCSVBody34, _ = json.Marshal(csvRollback34)
	exec(`ALTER TABLE online_catalog_barcode_batch_outbox ENABLE TRIGGER refuse_barcode_batch_event`)
	request("POST", "/api/v1/catalog/barcodes/import/apply", string(rollbackCSVBody34), tokenA, 503)
	checkProduct(csvRollbackID34a, 1, 125, true)
	checkProduct(csvRollbackID34b, 1, 125, true)
	request("GET", "/api/v1/catalog/barcodes/batches/"+csvRollback34.OperationID, "", tokenA, 404)
	request("GET", "/api/v1/catalog/barcodes/lookup?code="+gtin33("789123450013"), "", tokenA, 404)
	exec(`ALTER TABLE online_catalog_barcode_batch_outbox DISABLE TRIGGER refuse_barcode_batch_event`)
	request("POST", "/api/v1/catalog/barcodes/import/apply", string(rollbackCSVBody34), tokenA, 200)
	checkProduct(csvRollbackID34a, 2, 125, true)
	checkProduct(csvRollbackID34b, 2, 125, true)
	var barcodeBatchChecksum33 string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_barcode_batch_migrations WHERE version=15`).Scan(&barcodeBatchChecksum33) != nil {
		t.Fatal("checksum lote ausente")
	}
	exec(`UPDATE online_catalog_barcode_batch_migrations SET version=16 WHERE version=15`)
	if onlinecatalog.MigrateBarcodeBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckBarcodeBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("versão futura lote aceita")
	}
	exec(`UPDATE online_catalog_barcode_batch_migrations SET version=15,checksum='tampered' WHERE version=16`)
	if onlinecatalog.MigrateBarcodeBatches(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("checksum lote alterado aceito")
	}
	exec(`UPDATE online_catalog_barcode_batch_migrations SET checksum=$1 WHERE version=15`, barcodeBatchChecksum33)
	var barcodeChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_barcode_migrations WHERE version=14`).Scan(&barcodeChecksum) != nil {
		t.Fatal("checksum código ausente")
	}
	exec(`UPDATE online_catalog_barcode_migrations SET version=15 WHERE version=14`)
	if onlinecatalog.MigrateBarcodes(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckBarcodes(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema código futuro aceito")
	}
	exec(`UPDATE online_catalog_barcode_migrations SET version=14,checksum='tampered' WHERE version=15`)
	if onlinecatalog.MigrateBarcodes(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema código adulterado aceito")
	}
	exec(`UPDATE online_catalog_barcode_migrations SET checksum=$1 WHERE version=14`, barcodeChecksum)
	var adjustmentChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_adjustment_migrations WHERE version=13`).Scan(&adjustmentChecksum) != nil {
		t.Fatal("checksum reajuste ausente")
	}
	exec(`UPDATE online_catalog_adjustment_migrations SET version=14 WHERE version=13`)
	if onlinecatalog.MigrateAdjustments(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckAdjustments(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema reajuste futuro aceito")
	}
	exec(`UPDATE online_catalog_adjustment_migrations SET version=13,checksum='tampered' WHERE version=14`)
	if onlinecatalog.MigrateAdjustments(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema reajuste adulterado aceito")
	}
	exec(`UPDATE online_catalog_adjustment_migrations SET checksum=$1 WHERE version=13`, adjustmentChecksum)
	var importsChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_import_migrations WHERE version=12`).Scan(&importsChecksum) != nil {
		t.Fatal("checksum CSV ausente")
	}
	exec(`UPDATE online_catalog_import_migrations SET version=13 WHERE version=12`)
	if onlinecatalog.MigrateImports(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckImports(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema CSV futuro aceito")
	}
	exec(`UPDATE online_catalog_import_migrations SET version=12,checksum='tampered' WHERE version=13`)
	if onlinecatalog.MigrateImports(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema CSV adulterado aceito")
	}
	exec(`UPDATE online_catalog_import_migrations SET checksum=$1 WHERE version=12`, importsChecksum)
	var undoChecksum string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_catalog_undo_migrations WHERE version=11`).Scan(&undoChecksum) != nil {
		t.Fatal("checksum de reversão ausente")
	}
	exec(`UPDATE online_catalog_undo_migrations SET version=12 WHERE version=11`)
	if onlinecatalog.MigrateUndo(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.CheckUndo(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema futuro da reversão aceito")
	}
	exec(`UPDATE online_catalog_undo_migrations SET version=11,checksum='tampered' WHERE version=12`)
	if onlinecatalog.MigrateUndo(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema adulterado da reversão aceito")
	}
	exec(`UPDATE online_catalog_undo_migrations SET checksum=$1 WHERE version=11`, undoChecksum)
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

	// Delivery35: five quality APIs against actual isolated PostgreSQL.
	qualityIDs35 := make([]int64, 0, 4)
	for i := 0; i < 4; i++ {
		description := ""
		if i == 1 || i == 3 {
			description = "Cadastro conferido"
		}
		stock := 5
		if i == 0 {
			stock = 0
		}
		created := request("POST", "/api/v1/produtos/", fmt.Sprintf(`{"nome":"Quality35 %d","sku":"quality35-%d","descricao":%q,"preco":1.25,"estoque":%d}`, i, i, description, stock), tokenA, 201)
		qualityIDs35 = append(qualityIDs35, int64(created["ID"].(float64)))
	}
	for i := 2; i < 4; i++ {
		request("POST", fmt.Sprintf("/api/v1/produtos/%d/barcodes", qualityIDs35[i]), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"code":%q,"reason":"Conferência qualidade"}`, uuid.NewString(), gtin33(fmt.Sprintf("78912345990%d", i))), tokenA, 200)
	}
	request("POST", "/api/v1/produtos/", `{"nome":"Quality35 foreign","sku":"quality35-foreign","preco":1.25}`, tokenB, 201)
	qualityBase35 := "/api/v1/catalog/quality/"
	summary35 := request("GET", qualityBase35+"summary?q=Quality35", "", tokenA, 200)
	if summary35["products"] != float64(4) || summary35["pending_products"] != float64(3) ||
		summary35["no_active_gtin"] != float64(2) || summary35["no_description"] != float64(2) || summary35["zero_stock"] != float64(1) {
		t.Fatal("contagens de qualidade incorretas")
	}
	foreignSummary35 := request("GET", qualityBase35+"summary?q=Quality35", "", tokenB, 200)
	if foreignSummary35["products"] != float64(1) {
		t.Fatal("qualidade vazou empresa")
	}
	for _, tc := range []struct {
		path  string
		count int
	}{{"no-active-gtin", 2}, {"no-description", 2}, {"zero-stock", 1}} {
		page35 := request("GET", qualityBase35+tc.path+"?q=Quality35&limit=1", "", tokenA, 200)
		if page35["total"] != float64(tc.count) || len(page35["items"].([]interface{})) != 1 {
			t.Fatal("lista de qualidade incompleta")
		}
		if tc.count == 2 {
			if page35["has_more"] != true || page35["next_offset"] != float64(1) {
				t.Fatal("paginação de qualidade falsa")
			}
			next35 := request("GET", qualityBase35+tc.path+"?q=Quality35&limit=1&offset=1", "", tokenA, 200)
			if next35["has_more"] != false || next35["items"].([]interface{})[0].(map[string]interface{})["id"] == page35["items"].([]interface{})[0].(map[string]interface{})["id"] {
				t.Fatal("página repetida")
			}
		}
	}
	empty35 := request("GET", qualityBase35+"zero-stock?q=Quality35&offset=99", "", tokenA, 200)
	if len(empty35["items"].([]interface{})) != 0 || empty35["has_more"] != false {
		t.Fatal("página vazia incorreta")
	}
	request("GET", qualityBase35+"summary?tenant_id="+tenantB, "", tokenA, 400)
	request("GET", qualityBase35+"no-active-gtin?issue=zero_stock", "", tokenA, 400)
	request("GET", qualityBase35+"summary?q=a&q=b", "", tokenA, 400)
	var qualityWritesBefore35, qualityWritesAfter35 int
	countQualityWrites35 := func() int {
		var n int
		if sqlDB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM online_catalog_operations WHERE tenant_id=$1)+(SELECT count(*) FROM online_catalog_barcode_operations WHERE tenant_id=$1)+(SELECT count(*) FROM online_catalog_barcode_outbox WHERE tenant_id=$1)`, tenantA).Scan(&n) != nil {
			t.Fatal("contagem de escritas indisponível")
		}
		return n
	}
	qualityWritesBefore35 = countQualityWrites35()
	exportReq35 := httptest.NewRequest("GET", qualityBase35+"export.csv?q=Quality35&limit=1", nil)
	exportReq35.Header.Set("Authorization", "Bearer "+tokenA)
	exportResp35, exportErr35 := app.Test(exportReq35, 10000)
	if exportErr35 != nil {
		t.Fatal("CSV qualidade indisponível")
	}
	exportData35, exportErr35 := io.ReadAll(exportResp35.Body)
	exportResp35.Body.Close()
	if exportErr35 != nil || exportResp35.StatusCode != 200 || !strings.HasPrefix(string(exportData35), "product_id;version;sku;nome;") ||
		exportResp35.Header.Get("Cache-Control") != "no-store" || exportResp35.Header.Get("X-Content-Type-Options") != "nosniff" ||
		exportResp35.Header.Get("X-Titan-Report-Total") != "3" || exportResp35.Header.Get("X-Titan-Report-Count") != "1" ||
		exportResp35.Header.Get("X-Titan-Report-Has-More") != "true" || exportResp35.Header.Get("X-Titan-Report-Next-Offset") != "1" || len(exportResp35.Header.Get("X-Titan-Source-Hash")) != 64 {
		t.Fatal("metadados CSV qualidade incorretos")
	}
	qualityWritesAfter35 = countQualityWrites35()
	if qualityWritesAfter35 != qualityWritesBefore35 {
		t.Fatal("consulta alterou catálogo")
	}
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/details", qualityIDs35[0]), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"nome":"Quality35 0","sku":"quality35-0","descricao":"Agora completo"}`, uuid.NewString()), tokenA, 200)
	corrected35 := request("GET", qualityBase35+"summary?q=Quality35", "", tokenA, 200)
	if corrected35["no_description"] != float64(1) || corrected35["pending_products"] != float64(3) {
		t.Fatal("pendência não acompanhou correção")
	}
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/active", qualityIDs35[1]), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"ativo":false}`, uuid.NewString()), tokenA, 200)
	active35 := request("GET", qualityBase35+"summary?q=Quality35", "", tokenA, 200)
	inactive35 := request("GET", qualityBase35+"summary?q=Quality35&active=inactive", "", tokenA, 200)
	if active35["products"] != float64(3) || inactive35["products"] != float64(1) {
		t.Fatal("filtro ativo incorreto")
	}
	request("POST", "/api/v1/produtos/", `{"nome":"Quality35 %literal","sku":"quality35-literal","preco":1.25}`, tokenA, 201)
	literal35 := request("GET", qualityBase35+"summary?q=%25literal", "", tokenA, 200)
	if literal35["products"] != float64(1) {
		t.Fatal("busca percentual não literal")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", qualityBase35+"summary", "", tokenA, 401)
	if _, err = store.QualitySummary(ctx, actor, onlinecatalog.QualityQuery{Active: "active", Issue: "any", Limit: 50}); err != onlinecatalog.ErrDenied {
		t.Fatal("qualidade aceitou sessão revogada")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)

	// Delivery36: durable human review; never changes prices, cash or stock.
	if onlinecatalog.MigrateDiscountReviews(ctx, sqlDB) != nil || onlinecatalog.MigrateDiscountReviews(ctx, sqlDB) != nil || onlinecatalog.CheckDiscountReviews(ctx, sqlDB) != nil {
		t.Fatal("migração de revisão de desconto falhou")
	}
	newReview36 := func(sku string) (int64, int64) {
		t.Helper()
		p := request("POST", "/api/v1/produtos/", fmt.Sprintf(`{"nome":"Revisão humana","sku":%q,"preco":10.00,"estoque":100}`, sku), tokenA, 201)
		pid := int64(p["ID"].(float64))
		var sid int64
		if sqlDB.QueryRowContext(ctx, `INSERT INTO discount_suggestions(tenant_id,product_id,suggested_discount,suggested_range,reason,criteria) VALUES($1,$2,15.00,'10% - 20%','Baixo giro','PRODUTO_PARADO') RETURNING id`, tenantA, pid).Scan(&sid) != nil {
			t.Fatal("sugestão de fixture indisponível")
		}
		return sid, pid
	}
	bodyReview36 := func(v onlinecatalog.DiscountReviewInput) string {
		raw, er := json.Marshal(v)
		if er != nil {
			t.Fatal("entrada de revisão inválida")
		}
		return string(raw)
	}
	reviewID36, productID36 := newReview36("discount-review-approve")
	reviewPath36 := fmt.Sprintf("/api/v1/discounts/suggestions/%d", reviewID36)
	current36 := request("GET", reviewPath36, "", tokenA, 200)
	if current36["discount_bps"] != float64(1500) || current36["status"] != "PENDING" {
		t.Fatal("detalhes da sugestão incorretos")
	}
	request("GET", reviewPath36, "", tokenB, 404)
	input36 := onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Conferência humana"}
	decision36 := request("POST", reviewPath36+"/approve", bodyReview36(input36), tokenA, 200)
	replay36 := request("POST", reviewPath36+"/approve", bodyReview36(input36), tokenA, 200)
	firstJSON36, _ := json.Marshal(decision36)
	replayJSON36, _ := json.Marshal(replay36)
	if string(firstJSON36) != string(replayJSON36) || decision36["after"].(map[string]interface{})["status"] != "APPROVED" {
		t.Fatal("replay duplicou ou mudou decisão")
	}
	request("GET", "/api/v1/discounts/reviews/"+input36.OperationID, "", tokenA, 200)
	request("GET", "/api/v1/discounts/reviews/"+input36.OperationID, "", tokenB, 404)
	request("GET", "/api/v1/discounts/reviews/"+input36.OperationID, "", otherToken, 404)
	changedInput36 := input36
	changedInput36.Reason = "Motivo diferente"
	request("POST", reviewPath36+"/approve", bodyReview36(changedInput36), tokenA, 409)
	secondDecision36 := input36
	secondDecision36.OperationID = uuid.NewString()
	request("POST", reviewPath36+"/reject", bodyReview36(secondDecision36), tokenA, 409)
	history36 := request("GET", reviewPath36+"/reviews?limit=1", "", tokenA, 200)
	if len(history36["items"].([]interface{})) != 1 {
		t.Fatal("histórico da decisão ausente")
	}
	request("GET", reviewPath36+"/reviews", "", tokenB, 404)
	emptyHistory36 := request("GET", reviewPath36+"/reviews?offset=1", "", tokenA, 200)
	if len(emptyHistory36["items"].([]interface{})) != 0 {
		t.Fatal("histórico paginado incorreto")
	}
	checkProduct(productID36, 1, 1000, true)
	var unchangedStock36 int
	if sqlDB.QueryRowContext(ctx, `SELECT estoque FROM products WHERE tenant_id=$1 AND id=$2`, tenantA, productID36).Scan(&unchangedStock36) != nil || unchangedStock36 != 100 {
		t.Fatal("aprovação alterou estoque")
	}
	rejectID36, _ := newReview36("discount-review-reject")
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/reject", rejectID36), bodyReview36(onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Não aplicar"}), tokenA, 200)
	staleID36, staleProduct36 := newReview36("discount-review-stale")
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", staleProduct36), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"price_cents":1100}`, uuid.NewString()), tokenA, 200)
	staleReview36 := onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Prévia obsoleta"}
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", staleID36), bodyReview36(staleReview36), tokenA, 409)
	staleReview36.ExpectedProductVersion = 2
	staleReview36.ExpectedDiscountBPS = 1000
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", staleID36), bodyReview36(staleReview36), tokenA, 409)
	rollbackID36, _ := newReview36("discount-review-rollback")
	rollbackReview36 := onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Recuperação explícita"}
	exec(fmt.Sprintf(`CREATE FUNCTION refuse_discount_review36() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated fixture'; END $$; CREATE TRIGGER refuse_discount_review36 BEFORE INSERT ON online_discount_review_outbox FOR EACH ROW WHEN(NEW.operation_id='%s'::uuid) EXECUTE FUNCTION refuse_discount_review36()`, rollbackReview36.OperationID))
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", rollbackID36), bodyReview36(rollbackReview36), tokenA, 503)
	request("GET", "/api/v1/discounts/reviews/"+rollbackReview36.OperationID, "", tokenA, 404)
	rollbackCurrent36 := request("GET", fmt.Sprintf("/api/v1/discounts/suggestions/%d", rollbackID36), "", tokenA, 200)
	if rollbackCurrent36["status"] != "PENDING" {
		t.Fatal("falha tardia deixou aprovação parcial")
	}
	exec(`ALTER TABLE online_discount_review_outbox DISABLE TRIGGER refuse_discount_review36`)
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", rollbackID36), bodyReview36(rollbackReview36), tokenA, 200)
	concurrentID36, _ := newReview36("discount-review-concurrent")
	concurrentReview36 := onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Repetição simultânea"}
	reviewErrors36 := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			_, e := store.ReviewDiscount(ctx, actor, concurrentID36, "approve", concurrentReview36)
			reviewErrors36 <- e
		}()
	}
	for i := 0; i < 4; i++ {
		if <-reviewErrors36 != nil {
			t.Fatal("repetição concorrente falhou")
		}
	}
	var reviewCounts36, eventCounts36 int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_discount_reviews WHERE tenant_id=$1 AND suggestion_id=$2`, tenantA, concurrentID36).Scan(&reviewCounts36) != nil ||
		sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_discount_review_outbox WHERE tenant_id=$1 AND operation_id=$2`, tenantA, concurrentReview36.OperationID).Scan(&eventCounts36) != nil ||
		reviewCounts36 != 1 || eventCounts36 != 1 {
		t.Fatal("repetição criou decisão/evento duplicado")
	}
	competingID36, _ := newReview36("discount-review-competing")
	competingInputs36 := []onlinecatalog.DiscountReviewInput{{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Aprovar"}, {OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Recusar"}}
	for i, action36 := range []string{"approve", "reject"} {
		go func(v onlinecatalog.DiscountReviewInput, action string) {
			_, e := store.ReviewDiscount(ctx, actor, competingID36, action, v)
			reviewErrors36 <- e
		}(competingInputs36[i], action36)
	}
	winners36, conflicts36 := 0, 0
	for i := 0; i < 2; i++ {
		e := <-reviewErrors36
		if e == nil {
			winners36++
		} else if e == onlinecatalog.ErrConflict {
			conflicts36++
		} else {
			t.Fatal("erro inesperado na disputa")
		}
	}
	if winners36 != 1 || conflicts36 != 1 {
		t.Fatal("decisões concorrentes sobrescreveram resultado")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", reviewPath36, "", tokenA, 401)
	if _, e := store.DiscountReview(ctx, actor, input36.OperationID); e != onlinecatalog.ErrDenied {
		t.Fatal("sessão revogada recuperou decisão")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	var reviewChecksum36 string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_discount_review_migrations WHERE version=16`).Scan(&reviewChecksum36) != nil {
		t.Fatal("histórico revisão ausente")
	}
	exec(`UPDATE online_discount_review_migrations SET version=17 WHERE version=16`)
	if onlinecatalog.CheckDiscountReviews(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.MigrateDiscountReviews(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema futuro de revisão aceito")
	}
	exec(`UPDATE online_discount_review_migrations SET version=16,checksum='changed' WHERE version=17`)
	if onlinecatalog.CheckDiscountReviews(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("checksum de revisão adulterado aceito")
	}
	exec(`UPDATE online_discount_review_migrations SET checksum=$1 WHERE version=16`, reviewChecksum36)
	if onlinecatalog.CheckDiscountReviews(ctx, sqlDB) != nil {
		t.Fatal("schema de revisão restaurado incompatível")
	}
	// Delivery37: five read-only report functions on existing schema16.
	reportBase37 := "/api/v1/discounts/report/"
	pendingID37, pendingProduct37 := newReview36("report37-pending")
	approvedID37, _ := newReview36("report37-approved")
	rejectedID37, _ := newReview36("report37-rejected")
	legacyID37, _ := newReview36("report37-legacy")
	exec(`UPDATE products SET nome='Report37 =text' WHERE tenant_id=$1 AND sku LIKE 'report37-%'`, tenantA)
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", approvedID37), bodyReview36(onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "=human approval"}), tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/reject", rejectedID37), bodyReview36(onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Human refusal"}), tokenA, 200)
	exec(`UPDATE discount_suggestions SET status='APPROVED',reviewed_by=$3 WHERE tenant_id=$1 AND id=$2`, tenantA, legacyID37, userA)
	countReports37 := func() int64 {
		var n int64
		if sqlDB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM online_discount_reviews)+(SELECT count(*) FROM online_discount_review_outbox)+(SELECT count(*) FROM online_catalog_operations)`).Scan(&n) != nil {
			t.Fatal("report counters unavailable")
		}
		return n
	}
	beforeReads37 := countReports37()
	summary37 := request("GET", reportBase37+"summary?q=report37", "", tokenA, 200)
	if summary37["total"] != float64(4) || summary37["pending"] != float64(1) || summary37["approved"] != float64(2) || summary37["rejected"] != float64(1) || summary37["final_without_receipt"] != float64(1) {
		t.Fatal("report counts incorrect")
	}
	foreign37 := request("GET", reportBase37+"summary?q=report37", "", tokenB, 200)
	if foreign37["total"] != float64(0) {
		t.Fatal("report tenant isolation failed")
	}
	for _, path37 := range []string{"pending", "approved", "rejected"} {
		p37 := request("GET", reportBase37+path37+"?q=report37&limit=1", "", tokenA, 200)
		if len(p37["items"].([]interface{})) != 1 || len(p37["source_hash"].(string)) != 64 {
			t.Fatal("report page missing")
		}
	}
	approved37 := request("GET", reportBase37+"approved?q=report37&limit=1", "", tokenA, 200)
	if approved37["total"] != float64(2) || approved37["has_more"] != true || approved37["next_offset"] != float64(1) {
		t.Fatal("report pagination incorrect")
	}
	legacyPage37 := request("GET", reportBase37+"approved?q=report37-legacy", "", tokenA, 200)
	legacyItem37 := legacyPage37["items"].([]interface{})[0].(map[string]interface{})
	if legacyItem37["has_receipt"] != false || legacyItem37["operation_id"] != "" || legacyItem37["review_reason"] != "" {
		t.Fatal("legacy receipt fabricated")
	}
	empty37 := request("GET", reportBase37+"pending?q=report37&offset=1", "", tokenA, 200)
	if len(empty37["items"].([]interface{})) != 0 || empty37["has_more"] != false {
		t.Fatal("report empty page incorrect")
	}
	request("GET", reportBase37+"pending?status=APPROVED", "", tokenA, 400)
	request("GET", reportBase37+"summary?q=a&q=b", "", tokenA, 400)
	req37 := httptest.NewRequest("GET", reportBase37+"export.csv?q=report37&status=APPROVED&limit=1", nil)
	req37.Header.Set("Authorization", "Bearer "+tokenA)
	resp37, e37 := app.Test(req37, 5000)
	if e37 != nil {
		t.Fatal("report csv transport")
	}
	data37, e37 := io.ReadAll(resp37.Body)
	resp37.Body.Close()
	if e37 != nil || resp37.StatusCode != 200 || !strings.HasPrefix(string(data37), "suggestion_id;product_id;") || !strings.Contains(string(data37), "'=human approval") || resp37.Header.Get("Cache-Control") != "no-store" || resp37.Header.Get("X-Titan-Report-Total") != "2" || resp37.Header.Get("X-Titan-Report-Count") != "1" || resp37.Header.Get("X-Titan-Report-Has-More") != "true" || resp37.Header.Get("X-Titan-Report-Scope") != "online-discount-suggestions" {
		t.Fatal("report csv metadata or formula protection failed")
	}
	if countReports37() != beforeReads37 {
		t.Fatal("report mutated data")
	}
	checkProduct(pendingProduct37, 1, 1000, true)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/active", pendingProduct37), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"ativo":false}`, uuid.NewString()), tokenA, 200)
	inactive37 := request("GET", reportBase37+"pending?q=report37&active=inactive", "", tokenA, 200)
	if inactive37["total"] != float64(1) || int64(inactive37["items"].([]interface{})[0].(map[string]interface{})["id"].(float64)) != pendingID37 {
		t.Fatal("inactive suggestion hidden or mismatched")
	}
	exec(`UPDATE products SET sku='report37-%literal' WHERE tenant_id=$1 AND id=$2`, tenantA, pendingProduct37)
	literal37 := request("GET", reportBase37+"summary?q=%25literal", "", tokenA, 200)
	if literal37["total"] != float64(1) {
		t.Fatal("report wildcard was not literal")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", reportBase37+"summary", "", tokenA, 401)
	if _, e := store.DiscountReport(ctx, actor, onlinecatalog.DiscountReportQuery{Active: "all", Status: "all", Limit: 50}); e != onlinecatalog.ErrDenied {
		t.Fatal("report accepted revoked session")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	// Delivery38: compare approved snapshots to the current product, without mutations.
	// A fresh HTTP harness gives this independent scenario its own rate-limit budget.
	// Database, sessions and prior assertions remain unchanged; production limits stay active.
	app = fiber.New()
	Registrar(app)
	checkBase38 := "/api/v1/discounts/checks/"
	currentID38, currentProduct38 := newReview36("checks38-current")
	priceID38, priceProduct38 := newReview36("checks38-%literal")
	inactiveID38, inactiveProduct38 := newReview36("checks38-inactive")
	versionID38, versionProduct38 := newReview36("checks38-version")
	legacyID38, _ := newReview36("checks38-legacy")
	exec(`UPDATE products SET nome='=Checks38' WHERE tenant_id=$1 AND sku LIKE 'checks38-%'`, tenantA)
	for _, id38 := range []int64{currentID38, priceID38, inactiveID38, versionID38} {
		request("POST", fmt.Sprintf("/api/v1/discounts/suggestions/%d/approve", id38), bodyReview36(onlinecatalog.DiscountReviewInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Conferência antes da alteração"}), tokenA, 200)
	}
	exec(`UPDATE discount_suggestions SET status='APPROVED',reviewed_by=$3 WHERE tenant_id=$1 AND id=$2`, tenantA, legacyID38, userA)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/price", priceProduct38), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"price_cents":1200}`, uuid.NewString()), tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/active", inactiveProduct38), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"ativo":false}`, uuid.NewString()), tokenA, 200)
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/details", versionProduct38), fmt.Sprintf(`{"operation_id":%q,"expected_version":1,"nome":"=Checks38","sku":"checks38-version","descricao":"Mudança de dados"}`, uuid.NewString()), tokenA, 200)
	writesBefore38 := countReports37()
	counts38 := request("GET", checkBase38+"summary?q=checks38", "", tokenA, 200)
	if counts38["total"] != float64(5) || counts38["attention"] != float64(4) || counts38["price_changed"] != float64(1) || counts38["version_changed"] != float64(3) || counts38["inactive"] != float64(1) || counts38["no_receipt"] != float64(1) {
		t.Fatal("approval check counts incorrect")
	}
	foreign38 := request("GET", checkBase38+"summary?q=checks38", "", tokenB, 200)
	if foreign38["total"] != float64(0) {
		t.Fatal("approval checks leaked company")
	}
	for _, test38 := range []struct {
		id     int64
		issues int
	}{{currentID38, 0}, {priceID38, 2}, {inactiveID38, 2}, {versionID38, 1}, {legacyID38, 1}} {
		r38 := request("GET", fmt.Sprintf("%ssuggestions/%d", checkBase38, test38.id), "", tokenA, 200)
		if len(r38["issues"].([]interface{})) != test38.issues {
			t.Fatal("approval issues mismatch")
		}
		if test38.id == legacyID38 {
			if _, ok := r38["approved_price_cents"]; ok || r38["has_receipt"] != false {
				t.Fatal("legacy snapshot invented")
			}
		} else if r38["approved_price_cents"] != float64(1000) || r38["approved_product_version"] != float64(1) {
			t.Fatal("approval snapshot overwritten")
		}
	}
	request("GET", fmt.Sprintf("%ssuggestions/%d", checkBase38, currentID38), "", tokenB, 404)
	request("GET", fmt.Sprintf("%ssuggestions/%d", checkBase38, pendingID37), "", tokenA, 404)
	prices38 := request("GET", checkBase38+"price-changed?q=checks38&limit=1", "", tokenA, 200)
	if prices38["total"] != float64(1) || prices38["items"].([]interface{})[0].(map[string]interface{})["id"] != float64(priceID38) {
		t.Fatal("price drift list incorrect")
	}
	inactive38 := request("GET", checkBase38+"inactive?q=checks38", "", tokenA, 200)
	if inactive38["total"] != float64(1) || inactive38["items"].([]interface{})[0].(map[string]interface{})["id"] != float64(inactiveID38) {
		t.Fatal("inactive approval list incorrect")
	}
	empty38 := request("GET", checkBase38+"inactive?q=checks38&offset=1", "", tokenA, 200)
	if len(empty38["items"].([]interface{})) != 0 || empty38["has_more"] != false {
		t.Fatal("check empty page incorrect")
	}
	literal38 := request("GET", checkBase38+"summary?q=%25literal", "", tokenA, 200)
	if literal38["total"] != float64(1) {
		t.Fatal("check search not literal")
	}
	req38 := httptest.NewRequest("GET", checkBase38+"export.csv?q=checks38&issue=any&limit=1", nil)
	req38.Header.Set("Authorization", "Bearer "+tokenA)
	resp38, e38 := app.Test(req38, 5000)
	if e38 != nil {
		t.Fatal("check csv transport")
	}
	data38, e38 := io.ReadAll(resp38.Body)
	resp38.Body.Close()
	if e38 != nil || resp38.StatusCode != 200 || !strings.Contains(string(data38), "'=Checks38") || !strings.HasPrefix(string(data38), "suggestion_id;product_id;") || resp38.Header.Get("X-Titan-Report-Total") != "4" || resp38.Header.Get("X-Titan-Report-Count") != "1" || resp38.Header.Get("X-Titan-Report-Has-More") != "true" || resp38.Header.Get("Cache-Control") != "no-store" || len(resp38.Header.Get("X-Titan-Source-Hash")) != 64 {
		t.Fatal("check csv metadata/protection failed")
	}
	if countReports37() != writesBefore38 {
		t.Fatal("approval checks mutated audit/outbox")
	}
	checkProduct(currentProduct38, 1, 1000, true)
	checkProduct(priceProduct38, 2, 1200, true)
	checkProduct(inactiveProduct38, 2, 1000, false)
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", checkBase38+"summary", "", tokenA, 401)
	if _, e := store.DiscountCheck(ctx, actor, currentID38); e != onlinecatalog.ErrDenied {
		t.Fatal("check accepted revoked session")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	// Delivery39: an independent HTTP budget, with unchanged production limits.
	app = fiber.New()
	Registrar(app)
	if onlinecatalog.MigrateCosts(ctx, sqlDB) != nil || onlinecatalog.MigrateCosts(ctx, sqlDB) != nil || onlinecatalog.CheckCosts(ctx, sqlDB) != nil {
		t.Fatal("migração de custos falhou")
	}
	_, costProduct39 := newReview36("cost39")
	costURL39 := fmt.Sprintf("/api/v1/produtos/%d/cost", costProduct39)
	marginURL39 := fmt.Sprintf("/api/v1/produtos/%d/margin", costProduct39)
	unknown39 := request("GET", costURL39, "", tokenA, 200)["cost"].(map[string]interface{})
	if unknown39["known"] != false || unknown39["cost_cents"] != nil || unknown39["revision"] != float64(0) {
		t.Fatal("custo desconhecido inventado")
	}
	if r := request("GET", marginURL39, "", tokenA, 200); r["state"] != "unknown_cost" || r["margin_bps"] != nil {
		t.Fatal("margem desconhecida inventada")
	}
	costOp39 := uuid.NewString()
	costBody39 := fmt.Sprintf(`{"operation_id":%q,"expected_product_version":1,"expected_cost_revision":0,"cost_cents":700,"reason":"Nota conferida"}`, costOp39)
	costReceipt39 := request("POST", costURL39, costBody39, tokenA, 200)
	if costReceipt39["after"].(map[string]interface{})["revision"] != float64(1) || costReceipt39["before"].(map[string]interface{})["known"] != false {
		t.Fatal("revisão inicial incorreta")
	}
	costReceiptURL39 := "/api/v1/catalog/costs/operations/" + costOp39
	for _, receipt39 := range []map[string]interface{}{request("POST", costURL39, costBody39, tokenA, 200), request("GET", costReceiptURL39, "", tokenA, 200)} {
		b, _ := json.Marshal(receipt39)
		a, _ := json.Marshal(costReceipt39)
		if string(a) != string(b) {
			t.Fatal("recibo perdido ou replay alterou snapshot")
		}
	}
	checkProduct(costProduct39, 1, 1000, true)
	if m := request("GET", marginURL39, "", tokenA, 200); m["margin_cents"] != float64(300) || m["margin_bps"] != "3000" {
		t.Fatal("margem incorreta")
	}
	request("POST", costURL39, strings.Replace(costBody39, `"cost_cents":700`, `"cost_cents":701`, 1), tokenA, 409)
	request("GET", costURL39, "", tokenB, 404)
	request("GET", costReceiptURL39, "", tokenB, 404)
	request("GET", costURL39+"/history", "", tokenB, 404)
	if h := request("GET", costURL39+"/history?limit=1&offset=0", "", tokenA, 200); len(h["items"].([]interface{})) != 1 {
		t.Fatal("histórico ausente")
	}
	if h := request("GET", costURL39+"/history?offset=1", "", tokenA, 200); len(h["items"].([]interface{})) != 0 {
		t.Fatal("paginação incorreta")
	}
	otherUser39, otherSession39 := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO users(id,tenant_id,role) VALUES($1,$2,'owner')`, otherUser39, tenantA)
	exec(`INSERT INTO online_sessions(id,tenant_id,user_id,role,created_at,expires_at) VALUES($1,$2,$3,'owner',clock_timestamp(),clock_timestamp()+interval '1 hour')`, otherSession39, tenantA, otherUser39)
	otherToken39 := token(tenantA, otherUser39, otherSession39)
	request("GET", costReceiptURL39, "", otherToken39, 404)
	request("POST", costURL39, costBody39, otherToken39, 409)
	staleCost39 := onlinecatalog.CostInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedCostRevision: 0, CostCents: 800, Reason: "Concorrência"}
	if _, e := store.SetCost(ctx, actor, costProduct39, staleCost39); e != onlinecatalog.ErrConflict {
		t.Fatal("revisão antiga aceita")
	}
	staleCost39.ExpectedCostRevision = 1
	staleCost39.ExpectedProductVersion = 2
	if _, e := store.SetCost(ctx, actor, costProduct39, staleCost39); e != onlinecatalog.ErrConflict {
		t.Fatal("versão antiga aceita")
	}
	// An outbox failure rolls back cost, immutable receipt and outbox together.
	rollbackCostOp39 := uuid.NewString()
	exec(`CREATE FUNCTION fail_cost_outbox39() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation_id::text='` + rollbackCostOp39 + `' THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END $$`)
	exec(`CREATE TRIGGER reject_cost39 BEFORE INSERT ON online_cost_outbox FOR EACH ROW EXECUTE FUNCTION fail_cost_outbox39()`)
	rollbackCost39 := onlinecatalog.CostInput{OperationID: rollbackCostOp39, ExpectedProductVersion: 1, ExpectedCostRevision: 1, CostCents: 1200, Reason: "Nova nota"}
	if _, e := store.SetCost(ctx, actor, costProduct39, rollbackCost39); e != onlinecatalog.ErrUnavailable {
		t.Fatal("falha tardia não interrompeu")
	}
	if c, e := store.ProductCost(ctx, actor, costProduct39); e != nil || c.Cost.Revision != 1 || *c.Cost.CostCents != 700 {
		t.Fatal("rollback perdeu custo anterior")
	}
	var countCost39 int
	if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_cost_operations WHERE tenant_id=$1 AND operation_id=$2`, tenantA, rollbackCostOp39).Scan(&countCost39) != nil || countCost39 != 0 {
		t.Fatal("rollback deixou recibo")
	}
	exec(`ALTER TABLE online_cost_outbox DISABLE TRIGGER reject_cost39`)
	if _, e := store.SetCost(ctx, actor, costProduct39, rollbackCost39); e != nil {
		t.Fatal("nova tentativa após rollback falhou")
	}
	if m := request("GET", marginURL39, "", tokenA, 200); m["state"] != "negative_margin" || m["margin_cents"] != float64(-200) || m["margin_bps"] != "-2000" {
		t.Fatal("margem negativa incorreta")
	}
	// Concurrent replay commits only one receipt and one durable event.
	concurrentCost39 := onlinecatalog.CostInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedCostRevision: 2, CostCents: 600, Reason: "Replay concorrente"}
	costErrors39 := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, e := store.SetCost(ctx, actor, costProduct39, concurrentCost39); costErrors39 <- e }()
	}
	for i := 0; i < 2; i++ {
		if e := <-costErrors39; e != nil {
			t.Fatal("replay concorrente falhou")
		}
	}
	if sqlDB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM online_cost_operations WHERE tenant_id=$1 AND operation_id=$2)+(SELECT count(*) FROM online_cost_outbox WHERE tenant_id=$1 AND operation_id=$2)`, tenantA, concurrentCost39.OperationID).Scan(&countCost39) != nil || countCost39 != 2 {
		t.Fatal("recibo ou evento duplicado")
	}
	// Competing operations with one expected revision have exactly one winner.
	for i := 0; i < 2; i++ {
		v := concurrentCost39
		v.OperationID = uuid.NewString()
		v.ExpectedCostRevision = 3
		v.CostCents = int64(500 + i)
		go func(v onlinecatalog.CostInput) {
			_, e := store.SetCost(ctx, actor, costProduct39, v)
			costErrors39 <- e
		}(v)
	}
	wonCost39, conflictCost39 := 0, 0
	for i := 0; i < 2; i++ {
		e := <-costErrors39
		if e == nil {
			wonCost39++
		} else if e == onlinecatalog.ErrConflict {
			conflictCost39++
		} else {
			t.Fatal("erro inesperado em concorrência")
		}
	}
	if wonCost39 != 1 || conflictCost39 != 1 {
		t.Fatal("última gravação venceu")
	}
	// Replay recovers the old receipt even after later costs and price changes.
	exec(`UPDATE products SET preco=0,catalog_version=catalog_version+1 WHERE tenant_id=$1 AND id=$2`, tenantA, costProduct39)
	request("POST", costURL39, costBody39, tokenA, 200)
	if m := request("GET", marginURL39, "", tokenA, 200); m["state"] != "zero_price" || m["margin_bps"] != nil {
		t.Fatal("percentual com preço zero")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", costURL39, "", tokenA, 401)
	if _, e := store.CostOperation(ctx, actor, costOp39); e != onlinecatalog.ErrDenied {
		t.Fatal("sessão revogada consultou recibo")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	var checksumCost39 string
	if sqlDB.QueryRowContext(ctx, `SELECT checksum FROM online_cost_migrations WHERE version=17`).Scan(&checksumCost39) != nil {
		t.Fatal("histórico 17 ausente")
	}
	exec(`UPDATE online_cost_migrations SET checksum='invalid' WHERE version=17`)
	if onlinecatalog.CheckCosts(ctx, sqlDB) != onlinecatalog.ErrUnavailable || onlinecatalog.MigrateCosts(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("checksum alterado aceito")
	}
	exec(`UPDATE online_cost_migrations SET checksum=$1 WHERE version=17`, checksumCost39)
	exec(`INSERT INTO online_cost_migrations(version,checksum) VALUES(18,$1)`, checksumCost39)
	if onlinecatalog.MigrateCosts(ctx, sqlDB) != onlinecatalog.ErrUnavailable {
		t.Fatal("schema futuro aceito")
	}
	exec(`DELETE FROM online_cost_migrations WHERE version=18`)
	if onlinecatalog.CheckCosts(ctx, sqlDB) != nil {
		t.Fatal("schema 17 não recuperado")
	}
	// Delivery40: independent HTTP budget, unchanged real limiter and shared DB.
	app = fiber.New()
	Registrar(app)
	marginBase40 := "/api/v1/catalog/margins/"
	marginIDs40 := make([]int64, 5)
	for i := range marginIDs40 {
		sku40 := fmt.Sprintf("margin40-%d", i)
		if i == 0 {
			sku40 = "escape40-%literal"
		}
		body40 := fmt.Sprintf(`{"nome":%q,"sku":%q,"preco":10.00}`, fmt.Sprintf("=Margin40 %d", i), sku40)
		p40 := request("POST", "/api/v1/produtos/", body40, tokenA, 201)
		marginIDs40[i] = int64(p40["ID"].(float64))
		if i > 0 {
			cost40 := []int64{0, 600, 1200, 800, 100}[i]
			v40 := onlinecatalog.CostInput{OperationID: uuid.NewString(), ExpectedProductVersion: 1, ExpectedCostRevision: 0, CostCents: cost40, Reason: "Nota Margin40"}
			if _, e := store.SetCost(ctx, actor, marginIDs40[i], v40); e != nil {
				t.Fatal("custo de fixture40 falhou")
			}
		}
	}
	exec(`UPDATE products SET preco=0 WHERE tenant_id=$1 AND id=$2`, tenantA, marginIDs40[4])
	request("POST", "/api/v1/produtos/", `{"nome":"Margin40 outra empresa","sku":"margin40-foreign","preco":10.00}`, tokenB, 201)
	// Unique prefix isolates this assertion from earlier %literal fixtures.
	// The decoy must not match: treating % as a wildcard would return two rows.
	request("POST", "/api/v1/produtos/", `{"nome":"Escape40 decoy","sku":"escape40-Xliteral","preco":10.00}`, tokenA, 201)
	writes40 := func() string {
		t.Helper()
		var h string
		if sqlDB.QueryRowContext(ctx, `SELECT md5(jsonb_build_array((SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM products p),(SELECT jsonb_agg(to_jsonb(c) ORDER BY tenant_id,product_id) FROM online_product_costs c),(SELECT count(*) FROM online_cost_operations),(SELECT count(*) FROM online_cost_outbox))::text)`).Scan(&h) != nil {
			t.Fatal("verificação de leituras40 falhou")
		}
		return h
	}
	beforeReads40 := writes40()
	summary40 := request("GET", marginBase40+"summary?q=Margin40", "", tokenA, 200)
	if summary40["total"] != float64(5) || summary40["known_cost"] != float64(4) || summary40["missing_cost"] != float64(1) || summary40["negative_margin"] != float64(2) || summary40["below_target"] != float64(2) || summary40["zero_price"] != float64(1) {
		t.Fatal("contagens de margem incorretas")
	}
	foreignSummary40 := request("GET", marginBase40+"summary?q=Margin40", "", tokenB, 200)
	if foreignSummary40["total"] != float64(1) || foreignSummary40["known_cost"] != float64(0) {
		t.Fatal("resumo atravessou empresa")
	}
	missing40 := request("GET", marginBase40+"missing-cost?q=Margin40", "", tokenA, 200)
	if missing40["total"] != float64(1) || missing40["items"].([]interface{})[0].(map[string]interface{})["state"] != "unknown_cost" {
		t.Fatal("custo desconhecido inventado")
	}
	literal40 := request("GET", marginBase40+"missing-cost?q=escape40-%25literal", "", tokenA, 200)
	if literal40["total"] != float64(1) {
		t.Fatal("busca não escapou porcentagem literal")
	}
	negative40 := request("GET", marginBase40+"negative?q=Margin40&limit=1", "", tokenA, 200)
	if negative40["total"] != float64(2) || negative40["has_more"] != true || negative40["next_offset"] != float64(1) {
		t.Fatal("paginação negativa incorreta")
	}
	negativeNext40 := request("GET", marginBase40+"negative?q=Margin40&limit=1&offset=1", "", tokenA, 200)
	if negativeNext40["has_more"] != false || negativeNext40["items"].([]interface{})[0].(map[string]interface{})["state"] != "zero_price" {
		t.Fatal("preço zero omitido")
	}
	if r40 := request("GET", marginBase40+"below-target?q=Margin40&target_bps=0", "", tokenA, 200); r40["total"] != float64(1) {
		t.Fatal("meta zero incluiu preço zero ou custo desconhecido")
	}
	if r40 := request("GET", marginBase40+"below-target?q=Margin40&target_bps=4000", "", tokenA, 200); r40["total"] != float64(2) {
		t.Fatal("margem igual à meta entrou")
	}
	if r40 := request("GET", marginBase40+"negative?q=Margin40&offset=20", "", tokenA, 200); len(r40["items"].([]interface{})) != 0 || r40["has_more"] != false {
		t.Fatal("página vazia fabricada")
	}
	req40 := httptest.NewRequest("GET", marginBase40+"export.csv?q=Margin40&limit=2", nil)
	req40.Header.Set("Authorization", "Bearer "+tokenA)
	resp40, e40 := app.Test(req40, 10000)
	if e40 != nil {
		t.Fatal("CSV40 indisponível")
	}
	data40, e40 := io.ReadAll(resp40.Body)
	resp40.Body.Close()
	if e40 != nil || resp40.StatusCode != 200 || !strings.HasPrefix(string(data40), "product_id;sku;nome;") || !strings.Contains(string(data40), "'=Margin40") || !strings.Contains(string(data40), ";unknown_cost") || resp40.Header.Get("X-Titan-Report-Total") != "5" || resp40.Header.Get("X-Titan-Report-Count") != "2" || resp40.Header.Get("X-Titan-Report-Has-More") != "true" || resp40.Header.Get("Cache-Control") != "no-store" || len(resp40.Header.Get("X-Titan-Source-Hash")) != 64 {
		t.Fatal("CSV40 sem isolamento, proteção ou metadados")
	}
	if writes40() != beforeReads40 {
		t.Fatal("relatório alterou dados")
	}
	exec(`UPDATE products SET ativo=false WHERE tenant_id=$1 AND id=$2`, tenantA, marginIDs40[3])
	if r40 := request("GET", marginBase40+"below-target?q=Margin40&active=false", "", tokenA, 200); r40["total"] != float64(1) {
		t.Fatal("filtro de inativos incorreto")
	}
	exec(`UPDATE products SET deleted_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2`, tenantA, marginIDs40[1])
	if r40 := request("GET", marginBase40+"summary?q=Margin40", "", tokenA, 200); r40["total"] != float64(4) {
		t.Fatal("produto excluído em resumo")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", marginBase40+"summary?q=Margin40", "", tokenA, 401)
	q40 := onlinecatalog.MarginQuery{Active: "all", Issue: "all", TargetBPS: 3000, Limit: 50}
	if _, e := store.Margins(ctx, actor, q40); e != onlinecatalog.ErrDenied {
		t.Fatal("sessão revogada em relatório")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	// Delivery41: own HTTP budget; immutable cost history, no live-product joins.
	app = fiber.New()
	Registrar(app)
	historyBase41 := "/api/v1/catalog/cost-history/"
	p41 := request("POST", "/api/v1/produtos/", `{"nome":"=CostHistory41 original","sku":"history41-%literal","preco":10}`, tokenA, 201)
	id41 := int64(p41["ID"].(float64))
	ops41 := make([]string, 4)
	for i, cost41 := range []int64{700, 800, 600, 600} {
		ops41[i] = uuid.NewString()
		request("POST", fmt.Sprintf("/api/v1/produtos/%d/cost", id41), fmt.Sprintf(`{"operation_id":%q,"expected_product_version":1,"expected_cost_revision":%d,"cost_cents":%d,"reason":"=Invoice41"}`, ops41[i], i, cost41), tokenA, 200)
	}
	pSecond41 := request("POST", "/api/v1/produtos/", `{"nome":"CostHistory41 second","sku":"history41-Xliteral","preco":10}`, tokenA, 201)
	idSecond41 := int64(pSecond41["ID"].(float64))
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/cost", idSecond41), fmt.Sprintf(`{"operation_id":%q,"expected_product_version":1,"expected_cost_revision":0,"cost_cents":0,"reason":"Second invoice"}`, uuid.NewString()), otherToken39, 200)
	pForeign41 := request("POST", "/api/v1/produtos/", `{"nome":"CostHistory41 foreign","sku":"history41-foreign","preco":10}`, tokenB, 201)
	foreignID41 := int64(pForeign41["ID"].(float64))
	request("POST", fmt.Sprintf("/api/v1/produtos/%d/cost", foreignID41), fmt.Sprintf(`{"operation_id":%q,"expected_product_version":1,"expected_cost_revision":0,"cost_cents":500,"reason":"Foreign invoice"}`, uuid.NewString()), tokenB, 200)
	beforeReads41 := writes40()
	summary41 := request("GET", historyBase41+"summary?q=CostHistory41", "", tokenA, 200)
	if summary41["total"] != float64(5) || summary41["initial"] != float64(2) || summary41["increased"] != float64(1) || summary41["decreased"] != float64(1) || summary41["unchanged"] != float64(1) || summary41["products"] != float64(2) || summary41["actors"] != float64(2) {
		t.Fatal("histórico41 sem partição correta")
	}
	page41 := request("GET", historyBase41+"entries?q=CostHistory41&limit=2", "", tokenA, 200)
	if page41["total"] != float64(5) || page41["has_more"] != true || page41["next_offset"] != float64(2) || len(page41["items"].([]interface{})) != 2 {
		t.Fatal("paginação41 incorreta")
	}
	literal41 := request("GET", historyBase41+"entries?q=history41-%25literal", "", tokenA, 200)
	if literal41["total"] != float64(4) {
		t.Fatal("busca41 não literal")
	}
	for _, change41 := range []string{"increased", "decreased", "unchanged"} {
		r41 := request("GET", historyBase41+"entries?q=CostHistory41&change="+change41, "", tokenA, 200)
		if r41["total"] != float64(1) || r41["items"].([]interface{})[0].(map[string]interface{})["change"] != change41 {
			t.Fatal("classificação41 incorreta")
		}
	}
	detail41 := request("GET", historyBase41+"entries/"+ops41[0], "", tokenA, 200)
	request("GET", historyBase41+"entries/"+ops41[0], "", otherToken39, 200)
	request("GET", "/api/v1/catalog/costs/operations/"+ops41[0], "", otherToken39, 404)
	if detail41["delta_cents"] != nil || detail41["before"].(map[string]interface{})["cost_cents"] != nil {
		t.Fatal("custo inicial anterior inventado")
	}
	request("GET", historyBase41+"entries/"+ops41[0], "", tokenB, 404)
	request("GET", historyBase41+"entries/"+uuid.NewString(), "", tokenA, 404)
	productPage41 := request("GET", fmt.Sprintf("%sproducts/%d?limit=2&offset=2", historyBase41, id41), "", tokenA, 200)
	if productPage41["total"] != float64(4) || productPage41["has_more"] != false || len(productPage41["items"].([]interface{})) != 2 {
		t.Fatal("página por produto41 incorreta")
	}
	if r41 := request("GET", fmt.Sprintf("%sproducts/%d", historyBase41, id41), "", tokenB, 200); r41["total"] != float64(0) {
		t.Fatal("histórico por produto vazou empresa")
	}
	if r41 := request("GET", historyBase41+"summary?q=CostHistory41", "", tokenB, 200); r41["total"] != float64(1) {
		t.Fatal("resumo41 vazou empresa")
	}
	if r41 := request("GET", historyBase41+"entries?q=CostHistory41&actor_id="+userB, "", tokenA, 200); r41["total"] != float64(0) {
		t.Fatal("filtro de ator41 atravessou empresa")
	}
	if r41 := request("GET", historyBase41+"entries?q=CostHistory41&from=2000-01-01T00:00:00Z&until=2001-01-01T00:00:00Z", "", tokenA, 200); r41["total"] != float64(0) {
		t.Fatal("intervalo41 ignorado")
	}
	// From is inclusive, until is exclusive, both at the receipt's exact timestamp.
	boundary41 := detail41["created_at"].(string)
	if r41 := request("GET", fmt.Sprintf("%sentries?product_id=%d&from=%s", historyBase41, id41, boundary41), "", tokenA, 200); r41["total"] != float64(4) {
		t.Fatal("início41 não inclusivo")
	}
	if r41 := request("GET", fmt.Sprintf("%sentries?product_id=%d&until=%s", historyBase41, id41, boundary41), "", tokenA, 200); r41["total"] != float64(0) {
		t.Fatal("fim41 não exclusivo")
	}
	req41 := httptest.NewRequest("GET", historyBase41+"export.csv?q=CostHistory41&limit=2", nil)
	req41.Header.Set("Authorization", "Bearer "+tokenA)
	resp41, e41 := app.Test(req41, 10000)
	if e41 != nil {
		t.Fatal("CSV41 indisponível")
	}
	data41, e41 := io.ReadAll(resp41.Body)
	resp41.Body.Close()
	if e41 != nil || resp41.StatusCode != 200 || !strings.HasPrefix(string(data41), "operation_id;product_id;") || !strings.Contains(string(data41), "'=CostHistory41") || resp41.Header.Get("X-Titan-Report-Total") != "5" || resp41.Header.Get("X-Titan-Report-Count") != "2" || resp41.Header.Get("X-Titan-Report-Has-More") != "true" || resp41.Header.Get("Cache-Control") != "no-store" || len(resp41.Header.Get("X-Titan-Source-Hash")) != 64 {
		t.Fatal("CSV41 sem proteção ou metadados")
	}
	if writes40() != beforeReads41 {
		t.Fatal("consulta41 alterou dados")
	}
	exec(`UPDATE products SET nome='Renamed41',sku='renamed41',ativo=false,deleted_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2`, tenantA, id41)
	if r41 := request("GET", historyBase41+"entries?q=history41-%25literal", "", tokenA, 200); r41["total"] != float64(4) {
		t.Fatal("histórico41 dependeu do produto atual")
	}
	if r41 := request("GET", historyBase41+"entries/"+ops41[0], "", tokenA, 200); r41["product"].(map[string]interface{})["nome"] != "=CostHistory41 original" {
		t.Fatal("snapshot41 reescrito")
	}
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, sessionA)
	request("GET", historyBase41+"summary", "", tokenA, 401)
	if _, e := store.CostReport(ctx, actor, onlinecatalog.CostReportQuery{Change: "all", Limit: 50}); e != onlinecatalog.ErrDenied {
		t.Fatal("sessão revogada41 consultou histórico")
	}
	exec(`UPDATE online_sessions SET revoked_at=NULL WHERE id=$1`, sessionA)
	// Delivery42: online stock read models; no SQLite/POS movement or forecast writes.
	app = fiber.New()
	Registrar(app)
	stockBase42 := "/api/v1/catalog/stock/"
	stockIDs42 := make([]int64, 5)
	stockValues42 := []int64{0, 4, 7, 10, 1}
	minimumValues42 := []int64{5, 5, 7, 0, 0}
	demandValues42 := []string{"2.0000", "1.0000", "1.0000", "0.0000", "0.1429"}
	for i := range stockIDs42 {
		sku42 := fmt.Sprintf("stock42-%d", i)
		if i == 0 {
			sku42 = "stock42-%literal"
		}
		if i == 1 {
			sku42 = "stock42-Xliteral"
		}
		p42 := request("POST", "/api/v1/produtos/", fmt.Sprintf(`{"nome":%q,"sku":%q,"preco":10}`, fmt.Sprintf("=StockAnalysis42 %d", i), sku42), tokenA, 201)
		stockIDs42[i] = int64(p42["ID"].(float64))
		exec(`UPDATE products SET estoque=$3,estoque_minimo=$4,ponto_reposicao=$4,demanda_media_diaria=$5::numeric WHERE tenant_id=$1 AND id=$2`, tenantA, stockIDs42[i], stockValues42[i], minimumValues42[i], demandValues42[i])
	}
	pForeign42 := request("POST", "/api/v1/produtos/", `{"nome":"StockAnalysis42 foreign","sku":"stock42-foreign","preco":10}`, tokenB, 201)
	_ = pForeign42
	beforeReads42 := writes40()
	summary42 := request("GET", stockBase42+"summary?q=StockAnalysis42", "", tokenA, 200)
	if summary42["products"] != float64(5) || summary42["zero_stock"] != float64(1) || summary42["below_minimum"] != float64(2) || summary42["low_coverage"] != float64(3) || summary42["no_positive_demand"] != float64(1) {
		t.Fatal("resumo42 incorreto")
	}
	zero42 := request("GET", stockBase42+"zero?q=StockAnalysis42", "", tokenA, 200)
	if zero42["total"] != float64(1) || zero42["items"].([]interface{})[0].(map[string]interface{})["stock_units"] != float64(0) {
		t.Fatal("estoque zero42 incorreto")
	}
	literal42 := request("GET", stockBase42+"coverage?q=stock42-%25literal", "", tokenA, 200)
	if literal42["total"] != float64(1) {
		t.Fatal("busca42 tratou porcentagem como curinga")
	}
	below42 := request("GET", stockBase42+"below-minimum?q=StockAnalysis42&limit=1", "", tokenA, 200)
	if below42["total"] != float64(2) || below42["has_more"] != true || below42["next_offset"] != float64(1) || below42["items"].([]interface{})[0].(map[string]interface{})["missing_to_minimum"] != float64(5) {
		t.Fatal("paginação e falta42 incorretas")
	}
	belowNext42 := request("GET", stockBase42+"below-minimum?q=StockAnalysis42&limit=1&offset=1", "", tokenA, 200)
	if belowNext42["has_more"] != false || belowNext42["items"].([]interface{})[0].(map[string]interface{})["missing_to_minimum"] != float64(1) {
		t.Fatal("segunda página42 incorreta")
	}
	coverage42 := request("GET", stockBase42+"coverage?q=StockAnalysis42", "", tokenA, 200)
	items42 := coverage42["items"].([]interface{})
	if len(items42) != 5 || items42[2].(map[string]interface{})["coverage_days"] != "7.0000" || items42[2].(map[string]interface{})["low_coverage"] != false || items42[3].(map[string]interface{})["coverage_days"] != nil || items42[3].(map[string]interface{})["coverage_state"] != "no_positive_demand" || items42[4].(map[string]interface{})["coverage_days"] != "6.9979" || items42[4].(map[string]interface{})["low_coverage"] != true {
		t.Fatal("cobertura42 inventada ou arredondada para classificação")
	}
	if r42 := request("GET", stockBase42+"summary?q=StockAnalysis42&horizon_days=6", "", tokenA, 200); r42["low_coverage"] != float64(2) {
		t.Fatal("horizonte42 ignorado")
	}
	if r42 := request("GET", stockBase42+"coverage?q=StockAnalysis42&offset=50", "", tokenA, 200); len(r42["items"].([]interface{})) != 0 || r42["has_more"] != false {
		t.Fatal("página vazia42 fabricada")
	}
	if r42 := request("GET", stockBase42+"summary?q=StockAnalysis42", "", tokenB, 200); r42["products"] != float64(1) || r42["no_positive_demand"] != float64(1) {
		t.Fatal("resumo42 atravessou empresa")
	}
	stockSession42 := uuid.NewString()
	exec(`INSERT INTO online_sessions(id,tenant_id,user_id,role,created_at,expires_at) VALUES($1,$2,$3,'stock',clock_timestamp(),clock_timestamp()+interval '1 hour')`, stockSession42, tenantA, stockUser)
	stockToken42, errStock42 := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": stockUser, "tenant_id": tenantA, "role": "stock", "sid": stockSession42, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("catalog-fixture-only-secret"))
	if errStock42 != nil {
		t.Fatal("token stock42 indisponível")
	}
	request("GET", stockBase42+"summary?q=StockAnalysis42", "", stockToken42, 200)
	req42 := httptest.NewRequest("GET", stockBase42+"export.csv?q=StockAnalysis42&limit=2", nil)
	req42.Header.Set("Authorization", "Bearer "+stockToken42)
	resp42, e42 := app.Test(req42, 10000)
	if e42 != nil {
		t.Fatal("CSV42 indisponível")
	}
	data42, e42 := io.ReadAll(resp42.Body)
	resp42.Body.Close()
	if e42 != nil || resp42.StatusCode != 200 || !strings.HasPrefix(string(data42), "product_id;sku;nome;") || !strings.Contains(string(data42), "'=StockAnalysis42") || resp42.Header.Get("X-Titan-Report-Total") != "5" || resp42.Header.Get("X-Titan-Report-Count") != "2" || resp42.Header.Get("X-Titan-Report-Has-More") != "true" || resp42.Header.Get("Cache-Control") != "no-store" || len(resp42.Header.Get("X-Titan-Source-Hash")) != 64 {
		t.Fatal("CSV42 sem proteção ou metadados")
	}
	if writes40() != beforeReads42 {
		t.Fatal("relatório42 alterou estoque ou registros")
	}
	exec(`UPDATE products SET ativo=false WHERE tenant_id=$1 AND id=$2`, tenantA, stockIDs42[2])
	if r42 := request("GET", stockBase42+"summary?q=StockAnalysis42", "", tokenA, 200); r42["products"] != float64(4) {
		t.Fatal("ativo padrão42 incorreto")
	}
	if r42 := request("GET", stockBase42+"summary?q=StockAnalysis42&active=false", "", tokenA, 200); r42["products"] != float64(1) {
		t.Fatal("inativos42 ignorados")
	}
	exec(`UPDATE products SET deleted_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2`, tenantA, stockIDs42[3])
	if r42 := request("GET", stockBase42+"summary?q=StockAnalysis42&active=all", "", tokenA, 200); r42["products"] != float64(4) || r42["no_positive_demand"] != float64(0) {
		t.Fatal("excluído42 incluído")
	}
	// Invalid persisted demand must fail closed, not silently become zero demand.
	exec(`UPDATE products SET demanda_media_diaria=-1 WHERE tenant_id=$1 AND id=$2`, tenantA, stockIDs42[0])
	request("GET", stockBase42+"summary?q=StockAnalysis42", "", tokenA, 503)
	request("GET", stockBase42+"coverage?q=StockAnalysis42", "", tokenA, 503)
	exec(`UPDATE products SET demanda_media_diaria=2 WHERE tenant_id=$1 AND id=$2`, tenantA, stockIDs42[0])
	exec(`UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, stockSession42)
	request("GET", stockBase42+"summary", "", stockToken42, 401)
	stockReportActor42 := onlinecatalog.Actor{Tenant: tenantA, User: stockUser, Session: stockSession42, Role: "stock"}
	if _, e := store.StockReport(ctx, stockReportActor42, onlinecatalog.StockReportQuery{Active: "true", Issue: "all", HorizonDays: 7, Limit: 50}); e != onlinecatalog.ErrDenied {
		t.Fatal("sessão revogada42 consultou estoque")
	}
	// Schema and data remain for inspection. No DROP, TRUNCATE or cleanup SQL.
}
