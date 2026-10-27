package traderelay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sessionScopeFixture(t *testing.T, source Endpoint, hash string) (string, *QueueBinding) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session-context.json")
	raw, _ := json.Marshal(QueueScope{Version: 2, Source: source, SourceSessionHash: hash})
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	bound, e := BindQueueScope(path)
	if e != nil {
		t.Fatal(e)
	}
	return path, bound
}

func TestQueueScopeSessionExcludesSameActorOldAttemptWithoutReadingTokensOrProof(t *testing.T) {
	dir, i := queueFixture(t)
	i.Status = "uncertain"
	i.SourceTokenFile = "/never-read-old-session"
	i.TargetTokenFile = "/never-read-remote-session"
	if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
		t.Fatal(e)
	}
	proof := []byte(`{"original":"uncertain-result"}`)
	if e := os.WriteFile(statePath(dir, i.ID), proof, 0600); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(queuePath(dir))
	_, bound := sessionScopeFixture(t, i.Job.Source, digest("new-source-session"))
	view, e := bound.Read(dir)
	if e != nil || view.Total != 0 {
		t.Fatal(view, e)
	}
	batch, e := bound.Run(context.Background(), dir, 1, time.Now().Add(time.Hour))
	if e != nil || batch != (QueueBatch{}) {
		t.Fatal(batch, e)
	}
	after, _ := os.ReadFile(queuePath(dir))
	saved, _ := os.ReadFile(statePath(dir, i.ID))
	if string(before) != string(after) || string(proof) != string(saved) {
		t.Fatal("old uncertain operation changed")
	}
	// The exact original session remains visible, without exposing its binding.
	_, original := sessionScopeFixture(t, i.Job.Source, i.SourceSessionHash)
	view, e = original.Read(dir)
	if e != nil || view.Total != 1 || view.Items[0].ID != i.ID {
		t.Fatal(view, e)
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), i.SourceSessionHash) {
		t.Fatal("private session correlation exposed")
	}
}

func TestQueueScopeSessionReplacementStopsExistingBindingWithoutMutation(t *testing.T) {
	dir, i := queueFixture(t)
	if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
		t.Fatal(e)
	}
	path, bound := sessionScopeFixture(t, i.Job.Source, i.SourceSessionHash)
	before, _ := os.ReadFile(queuePath(dir))
	raw, _ := json.Marshal(QueueScope{Version: 2, Source: i.Job.Source, SourceSessionHash: digest("another-session-same-actor")})
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := bound.Read(dir); !errors.Is(e, ErrScopeChanged) {
		t.Fatal(e)
	}
	batch, e := bound.Run(context.Background(), dir, 1, time.Now())
	if !errors.Is(e, ErrScopeChanged) || batch != (QueueBatch{}) {
		t.Fatal(batch, e)
	}
	after, _ := os.ReadFile(queuePath(dir))
	if string(before) != string(after) {
		t.Fatal("rebound old attempt")
	}
}

func TestQueueScopeSessionStrictVersionedFormat(t *testing.T) {
	source, _ := json.Marshal(sampleJob().Source)
	for _, suffix := range []string{"", `,"source_session_sha256":null`, `,"source_session_sha256":""`, `,"source_session_sha256":"short"`, `,"source_session_sha256":"` + strings.Repeat("A", 64) + `"`, `,"source_session_sha256":"` + strings.Repeat("a", 64) + `","source_session_sha256":"` + strings.Repeat("a", 64) + `"`} {
		path := filepath.Join(t.TempDir(), "context.json")
		raw := `{"version":2,"source":` + string(source) + suffix + `}`
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := BindQueueScope(path); e == nil {
			t.Fatal("ambiguous session scope accepted")
		}
	}
	for _, value := range []string{`null`, `""`, `"` + strings.Repeat("a", 64) + `"`} {
		path := filepath.Join(t.TempDir(), "context.json")
		raw := `{"version":1,"source":` + string(source) + `,"source_session_sha256":` + value + `}`
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := BindQueueScope(path); e == nil {
			t.Fatal("session scope downgraded to identity-only filter")
		}
		raw = strings.Replace(raw, "source_session_sha256", "SOURCE_SESSION_SHA256", 1)
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := BindQueueScope(path); e == nil {
			t.Fatal("case alias downgraded to identity-only scope")
		}
	}
}
