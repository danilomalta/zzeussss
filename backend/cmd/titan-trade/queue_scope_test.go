package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/supplycycle"
	"titansystem-backend/internal/traderelay"
)

type scopeChangingWriter struct {
	bytes.Buffer
	change func()
}

func (w *scopeChangingWriter) Write(p []byte) (int, error) {
	n, e := w.Buffer.Write(p)
	if w.change != nil {
		f := w.change
		w.change = nil
		f()
	}
	return n, e
}
func TestQueueCLIScopeAndWatchContextReplacement(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "queue")
	scope := filepath.Join(base, "context.json")
	source := traderelay.Endpoint{URL: "http://127.0.0.1:12345/local/v1", Party: supplycycle.TradeParty{TenantID: "tenant", StoreID: "store", DeviceID: "device"}, ActorID: "actor", Fingerprint: strings.Repeat("1", 64)}
	raw, _ := json.Marshal(traderelay.QueueScope{Version: 1, Source: source})
	if e := os.WriteFile(scope, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := run(context.Background(), []string{"queue", "init", "--queue", dir}, &out); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if e := run(context.Background(), []string{"queue", "status", "--queue", dir, "--scope", scope}, &out); e != nil {
		t.Fatal(e)
	}
	var view traderelay.QueueView
	if e := json.Unmarshal(out.Bytes(), &view); e != nil || view.Total != 0 {
		t.Fatal(view, e)
	}
	w := &scopeChangingWriter{change: func() {
		source.ActorID = "new-actor"
		b, _ := json.Marshal(traderelay.QueueScope{Version: 1, Source: source})
		if e := os.WriteFile(scope, b, 0600); e != nil {
			t.Fatal(e)
		}
	}}
	e := run(context.Background(), []string{"queue", "watch", "--queue", dir, "--scope", scope}, w)
	if !errors.Is(e, traderelay.ErrScopeChanged) {
		t.Fatal("watch silently rebound", e)
	}
	if !strings.Contains(w.String(), `"watching":true`) || strings.Contains(w.String(), "new-actor") {
		t.Fatal("private context echoed")
	}
	for _, args := range [][]string{{"queue", "run", "--queue", dir, "--scope", ""}, {"queue", "run", "--queue", dir, "--scope="}, {"queue", "status", "--queue", dir, "--scope", "/missing"}, {"queue", "run", "--queue", dir, "--scope", "/missing"}, {"queue", "watch", "--queue", dir, "--scope", "/missing"}, {"queue", "init", "--queue", dir, "--scope", scope}} {
		out.Reset()
		if e = run(context.Background(), args, &out); e == nil || out.Len() != 0 {
			t.Fatal("invalid invocation accepted or echoed private data")
		}
	}
}
