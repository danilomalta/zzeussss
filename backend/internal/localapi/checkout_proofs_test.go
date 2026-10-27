package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/sale"
)

const proofsPath = "/local/v1/checkout/proofs"
const submitProofPath = planPath + "/stable-cart/parts/debit-1/proofs"
const submitProofBody = `{"operation_id":"submit-proof-one","proof_id":"proof-one","provider":"terminal-provider","terminal":"terminal-one","reference":"reference-one","occurred_at":"2026-10-19T12:00:00Z"}`
const reviewProofBody = `{"operation_id":"review-proof-one","reason":"Referencia documental conferida; liquidacao nao consultada"}`

func proofSetup(t *testing.T) (*stockHTTPFixture, string) {
	t.Helper()
	f := checkoutSetup(t)
	planRequest(t, f, "POST", planPath, planJSON(t, checkoutInput(f)), 201)
	policyPerson(t, f.db, f.owner, "proof-reviewer", "manager")
	return f, loginToken(t, f.app, "proof-reviewer")
}
func proofResult(t *testing.T, f *stockHTTPFixture, token, method, path, body string, want int) sale.ProofResult {
	t.Helper()
	status, data := request(t, f.app, method, path, body, token)
	if status != want {
		t.Fatalf("proof %s %s: %d want%d %s", method, path, status, want, data)
	}
	var r sale.ProofResult
	if want == 200 || want == 201 {
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "checkout-proofs.openapi.json", "ProofResult", data)
	}
	return r
}
func readProof(t *testing.T, f *stockHTTPFixture, token, id string, want int) sale.PaymentProof {
	t.Helper()
	status, data := request(t, f.app, "GET", proofsPath+"/"+id, "", token)
	if status != want {
		t.Fatalf("proof read %d want%d %s", status, want, data)
	}
	var p sale.PaymentProof
	if want == 200 {
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "checkout-proofs.openapi.json", "PaymentProof", data)
	}
	return p
}
func proofCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	n := []int{}
	for _, table := range []string{"checkout_payment_proofs", "checkout_proof_operations", "checkout_proof_events", "sales", "sale_payments", "cash_movements", "stock_movements", "outbox", "cash_journal_entries", "checkout_plan_cancellations"} {
		var v int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&v); err != nil {
			t.Fatal(err)
		}
		n = append(n, v)
	}
	return n
}
func TestHTTPCheckoutProof315ReviewNeverBecomesProviderApprovalOrMixedSale(t *testing.T) {
	f, manager := proofSetup(t)
	before := proofCounts(t, f)
	original := proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	if original.Proof.Method != "debit" || original.Proof.AmountCents != 9600 || original.Proof.ProviderConfirmed || original.Proof.Effect != "evidence_only" || original.Proof.State != "submitted" {
		t.Fatal(original)
	}
	proofResult(t, f, f.token, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 403)
	accepted := proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200)
	if accepted.Proof.State != "accepted" || accepted.Proof.ProviderConfirmed || accepted.Proof.ReviewerID == nil || *accepted.Proof.ReviewerID != "proof-reviewer" {
		t.Fatal(accepted)
	}
	credit := strings.NewReplacer("submit-proof-one", "submit-credit", "proof-one", "proof-credit", "reference-one", "reference-credit").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "debit-1", "credit-1", 1), credit, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-credit/accept", strings.Replace(reviewProofBody, "review-proof-one", "review-credit", 1), 200)
	p := planRequest(t, f, "GET", planPath+"/stable-cart", "", 200)
	if p.CanFinalize || p.State != "prepared" || p.TotalCents != 31500 {
		t.Fatal(p)
	}
	for _, part := range p.Parts {
		if part.ProviderConfirmed || part.State != "not_requested" {
			t.Fatal("document promoted payment", part)
		}
	}
	planRequest(t, f, "POST", planPath+"/stable-cart/finalize-cash", `{"operation_id":"finalize-mixed","received_cents":31500}`, 409)
	planRequest(t, f, "POST", planPath+"/stable-cart/cancel", `{"operation_id":"cancel-with-evidence","reason":"Nao comprova estorno"}`, 409)
	if n := proofCounts(t, f); !reflect.DeepEqual(before[3:], n[3:]) {
		t.Fatal("document created financial effects", before, n)
	}
	if r := proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 200); !r.Repeated || !reflect.DeepEqual(r.Proof, original.Proof) {
		t.Fatal("original submission overwritten", r)
	}
	if r := proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200); !r.Repeated || !reflect.DeepEqual(r.Proof, accepted.Proof) {
		t.Fatal(r)
	}
	proofResult(t, f, manager, "GET", proofsPath+"/operations/submit-proof-one", "", 404)
	proofResult(t, f, f.token, "GET", proofsPath+"/operations/submit-proof-one", "", 200)
	planCount(t, f, 1, 3, 1, 0, 1)
}
func TestHTTPCheckoutProofReplacementReferenceUniquenessAndCancellationRemainConservative(t *testing.T) {
	f, manager := proofSetup(t)
	proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	replacement := strings.NewReplacer("submit-proof-one", "submit-proof-two", "proof-one", "proof-two", "reference-one", "reference-two").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", submitProofPath, replacement, 409)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/reject", reviewProofBody, 200)
	proofResult(t, f, f.token, "POST", submitProofPath, replacement, 201)
	reusedRef := strings.NewReplacer("submit-proof-one", "submit-reused", "proof-one", "proof-reused", "terminal-provider", "TERMINAL-PROVIDER", "terminal-one", "TERMINAL-ONE").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "debit-1", "credit-1", 1), reusedRef, 409)
	proofResult(t, f, f.token, "POST", proofsPath+"/proof-two/withdraw", strings.Replace(reviewProofBody, "review-proof-one", "withdraw-two", 1), 200)
	planRequest(t, f, "POST", planPath+"/stable-cart/cancel", `{"operation_id":"cancel-retired-evidence","reason":"Retirada nao comprova estorno"}`, 409)
	if _, err := f.db.Exec(`INSERT INTO checkout_plan_cancellations VALUES(?,?,?,?,?,?)`, f.owner.TenantID, "stable-cart", "direct-cancel", f.owner.OwnerID, "Sem resolucao externa", "now"); err == nil {
		t.Fatal("SQL bypassed external cancellation guard")
	}
	third := strings.NewReplacer("submit-proof-one", "submit-proof-three", "proof-one", "proof-three", "reference-one", "reference-three").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", submitProofPath, third, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-three/accept", strings.Replace(reviewProofBody, "review-proof-one", "review-three", 1), 200)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-three/revoke", strings.Replace(reviewProofBody, "review-proof-one", "revoke-three", 1), 200)
	if p := readProof(t, f, f.token, "proof-three", 200); p.State != "revoked" || len(p.History) != 3 || p.ProviderConfirmed {
		t.Fatal(p)
	}
	if p := readProof(t, f, f.token, "proof-one", 200); p.State != "rejected" || p.Reference != "reference-one" {
		t.Fatal(p)
	}
}
func TestHTTPCheckoutProofFailuresRollbackDocumentHistoryAndEvents(t *testing.T) {
	for _, stage := range []string{"submit_header_ignore", "submit_event", "review_state_ignore", "review_event"} {
		t.Run(stage, func(t *testing.T) {
			f, manager := proofSetup(t)
			token := f.token
			path := submitProofPath
			body := submitProofBody
			want := 500
			triggers := map[string]string{"submit_header_ignore": `CREATE TRIGGER injected BEFORE INSERT ON checkout_proof_operations BEGIN SELECT RAISE(IGNORE); END`, "submit_event": `CREATE TRIGGER injected BEFORE INSERT ON checkout_proof_events BEGIN SELECT RAISE(ABORT,'event'); END`, "review_state_ignore": `CREATE TRIGGER injected BEFORE UPDATE ON checkout_payment_proofs BEGIN SELECT RAISE(IGNORE); END`, "review_event": `CREATE TRIGGER injected BEFORE INSERT ON checkout_proof_events WHEN NEW.event_type='checkout.proof.accept' BEGIN SELECT RAISE(ABORT,'event'); END`}
			if strings.HasPrefix(stage, "review") {
				proofResult(t, f, f.token, "POST", path, body, 201)
				token = manager
				path = proofsPath + "/proof-one/accept"
				body = reviewProofBody
			}
			if strings.Contains(stage, "ignore") {
				want = 409
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			before := proofCounts(t, f)
			proofResult(t, f, token, "POST", path, body, want)
			if !reflect.DeepEqual(before, proofCounts(t, f)) {
				t.Fatal("partial documentary commit")
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			want = 201
			if strings.HasPrefix(stage, "review") {
				want = 200
			}
			proofResult(t, f, token, "POST", path, body, want)
		})
	}
}
func TestHTTPCheckoutProofConcurrentSubmissionAndOppositeReviewsHaveOneWinner(t *testing.T) {
	f, manager := proofSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, id := range []string{"one", "two"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReplacer("submit-proof-one", "submit-proof-"+id, "proof-one", "proof-"+id, "reference-one", "reference-"+id).Replace(submitProofBody)
			s, _ := request(t, f.app, "POST", submitProofPath, body, f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for s := range statuses {
		if s == 201 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	var id string
	if err := f.db.QueryRow(`SELECT id FROM checkout_payment_proofs`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	statuses = make(chan int, 2)
	for _, action := range []string{"accept", "reject"} {
		action := action
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := request(t, f.app, "POST", proofsPath+"/"+id+"/"+action, strings.Replace(reviewProofBody, "review-proof-one", action+"-proof", 1), manager)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict = 0, 0
	for s := range statuses {
		if s == 200 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}
func TestHTTPCheckoutProofBackupRestartAndLostReplyKeepOriginalEvidence(t *testing.T) {
	f, manager := proofSetup(t)
	want := proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200)
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	archive := filepath.Join(dir, "proof.tytbak")
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	if p, err := sale.ReadPaymentProof(ctx, db, actor, f.device, "proof-one"); err != nil || p.State != "accepted" || p.ProviderConfirmed {
		t.Fatal(p, err)
	}
	if r, err := sale.PaymentProofReceipt(ctx, db, actor, f.device, "submit-proof-one"); err != nil || !reflect.DeepEqual(r, want) {
		t.Fatal(r, err)
	}
	if _, err := db.Exec(`DELETE FROM checkout_payment_proofs`); err == nil {
		t.Fatal("restored proof deletable")
	}
	var seq int
	var name, original string
	if err = f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &original); err != nil {
		t.Fatal(err)
	}
	if err = f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db, err = localdb.Open(ctx, original)
	if err != nil {
		t.Fatal(err)
	}
	defer f.db.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(f.db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if r := proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 200); !r.Repeated || !reflect.DeepEqual(r.Proof, want.Proof) {
		t.Fatal(r)
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}
func TestHTTPCheckoutProofStrictInputsMethodPermissionsAndHistoricalReads(t *testing.T) {
	f, manager := proofSetup(t)
	for _, bad := range []string{`{}`, strings.Replace(submitProofBody, `"proof-one"`, `"bad/id"`, 1), strings.Replace(submitProofBody, `"reference-one"`, `null`, 1), strings.Replace(submitProofBody, `"2026-10-19T12:00:00Z"`, `"yesterday"`, 1), strings.Replace(submitProofBody, `"terminal-provider"`, `"terminal provider"`, 1), strings.Replace(submitProofBody, `"reference":"reference-one"`, `"reference":"reference-one","amount_cents":1`, 1)} {
		proofResult(t, f, f.token, "POST", submitProofPath, bad, 400)
	}
	proofResult(t, f, manager, "POST", submitProofPath, submitProofBody, 404)
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "debit-1", "cash-1", 1), submitProofBody, 400)
	proofResult(t, f, f.token, "POST", strings.Replace(submitProofPath, "debit-1", "missing", 1), submitProofBody, 404)
	policyPerson(t, f.db, f.owner, "other-cashier", "cashier")
	other := loginToken(t, f.app, "other-cashier")
	proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	readProof(t, f, other, "proof-one", 404)
	proofResult(t, f, other, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 403)
	proofResult(t, f, other, "POST", proofsPath+"/proof-one/withdraw", reviewProofBody, 404)
	for _, q := range []string{"?limit=0", "?limit=101", "?offset=-1", "?limit=1&limit=2", "?state=paid", "?state=submitted&state=accepted", "?tenant_id=other"} {
		if s, _ := request(t, f.app, "GET", proofsPath+q, "", manager); s != 400 {
			t.Fatal(q, s)
		}
	}
	for _, q := range []string{`DELETE FROM checkout_payment_proofs`, `UPDATE checkout_payment_proofs SET amount_cents=1`, `UPDATE checkout_proof_operations SET reason='rewrite'`, `DELETE FROM checkout_proof_events`} {
		if _, err := f.db.Exec(q); err == nil {
			t.Fatal("editable evidence", q)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	readProof(t, f, manager, "proof-one", 200)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 403)
	if _, err := f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id='proof-reviewer'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	readProof(t, f, manager, "proof-one", 401)
}
