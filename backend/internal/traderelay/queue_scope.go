package traderelay

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// QueueScope is an additional dispatch filter, never a credential or a grant.
// A desktop host must derive it from its verified active API context. The CLI
// operator explicitly provisions this private file; no API or identity is
// selected by this package, and no token or business document belongs here.
type QueueScope struct {
	Version int      `json:"version"`
	Source  Endpoint `json:"source"`
}

// ErrScopeChanged deliberately exposes no path, identity, credential or body.
var ErrScopeChanged = fmt.Errorf("%w: worker context changed; original attempts preserved", ErrContext)

type QueueBinding struct {
	file  string
	scope QueueScope
}
type queueScopeGuardKey struct{}

func readQueueScope(file string) (QueueScope, error) {
	var scope QueueScope
	if !filepath.IsAbs(file) || len(file) > 4096 {
		return scope, ErrInvalid
	}
	raw, e := readPrivate(file, 4096)
	if e != nil || decode(raw, &scope) != nil || scope.Version != 1 || !validEndpoint(scope.Source) {
		return QueueScope{}, ErrInvalid
	}
	return scope, nil
}

// BindQueueScope pins the original context for this worker lifetime. Editing a
// file never silently rebinds an already running worker to another operator.
func BindQueueScope(file string) (*QueueBinding, error) {
	scope, e := readQueueScope(file)
	if e != nil {
		return nil, e
	}
	return &QueueBinding{file: file, scope: scope}, nil
}
func (b *QueueBinding) check() error {
	if b == nil {
		return ErrScopeChanged
	}
	scope, e := readQueueScope(b.file)
	if e != nil || scope != b.scope {
		return ErrScopeChanged
	}
	return nil
}
func checkScopeContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalid
	}
	if guard, ok := ctx.Value(queueScopeGuardKey{}).(func() error); ok {
		return guard()
	}
	return nil
}

// Read omits all other contexts, including their IDs and operation counts.
// It does not read session files, contact an API or mutate queue records.
func (b *QueueBinding) Read(dir string) (QueueView, error) {
	if b == nil {
		return QueueView{}, ErrInvalid
	}
	if e := b.check(); e != nil {
		return QueueView{}, e
	}
	q, e := readQueue(dir)
	if e != nil {
		return QueueView{}, e
	}
	view := QueueView{Version: 1, Items: []QueueItemView{}}
	for _, i := range q.Items {
		if i.Job.Source == b.scope.Source {
			view.Items = append(view.Items, itemView(i))
		}
	}
	view.Total = len(view.Items)
	if e = b.check(); e != nil {
		return QueueView{}, e
	}
	return view, nil
}

// Run processes only this immutable source context. Every HTTP call rechecks
// the private binding, including the last check before the sole POST. A request
// already dispatched can finish; its result/proof is persisted before stopping.
// A lost response keeps the original uncertain state and never authorizes retry.
func (b *QueueBinding) Run(ctx context.Context, dir string, limit int, now time.Time) (QueueBatch, error) {
	if b == nil {
		return QueueBatch{}, ErrInvalid
	}
	return queueRun(ctx, dir, limit, now, b)
}
