package fiscalproduction

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func concurrentVaultCalls(n int, call func(int) error) []error {
	start := make(chan struct{})
	out := make([]error, n)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; out[i] = call(i) }(i)
	}
	close(start)
	wg.Wait()
	return out
}
func vaultHandles(t *testing.T, v *FiscalSecretVault, path string) []*FiscalSecretVault {
	t.Helper()
	out := make([]*FiscalSecretVault, 4)
	for i := range out {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		out[i] = &FiscalSecretVault{db: db, aead: v.aead}
	}
	return out
}
func TestVaultConcurrentInitializeKeyControl(t *testing.T) {
	v, path := testVault(t)
	if _, err := v.db.Exec(`DELETE FROM fiscal_vault_control`); err != nil {
		t.Fatal(err)
	}
	handles := vaultHandles(t, v, path)
	errs := concurrentVaultCalls(4, func(i int) error { return handles[i].initializeKeyControl() })
	for _, err := range errs {
		if err != nil {
			t.Errorf("initialization failed: %v", err)
		}
	}
	var count int
	if err := v.db.QueryRow(`SELECT count(*) FROM fiscal_vault_control`).Scan(&count); err != nil || count != 1 {
		t.Fatal("inconsistent key marker")
	}
	if err := v.bindTenant(context.Background(), "client"); err != nil {
		t.Fatal(err)
	}
}
func TestVaultConcurrentBindTenant(t *testing.T) {
	v, path := testVault(t)
	handles := vaultHandles(t, v, path)
	for _, err := range concurrentVaultCalls(4, func(i int) error { return handles[i].bindTenant(context.Background(), "client") }) {
		if err != nil {
			t.Errorf("same tenant bind failed: %v", err)
		}
	}
	var tenant string
	if err := v.db.QueryRow(`SELECT tenant FROM fiscal_vault_control WHERE id=1`).Scan(&tenant); err != nil || tenant != "client" {
		t.Fatal("wrong tenant binding")
	}
	if err := v.bindTenant(context.Background(), "other"); err == nil {
		t.Fatal("foreign tenant accepted")
	}
}
func TestVaultConcurrentPut(t *testing.T) {
	v, path := testVault(t)
	handles := vaultHandles(t, v, path)
	errs := concurrentVaultCalls(4, func(i int) error {
		return handles[i].Put(context.Background(), "client", "token", []byte(fmt.Sprintf("SyntheticCompleteValue%d", i)))
	})
	for _, err := range errs {
		if err != nil {
			t.Errorf("put failed: %v", err)
		}
	}
	value, err := v.read(context.Background(), "client", "token")
	if err != nil {
		t.Fatal(err)
	}
	valid := false
	for i := 0; i < 4; i++ {
		if string(value) == fmt.Sprintf("SyntheticCompleteValue%d", i) {
			valid = true
		}
	}
	if !valid {
		t.Fatal("partial or corrupted secret")
	}
	if _, err := v.read(context.Background(), "other", "token"); err == nil {
		t.Fatal("cross tenant read")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("SyntheticCompleteValue")) {
		t.Fatal("plaintext in vault")
	}
}
func TestVaultConcurrentRotateCustomerMasterKey(t *testing.T) {
	for _, samePath := range []bool{false, true} {
		t.Run(fmt.Sprint(samePath), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "vault.sqlite")
			old := filepath.Join(dir, "old.key")
			var display bytes.Buffer
			if err := CreateCustomerFiscalVault(path, old, &display); err != nil {
				t.Fatal(err)
			}
			v, err := OpenCustomerFiscalVault(path, old)
			if err != nil {
				t.Fatal(err)
			}
			for _, purpose := range []string{"token", "csc"} {
				if err = v.Put(context.Background(), "client", purpose, []byte("SyntheticOriginal"+purpose)); err != nil {
					t.Fatal(err)
				}
			}
			v.Close()
			targets := []string{filepath.Join(dir, "one.key"), filepath.Join(dir, "two.key")}
			if samePath {
				targets[1] = targets[0]
			}
			outputs := make([]bytes.Buffer, 2)
			errs := concurrentVaultCalls(2, func(i int) error { return RotateCustomerMasterKey(path, old, targets[i], &outputs[i]) })
			winners := 0
			winner := -1
			for i, err := range errs {
				if err == nil {
					winners++
					winner = i
				} else if !errors.Is(err, fiscal.ErrConflict) {
					t.Errorf("loser must return conflict: %v", err)
				}
			}
			if winners != 1 {
				t.Errorf("rotation winners: %d", winners)
			}
			for i, target := range targets {
				if errs[i] != nil && (!samePath) {
					if _, err := os.Lstat(target); !os.IsNotExist(err) {
						t.Errorf("loser left an orphan key file")
					}
				}
			}
			if winner < 0 {
				return
			}
			active, err := OpenCustomerFiscalVault(path, targets[winner])
			if err != nil {
				t.Fatal("winning key cannot open vault")
			}
			defer active.Close()
			for _, purpose := range []string{"token", "csc"} {
				plain, err := active.read(context.Background(), "client", purpose)
				if err != nil || string(plain) != "SyntheticOriginal"+purpose {
					t.Fatal("key/ciphertext inconsistency or secret corruption")
				}
			}
			key, err := os.ReadFile(targets[winner])
			if err != nil || !bytes.Equal(key, outputs[winner].Bytes()) {
				t.Fatal("displayed key differs from committed key")
			}
			if oldVault, err := OpenCustomerFiscalVault(path, old); err == nil {
				oldVault.Close()
				t.Fatal("old key still active")
			}
		})
	}
}
