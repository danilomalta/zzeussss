package localapi

// Explicit installed-screen acceptance. Only identities, catalogue, licence and
// initial stock are fixture provisioning. Business POSTs originate in React UI.
import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/stock"
)

func TestPilotInstalledCashPartialReceivingRestore(t *testing.T) { runInstalledPilot(t, false) }

func TestPilotInstalledCashClosingCustodyRestore(t *testing.T) { runInstalledPilot(t, true) }

func runInstalledPilot(t *testing.T, closing bool) {
	t.Helper()
	desktop, local, screen := os.Getenv("TITAN_INSTALLED_DESKTOP"), os.Getenv("TITAN_INSTALLED_LOCAL"), os.Getenv("TITAN_INSTALLED_SCREEN")
	if desktop == "" && local == "" && screen == "" {
		t.Skip("opt-in installed application acceptance; supply three TITAN_INSTALLED paths and -pilot-backup-cli")
	}
	for _, p := range []string{desktop, local, screen, *pilotBackupCLI} {
		if !filepath.IsAbs(p) {
			t.Fatal("explicit absolute installed binaries/controller required")
		}
		if v, e := os.Stat(p); e != nil || !v.Mode().IsRegular() {
			t.Fatal("installed acceptance input unavailable")
		}
	}
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Finance}), 200)
	policyPerson(t, f.db, f.owner, "pilot-reviewer", "manager")
	stockRequest(t, f, stock.Input{OperationID: "installed-initial-stock", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Disposable opening balance"}, 201)
	station := pilotPrepareCLIStation(t, f)
	var seq int
	var name, dbPath, sku string
	if e := f.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); e != nil {
		t.Fatal(e)
	}
	if e := f.db.QueryRow("SELECT sku FROM products WHERE id=?", f.product).Scan(&sku); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	os.Chmod(root, 0700)
	keys, _ := json.Marshal(map[string]string{"test-issuer": base64.StdEncoding.EncodeToString(f.private.Public().(ed25519.PublicKey))})
	trusted := filepath.Join(root, "issuer-public.json")
	if e := os.WriteFile(trusted, keys, 0600); e != nil {
		t.Fatal(e)
	}
	reserve, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	var child *exec.Cmd
	transport := &http.Transport{Proxy: nil}
	proxyTransport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	defer proxyTransport.CloseIdleConnections()
	stop := func() {
		transport.CloseIdleConnections()
		proxyTransport.CloseIdleConnections()
		if child != nil {
			if runtime.GOOS == "windows" {
				// Explicit crash/restart injection; Windows cannot send os.Interrupt.
				if err := child.Process.Kill(); err != nil {
					t.Error("installed crash injection failed")
				}
			} else {
				_ = child.Process.Signal(os.Interrupt)
			}
			done := make(chan error, 1)
			go func() { done <- child.Wait() }()
			select {
			case err := <-done:
				if err != nil && runtime.GOOS != "windows" {
					t.Error("installed server exit failed")
				}
			case <-time.After(5 * time.Second):
				child.Process.Kill()
				<-done
				t.Error("installed server drain failed")
			}
			child = nil
		}
	}
	defer stop()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	start := func(path string) {
		child = exec.Command(local, "serve", "--db", path, "--station", station, "--issuer-keys", trusted, "--port", strconv.Itoa(port))
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		child.Env = installedEnvironment()
		if err := child.Start(); err != nil {
			t.Fatal("installed server spawn failed")
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			r, err := client.Get(endpoint + "/local/v1/health")
			if err == nil {
				r.Body.Close()
				if r.StatusCode == 200 {
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatal("installed server readiness failed")
	}
	start(dbPath)
	target, _ := url.Parse(endpoint)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = proxyTransport
	var dropped atomic.Bool
	var finalPosts atomic.Int64
	proxy.ModifyResponse = func(r *http.Response) error {
		if r.Request.Method == "POST" && (r.Request.URL.Path == "/local/v1/sales" || strings.HasSuffix(r.Request.URL.Path, "/finalize-cash")) {
			finalPosts.Add(1)
			if (r.StatusCode == 200 || r.StatusCode == 201) && dropped.CompareAndSwap(false, true) {
				io.Copy(io.Discard, r.Body)
				r.Body.Close()
				return io.ErrUnexpectedEOF
			}
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		io.WriteString(w, `{"code":"discarded_response_for_acceptance"}`)
	}
	network := httptest.NewServer(proxy)
	defer network.Close()
	observer := &pilotHTTP{t: t, service: &pilotService{url: endpoint}, client: client, token: f.token}
	payload := map[string]any{"binary": desktop, "root": root, "url": network.URL, "owner": f.owner.OwnerID, "password": "senha-forte-de-teste-1", "token": f.token, "product": f.product, "location": f.shelf, "sku": sku}
	screenStep := func(phase string, resume bool) {
		t.Helper()
		payload["business"] = phase
		payload["resume"] = resume
		payload["owner"] = f.owner.OwnerID
		if phase == "approve" || phase == "receive-bag" {
			payload["owner"] = "pilot-reviewer"
		}
		if phase == "receive-bag" {
			raw, _ := json.Marshal(map[string]string{"identity_id": "pilot-reviewer", "password": "senha-forte-de-teste-1"})
			r, err := client.Post(endpoint+"/local/v1/login", "application/json", bytes.NewReader(raw))
			if err != nil {
				t.Fatal("receiver observation login unavailable")
			}
			var auth struct {
				Token string `json:"token"`
			}
			err = json.NewDecoder(r.Body).Decode(&auth)
			r.Body.Close()
			if err != nil || r.StatusCode != 200 || auth.Token == "" {
				t.Fatal("receiver observation login refused")
			}
			payload["token"] = auth.Token
		}
		body, _ := json.Marshal(payload)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", screen)
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "xvfb-run", "-a", "node", screen)
		}
		cmd.Env = installedEnvironment()
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.CombinedOutput()
		scan := bufio.NewScanner(bytes.NewReader(out))
		passed := false
		for scan.Scan() {
			var v map[string]any
			if json.Unmarshal(scan.Bytes(), &v) != nil {
				continue
			}
			if phase, ok := v["acceptancePhase"].(string); ok {
				t.Log("Screen phase:", phase)
			}
			if d, ok := v["businessDiagnostic"]; ok {
				clean, _ := json.Marshal(d)
				t.Log("UI control diagnostic:", string(clean))
			}
			if failure, ok := v["acceptanceFailure"].(string); ok {
				t.Log("Screen failure phase:", failure, "detail:", v["detail"])
			}
			if v["businessPhase"] == phase && v["passed"] == true {
				passed = true
			}
		}
		if err != nil || !passed {
			t.Fatalf("installed screen stage %s failed; credentials/output omitted (%v)", phase, err)
		}
	}
	screenStep("sale", false)
	if !dropped.Load() {
		t.Fatal("lost final response was not exercised")
	}
	stop()
	start(dbPath)
	screenStep("recover", true)
	if finalPosts.Load() != 1 {
		t.Fatal("recovery repeated fiscal/commercial finalization POST")
	}
	var closedCash string
	if closing {
		screenStep("close-bag", true)
		screenStep("receive-bag", true)
		var page struct {
			Items []struct {
				Cash string `json:"cash_session_id"`
			} `json:"items"`
		}
		observer.call("GET", "/local/v1/sales?limit=50&offset=0", nil, 200, &page)
		if len(page.Items) != 1 || page.Items[0].Cash == "" {
			t.Fatal("original drawer missing")
		}
		closedCash = page.Items[0].Cash
	} else {
		screenStep("supply", true)
		screenStep("approve", true)
		screenStep("finance", true)
	}

	snapshot := func() map[string]any {
		out := map[string]any{}
		paths := []string{"/local/v1/stock/balance?product_id=" + f.product + "&location_id=" + f.shelf, "/local/v1/sales?limit=50&offset=0", "/local/v1/purchase-orders?limit=50&offset=0", "/local/v1/finance/payables?limit=50&offset=0"}
		if closing {
			paths = append(paths, "/local/v1/cash/bags?limit=50&offset=0", "/local/v1/cash/sessions/"+closedCash+"/custody", "/local/v1/cash/sessions/"+closedCash+"/report")
		} else {
			paths = append(paths, "/local/v1/finance/payables/installed-payable", "/local/v1/purchase-deliveries/installed-trip")
		}
		for _, path := range paths {
			var v any
			observer.call("GET", path, nil, 200, &v)
			out[path] = v
		}
		return out
	}
	before := snapshot()
	stop()
	restored := pilotCLIBackupRoundTrip(t, f, dbPath, station)
	restoredStation := filepath.Join(filepath.Dir(restored), "customer-restored-station.json")
	stationBytes, err := os.ReadFile(station)
	if err != nil {
		t.Fatal("fixture station export failed")
	}
	if err = os.WriteFile(restoredStation, stationBytes, 0600); err != nil {
		t.Fatal("fixture station restore failed")
	}
	clear(stationBytes)
	station = restoredStation
	start(restored)
	observer.call("GET", "/local/v1/me", nil, 401, nil)
	// Repeated real app launches count against the durable login budget.
	// Respect the server's Retry-After once; never bypass/reset the security store.
	loginBody, _ := json.Marshal(map[string]string{"identity_id": f.owner.OwnerID, "password": "senha-forte-de-teste-1"})
	for attempt := 0; attempt < 2; attempt++ {
		response, err := client.Post(endpoint+"/local/v1/login", "application/json", bytes.NewReader(loginBody))
		if err != nil {
			t.Fatal("restored login unavailable")
		}
		if response.StatusCode == 429 && attempt == 0 {
			seconds, err := strconv.Atoi(response.Header.Get("Retry-After"))
			response.Body.Close()
			if err != nil || seconds < 1 || seconds > 60 {
				t.Fatal("invalid authentication retry policy")
			}
			t.Logf("Authentication budget preserved; observing Retry-After %ds without changing any timeout", seconds)
			time.Sleep(time.Duration(seconds) * time.Second)
			continue
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			t.Fatal("restored login refused after bounded wait")
		}
		var auth struct {
			Token string `json:"token"`
		}
		err = json.NewDecoder(response.Body).Decode(&auth)
		response.Body.Close()
		if err != nil || auth.Token == "" {
			t.Fatal("invalid restored authentication response")
		}
		observer.token = auth.Token
		break
	}
	payload["token"] = observer.token
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("installed workflow changed after CLI restore")
	}
	if closing {
		screenStep("closed-restored", true)
	} else {
		screenStep("restored", true)
	}
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	checks := []struct {
		sql  string
		want int64
	}{{"SELECT COUNT(*) FROM sales", 1}, {"SELECT SUM(total_cents) FROM sales WHERE status='committed'", 3198}, {"SELECT COUNT(*) FROM purchase_orders", 1}, {"SELECT COUNT(*) FROM purchase_receipts", 1}, {"SELECT SUM(quantity_milli) FROM stock_movements", 4000}, {"SELECT SUM(accepted_milli) FROM purchase_receipts", 1000}, {"SELECT SUM(delivered_milli) FROM purchase_receipts", 2000}, {"SELECT SUM(opening_cents) FROM cash_sessions", 1000}, {"SELECT COUNT(*) FROM payable_cash_payments", 1}, {"SELECT SUM(amount_cents) FROM cash_movements", 2173}, {"SELECT COUNT(*) FROM supplier_payables WHERE state='approved' AND total_cents=1025", 1}}
	if closing {
		checks = []struct {
			sql  string
			want int64
		}{
			{"SELECT COUNT(*) FROM sales WHERE status='committed' AND total_cents=3198", 1},
			{"SELECT SUM(quantity_milli) FROM stock_movements", 3000},
			{"SELECT COUNT(*) FROM cash_closures WHERE declared_cents=3198 AND expected_cents=3198 AND declared_cents-expected_cents=0", 1},
			{"SELECT COUNT(*) FROM cash_sessions WHERE closed_at IS NOT NULL", 1},
			{"SELECT COUNT(*) FROM cash_adjustments WHERE kind='withdrawal' AND amount_cents=1000", 1},
			{"SELECT SUM(amount_cents) FROM cash_movements", 2198},
			{"SELECT COUNT(*) FROM cash_bags WHERE state='received' AND declared_cents=1000 AND counted_cents=1000 AND difference_cents=0 AND sender_id<>receiver_id AND operator_id<>receiver_id", 1},
			{"SELECT COUNT(*) FROM cash_bag_operations", 3},
		}
	}
	for _, check := range checks {
		var got int64
		if err = db.QueryRow(check.sql).Scan(&got); err != nil || got != check.want {
			t.Fatal("installed outcome mismatch:", check.sql)
		}
	}
	if closing {
		t.Log("PASS: installed drawer count and official closure; sealed bag independently received without rebooking cash; records preserved after encrypted CLI restore")
	} else {
		t.Log("PASS: installed React screens -> real native HTTP -> SQLite; lost response recovery once; partial receiving; same IDs after actual encrypted backup CLI restore. No fiscal emission or commercial data.")
	}
}
func installedEnvironment() []string {
	out := []string{}
	for _, k := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT", "TEMP", "TMP", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}
