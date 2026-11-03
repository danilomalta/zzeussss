package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

type queuedFixture struct {
	b, s                         *tradeHTTPFixture
	j                            traderelay.Job
	dir, job, source, target, id string
}

func queueReal(t *testing.T, alter func(*tradeHTTPFixture, *traderelay.Job)) *queuedFixture {
	t.Helper()
	b, s, j, _ := relaySetup(t)
	if alter != nil {
		alter(s, &j)
	}
	base := t.TempDir()
	q := &queuedFixture{b: b, s: s, j: j, dir: filepath.Join(base, "queue"), job: filepath.Join(base, "job.json"), source: filepath.Join(base, "source-token"), target: filepath.Join(base, "target-token")}
	body, _ := json.Marshal(j)
	for path, value := range map[string][]byte{q.job: body, q.source: []byte(b.p.token), q.target: []byte(s.p.token)} {
		if e := os.WriteFile(path, value, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := traderelay.QueueInit(q.dir); e != nil {
		t.Fatal(e)
	}
	added, e := traderelay.QueueAdd(context.Background(), q.dir, q.job, q.source, q.target)
	if e != nil {
		t.Fatal(e)
	}
	q.id = added.Item.ID
	return q
}
func assertSingleQueuedEffect(t *testing.T, q *queuedFixture) {
	t.Helper()
	var count int
	if e := q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("remote effects%d %v", count, e)
	}
}
func TestTradeQueueRealDeliveryVerifyAndFrozenMessage(t *testing.T) {
	q := queueReal(t, nil)
	ctx := context.Background()
	changed := q.j
	changed.OperationID = "changed-after-authorization"
	modified, _ := json.Marshal(changed)
	_ = os.WriteFile(q.job, modified, 0600)
	b, e := traderelay.QueueRun(ctx, q.dir, 10, time.Now().Add(time.Second))
	if e != nil || b.Confirmed != 1 || b.Posts != 1 {
		t.Fatalf("delivery %+v %v", b, e)
	}
	assertSingleQueuedEffect(t, q)
	b, e = traderelay.QueueRun(ctx, q.dir, 10, time.Now().Add(time.Hour))
	if e != nil || b.Processed != 0 {
		t.Fatal("confirmed job dispatched again")
	}
	b, e = traderelay.QueueVerify(ctx, q.dir, q.id, time.Now())
	if e != nil || b.Confirmed != 1 || b.Posts != 0 || b.SourceCalls != 0 || b.TargetCalls != 3 {
		t.Fatalf("verify %+v %v", b, e)
	}
	// Neither a duplicate operation nor a new operation for the same message can
	// enter the queue. No request is emitted for these duplicate registrations.
	original, _ := json.Marshal(q.j)
	_ = os.WriteFile(q.job, original, 0600)
	if _, e = traderelay.QueueAdd(ctx, q.dir, q.job, q.source, q.target); e == nil {
		t.Fatal("duplicate queued")
	}
	j := q.j
	j.OperationID = "different-operation"
	body, _ := json.Marshal(j)
	_ = os.WriteFile(q.job, body, 0600)
	if _, e = traderelay.QueueAdd(ctx, q.dir, q.job, q.source, q.target); e == nil {
		t.Fatal("same message queued under new operation")
	}
}
func TestTradeQueueSessionReplacementRequiresExplicitResumeAndReadOnlyRecovery(t *testing.T) {
	var drop atomic.Bool
	drop.Store(true)
	q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
		p := relayFaultProxy(t, s.p.service.url, &drop, true)
		j.Target.URL = p.URL + "/local/v1"
	})
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	batch, e := traderelay.QueueRun(ctx, q.dir, 10, now)
	if e != nil || batch.Uncertain != 1 || batch.Posts != 1 {
		t.Fatalf("lost response %+v %v", batch, e)
	}
	proofPath := filepath.Join(q.dir, q.id+".state.json")
	proof, _ := os.ReadFile(proofPath)
	replacement := q.s.p.login(q.s.f.owner.OwnerID)
	_ = os.WriteFile(q.target, []byte(replacement), 0600)
	batch, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Minute))
	if e != nil || batch.Paused != 1 || batch.Posts != 0 || batch.TargetCalls != 0 || batch.SourceCalls != 0 {
		t.Fatalf("replacement used automatically %+v %v", batch, e)
	}
	after, _ := os.ReadFile(proofPath)
	if string(after) != string(proof) {
		t.Fatal("uncertain attempt deleted/changed")
	}
	if _, e = traderelay.QueueResume(ctx, q.dir, q.id, q.source, q.target); e != nil {
		t.Fatal(e)
	}
	if e = q.b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
		t.Fatal(e)
	}
	batch, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(2*time.Minute))
	if e != nil || batch.Confirmed != 1 || batch.Posts != 0 || batch.SourceCalls != 0 {
		t.Fatalf("offline recovery %+v %v", batch, e)
	}
	assertSingleQueuedEffect(t, q)
}
func TestTradeQueueUncertain404NeverAutomaticallyResends(t *testing.T) {
	var drop atomic.Bool
	drop.Store(true)
	q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
		p := relayFaultProxy(t, s.p.service.url, &drop, false)
		j.Target.URL = p.URL + "/local/v1"
	})
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	b, e := traderelay.QueueRun(ctx, q.dir, 10, now)
	if e != nil || b.Uncertain != 1 || b.Posts != 1 {
		t.Fatal("missing uncertain state")
	}
	b, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Minute))
	if e != nil || b.ManualRetry != 1 || b.Posts != 0 {
		t.Fatalf("404 resubmitted %+v %v", b, e)
	}
	b, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(time.Hour))
	if e != nil || b.Processed != 0 {
		t.Fatal("manual retry requirement ignored")
	}
	if _, e = traderelay.QueueResume(ctx, q.dir, q.id, q.source, q.target); e != nil {
		t.Fatal(e)
	}
	b, e = traderelay.QueueRun(ctx, q.dir, 10, now.Add(2*time.Hour))
	if e != nil || b.ManualRetry != 1 || b.Posts != 0 {
		t.Fatal("reauthorization alone retried uncertain effect")
	}
	drop.Store(false)
	b, e = traderelay.QueueRetry(ctx, q.dir, q.id, now.Add(3*time.Hour))
	if e != nil || b.Confirmed != 1 || b.Posts != 1 {
		t.Fatalf("explicit retry %+v %v", b, e)
	}
	assertSingleQueuedEffect(t, q)
}
func TestTradeQueueLogoutPausesWithoutCreatingAttempt(t *testing.T) {
	q := queueReal(t, nil)
	q.s.p.call("POST", "/local/v1/logout", nil, 204, nil)
	b, e := traderelay.QueueRun(context.Background(), q.dir, 10, time.Now().Add(time.Second))
	if e != nil || b.Paused != 1 || b.Posts != 0 {
		t.Fatalf("logout %+v %v", b, e)
	}
	if _, e = os.Stat(filepath.Join(q.dir, q.id+".state.json")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("logged out context created attempt")
	}
	v, e := traderelay.QueueRead(q.dir)
	if e != nil || v.Items[0].LastError != "authentication_or_permission" {
		t.Fatal("missing session-expiry pause")
	}
}
