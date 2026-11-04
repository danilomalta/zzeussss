package localapi

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRetailFrontendRealHTTPBridge(t *testing.T) {
	f, _ := storesSetup(t)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go func() { _ = f.app.Listener(listener) }()
	t.Cleanup(func() { _ = f.app.Shutdown(); _ = listener.Close() })
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "../../../frontend-web/tests/retail.bridge.mjs")
	scope, _ := json.Marshal(map[string]string{"tenant_id": f.owner.TenantID, "store_id": f.owner.StoreID, "device_id": f.device.DeviceID, "identity_id": f.owner.OwnerID})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Env = append(os.Environ(), "TITAN_RETAIL_TEST_BASE=http://"+listener.Addr().String(), "TITAN_RETAIL_TEST_TOKEN="+f.token, "TITAN_RETAIL_TEST_SCOPE="+string(scope))
	raw, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("retail bridge: %v: %s", e, raw)
	}
	var count int
	if e = f.db.QueryRow(`SELECT count(*) FROM stores WHERE id='bridge-store'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("bridge duplicated store", count, e)
	}
	t.Log("frontend client -> actual HTTP -> SQLite: original result recovered without replay")
}
