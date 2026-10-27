package traderelay

import (
	"context"
	"time"
)

// QueueRecoverSession authorizes only receipt reads with a new target session.
// The original source public key and exact job/envelope need a durable proof.
// It neither contacts the source nor authorizes a resend.
func QueueRecoverSession(ctx context.Context, dir, id, targetFile string) (QueueAction, error) {
	var out QueueAction
	if ctx == nil || !tokenPath(targetFile) {
		return out, ErrInvalid
	}
	tt, e := ReadToken(targetFile)
	if e != nil {
		return out, e
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return out, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return out, e
	}
	for n := range q.Items {
		i := &q.Items[n]
		if i.ID != id {
			continue
		}
		release, e := lockState(statePath(dir, id) + ".lock")
		if e != nil {
			return out, e
		}
		defer release()
		raw, e := readPrivate(statePath(dir, id), 4096)
		if e != nil {
			return out, e
		}
		var proof State
		if decode(raw, &proof) != nil || validateSnapshot(queueSnapshot{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Queue: queueFile{Version: 1, Items: []QueueItem{*i}}, States: map[string]State{id: proof}}) != nil {
			return out, ErrInvalid
		}
		client, close := newClient()
		defer close()
		target := api{i.Job.Target, tt, client, &out.TargetCalls}
		if _, e = target.identity(ctx); e != nil {
			return out, e
		}
		i.TargetTokenFile = targetFile
		i.TargetSessionHash = digest(tt)
		i.ReceiptOnlySession = true
		i.Status = "queued"
		i.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		i.NextAttemptAt = i.UpdatedAt
		i.LastError = ""
		i.LastAction = "operator_receipt_reauthorization"
		if e = saveQueue(dir, q); e != nil {
			return out, e
		}
		out.Item = itemView(*i)
		return out, nil
	}
	return out, ErrInvalid
}
