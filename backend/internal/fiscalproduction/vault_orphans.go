package fiscalproduction

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"titansystem-backend/internal/deployment"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/sqlitewrite"
)

// Key references contain only one-way fingerprints, never keys or passwords.
func referenceKeyTx(ctx context.Context, tx *sql.Tx, key []byte) error {
	digest := sha256.Sum256(key)
	_, err := tx.ExecContext(ctx, `INSERT INTO fiscal_vault_key_references(digest,created_at) VALUES(?,?) ON CONFLICT(digest) DO NOTHING`, hex.EncodeToString(digest[:]), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func referenceKeyFile(v *FiscalSecretVault, file string) error {
	key, err := readMasterKey(file)
	if err != nil {
		return err
	}
	defer clear(key)
	return sqlitewrite.Run(context.Background(), v.db, fiscal.ErrConflict, func(tx *sql.Tx) error {
		if _, err := v.checkKeyTx(context.Background(), tx); err != nil {
			return err
		}
		return referenceKeyTx(context.Background(), tx, key)
	})
}

type VaultOrphan struct {
	ID         string `json:"id"`
	ModifiedAt string `json:"modified_at"`
}
type orphanCandidate struct {
	public       VaultOrphan
	path, digest string
	info         os.FileInfo
}

// The customer explicitly chooses a dedicated key directory. No recursive scan,
// network call or arbitrary legacy-file classification is performed.
type keyCandidateRecord struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// An append-only private journal proves this installation created the candidate.
// Unregistered files (including other clients' keys) are never classified or removed.
func recordKeyCandidate(vault, file string, encoded []byte) error {
	raw, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(raw) != 32 {
		return ErrUnavailable
	}
	defer clear(raw)
	digest := sha256.Sum256(raw)
	record := keyCandidateRecord{file, hex.EncodeToString(digest[:])}
	body, _ := json.Marshal(record)
	dir := vault + ".key-candidates"
	if err = os.MkdirAll(dir, 0700); err != nil {
		return ErrUnavailable
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
		return ErrUnavailable
	}
	id := sha256.Sum256(body)
	fileID := filepath.Join(dir, hex.EncodeToString(id[:])+".json")
	if old, e := deployment.PrivateRead(fileID, 4096); e == nil && string(old) == string(body) {
		return nil
	}
	return privateNew(fileID, body)
}
func scanKeyDirectory(vault, directory string) ([]orphanCandidate, error) {
	dir, err := os.Lstat(directory)
	if err != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 || !filepath.IsAbs(directory) {
		return nil, ErrUnavailable
	}
	journal := vault + ".key-candidates"
	info, err := os.Lstat(journal)
	if os.IsNotExist(err) {
		return []orphanCandidate{}, nil
	}
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnavailable
	}
	entries, err := os.ReadDir(journal)
	if err != nil {
		return nil, ErrUnavailable
	}
	out := []orphanCandidate{}
	for _, entry := range entries {
		body, err := deployment.PrivateRead(filepath.Join(journal, entry.Name()), 4096)
		if err != nil {
			continue
		}
		var candidate keyCandidateRecord
		if json.Unmarshal(body, &candidate) != nil || !filepath.IsAbs(candidate.Path) || filepath.Dir(candidate.Path) != filepath.Clean(directory) {
			continue
		}
		file := candidate.Path
		info, err := os.Lstat(file)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 256 {
			continue
		}
		key, err := readMasterKey(file)
		if err != nil {
			continue
		}
		digest := sha256.Sum256(key)
		clear(key)
		if candidate.Digest != hex.EncodeToString(digest[:]) {
			continue
		}
		idData, _ := json.Marshal([]string{file, info.ModTime().UTC().Format(time.RFC3339Nano), candidate.Digest})
		id := sha256.Sum256(idData)
		out = append(out, orphanCandidate{VaultOrphan{hex.EncodeToString(id[:]), info.ModTime().UTC().Format(time.RFC3339Nano)}, file, candidate.Digest, info})
	}
	return out, nil
}

// ListVaultOrphans is installation-local. Its public shape is IDs/dates only.
// Historic backup keys not yet registered must be protected explicitly first.
func ListVaultOrphans(vault, keyFile, directory string) ([]VaultOrphan, error) {
	v, err := OpenCustomerFiscalVault(vault, keyFile)
	if err != nil {
		return nil, err
	}
	defer v.Close()
	if err = referenceKeyFile(v, keyFile); err != nil {
		return nil, err
	}
	candidates, err := scanKeyDirectory(vault, directory)
	if err != nil {
		return nil, err
	}
	out := []VaultOrphan{}
	err = sqlitewrite.Run(context.Background(), v.db, fiscal.ErrConflict, func(tx *sql.Tx) error {
		if _, err := v.checkKeyTx(context.Background(), tx); err != nil {
			return err
		}
		out = out[:0]
		for _, c := range candidates {
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM fiscal_vault_key_references WHERE digest=?`, c.digest).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				out = append(out, c.public)
			}
		}
		return nil
	})
	return out, err
}

// ProtectVaultKey registers a customer's historic backup key without exporting it.
func ProtectVaultKey(vault, keyFile, backup string) error {
	v, err := OpenCustomerFiscalVault(vault, keyFile)
	if err != nil {
		return err
	}
	defer v.Close()
	return referenceKeyFile(v, backup)
}

// Cleanup is explicit and local; references and file identity are rechecked
// under the same writer lock used by rotation. Copies of referenced keys are protected.
func RemoveVaultOrphan(vault, keyFile, directory, id, confirmation string) error {
	if len(id) != 64 || confirmation != "DELETE "+id {
		return fiscal.ErrInvalid
	}
	v, err := OpenCustomerFiscalVault(vault, keyFile)
	if err != nil {
		return err
	}
	defer v.Close()
	if err = referenceKeyFile(v, keyFile); err != nil {
		return err
	}
	candidates, err := scanKeyDirectory(vault, directory)
	if err != nil {
		return err
	}
	return sqlitewrite.Run(context.Background(), v.db, fiscal.ErrConflict, func(tx *sql.Tx) error {
		if _, err := v.checkKeyTx(context.Background(), tx); err != nil {
			return err
		}
		for _, c := range candidates {
			if c.public.ID != id {
				continue
			}
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM fiscal_vault_key_references WHERE digest=?`, c.digest).Scan(&n); err != nil {
				return err
			}
			if n != 0 {
				return fiscal.ErrConflict
			}
			info, err := os.Lstat(c.path)
			if err != nil || !os.SameFile(info, c.info) {
				return fiscal.ErrConflict
			}
			// Recompute the identifier immediately before removal; changed contents are refused.
			fresh, err := scanKeyDirectory(vault, directory)
			if err != nil {
				return err
			}
			same := false
			for _, f := range fresh {
				if f.public.ID == id && os.SameFile(info, f.info) {
					same = true
				}
			}
			if !same {
				return fiscal.ErrConflict
			}
			if err = os.Remove(c.path); err != nil {
				return ErrUnavailable
			}
			dir, err := os.Open(directory)
			if err != nil {
				return ErrUnavailable
			}
			defer dir.Close()
			return dir.Sync()
		}
		return fiscal.ErrNotFound
	})
}
