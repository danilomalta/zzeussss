package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitepath"
)

type RestoreTestReport struct {
	SchemaVersion   int  `json:"schema_version"`
	TablesChecked   int  `json:"tables_checked"`
	SessionsRevoked bool `json:"sessions_revoked"`
	DataMatches     bool `json:"data_matches"`
}

// TestRestore never publishes an installation. Both decrypted copies remain
// under the customer-selected private directory and are removed on every exit.
// Current-schema business rows are compared exactly, including all modules.
// The four security/recovery tables intentionally changed by Restore are checked
// separately or excluded; their operational barriers must not be undone.
func TestRestore(ctx context.Context, archive, root string, d identity.DeviceContext, key []byte) (RestoreTestReport, error) {
	var report RestoreTestReport
	root = filepath.Clean(root)
	resolved, err := filepath.EvalSymlinks(root)
	info, e := os.Lstat(root)
	if err != nil || e != nil || !filepath.IsAbs(root) || resolved != root || !info.IsDir() || info.Mode().Perm() != 0700 {
		return report, ErrBackup
	}
	dir, err := os.MkdirTemp(root, ".titan-restore-test-")
	if err != nil {
		return report, ErrBackup
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "restored.sqlite")
	var before Inspection
	if _, err = withSnapshotAt(ctx, archive, d, key, path, &before, dir, true); err != nil {
		return report, err
	}
	latest, err := localdb.LatestSchemaVersion()
	if err != nil || before.SchemaVersion != latest {
		return report, ErrBackup
	}
	if err = validate(ctx, path, d); err != nil {
		return report, err
	}
	after, err := inspectBusinessSnapshot(ctx, path)
	if err != nil || before.BusinessSHA256 != after.BusinessSHA256 || !reflect.DeepEqual(before.Records, after.Records) {
		return report, ErrBackup
	}
	u, err := sqlitepath.URL(path)
	if err != nil {
		return report, ErrBackup
	}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return report, ErrBackup
	}
	defer db.Close()
	var live, recovery, barrier int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&live) != nil || live != 0 {
		return report, ErrBackup
	}
	if db.QueryRowContext(ctx, `SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NULL`).Scan(&recovery) != nil || recovery != 0 {
		return report, ErrBackup
	}
	if db.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, d.TenantID).Scan(&barrier) != nil || barrier != 1 {
		return report, ErrBackup
	}
	return RestoreTestReport{latest, len(after.Records), true, true}, nil
}

func inspectBusinessSnapshot(ctx context.Context, path string) (Inspection, error) {
	u, err := sqlitepath.URL(path)
	if err != nil {
		return Inspection{}, ErrBackup
	}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return Inspection{}, ErrBackup
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('local_sessions','owner_recovery_keys','company_trade_recovery','fiscal_restore_barriers') ORDER BY name`)
	if err != nil {
		return Inspection{}, ErrBackup
	}
	var tables []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) != nil {
			rows.Close()
			return Inspection{}, ErrBackup
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Inspection{}, ErrBackup
	}
	return inspectTables(ctx, path, nil, tables)
}
