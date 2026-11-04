// Package sqlitewrite provides bounded, cross-process SQLite writer transactions.
package sqlitewrite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	sqlite "modernc.org/sqlite"
)

func Contention(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && (e.Code()&255 == 5 || e.Code()&255 == 6)
}

func Run(ctx context.Context, db *sql.DB, conflict error, fn func(*sql.Tx) error) error {
	return RunCleanup(ctx, db, conflict, func(tx *sql.Tx) (func(bool), error) { return nil, fn(tx) })
}

// RunCleanup invokes cleanup after COMMIT/ROLLBACK, before a possible retry.
// It is used to remove only an attempt's own, uncommitted key export.
func RunCleanup(ctx context.Context, source *sql.DB, conflict error, fn func(*sql.Tx) (func(bool), error)) error {
	var last error
	for i := 0; i < 4; i++ {
		err := runAttempt(ctx, source, fn)
		if !Contention(err) {
			return err
		}
		last = err
		if i < 3 {
			timer := time.NewTimer(time.Duration(i+1) * 25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	var cause *sqlite.Error
	errors.As(last, &cause)
	return errors.Join(conflict, fmt.Errorf("SQLite writer contention: %w", cause))
}

func runAttempt(ctx context.Context, source *sql.DB, fn func(*sql.Tx) (func(bool), error)) error {
	// Metadata only: no business/credential read is performed before BEGIN IMMEDIATE.
	file, err := databaseFile(ctx, source)
	if err != nil {
		return err
	}
	if file == "" {
		return errors.New("immediate fiscal writes require the existing file-backed SQLite database")
	}
	u := url.URL{Scheme: "file", Path: file}
	q := u.Query()
	q.Set("mode", "rw")
	q.Set("_txlock", "immediate")
	q.Add("_pragma", "busy_timeout(1000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(FULL)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	cleanup, err := fn(tx)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
	}
	if cleanup != nil {
		cleanup(err == nil)
	}
	return err
}

func databaseFile(ctx context.Context, source *sql.DB) (string, error) {
	conn, err := source.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("SQLite writer metadata connection: %w", err)
	}
	defer conn.Close()
	// A cold source connection can encounter an exclusive writer before the
	// separately configured writer connection is opened. Set the same bounded
	// wait on the exact connection used for metadata, not an arbitrary pool member.
	var timeout int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		return "", fmt.Errorf("SQLite writer metadata timeout query: %w", err)
	}
	// Other callers can already configure a longer wait (localdb uses 5000ms).
	// Never shorten that policy when preparing a cold/unconfigured connection.
	if timeout < 1000 {
		if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout(1000)`); err != nil {
			return "", fmt.Errorf("SQLite writer metadata timeout: %w", err)
		}
	}
	var file string
	if err := conn.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&file); err != nil {
		return "", fmt.Errorf("SQLite writer metadata query: %w", err)
	}
	// The connection must return to the pool before BEGIN IMMEDIATE/callback;
	// callers may have a one-connection source pool.
	return file, nil
}
