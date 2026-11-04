package authabuse_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"titansystem-backend/internal/authabuse"
)

func postgresBudget(t *testing.T) (authabuse.Store, string) {
	t.Helper()
	raw := os.Getenv("TITAN_SESSION_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable loopback PostgreSQL *_test; TITAN_SESSION_TEST_DATABASE_URL missing")
	}
	cfg, e := pgx.ParseConfig(raw)
	if e != nil {
		t.Fatal("invalid disposable URL")
	}
	ip := net.ParseIP(cfg.Host)
	if !strings.HasSuffix(cfg.Database, "_test") || (cfg.Host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		t.Fatal("requires loopback *_test")
	}
	base := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { base.Close() })
	schema := fmt.Sprintf("p08_budgets_%d", time.Now().UnixNano())
	if _, e := base.Exec(`CREATE SCHEMA ` + schema); e != nil {
		t.Fatal(e)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	if _, e := db.Exec(authabuse.Schema); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal("invalid disposable URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return authabuse.Store{DB: db, Postgres: true}, u.String()
}
func TestPostgresDurableBudgetConcurrencyRestartAndRollback(t *testing.T) {
	s, raw := postgresBudget(t)
	ctx := context.Background()
	out := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, e := sql.Open("pgx", raw)
			if e != nil {
				out <- e
				return
			}
			defer db.Close()
			out <- (authabuse.Store{DB: db, Postgres: true}).Take(ctx, "scope", "login", "peer", "")
		}()
	}
	wg.Wait()
	close(out)
	allowed := 0
	for e := range out {
		if e == nil {
			allowed++
		} else if !errors.Is(e, authabuse.ErrLimited) {
			t.Fatal(e)
		}
	}
	if allowed != 5 {
		t.Fatalf("admitted %d", allowed)
	}
	db, e := sql.Open("pgx", raw)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if !errors.Is((authabuse.Store{DB: db, Postgres: true}).Take(ctx, "scope", "login", "peer", ""), authabuse.ErrLimited) {
		t.Fatal("reopened connection bypass")
	}
	if e := s.Take(ctx, "other-tenant", "login", "peer", ""); e != nil {
		t.Fatal("scope leaked")
	}
	if _, e := s.DB.Exec(`CREATE FUNCTION reject_budget() RETURNS TRIGGER AS $$ BEGIN IF NEW.attempts=2 THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql; CREATE TRIGGER reject_budget BEFORE UPDATE ON auth_attempt_budgets FOR EACH ROW EXECUTE FUNCTION reject_budget()`); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(s.Take(ctx, "other-tenant", "login", "peer", ""), authabuse.ErrUnavailable) {
		t.Fatal("DB failure allowed")
	}
	var n int
	if e := s.DB.QueryRow(`SELECT max(attempts) FROM auth_attempt_budgets WHERE attempts<5`).Scan(&n); e != nil || n != 1 {
		t.Fatal("rollback not preserved")
	}
}
func TestPostgresBudgetProcessHelper(t *testing.T) {
	raw := os.Getenv("TITAN_P08_CHILD_PG")
	if raw == "" {
		return
	}
	db, e := sql.Open("pgx", raw)
	if e != nil {
		os.Exit(2)
	}
	s := authabuse.Store{DB: db, Postgres: true}
	n := 0
	for i := 0; i < 5; i++ {
		e = s.Take(context.Background(), "process", "login", "peer", "")
		if e == nil {
			n++
		} else if !errors.Is(e, authabuse.ErrLimited) {
			os.Exit(3)
		}
	}
	db.Close()
	fmt.Print(n)
	os.Exit(0)
}
func TestPostgresDurableBudgetAcrossProcesses(t *testing.T) {
	_, raw := postgresBudget(t)
	type result struct {
		raw string
		err error
	}
	out := make(chan result, 2)
	for i := 0; i < 2; i++ {
		c := exec.Command(os.Args[0], "-test.run=^TestPostgresBudgetProcessHelper$", "-test.timeout=120s")
		c.Env = append(os.Environ(), "TITAN_P08_CHILD_PG="+raw)
		go func() { b, e := c.Output(); out <- result{string(b), e} }()
	}
	n := 0
	for i := 0; i < 2; i++ {
		r := <-out
		if r.err != nil {
			t.Fatal("child failed", r.err)
		}
		a, e := strconv.Atoi(r.raw)
		if e != nil {
			t.Fatal("invalid child output")
		}
		n += a
	}
	if n != 5 {
		t.Fatalf("processes admitted %d", n)
	}
}
