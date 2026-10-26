package traderelay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func queueFixture(t *testing.T) (string, QueueItem) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "queue")
	if e := QueueInit(dir); e != nil {
		t.Fatal(e)
	}
	source, target := filepath.Join(dir, "source-token"), filepath.Join(dir, "target-token")
	if e := os.WriteFile(source, []byte("new-source-session"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(target, []byte("target-session"), 0600); e != nil {
		t.Fatal(e)
	}
	j := sampleJob()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	i := QueueItem{ID: digest(j), Job: j, EnvelopeHash: strings.Repeat("d", 64), SourceTokenFile: source, TargetTokenFile: target, SourceSessionHash: digest("old-source-session"), TargetSessionHash: digest("target-session"), Status: "queued", CreatedAt: now, UpdatedAt: now, NextAttemptAt: now, LastAction: "authorized_add"}
	return dir, i
}
func TestQueueInitPreservesExistingAndRequiresPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "queue")
	if e := QueueInit(dir); e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(queuePath(dir))
	if e := QueueInit(dir); e == nil {
		t.Fatal("existing queue replaced")
	}
	after, _ := os.ReadFile(queuePath(dir))
	if string(after) != string(original) {
		t.Fatal("existing data changed")
	}
	other := t.TempDir()
	_ = os.WriteFile(filepath.Join(other, "important"), []byte("keep"), 0600)
	if e := QueueInit(other); e == nil {
		t.Fatal("nonempty directory accepted")
	}
	link := filepath.Join(t.TempDir(), "link")
	_ = os.Symlink(dir, link)
	if _, e := QueueRead(link); e == nil {
		t.Fatal("symbolic directory accepted")
	}
	_ = os.Chmod(dir, 0755)
	if _, e := QueueRead(dir); e == nil {
		t.Fatal("public queue accepted")
	}
}
func TestQueueSessionChangePausesBeforeNetworkAndKeepsOriginalState(t *testing.T) {
	dir, i := queueFixture(t)
	if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
		t.Fatal(e)
	}
	proof := []byte(`{"preserve":"original uncertain proof"}`)
	path := statePath(dir, i.ID)
	_ = os.WriteFile(path, proof, 0600)
	batch, e := QueueRun(context.Background(), dir, 10, time.Now().Add(time.Hour))
	if e != nil || batch.Paused != 1 || batch.Posts != 0 || batch.SourceCalls != 0 || batch.TargetCalls != 0 {
		t.Fatalf("session change caused network: %+v %v", batch, e)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(proof) {
		t.Fatal("old proof modified")
	}
	v, e := QueueRead(dir)
	if e != nil || v.Items[0].LastError != "session_or_context_changed" {
		t.Fatal("missing explicit pause")
	}
	batch, e = QueueRun(context.Background(), dir, 10, time.Now().Add(2*time.Hour))
	if e != nil || batch.Processed != 0 {
		t.Fatal("paused attempt restarted automatically")
	}
}
func TestQueueBoundedPassSchedulePauseAndSafeStatus(t *testing.T) {
	dir, i := queueFixture(t)
	i.NextAttemptAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
		t.Fatal(e)
	}
	b, e := QueueRun(context.Background(), dir, 10, time.Now())
	if e != nil || b.Processed != 0 {
		t.Fatal("future item processed")
	}
	if e = QueuePause(dir, i.ID); e != nil {
		t.Fatal(e)
	}
	b, e = QueueRun(context.Background(), dir, 10, time.Now().Add(2*time.Hour))
	if e != nil || b.Processed != 0 {
		t.Fatal("paused item processed")
	}
	v, e := QueueRead(dir)
	if e != nil {
		t.Fatal(e)
	}
	text, _ := json.Marshal(v)
	for _, s := range []string{i.SourceTokenFile, i.TargetTokenFile, i.SourceSessionHash, i.TargetSessionHash, "source_token", "session_sha"} {
		if strings.Contains(string(text), s) {
			t.Fatal("status exposes private binding")
		}
	}
	for _, n := range []int{0, 101, -1} {
		if _, e = QueueRun(context.Background(), dir, n, time.Now()); e == nil {
			t.Fatal("invalid batch size accepted")
		}
	}
	if _, e = QueueVerify(context.Background(), dir, i.ID, time.Now()); e == nil {
		t.Fatal("verify created first attempt")
	}
}
func TestQueueMalformedMetadataNeverRunsOrEchoesArbitraryErrors(t *testing.T) {
	for _, which := range []string{"job", "session", "error", "action", "duplicate", "permissions", "schedule"} {
		t.Run(which, func(t *testing.T) {
			dir, i := queueFixture(t)
			q := queueFile{1, []QueueItem{i}}
			switch which {
			case "job":
				q.Items[0].Job.OperationID = "changed"
			case "session":
				q.Items[0].SourceSessionHash = "not-a-hash"
			case "error":
				q.Items[0].LastError = "secret-from-corrupted-file"
			case "action":
				q.Items[0].LastAction = "unknown"
			case "schedule":
				q.Items[0].NextAttemptAt = ""
			case "duplicate":
				q.Items = append(q.Items, i)
			}
			if e := saveQueue(dir, q); e != nil {
				t.Fatal(e)
			}
			if which == "permissions" {
				_ = os.Chmod(queuePath(dir), 0644)
			}
			if _, e := QueueRead(dir); e == nil {
				t.Fatal("damaged queue accepted")
			}
			if _, e := QueueRun(context.Background(), dir, 10, time.Now().Add(time.Hour)); e == nil {
				t.Fatal("damaged queue executed")
			}
		})
	}
}
func TestQueueRefusesSilentTruncationAndConflictingWorkers(t *testing.T) {
	dir, i := queueFixture(t)
	q := queueFile{1, []QueueItem{}}
	for n := 0; n < MaxQueueItems; n++ {
		item := i
		item.Job.MessageID = fmt.Sprintf("message-%d", n)
		item.Job.OperationID = fmt.Sprintf("operation-%d", n)
		item.ID = digest(item.Job)
		q.Items = append(q.Items, item)
	}
	if e := saveQueue(dir, q); e != nil {
		t.Fatal(e)
	}
	job := filepath.Join(dir, "job.json")
	body, _ := json.Marshal(sampleJob())
	_ = os.WriteFile(job, body, 0600)
	if _, e := QueueAdd(context.Background(), dir, job, i.SourceTokenFile, i.TargetTokenFile); !errors.Is(e, ErrQueueFull) {
		t.Fatalf("queue limit: %v", e)
	}
	view, e := QueueRead(dir)
	if e != nil || view.Total != MaxQueueItems {
		t.Fatal("queue truncated")
	}
	unlock, e := queueLock(dir)
	if e != nil {
		t.Fatal(e)
	}
	if status, e := QueueRead(dir); e != nil || status.Total != MaxQueueItems {
		unlock()
		t.Fatal("status blocked behind network worker lock")
	}
	if _, e = QueueRun(context.Background(), dir, 1, time.Now()); !errors.Is(e, ErrLocked) {
		unlock()
		t.Fatal("second worker acquired queue")
	}
	unlock()
}
func TestQueueBackoffIsBounded(t *testing.T) {
	if backoff(1) != 2*time.Second || backoff(2) != 4*time.Second || backoff(1000000) != 5*time.Minute {
		t.Fatal("unbounded scheduling")
	}
}

func TestQueueDecoderRejectsExcessiveNestingBeforeStackGrowth(t *testing.T) {
	var out any
	if e := decode([]byte(strings.Repeat("[", 10000)+strings.Repeat("]", 10000)), &out); e == nil {
		t.Fatal("unbounded nested state accepted")
	}
}
