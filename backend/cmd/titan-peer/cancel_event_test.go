package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPeerAdminCancellationGrantMustBeExplicit(t *testing.T) {
	f := newPeerFixture(t)
	path, prints := administrativePeer(t, f)
	ctx := context.Background()
	call := func(events string) error {
		return runPeer(ctx, adminArguments("approve", f, path, prints, events), strings.NewReader(peerTestPassword), &bytes.Buffer{})
	}
	if err := call("sale.committed"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM sync_incoming_peers WHERE event_type='sale.cancelled'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("aprovacao automatica: %d %v", count, err)
	}
	if err := call("sale.cancelled,sale.cancelled"); err == nil {
		t.Fatal("duplicidade aceita")
	}
	if err := call("sale.cancelled"); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM sync_incoming_peers WHERE event_type='sale.cancelled' AND status='active'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("aprovacao: %d %v", count, err)
	}
}
