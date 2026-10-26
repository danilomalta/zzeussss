package localapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func writeQueueScope(t *testing.T, path string, source traderelay.Endpoint) *traderelay.QueueBinding {
	t.Helper()
	raw, _ := json.Marshal(traderelay.QueueScope{Version: 1, Source: source})
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	bound, e := traderelay.BindQueueScope(path)
	if e != nil {
		t.Fatal(e)
	}
	return bound
}
func TestTradeQueueScopeRealMixedCompaniesAndAllSourceDimensions(t *testing.T) {
	first, other := queueReal(t, nil), queueReal(t, nil)
	ctx := context.Background()
	if _, e := traderelay.QueueAdd(ctx, first.dir, other.job, other.source, other.target); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "scope.json")
	original, _ := os.ReadFile(filepath.Join(first.dir, "queue.json"))
	for _, which := range []string{"tenant", "store", "device", "actor", "url", "fingerprint"} {
		source := first.j.Source
		switch which {
		case "tenant":
			source.Party.TenantID = "other-tenant"
		case "store":
			source.Party.StoreID = "other-store"
		case "device":
			source.Party.DeviceID = "other-device"
		case "actor":
			source.ActorID = "other-actor"
		case "url":
			source.URL = "http://127.0.0.1:1/local/v1"
		case "fingerprint":
			source.Fingerprint = strings.Repeat("0", 64)
		}
		bound := writeQueueScope(t, path, source)
		v, e := bound.Read(first.dir)
		if e != nil || v.Total != 0 {
			t.Fatal(which, v, e)
		}
		b, e := bound.Run(ctx, first.dir, 10, time.Now().Add(time.Second))
		if e != nil || b != (traderelay.QueueBatch{}) {
			t.Fatal(which, b, e)
		}
	}
	after, _ := os.ReadFile(filepath.Join(first.dir, "queue.json"))
	if string(after) != string(original) {
		t.Fatal("foreign filtering rewrote original attempts")
	}
	bound := writeQueueScope(t, path, first.j.Source)
	view, e := bound.Read(first.dir)
	if e != nil || view.Total != 1 || view.Items[0].ID != first.id {
		t.Fatal(view, e)
	}
	beforeOther, e := traderelay.QueueRead(first.dir)
	if e != nil {
		t.Fatal(e)
	}
	batch, e := bound.Run(ctx, first.dir, 1, time.Now().Add(time.Second))
	if e != nil || batch.Confirmed != 1 || batch.Posts != 1 {
		t.Fatal(batch, e)
	}
	assertSingleQueuedEffect(t, first)
	all, e := traderelay.QueueRead(first.dir)
	if e != nil || all.Total != 2 || all.Items[1].ID != other.id || !reflect.DeepEqual(all.Items[1], beforeOther.Items[1]) {
		t.Fatal("other company attempt changed", e)
	}
	var effects int
	if e = other.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, other.j.OperationID).Scan(&effects); e != nil || effects != 0 {
		t.Fatal(effects, e)
	}
}

func TestTradeQueueScopeChangeDuringRealHTTPNeverRepostsOrDropsProof(t *testing.T) {
	for _, mode := range []string{"before-post", "confirmed-response", "lost-response"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scope.json")
			var armed atomic.Bool
			var identities, posts atomic.Int32
			var source traderelay.Endpoint
			var writeFailed atomic.Bool
			q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
				tr := http.DefaultTransport.(*http.Transport).Clone()
				tr.Proxy = nil
				t.Cleanup(tr.CloseIdleConnections)
				proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					trigger := false
					if armed.Load() {
						if mode == "before-post" && r.Method == "GET" && r.URL.Path == "/local/v1/company-trade/identity" {
							trigger = identities.Add(1) == 2
						}
						if mode != "before-post" && r.Method == "POST" {
							trigger = true
						}
					}
					if trigger {
						next := source
						next.ActorID = "replacement-actor"
						raw, _ := json.Marshal(traderelay.QueueScope{Version: 1, Source: next})
						if e = os.WriteFile(path, raw, 0600); e != nil {
							writeFailed.Store(true)
						}
						if mode == "lost-response" {
							conn, _, e := w.(http.Hijacker).Hijack()
							if e == nil {
								conn.Close()
							}
							return
						}
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(res.StatusCode)
					io.Copy(w, res.Body)
				}))
				t.Cleanup(proxy.Close)
				j.Target.URL = proxy.URL + "/local/v1"
			})
			source = q.j.Source
			bound := writeQueueScope(t, path, source)
			armed.Store(true)
			batch, e := bound.Run(context.Background(), q.dir, 10, time.Now().Add(time.Second))
			if !errors.Is(e, traderelay.ErrScopeChanged) || writeFailed.Load() {
				t.Fatal(batch, e)
			}
			all, e := traderelay.QueueRead(q.dir)
			if e != nil || all.Total != 1 || all.Items[0].OperationID != q.j.OperationID || all.Items[0].MessageID != q.j.MessageID {
				t.Fatal(all, e)
			}
			proofPath := filepath.Join(q.dir, q.id+".state.json")
			proof, _ := os.ReadFile(proofPath)
			var effects int
			if e = q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&effects); e != nil {
				t.Fatal(e)
			}
			if mode == "before-post" {
				if posts.Load() != 0 || effects != 0 || batch.Posts != 0 || all.Items[0].Status != "paused" || len(proof) != 0 {
					t.Fatal("old query allowed POST", batch, all)
				}
			} else {
				if posts.Load() != 1 || effects != 1 || batch.Posts != 1 || len(proof) == 0 {
					t.Fatal("lost original effect/proof", batch, all)
				}
				if mode == "lost-response" && all.Items[0].Status != "uncertain" {
					t.Fatal(all)
				}
				if mode == "confirmed-response" && all.Items[0].Status != "confirmed" {
					t.Fatal(all)
				}
			}
			// An already-bound worker must not start using the replacement context.
			if _, e = bound.Read(q.dir); !errors.Is(e, traderelay.ErrScopeChanged) {
				t.Fatal("silently rebound", e)
			}
			replacement, e := traderelay.BindQueueScope(path)
			if e != nil {
				t.Fatal(e)
			}
			b, e := replacement.Run(context.Background(), q.dir, 10, time.Now().Add(time.Hour))
			if e != nil || b.Processed != 0 {
				t.Fatal("new context touched old attempt", b, e)
			}
			after, _ := os.ReadFile(proofPath)
			if string(after) != string(proof) {
				t.Fatal("context replacement changed proof")
			}
			armed.Store(false)
			restored := writeQueueScope(t, path, source)
			b, e = restored.Run(context.Background(), q.dir, 10, time.Now().Add(time.Hour))
			if e != nil || b.Posts != 0 {
				t.Fatal("automatic resend after context return", b, e)
			}
			if mode == "lost-response" {
				if b.Confirmed != 1 || b.SourceCalls != 0 {
					t.Fatal("did not recover original receipt", b)
				}
				assertSingleQueuedEffect(t, q)
			}
			t.Logf("SCOPED REAL HTTP: %s; posts%d; original operation/message retained; replaced context never dispatched; no automatic resend", mode, posts.Load())
		})
	}
}
func TestTradeQueueScopeCompiledExecutableFiltersAndDelivers(t *testing.T) {
	q := queueReal(t, nil)
	path := filepath.Join(t.TempDir(), "scope.json")
	wrong := q.j.Source
	wrong.ActorID = "other-actor"
	writeQueueScope(t, path, wrong)
	binary := relayExecutable(t)
	call := func(command string, out any) {
		t.Helper()
		cmd := exec.Command(binary, "queue", command, "--queue", q.dir, "--scope", path)
		data, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatal("compiled scoped command failed; output omitted", e)
		}
		if json.Unmarshal(data, out) != nil {
			t.Fatal("invalid scoped output")
		}
	}
	var view traderelay.QueueView
	call("status", &view)
	if view.Total != 0 {
		t.Fatal("other context exposed")
	}
	var batch traderelay.QueueBatch
	call("run", &batch)
	if batch.Processed != 0 || batch.Posts != 0 {
		t.Fatal(batch)
	}
	writeQueueScope(t, path, q.j.Source)
	call("status", &view)
	if view.Total != 1 || view.Items[0].ID != q.id {
		t.Fatal(view)
	}
	call("run", &batch)
	if batch.Confirmed != 1 || batch.Posts != 1 {
		t.Fatal(batch)
	}
	assertSingleQueuedEffect(t, q)
	call("run", &batch)
	if batch.Processed != 0 || batch.Posts != 0 {
		t.Fatal(batch)
	}
	t.Log("COMPILED SCOPED QUEUE: real HTTP delivery, wrong actor hidden and untouched, original IDs, repeat produces zero POST")
}

func TestTradeQueueScopeCompiledWatchNeverRebindsToReplacement(t *testing.T) {
	q := queueReal(t, nil)
	path := filepath.Join(t.TempDir(), "scope.json")
	wrong := q.j.Source
	wrong.ActorID = "another-operator"
	writeQueueScope(t, path, wrong)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, relayExecutable(t), "queue", "watch", "--queue", q.dir, "--scope", path, "--interval", "1s", "--limit", "1")
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	first, e := bufio.NewReader(stdout).ReadString('\n')
	if e != nil || !strings.Contains(first, `"watching":true`) {
		t.Fatal("worker did not start")
	}
	// Replacing the file with a scope that now matches the pending item must
	// stop this worker, rather than silently expanding its dispatch authority.
	writeQueueScope(t, path, q.j.Source)
	e = cmd.Wait()
	if e == nil || ctx.Err() != nil {
		t.Fatal("watch did not stop after scope replacement")
	}
	if !strings.Contains(stderr.String(), "worker context changed") {
		t.Fatal("missing bounded context error")
	}
	view, e := traderelay.QueueRead(q.dir)
	if e != nil || view.Total != 1 || view.Items[0].Status != "queued" || view.Items[0].Attempts != 0 {
		t.Fatal(view, e)
	}
	var count int
	if e = q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&count); e != nil || count != 0 {
		t.Fatal("new context was used automatically", count, e)
	}
	t.Log("COMPILED WATCH: replacement context stopped worker; pending original record retained with zero attempts and zero remote effect")
}

func TestTradeQueueScopeRestoredReceiptKeepsBarrierWithSourceOffline(t *testing.T) {
	q := queueReal(t, nil)
	scope := filepath.Join(t.TempDir(), "scope.json")
	bound := writeQueueScope(t, scope, q.j.Source)
	first, e := bound.Run(context.Background(), q.dir, 10, time.Now().Add(time.Second))
	if e != nil || first.Confirmed != 1 || first.Posts != 1 {
		t.Fatal(first, e)
	}
	base := t.TempDir()
	archive := filepath.Join(base, "queue.enc")
	restored := filepath.Join(base, "restored")
	key := bytes.Repeat([]byte{19}, 32)
	if _, e = traderelay.QueueBackup(q.dir, key, archive); e != nil {
		t.Fatal(e)
	}
	if _, e = traderelay.QueueRestore(archive, key, restored); e != nil {
		t.Fatal(e)
	}
	view, e := bound.Read(restored)
	if e != nil || view.Total != 1 || !view.Items[0].RecoveryOnly || view.Items[0].Status != "paused" {
		t.Fatal(view, e)
	}
	idle, e := bound.Run(context.Background(), restored, 10, time.Now().Add(time.Hour))
	if e != nil || idle.Processed != 0 || idle.Posts != 0 {
		t.Fatal("restored queue dispatched automatically", idle, e)
	}
	if e = q.b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(q.source); e != nil {
		t.Fatal(e)
	}
	if _, e = traderelay.QueueRecoverSession(context.Background(), restored, q.id, q.target); e != nil {
		t.Fatal("receipt reauthorization contacted source", e)
	}
	recovered, e := bound.Run(context.Background(), restored, 10, time.Now().Add(time.Hour))
	if e != nil || recovered.Confirmed != 1 || recovered.Posts != 0 || recovered.SourceCalls != 0 {
		t.Fatal("scope lost restore barrier", recovered, e)
	}
	view, e = bound.Read(restored)
	if e != nil || view.Total != 1 || !view.Items[0].RecoveryOnly || !view.Items[0].ReceiptOnlySession || view.Items[0].ID != q.id {
		t.Fatal(view, e)
	}
	assertSingleQueuedEffect(t, q)
	t.Log("SCOPED RESTORED QUEUE: explicit receipt reauthorization, source API/file absent, original IDs and permanent recovery-only barrier retained, zero POST")
}
