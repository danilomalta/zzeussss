package traderelay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func backupQueueFixture(t *testing.T) (string, QueueItem, []byte, string) {
	t.Helper()
	dir, i := queueFixture(t)
	if e := saveQueue(dir, queueFile{1, []QueueItem{i}}); e != nil {
		t.Fatal(e)
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	return dir, i, key, filepath.Join(t.TempDir(), "queue.backup")
}
func TestQueueBackupRoundTripPrivateAndReceiptOnly(t *testing.T) {
	dir, i, key, archive := backupQueueFixture(t)
	before, _ := os.ReadFile(queuePath(dir))
	info, e := QueueBackup(dir, key, archive)
	if e != nil || info.Items != 1 || info.EngineStates != 0 {
		t.Fatalf("backup %+v %v", info, e)
	}
	after, _ := os.ReadFile(queuePath(dir))
	if !bytes.Equal(before, after) {
		t.Fatal("source queue changed")
	}
	encrypted, _ := os.ReadFile(archive)
	if bytes.Contains(encrypted, []byte(i.ID)) || bytes.Contains(encrypted, []byte("new-source-session")) {
		t.Fatal("unencrypted metadata/token")
	}
	s, raw, e := loadQueueBackup(archive, key)
	if e != nil || len(s.Queue.Items) != 1 || bytes.Contains(raw, []byte("new-source-session")) || bytes.Contains(raw, []byte("target-session\"")) {
		t.Fatal("token copied")
	}
	checked, e := QueueVerifyBackup(archive, key)
	if e != nil || checked != info {
		t.Fatal("verify differs")
	}
	dest := filepath.Join(t.TempDir(), "restored")
	restored, e := QueueRestore(archive, key, dest)
	if e != nil || restored.SnapshotSHA256 != info.SnapshotSHA256 {
		t.Fatal(e)
	}
	view, e := QueueRead(dest)
	if e != nil || !view.Items[0].RecoveryOnly || view.Items[0].Status != "paused" || view.Items[0].OperationID != i.Job.OperationID {
		t.Fatalf("unsafe restoration %+v %v", view, e)
	}
	b, e := QueueRun(context.Background(), dest, 10, time.Now().Add(time.Hour))
	if e != nil || b.Processed != 0 || b.Posts != 0 {
		t.Fatal("restored queue auto-ran")
	}
	if _, e = QueueRetry(context.Background(), dest, i.ID, time.Now()); e == nil {
		t.Fatal("restored queue retry allowed")
	}
	st, _ := os.Stat(dest)
	f, _ := os.Stat(queuePath(dest))
	a, _ := os.Stat(archive)
	if st.Mode().Perm() != 0700 || f.Mode().Perm() != 0600 || a.Mode().Perm() != 0600 {
		t.Fatal("unsafe permissions")
	}
	if _, e = QueueRestore(archive, key, dest); e == nil {
		t.Fatal("existing directory overwritten")
	}
	if _, e = QueueBackup(dir, key, archive); e == nil {
		t.Fatal("existing archive overwritten")
	}
}
func TestQueueBackupRejectsTamperWrongKeyAndPublicFiles(t *testing.T) {
	dir, _, key, archive := backupQueueFixture(t)
	if _, e := QueueBackup(dir, key, archive); e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(archive)
	wrong := make([]byte, 32)
	if _, e := QueueVerifyBackup(archive, wrong); e == nil {
		t.Fatal("wrong key accepted")
	}
	cases := [][]byte{nil, []byte("TYTQBAK1"), append([]byte(nil), original...), append([]byte(nil), original...)}
	cases[2][len(original)-1] ^= 1
	cases[3][0] ^= 1
	for n, b := range cases {
		if e := os.WriteFile(archive, b, 0600); e != nil {
			t.Fatal(e)
		}
		dest := filepath.Join(t.TempDir(), "must-not-exist")
		if _, e := QueueRestore(archive, key, dest); e == nil {
			t.Fatalf("invalid case%d accepted", n)
		}
		if _, e := os.Lstat(dest); !os.IsNotExist(e) {
			t.Fatal("invalid archive created destination")
		}
	}
	_ = os.WriteFile(archive, original, 0600)
	_ = os.Chmod(archive, 0644)
	if _, e := QueueVerifyBackup(archive, key); e == nil {
		t.Fatal("public archive accepted")
	}
	_ = os.Chmod(archive, 0600)
	link := filepath.Join(t.TempDir(), "link")
	_ = os.Symlink(archive, link)
	if _, e := QueueVerifyBackup(link, key); e == nil {
		t.Fatal("symlink archive accepted")
	}
}
func TestQueueSnapshotValidationAndLocks(t *testing.T) {
	dir, i, key, archive := backupQueueFixture(t)
	i.Status = "confirmed"
	i.NextAttemptAt = ""
	_ = saveQueue(dir, queueFile{1, []QueueItem{i}})
	if _, e := QueueBackup(dir, key, archive); e == nil {
		t.Fatal("missing confirmed proof accepted")
	}
	i.Status = "queued"
	i.NextAttemptAt = i.CreatedAt
	_ = saveQueue(dir, queueFile{1, []QueueItem{i}})
	release, e := lockState(statePath(dir, i.ID) + ".lock")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = QueueBackup(dir, key, archive); e != ErrLocked {
		t.Fatalf("active state not locked: %v", e)
	}
	release()
	if _, e = QueueBackup(dir, key, archive); e != nil {
		t.Fatal(e)
	}
	s, _, e := loadQueueBackup(archive, key)
	if e != nil {
		t.Fatal(e)
	}
	s.States["../escape"] = State{}
	if validateSnapshot(s) == nil {
		t.Fatal("unknown/path state accepted")
	}
	delete(s.States, "../escape")
	s.States[i.ID] = State{Version: 1, Status: "confirmed", JobSHA256: i.ID, EnvelopeSHA256: i.EnvelopeHash, SourcePublicKey: make([]byte, 32)}
	if validateSnapshot(s) == nil {
		t.Fatal("wrong pinned public key accepted")
	}
	out := filepath.Join(t.TempDir(), "destination")
	_ = os.Mkdir(out, 0700)
	_ = os.WriteFile(filepath.Join(out, "important"), []byte("keep"), 0600)
	staging := filepath.Join(t.TempDir(), "staging")
	_ = os.Mkdir(staging, 0700)
	if e = publishQueueDirectory(staging, out); e == nil {
		t.Fatal("destination replaced")
	}
	got, _ := os.ReadFile(filepath.Join(out, "important"))
	if string(got) != "keep" {
		t.Fatal("original lost")
	}
}
func TestQueueRestoredFlagSurvivesJSONAndPause(t *testing.T) {
	dir, i, _, _ := backupQueueFixture(t)
	i.RecoveryOnly = true
	i.Status = "paused"
	i.NextAttemptAt = ""
	i.LastAction = "restored_snapshot"
	i.LastError = "restored_queue_review_required"
	_ = saveQueue(dir, queueFile{1, []QueueItem{i}})
	if e := QueuePause(dir, i.ID); e != nil {
		t.Fatal(e)
	}
	q, e := readQueue(dir)
	if e != nil || !q.Items[0].RecoveryOnly {
		t.Fatal("barrier lost")
	}
	raw, _ := json.Marshal(itemView(q.Items[0]))
	if !bytes.Contains(raw, []byte(`"recovery_only":true`)) {
		t.Fatal("barrier hidden from operator")
	}
}
