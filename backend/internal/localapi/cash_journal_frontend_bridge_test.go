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
func TestCashJournalFrontendBridge(t *testing.T) {
	script := os.Getenv("TITAN_CASH_JOURNAL_BRIDGE")
	if script == "" {
		t.Skip("optional: set TITAN_CASH_JOURNAL_BRIDGE to frontend-web/tests/cashJournal.bridge.mjs")
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal("Node required for requested bridge", e)
	}
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	if code, body := request(t, f.app, "POST", "/local/v1/sales/"+cashPlanID+"/cancel", cancelBody, f.token); code != 200 {
		t.Fatalf("synthetic reversal: %d %s", code, body)
	}
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-Á QA","opening_cents":0}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-Á QA","operation_id":"close-unicode","declared_cents":0}`, 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-%literal","opening_cents":0}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-%literal","operation_id":"close-literal","declared_cents":0}`, 200)
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
