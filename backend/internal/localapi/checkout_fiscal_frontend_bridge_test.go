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

// Reuse the existing disposable HTTP/SQLite/mTLS acceptance fixture. The
// JavaScript client talks to this listener, never to an installed company.
func runCheckoutNFCeFrontendBridge(t *testing.T, p *productionFixture, policy string) {
	t.Helper()
	script := os.Getenv("TITAN_CHECKOUT_NFCE_BRIDGE")
	if script == "" {
		return // Explicit opt-in, consistent with the other frontend bridges.
	}
	t.Run("frontend_real_http_bridge", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _ = p.f.app.Listener(listener) }()
		defer p.f.app.Shutdown()
		defer listener.Close()
		bootstrap, err := json.Marshal(map[string]any{
			"url": "http://" + listener.Addr().String(), "token": p.f.token,
			"session": map[string]string{"tenant_id": p.f.owner.TenantID, "store_id": p.f.owner.StoreID, "identity_id": p.f.owner.OwnerID, "device_id": p.f.device.DeviceID},
			"product": p.f.product, "location": p.f.shelf, "policy": policy,
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", script)
		cmd.Stdin = bytes.NewReader(bootstrap)
		before := p.issues.Load()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("NFC-e JavaScript HTTP bridge: %v\n%s", err, out)
		}
		if p.issues.Load() != before+1 {
			t.Fatal("JavaScript checkout did not emit exactly once to the disposable authority")
		}
		var count int
		if err = p.f.db.QueryRow(`SELECT count(*) FROM sales WHERE tenant_id=? AND id='checkout-browser'`, p.f.owner.TenantID).Scan(&count); err != nil || count != 1 {
			t.Fatal("JavaScript checkout did not retain exactly one sale", count, err)
		}
		t.Log(string(out))
	})
}
