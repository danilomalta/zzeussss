package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func TestTradeQueueBackupCompiledRestoreNeverReplaysOldSnapshot(t *testing.T) {
	binary := relayExecutable(t)
	for _, kind := range []string{"unsent", "delivered-after-backup", "uncertain-delivered", "confirmed"} {
		t.Run(kind, func(t *testing.T) {
			var drop atomic.Bool
			drop.Store(true)
			q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
				if kind == "uncertain-delivered" {
					p := relayFaultProxy(t, s.p.service.url, &drop, true)
					j.Target.URL = p.URL + "/local/v1"
				}
			})
			ctx := context.Background()
			now := time.Now().Add(time.Second)
			if kind == "uncertain-delivered" || kind == "confirmed" {
				b, e := traderelay.QueueRun(ctx, q.dir, 10, now)
				if e != nil || b.Posts != 1 {
					t.Fatal("initial delivery", e)
				}
				assertSingleQueuedEffect(t, q)
			}
			base := t.TempDir()
			key := make([]byte, 32)
			if _, e := rand.Read(key); e != nil {
				t.Fatal(e)
			}
			keyFile := filepath.Join(base, "backup.key")
			if e := os.WriteFile(keyFile, key, 0600); e != nil {
				t.Fatal(e)
			}
			archive, dest := filepath.Join(base, "queue.enc"), filepath.Join(base, "restored")
			call := func(args ...string) traderelay.QueueBackupInfo {
				t.Helper()
				cmd := exec.Command(binary, append([]string{"queue"}, args...)...)
				out, e := cmd.CombinedOutput()
				if e != nil {
					t.Fatalf("queue backup CLI failed: %v %s", e, out)
				}
				var info traderelay.QueueBackupInfo
				if json.Unmarshal(out, &info) != nil || info.Items != 1 || info.Scope != "local_trade_queue_v1" {
					t.Fatal("invalid backup result")
				}
				return info
			}
			original, _ := os.ReadFile(filepath.Join(q.dir, "queue.json"))
			info := call("backup", "--queue", q.dir, "--key", keyFile, "--out", archive)
			checked := call("verify-backup", "--archive", archive, "--key", keyFile)
			if info != checked {
				t.Fatal("backup digest mismatch")
			}
			after, _ := os.ReadFile(filepath.Join(q.dir, "queue.json"))
			if string(original) != string(after) {
				t.Fatal("backup changed source")
			}
			if kind == "delivered-after-backup" {
				b, e := traderelay.QueueRun(ctx, q.dir, 10, now)
				if e != nil || b.Confirmed != 1 {
					t.Fatal("later delivery", e)
				}
				assertSingleQueuedEffect(t, q)
			}
			restored := call("restore", "--queue", dest, "--archive", archive, "--key", keyFile)
			if restored.SnapshotSHA256 != info.SnapshotSHA256 {
				t.Fatal("restore changed original snapshot digest")
			}
			b, e := traderelay.QueueRun(ctx, dest, 10, now.Add(time.Hour))
			if e != nil || b.Processed != 0 || b.Posts != 0 {
				t.Fatal("automatic restored dispatch")
			}
			view, e := traderelay.QueueRead(dest)
			if e != nil || view.Total != 1 || !view.Items[0].RecoveryOnly || view.Items[0].OperationID != q.j.OperationID || view.Items[0].MessageID != q.j.MessageID {
				t.Fatal("restoration lost identity/barrier")
			}
			if _, e = traderelay.QueueResume(ctx, dest, q.id, q.source, q.target); e != nil {
				t.Fatal("explicit authorization", e)
			}
			// Existing original proof allows target-only recovery even with source offline.
			if kind == "uncertain-delivered" || kind == "confirmed" {
				if e = q.b.f.app.ShutdownWithTimeout(3 * time.Second); e != nil {
					t.Fatal(e)
				}
			}
			b, e = traderelay.QueueRun(ctx, dest, 10, now.Add(2*time.Hour))
			if e != nil || b.Posts != 0 {
				t.Fatalf("restored write %+v %v", b, e)
			}
			if kind == "unsent" {
				if b.Paused != 1 {
					t.Fatalf("missing receipt not paused %+v", b)
				}
				if _, e = traderelay.QueueRetry(ctx, dest, q.id, now.Add(3*time.Hour)); e == nil {
					t.Fatal("restored uncertain history retried")
				}
				var count int
				if e = q.s.f.db.QueryRow(`SELECT count(*) FROM company_trade_operations WHERE operation_id=?`, q.j.OperationID).Scan(&count); e != nil || count != 0 {
					t.Fatal("restore created remote operation")
				}
			} else {
				if b.Confirmed != 1 {
					t.Fatalf("original receipt not recovered %+v", b)
				}
				assertSingleQueuedEffect(t, q)
			}
			if (kind == "uncertain-delivered" || kind == "confirmed") && b.SourceCalls != 0 {
				t.Fatal("recovery contacted offline sender")
			}
			t.Logf("encrypted queue restore: %s; original identifiers, posts=%d, confirmed=%d paused=%d", kind, b.Posts, b.Confirmed, b.Paused)
		})
	}
}
