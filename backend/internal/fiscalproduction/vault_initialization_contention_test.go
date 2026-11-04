package fiscalproduction

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
)

// A transient exclusive writer must not exhaust initialization retries before
// the configured writer busy timeout has had a chance to wait for that writer.
func TestVaultInitializeWaitsForTransientExclusiveLock(t *testing.T) {
	v, path := testVault(t)
	if _, err := v.db.Exec(`DELETE FROM fiscal_vault_control`); err != nil {
		t.Fatal(err)
	}
	handles := vaultHandles(t, v, path) // Cold connections, as at startup.
	holder, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := holder.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `BEGIN EXCLUSIVE`); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), `ROLLBACK`)
	released := make(chan error, 1)
	go func() {
		time.Sleep(250 * time.Millisecond)
		_, err := conn.ExecContext(context.Background(), `ROLLBACK`)
		released <- err
	}()
	// Diagnose metadata only; never log business data or credential values.
	var file string
	started := time.Now()
	probeErr := handles[0].db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&file)
	var sqliteErr *sqlite.Error
	if errors.As(probeErr, &sqliteErr) {
		t.Logf("stage=source_metadata sqlite_code=%d elapsed_ms=%d", sqliteErr.Code(), time.Since(started).Milliseconds())
	}
	errs := concurrentVaultCalls(4, func(i int) error { return handles[i].initializeKeyControl() })
	if err := <-released; err != nil {
		t.Fatal("exclusive writer release failed")
	}
	for _, err := range errs {
		if err != nil {
			code := 0
			if errors.As(err, &sqliteErr) {
				code = sqliteErr.Code()
			}
			t.Errorf("initialization failed after transient writer: sqlite_code=%d", code)
		}
	}
	var count int
	if err := v.db.QueryRow(`SELECT count(*) FROM fiscal_vault_control`).Scan(&count); err != nil || count != 1 {
		t.Fatal("initialization did not produce exactly one key control")
	}
	if err := v.bindTenant(context.Background(), "client"); err != nil {
		t.Fatal("initialized key control cannot bind tenant")
	}
}
