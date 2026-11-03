package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func TestTradeQueueContextCompiledCommandVerifiesSourceAndNeverOverwrites(t *testing.T) {
	q := queueReal(t, nil)
	parent := t.TempDir()
	if e := os.Chmod(parent, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(parent, "verified-context.json")
	binary := relayExecutable(t)
	cmd := exec.Command(binary, "queue", "context", "--job", q.job, "--source-token", q.source, "--out", path)
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal("compiled verified context failed; output omitted", e)
	}
	var result map[string]bool
	if json.Unmarshal(output, &result) != nil || len(result) != 1 || !result["context_created"] {
		t.Fatal("context command exposed metadata or returned invalid result")
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var scope traderelay.QueueScope
	if json.Unmarshal(before, &scope) != nil || scope.Version != 2 || scope.Source != q.j.Source || len(scope.SourceSessionHash) != 64 {
		t.Fatal("invalid authenticated context")
	}
	if strings.Contains(string(before), q.b.p.token) {
		t.Fatal("context persisted bearer")
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("context not private", e)
	}
	bound, e := traderelay.BindQueueScope(path)
	if e != nil {
		t.Fatal(e)
	}
	view, e := bound.Read(q.dir)
	if e != nil || view.Total != 1 || view.Items[0].ID != q.id {
		t.Fatal(view, e)
	}
	// The command only issued authenticated reads. No remote business effect yet.
	var count int
	if e = q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&count); e != nil || count != 0 {
		t.Fatal("context creation dispatched a document", e)
	}
	again := exec.Command(binary, "queue", "context", "--job", q.job, "--source-token", q.source, "--out", path)
	if e = again.Run(); e == nil {
		t.Fatal("existing context overwritten")
	}
	after, e := os.ReadFile(path)
	if e != nil || string(before) != string(after) {
		t.Fatal("existing context changed", e)
	}
	batch, e := bound.Run(context.Background(), q.dir, 1, time.Now().Add(time.Second))
	if e != nil || batch.Confirmed != 1 || batch.Posts != 1 {
		t.Fatal(batch, e)
	}
	assertSingleQueuedEffect(t, q)
}

func TestTradeQueueContextRefusesWrongIdentityAndRevokedSession(t *testing.T) {
	q := queueReal(t, nil)
	parent := t.TempDir()
	if e := os.Chmod(parent, 0700); e != nil {
		t.Fatal(e)
	}
	for _, which := range []string{"actor", "fingerprint", "logout"} {
		t.Run(which, func(t *testing.T) {
			source := q.j.Source
			if which == "actor" {
				source.ActorID = "another-actor"
			}
			if which == "fingerprint" {
				source.Fingerprint = strings.Repeat("0", 64)
			}
			if which == "logout" {
				q.b.p.call("POST", "/local/v1/logout", nil, 204, nil)
			}
			path := filepath.Join(parent, which+".json")
			if e := traderelay.CreateQueueSessionScope(context.Background(), source, q.source, path); e == nil {
				t.Fatal("unverified context published")
			}
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				t.Fatal("failed verification left a context file")
			}
		})
	}
}

func TestTradeQueueContextSessionFileReplacementDuringVerificationPublishesNothing(t *testing.T) {
	q := queueReal(t, nil)
	replacement := q.b.p.login(q.b.f.owner.OwnerID)
	var changed, failed atomic.Bool
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	t.Cleanup(tr.CloseIdleConnections)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, e := http.NewRequestWithContext(r.Context(), r.Method, q.b.p.service.url+r.URL.RequestURI(), r.Body)
		if e != nil {
			w.WriteHeader(500)
			return
		}
		request.Header = r.Header.Clone()
		response, e := tr.RoundTrip(request)
		if e != nil {
			w.WriteHeader(502)
			return
		}
		defer response.Body.Close()
		if r.URL.Path == "/local/v1/company-trade/identity" {
			if e = os.WriteFile(q.source, []byte(replacement), 0600); e != nil {
				failed.Store(true)
			}
			changed.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	}))
	t.Cleanup(proxy.Close)
	parent := t.TempDir()
	if e := os.Chmod(parent, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(parent, "never-publish.json")
	source := q.j.Source
	source.URL = proxy.URL + "/local/v1"
	e := traderelay.CreateQueueSessionScope(context.Background(), source, q.source, path)
	if !errors.Is(e, traderelay.ErrScopeChanged) || !changed.Load() || failed.Load() {
		t.Fatal("session changed during verification", e)
	}
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		t.Fatal("stale authenticated context published")
	}
}
