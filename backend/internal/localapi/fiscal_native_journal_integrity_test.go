package localapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// These are disposable SQLite fault injections, not fiscal authorizations.
// A failed write must never grant permission to POST or claim a saved response.
func TestNativeFiscalJournalReservationRequiresDurableInsert(t *testing.T) {
	for _, failure := range []string{"IGNORE", "ABORT, 'synthetic journal failure'"} {
		t.Run(failure, func(t *testing.T) {
			f := fiscalFixture(t)
			intent := fiscalIntent(t, f, fiscalIntentBody, 201)
			r := fiscalJournal(t, f, f.db, intent, f.token)
			in := fiscalRecorded(t, r)
			ctx := context.Background()
			if _, err := f.db.Exec(`CREATE TRIGGER test_native_request_failure BEFORE INSERT ON fiscal_transmission_requests BEGIN SELECT RAISE(` + failure + `); END`); err != nil {
				t.Fatal(err)
			}
			if err := r.Reserve(ctx, in); err == nil {
				t.Fatal("reservation reported success without a durable request")
			} else if failure == "IGNORE" && !errors.Is(err, fiscal.ErrConflict) {
				t.Fatal("ignored reservation did not return a conflict", err)
			}
			if err := r.BeforeSend(ctx, in.OperationID); !errors.Is(err, fiscal.ErrNotFound) {
				t.Fatal("missing reservation granted send authority", err)
			}
			if _, _, err := r.Read(ctx); !errors.Is(err, fiscal.ErrNotFound) {
				t.Fatal("failed reservation survived rollback", err)
			}
			if _, err := f.db.Exec(`DROP TRIGGER test_native_request_failure`); err != nil {
				t.Fatal(err)
			}
			if err := r.Reserve(ctx, in); err != nil {
				t.Fatal("same request could not recover after failed write", err)
			}
			if err := r.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
				t.Fatal("durable reservation lost its no-POST barrier", err)
			}
			request, result, err := r.Read(ctx)
			if err != nil || request.OperationID != in.OperationID || request.RequestSHA256 != in.RequestSHA256 || string(request.Body) != string(in.Body) || result.State != "uncertain" {
				t.Fatal("recovery changed original request", err)
			}
		})
	}
}

func nativeJournalResponse(in fiscalcore.RecordedRequest) fiscalcore.ExchangeResult {
	body := []byte("synthetic durable response; not fiscal authorization")
	sum := sha256.Sum256(body)
	return fiscalcore.ExchangeResult{OperationID: in.OperationID, State: "response_received", HTTPStatus: 200, Body: body, SHA256: hex.EncodeToString(sum[:])}
}

func TestNativeFiscalJournalCompletionRequiresDurableInsert(t *testing.T) {
	for _, failure := range []string{"IGNORE", "ABORT, 'synthetic journal failure'"} {
		t.Run(failure, func(t *testing.T) {
			f := fiscalFixture(t)
			intent := fiscalIntent(t, f, fiscalIntentBody, 201)
			r := fiscalJournal(t, f, f.db, intent, f.token)
			in := fiscalRecorded(t, r)
			ctx := context.Background()
			if err := r.Reserve(ctx, in); err != nil {
				t.Fatal(err)
			}
			response := nativeJournalResponse(in)
			if _, err := f.db.Exec(`CREATE TRIGGER test_native_result_failure BEFORE INSERT ON fiscal_transmission_results BEGIN SELECT RAISE(` + failure + `); END`); err != nil {
				t.Fatal(err)
			}
			if err := r.Complete(ctx, response); err == nil {
				t.Fatal("completion reported success without a durable response")
			} else if failure == "IGNORE" && !errors.Is(err, fiscal.ErrConflict) {
				t.Fatal("ignored completion did not return a conflict", err)
			}
			request, result, err := r.Read(ctx)
			if err != nil || request.OperationID != in.OperationID || result.State != "uncertain" || len(result.Body) != 0 {
				t.Fatal("failed completion lost original request or fabricated a result", err)
			}
			if err := r.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
				t.Fatal("failed completion allowed another POST", err)
			}
			if _, err := f.db.Exec(`DROP TRIGGER test_native_result_failure`); err != nil {
				t.Fatal(err)
			}
			if err := r.Complete(ctx, response); err != nil {
				t.Fatal("original completion could not recover", err)
			}
			if err := r.Complete(ctx, response); err != nil {
				t.Fatal("identical completion was not idempotent", err)
			}
			changed := response
			changed.HTTPStatus = 500
			if err := r.Complete(ctx, changed); !errors.Is(err, fiscal.ErrConflict) {
				t.Fatal("original response was overwritten", err)
			}
			request, result, err = r.Read(ctx)
			if err != nil || request.OperationID != in.OperationID || result.SHA256 != response.SHA256 || string(result.Body) != string(response.Body) || result.HTTPStatus != response.HTTPStatus {
				t.Fatal("recovery changed original response", err)
			}
		})
	}
}

func TestNativeFiscalJournalConcurrentCompletionIsConsistent(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
	}
	response := nativeJournalResponse(in)
	var wg sync.WaitGroup
	outcomes := make(chan error, 4)
	for i := 0; i < cap(outcomes); i++ {
		wg.Add(1)
		go func() { defer wg.Done(); outcomes <- r.Complete(ctx, response) }()
	}
	wg.Wait()
	close(outcomes)
	for err := range outcomes {
		if err != nil {
			t.Fatal("identical concurrent completion diverged", err)
		}
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_results WHERE intent_id=?`, intent.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("completion duplicated or disappeared", count, err)
	}
	_, result, err := r.Read(ctx)
	if err != nil || result.SHA256 != response.SHA256 || string(result.Body) != string(response.Body) {
		t.Fatal("concurrent completion corrupted original", err)
	}
}
