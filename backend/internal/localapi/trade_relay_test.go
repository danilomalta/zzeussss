package localapi

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/localdb/supplycycle"
	"titansystem-backend/internal/traderelay"
)

func relaySetup(t *testing.T) (*tradeHTTPFixture, *tradeHTTPFixture, traderelay.Job, string) {
	t.Helper()
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	tradeBind(t, b, s)
	var offer supplycycle.TradeResult
	b.p.call("POST", cycleOrder+"/company-trade/export", supplycycle.TradeExportInput{OperationID: "relay-export", PeerID: "vendor", Reason: "Envio autorizado do pedido aprovado"}, 200, &offer)
	endpoint := func(f *tradeHTTPFixture) traderelay.Endpoint {
		return traderelay.Endpoint{URL: f.p.service.url + "/local/v1", Party: tradePartyOf(f.f), ActorID: f.f.owner.OwnerID, Fingerprint: supplycycle.TradeFingerprint(f.key.Public().(ed25519.PublicKey))}
	}
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	return b, s, traderelay.Job{Version: 1, Source: endpoint(b), Target: endpoint(s), MessageID: offer.Message.ID, PeerID: "customer", OperationID: "relay-import"}, filepath.Join(dir, "attempt.json")
}
func TestTradeRelayRealHTTPBothDirectionsAndRecovery(t *testing.T) {
	b, s, j, state := relaySetup(t)
	ctx := context.Background()
	r, e := traderelay.Run(ctx, j, b.p.token, s.p.token, state, "send")
	if e != nil || r.Status != "confirmed" || r.Posts != 1 {
		t.Fatalf("send: %+v %v", r, e)
	}
	r, e = traderelay.Run(ctx, j, b.p.token, s.p.token, state, "recover")
	if e != nil || r.Posts != 0 || r.Status != "confirmed" {
		t.Fatalf("recover: %+v %v", r, e)
	}
	var decision supplycycle.TradeResult
	s.p.call("POST", "/local/v1/company-trade/orders/"+j.MessageID+"/decision", supplycycle.TradeDecisionInput{OperationID: "relay-human-decision", Decision: "confirmed", AcceptedMilli: 3000, PromisedAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339), Reason: "Fornecedor confirmou explicitamente"}, 200, &decision)
	j.Source, j.Target = j.Target, j.Source
	j.PeerID = "vendor"
	j.MessageID = decision.Message.ID
	j.OperationID = "relay-apply-decision"
	state = filepath.Join(filepath.Dir(state), "decision.json")
	r, e = traderelay.Run(ctx, j, s.p.token, b.p.token, state, "send")
	if e != nil || r.Status != "confirmed" || r.Posts != 1 {
		t.Fatalf("decision: %+v %v", r, e)
	}
	var receipt supplycycle.TradeResult
	b.p.call("GET", "/local/v1/company-trade/operations/"+j.OperationID, nil, 200, &receipt)
	if receipt.Cycle == nil || receipt.Cycle.Decision == nil || !receipt.Cycle.Decision.ExternalVerified {
		t.Fatal("decision not authenticated")
	}
	body, _ := os.ReadFile(state)
	if strings.Contains(string(body), s.p.token) || strings.Contains(string(body), b.p.token) || strings.Contains(string(body), "envelope\"") || strings.Contains(string(body), "reason") {
		t.Fatal("state leaked credentials or document")
	}
}

// This proxy forwards to the real Go service. The ONLY injected behavior is a
// dropped HTTP response before/after the real transaction, not a business mock.
func relayFaultProxy(t *testing.T, destination string, drop *atomic.Bool, after bool) *httptest.Server {
	t.Helper()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fault := r.Method == "POST" && drop.Load()
		if fault && !after {
			c, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				c.Close()
			}
			return
		}
		req, e := http.NewRequestWithContext(r.Context(), r.Method, destination+r.URL.RequestURI(), r.Body)
		if e != nil {
			t.Error(e)
			w.WriteHeader(500)
			return
		}
		req.Header = r.Header.Clone()
		res, e := transport.RoundTrip(req)
		if e != nil {
			w.WriteHeader(502)
			return
		}
		defer res.Body.Close()
		body, e := io.ReadAll(res.Body)
		if e != nil {
			w.WriteHeader(502)
			return
		}
		if fault {
			c, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				c.Close()
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(body)
	}))
	t.Cleanup(proxy.Close)
	return proxy
}
func TestTradeRelayLostResponseRecoversOriginalWithoutPost(t *testing.T) {
	b, s, j, state := relaySetup(t)
	var drop atomic.Bool
	drop.Store(true)
	proxy := relayFaultProxy(t, s.p.service.url, &drop, true)
	j.Target.URL = proxy.URL + "/local/v1"
	r, e := traderelay.Run(context.Background(), j, b.p.token, s.p.token, state, "send")
	if !errors.Is(e, traderelay.ErrUncertain) || r.Posts != 1 {
		t.Fatalf("lost response: %+v %v", r, e)
	}
	before, _ := os.ReadFile(state)
	r, e = traderelay.Run(context.Background(), j, b.p.token, s.p.token, state, "send")
	if !errors.Is(e, traderelay.ErrUncertain) || r.Posts != 0 {
		t.Fatal("send auto-retried")
	}
	after, _ := os.ReadFile(state)
	if string(before) != string(after) {
		t.Fatal("uncertain state overwritten")
	}
	if e = b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
		t.Fatal(e)
	}
	r, e = traderelay.Run(context.Background(), j, "", s.p.token, state, "recover")
	if e != nil || r.Status != "confirmed" || r.Posts != 0 || r.SourceCalls != 0 {
		t.Fatalf("recovery: %+v %v", r, e)
	}
	var total int
	if e = s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, j.OperationID).Scan(&total); e != nil || total != 1 {
		t.Fatal("duplicated effect")
	}
}
func TestTradeRelayExplicitRetryAfterConfirmed404KeepsIDs(t *testing.T) {
	b, s, j, state := relaySetup(t)
	var drop atomic.Bool
	drop.Store(true)
	proxy := relayFaultProxy(t, s.p.service.url, &drop, false)
	j.Target.URL = proxy.URL + "/local/v1"
	ctx := context.Background()
	r, e := traderelay.Run(ctx, j, b.p.token, s.p.token, state, "send")
	if !errors.Is(e, traderelay.ErrUncertain) || r.Posts != 1 {
		t.Fatal("missing uncertain attempt")
	}
	r, e = traderelay.Run(ctx, j, b.p.token, s.p.token, state, "recover")
	if !errors.Is(e, traderelay.ErrUncertain) || r.Posts != 0 {
		t.Fatal("recovery wrote")
	}
	drop.Store(false)
	r, e = traderelay.Run(ctx, j, b.p.token, s.p.token, state, "retry")
	if e != nil || r.Status != "confirmed" || r.OperationID != j.OperationID || r.Posts != 1 {
		t.Fatalf("retry: %+v %v", r, e)
	}
}
func TestTradeRelayRejectsWrongContextAndRestoreGate(t *testing.T) {
	for _, scenario := range []string{"actor", "fingerprint", "target-party", "restored"} {
		t.Run(scenario, func(t *testing.T) {
			b, s, j, state := relaySetup(t)
			switch scenario {
			case "actor":
				j.Source.ActorID = "other"
			case "fingerprint":
				j.Target.Fingerprint = strings.Repeat("0", 64)
			case "target-party":
				j.Target.Party.DeviceID = "other"
			case "restored":
				_, e := b.f.db.Exec(`INSERT INTO company_trade_recovery(tenant_id,store_id,generation,required,approved_by,verification_reference,updated_at) VALUES(?,?,?,1,'','','2026-10-25T00:00:00Z')`, b.f.device.TenantID, b.f.device.StoreID, "restored-generation")
				if e != nil {
					t.Fatal(e)
				}
			}
			r, e := traderelay.Run(context.Background(), j, b.p.token, s.p.token, state, "send")
			if !errors.Is(e, traderelay.ErrContext) || r.Posts != 0 {
				t.Fatalf("context allowed: %+v %v", r, e)
			}
			if _, e = os.Stat(state); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("unexpected state write")
			}
		})
	}
}

func TestTradeRelayRefusesRedirectAndUntrustedTLS(t *testing.T) {
	for _, scenario := range []string{"redirect", "tls"} {
		t.Run(scenario, func(t *testing.T) {
			b, s, j, state := relaySetup(t)
			var captured atomic.Int32
			sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured.Add(1); w.WriteHeader(500) }))
			defer sink.Close()
			var front *httptest.Server
			if scenario == "redirect" {
				front = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, sink.URL+"/steal", http.StatusTemporaryRedirect)
				}))
			} else {
				front = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured.Add(1); w.WriteHeader(500) }))
			}
			defer front.Close()
			j.Target.URL = front.URL + "/local/v1"
			result, e := traderelay.Run(context.Background(), j, b.p.token, s.p.token, state, "send")
			if e == nil || result.Posts != 0 || captured.Load() != 0 {
				t.Fatalf("unsafe transport permitted: %+v %v calls%d", result, e, captured.Load())
			}
		})
	}
}
func TestTradeRelaySessionChangeKeepsUncertainAttempt(t *testing.T) {
	b, s, j, state := relaySetup(t)
	var drop atomic.Bool
	drop.Store(true)
	proxy := relayFaultProxy(t, s.p.service.url, &drop, true)
	j.Target.URL = proxy.URL + "/local/v1"
	_, e := traderelay.Run(context.Background(), j, b.p.token, s.p.token, state, "send")
	if !errors.Is(e, traderelay.ErrUncertain) {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(state)
	for _, mode := range []string{"recover", "retry"} {
		result, e := traderelay.Run(context.Background(), j, b.p.token, "expired-session", state, mode)
		var h traderelay.HTTPError
		if !errors.As(e, &h) || h.Status != 401 || result.Posts != 0 {
			t.Fatal("stale session allowed write")
		}
		after, _ := os.ReadFile(state)
		if string(before) != string(after) {
			t.Fatal("stale session erased uncertainty")
		}
	}
}
