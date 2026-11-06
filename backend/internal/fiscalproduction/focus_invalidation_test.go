package fiscalproduction

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
)

// A local refusal test, never evidence of Focus/SEFAZ acceptance or a free range.
func TestFocusInvalidationUnavailableNeverSendsOrMutates(t *testing.T) {
	a := focusContext(t) // Discardable SQLite, synthetic owner/session/license.
	var requests atomic.Int32
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	counts := func() [4]int {
		t.Helper()
		var out [4]int
		for i, table := range []string{"fiscal_invalidation_requests", "fiscal_invalidation_results", "fiscal_number_allocations", "outbox"} {
			if err := a.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&out[i]); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	before := counts()
	want := DocumentResult{State: "unavailable", ErrorCode: "focus_invalidation_unavailable"}
	for _, model := range []string{"55", "65"} {
		for _, environment := range []int{2, 1} {
			for attempt := 0; attempt < 3; attempt++ {
				out, err := p.Invalidate(context.Background(), a, DocumentRequest{
					Model: model, Environment: environment, OperationID: "same-unavailable-operation",
				})
				if !errors.Is(err, ErrFocusInvalidationUnavailable) || !reflect.DeepEqual(out, want) {
					t.Fatalf("model=%s environment=%d attempt=%d: expected explicit unavailable without fiscal evidence", model, environment, attempt)
				}
			}
		}
	}
	if requests.Load() != 0 {
		t.Fatal("unavailable invalidation contacted the remote service")
	}
	if after := counts(); after != before {
		t.Fatal("unavailable invalidation changed numbering, journals or audit outbox")
	}
}
