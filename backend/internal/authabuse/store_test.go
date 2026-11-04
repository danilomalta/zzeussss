package authabuse_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"titansystem-backend/internal/authabuse"
	"titansystem-backend/internal/localdb"
)

func sqliteStore(t *testing.T) (authabuse.Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "attempts.sqlite")
	db, e := localdb.Open(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return authabuse.Store{DB: db}, p
}
func TestDurableBudgetsRestartIsolationAndPrivacy(t *testing.T) {
	s, p := sqliteStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if e := s.Take(ctx, "tenant-a", "login", "peer", ""); e != nil {
			t.Fatal(e)
		}
	}
	db, e := sql.Open("sqlite", p)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := authabuse.Store{DB: db}
	if !errors.Is(a.Take(ctx, "tenant-a", "login", "peer", ""), authabuse.ErrLimited) {
		t.Fatal("restart bypass")
	}
	for _, scope := range []string{"tenant-b", "tenant-c"} {
		if e := a.Take(ctx, scope, "login", "peer", ""); e != nil {
			t.Fatal("scope leaked")
		}
	}
	if e := a.Take(ctx, "tenant-a", "recovery", "peer", ""); e != nil {
		t.Fatal("channel leaked")
	}
	for i := 0; i < 6; i++ {
		e := s.Take(ctx, "tenant-a", "login", "", "person@example.test")
		if (i == 5) != errors.Is(e, authabuse.ErrLimited) {
			t.Fatal("account pressure")
		}
	}
	rows, e := db.Query(`SELECT budget_key FROM auth_attempt_budgets`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if rows.Scan(&k) != nil || len(k) != 64 {
			t.Fatal("raw identifier retained")
		}
	}
	if e := rows.Err(); e != nil {
		t.Fatal(e)
	}
}
func TestDurableInstallationBudgetAndWindowRecovery(t *testing.T) {
	s, _ := sqliteStore(t)
	ctx := context.Background()
	for i := 0; i < 120; i++ {
		if e := s.Take(ctx, "scope", "login", strconv.Itoa(i), ""); e != nil {
			t.Fatal(e)
		}
	}
	if !errors.Is(s.Take(ctx, "scope", "login", "new", ""), authabuse.ErrLimited) {
		t.Fatal("distributed bypass")
	}
	if _, e := s.DB.Exec(`UPDATE auth_attempt_budgets SET window_unix=window_unix-120`); e != nil {
		t.Fatal(e)
	}
	if e := s.Take(ctx, "scope", "login", "new", ""); e != nil {
		t.Fatal("expired policy did not recover")
	}
}
func TestDurableBudgetConcurrentAdmissions(t *testing.T) {
	s, p := sqliteStore(t)
	out := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, e := sql.Open("sqlite", p)
			if e != nil {
				out <- e
				return
			}
			defer db.Close()
			out <- (authabuse.Store{DB: db}).Take(context.Background(), "scope", "login", "peer", "")
		}()
	}
	wg.Wait()
	close(out)
	n := 0
	for e := range out {
		if e == nil {
			n++
		} else if !errors.Is(e, authabuse.ErrLimited) {
			t.Fatal(e)
		}
	}
	if n != 5 {
		t.Fatalf("admitted %d", n)
	}
	if e := s.DB.QueryRow(`SELECT count(*) FROM auth_attempt_budgets`).Scan(&n); e != nil || n != 2 {
		t.Fatal("partial reservation")
	}
}
func TestAuthBudgetProcessHelper(t *testing.T) {
	p := os.Getenv("TITAN_P08_CHILD_SQLITE")
	if p == "" {
		return
	}
	db, e := sql.Open("sqlite", p)
	if e != nil {
		os.Exit(2)
	}
	s := authabuse.Store{DB: db}
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
func TestDurableBudgetAcrossProcesses(t *testing.T) {
	_, p := sqliteStore(t)
	type result struct {
		raw string
		err error
	}
	out := make(chan result, 2)
	for i := 0; i < 2; i++ {
		c := exec.Command(os.Args[0], "-test.run=^TestAuthBudgetProcessHelper$", "-test.timeout=120s")
		c.Env = append(os.Environ(), "TITAN_P08_CHILD_SQLITE="+p)
		go func() { b, e := c.Output(); out <- result{string(b), e} }()
	}
	total := 0
	for i := 0; i < 2; i++ {
		r := <-out
		if r.err != nil {
			t.Fatal("child failed", r.err)
		}
		n, e := strconv.Atoi(r.raw)
		if e != nil {
			t.Fatal("invalid child output")
		}
		total += n
	}
	if total != 5 {
		t.Fatalf("processes admitted %d", total)
	}
}
func TestDurableBudgetRollbackAndFailClosed(t *testing.T) {
	s, _ := sqliteStore(t)
	if _, e := s.DB.Exec(`CREATE TRIGGER fail_budget BEFORE INSERT ON auth_attempt_budgets WHEN (SELECT count(*) FROM auth_attempt_budgets)=1 BEGIN SELECT RAISE(ABORT,'fixture'); END`); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(s.Take(context.Background(), "scope", "login", "peer", ""), authabuse.ErrUnavailable) {
		t.Fatal("write failure accepted")
	}
	var n int
	if e := s.DB.QueryRow(`SELECT count(*) FROM auth_attempt_budgets`).Scan(&n); e != nil || n != 0 {
		t.Fatal("partial reservation")
	}
	if !errors.Is((authabuse.Store{}).Take(context.Background(), "s", "login", "p", ""), authabuse.ErrUnavailable) {
		t.Fatal("missing db accepted")
	}
}
