package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/payables"
)

var flowFixtures = flag.String("payables-flow-fixtures", "", "Explicit test-only output directory for sanitized contract examples; empty compares checked-in fixtures")

type flowExample struct {
	Name     string `json:"name"`
	Actor    string `json:"actor"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Request  any    `json:"request,omitempty"`
	Status   int    `json:"status"`
	Response any    `json:"response"`
}

func flowCanonical(t *testing.T, f *httpContractFixture, v any) any {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if k == "created_at" || k == "updated_at" || k == "generated_at" || k == "occurred_at" {
				x[k] = "2026-10-20T12:00:00Z"
				continue
			}
			if k == "movement_id" {
				x[k] = "movement-test-" + x["operation_id"].(string)
				continue
			}
			x[k] = flowCanonical(t, f, val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = flowCanonical(t, f, val)
		}
		return x
	case string:
		for old, replacement := range map[string]string{f.owner.TenantID: "tenant-test", f.owner.StoreID: "store-test", f.owner.DeviceID: "device-test", f.owner.OwnerID: "owner-test"} {
			x = strings.ReplaceAll(x, old, replacement)
		}
		return x
	default:
		return v
	}
}
func flowReopen(t *testing.T, f *httpContractFixture) {
	t.Helper()
	var n int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	f.db = db
	t.Cleanup(func() { db.Close() })
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
}
func TestHTTPPayablesFlowEndToEndAndVerifiedExamples(t *testing.T) {
	f, reviewer, accountant := payableSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 90, []modules.ID{modules.Core, modules.Finance, modules.POS, modules.Inventory, modules.Orders}), 200)
	steps := []flowExample{}
	call := func(name, actor, token, method, path, body string, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Titan-Error-Format", "v1")
		response, e := f.app.Test(req, -1)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		b, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		if response.StatusCode != want {
			t.Fatalf("%s got%d want%d %s", name, response.StatusCode, want, b)
		}
		if want >= 400 {
			domainSchemaFile(t, "http-errors.openapi.json", "Error", b)
		} else if !strings.Contains(response.Header.Get("Content-Type"), "text/csv") {
			schemaFile, schemaName := "supplier-payables.openapi.json", "PayableResult"
			clean := strings.Split(path, "?")[0]
			switch {
			case strings.Contains(clean, "/cash/movements/operations/"):
				schemaFile, schemaName = "cash-movement-receipt.openapi.json", "MovementReceipt"
			case strings.HasSuffix(clean, "installment-balances"):
				schemaFile, schemaName = "payable-readiness.openapi.json", "InstallmentBalances"
			case strings.HasSuffix(clean, "cash-sources") || strings.HasSuffix(clean, "return-sources"):
				schemaFile, schemaName = "payable-readiness.openapi.json", "CashCandidates"
			case strings.Contains(clean, "/payable-cash/"):
				schemaFile, schemaName = "payable-cash.openapi.json", "CashResult"
			case strings.HasSuffix(clean, "cash-payments"):
				schemaFile, schemaName = "payable-cash.openapi.json", "CashResult"
				if method == "GET" {
					schemaName = "CashPage"
				}
			case strings.HasSuffix(clean, "/report"):
				schemaName = "PayablesReport"
			case clean == payablesPath && method == "GET":
				schemaName = "PayablePage"
			case clean == payablesPath+"/invoice-one" && method == "GET":
				schemaName = "Payable"
			}
			domainSchemaFile(t, schemaFile, schemaName, b)
		}
		var input, output any
		if body != "" {
			if e = json.Unmarshal([]byte(body), &input); e != nil {
				t.Fatal(e)
			}
		}
		if strings.Contains(response.Header.Get("Content-Type"), "text/csv") {
			output = string(b)
		} else if e = json.Unmarshal(b, &output); e != nil {
			t.Fatal(e)
		}
		steps = append(steps, flowExample{name, actor, method, path, flowCanonical(t, f, input), want, flowCanonical(t, f, output)})
		return b
	}
	call("create_document", "owner", f.token, "POST", payablesPath, payableBody, 201)
	afterCreate := pcCounts(t, f)
	call("recover_document_after_discarded_reply", "owner", f.token, "GET", payablesPath+"/operations/create-invoice", "", 200)
	if !reflect.DeepEqual(afterCreate, pcCounts(t, f)) {
		t.Fatal("document lookup wrote")
	}
	call("creator_cannot_approve", "owner", f.token, "POST", payablesPath+"/invoice-one/approve", payableApprove, 403)
	call("approve_document", "reviewer", reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	call("recover_approval", "reviewer", reviewer, "GET", payablesPath+"/operations/approve-invoice", "", 200)
	call("replay_original_create", "owner", f.token, "POST", payablesPath, payableBody, 200)
	call("document_with_current_cash", "accountant", accountant, "GET", payablesPath+"/invoice-one", "", 200)
	call("documents_page", "accountant", accountant, "GET", payablesPath+"?state=approved&limit=1&offset=0", "", 200)
	call("initial_balances", "accountant", accountant, "GET", readinessBalances, "", 200)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":50000}`, 201)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcMove(t, f, f.token, "withdraw-two", "withdrawal", 4000)
	call("recover_original_withdrawal", "owner", f.token, "GET", "/local/v1/cash/movements/operations/withdraw-one", "", 200)
	beforeQuery := pcCounts(t, f)
	call("select_payment_source", "owner", f.token, "GET", readinessPay, "", 200)
	if !reflect.DeepEqual(beforeQuery, pcCounts(t, f)) {
		t.Fatal("source consultation reserved money")
	}
	cashBefore := payableCounts(t, f)
	first := call("confirm_first_cash_link", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	if !reflect.DeepEqual(cashBefore, payableCounts(t, f)) {
		t.Fatal("cash link created a new drawer movement")
	}
	// The result is deliberately not used for client confirmation. Recovery uses
	// only the original actor's receipt; it never creates a replacement operation.
	_ = first
	committed := pcCounts(t, f)
	call("recover_cash_link_after_discarded_reply", "owner", f.token, "GET", pcPath+"/operations/pay-one", "", 200)
	if !reflect.DeepEqual(committed, pcCounts(t, f)) {
		t.Fatal("recovery duplicated a payment")
	}
	call("replay_same_cash_link", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 200)
	call("same_id_changed_content", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", strings.Replace(pcPay, "receipt-one", "receipt-different", 1), 409)
	duplicateSource := strings.NewReplacer("pay-one", "pay-duplicate", "payment-one", "payment-duplicate", "receipt-one", "receipt-duplicate").Replace(pcPay)
	call("same_origin_different_ids", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", duplicateSource, 409)
	second := strings.NewReplacer("pay-one", "pay-two", "payment-one", "payment-two", "withdraw-one", "withdraw-two", "receipt-one", "receipt-two").Replace(pcPay)
	call("complete_first_installment", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", second, 201)
	pcMove(t, f, f.token, "withdraw-over", "withdrawal", 1)
	over := strings.NewReplacer("pay-one", "pay-over", "payment-one", "payment-over", "withdraw-one", "withdraw-over", "receipt-one", "receipt-over").Replace(pcPay)
	call("above_installment_balance", "owner", f.token, "POST", payablesPath+"/invoice-one/cash-payments", over, 409)
	call("fully_applied_source_list", "owner", f.token, "GET", readinessPay, "", 200)
	pcMove(t, f, f.token, "return-supply", "supply", 6000)
	call("recover_original_supply", "owner", f.token, "GET", "/local/v1/cash/movements/operations/return-supply", "", 200)
	call("select_return_source", "owner", f.token, "GET", pcPath+"/payment-one/return-sources", "", 200)
	cashBefore = payableCounts(t, f)
	call("confirm_integral_return", "owner", f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	if !reflect.DeepEqual(cashBefore, payableCounts(t, f)) {
		t.Fatal("return link created a new drawer movement")
	}
	call("recover_integral_return", "owner", f.token, "GET", pcPath+"/operations/return-one", "", 200)
	call("replay_integral_return", "owner", f.token, "POST", pcPath+"/payment-one/return", pcReturn, 200)
	call("second_return_new_operation", "owner", f.token, "POST", pcPath+"/payment-one/return", strings.Replace(pcReturn, "return-one", "return-duplicate", 1), 409)
	call("original_payment_receipt_is_immutable", "owner", f.token, "GET", pcPath+"/operations/pay-one", "", 200)
	call("cancel_with_applied_money", "reviewer", reviewer, "POST", payablesPath+"/invoice-one/cancel", `{"operation_id":"cancel-paid","reason":"Documento precisa de revisao"}`, 409)
	beforeRestart := readinessParts(t, f, accountant)
	flowReopen(t, f)
	afterRestart := readinessParts(t, f, accountant)
	if !reflect.DeepEqual(beforeRestart.Items, afterRestart.Items) || beforeRestart.Summary != afterRestart.Summary {
		t.Fatal("restart lost accounting state")
	}
	balances := call("balances_after_restart", "accountant", accountant, "GET", readinessBalances, "", 200)
	var parts payables.InstallmentBalances
	if e := json.Unmarshal(balances, &parts); e != nil {
		t.Fatal(e)
	}
	if parts.Summary.PaidCents != 10000 || parts.Summary.ReturnedCents != 6000 || parts.Summary.AppliedCents != 4000 || parts.Summary.OutstandingCents != 27500 || parts.Items[0].OutstandingCents != 6000 {
		t.Fatal(parts)
	}
	call("cash_history_after_restart", "accountant", accountant, "GET", payablesPath+"/invoice-one/cash-payments?limit=1&offset=0", "", 200)
	call("cash_history_second_page", "accountant", accountant, "GET", payablesPath+"/invoice-one/cash-payments?limit=1&offset=1", "", 200)
	reportBody := call("final_report", "accountant", accountant, "GET", payablesPath+"/report"+payableQueryString, "", 200)
	var report payables.Report
	if e := json.Unmarshal(reportBody, &report); e != nil {
		t.Fatal(e)
	}
	csvBody := call("final_csv_format2", "accountant", accountant, "GET", payablesPath+"/report.csv"+payableQueryString, "", 200)
	parser := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(csvBody, []byte{239, 187, 191})))
	parser.Comma = ';'
	records, e := parser.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	metrics := map[string]int64{}
	items := map[string][]string{}
	for _, row := range records[1:] {
		if row[0] != "2" {
			t.Fatal(row)
		}
		if row[1] == "summary" {
			n, e := strconv.ParseInt(row[8], 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			metrics[row[7]] = n
		} else {
			items[row[14]] = row
		}
	}
	if !reflect.DeepEqual(metrics, report.Summary) || metrics["cash_applied_cents"] != parts.Summary.AppliedCents || metrics["approved_outstanding_cents"] != parts.Summary.OutstandingCents || metrics["approved_cents"] != 31500 {
		t.Fatal("CSV/report/balances disagree")
	}
	for _, p := range parts.Items {
		row := items[p.Installment.ID]
		if row[16] != strconv.FormatInt(p.Installment.AmountCents, 10) || row[18] != strconv.FormatInt(p.AppliedCents, 10) || row[19] != strconv.FormatInt(p.OutstandingCents, 10) {
			t.Fatal(row, p)
		}
	}
	fixture := map[string]any{"test_data_only": true, "normalization": "Only generated actor/context/movement IDs and timestamps are normalized; amounts, states, original operation IDs and HTTP results come from real handlers and disposable SQLite.", "steps": steps}
	actual, e := json.MarshalIndent(fixture, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	actual = append(actual, '\n')
	name := "../../../docs/api/examples/payables-flow.json"
	if *flowFixtures != "" {
		if e = os.MkdirAll(*flowFixtures, 0700); e != nil {
			t.Fatal(e)
		}
		name = filepath.Join(*flowFixtures, "payables-flow.json")
		if e = os.WriteFile(name, actual, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(*flowFixtures, "payables-flow.csv"), csvBody, 0600); e != nil {
			t.Fatal(e)
		}
	} else {
		saved, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(saved, actual) {
			t.Fatal("verified examples differ from actual flow; regenerate only into explicit scratch fixture directory and inspect differences")
		}
		saved, e = os.ReadFile("../../../docs/api/examples/payables-flow.csv")
		if e != nil || !bytes.Equal(saved, csvBody) {
			t.Fatal("CSV example differs from final actual report")
		}
	}
}

type flowDropTransport struct {
	base  *http.Transport
	posts atomic.Int32
}

func (d *flowDropTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response, e := d.base.RoundTrip(req)
	if e == nil && req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/cash-payments") && response.StatusCode == 201 {
		d.posts.Add(1)
		response.Body.Close()
		return nil, errors.New("test-only discarded committed HTTP reply")
	}
	return response, e
}
func TestHTTPPayablesFlowLostReplyOverLoopbackAndAuthoritativeRecovery(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.ShutdownWithTimeout(2 * time.Second)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	drop := &flowDropTransport{base: transport}
	client := &http.Client{Transport: drop, Timeout: 5 * time.Second}
	origin := "http://" + listener.Addr().String()
	req, e := http.NewRequest("POST", origin+payablesPath+"/invoice-one/cash-payments", strings.NewReader(pcPay))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	if response, e := client.Do(req); e == nil {
		response.Body.Close()
		t.Fatal("lost response was not simulated")
	}
	if drop.posts.Load() != 1 {
		t.Fatal("request did not commit before transport discarded reply")
	}
	before := pcCounts(t, f)
	req, e = http.NewRequest("GET", origin+pcPath+"/operations/pay-one", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	response, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	var receipt payables.CashResult
	if e = json.NewDecoder(response.Body).Decode(&receipt); e != nil {
		t.Fatal(e)
	}
	if response.StatusCode != 200 || receipt.OperationID != "pay-one" || receipt.Payment.AmountCents != 6000 || drop.posts.Load() != 1 || !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("lost reply recovery duplicated or failed", response.StatusCode, receipt)
	}
}
func TestHTTPPayablesFlowExpiredLicenseAndRevokedSession(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 90, IssuedAt: now - 600, NotBefore: now - 600, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Finance, modules.POS, modules.Inventory, modules.Orders}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=? WHERE tenant_id=? AND revision=90`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload)), f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{payablesPath + "/invoice-one", readinessBalances, readinessPay, pcPath + "/operations/pay-one", "/local/v1/cash/movements/operations/withdraw-one", payablesPath + "/operations/create-invoice", payablesPath + "/report" + payableQueryString, payablesPath + "/report.csv" + payableQueryString} {
		if s, b := request(t, f.app, "GET", path, "", f.token); s != 200 {
			t.Fatal(path, s, string(b))
		}
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 403)
	if s, b := request(t, f.app, "POST", "/local/v1/logout", "", f.token); s != 204 {
		t.Fatal(s, string(b))
	}
	for _, path := range []string{readinessBalances, readinessPay, pcPath + "/operations/pay-one"} {
		if s, _ := request(t, f.app, "GET", path, "", f.token); s != 401 {
			t.Fatal("revoked session read", s)
		}
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 401)
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	pcResult(t, f, f.token, "GET", pcPath+"/operations/pay-one", "", 200)
}

func TestHTTPPayablesFlowOpenAPIExamplesStayInSync(t *testing.T) {
	b, e := os.ReadFile("../../../docs/api/examples/payables-flow.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Steps []flowExample `json:"steps"`
	}
	if e = json.Unmarshal(b, &fixture); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("../../../docs/api/payables-flow.openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct {
					Examples map[string]struct {
						Value any `json:"value"`
					} `json:"examples"`
				} `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if e = json.Unmarshal(b, &spec); e != nil {
		t.Fatal(e)
	}
	for _, step := range fixture.Steps {
		found := 0
		method := strings.ToLower(step.Method)
		actual := strings.Split(strings.Split(step.Path, "?")[0], "/")
		for pattern, item := range spec.Paths {
			if _, literal := spec.Paths[strings.Split(step.Path, "?")[0]][method]; literal && pattern != strings.Split(step.Path, "?")[0] {
				continue
			}
			expected := strings.Split(pattern, "/")
			if len(actual) != len(expected) {
				continue
			}
			match := true
			for i, v := range expected {
				if !strings.HasPrefix(v, "{") && actual[i] != v {
					match = false
				}
			}
			if !match {
				continue
			}
			op, ok := item[method]
			if !ok {
				continue
			}
			media := "application/json"
			if strings.Contains(step.Name, "csv") {
				media = "text/csv"
			}
			value, ok := op.Responses[strconv.Itoa(step.Status)].Content[media].Examples[step.Name]
			if !ok || !reflect.DeepEqual(value.Value, step.Response) {
				t.Fatalf("OpenAPI example differs: %s", step.Name)
			}
			found++
		}
		if found != 1 {
			t.Fatalf("example route mapping %s: %d", step.Name, found)
		}
	}
}
