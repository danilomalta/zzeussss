package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/stationfile"
)

type pairingFiles struct {
	descriptor, station, challenge, answer string
	prints                                 incoming.PublicFingerprints
	key                                    ed25519.PrivateKey
}

func publicPairFiles(t *testing.T, f peerFixture) pairingFiles {
	t.Helper()
	dir := filepath.Dir(f.dbPath)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encryption, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	target := identity.DeviceContext{TenantID: f.result.TenantID, StoreID: f.result.StoreID, DeviceID: "candidate"}
	descriptor, err := incoming.CreatePublicDescriptor(target, 1, encryption, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(descriptor)
	_, prints, err := incoming.ParsePublicDescriptor(raw)
	if err != nil {
		t.Fatal(err)
	}
	files := pairingFiles{descriptor: filepath.Join(dir, "candidate-public.json"), station: filepath.Join(dir, "candidate-private.json"), challenge: filepath.Join(dir, "challenge-public.json"), answer: filepath.Join(dir, "answer-public.json"), prints: prints, key: key}
	if err := os.WriteFile(files.descriptor, raw, 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(stationfile.Station{TenantID: target.TenantID, StoreID: target.StoreID, DeviceID: target.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(key)})
	if err := os.WriteFile(files.station, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return files
}
func pairStartArgs(f peerFixture, p pairingFiles) []string {
	return []string{"pair-start", "--db", f.dbPath, "--station", f.stationPath, "--owner", f.result.OwnerID, "--in", p.descriptor, "--out", p.challenge, "--name", "Segundo aparelho", "--signing-sha256", p.prints.SigningSHA256, "--encryption-sha256", p.prints.EncryptionSHA256}
}
func pairFinishArgs(f peerFixture, p pairingFiles) []string {
	return []string{"pair-finish", "--db", f.dbPath, "--station", f.stationPath, "--owner", f.result.OwnerID, "--in", p.answer, "--signing-sha256", p.prints.SigningSHA256, "--encryption-sha256", p.prints.EncryptionSHA256}
}
func pairAnswerArgs(t *testing.T, p pairingFiles) []string {
	t.Helper()
	raw, err := os.ReadFile(p.challenge)
	if err != nil {
		t.Fatal(err)
	}
	request, err := incoming.ParsePairChallenge(raw)
	if err != nil {
		t.Fatal(err)
	}
	return []string{"pair-answer", "--station", p.station, "--in", p.challenge, "--out", p.answer, "--requester-sha256", incoming.PairRequesterFingerprint(request)}
}
func startAndAnswerPair(t *testing.T, f peerFixture, p pairingFiles) {
	t.Helper()
	ctx := context.Background()
	if err := runPeer(ctx, pairStartArgs(f, p), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := runPeer(ctx, pairAnswerArgs(t, p), nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestPairCLIChallengeProofAndOwnerApprovalRemainSeparate(t *testing.T) {
	f := newPeerFixture(t)
	p := publicPairFiles(t, f)
	ctx := context.Background()
	var output bytes.Buffer
	if err := runPeer(ctx, pairStartArgs(f, p), strings.NewReader(peerTestPassword), &output); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := f.db.QueryRow("SELECT status FROM device_pairings WHERE device_id='candidate'").Scan(&status); err != nil || status != "pending" {
		t.Fatal("start approved device")
	}
	answerArgs := pairAnswerArgs(t, p)
	wrong := append([]string(nil), answerArgs...)
	wrong[len(wrong)-1] = "wrong"
	if err := runPeer(ctx, wrong, nil, &output); err == nil {
		t.Fatal("unverified requester accepted")
	}
	wrong = append([]string(nil), answerArgs...)
	wrong[2] = f.stationPath
	if err := runPeer(ctx, wrong, nil, &output); err == nil {
		t.Fatal("wrong private station signed proof")
	}
	if err := runPeer(ctx, answerArgs, nil, &output); err != nil {
		t.Fatal(err)
	}
	if err := runPeer(ctx, answerArgs, nil, &output); err == nil {
		t.Fatal("public answer overwritten")
	}
	if err := f.db.QueryRow("SELECT status FROM device_pairings WHERE device_id='candidate'").Scan(&status); err != nil || status != "pending" {
		t.Fatal("proof approved without owner")
	}
	finish := pairFinishArgs(f, p)
	if err := runPeer(ctx, finish, strings.NewReader("wrong-password"), &output); err == nil {
		t.Fatal("wrong owner password accepted")
	}
	for i := 0; i < 2; i++ {
		if err := runPeer(ctx, finish, strings.NewReader(peerTestPassword), &output); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.QueryRow("SELECT status FROM device_pairings WHERE device_id='candidate'").Scan(&status); err != nil || status != "approved" {
		t.Fatal("missing approval")
	}
	if peerCount(t, f.db, "device_pairing_events") != 3 || peerCount(t, f.db, "sync_incoming_peers") != 0 || peerCount(t, f.db, "device_encryption_keys") != 0 {
		t.Fatal("duplicate audit or implicit grants")
	}
	for _, path := range []string{p.challenge, p.answer} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "private_key") || strings.Contains(string(raw), base64.RawURLEncoding.EncodeToString(p.key)) {
			t.Fatal("private key exported")
		}
	}
	if strings.Contains(output.String(), peerTestPassword) {
		t.Fatal("password printed")
	}
}

func TestPairCLIFinalAuditFailureRollsBackProofAndApproval(t *testing.T) {
	f := newPeerFixture(t)
	p := publicPairFiles(t, f)
	startAndAnswerPair(t, f, p)
	if _, err := f.db.Exec("CREATE TRIGGER reject_pair_approval BEFORE INSERT ON device_pairing_events WHEN NEW.action='approved' BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := runPeer(context.Background(), pairFinishArgs(f, p), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err == nil {
		t.Fatal("audit failure hidden")
	}
	var status string
	if err := f.db.QueryRow("SELECT status FROM device_pairings WHERE device_id='candidate'").Scan(&status); err != nil || status != "pending" || peerCount(t, f.db, "device_pairing_events") != 1 {
		t.Fatal("partial proof approval persisted")
	}
}

func TestPairCLIRevocationExpiryAndManagerBlockApproval(t *testing.T) {
	for _, scenario := range []string{"owner", "candidate", "local", "expiry", "manager"} {
		t.Run(scenario, func(t *testing.T) {
			f := newPeerFixture(t)
			p := publicPairFiles(t, f)
			startAndAnswerPair(t, f, p)
			switch scenario {
			case "owner":
				if _, err := f.db.Exec("UPDATE memberships SET status='revoked'"); err != nil {
					t.Fatal(err)
				}
			case "candidate":
				if _, err := f.db.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id='candidate'"); err != nil {
					t.Fatal(err)
				}
			case "local":
				if _, err := f.db.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id=?", f.result.DeviceID); err != nil {
					t.Fatal(err)
				}
			case "manager":
				if _, err := f.db.Exec("UPDATE memberships SET role='manager'"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec("INSERT INTO membership_stores VALUES (?,?,?)", f.result.TenantID, f.result.OwnerID, f.result.StoreID); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				raw, err := os.ReadFile(p.answer)
				if err != nil {
					t.Fatal(err)
				}
				answer, err := incoming.ParsePairAnswer(raw)
				if err != nil {
					t.Fatal(err)
				}
				actor := identity.Scope{TenantID: f.result.TenantID, StoreID: f.result.StoreID, IdentityID: f.result.OwnerID}
				expires := time.Now().Unix() - 1
				if _, err := f.db.Exec("UPDATE device_pairings SET challenge_expires_unix=? WHERE device_id='candidate'", expires); err != nil {
					t.Fatal(err)
				}
				target := answer.Request.Target.Binding.Device
				if err := identity.CompleteOwnedPairing(context.Background(), f.db, actor, f.device(), target, ed25519.PublicKey(answer.Request.Target.SigningPublicKey), answer.Request.Challenge, answer.Proof, expires); err == nil {
					t.Fatal("expired challenge accepted")
				}
			}
			if err := runPeer(context.Background(), pairFinishArgs(f, p), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err == nil {
				t.Fatal("unauthorized approval")
			}
		})
	}
}
