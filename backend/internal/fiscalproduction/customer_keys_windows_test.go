package fiscalproduction

import (
	"testing"
	"titansystem-backend/internal/privatefile"
)

// Reuse all existing key-generation/export/rotation/isolation assertions. Only
// the disposable parent is created with a Windows DACL instead of POSIX bits.
func TestWindowsCustomerKeyLifecycle(t *testing.T) {
	dir, err := privatefile.TempDir(t.TempDir(), "customer-")
	if err != nil {
		t.Fatal("private customer key directory unavailable")
	}
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Run("generation_backup_rotation", TestCustomerMasterKeyGenerationBackupRotationAndStaleWriter)
	t.Run("independent_customer_keys", TestIndependentCustomerKeysCannotOpenOtherVault)
}
