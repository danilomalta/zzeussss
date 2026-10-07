package onlinesessions

import (
	"context"
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"testing"
)

func recoveryPostgresFlow(t *testing.T, ctx context.Context, db *sql.DB, s *Store) {
	t.Helper()
	p := Scope{Tenant: testTenant, User: testUser, Role: "owner"}
	var hash string
	if db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, testUser).Scan(&hash) != nil {
		t.Fatal("recovery fixture unavailable")
	}
	session, e := s.Create(ctx, p, hash)
	if e != nil {
		t.Fatal("recovery session unavailable")
	}
	p.Session, _ = ParseRefresh(session.Refresh)
	if _, e = s.IssueRecovery(ctx, p, "incorrect-password"); e != ErrDenied {
		t.Fatal("wrong password issued key")
	}
	key, e := s.IssueRecovery(ctx, p, nextPassword)
	if e != nil {
		t.Fatal("key issue failed", e)
	}
	if _, e = s.IssueRecovery(ctx, p, nextPassword); e != ErrRecoveryRate {
		t.Fatal("durable issuance limit failed")
	}
	if _, e = db.ExecContext(ctx, `UPDATE online_recovery_keys SET issued_at=issued_at-interval '2 minutes'`); e != nil {
		t.Fatal("fixture cooldown unavailable")
	}
	replacement, e := s.IssueRecovery(ctx, p, nextPassword)
	if e != nil {
		t.Fatal("replacement failed")
	}
	finalPassword := "recovered-test-password"
	if s.Recover(ctx, key.Key, finalPassword) != ErrDenied {
		t.Fatal("replaced key accepted")
	}
	if _, e = db.ExecContext(ctx, `CREATE TRIGGER reject_recovery BEFORE INSERT ON online_recovery_audit FOR EACH ROW WHEN (NEW.event='recovered') EXECUTE FUNCTION reject_rotation()`); e != nil {
		t.Fatal("recovery fault injection unavailable")
	}
	var expectedLive int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL`, testTenant, testUser).Scan(&expectedLive) != nil {
		t.Fatal("session fixture unavailable")
	}
	if s.Recover(ctx, replacement.Key, finalPassword) != ErrUnavailable {
		t.Fatal("audit failure not reported")
	}
	var unchanged string
	var consumed bool
	if db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, testUser).Scan(&unchanged) != nil || unchanged != hash {
		t.Fatal("password rollback failed")
	}
	if db.QueryRowContext(ctx, `SELECT consumed_at IS NOT NULL FROM online_recovery_keys WHERE tenant_id=$1 AND user_id=$2`, testTenant, testUser).Scan(&consumed) != nil || consumed {
		t.Fatal("key rollback failed")
	}
	var live int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL`, testTenant, testUser).Scan(&live) != nil || live != expectedLive {
		t.Fatal("session rollback failed")
	}
	if _, e = db.ExecContext(ctx, `ALTER TABLE online_recovery_audit DISABLE TRIGGER reject_recovery`); e != nil {
		t.Fatal("fixture trigger unavailable")
	}
	results := make(chan struct {
		kind string
		err  error
	}, 4)
	var wg sync.WaitGroup
	wg.Add(4)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			results <- struct {
				kind string
				err  error
			}{"recover", s.Recover(ctx, replacement.Key, finalPassword)}
		}()
	}
	go func() {
		defer wg.Done()
		_, e := s.Create(ctx, Scope{Tenant: testTenant, User: testUser, Role: "owner"}, hash)
		results <- struct {
			kind string
			err  error
		}{"login", e}
	}()
	go func() {
		defer wg.Done()
		_, e := s.Rotate(ctx, session.Refresh)
		results <- struct {
			kind string
			err  error
		}{"refresh", e}
	}()
	wg.Wait()
	close(results)
	success, denied := 0, 0
	for r := range results {
		if r.kind == "recover" {
			if r.err == nil {
				success++
			} else if r.err == ErrDenied {
				denied++
			} else {
				t.Fatal("recovery concurrency failed", r.err)
			}
		} else if r.err != nil && r.err != ErrDenied {
			t.Fatal("session concurrency failed")
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal("key was not single use")
	}
	if db.QueryRowContext(ctx, `SELECT count(*) FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL`, testTenant, testUser).Scan(&live) != nil || live != 0 {
		t.Fatal("recovery left old sessions active")
	}
	if s.Recover(ctx, replacement.Key, finalPassword) != ErrDenied {
		t.Fatal("consumed key accepted")
	}
	var total int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM online_recovery_audit WHERE event='recovered'`).Scan(&total) != nil || total != 1 {
		t.Fatal("recovery audit duplicated")
	}
	if db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, testUser).Scan(&hash) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(finalPassword)) != nil {
		t.Fatal("recovered password unavailable")
	}
	session, e = s.Create(ctx, Scope{Tenant: testTenant, User: testUser, Role: "owner"}, hash)
	if e != nil {
		t.Fatal("login after recovery failed")
	}
	p.Session, _ = ParseRefresh(session.Refresh)
	if _, e = db.ExecContext(ctx, `UPDATE online_recovery_keys SET issued_at=issued_at-interval '2 minutes'`); e != nil {
		t.Fatal("fixture cooldown unavailable")
	}
	expiryKey, e := s.IssueRecovery(ctx, p, finalPassword)
	if e != nil {
		t.Fatal("expiry fixture unavailable")
	}
	if _, e = db.ExecContext(ctx, `UPDATE online_recovery_keys SET issued_at=clock_timestamp()-interval '2 days',expires_at=clock_timestamp()-interval '1 day'`); e != nil {
		t.Fatal("expiry fixture failed")
	}
	if s.Recover(ctx, expiryKey.Key, nextPassword) != ErrDenied {
		t.Fatal("expired key accepted")
	}
	stale, e := s.IssueRecovery(ctx, p, finalPassword)
	if e != nil {
		t.Fatal("stale fixture unavailable")
	}
	if e = s.ChangePassword(ctx, p, finalPassword, nextPassword); e != nil {
		t.Fatal("password fixture failed")
	}
	if s.Recover(ctx, stale.Key, finalPassword) != ErrDenied {
		t.Fatal("password change did not invalidate key")
	}
}
