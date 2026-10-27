package traderelay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func scopeFixture(t *testing.T, source Endpoint) (string, *QueueBinding) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "context.json")
	raw, _ := json.Marshal(QueueScope{Version: 1, Source: source})
	if e := os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	bound, e := BindQueueScope(file)
	if e != nil {
		t.Fatal(e)
	}
	return file, bound
}
func TestQueueScopeReadAndRunExcludeOtherActorsWithoutReadingTheirTokens(t *testing.T) {
	dir, own := queueFixture(t)
	other := own
	other.Job.Source.ActorID = "another-actor"
	other.Job.OperationID = "other-op"
	other.Job.MessageID = "other-message"
	other.ID = digest(other.Job)
	other.SourceTokenFile = "/never-read-other-token"
	other.TargetTokenFile = "/never-read-target-token"
	if e := saveQueue(dir, queueFile{1, []QueueItem{other, own}}); e != nil {
		t.Fatal(e)
	}
	_, bound := scopeFixture(t, own.Job.Source)
	view, e := bound.Read(dir)
	if e != nil || view.Total != 1 || view.Items[0].ID != own.ID {
		t.Fatal(view, e)
	}
	raw, _ := json.Marshal(view)
	for _, hidden := range []string{other.ID, other.Job.OperationID, other.SourceTokenFile, own.SourceSessionHash, own.SourceTokenFile} {
		if strings.Contains(string(raw), hidden) {
			t.Fatal("status leaked another context or private binding")
		}
	}
	// The own synthetic token changed, so only that item pauses, without network.
	batch, e := bound.Run(context.Background(), dir, 1, time.Now().Add(time.Hour))
	if e != nil || batch.Processed != 1 || batch.Paused != 1 || batch.Posts != 0 || batch.SourceCalls != 0 || batch.TargetCalls != 0 {
		t.Fatal(batch, e)
	}
	q, e := readQueue(dir)
	if e != nil || !reflect.DeepEqual(q.Items[0], other) || q.Items[1].Status != "paused" {
		t.Fatal("other context changed", e)
	}
}
func TestQueueScopeMatchesEverySourceDimensionAndEmptyReadNeverMutates(t *testing.T) {
	for _, which := range []string{"tenant", "store", "device", "actor", "url", "fingerprint"} {
		t.Run(which, func(t *testing.T) {
			dir, i := queueFixture(t)
			if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
				t.Fatal(e)
			}
			source := i.Job.Source
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
				source.URL = "https://another.example/local/v1"
			case "fingerprint":
				source.Fingerprint = strings.Repeat("9", 64)
			}
			_, bound := scopeFixture(t, source)
			before, _ := os.ReadFile(queuePath(dir))
			view, e := bound.Read(dir)
			if e != nil || view.Total != 0 || len(view.Items) != 0 {
				t.Fatal(view, e)
			}
			result, e := bound.Run(context.Background(), dir, 10, time.Now().Add(time.Hour))
			if e != nil || result != (QueueBatch{}) {
				t.Fatal(result, e)
			}
			after, _ := os.ReadFile(queuePath(dir))
			if string(before) != string(after) {
				t.Fatal("foreign queue records rewritten")
			}
		})
	}
}
func TestQueueScopeRejectsAmbiguousPublicAndSymbolicFiles(t *testing.T) {
	source, _ := json.Marshal(sampleJob().Source)
	valid := `{"version":1,"source":` + string(source) + `}`
	for _, raw := range []string{`null`, `{}`, strings.Replace(valid, `"version":1`, `"version":2`, 1), strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1), strings.TrimSuffix(valid, "}") + `,"token":"do-not-accept"}`, strings.Replace(valid, `"actor_id":`, `"actor_id":"other","actor_id":`, 1), valid + ` {}`, `{"version":1,"source":null}`} {
		file := filepath.Join(t.TempDir(), "context.json")
		if e := os.WriteFile(file, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := BindQueueScope(file); e == nil {
			t.Fatal("malformed scope accepted")
		}
	}
	file, _ := scopeFixture(t, sampleJob().Source)
	if e := os.Chmod(file, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := BindQueueScope(file); e == nil {
		t.Fatal("public scope accepted")
	}
	os.Chmod(file, 0600)
	link := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(file, link); e != nil {
		t.Fatal(e)
	}
	if _, e := BindQueueScope(link); e == nil {
		t.Fatal("symlink scope accepted")
	}
	if _, e := BindQueueScope("relative.json"); e == nil {
		t.Fatal("relative scope accepted")
	}
}
func TestQueueScopeChangedOrRemovedNeverRebindsRunningWorker(t *testing.T) {
	for _, which := range []string{"actor", "removed", "public", "invalid"} {
		t.Run(which, func(t *testing.T) {
			dir, i := queueFixture(t)
			if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
				t.Fatal(e)
			}
			file, bound := scopeFixture(t, i.Job.Source)
			before, _ := os.ReadFile(queuePath(dir))
			switch which {
			case "actor":
				source := i.Job.Source
				source.ActorID = "new-actor"
				raw, _ := json.Marshal(QueueScope{Version: 1, Source: source})
				os.WriteFile(file, raw, 0600)
			case "removed":
				os.Remove(file)
			case "public":
				os.Chmod(file, 0644)
			case "invalid":
				os.WriteFile(file, []byte(`{}`), 0600)
			}
			if _, e := bound.Read(dir); !errors.Is(e, ErrScopeChanged) {
				t.Fatal("read rebound", e)
			}
			result, e := bound.Run(context.Background(), dir, 10, time.Now())
			if !errors.Is(e, ErrScopeChanged) || result != (QueueBatch{}) {
				t.Fatal(result, e)
			}
			after, _ := os.ReadFile(queuePath(dir))
			if string(before) != string(after) {
				t.Fatal("context replacement altered old attempt")
			}
		})
	}
}
func TestQueueScopeInvalidArgumentsDoNotPanic(t *testing.T) {
	var nilBinding *QueueBinding
	if _, e := nilBinding.Read("/not-read"); e == nil {
		t.Fatal("nil accepted")
	}
	if _, e := nilBinding.Run(context.Background(), "/not-read", 1, time.Now()); e == nil {
		t.Fatal("nil accepted")
	}
	dir, i := queueFixture(t)
	_, bound := scopeFixture(t, i.Job.Source)
	if _, e := bound.Run(nil, dir, 1, time.Now()); e == nil {
		t.Fatal("nil context accepted")
	}
}
