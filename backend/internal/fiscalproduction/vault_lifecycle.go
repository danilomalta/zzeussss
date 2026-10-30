package fiscalproduction

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"titansystem-backend/internal/deployment"
)

var keyCheckPlain = []byte("TitanSystem customer vault key v1")

func keyCheckAAD(tenant string) []byte { return []byte("titan-vault-control-v1\x00" + tenant) }
func sealVault(a cipher.AEAD, raw, aad []byte) ([]byte, error) {
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrUnavailable
	}
	return a.Seal(nonce, nonce, raw, aad), nil
}
func openSealed(a cipher.AEAD, raw, aad []byte) ([]byte, error) {
	if len(raw) < a.NonceSize()+a.Overhead() {
		return nil, ErrUnavailable
	}
	n := a.NonceSize()
	plain, err := a.Open(nil, raw[:n], raw[n:], aad)
	if err != nil {
		return nil, ErrUnavailable
	}
	return plain, nil
}
func (v *FiscalSecretVault) checkKeyTx(ctx context.Context, tx *sql.Tx) (string, error) {
	var tenant string
	var sealed []byte
	if tx.QueryRowContext(ctx, `SELECT tenant,key_check FROM fiscal_vault_control WHERE id=1`).Scan(&tenant, &sealed) != nil {
		return "", ErrUnavailable
	}
	raw, err := openSealed(v.aead, sealed, keyCheckAAD(tenant))
	defer clear(raw)
	if err != nil || string(raw) != string(keyCheckPlain) {
		return "", ErrUnavailable
	}
	return tenant, nil
}
func (v *FiscalSecretVault) initializeKeyControl() error {
	ctx := context.Background()
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS fiscal_vault_control(id INTEGER PRIMARY KEY CHECK(id=1),tenant TEXT NOT NULL,key_check BLOB NOT NULL)`); err != nil {
		return err
	}
	var count int
	if tx.QueryRow(`SELECT count(*) FROM fiscal_vault_control`).Scan(&count) != nil {
		return ErrUnavailable
	}
	if count == 0 {
		rows, err := tx.Query(`SELECT tenant,purpose,ciphertext FROM fiscal_secret_vault`)
		if err != nil {
			return err
		}
		tenant := ""
		for rows.Next() {
			var t, p string
			var sealed []byte
			if rows.Scan(&t, &p, &sealed) != nil {
				rows.Close()
				return ErrUnavailable
			}
			if tenant != "" && tenant != t {
				rows.Close()
				return ErrUnavailable
			}
			tenant = t
			aad, err := vaultAAD(t, p)
			if err != nil {
				rows.Close()
				return err
			}
			raw, err := openSealed(v.aead, sealed, aad)
			clear(raw)
			if err != nil {
				rows.Close()
				return err
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		sealed, err := sealVault(v.aead, keyCheckPlain, keyCheckAAD(tenant))
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO fiscal_vault_control VALUES(1,?,?)`, tenant, sealed); err != nil {
			return err
		}
	}
	if _, err = v.checkKeyTx(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (v *FiscalSecretVault) bindTenant(ctx context.Context, tenant string) error {
	if tenant == "" {
		return ErrUnavailable
	}
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	bound, err := v.checkKeyTx(ctx, tx)
	if err != nil {
		return err
	}
	if bound != "" && bound != tenant {
		return ErrUnavailable
	}
	if bound == "" {
		var other int
		if tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_secret_vault WHERE tenant<>?`, tenant).Scan(&other) != nil || other != 0 {
			return ErrUnavailable
		}
		sealed, err := sealVault(v.aead, keyCheckPlain, keyCheckAAD(tenant))
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE fiscal_vault_control SET tenant=?,key_check=? WHERE id=1`, tenant, sealed); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func readMasterKey(path string) ([]byte, error) {
	raw, err := deployment.PrivateRead(path, 256)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer clear(raw)
	key, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != 32 {
		clear(key)
		return nil, ErrUnavailable
	}
	return key, nil
}

// OpenCustomerFiscalVault never exports the key. A wrong/missing key fails even
// on an empty vault, before a provider can start. Paths are trusted process input.
func OpenCustomerFiscalVault(path, keyFile string) (*FiscalSecretVault, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, ErrUnavailable
	}
	key, err := readMasterKey(keyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer clear(key)
	return openVaultKey(path, key)
}
func privateNew(path string, raw []byte) error {
	if !filepath.IsAbs(path) {
		return ErrUnavailable
	}
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrUnavailable
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrUnavailable
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		f.Close()
		return ErrUnavailable
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return ErrUnavailable
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return ErrUnavailable
	}
	if err = f.Close(); err != nil {
		return ErrUnavailable
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return ErrUnavailable
	}
	defer dir.Close()
	return dir.Sync()
}

// CreateCustomerFiscalVault is an installation-only operation. display must be
// the customer's private terminal, never an HTTP response or application log.
func CreateCustomerFiscalVault(path, keyFile string, display io.Writer) error {
	if display == nil || !filepath.IsAbs(path) || path == keyFile {
		return ErrUnavailable
	}
	for _, p := range []string{path, keyFile} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return ErrUnavailable
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ErrUnavailable
	}
	defer clear(key)
	encoded := []byte(base64.StdEncoding.EncodeToString(key))
	defer clear(encoded)
	if err := privateNew(keyFile, append(encoded, '\n')); err != nil {
		return err
	}
	v, err := openVaultKey(path, key)
	if err != nil {
		return err
	}
	v.Close()
	_, err = display.Write(append(encoded, '\n'))
	return err
}
func ExportCustomerMasterKey(source, destination string) error {
	key, err := readMasterKey(source)
	if err != nil {
		return err
	}
	defer clear(key)
	encoded := []byte(base64.StdEncoding.EncodeToString(key) + "\n")
	defer clear(encoded)
	return privateNew(destination, encoded)
}

// Rotation re-encrypts all records and key control in one transaction. Old key
// files/backups are preserved; stale open handles cannot commit old-key writes.
func RotateCustomerMasterKey(path, oldFile, newFile string, display io.Writer) error {
	if display == nil || oldFile == newFile || newFile == path {
		return ErrUnavailable
	}
	if _, err := os.Lstat(newFile); !os.IsNotExist(err) {
		return ErrUnavailable
	}
	v, err := OpenCustomerFiscalVault(path, oldFile)
	if err != nil {
		return err
	}
	defer v.Close()
	ctx := context.Background()
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	tenant, err := v.checkKeyTx(ctx, tx)
	if err != nil {
		return err
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return ErrUnavailable
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return ErrUnavailable
	}
	next, err := cipher.NewGCM(block)
	if err != nil {
		return ErrUnavailable
	}
	type row struct {
		tenant, purpose string
		sealed          []byte
	}
	records := []row{}
	rows, err := tx.QueryContext(ctx, `SELECT tenant,purpose,ciphertext FROM fiscal_secret_vault`)
	if err != nil {
		return ErrUnavailable
	}
	for rows.Next() {
		var r row
		if rows.Scan(&r.tenant, &r.purpose, &r.sealed) != nil {
			rows.Close()
			return ErrUnavailable
		}
		aad, err := vaultAAD(r.tenant, r.purpose)
		if err != nil {
			rows.Close()
			return err
		}
		raw, err := openSealed(v.aead, r.sealed, aad)
		if err != nil {
			rows.Close()
			return err
		}
		r.sealed, err = sealVault(next, raw, aad)
		clear(raw)
		if err != nil {
			rows.Close()
			return err
		}
		records = append(records, r)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return ErrUnavailable
	}
	rows.Close()
	encoded := []byte(base64.StdEncoding.EncodeToString(key) + "\n")
	defer clear(encoded)
	if err = privateNew(newFile, encoded); err != nil {
		return err
	}
	for _, r := range records {
		if _, err = tx.ExecContext(ctx, `UPDATE fiscal_secret_vault SET ciphertext=? WHERE tenant=? AND purpose=?`, r.sealed, r.tenant, r.purpose); err != nil {
			return ErrUnavailable
		}
	}
	marker, err := sealVault(next, keyCheckPlain, keyCheckAAD(tenant))
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE fiscal_vault_control SET key_check=? WHERE id=1`, marker); err != nil {
		return ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return ErrUnavailable
	}
	_, err = display.Write(encoded)
	return err
}
