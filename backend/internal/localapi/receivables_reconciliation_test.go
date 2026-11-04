package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/receivables"
)

func arReconcileBody(id, action string) string {
	return fmt.Sprintf(`{"document_id":%q,"action":%q,"confirmed":true}`, id, action)
}
func TestHTTPReceivablesReconcileAbsentDurableFence(t *testing.T) {
	f := arSetup(t)
	path := arPath + "/operations/bill-create/reconcile"
	body := arReconcileBody("bill", "create")
	first := arResult(t, f, path, body, 200)
	if first.Resolution != "not_applied" || first.DocumentID != "bill" || first.Repeated {
		t.Fatal("not a new fence", first)
	}
	again := arResult(t, f, path, body, 200)
	if !again.Repeated || again.Resolution != "not_applied" {
		t.Fatal("duplicate reconciliation", again)
	}
	arResult(t, f, arPath, arBody, 409)
	code, raw := request(t, f.app, "GET", arPath+"/operations/bill-create", "", f.token)
	if code != 200 {
		t.Fatal(code)
	}
	var result receivables.Result
	if json.Unmarshal(raw, &result) != nil || result.Resolution != "not_applied" || !result.Repeated {
		t.Fatal("lost reconciliation reply", string(raw))
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM receivable_documents`).Scan(&n); e != nil || n != 0 {
		t.Fatal("late create took effect", n, e)
	}
	for _, sql := range []string{`UPDATE receivable_reconciliations SET action='cancel'`, `DELETE FROM receivable_reconciliations`} {
		if _, e := f.db.Exec(sql); e == nil {
			t.Fatal("audit changed")
		}
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "reconciled.tytbak")
	ctx := context.Background()
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Restore(ctx, archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	got, e := receivables.Operation(ctx, db, actor, f.device, "bill-create")
	if e != nil || got.Resolution != "not_applied" || got.DocumentID != "bill" {
		t.Fatal("fence lost after restore", got, e)
	}
}
func TestHTTPReceivablesReconcileExistingReceiptNeverReversesCash(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("paid", "one", 300, 0), 200)
	got := arResult(t, f, arPath+"/operations/paid/reconcile", arReconcileBody("bill", "receive_cash"), 200)
	if !got.Repeated || got.Resolution != "" || got.Document.ReceivedCents != 300 {
		t.Fatal(got)
	}
	arResult(t, f, arPath+"/operations/paid/reconcile", arReconcileBody("different", "receive_cash"), 409)
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM receivable_reconciliations`).Scan(&n); e != nil || n != 0 {
		t.Fatal("receipt was fenced", n, e)
	}
	var amount int64
	if e := f.db.QueryRow(`SELECT sum(amount_cents) FROM cash_adjustments WHERE kind='supply'`).Scan(&amount); e != nil || amount != 300 {
		t.Fatal("cash changed", amount, e)
	}
}
func TestHTTPReceivablesReconcileRacesReceipt(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	var wg sync.WaitGroup
	var payment int
	var reconciled receivables.Result
	wg.Add(2)
	go func() {
		defer wg.Done()
		payment, _ = request(t, f.app, "POST", arPath+"/bill/receive-cash", arReceive("race-paid", "one", 300, 0), f.token)
	}()
	go func() {
		defer wg.Done()
		reconciled = arResult(t, f, arPath+"/operations/race-paid/reconcile", arReconcileBody("bill", "receive_cash"), 200)
	}()
	wg.Wait()
	var count, fences int
	if e := f.db.QueryRow(`SELECT count(*) FROM receivable_cash_receipts`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if e := f.db.QueryRow(`SELECT count(*) FROM receivable_reconciliations`).Scan(&fences); e != nil {
		t.Fatal(e)
	}
	if payment == 200 {
		if count != 1 || fences != 0 || reconciled.Resolution != "" {
			t.Fatal("applied receipt lost", count, fences, reconciled)
		}
	} else if payment == 409 {
		if count != 0 || fences != 1 || reconciled.Resolution != "not_applied" {
			t.Fatal("late write passed fence", count, fences, reconciled)
		}
	} else {
		t.Fatal("unexpected receipt status", payment)
	}
}
func TestHTTPReceivablesReconcileRollbackAndDuplicate(t *testing.T) {
	for _, mode := range []string{"ABORT", "IGNORE"} {
		t.Run(mode, func(t *testing.T) {
			f := arSetup(t)
			query := `CREATE TRIGGER reject_reconcile BEFORE INSERT ON receivable_reconciliations BEGIN SELECT RAISE(` + mode
			if mode == "ABORT" {
				query += `,'synthetic audit failure'`
			}
			query += `); END;`
			if _, e := f.db.Exec(query); e != nil {
				t.Fatal(e)
			}
			want := 500
			if mode == "IGNORE" {
				want = 409
			}
			arResult(t, f, arPath+"/operations/bill-create/reconcile", arReconcileBody("bill", "create"), want)
			code, _ := request(t, f.app, "GET", arPath+"/operations/bill-create", "", f.token)
			if code != 404 {
				t.Fatal("partial fence survived", code)
			}
			arResult(t, f, arPath, arBody, 201)
		})
	}
	t.Run("concurrent duplicate", func(t *testing.T) {
		f := arSetup(t)
		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				defer wg.Done()
				arResult(t, f, arPath+"/operations/bill-create/reconcile", arReconcileBody("bill", "create"), 200)
			}()
		}
		wg.Wait()
		var n int
		if e := f.db.QueryRow(`SELECT count(*) FROM receivable_reconciliations`).Scan(&n); e != nil || n != 1 {
			t.Fatal(n, e)
		}
	})
}
func TestHTTPReceivablesReconcileAuthorizationAndCorruptCash(t *testing.T) {
	f := arSetup(t)
	path := arPath + "/operations/orphan/reconcile"
	body := arReconcileBody("bill", "receive_cash")
	for _, role := range []string{"cashier", "accountant"} {
		name := "reconcile-" + role
		policyPerson(t, f.db, f.owner, name, role)
		token := loginToken(t, f.app, name)
		code, _ := request(t, f.app, "POST", path, body, token)
		if code != 403 {
			t.Fatal(role, code)
		}
	}
	for _, bad := range []string{`{"document_id":"bill","action":"receive_cash","confirmed":false}`, `{"document_id":"bill","action":"receive_cash","confirmed":true,"tenant_id":"forged"}`} {
		arResult(t, f, path, bad, 400)
	}
	raw, _ := json.Marshal([]string{f.owner.TenantID, f.owner.StoreID, f.device.DeviceID, "orphan"})
	op := fmt.Sprintf("ar-%x", sha256.Sum256(raw))
	cashRequest(t, f, "movements", fmt.Sprintf(`{"session_id":"ar-turn","operation_id":%q,"kind":"supply","amount_cents":10,"reason":"Synthetic inconsistent cash origin"}`, op), 200)
	arResult(t, f, path, body, 409)
	// A different user cannot retire another user's already-persisted fence.
	arResult(t, f, arPath+"/operations/owned/reconcile", arReconcileBody("bill", "create"), 200)
	policyPerson(t, f.db, f.owner, "reconcile-manager", "manager")
	manager := loginToken(t, f.app, "reconcile-manager")
	code, _ := request(t, f.app, "GET", arPath+"/operations/owned", "", manager)
	if code != 404 {
		t.Fatal("other actor read fence", code)
	}
	code, _ = request(t, f.app, "POST", arPath+"/operations/owned/reconcile", arReconcileBody("bill", "create"), manager)
	if code != 409 {
		t.Fatal("other actor adopted fence", code)
	}
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: "another-store", IdentityID: f.owner.OwnerID}
	if _, e := receivables.Operation(context.Background(), f.db, actor, f.device, "owned"); e == nil {
		t.Fatal("cross-store read")
	}
	actor.StoreID = f.owner.StoreID
	actor.TenantID = "another-tenant"
	if _, e := receivables.Operation(context.Background(), f.db, actor, f.device, "owned"); e == nil {
		t.Fatal("cross-tenant read")
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 91, []modules.ID{modules.Core}), 200)
	arResult(t, f, arPath+"/operations/unlicensed/reconcile", arReconcileBody("bill", "create"), 403)
}

func TestHTTPReceivablesReconcileDelayedCashAndCancellation(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath+"/operations/late-cash/reconcile", arReconcileBody("bill", "receive_cash"), 200)
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("late-cash", "one", 300, 0), 409)
	arResult(t, f, arPath+"/operations/late-cancel/reconcile", arReconcileBody("bill", "cancel"), 200)
	arResult(t, f, arPath+"/bill/cancel", `{"operation_id":"late-cancel","reason":"Late command must not cancel"}`, 409)
	code, raw := request(t, f.app, "GET", arPath+"/bill", "", f.token)
	var doc receivables.Document
	if code != 200 || json.Unmarshal(raw, &doc) != nil || doc.State != "open" || doc.ReceivedCents != 0 {
		t.Fatal("fencing changed document", code, doc)
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM cash_adjustments`).Scan(&n); e != nil || n != 0 {
		t.Fatal("late cash applied", n, e)
	}
}
func TestHTTPReceivablesReconcileRevokedBeforeTransaction(t *testing.T) {
	f := arSetup(t)
	ctx := context.Background()
	if e := localauth.Logout(ctx, f.db, f.token); e != nil {
		t.Fatal(e)
	}
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	contracts, e := entitlementstore.New(f.db, verifier, nil)
	if e != nil {
		t.Fatal(e)
	}
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	_, e = receivables.Reconcile(ctx, f.db, contracts, actor, f.device, f.token, "stale", receivables.ReconcileInput{DocumentID: "bill", Action: "create", Confirmed: true})
	if !errors.Is(e, localauth.ErrDenied) {
		t.Fatal("revocation between HTTP and transaction", e)
	}
	arResult(t, f, arPath+"/operations/stale/reconcile", arReconcileBody("bill", "create"), 401)
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM receivable_reconciliations`).Scan(&n); e != nil || n != 0 {
		t.Fatal("revoked fence committed", n, e)
	}
}
