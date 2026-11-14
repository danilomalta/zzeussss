package localapi

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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/stationfile"
	"titansystem-backend/internal/localdb/supplycycle"
)

// Supplier/receiving preconditions use the existing real HTTP operations; their
// installed-screen proof is TestTradeInstalledScreenAuthenticatedReceivingFinance.
// Approval, withdrawal, payment and recovery below originate in installed forms.
func TestTradeInstalledPaymentApprovalRecoveryRestartRestore(t *testing.T) {
	desktop, local, screen := os.Getenv("TITAN_INSTALLED_DESKTOP"), os.Getenv("TITAN_INSTALLED_LOCAL"), os.Getenv("TITAN_INSTALLED_SCREEN")
	if desktop == "" && local == "" && screen == "" {
		t.Skip("opt-in installed payment acceptance; supply installed paths and backup CLI")
	}
	for _, p := range []string{desktop, local, screen, *pilotBackupCLI} {
		if !filepath.IsAbs(p) {
			t.Fatal("absolute installed acceptance paths required")
		}
		if v, e := os.Stat(p); e != nil || !v.Mode().IsRegular() {
			t.Fatal("installed acceptance input unavailable")
		}
	}
	b, supplier := newTradeHTTP(t, true), newTradeHTTP(t, false)
	d := tradeDecide(t, b, supplier)
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(d), 200, nil)
	cycleCall(t, b.f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-one", 3000), 200)
	for i, action := range []string{"load", "arrive"} {
		cycleCall(t, b.f, "POST", cycleDelivery+"/actions", supplycycle.ActionInput{OperationID: "action-" + action, ExpectedRevision: int64(i + 1), Action: action, EvidenceReference: "Synthetic physical observation", ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Explicit observation"}, 200)
	}
	cycleCall(t, b.f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 200)
	finance := cycleFinanceInput()
	finance.PayableID = "authenticated-payable"
	finance.TotalCents = 3150
	finance.Installments = []payables.Installment{{ID: "accepted-part", DueOn: time.Now().UTC().Format("2006-01-02"), AmountCents: 3150}}
	cycleCall(t, b.f, "POST", cycleDelivery+"/payable", finance, 200)
	for _, role := range []string{"accountant", "cashier"} {
		id := "payment-" + role
		policyPerson(t, b.f.db, b.f.owner, id, role)
		token := b.p.login(id)
		original := b.p.token
		b.p.token = token
		b.p.call("POST", payablesPath+"/authenticated-payable/approve", map[string]string{"operation_id": "denied-" + role, "reason": "Must be independently authorized"}, 403, nil)
		b.p.token = original
	}
	b.p.call("POST", payablesPath+"/authenticated-payable/approve", map[string]string{"operation_id": "denied-creator", "reason": "Creator cannot approve own obligation"}, 403, nil)
	root := privatePilotDir(t)
	station := filepath.Join(root, "station.json")
	raw, e := json.Marshal(stationfile.Station{TenantID: b.f.device.TenantID, StoreID: b.f.device.StoreID, DeviceID: b.f.device.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(b.key)})
	if e != nil {
		t.Fatal("synthetic station encoding failed")
	}
	if e = os.WriteFile(station, raw, 0600); e != nil {
		t.Fatal("synthetic station unavailable")
	}
	clear(raw)
	keys, _ := json.Marshal(map[string]string{"test-issuer": base64.StdEncoding.EncodeToString(b.f.private.Public().(ed25519.PublicKey))})
	trusted := filepath.Join(root, "issuer-public.json")
	if e = os.WriteFile(trusted, keys, 0600); e != nil {
		t.Fatal("public issuer unavailable")
	}
	var seq int
	var name, dbPath string
	if e = b.f.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); e != nil {
		t.Fatal(e)
	}
	if e = b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
		t.Fatal("fixture authority shutdown failed")
	}
	if e = b.f.db.Close(); e != nil {
		t.Fatal("fixture database close failed")
	}
	reserve, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	tr := &http.Transport{Proxy: nil}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	var child *exec.Cmd
	stop := func() {
		tr.CloseIdleConnections()
		if child == nil {
			return
		}
		if runtime.GOOS == "windows" {
			e = child.Process.Kill()
		} else {
			e = child.Process.Signal(os.Interrupt)
		}
		if e != nil {
			t.Error("native stop signal failed")
		}
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		select {
		case err := <-done:
			if err != nil && runtime.GOOS != "windows" {
				t.Error("native shutdown failed")
			}
		case <-time.After(5 * time.Second):
			child.Process.Kill()
			<-done
			t.Error("native shutdown deadline exceeded")
		}
		child = nil
	}
	defer stop()
	start := func(path string) {
		child = exec.Command(local, "serve", "--db", path, "--station", station, "--issuer-keys", trusted, "--port", strconv.Itoa(port))
		child.Env = installedEnvironment()
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		if e := child.Start(); e != nil {
			t.Fatal("native launch failed")
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			r, e := client.Get(endpoint + "/local/v1/health")
			if e == nil {
				r.Body.Close()
				if r.StatusCode == 200 {
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatal("native server readiness failed")
	}
	start(dbPath)
	observer := &pilotHTTP{t: t, service: &pilotService{url: endpoint}, client: client, token: b.f.token}
	var paymentPosts atomic.Int32
	var drop atomic.Bool
	drop.Store(true)
	var mu sync.Mutex
	var input payables.CashInput
	target, _ := url.Parse(endpoint)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = tr
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		if r.Method == "POST" && r.URL.Path == payablesPath+"/authenticated-payable/cash-payments" {
			paymentPosts.Add(1)
			data, err := io.ReadAll(io.LimitReader(r.Body, 8193))
			if err == nil {
				mu.Lock()
				_ = json.Unmarshal(data, &input)
				mu.Unlock()
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
		}
		director(r)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		if r.Request.Method == "POST" && r.Request.URL.Path == payablesPath+"/authenticated-payable/cash-payments" && (r.StatusCode == 200 || r.StatusCode == 201) && drop.CompareAndSwap(true, false) {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		io.WriteString(w, `{"code":"discarded_response_for_acceptance"}`)
	}
	gate := httptest.NewServer(proxy)
	defer gate.Close()
	ownerRoot, reviewerRoot := privatePilotDir(t), privatePilotDir(t)
	run := func(phase, actor, profile string, resume bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"binary": desktop, "root": profile, "url": gate.URL, "owner": actor, "password": "senha-forte-de-teste-1", "business": "trade", "tradePhase": phase, "resume": resume})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", screen)
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "xvfb-run", "-a", "node", screen)
		}
		cmd.Env = installedEnvironment()
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.CombinedOutput()
		passed := false
		scan := bufio.NewScanner(bytes.NewReader(out))
		for scan.Scan() {
			var v map[string]any
			if json.Unmarshal(scan.Bytes(), &v) != nil {
				continue
			}
			if s, ok := v["acceptancePhase"].(string); ok {
				t.Log("Installed phase:", s)
			}
			if s, ok := v["acceptanceFailure"].(string); ok {
				t.Log("Installed failure:", s, "detail:", v["detail"])
			}
			if v["businessPhase"] == "trade" && v["passed"] == true {
				passed = true
			}
		}
		if err != nil || !passed {
			t.Fatalf("installed payment %s failed; private output omitted (%v)", phase, err)
		}
	}
	run("payment-approve", "cycle-reviewer", reviewerRoot, false)
	var approved payables.Payable
	observer.call("GET", payablesPath+"/authenticated-payable", nil, 200, &approved)
	if approved.State != "approved" {
		t.Fatal("independent approval not persisted")
	}
	approvals := 0
	for _, event := range approved.History {
		if event.ToState == "approved" {
			approvals++
			if event.ActorID != "cycle-reviewer" || event.ActorID == approved.CreatorID {
				t.Fatal("approval did not preserve independent reviewer identity")
			}
		}
	}
	if approvals != 1 {
		t.Fatal("approval audit missing or duplicated")
	}
	run("payment-submit", b.f.owner.OwnerID, ownerRoot, false)
	if drop.Load() || paymentPosts.Load() != 1 {
		t.Fatal("payment lost response not exercised exactly once")
	}
	mu.Lock()
	original := input
	mu.Unlock()
	if original.OperationID == "" || original.PaymentID == "" {
		t.Fatal("original payment identity missing")
	}
	var receipt payables.CashResult
	observer.call("GET", pcPath+"/operations/"+original.OperationID, nil, 200, &receipt)
	if receipt.Payment.AmountCents != 3150 || receipt.Payment.ActorID != b.f.owner.OwnerID {
		t.Fatal("cash payment amount/actor changed")
	}
	var paid struct {
		Cash payables.CashSummary `json:"cash"`
	}
	observer.call("GET", payablesPath+"/authenticated-payable", nil, 200, &paid)
	if paid.Cash.AppliedCents != 3150 || paid.Cash.OutstandingCents != 0 {
		t.Fatal("paid obligation has inconsistent exact balance")
	}
	stop()
	start(dbPath)
	run("payment-recover", b.f.owner.OwnerID, ownerRoot, true)
	if paymentPosts.Load() != 1 {
		t.Fatal("recovery automatically resent payment")
	}
	var recovered payables.CashResult
	observer.call("GET", pcPath+"/operations/"+original.OperationID, nil, 200, &recovered)
	if !reflect.DeepEqual(receipt, recovered) {
		t.Fatal("restart recovery replaced original receipt")
	}
	var replay payables.CashResult
	observer.call("POST", payablesPath+"/authenticated-payable/cash-payments", original, 200, &replay)
	if !replay.Repeated || !reflect.DeepEqual(replay.Payment, receipt.Payment) {
		t.Fatal("explicit replay duplicated payment")
	}
	snapshot := func() map[string]any {
		m := map[string]any{}
		for _, path := range []string{cycleOrder + "/supply-cycle", cycleDelivery, payablesPath + "/authenticated-payable", pcPath + "/operations/" + original.OperationID} {
			var v any
			observer.call("GET", path, nil, 200, &v)
			m[path] = v
		}
		return m
	}
	before := snapshot()
	stop()
	restored := pilotCLIBackupRoundTrip(t, &stockHTTPFixture{httpContractFixture: b.f}, dbPath, station)
	start(restored)
	observer.call("GET", "/local/v1/me", nil, 401, nil)
	// Restore preserves the durable installation login budget. Respect the
	// advertised window, exactly as the existing installed pilot does.
	loginBody, _ := json.Marshal(map[string]string{"identity_id": b.f.owner.OwnerID, "password": "senha-forte-de-teste-1"})
	for attempt := 0; attempt < 2; attempt++ {
		r, err := client.Post(endpoint+"/local/v1/login", "application/json", bytes.NewReader(loginBody))
		if err != nil {
			t.Fatal("restored login unavailable")
		}
		if r.StatusCode == 429 && attempt == 0 {
			seconds, err := strconv.Atoi(r.Header.Get("Retry-After"))
			r.Body.Close()
			if err != nil || seconds < 1 || seconds > 60 {
				t.Fatal("invalid authentication retry policy")
			}
			t.Logf("Authentication budget preserved; observing Retry-After %ds", seconds)
			time.Sleep(time.Duration(seconds) * time.Second)
			continue
		}
		var auth struct {
			Token string `json:"token"`
		}
		err = json.NewDecoder(r.Body).Decode(&auth)
		r.Body.Close()
		if err != nil || r.StatusCode != 200 || auth.Token == "" {
			t.Fatal("restored login refused after bounded policy wait")
		}
		observer.token = auth.Token
		break
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("restoration changed source, approval or original payment")
	}
	run("payment-restored", b.f.owner.OwnerID, ownerRoot, true)
	stop()
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, c := range []struct {
		sql  string
		want int64
	}{
		{"SELECT count(*) FROM payable_cash_payments", 1},
		{"SELECT count(*) FROM supplier_payables WHERE state='approved' AND total_cents=3150", 1},
		{"SELECT sum(amount_cents) FROM cash_movements", -3150},
		{"SELECT count(*) FROM cash_adjustments WHERE kind='withdrawal' AND amount_cents=3150", 1},
		{"SELECT sum(opening_cents) FROM cash_sessions", 10000},
		{"SELECT sum(quantity_milli) FROM stock_movements", 2500},
	} {
		var n int64
		if err = db.QueryRow(c.sql).Scan(&n); err != nil || n != c.want {
			t.Fatal("restored ledger count or exact sum mismatch", err)
		}
	}
	t.Log("PASS: independent manager approval; accountant/cashier/creator403; explicit cash3150, one withdrawal/payment; lost reply and native restart recovered original receipt without POST; explicit replay idempotent; encrypted CLI restore preserved source/approval/payment/stock and revoked old sessions")
}
