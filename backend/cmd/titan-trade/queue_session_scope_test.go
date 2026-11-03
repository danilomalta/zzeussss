package main

import (
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

func TestQueueCLISessionScopeWatchStopsOnSameActorNewSession(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "queue")
	path := filepath.Join(base, "scope.json")
	source := traderelay.Endpoint{URL: "http://127.0.0.1:12345/local/v1", Party: supplycycle.TradeParty{TenantID: "tenant", StoreID: "store", DeviceID: "device"}, ActorID: "actor", Fingerprint: strings.Repeat("1", 64)}
	scope := traderelay.QueueScope{Version: 2, Source: source, SourceSessionHash: strings.Repeat("a", 64)}
	save := func() {
		t.Helper()
		raw, _ := json.Marshal(scope)
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	save()
	if e := traderelay.QueueInit(dir); e != nil {
		t.Fatal(e)
	}
	w := &scopeChangingWriter{change: func() { scope.SourceSessionHash = strings.Repeat("b", 64); save() }}
	e := run(context.Background(), []string{"queue", "watch", "--queue", dir, "--scope", path}, w)
	if !errors.Is(e, traderelay.ErrScopeChanged) {
		t.Fatal("same actor new session rebound worker", e)
	}
	for _, private := range []string{scope.SourceSessionHash, source.ActorID, source.Party.TenantID, path} {
		if strings.Contains(w.String(), private) {
			t.Fatal("watch output exposed private context")
		}
	}
}
