package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
)

// CustomerVaultDiagnostic is installation-local and must never be exposed as a
// remote path-reading API. It opens no writable connection, transaction or key
// journal; in particular it cannot initialize, bind, rotate or clean the vault.
// Wrong/missing keys and concurrent changes return unavailable, never secrets.
func CustomerVaultDiagnostic(ctx context.Context, path, keyFile, exportedFile string) (available, exported bool) {
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil || resolved != path || !privateReference(path) {
		return false, false
	}
	key, err := readMasterKey(keyFile)
	if err != nil {
		return false, false
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return false, false
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return false, false
	}
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return false, false
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var tenant string
	var sealed []byte
	if db.QueryRowContext(ctx, `SELECT tenant,key_check FROM fiscal_vault_control WHERE id=1`).Scan(&tenant, &sealed) != nil {
		return false, false
	}
	raw, err := openSealed(a, sealed, keyCheckAAD(tenant))
	defer clear(raw)
	if err != nil || !bytes.Equal(raw, keyCheckPlain) {
		return false, false
	}
	copy, err := readMasterKey(exportedFile)
	defer clear(copy)
	sourceInfo, se := os.Stat(keyFile)
	copyInfo, ce := os.Stat(exportedFile)
	return true, err == nil && se == nil && ce == nil && !os.SameFile(sourceInfo, copyInfo) && bytes.Equal(key, copy)
}
