package fiscalproduction

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomerInitializationLoserLeavesNoOrphan(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault.sqlite")
	keys := []string{filepath.Join(dir, "one.key"), filepath.Join(dir, "two.key")}
	outputs := make([]bytes.Buffer, 2)
	errs := concurrentVaultCalls(2, func(i int) error { return CreateCustomerFiscalVault(vault, keys[i], &outputs[i]) })
	wins := 0
	for i, err := range errs {
		if err == nil {
			wins++
			v, e := OpenCustomerFiscalVault(vault, keys[i])
			if e != nil {
				t.Fatal("winning key unusable")
			}
			v.Close()
		} else if _, e := os.Lstat(keys[i]); !os.IsNotExist(e) {
			t.Errorf("initialization loser left its key file (%d)", i)
		}
	}
	if wins != 1 {
		t.Fatal(fmt.Sprintf("initialization winners: %d", wins))
	}
}
