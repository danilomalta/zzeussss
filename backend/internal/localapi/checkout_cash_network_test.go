package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

func TestHTTPCheckoutCashLoopbackLostResultRestartAndClose(t *testing.T) {
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
	stockRequest(t, f, stockEntry(f, "network-entry", 10000), 201)
	stockRequest(t, f, stock.Input{OperationID: "network-transfer", Kind: "transfer", ProductID: f.product, FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "disposable setup"}, 201)
	start := func(app *fiber.App) (string, func()) {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- app.Listener(ln) }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				if err := app.ShutdownWithTimeout(3 * time.Second); err != nil {
					t.Error("test server shutdown failed")
				}
				if err := <-done; err != nil {
					t.Error("test listener failed")
				}
			})
		}
		return "http://" + ln.Addr().String(), stop
	}
	endpoint, stop := start(f.app)
	defer stop()
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	do := func(method, path, body string, want int, discard bool) []byte {
		t.Helper()
		req, err := http.NewRequest(method, endpoint+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+f.token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("loopback request failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("loopback %s: status %d want %d", path, resp.StatusCode, want)
		}
		if discard {
			return nil
		} // Do not read/decode the committed command's result.
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal("loopback result unavailable")
		}
		return raw
	}
	do("POST", "/local/v1/cash/open", cashOpenBody, 201, false)
	in := sale.PlanInput{OperationID: "prepare-cash", CheckoutID: cashPlanID, CashSessionID: "turn-one", ExpectedTotalCents: 3198, Items: salesInput(f).Items, Parts: []sale.PlanPartInput{{PartID: "cash-one", Method: "cash", AmountCents: 1000}, {PartID: "cash-two", Method: "cash", AmountCents: 2198}}}
	do("POST", planPath, planJSON(t, in), 201, false)
	do("POST", finalizeCashPath, finalizeCashBody, 201, true)
	stop()
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f.db = db
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, stopAgain := start(f.app)
	defer stopAgain()
	var plan sale.Plan
	if json.Unmarshal(do("GET", planPath+"/"+cashPlanID, "", 200, false), &plan) != nil || plan.Completion == nil || plan.Completion.ChangeCents != 802 || plan.Completion.ReceivedCents != 4000 {
		t.Fatal("restart lost tender/change")
	}
	if json.Unmarshal(do("POST", finalizeCashPath, finalizeCashBody, 200, false), &plan) != nil || !plan.Repeated {
		t.Fatal("retry duplicated or failed")
	}
	cashFinalCounts(t, f, 1)
	do("POST", "/local/v1/cash/close", `{"session_id":"turn-one","operation_id":"close-network","declared_cents":4198}`, 200, false)
	var expected int64
	if err := db.QueryRow(`SELECT SUM(cash_delta_cents) FROM cash_journal_entries`).Scan(&expected); err != nil || expected != 4198 {
		t.Fatal("journal does not reconcile", expected)
	}
}
