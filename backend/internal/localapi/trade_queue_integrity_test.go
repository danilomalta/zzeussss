package localapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func TestTradeQueueFrozenHashCannotBeReplacedByCurrentSignedMessage(t *testing.T) {
	q := queueReal(t, nil)
	path := filepath.Join(q.dir, "queue.json")
	body, _ := os.ReadFile(path)
	var saved struct {
		Version int                    `json:"version"`
		Items   []traderelay.QueueItem `json:"items"`
	}
	if e := json.Unmarshal(body, &saved); e != nil {
		t.Fatal(e)
	}
	saved.Items[0].EnvelopeHash = strings.Repeat("0", 64)
	body, _ = json.Marshal(saved)
	_ = os.WriteFile(path, body, 0600)
	result, e := traderelay.QueueRun(context.Background(), q.dir, 10, time.Now().Add(time.Second))
	if e != nil || result.Paused != 1 || result.Posts != 0 {
		t.Fatalf("changed approval accepted %+v %v", result, e)
	}
	var total int
	_ = q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&total)
	if total != 0 {
		t.Fatal("unapproved envelope reached recipient")
	}
	if _, e = traderelay.QueueResume(context.Background(), q.dir, q.id, q.source, q.target); e == nil {
		t.Fatal("resume replaced frozen hash")
	}
}
func TestTradeQueueBoundedPassDoesNotLetExpiredSessionBlockOtherCompany(t *testing.T) {
	first, second := queueReal(t, nil), queueReal(t, nil)
	ctx := context.Background()
	if _, e := traderelay.QueueAdd(ctx, first.dir, second.job, second.source, second.target); e != nil {
		t.Fatal(e)
	}
	first.s.p.call("POST", "/local/v1/logout", nil, 204, nil)
	result, e := traderelay.QueueRun(ctx, first.dir, 1, time.Now().Add(time.Second))
	if e != nil || result.Processed != 1 || result.Paused != 1 || result.Posts != 0 {
		t.Fatalf("batch limit %+v %v", result, e)
	}
	result, e = traderelay.QueueRun(ctx, first.dir, 1, time.Now().Add(2*time.Second))
	if e != nil || result.Confirmed != 1 || result.Posts != 1 {
		t.Fatalf("healthy job blocked %+v %v", result, e)
	}
	assertSingleQueuedEffect(t, second)
	view, e := traderelay.QueueRead(first.dir)
	if e != nil || view.Total != 2 || view.Items[0].Status != "paused" || view.Items[1].Status != "confirmed" {
		t.Fatal("lost isolated queue states")
	}
}
func TestTradeQueueLostConfirmedReceiptNeverAuthorizesResend(t *testing.T) {
	var missing atomic.Bool
	var posts atomic.Int32
	q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
		operation := j.OperationID
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.Proxy = nil
		t.Cleanup(tr.CloseIdleConnections)
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if missing.Load() && r.Method == "GET" && r.URL.Path == "/local/v1/company-trade/operations/"+operation {
				w.WriteHeader(404)
				return
			}
			if r.Method == "POST" {
				posts.Add(1)
			}
			req, e := http.NewRequestWithContext(r.Context(), r.Method, s.p.service.url+r.URL.RequestURI(), r.Body)
			if e != nil {
				w.WriteHeader(500)
				return
			}
			req.Header = r.Header.Clone()
			res, e := tr.RoundTrip(req)
			if e != nil {
				w.WriteHeader(502)
				return
			}
			defer res.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(res.StatusCode)
			_, _ = io.Copy(w, res.Body)
		}))
		t.Cleanup(proxy.Close)
		j.Target.URL = proxy.URL + "/local/v1"
	})
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	result, e := traderelay.QueueRun(ctx, q.dir, 10, now)
	if e != nil || result.Confirmed != 1 {
		t.Fatal("first send failed")
	}
	missing.Store(true)
	result, e = traderelay.QueueVerify(ctx, q.dir, q.id, now.Add(time.Minute))
	if e != nil || result.Paused != 1 || result.Posts != 0 {
		t.Fatal("missing confirmed receipt was retried")
	}
	view, _ := traderelay.QueueRead(q.dir)
	if view.Items[0].LastError != "confirmed_receipt_missing" {
		t.Fatal("missing explicit lost-receipt warning")
	}
	if _, e = traderelay.QueueResume(ctx, q.dir, q.id, q.source, q.target); e != nil {
		t.Fatal(e)
	}
	result, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(2*time.Minute))
	if e != nil || result.Posts != 0 || result.ManualRetry != 1 {
		t.Fatal("reauthorization resent confirmed effect")
	}
	result, e = traderelay.QueueRetry(ctx, q.dir, q.id, now.Add(3*time.Minute))
	if e != nil || result.Posts != 0 || result.Paused != 1 || posts.Load() != 1 {
		t.Fatalf("explicit retry duplicated confirmed effect %+v %v posts%d", result, e, posts.Load())
	}
	assertSingleQueuedEffect(t, q)
}
