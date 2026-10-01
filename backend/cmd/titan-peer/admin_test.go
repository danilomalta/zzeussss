package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
)

func administrativePeer(t *testing.T, f peerFixture) (string, incoming.PublicFingerprints) {
	t.Helper()
	ctx := context.Background()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.Scope{TenantID: f.result.TenantID, StoreID: f.result.StoreID, IdentityID: f.result.OwnerID}
	challenge, err := identity.RequestPairing(ctx, f.db, actor, "remote", "Outro aparelho", public)
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.ProvePairing(ctx, f.db, f.result.TenantID, "remote", ed25519.Sign(key, identity.PairingMessage(f.result.TenantID, f.result.StoreID, "remote", challenge))); err != nil {
		t.Fatal(err)
	}
	if err := identity.ApprovePairing(ctx, f.db, actor, "remote"); err != nil {
		t.Fatal(err)
	}
	encryption, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	device := identity.DeviceContext{TenantID: f.result.TenantID, StoreID: f.result.StoreID, DeviceID: "remote"}
	descriptor, err := incoming.CreatePublicDescriptor(device, 1, encryption, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	_, prints, err := incoming.ParsePublicDescriptor(raw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(f.dbPath), "remote-public.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, prints
}
func adminArguments(command string, f peerFixture, path string, prints incoming.PublicFingerprints, events string) []string {
	args := []string{command, "--db", f.dbPath, "--station", f.stationPath, "--owner", f.result.OwnerID, "--in", path, "--signing-sha256", prints.SigningSHA256, "--encryption-sha256", prints.EncryptionSHA256}
	if events != "" {
		args = append(args, "--events", events)
	}
	return args
}

func TestPeerAdminAuthenticatesOwnerAndSeparatesTrustGrantRevoke(t *testing.T) {
	f := newPeerFixture(t)
	path, prints := administrativePeer(t, f)
	var output bytes.Buffer
	ctx := context.Background()
	trust := adminArguments("trust", f, path, prints, "")
	if err := runPeer(ctx, trust, strings.NewReader("wrong-password"), &output); err == nil {
		t.Fatal("wrong password accepted")
	}
	wrong := prints
	wrong.EncryptionSHA256 = "wrong"
	if err := runPeer(ctx, adminArguments("trust", f, path, wrong, ""), strings.NewReader(peerTestPassword), &output); err == nil {
		t.Fatal("unchecked fingerprint accepted")
	}
	if peerCount(t, f.db, "device_encryption_keys") != 0 {
		t.Fatal("failed trust wrote key")
	}
	if err := runPeer(ctx, trust, strings.NewReader(peerTestPassword), &output); err != nil {
		t.Fatal(err)
	}
	if peerCount(t, f.db, "sync_incoming_peers") != 0 {
		t.Fatal("trust granted reception")
	}
	if err := runPeer(ctx, adminArguments("approve", f, path, prints, "sale.committed,stock.operation"), strings.NewReader(peerTestPassword), &output); err != nil {
		t.Fatal(err)
	}
	if peerCount(t, f.db, "sync_incoming_peers") != 2 {
		t.Fatal("missing event grants")
	}
	if err := runPeer(ctx, adminArguments("revoke", f, path, prints, "sale.committed"), strings.NewReader(peerTestPassword), &output); err != nil {
		t.Fatal(err)
	}
	var active int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM sync_incoming_peers WHERE status='active'").Scan(&active); err != nil || active != 1 {
		t.Fatal("revocation affected unselected event")
	}
	if strings.Contains(output.String(), peerTestPassword) || strings.Contains(output.String(), "private_key") {
		t.Fatal("secret printed")
	}
}

func TestPeerAdminRejectsInvalidEventsAndManager(t *testing.T) {
	f := newPeerFixture(t)
	path, prints := administrativePeer(t, f)
	ctx := context.Background()
	for _, events := range []string{"", "unknown", "sale.committed,sale.committed"} {
		if err := runPeer(ctx, adminArguments("approve", f, path, prints, events), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err == nil {
			t.Fatal("invalid event list accepted")
		}
	}
	if _, err := f.db.Exec("UPDATE memberships SET role='manager'"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO membership_stores (tenant_id,identity_id,store_id) VALUES (?,?,?)", f.result.TenantID, f.result.OwnerID, f.result.StoreID); err != nil {
		t.Fatal(err)
	}
	if err := runPeer(ctx, adminArguments("approve", f, path, prints, "sale.committed"), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err == nil {
		t.Fatal("manager approved peer")
	}
	if peerCount(t, f.db, "device_encryption_keys") != 0 || peerCount(t, f.db, "sync_incoming_peers") != 0 {
		t.Fatal("manager changed trust")
	}
}
