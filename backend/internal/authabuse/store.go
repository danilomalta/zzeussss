// Package authabuse keeps bounded authentication budgets in the customer's DB.
// It never stores passwords, recovery secrets, complete account names or raw IPs.
package authabuse

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"titansystem-backend/internal/sqlitewrite"
)

const Schema = `CREATE TABLE auth_attempt_budgets (
 budget_key VARCHAR(64) PRIMARY KEY,
 window_unix BIGINT NOT NULL,
 attempts INTEGER NOT NULL CHECK(attempts > 0)
);`

var ErrLimited = errors.New("authentication attempt budget exceeded")
var ErrUnavailable = errors.New("authentication protection unavailable")

type Store struct {
	DB       *sql.DB
	Postgres bool
}

// Take reserves all budgets atomically, including on rejected attempts. Clock
// and counters belong to the database, so separate processes share the policy.
// Failed-account pressure is evaluated only AFTER credential verification: an
// attacker cannot lock out a victim who knows their password/recovery secret.
func (s Store) Take(ctx context.Context, scope, channel, peer, failedAccount string) error {
	if s.DB == nil || scope == "" || channel == "" {
		return ErrUnavailable
	}
	limited := false
	run := func(tx *sql.Tx) error {
		limited = false
		var now int64
		clock := `SELECT CAST(strftime('%s','now') AS INTEGER)`
		if s.Postgres {
			clock = `SELECT CAST(EXTRACT(EPOCH FROM clock_timestamp()) AS BIGINT)`
		}
		if err := tx.QueryRowContext(ctx, clock).Scan(&now); err != nil {
			return err
		}
		window := now
		// Expired entries cannot accumulate across windows or disclose account lists.
		if _, err := tx.ExecContext(ctx, s.query(`DELETE FROM auth_attempt_budgets WHERE window_unix < ?`), window-120); err != nil {
			return err
		}
		limits := []struct {
			kind, value string
			max         int
		}{{"installation", "", 120}, {"peer", peer, 5}}
		if failedAccount != "" {
			limits = []struct {
				kind, value string
				max         int
			}{{"failed-account", failedAccount, 5}}
		}
		for _, item := range limits {
			digest := sha256.Sum256([]byte(scope + "\x00" + channel + "\x00" + item.kind + "\x00" + item.value))
			key := hex.EncodeToString(digest[:])
			var count int
			err := tx.QueryRowContext(ctx, s.query(`INSERT INTO auth_attempt_budgets(budget_key,window_unix,attempts) VALUES(?,?,1) ON CONFLICT(budget_key) DO UPDATE SET window_unix=CASE WHEN auth_attempt_budgets.window_unix>excluded.window_unix-60 THEN auth_attempt_budgets.window_unix ELSE excluded.window_unix END,attempts=CASE WHEN auth_attempt_budgets.window_unix>excluded.window_unix-60 THEN MIN(auth_attempt_budgets.attempts+1,121) ELSE 1 END RETURNING attempts`), key, window).Scan(&count)
			if err != nil {
				return err
			}
			if count > item.max {
				limited = true
			}
		}
		return nil
	}
	var err error
	if s.Postgres {
		var tx *sql.Tx
		tx, err = s.DB.BeginTx(ctx, nil)
		if err == nil {
			defer tx.Rollback()
			// One short transaction serializes only budget metadata, never bcrypt or a
			// user/session transaction. lock_timeout bounds unavailable instances.
			_, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout = '1000ms'`)
			if err == nil {
				_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203081)`)
			}
			if err == nil {
				err = run(tx)
			}
			if err == nil {
				err = tx.Commit()
			}
		}
	} else {
		err = sqlitewrite.Run(ctx, s.DB, ErrUnavailable, run)
	}
	if err != nil {
		return ErrUnavailable
	}
	if limited {
		return ErrLimited
	}
	return nil
}
func (s Store) query(q string) string {
	if !s.Postgres {
		return q
	}
	result := ""
	n := 1
	for _, c := range q {
		if c == '?' {
			result += "$" + strconv.Itoa(n)
			n++
		} else {
			result += string(c)
		}
	}
	// PostgreSQL uses LEAST instead of SQLite's scalar MIN.
	for i := 0; i+4 <= len(result); i++ {
		if result[i:i+4] == "MIN(" {
			return result[:i] + "LEAST(" + result[i+4:]
		}
	}
	return result
}
