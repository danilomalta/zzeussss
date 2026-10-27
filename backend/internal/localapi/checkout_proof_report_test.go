package localapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/sale"
)

func proofReport(t *testing.T, f *stockHTTPFixture, token string, want int) sale.ProofReport {
	t.Helper()
	status, b := request(t, f.app, "GET", proofsPath+"/report", "", token)
	if status != want {
		t.Fatalf("proof report %d want%d %s", status, want, b)
	}
	var r sale.ProofReport
	if want == 200 {
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "checkout-proofs.openapi.json", "ProofReport", b)
	} else if bytes.Contains(b, []byte("reported_active_cents")) {
		t.Fatal("partial report", string(b))
	}
	return r
}
func TestHTTPCheckoutProofReportQueueCSVNeverDoubleCountReplacements(t *testing.T) {
	f, manager := proofSetup(t)
	if r := proofReport(t, f, manager, 200); r.TotalCount != 0 || r.ReportedActiveCents != 0 || r.SettledCents != nil || r.ProviderConfirmed {
		t.Fatal(r)
	}
	proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200)
	credit := strings.NewReplacer("submit-proof-one", "submit-credit", "proof-one", "proof-credit", "reference-one", "reference-credit").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "debit-1", "credit-1", 1), credit, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-credit/accept", strings.Replace(reviewProofBody, "review-proof-one", "review-credit", 1), 200)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/revoke", strings.Replace(reviewProofBody, "review-proof-one", "revoke-debit", 1), 200)
	replacement := strings.NewReplacer("submit-proof-one", "submit-proof-two", "proof-one", "proof-two", "reference-one", "reference-two").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", submitProofPath, replacement, 201)
	r := proofReport(t, f, manager, 200)
	if r.Scope != "unverified_checkout_evidence" || r.TotalCount != 3 || r.PlansWithEvidence != 1 || r.ReportedActiveCents != 21500 || r.AcceptedEvidenceCents != 11900 || r.ProviderConfirmed || r.SettledCents != nil || r.Methods[0].HistoricalCount != 1 || r.Methods[0].SubmittedCount != 1 || r.Methods[0].ReportedActiveCents != 9600 {
		t.Fatal(r)
	}
	status, b := request(t, f.app, "GET", proofsPath, "", manager)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	domainSchemaFile(t, "checkout-proofs.openapi.json", "ProofPage", b)
	var page sale.ProofPage
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ID != "proof-two" {
		t.Fatal(page)
	}
	status, b = request(t, f.app, "GET", proofsPath+"?state=all&limit=1&offset=1", "", manager)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	domainSchemaFile(t, "checkout-proofs.openapi.json", "ProofPage", b)
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 3 || len(page.Items) != 1 || !page.HasMore || page.Items[0].ID != "proof-credit" {
		t.Fatal(page)
	}
	status, b = request(t, f.app, "GET", proofsPath+"/report.csv", "", manager)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(b), "\ufeff")))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatal(rows)
	}
	index := map[string]int{}
	for i, k := range rows[0] {
		index[k] = i
	}
	if rows[1][index["reported_active_cents"]] != "21500" || rows[1][index["accepted_evidence_cents"]] != "11900" || rows[1][index["settled_cents"]] != "" || rows[2][index["historical_count"]] != "1" || rows[5][index["state"]] != "revoked" || rows[7][index["proof_id"]] != "proof-two" {
		t.Fatal(rows)
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}
func TestHTTPCheckoutProofReportRejectsSourceOrHistoryDamageWithoutPartialExport(t *testing.T) {
	for _, damage := range []string{"source_part", "source_actor", "receipt", "event"} {
		t.Run(damage, func(t *testing.T) {
			f, manager := proofSetup(t)
			proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
			queries := map[string]string{"source_part": `UPDATE checkout_plan_parts SET amount_cents=1 WHERE part_id='debit-1'`, "source_actor": `UPDATE checkout_plans SET actor_identity_id='proof-reviewer'`, "receipt": `DROP TRIGGER checkout_proof_ops_no_update; UPDATE checkout_proof_operations SET result_json='{}'`, "event": `DROP TRIGGER checkout_proof_events_no_delete; DELETE FROM checkout_proof_events`}
			if _, err := f.db.Exec(queries[damage]); err != nil {
				t.Fatal(err)
			}
			before := proofCounts(t, f)
			readProof(t, f, manager, "proof-one", 409)
			proofReport(t, f, manager, 409)
			status, b := request(t, f.app, "GET", proofsPath+"/report.csv", "", manager)
			if status != 409 || bytes.Contains(b, []byte("format_version")) || bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
				t.Fatal(status, string(b))
			}
			if !reflect.DeepEqual(before, proofCounts(t, f)) {
				t.Fatal("read repaired source")
			}
		})
	}
}
func TestHTTPCheckoutProofReferenceFormulaKeepsJSONButRefusesCSV(t *testing.T) {
	f, manager := proofSetup(t)
	bad := strings.Replace(submitProofBody, "reference-one", "=HYPERLINK(1)", 1)
	proofResult(t, f, f.token, "POST", submitProofPath, bad, 201)
	if r := proofReport(t, f, manager, 200); r.Items[0].Reference != "=HYPERLINK(1)" || r.ReportedActiveCents != 9600 {
		t.Fatal(r)
	}
	if s, b := request(t, f.app, "GET", proofsPath+"/report.csv", "", manager); s != 409 || bytes.Contains(b, []byte("format_version")) {
		t.Fatal(s, string(b))
	}
	for _, path := range []string{proofsPath + "/report?limit=1", proofsPath + "/report.csv?state=all"} {
		if s, _ := request(t, f.app, "GET", path, "", manager); s != 400 {
			t.Fatal(path, s)
		}
	}
	policyPerson(t, f.db, f.owner, "report-cashier", "cashier")
	token := loginToken(t, f.app, "report-cashier")
	for _, path := range []string{proofsPath, proofsPath + "/report", proofsPath + "/report.csv"} {
		if s, _ := request(t, f.app, "GET", path, "", token); s != 403 {
			t.Fatal(path, s)
		}
	}
}
func TestHTTPCheckoutProofPixSupportedCreditAccountBlockedAndClosedShiftEvidenceIsHistorical(t *testing.T) {
	f, manager := proofSetup(t)
	alternative := checkoutInput(f)
	alternative.OperationID = "prepare-pix"
	alternative.CheckoutID = "pix-cart"
	alternative.Parts[1].Method = "pix"
	alternative.Parts[2].Method = "creditAccount"
	planRequest(t, f, "POST", planPath, planJSON(t, alternative), 201)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	// A reference to past external activity can be recorded after closing; it
	// still cannot re-open the drawer or produce a sale.
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "stable-cart", "pix-cart", 1), submitProofBody, 201)
	proofResult(t, f, f.token, "POST", planPath+"/pix-cart/parts/credit-1/proofs", strings.NewReplacer("submit-proof-one", "submit-credit-account", "proof-one", "proof-account", "reference-one", "ref-account").Replace(submitProofBody), 400)
	if p := readProof(t, f, manager, "proof-one", 200); p.Method != "pix" || p.ProviderConfirmed || p.AmountCents != 9600 {
		t.Fatal(p)
	}
	if r := proofReport(t, f, manager, 200); r.Methods[2].ReportedActiveCents != 9600 || r.Methods[0].ReportedActiveCents != 0 || r.SettledCents != nil {
		t.Fatal(r)
	}
	if p := planRequest(t, f, "GET", planPath+"/pix-cart", "", 200); p.CanFinalize || p.State != "prepared" {
		t.Fatal(p)
	}
}
func TestHTTPCheckoutProofReportLimitIsExplicitWithoutPartialData(t *testing.T) {
	f, manager := proofSetup(t)
	proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	// Size guard fixture only: duplicate historical rows are deliberately not
	// offered as valid audit histories or a production throughput benchmark.
	_, err := f.db.Exec(`WITH RECURSIVE n(x) AS(SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<1000)
 INSERT INTO checkout_payment_proofs(tenant_id,store_id,device_id,id,checkout_id,part_id,submitter_id,method,amount_cents,provider,terminal,reference,occurred_at,state,reviewer_id,created_at,updated_at,last_operation_id)
 SELECT p.tenant_id,p.store_id,p.device_id,'limit-'||n.x,p.checkout_id,p.part_id,p.submitter_id,p.method,p.amount_cents,p.provider,p.terminal,'limit-reference-'||n.x,p.occurred_at,'rejected','proof-reviewer',p.created_at,p.updated_at,p.last_operation_id FROM checkout_payment_proofs p,n WHERE p.id='proof-one'`)
	if err != nil {
		t.Fatal(err)
	}
	proofReport(t, f, manager, 422)
	if s, b := request(t, f.app, "GET", proofsPath+"/report.csv", "", manager); s != 422 || bytes.Contains(b, []byte("format_version")) {
		t.Fatal(s, string(b))
	}
}
