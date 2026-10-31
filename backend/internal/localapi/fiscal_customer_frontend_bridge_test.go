package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Explicit opt-in, like the existing delivery/accounting frontend bridges.
// Identity, contract and SQLite are disposable; never uses an installed service.
func TestFiscalCustomerFrontendBridge(t *testing.T) {
	script := os.Getenv("TITAN_FISCAL_CUSTOMER_BRIDGE")
	if script == "" {
		t.Skip("optional: set TITAN_FISCAL_CUSTOMER_BRIDGE to frontend-web/tests/fiscalCustomer.bridge.mjs")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node required for requested fiscal frontend bridge")
	}
	f := fiscalFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.Shutdown()
	defer listener.Close()
	bootstrap, err := json.Marshal(map[string]string{"url": "http://" + listener.Addr().String(), "token": f.token})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script)
	// Disposable authentication travels over stdin, never argv or test output.
	cmd.Stdin = bytes.NewReader(bootstrap)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fiscal frontend bridge failed: %v\n%s", err, out)
	}
	var count int
	if err = f.db.QueryRow(`SELECT count(*) FROM fiscal_profiles`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("expected exactly three immutable profile revisions: count=%d err=%v", count, err)
	}
	t.Log(string(out))
}
