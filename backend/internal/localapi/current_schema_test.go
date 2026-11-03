package localapi

import (
	"testing"
	"titansystem-backend/internal/localdb"
)

func currentSchemaVersion(t *testing.T) int {
	t.Helper()
	version, err := localdb.LatestSchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	return version
}
