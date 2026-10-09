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
	// Schema and data remain for inspection. No DROP, TRUNCATE or cleanup SQL.
}
