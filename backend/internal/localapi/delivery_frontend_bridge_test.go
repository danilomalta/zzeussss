package localapi

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Explicitly opt in: the backend repository can be tested without a frontend checkout.
func TestDeliveryFrontendBridge(t *testing.T) {
	script := os.Getenv("TITAN_DELIVERY_BRIDGE")
	if script == "" {
		t.Skip("optional: set TITAN_DELIVERY_BRIDGE to frontend-web/tests/deliveryCycle.bridge.mjs")
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal("Node required for requested bridge", e)
	}
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.Shutdown()
	defer listener.Close()
	session, _ := json.Marshal(map[string]any{"tenant_id": f.owner.TenantID, "store_id": f.owner.StoreID, "identity_id": f.owner.OwnerID, "device_id": f.device.DeviceID, "expires_unix": time.Now().Add(time.Minute).Unix()})
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script)
	cmd.Env = append(os.Environ(), "TITAN_BRIDGE_URL=http://"+listener.Addr().String(), "TITAN_BRIDGE_TOKEN="+f.token, "TITAN_BRIDGE_SESSION="+string(session))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("frontend HTTP contract: %v\n%s", e, out)
	}
	t.Log(string(out))
}
