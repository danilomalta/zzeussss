package localapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/supplycycle"
)

// Two independent SQLite authorities, actual installed React screens and TCP.
// Only the initial approved purchase, identities and licences are fixtures.
// Envelopes travel explicitly through the existing public import/export forms.
func TestTradeInstalledScreenAuthenticatedPartialDecisionRecovery(t *testing.T) {
	runTradeInstalledPilot(t, false)
}

func TestTradeInstalledScreenAuthenticatedReceivingFinance(t *testing.T) {
	runTradeInstalledPilot(t, true)
}

func runTradeInstalledPilot(t *testing.T, receiving bool) {
	t.Helper()
	desktop, screen := os.Getenv("TITAN_INSTALLED_DESKTOP"), os.Getenv("TITAN_INSTALLED_SCREEN")
	if desktop == "" && screen == "" {
		t.Skip("opt-in installed application acceptance; supply TITAN_INSTALLED_DESKTOP and TITAN_INSTALLED_SCREEN")
	}
	for _, p := range []string{desktop, screen} {
		if !filepath.IsAbs(p) {
			t.Fatal("absolute installed acceptance paths required")
		}
		if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
			t.Fatal("installed acceptance input unavailable")
		}
	}
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	if b.f.owner.TenantID == s.f.owner.TenantID || bytes.Equal(b.key, s.key) {
		t.Fatal("companies must have independent authority")
	}
	var buyerIdentity, supplierIdentity map[string]any
	b.p.call("GET", "/local/v1/company-trade/identity", nil, 200, &buyerIdentity)
	s.p.call("GET", "/local/v1/company-trade/identity", nil, 200, &supplierIdentity)
	var drop atomic.Bool
	fault := relayFaultProxy(t, b.p.service.url, &drop, true)
	target, err := url.Parse(b.p.service.url)
	if err != nil {
		t.Fatal("invalid disposable endpoint")
	}
	normal := httputil.NewSingleHostReverseProxy(target)
	transport := &http.Transport{Proxy: nil}
	normal.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	var decisionPosts atomic.Int32
	gate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == cycleOrder+"/company-trade/decision" {
			decisionPosts.Add(1)
			proxy := httputil.NewSingleHostReverseProxy(mustTradeURL(t, fault.URL))
			proxy.Transport = transport
			proxy.ServeHTTP(w, r)
			return
		}
		normal.ServeHTTP(w, r)
	}))
	t.Cleanup(gate.Close)
	buyerRoot, supplierRoot := privatePilotDir(t), privatePilotDir(t)
	run := func(party *tradeHTTPFixture, root, endpoint, phase string, resume bool, values map[string]any) {
		t.Helper()
		payload := map[string]any{"binary": desktop, "root": root, "url": endpoint, "owner": party.f.owner.OwnerID, "password": "senha-forte-de-teste-1", "business": "trade", "tradePhase": phase, "resume": resume}
		for key, value := range values {
			payload[key] = value
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal("invalid disposable controller input")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", screen)
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "xvfb-run", "-a", "node", screen)
		}
		cmd.Env = installedEnvironment()
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.CombinedOutput()
		passed := false
		scan := bufio.NewScanner(bytes.NewReader(out))
		for scan.Scan() {
			var v map[string]any
			if json.Unmarshal(scan.Bytes(), &v) != nil {
				continue
			}
			if label, ok := v["acceptancePhase"].(string); ok {
				t.Log("Installed phase:", label)
			}
			if label, ok := v["acceptanceFailure"].(string); ok {
				t.Log("Installed failure:", label, "detail:", v["detail"])
			}
			if v["businessPhase"] == "trade" && v["passed"] == true {
				passed = true
			}
		}
		if err != nil || !passed {
			t.Fatalf("installed trade %s failed; private input/output omitted (%v)", phase, err)
		}
	}
	run(b, buyerRoot, gate.URL, "export", false, map[string]any{"peer": "vendor", "partyRole": "supplier", "remoteIdentity": supplierIdentity})
	var offers supplycycle.TradeMessagePage
	b.p.call("GET", "/local/v1/company-trade/messages?limit=20&offset=0&direction=sent&kind=order", nil, 200, &offers)
	if len(offers.Items) != 1 {
		t.Fatal("exactly one signed offer required")
	}
	offer := offers.Items[0]
	promised := time.Now().Add(time.Hour).Format("2006-01-02T15:04:05")
	run(s, supplierRoot, s.p.service.url, "decide", false, map[string]any{"peer": "customer", "partyRole": "customer", "remoteIdentity": buyerIdentity, "envelope": offer.Envelope, "messageID": offer.Message.ID, "promised": promised})
	var decisions supplycycle.TradeMessagePage
	s.p.call("GET", "/local/v1/company-trade/messages?limit=20&offset=0&direction=sent&kind=decision", nil, 200, &decisions)
	if len(decisions.Items) != 1 || decisions.Items[0].Message.AcceptedMilli != 3000 {
		t.Fatal("supplier decision must preserve explicit partial quantity")
	}
	drop.Store(true)
	run(b, buyerRoot, gate.URL, "apply", true, map[string]any{"peer": "vendor", "envelope": decisions.Items[0].Envelope})
	var before supplycycle.Cycle
	b.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, &before)
	if before.Decision == nil || !before.Decision.ExternalVerified || before.Decision.EvidenceMode != "authenticated_company" || before.Decision.AcceptedMilli != 3000 {
		t.Fatal("lost response must follow authenticated commit")
	}
	drop.Store(false)
	run(b, buyerRoot, gate.URL, "recover", true, nil)
	var after supplycycle.Cycle
	b.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, &after)
	if decisionPosts.Load() != 1 || after.Decision == nil || !reflect.DeepEqual(before.Decision, after.Decision) {
		t.Fatal("recovery duplicated or replaced original decision")
	}
	if receiving {
		count := func(table string) int64 {
			t.Helper()
			var n int64
			if err := b.f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
				t.Fatal("disposable observation unavailable")
			}
			return n
		}
		stockBefore, cashBefore, payableBefore := count("stock_movements"), count("cash_movements"), count("supplier_payables")
		stockTotal := func() int64 {
			t.Helper()
			var n int64
			if err := b.f.db.QueryRow("SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements").Scan(&n); err != nil {
				t.Fatal("disposable stock total unavailable")
			}
			return n
		}
		quantityBefore := stockTotal()
		run(b, buyerRoot, gate.URL, "receive", true, nil)
		var received supplycycle.Cycle
		b.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, &received)
		// Received deliveries commit only accepted units, preserving refused capacity.
		if received.Decision == nil || !reflect.DeepEqual(received.Decision, after.Decision) || !received.Decision.ExternalVerified || received.EffectiveAcceptedMilli != 2500 || received.AllocatedMilli != 2500 || received.Total != 1 || len(received.Deliveries) != 1 || received.Deliveries[0].PlannedMilli != 3000 || received.Deliveries[0].State != "received" {
			t.Fatal("receipt lost authenticated source or partial quantities")
		}
		var payable struct {
			TotalCents int64  `json:"total_cents"`
			State      string `json:"state"`
		}
		b.p.call("GET", "/local/v1/finance/payables/authenticated-payable", nil, 200, &payable)
		if payable.TotalCents != 3150 || payable.State != "pending" {
			t.Fatal("obligation must keep explicit amount and await independent review")
		}
		if count("stock_movements") != stockBefore+1 || stockTotal() != quantityBefore+2500 || count("cash_movements") != cashBefore || count("supplier_payables") != payableBefore+1 || decisionPosts.Load() != 1 {
			t.Fatal("receiving duplicated stock, obligation or supplier decision, or booked money")
		}
		t.Log("PASS: authenticated supplier -> installed receiving 3000 delivered/2500 accepted/500 refused -> pending payable3150; one stock entry/obligation and no cash movement")
	}
	original := b.p.token
	b.p.token = s.p.token
	b.p.call("GET", "/local/v1/company-trade/identity", nil, 401, nil)
	b.p.token = original
	t.Log("PASS: installed screens, two independent companies, signed partial supplier decision, response loss and explicit GET recovery with one POST; no commercial data or fiscal issuance")
}

func mustTradeURL(t *testing.T, value string) *url.URL {
	t.Helper()
	u, err := url.Parse(value)
	if err != nil {
		t.Fatal("invalid disposable relay URL")
	}
	return u
}
