package localapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/traderelay"
)

func writeSessionQueueScope(t *testing.T, path string, source traderelay.Endpoint, token string) *traderelay.QueueBinding {
	t.Helper()
	rawToken, _ := json.Marshal(token)
	hash := sha256.Sum256(rawToken)
	raw, _ := json.Marshal(traderelay.QueueScope{Version: 2, Source: source, SourceSessionHash: hex.EncodeToString(hash[:])})
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	bound, e := traderelay.BindQueueScope(path)
	if e != nil {
		t.Fatal(e)
	}
	return bound
}

func TestTradeQueueSessionScopeNewLoginSameActorNeverResumesOldAttempt(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		name := "queued"
		if uncertain {
			name = "uncertain"
		}
		t.Run(name, func(t *testing.T) {
			var drop atomic.Bool
			drop.Store(uncertain)
			q := queueReal(t, func(s *tradeHTTPFixture, j *traderelay.Job) {
				if uncertain {
					proxy := relayFaultProxy(t, s.p.service.url, &drop, true)
					j.Target.URL = proxy.URL + "/local/v1"
				}
			})
			now := time.Now().Add(time.Second)
			path := filepath.Join(t.TempDir(), "session-scope.json")
			original := writeSessionQueueScope(t, path, q.j.Source, q.b.p.token)
			if uncertain {
				b, e := original.Run(context.Background(), q.dir, 1, now)
				if e != nil || b.Uncertain != 1 || b.Posts != 1 {
					t.Fatal(b, e)
				}
			}
			before, e := os.ReadFile(filepath.Join(q.dir, "queue.json"))
			if e != nil {
				t.Fatal(e)
			}
			proofPath := filepath.Join(q.dir, q.id+".state.json")
			proof, proofErr := os.ReadFile(proofPath)
			nextToken := q.b.p.login(q.b.f.owner.OwnerID)
			if nextToken == q.b.p.token {
				t.Fatal("fixture did not create another authenticated session")
			}
			current := writeSessionQueueScope(t, path, q.j.Source, nextToken)
			view, e := current.Read(q.dir)
			if e != nil || view.Total != 0 {
				t.Fatal(view, e)
			}
			b, e := current.Run(context.Background(), q.dir, 1, now.Add(time.Minute))
			if e != nil || b != (traderelay.QueueBatch{}) {
				t.Fatal("new login dispatched old session", b, e)
			}
			after, e := os.ReadFile(filepath.Join(q.dir, "queue.json"))
			if e != nil || string(before) != string(after) {
				t.Fatal("original metadata changed", e)
			}
			afterProof, afterErr := os.ReadFile(proofPath)
			if (proofErr == nil) != (afterErr == nil) || string(proof) != string(afterProof) {
				t.Fatal("original proof changed")
			}
			// Rebinding to the explicitly selected original context delivers once or
			// recovers the existing receipt; it never repeats an uncertain POST.
			original = writeSessionQueueScope(t, path, q.j.Source, q.b.p.token)
			b, e = original.Run(context.Background(), q.dir, 1, now.Add(2*time.Minute))
			expectedPosts := 1
			if uncertain {
				expectedPosts = 0
			}
			if e != nil || b.Confirmed != 1 || b.Posts != expectedPosts {
				t.Fatal(b, e)
			}
			assertSingleQueuedEffect(t, q)
		})
	}
}
