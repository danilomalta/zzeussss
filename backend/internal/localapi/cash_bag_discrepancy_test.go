package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb/entitlementstore"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/identity"
)

const bagReviewPath = bagsPath + "/bag-one/review"

func reviewHTTP(t *testing.T, f *stockHTTPFixture, token, method, body string, want int) cashcustody.ReviewResult {
	t.Helper()
	code, data := request(t, f.app, method, bagReviewPath, body, token)
	if code != want {
		t.Fatalf("review %s: %d want %d: %s", method, code, want, data)
	}
	var out cashcustody.ReviewResult
	if want == 200 {
		var target any = &out
		if method == "GET" {
			target = &out.Review
		}
		schema := "CustodyReviewResult"
		if method == "GET" {
			schema = "CustodyReview"
		}
		domainSchemaFile(t, "cash-bags.openapi.json", schema, data)
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func receivedForReview(t *testing.T) (*stockHTTPFixture, string) {
	t.Helper()
	f, receiver := bagSetup(t)
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
	return f, receiver
}
func reviewBody(op, action string, rev, count int) string {
	return fmt.Sprintf(`{"operation_id":%q,"expected_revision":%d,"action":%q,"counted_cents":%d,"reason":"Private investigation evidence"}`, op, rev, action, count)
}
func TestHTTPBagDiscrepancyInvestigationCompensationRollbackAndRestore(t *testing.T) {
	f, receiver := receivedForReview(t)
	before := bagCounts(t, f)
	original := bagRead(t, f, f.token, "bag-one", 200)
	pending := reviewHTTP(t, f, receiver, "GET", "", 200).Review
	if pending.State != "pending" || pending.DifferenceCents != -10 {
		t.Fatal(pending)
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		id := "review-" + role
		policyPerson(t, f.db, f.owner, id, role)
		token := loginToken(t, f.app, id)
		reviewHTTP(t, f, token, "POST", reviewBody("deny-"+role, "investigate", 0, 290), 403)
	}
	body := reviewBody("investigate-one", "investigate", 0, 290)
	first := reviewHTTP(t, f, f.token, "POST", body, 200)
	if first.Review.State != "investigating" || first.Review.Revision != 1 {
		t.Fatal(first)
	}
	if _, err := f.db.Exec(`CREATE TRIGGER review_audit_failure BEFORE INSERT ON cash_bag_review_events BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	recount := reviewBody("recount-one", "recount", 1, 300)
	reviewHTTP(t, f, f.token, "POST", recount, 500)
	if got := reviewHTTP(t, f, f.token, "GET", "", 200).Review; !reflect.DeepEqual(got, first.Review) {
		t.Fatal("audit failure partially committed")
	}
	if _, err := f.db.Exec(`DROP TRIGGER review_audit_failure`); err != nil {
		t.Fatal(err)
	}
	counted := reviewHTTP(t, f, f.token, "POST", recount, 200)
	if counted.Review.History[1].DeltaCents != 10 || counted.Review.DifferenceCents != 0 || counted.Review.OriginalCountedCents != 290 {
		t.Fatal("compensation changed original", counted)
	}
	done := reviewHTTP(t, f, f.token, "POST", reviewBody("resolve-one", "resolve", 2, 300), 200)
	if done.Review.State != "resolved" || done.Review.Revision != 3 {
		t.Fatal(done)
	}
	replay := reviewHTTP(t, f, f.token, "POST", body, 200)
	if !replay.Repeated || !reflect.DeepEqual(replay.Review, first.Review) {
		t.Fatal("replay did not preserve original receipt")
	}
	reviewHTTP(t, f, f.token, "POST", reviewBody("investigate-one", "investigate", 0, 291), 409)
	reviewHTTP(t, f, f.token, "POST", reviewBody("closed", "recount", 3, 290), 409)
	if !reflect.DeepEqual(before, bagCounts(t, f)) || !reflect.DeepEqual(original, bagRead(t, f, f.token, "bag-one", 200)) {
		t.Fatal("investigation changed bag, cash or stock")
	}
	for _, table := range []string{"cash_bag_reviews", "cash_bag_review_events"} {
		var n int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 3 {
			t.Fatal(table, n, err)
		}
		if _, err := f.db.Exec(`DELETE FROM ` + table); err == nil {
			t.Fatal("mutable review history")
		}
	}
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "review.tytbak")
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
	got, err := cashcustody.ReadReview(ctx, db, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}, f.device, "bag-one")
	if err != nil || !reflect.DeepEqual(got, done.Review) {
		t.Fatal("restore lost investigation", err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := request(t, app, "GET", bagReviewPath, "", f.token); status != 401 {
		t.Fatalf("restored old session: %d", status)
	}
	contracts, err := entitlementstore.New(db, verifier, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var input cashcustody.ReviewInput
	if err = json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatal(err)
	}
	receipt, err := cashcustody.WriteReview(ctx, db, contracts, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}, f.device, "bag-one", input)
	if err != nil || !receipt.Repeated || !reflect.DeepEqual(receipt.Review, first.Review) {
		t.Fatal("restored original operation not recoverable", err)
	}

}
func TestHTTPBagDiscrepancyConcurrentRevisionSingleWinner(t *testing.T) {
	f, _ := receivedForReview(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", bagReviewPath, reviewBody(fmt.Sprintf("race-%d", i), "investigate", 0, 290), f.token)
			statuses <- code
		}(i)
	}
	wg.Wait()
	close(statuses)
	wins := 0
	for code := range statuses {
		if code == 200 {
			wins++
		} else if code != 409 {
			t.Fatalf("unexpected contention result %d", code)
		}
	}
	if wins != 1 {
		t.Fatalf("winners %d", wins)
	}
	v := reviewHTTP(t, f, f.token, "GET", "", 200).Review
	if v.Revision != 1 || v.DifferenceCents != -10 {
		t.Fatal(v)
	}
}

func TestHTTPBagDiscrepancyFrontendRealHTTPRecovery(t *testing.T) {
	f, _ := receivedForReview(t)
	before := bagCounts(t, f)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = f.app.Listener(listener) }()
	defer f.app.Shutdown()
	defer listener.Close()
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "../../../frontend-web/tests/cashBagInvestigation.bridge.mjs")
	session, _ := json.Marshal(map[string]any{"tenant_id": f.owner.TenantID, "store_id": f.owner.StoreID, "identity_id": f.owner.OwnerID, "device_id": f.device.DeviceID, "expires_unix": time.Now().Add(time.Minute).Unix()})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Env = append(os.Environ(), "TITAN_BRIDGE_URL=http://"+listener.Addr().String(), "TITAN_BRIDGE_TOKEN="+f.token, "TITAN_BRIDGE_SESSION="+string(session))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("custody frontend bridge: %v %s", err, output)
	}
	if !reflect.DeepEqual(before, bagCounts(t, f)) {
		t.Fatal("frontend investigation moved money")
	}
}
func TestHTTPBagDiscrepancyScopeStrictInputsAndNoImplicitLoss(t *testing.T) {
	f, _ := receivedForReview(t)
	for _, body := range []string{`{}`, reviewBody("bad", "investigate", 0, 290)[:10], `{"operation_id":"bad","operation_id":"other","expected_revision":0,"action":"investigate","counted_cents":290,"reason":"test"}`, reviewBody("bad", "recount", 0, -1)} {
		reviewHTTP(t, f, f.token, "POST", body, 400)
	}
	reviewHTTP(t, f, f.token, "POST", reviewBody("premature", "resolve", 0, 290), 409)
	_, err := cashcustody.ReadReview(context.Background(), f.db, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: "other-tenant", StoreID: f.owner.StoreID}, f.device, "bag-one")
	if err == nil {
		t.Fatal("tenant crossing permitted")
	}
	_, err = cashcustody.ReadReview(context.Background(), f.db, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: "other-store"}, f.device, "bag-one")
	if err == nil {
		t.Fatal("store crossing permitted")
	}
	device := f.device
	device.DeviceID = "other-device"
	_, err = cashcustody.ReadReview(context.Background(), f.db, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}, device, "bag-one")
	if err == nil {
		t.Fatal("device crossing permitted")
	}
	reviewHTTP(t, f, f.token, "POST", reviewBody("start", "investigate", 0, 290), 200)
	done := reviewHTTP(t, f, f.token, "POST", reviewBody("acknowledge", "resolve", 1, 290), 200)
	if done.Review.DifferenceCents != -10 || done.Review.CountedCents != 290 || done.Review.History[1].DeltaCents != 0 {
		t.Fatal("resolution fabricated money", done)
	}
}
