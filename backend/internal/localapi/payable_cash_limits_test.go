package localapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/payables"
)

func TestHTTPPayableCashMaximumExactAmountAndLifetimeLimit(t *testing.T) {
	f := httpContractSetup(t)
	reviewer, _ := pcPrepare(t, f)
	var in payables.CreateInput
	if e := json.Unmarshal([]byte(payableBody), &in); e != nil {
		t.Fatal(e)
	}
	in.OperationID = "create-max"
	in.PayableID = "invoice-max"
	in.DocumentReference = "NF-max"
	in.TotalCents = payables.MaxSafe
	in.Installments = []payables.Installment{{ID: "part-one", DueOn: "2026-10-10", AmountCents: payables.MaxSafe}}
	b, _ := json.Marshal(in)
	payableResult(t, f, f.token, "POST", payablesPath, string(b), 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-max/approve", strings.Replace(payableApprove, "approve-invoice", "approve-max", 1), 200)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":9007199254740991}`, 201)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", payables.MaxSafe)
	got := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-max/cash-payments", pcPay, 201)
	if got.Payment.AmountCents != payables.MaxSafe {
		t.Fatal(got)
	}
	pcMove(t, f, f.token, "return-supply", "supply", payables.MaxSafe)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	pcMove(t, f, f.token, "withdraw-small", "withdrawal", 1)
	before := pcCounts(t, f)
	body := strings.NewReplacer("pay-one", "pay-small", "payment-one", "payment-small", "withdraw-one", "withdraw-small", "receipt-one", "receipt-small").Replace(pcPay)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-max/cash-payments", body, 422)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("overflow allocation")
	}
}
func TestHTTPPayableCashDamagedSourceAndPendingDocumentRefused(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	payableResult(t, f, f.token, "POST", payablesPath, strings.NewReplacer("create-invoice", "create-pending", "invoice-one", "invoice-pending", "NFe-123", "NF-pending").Replace(payableBody), 201)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-pending/cash-payments", pcPay, 409)
	pcMove(t, f, f.token, "wrong-kind", "supply", 10000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", strings.Replace(pcPay, "withdraw-one", "wrong-kind", 1), 409)
	before := pcCounts(t, f)
	if _, e := f.db.Exec(`UPDATE cash_adjustments SET amount_cents=9999 WHERE operation_id='withdraw-one'`); e != nil {
		t.Fatal(e)
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 409)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("damaged audit allocated")
	}
}
