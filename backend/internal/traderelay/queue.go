package traderelay

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/localdb/supplycycle"
)

const MaxQueueItems = 500
const maxQueueBytes = 4 << 20

var ErrQueueFull = errors.New("private queue limit reached; preserve existing records")

type QueueItem struct {
	ReceiptOnlySession bool   `json:"receipt_only_session,omitempty"`
	RecoveryOnly       bool   `json:"recovery_only,omitempty"`
	ID                 string `json:"id"`
	Job                Job    `json:"job"`
	EnvelopeHash       string `json:"envelope_sha256"`
	SourceTokenFile    string `json:"source_token_file"`
	TargetTokenFile    string `json:"target_token_file"`
	SourceSessionHash  string `json:"source_session_sha256"`
	TargetSessionHash  string `json:"target_session_sha256"`
	Status             string `json:"status"`
	Attempts           int    `json:"attempts"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	NextAttemptAt      string `json:"next_attempt_at"`
	LastError          string `json:"last_error"`
	LastAction         string `json:"last_action"`
}
type queueFile struct {
	Version int         `json:"version"`
	Items   []QueueItem `json:"items"`
}
type QueueItemView struct {
	ReceiptOnlySession bool   `json:"receipt_only_session,omitempty"`
	RecoveryOnly       bool   `json:"recovery_only,omitempty"`
	ID                 string `json:"id"`
	OperationID        string `json:"operation_id"`
	MessageID          string `json:"message_id"`
	Status             string `json:"status"`
	Attempts           int    `json:"attempts"`
	NextAttemptAt      string `json:"next_attempt_at"`
	LastError          string `json:"last_error"`
	LastAction         string `json:"last_action"`
}
type QueueView struct {
	Version int             `json:"version"`
	Total   int             `json:"total"`
	Items   []QueueItemView `json:"items"`
}
type QueueAction struct {
	Item        QueueItemView `json:"item"`
	SourceCalls int           `json:"source_http_calls"`
	TargetCalls int           `json:"target_http_calls"`
}
type QueueBatch struct {
	Processed   int `json:"processed"`
	Confirmed   int `json:"confirmed"`
	Uncertain   int `json:"uncertain"`
	Paused      int `json:"paused"`
	Retrying    int `json:"retrying"`
	ManualRetry int `json:"manual_retry"`
	Posts       int `json:"posts"`
	SourceCalls int `json:"source_http_calls"`
	TargetCalls int `json:"target_http_calls"`
}

func itemView(i QueueItem) QueueItemView {
	return QueueItemView{ID: i.ID, OperationID: i.Job.OperationID, MessageID: i.Job.MessageID, Status: i.Status, Attempts: i.Attempts, NextAttemptAt: i.NextAttemptAt, LastError: i.LastError, LastAction: i.LastAction, RecoveryOnly: i.RecoveryOnly, ReceiptOnlySession: i.ReceiptOnlySession}
}
func queuePath(dir string) string     { return filepath.Join(dir, "queue.json") }
func statePath(dir, id string) string { return filepath.Join(dir, id+".state.json") }
func privateDir(dir string) bool {
	if !filepath.IsAbs(dir) {
		return false
	}
	s, e := os.Lstat(dir)
	return e == nil && s.IsDir() && s.Mode().Perm() == 0700 && s.Mode()&os.ModeSymlink == 0
}
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, b := range s {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}
func tokenPath(s string) bool { return filepath.IsAbs(s) && len(s) <= 4096 }
func queueStatus(s string) bool {
	switch s {
	case "queued", "retrying", "uncertain", "needs_retry", "paused", "confirmed":
		return true
	}
	return false
}
func validQueueError(s string) bool {
	switch s {
	case "", "authentication_or_permission", "api_refused", "session_or_context_changed", "transport_unavailable", "result_uncertain", "attempt_locked", "private_state_invalid", "confirmed_receipt_missing", "restored_queue_review_required", "restored_receipt_missing", "receipt_only_receipt_missing":
		return true
	}
	return false
}
func validQueueAction(s string) bool {
	switch s {
	case "authorized_add", "operator_pause", "operator_reauthorization", "worker_attempt", "operator_retry", "operator_verify", "restored_snapshot", "operator_receipt_reauthorization":
		return true
	}
	return false
}
func queueTime(s string) bool {
	t, e := time.Parse(time.RFC3339Nano, s)
	return e == nil && t.Year() >= 2000 && t.Year() <= 2200
}
func readQueue(dir string) (queueFile, error) {
	q := queueFile{}
	if !privateDir(dir) {
		return q, ErrInvalid
	}
	b, e := readPrivate(queuePath(dir), maxQueueBytes)
	if e != nil || decode(b, &q) != nil || q.Version != 1 || q.Items == nil || len(q.Items) > MaxQueueItems {
		return queueFile{}, ErrInvalid
	}
	if validateQueue(q) != nil {
		return queueFile{}, ErrInvalid
	}
	return q, nil
}
func validateQueue(q queueFile) error {
	if q.Version != 1 || q.Items == nil || len(q.Items) > MaxQueueItems {
		return ErrInvalid
	}
	seen := map[string]bool{}
	operations := map[string]bool{}
	messages := map[string]bool{}
	for _, i := range q.Items {
		op := digest(struct {
			Target    supplycycle.TradeParty
			Operation string
		}{i.Job.Target.Party, i.Job.OperationID})
		msg := digest(struct {
			Target  supplycycle.TradeParty
			Message string
		}{i.Job.Target.Party, i.Job.MessageID})
		if i.Job.Validate() != nil || i.ID != digest(i.Job) || !validHash(i.EnvelopeHash) || !validHash(i.SourceSessionHash) || !validHash(i.TargetSessionHash) || !tokenPath(i.SourceTokenFile) || !tokenPath(i.TargetTokenFile) || !queueStatus(i.Status) || !validQueueError(i.LastError) || !validQueueAction(i.LastAction) || i.Attempts < 0 || i.Attempts > 1000000 || !queueTime(i.CreatedAt) || !queueTime(i.UpdatedAt) || (i.NextAttemptAt != "" && !queueTime(i.NextAttemptAt)) || seen[i.ID] || operations[op] || messages[msg] {
			return ErrInvalid
		}
		scheduled := i.Status == "queued" || i.Status == "retrying" || i.Status == "uncertain"
		if scheduled != (i.NextAttemptAt != "") {
			return ErrInvalid
		}
		seen[i.ID] = true
		operations[op] = true
		messages[msg] = true
	}
	return nil
}
func queueLock(dir string) (func(), error) {
	if !privateDir(dir) {
		return nil, ErrInvalid
	}
	return lockState(filepath.Join(dir, "queue.lock"))
}
func saveQueue(dir string, q queueFile) error {
	b, e := json.Marshal(q)
	if e != nil || len(b) > maxQueueBytes {
		return ErrQueueFull
	}
	return saveJSON(queuePath(dir), q)
}
func QueueInit(dir string) error {
	if !filepath.IsAbs(dir) {
		return ErrInvalid
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		if !errors.Is(e, os.ErrExist) || !privateDir(dir) {
			return ErrInvalid
		}
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			return ErrInvalid
		}
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return e
	}
	defer unlock()
	if _, e = os.Lstat(queuePath(dir)); !errors.Is(e, os.ErrNotExist) {
		return ErrInvalid
	}
	return saveQueue(dir, queueFile{Version: 1, Items: []QueueItem{}})
}

// QueueRead is local metadata only; private token paths and session hashes are
// intentionally absent from this view. It never calls an API or sends a job.
func QueueRead(dir string) (QueueView, error) {
	q, e := readQueue(dir)
	if e != nil {
		return QueueView{}, e
	}
	view := QueueView{Version: 1, Total: len(q.Items), Items: []QueueItemView{}}
	for _, i := range q.Items {
		view.Items = append(view.Items, itemView(i))
	}
	return view, nil
}
func authorizeQueued(ctx context.Context, j Job, st, tt string) (string, int, int, error) {
	sc, tc := 0, 0
	client, close := newClient()
	defer close()
	source, target := api{j.Source, st, client, &sc}, api{j.Target, tt, client, &tc}
	public, e := source.identity(ctx)
	if e != nil {
		return "", sc, tc, e
	}
	if _, e = target.identity(ctx); e != nil {
		return "", sc, tc, e
	}
	if e = source.ready(ctx); e != nil {
		return "", sc, tc, e
	}
	if e = target.ready(ctx); e != nil {
		return "", sc, tc, e
	}
	var record supplycycle.TradeMessageRecord
	if e = source.call(ctx, "GET", "/company-trade/messages/"+url.PathEscape(j.MessageID), nil, &record); e != nil {
		return "", sc, tc, e
	}
	m, e := supplycycle.ParseTradeEnvelope(record.Envelope, public)
	if e != nil || m != record.Message || record.Direction != "sent" || m.ID != j.MessageID || m.Source != j.Source.Party || m.Target != j.Target.Party {
		return "", sc, tc, ErrContext
	}
	return supplycycle.TradeEnvelopeSHA256(record.Envelope), sc, tc, nil
}
func QueueAdd(ctx context.Context, dir, jobFile, sourceFile, targetFile string) (QueueAction, error) {
	var result QueueAction
	if ctx == nil || !tokenPath(sourceFile) || !tokenPath(targetFile) {
		return result, ErrInvalid
	}
	j, e := ReadJob(jobFile)
	if e != nil {
		return result, e
	}
	st, e := ReadToken(sourceFile)
	if e != nil {
		return result, e
	}
	tt, e := ReadToken(targetFile)
	if e != nil {
		return result, e
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return result, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return result, e
	}
	if len(q.Items) >= MaxQueueItems {
		return result, ErrQueueFull
	}
	for _, i := range q.Items {
		if i.Job.Target.Party == j.Target.Party && (i.Job.OperationID == j.OperationID || i.Job.MessageID == j.MessageID) {
			return result, ErrInvalid
		}
	}
	hash, sc, tc, e := authorizeQueued(ctx, j, st, tt)
	result.SourceCalls = sc
	result.TargetCalls = tc
	if e != nil {
		return result, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	i := QueueItem{ID: digest(j), Job: j, EnvelopeHash: hash, SourceTokenFile: sourceFile, TargetTokenFile: targetFile, SourceSessionHash: digest(st), TargetSessionHash: digest(tt), Status: "queued", CreatedAt: now, UpdatedAt: now, NextAttemptAt: now, LastAction: "authorized_add"}
	q.Items = append(q.Items, i)
	if e = saveQueue(dir, q); e != nil {
		return result, e
	}
	result.Item = itemView(i)
	return result, nil
}
func QueuePause(dir, id string) error {
	unlock, e := queueLock(dir)
	if e != nil {
		return e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return e
	}
	for n := range q.Items {
		if q.Items[n].ID == id {
			if q.Items[n].Status == "confirmed" {
				return ErrInvalid
			}
			q.Items[n].Status = "paused"
			q.Items[n].NextAttemptAt = ""
			q.Items[n].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			q.Items[n].LastAction = "operator_pause"
			return saveQueue(dir, q)
		}
	}
	return ErrInvalid
}

// QueueResume is the explicit session reauthorization. The original job and
// signed envelope hash cannot change. An uncertain engine state stays intact.
func QueueResume(ctx context.Context, dir, id, sourceFile, targetFile string) (QueueAction, error) {
	var result QueueAction
	if ctx == nil || !tokenPath(sourceFile) || !tokenPath(targetFile) {
		return result, ErrInvalid
	}
	st, e := ReadToken(sourceFile)
	if e != nil {
		return result, e
	}
	tt, e := ReadToken(targetFile)
	if e != nil {
		return result, e
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return result, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return result, e
	}
	for n := range q.Items {
		if q.Items[n].ID != id {
			continue
		}
		i := &q.Items[n]
		if i.Status == "confirmed" {
			return result, ErrInvalid
		}
		hash, sc, tc, e := authorizeQueued(ctx, i.Job, st, tt)
		result.SourceCalls = sc
		result.TargetCalls = tc
		if e != nil {
			return result, e
		}
		if hash != i.EnvelopeHash {
			return result, ErrContext
		}
		i.SourceTokenFile = sourceFile
		i.TargetTokenFile = targetFile
		i.SourceSessionHash = digest(st)
		i.TargetSessionHash = digest(tt)
		i.ReceiptOnlySession = false
		i.Status = "queued"
		i.LastError = ""
		i.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		i.NextAttemptAt = i.UpdatedAt
		i.LastAction = "operator_reauthorization"
		if e = saveQueue(dir, q); e != nil {
			return result, e
		}
		result.Item = itemView(*i)
		return result, nil
	}
	return result, ErrInvalid
}
func boundTokens(i QueueItem) (string, string, error) {
	st, e := ReadToken(i.SourceTokenFile)
	if e != nil {
		return "", "", e
	}
	tt, e := ReadToken(i.TargetTokenFile)
	if e != nil {
		return "", "", e
	}
	if digest(st) != i.SourceSessionHash || digest(tt) != i.TargetSessionHash {
		return "", "", ErrContext
	}
	return st, tt, nil
}
func backoff(attempts int) time.Duration {
	seconds := 2
	for n := 1; n < attempts && seconds < 300; n++ {
		seconds *= 2
	}
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}
func errorCode(e error) string {
	var h HTTPError
	switch {
	case errors.As(e, &h):
		if h.Status == 401 || h.Status == 403 {
			return "authentication_or_permission"
		}
		return "api_refused"
	case errors.Is(e, ErrContext):
		return "session_or_context_changed"
	case errors.Is(e, ErrTransport):
		return "transport_unavailable"
	case errors.Is(e, ErrUncertain):
		return "result_uncertain"
	case errors.Is(e, ErrLocked):
		return "attempt_locked"
	default:
		return "private_state_invalid"
	}
}
func attemptQueued(ctx context.Context, dir string, i *QueueItem, now time.Time, explicitRetry bool) (Result, error) {
	if i.Attempts >= 1000000 {
		return Result{}, ErrInvalid
	}
	i.Attempts++
	i.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	i.LastAction = "worker_attempt"
	if explicitRetry {
		i.LastAction = "operator_retry"
	}
	var st, tt string
	var e error
	if i.ReceiptOnlySession {
		tt, e = ReadToken(i.TargetTokenFile)
		if e == nil && digest(tt) != i.TargetSessionHash {
			e = ErrContext
		}
	} else {
		st, tt, e = boundTokens(*i)
	}
	if e != nil {
		i.Status = "paused"
		i.NextAttemptAt = ""
		i.LastError = errorCode(e)
		return Result{}, nil
	}
	path := statePath(dir, i.ID)
	_, statErr := os.Lstat(path)
	mode := "send"
	if statErr == nil {
		mode = "recover"
	} else if !errors.Is(statErr, os.ErrNotExist) {
		i.Status = "paused"
		i.NextAttemptAt = ""
		i.LastError = "private_state_invalid"
		return Result{}, nil
	}
	if i.ReceiptOnlySession && mode != "recover" {
		i.Status = "paused"
		i.NextAttemptAt = ""
		i.LastError = "private_state_invalid"
		return Result{}, nil
	}
	if explicitRetry {
		if i.ReceiptOnlySession {
			return Result{}, ErrInvalid
		}
		if mode != "recover" {
			return Result{}, ErrInvalid
		}
		mode = "retry"
	}
	result, e := runPolicy(ctx, i.Job, st, tt, path, mode, i.EnvelopeHash, i.RecoveryOnly || i.ReceiptOnlySession)
	if e == nil && result.Status == "confirmed" {
		i.Status = "confirmed"
		i.NextAttemptAt = ""
		i.LastError = ""
		return result, nil
	}
	i.LastError = errorCode(e)
	i.NextAttemptAt = ""
	switch {
	case result.Status == "not_found":
		i.Status = "needs_retry"
		if i.ReceiptOnlySession {
			i.Status = "paused"
			i.LastError = "receipt_only_receipt_missing"
		}
		if i.RecoveryOnly {
			i.Status = "paused"
			i.LastError = "restored_receipt_missing"
		}
	case errors.Is(e, ErrTransport):
		i.Status = "retrying"
		if mode == "recover" {
			i.Status = "uncertain"
		}
		i.NextAttemptAt = now.Add(backoff(i.Attempts)).UTC().Format(time.RFC3339Nano)
	case errors.Is(e, ErrUncertain) && result.Status == "uncertain":
		i.Status = "uncertain"
		i.NextAttemptAt = now.Add(backoff(i.Attempts)).UTC().Format(time.RFC3339Nano)
	case errors.Is(e, ErrLocked):
		i.Status = "retrying"
		if mode == "recover" {
			i.Status = "uncertain"
		}
		i.NextAttemptAt = now.Add(backoff(i.Attempts)).UTC().Format(time.RFC3339Nano)
	default:
		i.Status = "paused"
	}
	return result, nil
}
func addBatch(b *QueueBatch, i QueueItem, r Result) {
	b.Processed++
	b.Posts += r.Posts
	b.SourceCalls += r.SourceCalls
	b.TargetCalls += r.TargetCalls
	switch i.Status {
	case "confirmed":
		b.Confirmed++
	case "uncertain":
		b.Uncertain++
	case "paused":
		b.Paused++
	case "needs_retry":
		b.ManualRetry++
	case "retrying":
		b.Retrying++
	}
}

// QueueRun performs a bounded pass. Existing engine state forces recovery-only;
// a persisted uncertain operation is never automatically resent after restart.
func QueueRun(ctx context.Context, dir string, limit int, now time.Time) (QueueBatch, error) {
	return queueRun(ctx, dir, limit, now, nil)
}
func queueRun(ctx context.Context, dir string, limit int, now time.Time, binding *QueueBinding) (QueueBatch, error) {
	var batch QueueBatch
	if ctx == nil || limit < 1 || limit > 100 || now.IsZero() {
		return batch, ErrInvalid
	}
	if binding != nil {
		if e := binding.check(); e != nil {
			return batch, e
		}
		ctx = context.WithValue(ctx, queueScopeGuardKey{}, binding.check)
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return batch, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return batch, e
	}
	for n := range q.Items {
		if batch.Processed >= limit {
			break
		}
		if e = ctx.Err(); e != nil {
			return batch, e
		}
		i := &q.Items[n]
		if binding != nil && !binding.matches(*i) {
			continue
		}
		if i.Status != "queued" && i.Status != "retrying" && i.Status != "uncertain" {
			continue
		}
		due, e := time.Parse(time.RFC3339Nano, i.NextAttemptAt)
		if e != nil {
			return batch, ErrInvalid
		}
		if now.Before(due) {
			continue
		}
		if e := checkScopeContext(ctx); e != nil {
			return batch, e
		}
		r, e := attemptQueued(ctx, dir, i, now, false)
		if e != nil {
			return batch, e
		}
		if e = saveQueue(dir, q); e != nil {
			return batch, e
		}
		addBatch(&batch, *i, r)
	}
	if e := checkScopeContext(ctx); e != nil {
		return batch, e
	}
	return batch, nil
}
func QueueRetry(ctx context.Context, dir, id string, now time.Time) (QueueBatch, error) {
	var batch QueueBatch
	if ctx == nil || now.IsZero() {
		return batch, ErrInvalid
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return batch, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return batch, e
	}
	for n := range q.Items {
		if q.Items[n].ID != id {
			continue
		}
		i := &q.Items[n]
		if i.Status != "needs_retry" || i.RecoveryOnly || i.ReceiptOnlySession {
			return batch, ErrInvalid
		}
		r, e := attemptQueued(ctx, dir, i, now, true)
		if e != nil {
			return batch, e
		}
		if e = saveQueue(dir, q); e != nil {
			return batch, e
		}
		addBatch(&batch, *i, r)
		return batch, nil
	}
	return batch, ErrInvalid
}

// QueueVerify explicitly rechecks an original receipt. It never creates a first
// attempt and never converts a lost confirmed receipt into a new POST.
func QueueVerify(ctx context.Context, dir, id string, now time.Time) (QueueBatch, error) {
	var batch QueueBatch
	if ctx == nil || now.IsZero() {
		return batch, ErrInvalid
	}
	unlock, e := queueLock(dir)
	if e != nil {
		return batch, e
	}
	defer unlock()
	q, e := readQueue(dir)
	if e != nil {
		return batch, e
	}
	for n := range q.Items {
		if q.Items[n].ID != id {
			continue
		}
		i := &q.Items[n]
		if _, e = os.Lstat(statePath(dir, i.ID)); e != nil && !i.RecoveryOnly {
			return batch, ErrInvalid
		}
		wasConfirmed := i.Status == "confirmed"
		r, e := attemptQueued(ctx, dir, i, now, false)
		if e != nil {
			return batch, e
		}
		i.LastAction = "operator_verify"
		if wasConfirmed && i.Status == "needs_retry" {
			i.Status = "paused"
			i.LastError = "confirmed_receipt_missing"
			i.NextAttemptAt = ""
		}
		if e = saveQueue(dir, q); e != nil {
			return batch, e
		}
		addBatch(&batch, *i, r)
		return batch, nil
	}
	return batch, ErrInvalid
}
