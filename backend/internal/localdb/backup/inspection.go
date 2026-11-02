package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"

	"titansystem-backend/internal/localdb/identity"
)

// Inspection describes only the explicitly named pilot tables. It never exports
// row contents, credentials, device keys or sessions. Missing older-schema tables
// are reported as unavailable, never silently counted as zero.
type Inspection struct {
	Version        int              `json:"version"`
	Profile        string           `json:"profile"`
	SchemaVersion  int              `json:"schema_version"`
	ArchiveSHA256  string           `json:"archive_sha256"`
	BusinessSHA256 string           `json:"business_sha256"`
	Records        map[string]int64 `json:"records"`
	Unavailable    []string         `json:"unavailable"`
}

var pilotTables = []string{
	"cash_movements", "cash_sessions", "payable_cash_payments",
	"purchase_deliveries", "purchase_order_items", "purchase_orders",
	"purchase_receipts", "purchase_receiving_cases", "purchase_supplier_decisions",
	"sale_payments", "sale_tax_bindings", "sales", "stock_movements",
	"supplier_payable_installments", "supplier_payables",
}

// Inspect authenticates/decrypts/validates exactly as Verify does. Its bounded
// disposable snapshot is removed on success and failure. It never restores into
// an installation, migrates the original or revokes its current sessions.
func Inspect(ctx context.Context, archive string, device identity.DeviceContext, key []byte) (Inspection, error) {
	var out Inspection
	if _, err := withSnapshot(ctx, archive, device, key, "", &out); err != nil {
		return Inspection{}, err
	}
	return out, nil
}

func inspectSnapshot(ctx context.Context, path string, sealed []byte) (Inspection, error) {
	var out Inspection
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return out, ErrBackup
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	out = Inspection{Version: 1, Profile: "pilot_cash_v1", Records: map[string]int64{}, Unavailable: []string{}}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&out.SchemaVersion); err != nil {
		return Inspection{}, ErrBackup
	}
	archiveHash := sha256.Sum256(sealed)
	out.ArchiveSHA256 = hex.EncodeToString(archiveHash[:])
	h := sha256.New()
	encode := json.NewEncoder(h)
	for _, name := range pilotTables {
		var exists int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&exists); err != nil {
			return Inspection{}, ErrBackup
		}
		if exists != 1 {
			out.Unavailable = append(out.Unavailable, name)
			if err = encode.Encode([]string{name, "unavailable"}); err != nil {
				return Inspection{}, ErrBackup
			}
			continue
		}
		rows, e := db.QueryContext(ctx, `SELECT * FROM "`+name+`" LIMIT 0`)
		if e != nil {
			return Inspection{}, ErrBackup
		}
		columns, e := rows.Columns()
		closeErr := rows.Close()
		if e != nil || closeErr != nil || len(columns) == 0 {
			return Inspection{}, ErrBackup
		}
		if e = encode.Encode([]any{name, columns}); e != nil {
			return Inspection{}, ErrBackup
		}
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
		}
		// Ordering by every column remains deterministic after VACUUM/restore,
		// unlike rowid (which is not a business identifier).
		rows, e = db.QueryContext(ctx, `SELECT * FROM "`+name+`" ORDER BY `+strings.Join(quoted, ","))
		if e != nil {
			return Inspection{}, ErrBackup
		}
		var count int64
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if e = rows.Scan(dest...); e != nil {
				rows.Close()
				return Inspection{}, ErrBackup
			}
			if e = encode.Encode(values); e != nil {
				rows.Close()
				return Inspection{}, ErrBackup
			}
			count++
		}
		e = rows.Err()
		closeErr = rows.Close()
		if e != nil || closeErr != nil {
			return Inspection{}, ErrBackup
		}
		out.Records[name] = count
	}
	out.BusinessSHA256 = hex.EncodeToString(h.Sum(nil))
	return out, nil
}
