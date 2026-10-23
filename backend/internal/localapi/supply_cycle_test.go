package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/supplycycle"
)

const cycleOrder = "/local/v1/purchase-orders/order"
const cycleDelivery = "/local/v1/purchase-deliveries/trip-one"

func cycleFixture(t *testing.T) (*httpContractFixture, string) {
	t.Helper()
	f := overviewFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 99, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Logistics, modules.Finance, modules.POS}), 200)
	policyPerson(t, f.db, f.owner, "cycle-reviewer", "manager")
	return f, loginToken(t, f.app, "cycle-reviewer")
}
func cycleDecisionInput() supplycycle.DecisionInput {
	return supplycycle.DecisionInput{OperationID: "decision-one", Decision: "confirmed", AcceptedMilli: 5000, PromisedAt: "2026-11-01T12:00:00Z", Contact: "Fornecedor por telefone", EvidenceReference: "protocolo-123", ObservedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), Reason: "Atendimento parcial permitido pelo acordo"}
}
func cyclePlanInput(id string, qty int64) supplycycle.PlanInput {
	return supplycycle.PlanInput{OperationID: "plan-" + id, DeliveryID: id, DecisionRevision: 1, DeliveryReference: "external-" + id, PlannedMilli: qty, Reason: "Entrega aprovada pelo comercio"}
}
func cycleFinanceInput() supplycycle.FinanceInput {
	return supplycycle.FinanceInput{OperationID: "finance-one", ExpectedRevision: 4, PayableID: "invoice-one", DocumentReference: "NFe-123", SourceReference: "Documento conferido manualmente", Description: "Compra recebida", IssuedOn: "2026-10-01", Currency: "BRL", TotalCents: 31500, Installments: []payables.Installment{{ID: "part-one", DueOn: "2026-10-10", AmountCents: 10000}, {ID: "part-two", DueOn: "2026-10-20", AmountCents: 9600}, {ID: "part-three", DueOn: "2026-11-20", AmountCents: 11900}}}
}
func cycleCall(t *testing.T, f *httpContractFixture, method, path string, in any, want int) supplycycle.Result {
	t.Helper()
	b := agendaCall(t, f, method, path, in, want)
	var r supplycycle.Result
	if want == 200 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e, string(b))
		}
		domainSchemaFile(t, "purchase-supply-cycle.openapi.json", "Result", b)
		for name, value := range map[string]any{"Decision": r.Decision, "Delivery": r.Delivery, "Receipt": r.Receipt, "Payable": r.Payable, "Origin": r.Origin} {
			raw, _ := json.Marshal(value)
			if string(raw) != "null" {
				domainSchemaFile(t, "purchase-supply-cycle.openapi.json", name, raw)
			}
		}
	}
	return r
}
func cycleAtArrival(t *testing.T, f *httpContractFixture) {
	t.Helper()
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", cycleDecisionInput(), 200)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-one", 3000), 200)
	for i, action := range []string{"load", "arrive"} {
		cycleCall(t, f, "POST", cycleDelivery+"/actions", supplycycle.ActionInput{OperationID: "act-" + action, ExpectedRevision: int64(i + 1), Action: action, EvidenceReference: "conferencia-manual-" + action, ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Conferido pelo responsavel"}, 200)
	}
}
func cycleReceiveInput() supplycycle.ReceiveInput {
	return supplycycle.ReceiveInput{OperationID: "receive-trip", ExpectedRevision: 3, LocationID: "receiving-room", DeliveredMilli: 3000, AcceptedMilli: 2500, Reason: "500 recusados na conferencia"}
}
func cycleReceived(t *testing.T, f *httpContractFixture) supplycycle.Result {
	t.Helper()
	cycleAtArrival(t, f)
	return cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 200)
}
func cycleCounts(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	out := []int64{}
	for _, table := range []string{"purchase_supplier_decisions", "purchase_deliveries", "purchase_cycle_operations", "purchase_cycle_events", "payable_receiving_origins", "supplier_payables", "supplier_payable_operations", "supplier_payable_events", "purchase_receipts", "stock_operations", "stock_movements", "cash_movements", "outbox"} {
		var n int64
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	return out
}
func TestSupplyCycleVerticalReceivingFinancialApprovalPartialPaymentAndReturn(t *testing.T) {
	f, reviewer := cycleFixture(t)
	first := cycleReceived(t, f)
	if first.Receipt.AcceptedMilli != 2500 || first.Delivery.State != "received" || first.Delivery.Revision != 4 {
		t.Fatal(first)
	}
	before := cycleCounts(t, f)
	original := cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/receive-trip", nil, 200)
	if !reflect.DeepEqual(first, original) || !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("recovery changed effect")
	}
	retry := cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 200)
	retry.Repeated = false
	if !reflect.DeepEqual(first, retry) || !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("duplicate stock")
	}
	// Rejected quantity does not consume a second delivery's commitment.
	next := cyclePlanInput("trip-two", 2500)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", next, 200)
	next = cyclePlanInput("trip-too-many", 1)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", next, 409)
	finance := cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	if finance.Payable.State != "pending" || finance.Payable.TotalCents != 31500 || finance.Origin.FiscalAuthorizationVerified {
		t.Fatal(finance)
	}
	before = cycleCounts(t, f)
	r := cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	if !r.Repeated || !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("duplicate obligation")
	}
	// Existing independent review and audited cash services close the flow.
	payableResult(t, f, f.token, "POST", payablesPath+"/invoice-one/approve", payableApprove, 403)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":50000}`, 201)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	p := payableRead(t, f, f.token)
	if p.Cash.AppliedCents != 6000 || p.Cash.OutstandingCents != 25500 {
		t.Fatal(p.Cash)
	}
	pcMove(t, f, f.token, "return-supply", "supply", 6000)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	p = payableRead(t, f, f.token)
	if p.Cash.AppliedCents != 0 || p.Cash.OutstandingCents != 31500 {
		t.Fatal(p.Cash)
	}
	// Physical/history responses contain no amounts from the finance operation.
	b := agendaCall(t, f, "GET", cycleDelivery+"/history?limit=2&offset=2", nil, 200)
	var history supplycycle.History
	if json.Unmarshal(b, &history) != nil || history.Total != 4 || len(history.Items) != 2 || strings.Contains(string(b), "31500") {
		t.Fatal(string(b))
	}
	var cycle supplycycle.Cycle
	b = agendaCall(t, f, "GET", cycleOrder+"/supply-cycle?offset=99&limit=1", nil, 200)
	if json.Unmarshal(b, &cycle) != nil || cycle.Total != 2 || len(cycle.Deliveries) != 0 || cycle.EffectiveAcceptedMilli != 2500 || cycle.AllocatedMilli != 5000 {
		t.Fatal(string(b))
	}
}
func TestSupplyCycleReceiveAndFinanceRollbackAtFinalEvent(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	before := cycleCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER cycle_fault BEFORE INSERT ON purchase_cycle_events WHEN NEW.operation_id='receive-trip' BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); e != nil {
		t.Fatal(e)
	}
	cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 500)
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("partial stock after failure")
	}
	if _, e := f.db.Exec(`DROP TRIGGER cycle_fault`); e != nil {
		t.Fatal(e)
	}
	cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 200)
	before = cycleCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER cycle_fault BEFORE INSERT ON purchase_cycle_events WHEN NEW.operation_id='finance-one' BEGIN SELECT RAISE(ABORT,'injected finance failure'); END`); e != nil {
		t.Fatal(e)
	}
	cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 500)
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("orphaned payable or origin")
	}
	if _, e := f.db.Exec(`DROP TRIGGER cycle_fault`); e != nil {
		t.Fatal(e)
	}
	cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
}
func TestSupplyCycleVoidBlocksNewApprovalAndPaymentPreservingReceipts(t *testing.T) {
	for _, approve := range []bool{false, true} {
		t.Run(fmt.Sprint(approve), func(t *testing.T) {
			f, reviewer := cycleFixture(t)
			received := cycleReceived(t, f)
			first := cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
			if approve {
				payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
			}
			agendaCall(t, f, "POST", receiptVoidPath(received.Receipt.ID), receiptVoidBody(), 200)
			var origin supplycycle.Origin
			b := agendaCall(t, f, "GET", "/local/v1/finance/receiving-origins/invoice-one", nil, 200)
			if json.Unmarshal(b, &origin) != nil || !origin.ReviewRequired {
				t.Fatal(string(b))
			}
			original := cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/finance-one", nil, 200)
			if !reflect.DeepEqual(first, original) {
				t.Fatal("original changed after void")
			}
			if approve {
				pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 409)
			} else {
				payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 409)
			}
		})
	}
}
func TestSupplyCycleCompetingPlansAndReceiptsSerialize(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", cycleDecisionInput(), 200)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, id := range []string{"trip-one", "trip-two"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			raw, _ := json.Marshal(cyclePlanInput(id, 3000))
			code, _ := request(t, f.app, "POST", cycleOrder+"/deliveries", string(raw), f.token)
			codes <- code
		}(id)
	}
	wg.Wait()
	close(codes)
	seen := map[int]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[200] != 1 || seen[409] != 1 {
		t.Fatal(seen)
	}
}
func TestSupplyCycleCompetingReceiveAndFinanceNeverDuplicate(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	for _, kind := range []string{"receive", "payable"} {
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var in any
				if kind == "receive" {
					v := cycleReceiveInput()
					v.OperationID = fmt.Sprintf("receive-race-%d", i)
					in = v
				} else {
					v := cycleFinanceInput()
					v.OperationID = fmt.Sprintf("finance-race-%d", i)
					v.PayableID = fmt.Sprintf("invoice-%d", i)
					v.DocumentReference = fmt.Sprintf("doc-%d", i)
					in = v
				}
				raw, _ := json.Marshal(in)
				code, _ := request(t, f.app, "POST", cycleDelivery+"/"+kind, string(raw), f.token)
				codes <- code
			}(i)
		}
		wg.Wait()
		close(codes)
		seen := map[int]int{}
		for code := range codes {
			seen[code]++
		}
		if seen[200] != 1 || seen[409] != 1 {
			t.Fatal(kind, seen)
		}
	}
	materialCount(t, f, "purchase_receipts", 1)
	materialCount(t, f, "supplier_payables", 1)
	materialCount(t, f, "payable_receiving_origins", 1)
}
func TestSupplyCycleStrictInputStateAndNoImplicitAuthorization(t *testing.T) {
	f, _ := cycleFixture(t)
	in := cycleDecisionInput()
	raw, _ := json.Marshal(in)
	before := cycleCounts(t, f)
	for _, body := range []string{strings.Replace(string(raw), `"accepted_milli":5000`, `"accepted_milli":5000.0`, 1), strings.Replace(string(raw), `"reason":`, `"tenant_id":"foreign","reason":`, 1), strings.Replace(string(raw), `"operation_id":`, `"operation_id":"duplicate","operation_id":`, 1)} {
		code, _ := request(t, f.app, "POST", cycleOrder+"/supplier-decisions", body, f.token)
		if code != 400 {
			t.Fatal(code, body)
		}
	}
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("invalid input wrote")
	}
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", in, 200)
	changed := in
	changed.Reason = "Changed reason"
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", changed, 409)
	v := cyclePlanInput("trip-one", 3000)
	v.AppointmentID = "missing"
	v.WindowEvidenceReference = "manual"
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", v, 409)
	v = cyclePlanInput("trip-one", 3000)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", v, 200)
	cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 409)
	changed = cycleDecisionInput()
	changed.OperationID = "decision-stale"
	changed.ExpectedRevision = 0
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", changed, 409)
	changed.ExpectedRevision = 1
	changed.AcceptedMilli = 2000
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", changed, 409)
	for _, q := range []string{"?limit=0", "?limit=1&limit=2", "?extra=1"} {
		agendaCall(t, f, "GET", cycleOrder+"/supply-cycle"+q, nil, 400)
	}
}
func TestSupplyCycleFinancialReadRestrictionAndOriginalActor(t *testing.T) {
	f, reviewer := cycleFixture(t)
	cycleReceived(t, f)
	cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	stockToken := caseWorker(t, f, "cycle-stock")
	for _, path := range []string{cycleDelivery, cycleDelivery + "/history", cycleOrder + "/supply-cycle"} {
		code, b := request(t, f.app, "GET", path, "", stockToken)
		if code != 200 || strings.Contains(string(b), "31500") {
			t.Fatal(code, string(b))
		}
	}
	code, _ := request(t, f.app, "GET", "/local/v1/finance/receiving-origins/invoice-one", "", stockToken)
	if code != 403 {
		t.Fatal(code)
	}
	code, _ = request(t, f.app, "GET", "/local/v1/purchase-cycle-operations/finance-one", "", reviewer)
	if code != 404 {
		t.Fatal(code)
	}
	raw, _ := json.Marshal(cycleFinanceInput())
	code, _ = request(t, f.app, "POST", cycleDelivery+"/payable", string(raw), stockToken)
	if code != 403 {
		t.Fatal(code)
	}
}
func TestSupplyCycleBackupRestoreAndRestartRecoverOriginal(t *testing.T) {
	f, _ := cycleFixture(t)
	first := cycleReceived(t, f)
	finance := cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "cycle.tytbak")
	path := filepath.Join(dir, "restored.sqlite")
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r, e := supplycycle.Operation(context.Background(), db, agendaActor(f), f.device, "receive-trip")
	if e != nil || !reflect.DeepEqual(r, first) {
		t.Fatal(r, e)
	}
	o, e := supplycycle.GetOrigin(context.Background(), db, agendaActor(f), f.device, "invoice-one")
	if e != nil || !reflect.DeepEqual(o, *finance.Origin) {
		t.Fatal(o, e)
	}
	var n int
	var name, original string
	if e = f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &original); e != nil {
		t.Fatal(e)
	}
	if e = f.db.Close(); e != nil {
		t.Fatal(e)
	}
	f.db, e = localdb.Open(context.Background(), original)
	if e != nil {
		t.Fatal(e)
	}
	defer f.db.Close()
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(f.db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	r = cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/receive-trip", nil, 200)
	if !reflect.DeepEqual(r, first) {
		t.Fatal(r)
	}
}

type cycleDropTransport struct {
	base      *http.Transport
	discarded atomic.Int32
}

func (d *cycleDropTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r, e := d.base.RoundTrip(req)
	if e == nil && req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/receive") && r.StatusCode == 200 {
		d.discarded.Add(1)
		r.Body.Close()
		return nil, errors.New("test-only dropped committed response")
	}
	return r, e
}
func TestSupplyCycleActualLoopbackLostReplyRecoveryWithoutSecondPOST(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
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
	drop := &cycleDropTransport{base: tr}
	client := &http.Client{Transport: drop, Timeout: 5 * time.Second}
	origin := "http://" + listener.Addr().String()
	b, _ := json.Marshal(cycleReceiveInput())
	req, e := http.NewRequest("POST", origin+cycleDelivery+"/receive", strings.NewReader(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	if r, e := client.Do(req); e == nil {
		r.Body.Close()
		t.Fatal("expected dropped reply")
	}
	if drop.discarded.Load() != 1 {
		t.Fatal("POST did not commit")
	}
	before := cycleCounts(t, f)
	req, e = http.NewRequest("GET", origin+"/local/v1/purchase-cycle-operations/receive-trip", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	r, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var out supplycycle.Result
	if json.NewDecoder(r.Body).Decode(&out) != nil || r.StatusCode != 200 || out.Receipt == nil || out.Receipt.AcceptedMilli != 2500 || !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal(out)
	}
}

func TestSupplyCycleManualDecisionRevisionCancellationAndLinkedWindow(t *testing.T) {
	f, _ := cycleFixture(t)
	declined := cycleDecisionInput()
	declined.Decision = "declined"
	declined.AcceptedMilli = 0
	declined.PromisedAt = ""
	first := cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", declined, 200)
	if first.Decision.ExternalVerified || first.Decision.EvidenceMode != "manual_external" {
		t.Fatal(first)
	}
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-one", 3000), 409)
	confirmed := cycleDecisionInput()
	confirmed.OperationID = "decision-confirm"
	confirmed.ExpectedRevision = 1
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", confirmed, 200)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("trip-window", "order", agendaStart()), 200)
	replacement := caseOpenBody()
	replacement.RequestedOutcome = "replacement"
	caseResponse(t, f, caseOpenPath, replacement, 200)
	plan := cyclePlanInput("trip-one", 3000)
	plan.DecisionRevision = 2
	plan.AppointmentID = "trip-window"
	plan.WindowEvidenceReference = "manual-accepted-window"
	plan.ReceivingCaseID = "case"
	planned := cycleCall(t, f, "POST", cycleOrder+"/deliveries", plan, 200)
	if planned.Delivery.AppointmentID != "trip-window" || planned.Delivery.ReceivingCaseID != "case" {
		t.Fatal(planned)
	}
	cancel := supplycycle.ActionInput{OperationID: "cancel-trip", ExpectedRevision: 1, Action: "cancel", EvidenceReference: "manual-cancel", ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Cancelamento humano da entrega"}
	cycleCall(t, f, "POST", cycleDelivery+"/actions", cancel, 200)
	plan = cyclePlanInput("replacement-trip", 5000)
	plan.DecisionRevision = 2
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", plan, 200)
	// Cancelling a shipment does not cancel the dock booking or resolve its case.
	var appointment dockagenda.Detail
	b := agendaCall(t, f, "GET", appointmentPath+"/trip-window", nil, 200)
	if json.Unmarshal(b, &appointment) != nil || appointment.Appointment.Status != "reserved" || caseGet(t, f).Case.Status != "open" {
		t.Fatal(string(b))
	}
	original := cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/decision-one", nil, 200)
	if !reflect.DeepEqual(original, first) {
		t.Fatal("manual decision history changed")
	}
}

func TestSupplyCycleLegacyReceiptAlsoConsumesCommitment(t *testing.T) {
	f, _ := cycleFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	cycleCall(t, f, "POST", cycleOrder+"/supplier-decisions", cycleDecisionInput(), 200)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-one", 2000), 200)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-two", 2000), 409)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-two", 1000), 200)
}

func TestSupplyCycleLocalReceivingAuthorizationIsStillRequired(t *testing.T) {
	f := agendaFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 99, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Logistics, modules.Finance}), 200)
	if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'receiving-room','receiving','Conferencia')`, f.owner.TenantID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	cycleAtArrival(t, f)
	before := cycleCounts(t, f)
	cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 409)
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("supplier evidence bypassed local receiving authority")
	}
}

func TestSupplyCyclePublishedExamplesMatchRealHandlers(t *testing.T) {
	f, reviewer := cycleFixture(t)
	b, e := os.ReadFile("../../../docs/api/examples/purchase-supply-cycle.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Steps []struct {
			Name, Actor, Method, Path string
			Body                      json.RawMessage
			Status                    int
			Expected                  map[string]any `json:"expected_subset"`
		}
	}
	if e = json.Unmarshal(b, &fixture); e != nil {
		t.Fatal(e)
	}
	var subset func(any, any) bool
	subset = func(want, got any) bool {
		if m, ok := want.(map[string]any); ok {
			actual, ok := got.(map[string]any)
			if !ok {
				return false
			}
			for k, v := range m {
				if !subset(v, actual[k]) {
					return false
				}
			}
			return true
		}
		return reflect.DeepEqual(want, got)
	}
	for _, step := range fixture.Steps {
		t.Run(step.Name, func(t *testing.T) {
			token := f.token
			if step.Actor == "reviewer" {
				token = reviewer
			}
			body := string(step.Body)
			if body == "null" {
				body = ""
			}
			status, b := request(t, f.app, step.Method, step.Path, body, token)
			var got map[string]any
			if json.Unmarshal(b, &got) != nil || status != step.Status || !subset(step.Expected, got) {
				t.Fatal(status, step.Status, string(b))
			}
		})
	}
	materialCount(t, f, "purchase_receipts", 1)
	materialCount(t, f, "supplier_payables", 1)
}

func TestSupplyCycleDamagedStockEvidenceBlocksFinancialApproval(t *testing.T) {
	f, reviewer := cycleFixture(t)
	received := cycleReceived(t, f)
	cycleCall(t, f, "POST", cycleDelivery+"/payable", cycleFinanceInput(), 200)
	if _, e := f.db.Exec(`UPDATE stock_movements SET reason='damaged evidence' WHERE id=?`, received.Receipt.MovementID); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 409)
	b := agendaCall(t, f, "GET", "/local/v1/finance/receiving-origins/invoice-one", nil, 200)
	var o supplycycle.Origin
	if json.Unmarshal(b, &o) != nil || !o.ReviewRequired {
		t.Fatal(string(b))
	}
}
