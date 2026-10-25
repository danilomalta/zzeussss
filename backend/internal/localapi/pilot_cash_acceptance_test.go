package localapi

// This acceptance test uses the production HTTP handlers and SQLite migrations.
// Only installation provisioning and test license issuance are fixture setup.
// Each service lifetime is a separate OS process; no in-memory API substitutes.
import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/supplycycle"
)

var pilotReviewDir = flag.String("pilot-cash-review-dir", "", "Explicit private directory for disposable browser review endpoint; never production")
var pilotReviewWait = flag.Duration("pilot-cash-review-wait", 0, "Optional disposable browser review lifetime, maximum 10m")

type pilotProcessConfig struct {
	Database string
	Ready    string
	Device   identity.DeviceContext
	Issuer   ed25519.PublicKey
}

// TestPilotCashProcess is only the child of the acceptance test. Ordinary test
// invocation skips it. EOF on stdin gracefully stops the API and closes SQLite.
func TestPilotCashProcess(t *testing.T) {
	path := os.Getenv("TITAN_PILOT_CHILD")
	if path == "" {
		t.Skip("acceptance-test child only")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c pilotProcessConfig
	if err = json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), c.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": c.Issuer})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, c.Device, v)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	if err = os.WriteFile(c.Ready, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err = app.ShutdownWithTimeout(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

type pilotService struct {
	url     string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	output  bytes.Buffer
	stopped bool
}

func pilotStart(t *testing.T, db string, f *stockHTTPFixture) *pilotService {
	t.Helper()
	dir := t.TempDir()
	c := pilotProcessConfig{db, filepath.Join(dir, "ready"), f.device, f.private.Public().(ed25519.PublicKey)}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "process.json")
	if err = os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := &pilotService{cmd: exec.Command(executable, "-test.run=^TestPilotCashProcess$", "-test.timeout=45m")}
	// Do not inherit commercial DB URLs or deployment keys from the operator.
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT"} {
		if value, ok := os.LookupEnv(name); ok {
			s.cmd.Env = append(s.cmd.Env, name+"="+value)
		}
	}
	s.cmd.Env = append(s.cmd.Env, "TITAN_PILOT_CHILD="+config)
	s.cmd.Stdout = &s.output
	s.cmd.Stderr = &s.output
	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !s.stopped {
			_ = s.cmd.Process.Kill()
			_ = s.cmd.Wait()
		}
	})
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		ready, e := os.ReadFile(c.Ready)
		if e == nil {
			s.url = string(ready)
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("disposable API did not become ready")
	return nil
}
func (s *pilotService) stop(t *testing.T) {
	t.Helper()
	if s.stopped {
		return
	}
	_ = s.stdin.Close()
	err := s.cmd.Wait()
	s.stopped = true
	if err != nil {
		t.Fatalf("API process: %v %s", err, s.output.String())
	}
}

type pilotHTTP struct {
	t       *testing.T
	service *pilotService
	client  *http.Client
	token   string
	steps   int
}

func (p *pilotHTTP) call(method, path string, body any, want int, into any) []byte {
	p.t.Helper()
	var input io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			p.t.Fatal(err)
		}
		input = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, p.service.url+path, input)
	if err != nil {
		p.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	res, err := p.client.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		p.t.Fatal(err)
	}
	if res.StatusCode != want {
		p.t.Fatalf("%s %s: got %d want %d: %s", method, path, res.StatusCode, want, raw)
	}
	if into != nil {
		if err = json.Unmarshal(raw, into); err != nil {
			p.t.Fatal(err)
		}
	}
	p.steps++
	// Never log Authorization, login payloads, tokens or session contents.
	p.t.Logf("HTTP %s %s -> %d", method, path, want)
	return raw
}
func (p *pilotHTTP) login(id string) string {
	p.t.Helper()
	var r struct {
		Token string `json:"token"`
	}
	p.call("POST", "/local/v1/login", map[string]string{"identity_id": id, "password": "senha-forte-de-teste-1"}, 200, &r)
	if r.Token == "" {
		p.t.Fatal("missing session")
	}
	return r.Token
}

func TestPilotCashSaleToSupplierFinanceRestartAndRestore(t *testing.T) {
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Finance}), 200)
	policyPerson(t, f.db, f.owner, "pilot-reviewer", "manager")
	var seq int
	var name, dbPath string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &dbPath); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	p := &pilotHTTP{t: t, service: pilotStart(t, dbPath, f), client: &http.Client{Transport: pilotAcceptanceTransport(t, transport), Timeout: 30 * time.Second}, token: f.token}
	defer func() { p.service.stop(t) }()
	// The initial 5 units and R$10 drawer float are explicit fixture inputs.
	p.call("POST", "/local/v1/stock/operations", stock.Input{OperationID: "pilot-opening-stock", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Saldo inicial descartavel do piloto"}, 201, nil)
	p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "turn-one", "opening_cents": 1000}, 201, nil)
	var cart sale.CartResult
	p.call("POST", cartsPath, cartInput(f), 200, &cart)
	p.call("POST", cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200, nil)
	p.call("POST", cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200, &cart)
	if cart.Cart.State != "completed" || cart.Cart.TotalCents != 3198 || cart.Cart.ReceivedCents != 4000 || cart.Cart.ChangeCents != 802 {
		t.Fatal("incorrect sale/tender/change", cart.Cart)
	}
	// A completed reply is deliberately discarded; restart then recover the
	// original operation without submitting another sale POST.
	if err := p.service.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = p.service.cmd.Wait()
	p.service.stopped = true
	p.service = pilotStart(t, dbPath, f)
	var recovered sale.CartResult
	p.call("GET", "/local/v1/pos/cart-operations/finalize_cash-one", nil, 200, &recovered)
	if !reflect.DeepEqual(cart, recovered) {
		t.Fatal("sale receipt changed across process restart")
	}
	var receipt sale.Receipt
	p.call("GET", "/local/v1/sales/"+cart.Cart.SaleID, nil, 200, &receipt)
	if receipt.TotalCents != 3198 || receipt.Status != "committed" || receipt.FiscalAuthorized || len(receipt.Payments) != 1 || receipt.Payments[0].PaymentID != cart.Cart.PaymentID {
		t.Fatal("sale is not one real cash payment", receipt)
	}
	balance := func(want int64) {
		t.Helper()
		var b struct {
			Quantity int64 `json:"quantity_milli"`
		}
		p.call("GET", "/local/v1/stock/balance?product_id="+f.product+"&location_id="+f.shelf, nil, 200, &b)
		if b.Quantity != want {
			t.Fatalf("stock %d want %d", b.Quantity, want)
		}
	}
	balance(3000)
	p.call("POST", "/local/v1/replenishment/policies", map[string]any{"operation_id": "pilot-policy", "product_id": f.product, "minimum_milli": 4000, "target_milli": 5000}, 200, nil)
	var suggestion struct {
		ID       string `json:"suggestion_id"`
		Quantity int64  `json:"recommended_milli"`
		Observed int64  `json:"observed_milli"`
	}
	p.call("POST", "/local/v1/replenishment/suggestions", restockSuggestInput(f.product, "pilot-suggest"), 200, &suggestion)
	if suggestion.Quantity != 2000 || suggestion.Observed != 3000 {
		t.Fatal("restock not derived from persisted sale", suggestion)
	}
	// Approval is a separate, explicit human operation; it is never automatic.
	p.call("POST", "/local/v1/replenishment/reviews", restockReviewInput(suggestion.ID, "pilot-approve", "approved"), 200, nil)
	p.call("POST", "/local/v1/purchase-suppliers", map[string]any{"operation_id": "supplier-op", "id": "supplier", "name": "Fornecedor externo do piloto", "status": "active"}, 201, nil)
	p.call("POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(suggestion.ID)), 201, nil)
	decision := cycleDecisionInput()
	decision.AcceptedMilli = 2000
	decision.Reason = "Confirmacao externa registrada manualmente, sem envio automatico"
	p.call("POST", cycleOrder+"/supplier-decisions", decision, 200, nil)
	p.call("POST", receivingAuthPath, receivingAuthBody(), 200, nil)
	p.call("POST", cycleOrder+"/deliveries", cyclePlanInput("trip-one", 2000), 200, nil)
	for i, action := range []string{"load", "arrive"} {
		p.call("POST", cycleDelivery+"/actions", supplycycle.ActionInput{OperationID: "pilot-" + action, ExpectedRevision: int64(i + 1), Action: action, EvidenceReference: "registro-manual-" + action, ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Conferido pelo responsavel"}, 200, nil)
	}
	receive := cycleReceiveInput()
	receive.LocationID = f.shelf
	receive.DeliveredMilli = 2000
	receive.AcceptedMilli = 2000
	receive.Reason = "Entrega integral conferida"
	var received supplycycle.Result
	p.call("POST", cycleDelivery+"/receive", receive, 200, &received)
	if received.Delivery.State != "received" || received.Receipt.AcceptedMilli != 2000 {
		t.Fatal("receipt missing", received)
	}
	balance(5000)
	finance := cycleFinanceInput()
	finance.TotalCents = 2050
	finance.DocumentReference = "RECIBO-PILOTO-1"
	finance.Description = "Duas unidades recebidas a R$10,25"
	finance.Installments = []payables.Installment{{ID: "part-one", DueOn: "2026-10-25", AmountCents: 2050}}
	var obligation supplycycle.Result
	p.call("POST", cycleDelivery+"/payable", finance, 200, &obligation)
	if obligation.Payable.State != "pending" || obligation.Origin.ReceiptID != received.Receipt.ID || obligation.Origin.FiscalAuthorizationVerified {
		t.Fatal("unlinked or falsely fiscal payable", obligation)
	}
	p.call("POST", payablesPath+"/invoice-one/approve", json.RawMessage(payableApprove), 403, nil)
	ownerToken := p.token
	p.token = p.login("pilot-reviewer")
	p.call("POST", payablesPath+"/invoice-one/approve", json.RawMessage(payableApprove), 200, nil)
	p.token = ownerToken
	p.call("POST", "/local/v1/cash/movements", map[string]any{"session_id": "turn-one", "operation_id": "withdraw-one", "kind": "withdrawal", "amount_cents": 2050, "reason": "Pagamento manual em especie ao fornecedor"}, 200, nil)
	p.call("POST", payablesPath+"/invoice-one/cash-payments", json.RawMessage(pcPay), 201, nil)
	p.call("POST", "/local/v1/cash/close", map[string]any{"session_id": "turn-one", "operation_id": "close-one", "declared_cents": 2148}, 200, nil)
	var report register.ClosedReport
	p.call("GET", closedReportPath, nil, 200, &report)
	if report.Cash.ExpectedCents != 2148 || report.Cash.DifferenceCents != 0 || report.Cash.WithdrawalCents != 2050 || report.NetSalesCents != 3198 || report.SaleCount != 1 {
		t.Fatal("drawer double-counted tender/change/payment", report)
	}
	var payable payables.Payable
	p.call("GET", payablesPath+"/invoice-one", nil, 200, &payable)
	if payable.Cash == nil || payable.Cash.AppliedCents != 2050 || payable.Cash.OutstandingCents != 0 {
		t.Fatal("supplier still outstanding", payable.Cash)
	}

	// Verify the complete business snapshot via HTTP, not cached client objects.
	paths := []string{"/local/v1/replenishment/operations/policy/pilot-policy", "/local/v1/replenishment/operations/suggest/pilot-suggest", "/local/v1/replenishment/operations/review/pilot-approve", "/local/v1/replenishment/suggestions", "/local/v1/purchase-suppliers?offset=0", cartsPath + "/stable-cart", "/local/v1/sales/" + cart.Cart.SaleID, closedReportPath, cycleOrder + "/trace", cycleOrder + "/supply-cycle?limit=50&offset=0", cycleDelivery, payablesPath + "/invoice-one", "/local/v1/finance/receiving-origins/invoice-one", "/local/v1/pos/cart-operations/finalize_cash-one", "/local/v1/purchase-cycle-operations/receive-trip", pcPath + "/operations/pay-one"}
	snapshot := func() map[string]any {
		t.Helper()
		out := map[string]any{}
		for _, path := range paths {
			var v any
			p.call("GET", path, nil, 200, &v)
			out[path] = v
		}
		return out
	}
	want := snapshot()
	p.service.stop(t)
	p.service = pilotStart(t, dbPath, f)
	if got := snapshot(); !reflect.DeepEqual(want, got) {
		t.Fatal("complete workflow changed after service restart")
	}
	balance(5000)

	// Take a consistent encrypted live SQLite snapshot while the service is up.
	ctx := context.Background()
	db, err := localdb.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	archiveDir := t.TempDir()
	archive := filepath.Join(archiveDir, "pilot.tytbak")
	restored := filepath.Join(archiveDir, "restored.sqlite")
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if err = backup.Create(ctx, db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err = backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	wrong := append([]byte{}, key...)
	wrong[0] ^= 1
	if err = backup.Verify(ctx, archive, f.device, wrong); err == nil {
		t.Fatal("wrong backup key accepted")
	}
	if err = backup.Restore(ctx, archive, restored, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err = backup.Restore(ctx, archive, restored, f.device, key); err == nil {
		t.Fatal("restore overwrote existing database")
	}
	p.service.stop(t)
	p.service = pilotStart(t, restored, f)
	// Restore deliberately revokes saved sessions; authenticate again, never
	// weaken revocation to make recovery pass.
	p.call("GET", "/local/v1/me", nil, 401, nil)
	p.token = p.login(f.owner.OwnerID)
	if got := snapshot(); !reflect.DeepEqual(want, got) {
		t.Fatal("restored workflow differs from original")
	}
	balance(5000)
	// Receipt queries above are enough for recovery. Explicit same-ID repeats
	// below prove a user retry cannot duplicate stock, sale, withdrawal or link.
	p.call("POST", cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200, nil)
	p.call("POST", cycleDelivery+"/receive", receive, 200, nil)
	p.call("POST", payablesPath+"/invoice-one/cash-payments", json.RawMessage(pcPay), 200, nil)
	if got := snapshot(); !reflect.DeepEqual(want, got) {
		t.Fatal("idempotent retry changed workflow")
	}
	balance(5000)
	restoredDB, err := localdb.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	for _, check := range []struct {
		query string
		want  int64
	}{
		{`SELECT COUNT(*) FROM sales`, 1}, {`SELECT COUNT(*) FROM sale_payments WHERE method='cash'`, 1},
		{`SELECT COUNT(*) FROM cash_movements`, 2}, {`SELECT SUM(amount_cents) FROM cash_movements WHERE amount_cents>0`, 3198},
		{`SELECT COUNT(*) FROM purchase_receipts`, 1}, {`SELECT COUNT(*) FROM supplier_payables`, 1},
		{`SELECT COUNT(*) FROM payable_cash_payments`, 1}, {`SELECT SUM(quantity_milli) FROM stock_movements`, 5000},
	} {
		var got int64
		if err = restoredDB.QueryRow(check.query).Scan(&got); err != nil || got != check.want {
			t.Fatalf("%s: got %d want %d err %v", check.query, got, check.want, err)
		}
	}
	if *pilotReviewDir != "" {
		if *pilotReviewWait <= 0 || *pilotReviewWait > 10*time.Minute {
			t.Fatal("review requires explicit duration in (0,10m]")
		}
		info, e := os.Lstat(*pilotReviewDir)
		if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("review directory must be existing private 0700 directory")
		}
		raw, e := json.Marshal(map[string]any{"test_data_only": true, "url": p.service.url, "identity_id": f.owner.OwnerID, "password": "senha-forte-de-teste-1", "sale_id": cart.Cart.SaleID, "product_id": f.product, "location_id": f.shelf, "expires_at": time.Now().Add(*pilotReviewWait).UTC().Format(time.RFC3339)})
		if e != nil {
			t.Fatal(e)
		}
		file, e := os.OpenFile(filepath.Join(*pilotReviewDir, "review.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = file.Write(raw); e != nil {
			file.Close()
			t.Fatal(e)
		}
		if e = file.Close(); e != nil {
			t.Fatal(e)
		}
		t.Log("Disposable review ready; production handlers, restored fixture DB, no commercial data")
		time.Sleep(*pilotReviewWait)
	}
	t.Logf("PASS: %d real loopback HTTP requests; 4 service processes; original stock 5000 - sale 2000 + receipt 2000 = 5000 milli; drawer 1000 + sale 3198 - supplier 2050 = 2148 cents; payable outstanding 0; encrypted backup verified/restored; original IDs unchanged", p.steps)
}
