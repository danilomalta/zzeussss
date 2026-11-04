package fiscalproduction

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomerVaultDiagnosticReadOnlyAndNeverConfusesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.sqlite")
	key := filepath.Join(dir, "key")
	copy := filepath.Join(dir, "copy")
	if CreateCustomerFiscalVault(path, key, &bytes.Buffer{}) != nil {
		t.Fatal("init")
	}
	if ExportCustomerMasterKey(key, copy) != nil {
		t.Fatal("export")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	available, exported := CustomerVaultDiagnostic(context.Background(), path, key, copy)
	if !available || !exported {
		t.Fatal("available copy not recognized")
	}
	if a, e := CustomerVaultDiagnostic(context.Background(), path, key, key); !a || e {
		t.Fatal("active key is not a backup copy")
	}
	other := filepath.Join(dir, "other")
	if CreateCustomerFiscalVault(filepath.Join(dir, "other.sqlite"), other, &bytes.Buffer{}) != nil {
		t.Fatal("other")
	}
	if a, e := CustomerVaultDiagnostic(context.Background(), path, other, copy); a || e {
		t.Fatal("foreign key")
	}
	if a, e := CustomerVaultDiagnostic(context.Background(), path, key, other); !a || e {
		t.Fatal("foreign copy")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("read-only diagnostic changed vault")
	}
	missing := filepath.Join(dir, "missing")
	if a, e := CustomerVaultDiagnostic(context.Background(), missing, key, copy); a || e {
		t.Fatal("missing vault")
	}
	if _, e := os.Stat(missing); !os.IsNotExist(e) {
		t.Fatal("created missing vault")
	}
}
