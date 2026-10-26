package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func TestTradeQueueRecoverySessionCompiledWithSourceOffline(t *testing.T) {
	binary := relayExecutable(t)
	for _, scenario := range []string{"uncertain", "confirmed", "restored"} {
		t.Run(scenario, func(t *testing.T) {
			var drop atomic.Bool
			drop.Store(true)
			q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
				if scenario != "confirmed" {
					proxy := relayFaultProxy(t, s.p.service.url, &drop, true)
					j.Target.URL = proxy.URL + "/local/v1"
				}
			})
			ctx := context.Background()
			now := time.Now().Add(time.Second)
			initial, e := traderelay.QueueRun(ctx, q.dir, 10, now)
			if e != nil || initial.Posts != 1 {
				t.Fatal("initial delivery", e)
			}
			assertSingleQueuedEffect(t, q)
			if scenario == "restored" {
				key := make([]byte, 32)
				if _, e = rand.Read(key); e != nil {
					t.Fatal(e)
				}
				archive := filepath.Join(t.TempDir(), "queue.enc")
				if _, e = traderelay.QueueBackup(q.dir, key, archive); e != nil {
					t.Fatal(e)
				}
				dest := filepath.Join(t.TempDir(), "restored")
				if _, e = traderelay.QueueRestore(archive, key, dest); e != nil {
					t.Fatal(e)
				}
				q.dir = dest
			}
			proof := filepath.Join(q.dir, q.id+".state.json")
			before, _ := os.ReadFile(proof)
			q.s.p.call("POST", "/local/v1/logout", nil, 204, nil)
			replacement := q.s.p.login(q.s.f.owner.OwnerID)
			if e = os.WriteFile(q.target, []byte(replacement), 0600); e != nil {
				t.Fatal(e)
			}
			if e = os.Remove(q.source); e != nil {
				t.Fatal(e)
			} // Only synthetic credential in this fixture.
			if e = q.b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(binary, "queue", "recover-session", "--queue", q.dir, "--id", q.id, "--target-token", q.target)
			raw, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("compiled recovery grant: %v %s", e, raw)
			}
			var action traderelay.QueueAction
			if json.Unmarshal(raw, &action) != nil || !action.Item.ReceiptOnlySession || action.SourceCalls != 0 || action.TargetCalls != 2 || action.Item.OperationID != q.j.OperationID {
				t.Fatalf("unsafe grant %+v", action)
			}
			after, _ := os.ReadFile(proof)
			if string(before) != string(after) {
				t.Fatal("grant changed original proof")
			}
			b, e := traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Hour))
			if e != nil || b.Confirmed != 1 || b.Posts != 0 || b.SourceCalls != 0 || b.TargetCalls != 3 {
				t.Fatalf("offline read-only recovery %+v %v", b, e)
			}
			assertSingleQueuedEffect(t, q)
			if _, e = traderelay.QueueRetry(ctx, q.dir, q.id, now.Add(2*time.Hour)); e == nil {
				t.Fatal("receipt-only authorization permitted retry")
			}
			t.Logf("receipt-only fresh session: %s, sender offline/credential absent, zero POST, original receipt confirmed", scenario)
		})
	}
}
func TestTradeQueueRecoverySessionRequiresExistingProofAndCorrectTarget(t *testing.T) {
	q := queueReal(t, nil)
	ctx := context.Background()
	original, _ := os.ReadFile(filepath.Join(q.dir, "queue.json"))
	a, e := traderelay.QueueRecoverSession(ctx, q.dir, q.id, q.target)
	if e == nil || a.SourceCalls != 0 || a.TargetCalls != 0 {
		t.Fatal("missing proof accepted or contacted API")
	}
	unchanged, _ := os.ReadFile(filepath.Join(q.dir, "queue.json"))
	if string(original) != string(unchanged) {
		t.Fatal("failed grant changed queue")
	}
	b, e := traderelay.QueueRun(ctx, q.dir, 10, time.Now().Add(time.Second))
	if e != nil || b.Confirmed != 1 {
		t.Fatal(e)
	}
	original, _ = os.ReadFile(filepath.Join(q.dir, "queue.json"))
	wrong := filepath.Join(t.TempDir(), "wrong-token")
	_ = os.WriteFile(wrong, []byte(q.b.p.token), 0600)
	a, e = traderelay.QueueRecoverSession(ctx, q.dir, q.id, wrong)
	if e == nil || a.SourceCalls != 0 {
		t.Fatal("other company session accepted")
	}
	unchanged, _ = os.ReadFile(filepath.Join(q.dir, "queue.json"))
	if string(original) != string(unchanged) {
		t.Fatal("wrong target changed original authorization")
	}
	proof := filepath.Join(q.dir, q.id+".state.json")
	_ = os.WriteFile(proof, []byte(`{"version":1,"status":"confirmed"}`), 0600)
	a, e = traderelay.QueueRecoverSession(ctx, q.dir, q.id, q.target)
	if e == nil || a.TargetCalls != 0 {
		t.Fatal("invalid original proof accepted")
	}
}
func TestTradeQueueRecoverySessionNeverRecreatesMissingAttempt(t *testing.T) {
	q := queueReal(t, nil)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	if b, e := traderelay.QueueRun(ctx, q.dir, 10, now); e != nil || b.Confirmed != 1 {
		t.Fatal(e)
	}
	if _, e := traderelay.QueueRecoverSession(ctx, q.dir, q.id, q.target); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(q.dir, q.id+".state.json")); e != nil {
		t.Fatal(e)
	}
	b, e := traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Hour))
	if e != nil || b.Paused != 1 || b.Posts != 0 || b.SourceCalls != 0 || b.TargetCalls != 0 {
		t.Fatalf("missing proof caused action %+v %v", b, e)
	}
	assertSingleQueuedEffect(t, q)
}
func TestTradeQueueRecoverySessionReplacementPausesBeforeHTTP(t *testing.T) {
	q := queueReal(t, nil)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	if b, e := traderelay.QueueRun(ctx, q.dir, 10, now); e != nil || b.Confirmed != 1 {
		t.Fatal(e)
	}
	if _, e := traderelay.QueueRecoverSession(ctx, q.dir, q.id, q.target); e != nil {
		t.Fatal(e)
	}
	replacement := q.s.p.login(q.s.f.owner.OwnerID)
	_ = os.WriteFile(q.target, []byte(replacement), 0600)
	b, e := traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Hour))
	if e != nil || b.Paused != 1 || b.Posts != 0 || b.SourceCalls != 0 || b.TargetCalls != 0 {
		t.Fatalf("session implicitly rebound %+v %v", b, e)
	}
	assertSingleQueuedEffect(t, q)
}
func TestTradeQueueRecoverySession404KeepsPendingWithoutRetry(t *testing.T) {
	var drop atomic.Bool
	drop.Store(true)
	q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
		proxy := relayFaultProxy(t, s.p.service.url, &drop, false)
		j.Target.URL = proxy.URL + "/local/v1"
	})
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	if b, e := traderelay.QueueRun(ctx, q.dir, 10, now); e != nil || b.Uncertain != 1 {
		t.Fatal(e)
	}
	if _, e := traderelay.QueueRecoverSession(ctx, q.dir, q.id, q.target); e != nil {
		t.Fatal(e)
	}
	b, e := traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Hour))
	if e != nil || b.Paused != 1 || b.Posts != 0 || b.SourceCalls != 0 {
		t.Fatalf("404 resent %+v %v", b, e)
	}
	if _, e = traderelay.QueueRetry(ctx, q.dir, q.id, now.Add(2*time.Hour)); e == nil {
		t.Fatal("receipt-only session retried")
	}
	if _, e = traderelay.QueueResume(ctx, q.dir, q.id, q.source, q.target); e != nil {
		t.Fatal(e)
	}
	view, e := traderelay.QueueRead(q.dir)
	if e != nil || view.Items[0].ReceiptOnlySession {
		t.Fatal("explicit full authorization did not reset limited mode")
	}
	b, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(3*time.Hour))
	if e != nil || b.ManualRetry != 1 || b.Posts != 0 {
		t.Fatal("full session resume alone resent uncertain operation")
	}
}
