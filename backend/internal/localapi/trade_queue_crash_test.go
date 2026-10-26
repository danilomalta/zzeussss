package localapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func TestTradeQueueProcessKilledAfterRemoteCommitRecoversWithoutSecondPost(t *testing.T) {
	committed := make(chan struct{})
	var once sync.Once
	var block atomic.Bool
	var posts atomic.Int32
	block.Store(true)
	q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.Proxy = nil
		t.Cleanup(tr.CloseIdleConnections)
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			body, e := io.ReadAll(res.Body)
			if e != nil {
				w.WriteHeader(502)
				return
			}
			if r.Method == "POST" {
				posts.Add(1)
				if block.Load() {
					once.Do(func() { close(committed) })
					<-r.Context().Done()
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(res.StatusCode)
			_, _ = w.Write(body)
		}))
		t.Cleanup(proxy.Close)
		j.Target.URL = proxy.URL + "/local/v1"
	})
	binary := relayExecutable(t)
	cmd := exec.Command(binary, "queue", "watch", "--queue", q.dir, "--interval", "1s")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	select {
	case <-committed:
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("worker did not reach real commit: %s", output.Bytes())
	}
	if e := cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	if e := cmd.Wait(); e == nil {
		t.Fatal("forced process death was not observed")
	}
	block.Store(false)
	next := exec.Command(binary, "queue", "run", "--queue", q.dir)
	body, e := next.CombinedOutput()
	if e != nil {
		t.Fatalf("new worker recovery %v %s", e, body)
	}
	var result traderelay.QueueBatch
	if json.Unmarshal(body, &result) != nil || result.Confirmed != 1 || result.Posts != 0 || result.SourceCalls != 0 {
		t.Fatalf("duplicate/recovery %+v", result)
	}
	if posts.Load() != 1 {
		t.Fatalf("POST count%d", posts.Load())
	}
	assertSingleQueuedEffect(t, q)
	t.Log("QUEUE CRASH: compiled worker killed after real recipient commit; new process recovered receipt with zero POST; single remote effect")
}
