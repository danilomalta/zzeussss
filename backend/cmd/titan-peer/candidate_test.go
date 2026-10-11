package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/stationfile"
)

func TestCandidateCreatesIndependentPrivateKeysAndPreservesExistingFiles(t *testing.T) {
	f := newPeerFixture(t)
	p := publicPairFiles(t, f)
	dir := filepath.Join(t.TempDir(), "candidate")
	args := []string{"candidate-init", "--parent", p.descriptor, "--dir", dir, "--signing-sha256", p.prints.SigningSHA256, "--encryption-sha256", p.prints.EncryptionSHA256}
	var out bytes.Buffer
	wrong := append([]string(nil), args...)
	wrong[len(wrong)-1] = "wrong"
	if runPeer(context.Background(), wrong, nil, &out) == nil {
		t.Fatal("unverified parent accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("created state before verification")
	}
	if err := runPeer(context.Background(), args, nil, &out); err != nil {
		t.Fatal(err)
	}
	var station stationfile.Station
	raw, err := os.ReadFile(filepath.Join(dir, "device.station"))
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &station) != nil {
		t.Fatal("invalid station")
	}
	key, err := base64.RawURLEncoding.DecodeString(station.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicRaw, err := os.ReadFile(filepath.Join(dir, "peer-public.json"))
	if err != nil {
		t.Fatal(err)
	}
	descriptor, _, err := incoming.ParsePublicDescriptor(publicRaw)
	if err != nil {
		t.Fatal(err)
	}
	if station.DeviceID == "candidate" || station.TenantID != f.result.TenantID || !bytes.Equal(descriptor.SigningPublicKey, ed25519.PrivateKey(key).Public().(ed25519.PublicKey)) {
		t.Fatal("identity/key mismatch")
	}
	for _, name := range []string{"device.station", "peer.encryption", "peer-public.json"} {
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatal("not private regular file")
		}
	}
	if bytes.Contains(publicRaw, []byte(station.PrivateKey)) || strings.Contains(out.String(), station.PrivateKey) || strings.Contains(out.String(), peerTestPassword) {
		t.Fatal("private key/password exposed")
	}
	if peerCount(t, f.db, "device_pairings") != 1 {
		t.Fatal("candidate implicitly approved")
	}
	if runPeer(context.Background(), args, nil, &out) == nil {
		t.Fatal("existing keys replaced")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "device.station"))
	if !bytes.Equal(raw, after) {
		t.Fatal("keys changed")
	}
}

func TestPairInspectValidatesProofWithoutApprovalOrPrivateContent(t *testing.T) {
	f := newPeerFixture(t)
	p := publicPairFiles(t, f)
	startAndAnswerPair(t, f, p)
	for _, path := range []string{p.challenge, p.answer} {
		var out bytes.Buffer
		if err := runPeer(context.Background(), []string{"pair-inspect", "--in", path}, nil, &out); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if json.Unmarshal(out.Bytes(), &result) != nil || result["approved"] != false || result["requester_sha256"] == "" {
			t.Fatal("misleading inspection")
		}
		if strings.Contains(out.String(), "private_key") {
			t.Fatal("secret output")
		}
	}
	if peerCount(t, f.db, "sync_incoming_peers") != 0 {
		t.Fatal("implicit event grant")
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if os.WriteFile(invalid, []byte(`{"version":1,"proof":"fake"}`), 0600) != nil {
		t.Fatal("write")
	}
	if runPeer(context.Background(), []string{"pair-inspect", "--in", invalid}, nil, &bytes.Buffer{}) == nil {
		t.Fatal("fake proof accepted")
	}
}

func TestCandidateRejectsSymlinkParentBeforeCreatingPrivateFiles(t *testing.T) {
	f := newPeerFixture(t)
	p := publicPairFiles(t, f)
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	if os.Mkdir(actual, 0700) != nil {
		t.Fatal("mkdir")
	}
	link := filepath.Join(root, "link")
	if os.Symlink(actual, link) != nil {
		t.Fatal("symlink")
	}
	for _, dir := range []string{filepath.Join(link, "candidate"), link + "/../candidate"} {
		args := []string{"candidate-init", "--parent", p.descriptor, "--dir", dir, "--signing-sha256", p.prints.SigningSHA256, "--encryption-sha256", p.prints.EncryptionSHA256}
		if runPeer(context.Background(), args, nil, &bytes.Buffer{}) == nil {
			t.Fatal("unsafe directory accepted")
		}
	}
	entries, err := os.ReadDir(actual)
	if err != nil || len(entries) != 0 {
		t.Fatal("created redirected keys")
	}
}
