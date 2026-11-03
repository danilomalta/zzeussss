package localapi

import (
	"context"
	"crypto/ed25519"
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
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

const cartsPath = "/local/v1/pos/carts"

func cartInput(f *stockHTTPFixture) sale.CartSaveInput {
	return sale.CartSaveInput{OperationID: "save-one", CartID: "stable-cart", Label: "Carrinho suspenso", Items: salesInput(f).Items}
}
func cartAction(kind string, rev int64) sale.CartActionInput {
	v := sale.CartActionInput{OperationID: kind + "-one", Action: kind, ExpectedRevision: rev, Reason: "Acao solicitada pelo operador"}
	if kind == "claim" {
		v.CashSessionID = "turn-one"
	}
	if kind == "finalize_cash" {
		v.ReceivedCents = 4000
	}
	return v
}
func cartCall(t *testing.T, f *stockHTTPFixture, path string, in any, want int) sale.CartResult {
	t.Helper()
	b, _ := json.Marshal(in)
	code, raw := request(t, f.app, "POST", path, string(b), f.token)
	if code != want {
		t.Fatalf("%s got%d want%d %s", path, code, want, raw)
	}
	var v sale.CartResult
	if want == 200 {
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "suspended-carts.openapi.json", "CartResult", raw)
	}
	return v
}
func cartGet(t *testing.T, f *stockHTTPFixture, path string, want int) []byte {
	t.Helper()
	code, b := request(t, f.app, "GET", path, "", f.token)
	if code != want {
		t.Fatal(code, want, string(b))
	}
	return b
}
func cartCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	out := isolationCounts(t, f.db)
	for _, name := range []string{"pos_carts", "pos_cart_operations", "pos_cart_events"} {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + name).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	return out
}
func TestHTTPCartSaveClaimCashSaleAndReplay(t *testing.T) {
	f := salesSetup(t)
	in := cartInput(f)
	before := isolationCounts(t, f.db)
	first := cartCall(t, f, cartsPath, in, 200)
	if first.Cart.State != "suspended" || first.Cart.TotalCents != 3198 || first.Cart.Revision != 1 || !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
		t.Fatal(first)
	}
	again := cartCall(t, f, cartsPath, in, 200)
	if !again.Repeated || again.Cart.SaleID != first.Cart.SaleID {
		t.Fatal(again)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	last := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	if last.Cart.State != "completed" || last.Cart.ChangeCents != 802 {
		t.Fatal(last)
	}
	counts := cartCounts(t, f)
	again = cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	if !again.Repeated || !reflect.DeepEqual(counts, cartCounts(t, f)) {
		t.Fatal("duplicate sale")
	}
	receipt := readReceipt(t, f, first.Cart.SaleID)
	if receipt.TotalCents != 3198 || len(receipt.Payments) != 1 || receipt.Payments[0].PaymentID != first.Cart.PaymentID {
		t.Fatal(receipt)
	}
	assertStockBalance(t, f, f.shelf, 3000)
	cartGet(t, f, cartsPath+"/stable-cart", 200)
	cartGet(t, f, "/local/v1/pos/cart-operations/finalize_cash-one", 200)
}
func TestHTTPCartPriceChangeRequiresExplicitResave(t *testing.T) {
	f := salesSetup(t)
	in := cartInput(f)
	cartCall(t, f, cartsPath, in, 200)
	if _, e := f.db.Exec(`UPDATE products SET price_cents=1600 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); e != nil {
		t.Fatal(e)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 409)
	in.OperationID = "save-two"
	in.ExpectedRevision = 1
	r := cartCall(t, f, cartsPath, in, 200)
	if r.Cart.TotalCents != 3200 {
		t.Fatal(r)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 2), 200)
	if _, e := f.db.Exec(`UPDATE products SET price_cents=1601 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); e != nil {
		t.Fatal(e)
	}
	before := cartCounts(t, f)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 3), 409)
	if !reflect.DeepEqual(before, cartCounts(t, f)) {
		t.Fatal("stale prices sold")
	}
}
func TestHTTPCartRollbackIncludesSaleJournalStockOutbox(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	before := cartCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER cart_fault BEFORE INSERT ON pos_cart_events WHEN NEW.operation_id='finalize_cash-one' BEGIN SELECT RAISE(ABORT,'injected'); END`); e != nil {
		t.Fatal(e)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 500)
	if !reflect.DeepEqual(before, cartCounts(t, f)) {
		t.Fatal("partial sale effects")
	}
	if _, e := f.db.Exec(`DROP TRIGGER cart_fault`); e != nil {
		t.Fatal(e)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
}
func TestHTTPCartConcurrentClaimsAndSaleIDs(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, id := range []string{"claim-a", "claim-b"} {
		in := cartAction("claim", 1)
		in.OperationID = id
		b, _ := json.Marshal(in)
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", cartsPath+"/stable-cart/actions", body, f.token)
			codes <- code
		}(string(b))
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for n := range codes {
		if n == 200 {
			ok++
		} else if n == 409 {
			conflict++
		} else {
			t.Fatal(n)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	salesCounts(t, f, 1)
}
func TestHTTPCartReleaseCancelHistoryAndStrictInputs(t *testing.T) {
	f := salesSetup(t)
	in := cartInput(f)
	cartCall(t, f, cartsPath, in, 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("cancel", 2), 409)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("release", 2), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("cancel", 3), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 4), 409)
	b := cartGet(t, f, cartsPath+"/stable-cart/history?limit=2&offset=2", 200)
	var h sale.CartHistory
	if json.Unmarshal(b, &h) != nil || h.Total != 4 || len(h.Items) != 2 || h.HasMore {
		t.Fatal(string(b))
	}
	b = cartGet(t, f, cartsPath+"?state=cancelled&limit=1", 200)
	var page sale.CartPage
	if json.Unmarshal(b, &page) != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatal(string(b))
	}
	for _, q := range []string{"?state=", "?limit=0", "?limit=2&limit=3", "?tenant_id=other"} {
		cartGet(t, f, cartsPath+q, 400)
	}
	raw, _ := json.Marshal(in)
	for _, body := range []string{strings.Replace(string(raw), `"quantity_milli":2000`, `"quantity_milli":null`, 1), strings.TrimSuffix(string(raw), "}") + `,"price_cents":1}`, strings.Replace(string(raw), `"expected_revision":0`, `"expected_revision":0,"expected_revision":1`, 1)} {
		code, _ := request(t, f.app, "POST", cartsPath, body, f.token)
		if code != 400 {
			t.Fatal(code, body)
		}
	}
}
func TestHTTPCartBackupRestoreRecoversOriginalAndScopedReads(t *testing.T) {
	f := salesSetup(t)
	first := cartCall(t, f, cartsPath, cartInput(f), 200)
	key := make([]byte, 32)
	rand.Read(key)
	dir := t.TempDir()
	archive := filepath.Join(dir, "cart.tytbak")
	restored := filepath.Join(dir, "restored.sqlite")
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	got, e := sale.CartOperation(context.Background(), db, a, f.device, "save-one")
	if e != nil || !reflect.DeepEqual(first, got) {
		t.Fatal(got, e)
	}
	a.IdentityID = "missing"
	if _, e = sale.GetCart(context.Background(), db, a, f.device, "stable-cart"); e == nil {
		t.Fatal("foreign identity read")
	}
}

func TestHTTPCartInsufficientStockAndFractionalRounding(t *testing.T) {
	f := salesSetup(t)
	if _, e := f.db.Exec(`UPDATE products SET unit='kg' WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); e != nil {
		t.Fatal(e)
	}
	in := cartInput(f)
	in.Items[0].QuantityMilli = 125
	first := cartCall(t, f, cartsPath, in, 200)
	if first.Cart.TotalCents != 200 {
		t.Fatal(first)
	}
	stockRequest(t, f, stock.Input{OperationID: "drain-shelf", Kind: "loss", ProductID: f.product, FromLocationID: f.shelf, QuantityMilli: 4900, Reason: "Movimentacao legitima concorrente"}, 201)
	before := cartCounts(t, f)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 409)
	if !reflect.DeepEqual(before, cartCounts(t, f)) {
		t.Fatal("partial claim")
	}
	stockRequest(t, f, stock.Input{OperationID: "refill", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 25, Reason: "Conferencia"}, 201)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	r := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	if r.Cart.ChangeCents != 3800 {
		t.Fatal(r)
	}
	assertStockBalance(t, f, f.shelf, 0)
}
func TestHTTPCartClosedShiftReleaseAndNewShift(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 409)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("release", 2), 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-two","opening_cents":0}`, 201)
	claim := cartAction("claim", 3)
	claim.OperationID = "claim-two"
	claim.CashSessionID = "turn-two"
	cartCall(t, f, cartsPath+"/stable-cart/actions", claim, 200)
	r := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 4), 200)
	if r.Cart.CashSessionID != "turn-two" {
		t.Fatal(r)
	}
}
func TestHTTPCartOriginalActorReadAndChangedIdempotency(t *testing.T) {
	f := salesSetup(t)
	in := cartInput(f)
	cartCall(t, f, cartsPath, in, 200)
	in.Label = "Outro texto"
	cartCall(t, f, cartsPath, in, 409)
	in.CartID = "cart-two"
	cartCall(t, f, cartsPath, in, 409)
	policyPerson(t, f.db, f.owner, "other-cashier", "cashier")
	token := loginToken(t, f.app, "other-cashier")
	for _, path := range []string{cartsPath + "/stable-cart", "/local/v1/pos/cart-operations/save-one", cartsPath + "/stable-cart/history"} {
		code, _ := request(t, f.app, "GET", path, "", token)
		if code != 404 {
			t.Fatal(code, path)
		}
	}
	code, b := request(t, f.app, "GET", cartsPath, "", token)
	var page sale.CartPage
	if code != 200 || json.Unmarshal(b, &page) != nil || page.Total != 0 {
		t.Fatal(code, string(b))
	}
	in = cartInput(f)
	in.OperationID = "adopt-private-cart"
	raw, _ := json.Marshal(in)
	code, _ = request(t, f.app, "POST", cartsPath, string(raw), token)
	if code != 409 {
		t.Fatal(code)
	}
}
func TestHTTPCartCancellationOfSaleDoesNotReopenCart(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	r := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	code, b := request(t, f.app, "POST", "/local/v1/sales/"+r.Cart.SaleID+"/cancel", cancelBody, f.token)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	cartGet(t, f, cartsPath+"/stable-cart", 200)
	b = cartGet(t, f, "/local/v1/pos/cart-operations/finalize_cash-one", 200)
	var original sale.CartResult
	if json.Unmarshal(b, &original) != nil || !reflect.DeepEqual(original, r) {
		t.Fatal(string(b))
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 3), 409)
	assertStockBalance(t, f, f.shelf, 5000)
}
func TestHTTPCartSaveRollbackAndImmutableHistory(t *testing.T) {
	f := salesSetup(t)
	before := cartCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER cart_save_fault BEFORE INSERT ON pos_cart_events BEGIN SELECT RAISE(ABORT,'injected'); END`); e != nil {
		t.Fatal(e)
	}
	cartCall(t, f, cartsPath, cartInput(f), 500)
	if !reflect.DeepEqual(before, cartCounts(t, f)) {
		t.Fatal("orphaned cart")
	}
	if _, e := f.db.Exec(`DROP TRIGGER cart_save_fault`); e != nil {
		t.Fatal(e)
	}
	cartCall(t, f, cartsPath, cartInput(f), 200)
	for _, q := range []string{`DELETE FROM pos_carts`, `UPDATE pos_cart_operations SET action='forged'`, `DELETE FROM pos_cart_events`} {
		if _, e := f.db.Exec(q); e == nil {
			t.Fatal("mutable", q)
		}
	}
}
func TestHTTPCartReadSurvivesLicenseLossButWritesDoNot(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	cartGet(t, f, cartsPath+"/stable-cart", 200)
	cartGet(t, f, "/local/v1/pos/cart-operations/save-one", 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 403)
}
func TestHTTPCartRestartThenFinalizeExactlyOnce(t *testing.T) {
	f := salesSetup(t)
	first := cartCall(t, f, cartsPath, cartInput(f), 200)
	var n int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	var e error
	f.db, e = localdb.Open(context.Background(), path)
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
	b := cartGet(t, f, "/local/v1/pos/cart-operations/save-one", 200)
	var original sale.CartResult
	if json.Unmarshal(b, &original) != nil || !reflect.DeepEqual(first, original) {
		t.Fatal(string(b))
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	again := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	if !again.Repeated {
		t.Fatal(again)
	}
	salesCounts(t, f, 1)
}

type cartDropTransport struct {
	base      *http.Transport
	discarded atomic.Int32
}

func (d *cartDropTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r, e := d.base.RoundTrip(req)
	if e == nil && req.Method == "POST" && r.StatusCode == 200 {
		d.discarded.Add(1)
		r.Body.Close()
		return nil, errors.New("test-only lost committed response")
	}
	return r, e
}
func TestHTTPCartLoopbackLostFinalReplyRecoversWithoutSecondPOST(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
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
	drop := &cartDropTransport{base: tr}
	client := &http.Client{Transport: drop, Timeout: 5 * time.Second}
	origin := "http://" + listener.Addr().String()
	raw, _ := json.Marshal(cartAction("finalize_cash", 2))
	req, e := http.NewRequest("POST", origin+cartsPath+"/stable-cart/actions", strings.NewReader(string(raw)))
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
		t.Fatal("no commit")
	}
	before := cartCounts(t, f)
	req, e = http.NewRequest("GET", origin+"/local/v1/pos/cart-operations/finalize_cash-one", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	r, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var result sale.CartResult
	if r.StatusCode != 200 || json.NewDecoder(r.Body).Decode(&result) != nil || result.Cart.State != "completed" || !reflect.DeepEqual(before, cartCounts(t, f)) {
		t.Fatal(result)
	}
}
