package onlinesessions

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Opt-in only: a loopback PostgreSQL database named *_test. No .env is loaded.
// All fixtures live in a new schema; this never initializes the application's
// public schema or executes the destructive historical migration.
func TestPostgresDurableRotationConcurrencyAndRollback(t *testing.T) {
	raw := os.Getenv("TITAN_SESSION_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("PostgreSQL descartável não configurado; defina TITAN_SESSION_TEST_DATABASE_URL")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("pool de teste indisponível")
	}
	defer pool.Close()
	schema := "titan_sessions_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("schema de teste indisponível")
	}
	// Leave this isolated schema intact for inspection; no DROP statements.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	testPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("pool isolado indisponível")
	}
	defer testPool.Close()
	db := stdlib.OpenDBFromPool(testPool)
	defer db.Close()
	if _, err = db.ExecContext(ctx, `CREATE TABLE tenants(id UUID PRIMARY KEY,status TEXT NOT NULL); CREATE TABLE users(id UUID PRIMARY KEY,tenant_id UUID NOT NULL REFERENCES tenants(id),name TEXT NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL)`); err != nil {
		t.Fatal("fixture inválida")
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO tenants VALUES ($1,'active')`, testTenant); err != nil {
		t.Fatal("tenant fixture inválido")
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users VALUES ($1,$2,'Owner','hash','owner')`, testUser, testTenant); err != nil {
		t.Fatal("user fixture inválido")
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("migração repetida", err)
	}
	if err = CheckSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db, "test-only-key")
	scope := Scope{User: testUser, Tenant: testTenant, Role: "owner"}
	first, err := s.Create(ctx, scope, "hash")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := ParseRefresh(first.Refresh)
	scope.Session = id
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Rotate(ctx, first.Refresh); results <- err }()
	}
	wg.Wait()
	close(results)
	ok, denied := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if err == ErrDenied {
			denied++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || denied != 1 {
		t.Fatal("uso concorrente não foi serializado")
	}
	var revoked bool
	if db.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM online_sessions WHERE id=$1`, id).Scan(&revoked) != nil || !revoked {
		t.Fatal("replay não revogou família")
	}
	second, err := s.Create(ctx, Scope{User: testUser, Tenant: testTenant, Role: "owner"}, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `CREATE FUNCTION reject_rotation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_rotation BEFORE INSERT ON online_session_audit FOR EACH ROW WHEN (NEW.event='rotated') EXECUTE FUNCTION reject_rotation()`); err != nil {
		t.Fatal("trigger de falha inválido")
	}
	if pair, err := s.Rotate(ctx, second.Refresh); err != ErrUnavailable || pair.Access != "" {
		t.Fatal("falha de auditoria emitiu token")
	}
	var unused bool
	if db.QueryRowContext(ctx, `SELECT consumed_at IS NULL FROM online_refresh_tokens WHERE digest=$1`, tokenDigest(second.Refresh)).Scan(&unused) != nil || !unused {
		t.Fatal("consumo não sofreu rollback")
	}
	if _, err = db.ExecContext(ctx, `ALTER TABLE online_session_audit DISABLE TRIGGER reject_rotation`); err != nil {
		t.Fatal("não desativou trigger de teste")
	}
	rotated, err := s.Rotate(ctx, second.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	secondID, _ := ParseRefresh(rotated.Refresh)
	scope.Session = secondID
	if err = s.Revoke(ctx, scope, secondID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Rotate(ctx, rotated.Refresh); err != ErrDenied {
		t.Fatal("logout não bloqueou refresh")
	}
	items, err := s.List(ctx, scope)
	if err != nil || len(items) != 2 {
		t.Fatal("histórico de sessões não persistiu")
	}
}
