package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/supplycycle"
)

const returnsPath = "/local/v1/purchase-supplier-returns"

func returnCall(t *testing.T, f *httpContractFixture, path string, in any, want int) purchases.ReturnResult {
	t.Helper()
	b := agendaCall(t, f, "POST", path, in, want)
	var r purchases.ReturnResult
	if want == 200 {
		domainSchemaFile(t, "supplier-returns.openapi.json", "ReturnResult", b)
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e, string(b))
		}
	}
	return r
}
func returnFixture(t *testing.T) (*httpContractFixture, purchases.ReturnOpenInput) {
	t.Helper()
	f, _ := cycleFixture(t)
	receipt := cycleReceived(t, f).Receipt
	caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	in := purchases.ReturnOpenInput{OperationID: "return-open", ReturnID: "return-one", ReceiptID: receipt.ID, QuantityMilli: 1000, Reason: "Mercadoria avariada conferida"}
	returnCall(t, f, casesPath+"/case/returns", in, 200)
	return f, in
}
func returnAction(action string, rev int64) purchases.ReturnActionInput {
	return purchases.ReturnActionInput{OperationID: "return-" + action, ExpectedRevision: rev, Action: action, Contact: "Fornecedor por telefone", EvidenceReference: "protocolo-123", ObservedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), Reason: "Conferido manualmente"}
}
func returnReady(t *testing.T, f *httpContractFixture) {
	t.Helper()
	a := returnAction("approve", 1)
	a.QuantityMilli = 1000
	a.FinancialInstruction = "review"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	a = returnAction("schedule", 2)
	a.CollectionMode = "relationship_end"
	a.RouteReference = "rota-123"
	a.Deadline = "2026-12-01T00:00:00Z"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
}
func returnStockInput(id string, rev, qty int64) purchases.ReturnStockInput {
	return purchases.ReturnStockInput{OperationID: id, ExpectedRevision: rev, LocationID: "receiving-room", QuantityMilli: qty, HandoverReference: "recibo-assinado", ObservedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), Reason: "Movimentacao conferida"}
}
func returnCounts(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	v := cycleCounts(t, f)
	for _, table := range []string{"purchase_supplier_returns", "purchase_supplier_return_operations", "purchase_supplier_return_events"} {
		var n int64
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		v = append(v, n)
	}
	return v
}
func TestSupplierReturnPartialReceiptRecoveryReplayAndVoidGuard(t *testing.T) {
	f, in := returnFixture(t)
	returnReady(t, f)
	dispatch := returnStockInput("dispatch-one", 3, 1000)
	r := returnCall(t, f, returnsPath+"/return-one/dispatch", dispatch, 200)
	if r.Return.State != "dispatched" || r.StockOperationID == "" {
		t.Fatal(r)
	}
	before := returnCounts(t, f)
	again := returnCall(t, f, returnsPath+"/return-one/dispatch", dispatch, 200)
	if !again.Repeated || !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("duplicated stock")
	}
	agendaCall(t, f, "POST", receiptVoidPath(in.ReceiptID), receiptVoidBody(), 409)
	a := returnAction("supplier_receive", 4)
	a.QuantityMilli = 600
	r = returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	if r.Return.SupplierReceivedMilli != 600 {
		t.Fatal(r)
	}
	r = returnCall(t, f, returnsPath+"/return-one/recover", returnStockInput("recover-one", 5, 400), 200)
	if r.Return.State != "completed" || r.Return.RecoveredMilli != 400 {
		t.Fatal(r)
	}
	agendaCall(t, f, "POST", receiptVoidPath(in.ReceiptID), receiptVoidBody(), 409)
	agendaCall(t, f, "GET", returnsPath+"/return-one", nil, 200)
	agendaCall(t, f, "GET", returnsPath+"/return-one/history?limit=2&offset=4", nil, 200)
	agendaCall(t, f, "GET", "/local/v1/purchase-supplier-return-operations/dispatch-one", nil, 200)
}
func TestSupplierReturnFullRecoveryPermitsOriginalVoid(t *testing.T) {
	f, in := returnFixture(t)
	returnReady(t, f)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-one", 3, 1000), 200)
	returnCall(t, f, returnsPath+"/return-one/recover", returnStockInput("recover-one", 4, 1000), 200)
	agendaCall(t, f, "POST", receiptVoidPath(in.ReceiptID), receiptVoidBody(), 200)
}
func TestSupplierReturnRollbackAtFinalEvent(t *testing.T) {
	f, _ := returnFixture(t)
	returnReady(t, f)
	before := returnCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER return_fault BEFORE INSERT ON purchase_supplier_return_events WHEN NEW.operation_id='dispatch-one' BEGIN SELECT RAISE(ABORT,'injected'); END`); e != nil {
		t.Fatal(e)
	}
	in := returnStockInput("dispatch-one", 3, 1000)
	returnCall(t, f, returnsPath+"/return-one/dispatch", in, 500)
	if !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("partial stock or outbox")
	}
	if _, e := f.db.Exec(`DROP TRIGGER return_fault`); e != nil {
		t.Fatal(e)
	}
	returnCall(t, f, returnsPath+"/return-one/dispatch", in, 200)
}
func TestSupplierReturnQuotaStaleRevisionAndPermissions(t *testing.T) {
	f, in := returnFixture(t)
	in.OperationID = "second-open"
	in.ReturnID = "second"
	in.QuantityMilli = 1501
	returnCall(t, f, casesPath+"/case/returns", in, 409)
	a := returnAction("approve", 1)
	a.QuantityMilli = 1000
	a.FinancialInstruction = "review"
	worker := caseWorker(t, f, "return-worker")
	raw, _ := json.Marshal(a)
	status, b := request(t, f.app, "POST", returnsPath+"/return-one/actions", string(raw), worker)
	if status != 403 {
		t.Fatal(status, string(b))
	}
	returnReady(t, f)
	a.OperationID = "stale-approve"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 409)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("too-small", 3, 999), 409)
	agendaCall(t, f, "GET", returnsPath+"?state=wrong", nil, 400)
	agendaCall(t, f, "GET", returnsPath+"?supplier_id=", nil, 400)
}
func TestSupplierReturnFinanceReviewDoesNotCreateCredit(t *testing.T) {
	f, _ := returnFixture(t)
	cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	returnReady(t, f)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-one", 3, 1000), 200)
	before := returnCounts(t, f)
	b := agendaCall(t, f, "GET", "/local/v1/finance/receiving-return-review/invoice-one", nil, 200)
	var r payables.ReceivingReturnReview
	if e := json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	if r.TotalCents != 31500 || r.FinancialEffectRecorded || r.ExternalVerified || !r.ReviewRequired || !r.SourceComplete || len(r.Items) != 1 {
		t.Fatal(string(b))
	}
	if !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("read changed business state")
	}
}

func TestSupplierReturnPairedTripMustArriveAndCannotReuseOriginalReceipt(t *testing.T) {
	f, _ := returnFixture(t)
	a := returnAction("approve", 1)
	a.QuantityMilli = 1000
	a.FinancialInstruction = "replacement_expected"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	a = returnAction("schedule", 2)
	a.CollectionMode = "next_delivery"
	a.PairedDeliveryID = "trip-one"
	a.Deadline = "2026-12-01T00:00:00Z"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 409)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-two", 1000), 200)
	a.PairedDeliveryID = "trip-two"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	in := returnStockInput("dispatch-one", 3, 1000)
	returnCall(t, f, returnsPath+"/return-one/dispatch", in, 409)
	for i, action := range []string{"load", "arrive"} {
		cycleCall(t, f, "POST", "/local/v1/purchase-deliveries/trip-two/actions", supplycycle.ActionInput{OperationID: "second-" + action, ExpectedRevision: int64(i + 1), Action: action, EvidenceReference: "manual", ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Conferido pelo responsavel"}, 200)
	}
	returnCall(t, f, returnsPath+"/return-one/dispatch", in, 200)
}
func TestSupplierReturnConcurrentDispatchAndImmutableHistory(t *testing.T) {
	f, _ := returnFixture(t)
	returnReady(t, f)
	in := returnStockInput("dispatch-one", 3, 1000)
	b, _ := json.Marshal(in)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", returnsPath+"/return-one/dispatch", string(b), f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal(code)
		}
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM purchase_supplier_return_operations WHERE action='dispatch'`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	for _, query := range []string{`UPDATE purchase_supplier_return_events SET event_type='forged'`, `DELETE FROM purchase_supplier_return_operations`, `DELETE FROM purchase_supplier_returns`} {
		if _, e := f.db.Exec(query); e == nil {
			t.Fatal("history mutable", query)
		}
	}
}
func TestSupplierReturnStrictBodiesAndOriginalActorRecovery(t *testing.T) {
	f, _ := returnFixture(t)
	a := returnAction("approve", 1)
	a.QuantityMilli = 1000
	a.FinancialInstruction = "review"
	b, _ := json.Marshal(a)
	for _, raw := range []string{strings.Replace(string(b), `"quantity_milli":1000`, `"quantity_milli":null`, 1), strings.Replace(string(b), `"action":"approve"`, `"action":"approve","action":"cancel"`, 1), strings.TrimSuffix(string(b), "}") + `,"tenant_id":"other"}`} {
		code, _ := request(t, f.app, "POST", returnsPath+"/return-one/actions", raw, f.token)
		if code != 400 {
			t.Fatal(code, raw)
		}
	}
	worker := caseWorker(t, f, "return-worker")
	code, _ := request(t, f.app, "GET", "/local/v1/purchase-supplier-return-operations/return-open", "", worker)
	if code != 404 {
		t.Fatal(code)
	}
	code, _ = request(t, f.app, "GET", "/local/v1/finance/receiving-return-review/invoice-one", "", worker)
	if code != 403 {
		t.Fatal(code)
	}
}
func TestSupplierReturnBackupRestoreAndReopenPreserveStockReceipt(t *testing.T) {
	f, _ := returnFixture(t)
	returnReady(t, f)
	first := returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-one", 3, 1000), 200)
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "returns.tytbak")
	path := filepath.Join(dir, "restored.sqlite")
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	for _, restore := range []bool{false, true} {
		var original string
		if restore {
			original = path
		} else {
			var n int
			var name string
			if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &original); e != nil {
				t.Fatal(e)
			}
		}
		db, e := localdb.Open(context.Background(), original)
		if e != nil {
			t.Fatal(e)
		}
		r, e := purchases.SupplierReturnOperation(context.Background(), db, agendaActor(f), f.device, "dispatch-one")
		if e != nil || !reflect.DeepEqual(first, r) {
			t.Fatal(r, e)
		}
		db.Close()
	}
}

type supplierReturnDropTransport struct {
	base      *http.Transport
	discarded atomic.Int32
}

func (d *supplierReturnDropTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r, e := d.base.RoundTrip(req)
	if e == nil && req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/dispatch") && r.StatusCode == 200 {
		d.discarded.Add(1)
		r.Body.Close()
		return nil, errors.New("test-only lost committed response")
	}
	return r, e
}
func TestSupplierReturnActualLoopbackLostReplyRecoverWithoutSecondPOST(t *testing.T) {
	f, _ := returnFixture(t)
	returnReady(t, f)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.ShutdownWithTimeout(2 * time.Second)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	defer tr.CloseIdleConnections()
	drop := &supplierReturnDropTransport{base: tr}
	client := &http.Client{Transport: drop, Timeout: 5 * time.Second}
	origin := "http://" + listener.Addr().String()
	b, _ := json.Marshal(returnStockInput("dispatch-one", 3, 1000))
	req, e := http.NewRequest("POST", origin+returnsPath+"/return-one/dispatch", strings.NewReader(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	if r, e := client.Do(req); e == nil {
		r.Body.Close()
		t.Fatal("expected lost reply")
	}
	if drop.discarded.Load() != 1 {
		t.Fatal("POST did not commit")
	}
	before := returnCounts(t, f)
	req, e = http.NewRequest("GET", origin+"/local/v1/purchase-supplier-return-operations/dispatch-one", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	r, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var out purchases.ReturnResult
	if json.NewDecoder(r.Body).Decode(&out) != nil || r.StatusCode != 200 || out.Return.DispatchedMilli != 1000 || !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal(out)
	}
}

func TestSupplierReturnInsufficientStockDoesNotPartiallyDispatch(t *testing.T) {
	f, _ := returnFixture(t)
	returnReady(t, f)
	var head purchases.SupplierReturn
	b := agendaCall(t, f, "GET", returnsPath+"/return-one", nil, 200)
	if e := json.Unmarshal(b, &head); e != nil {
		t.Fatal(e)
	}
	balance, e := stock.Balance(context.Background(), f.db, agendaActor(f), f.device, head.ProductID, "receiving-room")
	if e != nil {
		t.Fatal(e)
	}
	_, e = stock.Record(context.Background(), f.db, agendaActor(f), f.device, stock.Input{OperationID: "other-loss", Kind: "loss", ProductID: head.ProductID, FromLocationID: "receiving-room", QuantityMilli: balance - 999, Reason: "Concorrencia legitima"})
	if e != nil {
		t.Fatal(e)
	}
	before := returnCounts(t, f)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-one", 3, 1000), 409)
	if !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("partial dispatch")
	}
}
func TestSupplierReturnConcurrentOriginalVoidCannotDoubleDebit(t *testing.T) {
	f, in := returnFixture(t)
	returnReady(t, f)
	dispatch, _ := json.Marshal(returnStockInput("dispatch-one", 3, 1000))
	void, _ := json.Marshal(receiptVoidBody())
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, v := range []struct{ path, body string }{{returnsPath + "/return-one/dispatch", string(dispatch)}, {receiptVoidPath(in.ReceiptID), string(void)}} {
		wg.Add(1)
		go func(path, body string) {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", path, body, f.token)
			codes <- code
		}(v.path, v.body)
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			ok++
		} else if code == 409 {
			conflict++
		} else {
			t.Fatal(code)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
}

func TestSupplierReturnPartialAuthorizationDeclineCancelAndQuotaRelease(t *testing.T) {
	f, in := returnFixture(t)
	a := returnAction("approve", 1)
	a.QuantityMilli = 600
	a.FinancialInstruction = "credit_requested"
	r := returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	if r.Return.AuthorizedMilli != 600 {
		t.Fatal(r)
	}
	in.OperationID = "open-two"
	in.ReturnID = "return-two"
	in.QuantityMilli = 1900
	returnCall(t, f, casesPath+"/case/returns", in, 200)
	a = returnAction("decline", 1)
	a.OperationID = "decline-two"
	returnCall(t, f, returnsPath+"/return-two/actions", a, 200)
	a = returnAction("cancel", 2)
	a.OperationID = "cancel-one"
	returnCall(t, f, returnsPath+"/return-one/actions", a, 200)
	in.OperationID = "open-three"
	in.ReturnID = "return-three"
	in.QuantityMilli = 2500
	returnCall(t, f, casesPath+"/case/returns", in, 200)
	before := returnCounts(t, f)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-cancelled", 3, 600), 409)
	if !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("cancelled return moved stock")
	}
}
