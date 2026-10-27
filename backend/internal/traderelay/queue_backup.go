package traderelay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
	"titansystem-backend/internal/localdb/supplycycle"
)

const maxQueueSnapshot = 8 << 20
const queueBackupMagic = "TYTQBAK1"

var queueBackupAAD = []byte("titan-trade-queue-backup-v1\x00")

type queueSnapshot struct {
	Version   int              `json:"version"`
	CreatedAt string           `json:"created_at"`
	Queue     queueFile        `json:"queue"`
	States    map[string]State `json:"states"`
}
type QueueBackupInfo struct {
	Version           int    `json:"version"`
	Scope             string `json:"scope"`
	Items             int    `json:"items"`
	EngineStates      int    `json:"engine_states"`
	RecordedConfirmed int    `json:"recorded_confirmed"`
	Uncertain         int    `json:"uncertain"`
	SnapshotSHA256    string `json:"snapshot_sha256"`
}

func validateSnapshot(s queueSnapshot) error {
	if s.Version != 1 || !queueTime(s.CreatedAt) || validateQueue(s.Queue) != nil || s.States == nil || len(s.States) > len(s.Queue.Items) {
		return ErrInvalid
	}
	known := map[string]bool{}
	for _, i := range s.Queue.Items {
		known[i.ID] = true
		st, ok := s.States[i.ID]
		if !ok {
			if i.Status == "confirmed" || i.Status == "uncertain" || i.Status == "needs_retry" {
				return ErrInvalid
			}
			continue
		}
		if st.Version != 1 || st.JobSHA256 != i.ID || st.EnvelopeSHA256 != i.EnvelopeHash || (st.Status != "confirmed" && st.Status != "uncertain") || len(st.SourcePublicKey) != ed25519.PublicKeySize || supplycycle.TradeFingerprint(st.SourcePublicKey) != i.Job.Source.Fingerprint || (i.Status == "confirmed" && st.Status != "confirmed") {
			return ErrInvalid
		}
	}
	for id := range s.States {
		if !known[id] {
			return ErrInvalid
		}
	}
	return nil
}
func snapshotInfo(s queueSnapshot, raw []byte) QueueBackupInfo {
	h := sha256.Sum256(raw)
	info := QueueBackupInfo{Version: 1, Scope: "local_trade_queue_v1", Items: len(s.Queue.Items), EngineStates: len(s.States), SnapshotSHA256: hex.EncodeToString(h[:])}
	for _, st := range s.States {
		if st.Status == "confirmed" {
			info.RecordedConfirmed++
		} else {
			info.Uncertain++
		}
	}
	return info
}
func queueAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrInvalid
	}
	c, e := aes.NewCipher(key)
	if e != nil {
		return nil, ErrInvalid
	}
	return cipher.NewGCM(c)
}

// QueueBackup reads only queue metadata and its referenced attempt states. It
// never opens token files, a database, station keys or unrelated directory files.
func QueueBackup(dir string, key []byte, out string) (QueueBackupInfo, error) {
	var zero QueueBackupInfo
	a, e := queueAEAD(key)
	if e != nil || !filepath.IsAbs(out) {
		return zero, ErrInvalid
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return zero, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return zero, e
	}
	s := queueSnapshot{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Queue: q, States: map[string]State{}}
	locks := []func(){}
	defer func() {
		for n := len(locks) - 1; n >= 0; n-- {
			locks[n]()
		}
	}()
	for _, i := range q.Items {
		path := statePath(dir, i.ID)
		release, e := lockState(path + ".lock")
		if e != nil {
			return zero, e
		}
		locks = append(locks, release)
		_, e = os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return zero, e
		}
		b, e := readPrivate(path, 4096)
		if e != nil {
			return zero, e
		}
		var st State
		if decode(b, &st) != nil {
			return zero, ErrInvalid
		}
		s.States[i.ID] = st
	}
	if validateSnapshot(s) != nil {
		return zero, ErrInvalid
	}
	raw, e := json.Marshal(s)
	if e != nil || len(raw) > maxQueueSnapshot {
		return zero, ErrInvalid
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return zero, e
	}
	encrypted := append([]byte(queueBackupMagic), nonce...)
	encrypted = a.Seal(encrypted, nonce, raw, queueBackupAAD)
	if e = publishPrivate(out, encrypted); e != nil {
		return zero, e
	}
	return snapshotInfo(s, raw), nil
}
func loadQueueBackup(archive string, key []byte) (queueSnapshot, []byte, error) {
	var s queueSnapshot
	a, e := queueAEAD(key)
	if e != nil {
		return s, nil, e
	}
	b, e := readPrivate(archive, maxQueueSnapshot+64)
	if e != nil {
		return s, nil, e
	}
	header := len(queueBackupMagic)
	if len(b) < header+a.NonceSize()+a.Overhead() || string(b[:header]) != queueBackupMagic {
		return s, nil, ErrInvalid
	}
	raw, e := a.Open(nil, b[header:header+a.NonceSize()], b[header+a.NonceSize():], queueBackupAAD)
	if e != nil || len(raw) > maxQueueSnapshot || decode(raw, &s) != nil || validateSnapshot(s) != nil {
		return queueSnapshot{}, nil, ErrInvalid
	}
	return s, raw, nil
}
func QueueVerifyBackup(archive string, key []byte) (QueueBackupInfo, error) {
	s, raw, e := loadQueueBackup(archive, key)
	if e != nil {
		return QueueBackupInfo{}, e
	}
	return snapshotInfo(s, raw), nil
}

// QueueRestore publishes only a NEW private directory. Every restored job is
// permanently receipt-only; even explicit reauthorization cannot resend it.
func QueueRestore(archive string, key []byte, dest string) (QueueBackupInfo, error) {
	s, raw, e := loadQueueBackup(archive, key)
	if e != nil {
		return QueueBackupInfo{}, e
	}
	if !filepath.IsAbs(dest) || filepath.Clean(dest) != dest || filepath.Base(dest) == "." {
		return QueueBackupInfo{}, ErrInvalid
	}
	if _, e = os.Lstat(dest); !errors.Is(e, os.ErrNotExist) {
		return QueueBackupInfo{}, ErrInvalid
	}
	tmp, e := os.MkdirTemp(filepath.Dir(dest), ".titan-queue-restore-")
	if e != nil {
		return QueueBackupInfo{}, e
	}
	// Failed staging remains private for inspection; existing destinations are never removed.
	for n := range s.Queue.Items {
		i := &s.Queue.Items[n]
		i.RecoveryOnly = true
		i.Status = "paused"
		i.NextAttemptAt = ""
		i.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		i.LastError = "restored_queue_review_required"
		i.LastAction = "restored_snapshot"
		if st, ok := s.States[i.ID]; ok {
			if e = save(statePath(tmp, i.ID), st); e != nil {
				return QueueBackupInfo{}, e
			}
		}
	}
	if e = saveQueue(tmp, s.Queue); e != nil {
		return QueueBackupInfo{}, e
	}
	if e = publishQueueDirectory(tmp, dest); e != nil {
		return QueueBackupInfo{}, e
	}
	if e = syncDirectory(filepath.Dir(dest)); e != nil {
		return QueueBackupInfo{}, e
	}
	return snapshotInfo(s, raw), nil
}
func syncDirectory(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func publishPrivate(out string, b []byte) error {
	if !filepath.IsAbs(out) {
		return ErrInvalid
	}
	f, e := os.CreateTemp(filepath.Dir(out), ".titan-queue-backup-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Link(name, out); e != nil {
		return e
	}
	return syncDirectory(filepath.Dir(out))
}
