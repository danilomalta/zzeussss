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
	"titansystem-backend/internal/localdb/supplycycle"
)

func sampleJob() Job {
	return Job{Version: 1, Source: Endpoint{URL: "http://127.0.0.1:8181/local/v1", Party: party("buyer"), ActorID: "owner", Fingerprint: strings.Repeat("a", 64)}, Target: Endpoint{URL: "https://supplier.example/local/v1", Party: party("seller"), ActorID: "owner", Fingerprint: strings.Repeat("b", 64)}, MessageID: "message", PeerID: "customer", OperationID: "original-operation"}
}
func party(tenant string) (p supplycycle.TradeParty) {
	p.TenantID = tenant
	p.StoreID = "store"
	p.DeviceID = "station"
	return
}

func TestJobRefusesUnsafeDestinationsAndAmbiguousIdentity(t *testing.T) {
	for _, raw := range []string{"http://supplier.example/local/v1", "http://127.1/local/v1", "https://x/local/v1?token=secret", "https://user:secret@x/local/v1", "https://x/local/v1#fragment", "https://x/local/v1/", "file:///local/v1", "https://x/local%2fv1", "http://localhost/local/v1", "https://x/local/v1\n"} {
		t.Run(raw, func(t *testing.T) {
			j := sampleJob()
			j.Target.URL = raw
			if j.Validate() == nil {
				t.Fatal("unsafe endpoint accepted")
			}
		})
	}
	j := sampleJob()
	if j.Validate() != nil {
		t.Fatal("valid job refused")
	}
	j.Target.Party.TenantID = j.Source.Party.TenantID
	if j.Validate() == nil {
		t.Fatal("same company accepted")
	}
	j = sampleJob()
	j.MessageID = "../other"
	if j.Validate() == nil {
		t.Fatal("path accepted")
	}
	j = sampleJob()
	j.Target.Fingerprint = strings.Repeat("A", 64)
	if j.Validate() == nil {
		t.Fatal("noncanonical fingerprint accepted")
	}
}
func TestPrivateInputsAndStrictJobJSON(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "job")
	b, _ := json.Marshal(sampleJob())
	if e := os.WriteFile(name, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJob(name); e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{strings.Replace(string(b), `"version":1`, `"version":1,"version":1`, 1), strings.Replace(string(b), `"actor_id":"owner"`, `"actor_id":"owner","actor_id":"other"`, 1), string(b) + `{}`, strings.Replace(string(b), `"version":1`, `"version":1,"unknown":true`, 1)} {
		if e := os.WriteFile(name, []byte(text), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadJob(name); e == nil {
			t.Fatal("ambiguous job accepted")
		}
	}
	if e := os.WriteFile(name, b, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(name, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJob(name); e == nil {
		t.Fatal("public job accepted")
	}
	if e := os.Chmod(name, 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(dir, "link")
	if e := os.Symlink(name, link); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJob(link); e == nil {
		t.Fatal("symlink accepted")
	}
	token := filepath.Join(dir, "token")
	for _, body := range []string{"", "two tokens", "a\nb", strings.Repeat("x", 4097)} {
		if e := os.WriteFile(token, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadToken(token); e == nil {
			t.Fatal("invalid token accepted")
		}
	}
	_ = os.WriteFile(token, []byte("synthetic-token\n"), 0600)
	if got, e := ReadToken(token); e != nil || got != "synthetic-token" {
		t.Fatal("token newline handling")
	}
}
func TestStateLockPersistsAcrossRecoveryAndExcludesConcurrentCommands(t *testing.T) {
	name := filepath.Join(t.TempDir(), "attempt.lock")
	release, e := lockState(name)
	if e != nil {
		t.Fatal(e)
	}
	if another, e := lockState(name); e == nil {
		another()
		t.Fatal("concurrent lock acquired")
	}
	release()
	release, e = lockState(name)
	if e != nil {
		t.Fatal(e)
	}
	release()
	target := filepath.Join(t.TempDir(), "target")
	_ = os.WriteFile(target, []byte("preserve"), 0600)
	link := filepath.Join(t.TempDir(), "link")
	_ = os.Symlink(target, link)
	if release, e = lockState(link); e == nil {
		release()
		t.Fatal("symlink lock followed")
	}
}
func TestSavedAttemptNeverOverwritesUnsafeFileOrUsesChangedJob(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	state := filepath.Join(dir, "state")
	j := sampleJob()
	s := State{Version: 1, JobSHA256: digest(j), EnvelopeSHA256: strings.Repeat("a", 64), Status: "uncertain"}
	if e := save(state, s); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(state)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, e := Run(ctx, j, "source", "target", state, "send"); !errors.Is(e, ErrUncertain) {
		t.Fatalf("send retried existing attempt: %v", e)
	}
	j.OperationID = "different"
	if _, e := Run(ctx, j, "source", "target", state, "retry"); !errors.Is(e, ErrInvalid) {
		t.Fatalf("job change accepted: %v", e)
	}
	after, _ := os.ReadFile(state)
	if string(after) != string(b) {
		t.Fatal("saved attempt changed")
	}
	_ = os.Chmod(state, 0644)
	if e := save(state, s); e == nil {
		t.Fatal("public state overwritten")
	}
}
