package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/identity"
)

const bagsPath = "/local/v1/cash/bags"
const createBagBody = `{"operation_id":"prepare-one","bag_id":"bag-one","withdrawal_operation_id":"withdraw-one","seal_id":"seal-one"}`
const dispatchBagBody = `{"operation_id":"dispatch-one"}`
const receiveBagBody = `{"operation_id":"receive-one","seal_id":"seal-one","counted_cents":290}`
const coveragePath = "/local/v1/cash/sessions/turn-one/custody"

func bagSetup(t *testing.T) (*stockHTTPFixture, string) {
	t.Helper()
	f := salesSetup(t)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	policyPerson(t, f.db, f.owner, "receiver", "manager")
	return f, loginToken(t, f.app, "receiver")
}
func bagResult(t *testing.T, f *stockHTTPFixture, token, method, path, body string, want int) cashcustody.Result {
	t.Helper()
	status, data := request(t, f.app, method, path, body, token)
	if status != want {
		t.Fatalf("%s %s: %d want %d %s", method, path, status, want, data)
	}
	var v cashcustody.Result
	if want == 200 || want == 201 {
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-bags.openapi.json", "BagResult", data)
	}
	return v
}
func bagRead(t *testing.T, f *stockHTTPFixture, token, id string, want int) cashcustody.Bag {
	t.Helper()
	status, data := request(t, f.app, "GET", bagsPath+"/"+id, "", token)
	if status != want {
		t.Fatalf("read %d want %d %s", status, want, data)
	}
	var b cashcustody.Bag
	if want == 200 {
		if err := json.Unmarshal(data, &b); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-bags.openapi.json", "Bag", data)
	}
	return b
}
func bagCoverage(t *testing.T, f *stockHTTPFixture, token string, want int) cashcustody.Coverage {
	t.Helper()
	status, data := request(t, f.app, "GET", coveragePath, "", token)
	if status != want {
		t.Fatalf("coverage %d want %d %s", status, want, data)
	}
	var v cashcustody.Coverage
	if want == 200 {
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-bags.openapi.json", "CustodyCoverage", data)
	} else if bytes.Contains(data, []byte("withdrawn_cents")) {
		t.Fatal("failed coverage exposed totals")
	}
	return v
}
func bagCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	var counts []int
	for _, table := range []string{"cash_movements", "cash_journal_entries", "stock_movements", "cash_bags", "cash_bag_operations", "cash_bag_outbox", "outbox"} {
		var n int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, n)
	}
	return counts
}

func TestHTTPBagLifecycleBlindCountAndDurableOperationReceipts(t *testing.T) {
	f, receiver := bagSetup(t)
	before := bagCounts(t, f)
	first := bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	if first.Bag.State != "prepared" || first.Bag.DeclaredCents == nil || *first.Bag.DeclaredCents != 300 || len(first.Bag.History) != 1 {
		t.Fatal(first)
	}
	if b := bagRead(t, f, receiver, "bag-one", 200); b.DeclaredCents != nil || b.CountedCents != nil || b.DifferenceCents != nil || len(b.History) != 1 {
		t.Fatal("receiver saw declaration", b)
	}
	bagCoverage(t, f, f.token, 409)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 409)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 403)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 403)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "seal-one", "wrong", 1), 409)
	bagResult(t, f, receiver, "GET", bagsPath+"/operations/prepare-one", "", 404)
	got := bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
	if got.Bag.State != "received" || got.Bag.CountedCents == nil || *got.Bag.CountedCents != 290 || got.Bag.DifferenceCents == nil || *got.Bag.DifferenceCents != -10 || got.Bag.DeclaredCents == nil || *got.Bag.DeclaredCents != 300 || got.Bag.ReceiverID == nil || *got.Bag.ReceiverID != "receiver" || len(got.Bag.History) != 3 {
		t.Fatal(got)
	}
	for i := 0; i < 2; i++ {
		r := bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
		if !r.Repeated || !reflect.DeepEqual(r.Bag, got.Bag) {
			t.Fatal("receive replay changed original result")
		}
	}
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "290", "291", 1), 409)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/cancel", `{"operation_id":"too-late","reason":"Late"}`, 409)
	replay := bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 200)
	if !replay.Repeated || !reflect.DeepEqual(replay.Bag, first.Bag) || replay.Bag.State != "prepared" {
		t.Fatal("creation receipt rewritten to latest bag", replay)
	}
	if current := bagRead(t, f, f.token, "bag-one", 200); current.State != "received" {
		t.Fatal("replay reopened bag", current)
	}
	if receipt := bagResult(t, f, receiver, "GET", bagsPath+"/operations/receive-one", "", 200); !reflect.DeepEqual(receipt.Bag, got.Bag) {
		t.Fatal(receipt)
	}
	after := bagCounts(t, f)
	if !reflect.DeepEqual(before[:3], after[:3]) || after[3] != 1 || after[4] != 3 || after[5] != before[5]+3 || after[6] != before[6] {
		t.Fatal("custody moved cash or duplicated operations", before, after)
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":700}`, 200)
	v := bagCoverage(t, f, f.token, 200)
	if v.WithdrawnCents != 300 || v.ReceivedDeclaredCents != 300 || v.ReceivedCountedCents != 290 || v.DifferenceCents != -10 || v.DivergentCount != 1 || !v.Complete {
		t.Fatal(v)
	}
	bagCoverage(t, f, receiver, 404)
	if r := closedReport(t, f.httpContractFixture, closedReportPath, 200); r.Cash.ExpectedCents != 700 {
		t.Fatal("received count rebooked cash", r)
	}
}

func TestHTTPBagCoveragePartitionsSangriasAndPreservesCancelRejectReplacement(t *testing.T) {
	f, receiver := bagSetup(t)
	for _, x := range []struct {
		op     string
		amount int
	}{{"w-two", 100}, {"w-three", 150}, {"w-four", 50}, {"w-five", 25}} {
		body, _ := json.Marshal(map[string]any{"session_id": "turn-one", "operation_id": x.op, "kind": "withdrawal", "amount_cents": x.amount, "reason": "Custody"})
		cashRequest(t, f.httpContractFixture, "movements", string(body), 200)
	}
	create := func(id, op string) {
		body, _ := json.Marshal(cashcustody.CreateInput{OperationID: "prepare-" + id, BagID: id, WithdrawalOperationID: op, SealID: "seal-" + id})
		bagResult(t, f, f.token, "POST", bagsPath, string(body), 201)
	}
	act := func(id, action, token, op string) {
		body := `{"operation_id":"` + op + `"}`
		if action == "reject" || action == "cancel" {
			body = `{"operation_id":"` + op + `","reason":"Conferencia; lacre incorreto"}`
		}
		bagResult(t, f, token, "POST", bagsPath+"/"+id+"/"+action, body, 200)
	}
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	act("bag-one", "dispatch", f.token, "send-one")
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
	create("bag-two", "w-two")
	create("bag-three", "w-three")
	act("bag-three", "dispatch", f.token, "send-three")
	create("bag-four", "w-four")
	act("bag-four", "cancel", f.token, "cancel-four")
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":375}`, 200)
	v := bagCoverage(t, f, f.token, 200)
	if v.WithdrawalCount != 5 || v.WithdrawnCents != 625 || v.UnassignedCents != 75 || v.PreparedCents != 100 || v.InTransitCents != 150 || v.ReceivedDeclaredCents != 300 || v.CancelledBags != 1 {
		t.Fatal(v)
	}
	create("bag-replacement", "w-four")
	act("bag-replacement", "dispatch", f.token, "send-replace")
	act("bag-replacement", "reject", receiver, "reject-replace")
	if b := bagRead(t, f, receiver, "bag-replacement", 200); b.DeclaredCents != nil || b.State != "rejected" {
		t.Fatal("reject disclosed declaration", b)
	}
	create("bag-final", "w-four")
	v = bagCoverage(t, f, f.token, 200)
	if v.UnassignedCents != 25 || v.PreparedCents != 150 || v.RejectedBags != 1 || v.CancelledBags != 1 || v.WithdrawnCents != 625 {
		t.Fatal(v)
	}
	body := strings.Replace(strings.Replace(createBagBody, "bag-one", "duplicate", 1), "prepare-one", "duplicate-op", 1)
	body = strings.Replace(body, "seal-one", "duplicate-seal", 1)
	bagResult(t, f, f.token, "POST", bagsPath, body, 409)
	if b := bagRead(t, f, f.token, "bag-four", 200); b.State != "cancelled" || len(b.History) != 2 {
		t.Fatal("cancel history erased", b)
	}
	status, data := request(t, f.app, "GET", bagsPath+"?state=prepared&session_id=turn-one&limit=1&offset=0", "", receiver)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var page cashcustody.Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatal(err)
	}
	domainSchemaFile(t, "cash-bags.openapi.json", "BagPage", data)
	if page.TotalCount != 2 || len(page.Items) != 1 || !page.HasMore || page.Items[0].DeclaredCents != nil {
		t.Fatal(page)
	}
	status, data = request(t, f.app, "GET", bagsPath+"?state=prepared&limit=1&offset=1", "", receiver)
	if status != 200 {
		t.Fatal(status)
	}
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatal(err)
	}
	if page.HasMore || len(page.Items) != 1 || page.Items[0].DeclaredCents != nil {
		t.Fatal(page)
	}
}

func TestHTTPBagSourceAuthorizationPolicyAndStrictInputs(t *testing.T) {
	f, receiver := bagSetup(t)
	for _, body := range []string{`{}`, strings.Replace(createBagBody, `"seal-one"`, `null`, 1), strings.Replace(createBagBody, `"seal_id"`, `"amount_cents"`, 1), strings.Replace(createBagBody, `"operation_id":"prepare-one"`, `"operation_id":"x","operation_id":"prepare-one"`, 1)} {
		bagResult(t, f, f.token, "POST", bagsPath, body, 400)
	}
	for _, bad := range []string{"bad/id", "../bag", "bad?id", "bad%id", "bad\\id", "bad id", "=formula"} {
		bagResult(t, f, f.token, "POST", bagsPath, strings.Replace(createBagBody, "bag-one", bad, 1), 400)
		bagResult(t, f, f.token, "POST", bagsPath, strings.Replace(createBagBody, "prepare-one", bad, 1), 400)
	}
	bagResult(t, f, receiver, "POST", bagsPath, createBagBody, 404)
	bagResult(t, f, f.token, "POST", bagsPath, strings.Replace(createBagBody, "withdraw-one", "missing", 1), 404)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	bagResult(t, f, f.token, "POST", bagsPath, strings.Replace(createBagBody, "withdraw-one", "supply-one", 1), 409)
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	for _, q := range []string{"?limit=0", "?limit=101", "?offset=-1", "?offset=1000000001", "?limit=1&limit=2", "?state=unknown", "?actor_id=other", "?session_id="} {
		if status, _ := request(t, f.app, "GET", bagsPath+q, "", f.token); status != 400 {
			t.Fatal(q, status)
		}
	}
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", "{\"operation_id\":\"x\",\"counted_cents\":1}", 400)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	for _, value := range []string{"-1", "1.5", "9007199254740992", "null"} {
		bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "290", value, 1), 400)
	}
	policyPerson(t, f.db, f.owner, "ordinary-cashier", "cashier")
	cashier := loginToken(t, f.app, "ordinary-cashier")
	bagRead(t, f, cashier, "bag-one", 403)
	if status, _ := request(t, f.app, "GET", bagsPath+"/bag-one?tenant_id=other", "", receiver); status != 400 {
		t.Fatal(status)
	}
	if status, _ := request(t, f.app, "GET", bagsPath+"/bag-one", `{}`, receiver); status != 400 {
		t.Fatal(status)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	bagRead(t, f, receiver, "bag-one", 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 403)
	if _, err := f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id='receiver'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	bagRead(t, f, receiver, "bag-one", 401)
}

func TestHTTPBagDisallowsDrawerOperatorAsReceiverWhenWithdrawalHasAnotherActor(t *testing.T) {
	f, manager := bagSetup(t)
	body := strings.Replace(withdrawalBody, "withdraw-one", "manager-withdraw", 1)
	if status, data := request(t, f.app, "POST", "/local/v1/cash/movements", body, manager); status != 200 {
		t.Fatal(status, string(data))
	}
	body = strings.Replace(createBagBody, "withdraw-one", "manager-withdraw", 1)
	bagResult(t, f, f.token, "POST", bagsPath, body, 404)
	bagResult(t, f, manager, "POST", bagsPath, body, 201)
	bagResult(t, f, manager, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 403)
	policyPerson(t, f.db, f.owner, "third-person", "manager")
	third := loginToken(t, f.app, "third-person")
	bagResult(t, f, third, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
}

func TestHTTPBagFailuresRollbackStateAuditOutboxAndPermitSafeRetry(t *testing.T) {
	for _, stage := range []string{"create_ignore", "operation_abort", "operation_ignore", "outbox_abort", "receive_ignore", "receive_outbox"} {
		t.Run(stage, func(t *testing.T) {
			f, receiver := bagSetup(t)
			methodPath := bagsPath
			body := createBagBody
			token := f.token
			want := 500
			if strings.HasPrefix(stage, "receive") {
				bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
				bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
				methodPath = bagsPath + "/bag-one/receive"
				body = receiveBagBody
				token = receiver
			}
			triggers := map[string]string{
				"create_ignore":    `CREATE TRIGGER injected BEFORE INSERT ON cash_bags BEGIN SELECT RAISE(IGNORE); END`,
				"operation_abort":  `CREATE TRIGGER injected BEFORE INSERT ON cash_bag_operations BEGIN SELECT RAISE(ABORT,'history failed'); END`,
				"operation_ignore": `CREATE TRIGGER injected BEFORE INSERT ON cash_bag_operations BEGIN SELECT RAISE(IGNORE); END`,
				"outbox_abort":     `CREATE TRIGGER injected BEFORE INSERT ON cash_bag_outbox WHEN NEW.event_type='cash.bag.create' BEGIN SELECT RAISE(ABORT,'outbox failed'); END`,
				"receive_ignore":   `CREATE TRIGGER injected BEFORE UPDATE ON cash_bags WHEN NEW.state='received' BEGIN SELECT RAISE(IGNORE); END`,
				"receive_outbox":   `CREATE TRIGGER injected BEFORE INSERT ON cash_bag_outbox WHEN NEW.event_type='cash.bag.receive' BEGIN SELECT RAISE(ABORT,'outbox failed'); END`,
			}
			if strings.Contains(stage, "ignore") {
				want = 409
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			before := bagCounts(t, f)
			bagResult(t, f, token, "POST", methodPath, body, want)
			if !reflect.DeepEqual(before, bagCounts(t, f)) {
				t.Fatal("partial custody operation")
			}
			if strings.HasPrefix(stage, "receive") {
				if b := bagRead(t, f, receiver, "bag-one", 200); b.State != "in_transit" || b.CountedCents != nil || len(b.History) != 2 {
					t.Fatal(b)
				}
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			want = 201
			if strings.HasPrefix(stage, "receive") {
				want = 200
			}
			bagResult(t, f, token, "POST", methodPath, body, want)
		})
	}
}

func TestHTTPBagRestartBackupAndConcurrentReceiveDoNotDuplicateCustody(t *testing.T) {
	f, receiver := bagSetup(t)
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
		}()
	}
	wg.Wait()
	want := bagRead(t, f, receiver, "bag-one", 200)
	before := bagCounts(t, f)
	if before[3] != 1 || before[4] != 3 {
		t.Fatal(before)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "bags.tytbak")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
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
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "receiver"}
	if b, err := cashcustody.Read(ctx, db, actor, f.device, "bag-one"); err != nil || !reflect.DeepEqual(b, want) {
		t.Fatal("restored bag", b, err)
	}
	var queued int
	if err = db.QueryRow(`SELECT count(*) FROM cash_bag_outbox`).Scan(&queued); err != nil || queued != 3 {
		t.Fatal("restored custody outbox", queued, err)
	}
	if _, err = db.Exec(`DELETE FROM cash_bag_outbox`); err == nil {
		t.Fatal("restored custody event mutable")
	}
	if _, err = db.Exec(`DELETE FROM cash_bag_operations`); err == nil {
		t.Fatal("restored history editable")
	}
	var seq int
	var name, originalPath string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &originalPath); err != nil {
		t.Fatal(err)
	}
	if err = f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db, err = localdb.Open(ctx, originalPath)
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
	if got := bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200); !got.Repeated || !reflect.DeepEqual(got.Bag, want) {
		t.Fatal("lost response replay after restart", got)
	}
	if !reflect.DeepEqual(before, bagCounts(t, f)) {
		t.Fatal("restart/replay duplicated custody")
	}
}

func TestHTTPBagRejectsChangedSourceAndHistoryErasure(t *testing.T) {
	f, _ := bagSetup(t)
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	for _, q := range []string{`DELETE FROM cash_bags`, `UPDATE cash_bags SET declared_cents=1`, `UPDATE cash_bags SET seal_id='new'`, `UPDATE cash_bags SET state='received',receiver_id='receiver',received_at='now'`, `DELETE FROM cash_bag_operations`, `UPDATE cash_bag_operations SET reason='changed'`, `DELETE FROM cash_bag_outbox`, `UPDATE cash_bag_outbox SET payload_json='{}'`} {
		if _, err := f.db.Exec(q); err == nil {
			t.Fatal("historical mutation accepted", q)
		}
	}
	if _, err := f.db.Exec(`UPDATE cash_adjustments SET amount_cents=1 WHERE kind='withdrawal'`); err != nil {
		t.Fatal(err)
	}
	bagRead(t, f, f.token, "bag-one", 409)
	if _, err := cashcustody.Read(context.Background(), f.db, journalActor(f), f.device, "bag-one"); !errors.Is(err, cashcustody.ErrInconsistent) {
		t.Fatal(err)
	}
}
