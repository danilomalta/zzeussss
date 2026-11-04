package localapi

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReceivablesFrontendRealHTTPBridge(t *testing.T) {
	script := os.Getenv("TITAN_RECEIVABLES_BRIDGE")
	if script == "" {
		t.Skip("opt in: TITAN_RECEIVABLES_BRIDGE=frontend-web/tests/receivables.bridge.mjs")
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal("Node required for requested bridge", e)
	}
	f := arSetup(t)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.Shutdown()
	defer listener.Close()
	session, _ := json.Marshal(map[string]string{"tenant_id": f.owner.TenantID, "store_id": f.owner.StoreID, "identity_id": f.owner.OwnerID, "device_id": f.device.DeviceID})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script)
	command.Env = append(os.Environ(), "TITAN_BRIDGE_URL=http://"+listener.Addr().String(), "TITAN_BRIDGE_TOKEN="+f.token, "TITAN_BRIDGE_SESSION="+string(session))
	output, e := command.CombinedOutput()
	if e != nil {
		t.Fatalf("real frontend bridge: %v\n%s", e, output)
	}
	t.Log(string(output))
	var amount, count int64
	if e = f.db.QueryRow(`SELECT sum(amount_cents),count(*) FROM cash_adjustments WHERE tenant_id=? AND kind='supply'`, f.owner.TenantID).Scan(&amount, &count); e != nil || amount != 1000 || count != 2 {
		t.Fatal("cash did not match real frontend receipts", amount, count, e)
	}
}

// This opt-in test closes whole Chrome processes between financial requests.
// Both SQLite and the Chromium profile are disposable; no installed service is used.
func TestReceivablesBrowserRestartRecovery(t *testing.T) {
	script := os.Getenv("TITAN_RECEIVABLES_BROWSER_BRIDGE")
	if script == "" {
		t.Skip("opt in: TITAN_RECEIVABLES_BROWSER_BRIDGE=frontend-web/tests/receivables.browser.mjs")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	f := arSetup(t)
	assets := filepath.Join(filepath.Dir(script), "../src/core/receivables")
	for _, name := range []string{"client.mjs", "recovery.mjs"} {
		file := filepath.Join(assets, name)
		f.app.Get("/ar-recovery/"+name, func(c *fiber.Ctx) error { return c.SendFile(file) })
	}
	f.app.Get("/ar-recovery/", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html")
		return c.SendString("<!doctype html><title>Disposable recovery test</title>")
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.Shutdown()
	defer listener.Close()
	session, _ := json.Marshal(map[string]string{"tenant_id": f.owner.TenantID, "store_id": f.owner.StoreID, "identity_id": f.owner.OwnerID, "device_id": f.device.DeviceID})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script)
	command.Env = append(os.Environ(), "TITAN_BRIDGE_URL=http://"+listener.Addr().String(), "TITAN_BRIDGE_TOKEN="+f.token, "TITAN_BRIDGE_SESSION="+string(session))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("browser recovery: %v\n%s", err, output)
	}
	t.Log(string(output))
	var amount, count int64
	if err = f.db.QueryRow(`SELECT sum(amount_cents),count(*) FROM cash_adjustments WHERE tenant_id=? AND kind='supply'`, f.owner.TenantID).Scan(&amount, &count); err != nil || amount != 1000 || count != 2 {
		t.Fatal("browser recovery duplicated cash", amount, count, err)
	}
}
