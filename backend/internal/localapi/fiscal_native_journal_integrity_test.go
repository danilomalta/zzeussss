package localapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
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

func TestNativeFiscalJournalEncryptedRowsAndMissingKey(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	key := syntheticNativeJournalKey(t)
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", "")
	if err := r.Reserve(ctx, in); !errors.Is(err, fiscal.ErrJournalKey) {
		t.Fatal("no private key allowed an unencrypted reservation", err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("missing-key write survived", count, err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", key)
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
	}
	response := nativeJournalResponse(in)
	if err := r.Complete(ctx, response); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		table, column string
		body          []byte
	}{
		{"fiscal_transmission_requests", "request_json", in.Body},
		{"fiscal_transmission_results", "result_json", response.Body},
	} {
		var stored string
		if err := f.db.QueryRow(`SELECT `+c.column+` FROM `+c.table+` WHERE intent_id=?`, intent.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(stored, "titan-fiscal-journal:v1:") || strings.Contains(stored, string(c.body)) || strings.Contains(stored, base64.StdEncoding.EncodeToString(c.body)) {
			t.Fatal("native journal stored readable payload")
		}
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", "")
	if _, _, err := r.Read(ctx); err == nil {
		t.Fatal("encrypted journal readable without key")
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", key)
	request, result, err := r.Read(ctx)
	if err != nil || string(request.Body) != string(in.Body) || string(result.Body) != string(response.Body) {
		t.Fatal("original proof was not recovered", err)
	}
}

type nativeEventJournal interface {
	Reserve(context.Context, fiscalcore.RecordedRequest) error
	Complete(context.Context, fiscalcore.ExchangeResult) error
	Read(context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error)
}

func TestNativeFiscalEventJournalIgnoredWriteRollsBack(t *testing.T) {
	for _, kind := range []string{"query", "cancellation", "invalidation"} {
		for _, phase := range []string{"requests", "results"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				var f *stockHTTPFixture
				var recorder nativeEventJournal
				var in fiscalcore.RecordedRequest
				switch kind {
				case "query":
					fixture, intent, _, _ := queryOriginalFixture(t)
					f = fixture
					r := querySQLRecorder(t, f, f.db, intent, "encrypted-query", f.token)
					recorder, in = r, querySQLRequest(t, r, "encrypted-query")
				case "cancellation":
					fixture, intent, cancellation := cancellationSQLFixture(t)
					f = fixture
					r := cancellationSQLRecorder(t, f, f.db, intent, "encrypted-cancel", f.token)
					recorder, in = r, cancellationSQLRequest(t, r, f, "encrypted-cancel", cancellation)
				case "invalidation":
					fixture, invalidation := invalidationSQLFixture(t)
					f = fixture
					r := invalidationSQLRecorder(t, f, f.db, "encrypted-invalidate", f.token, invalidation)
					recorder, in = r, invalidationSQLRequest(t, r, "encrypted-invalidate", invalidation)
				}
				ctx := context.Background()
				response := nativeJournalResponse(in)
				if phase == "results" {
					if err := recorder.Reserve(ctx, in); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := f.db.Exec(`CREATE TRIGGER test_encrypted_event_ignore BEFORE INSERT ON fiscal_` + kind + `_` + phase + ` BEGIN SELECT RAISE(IGNORE); END`); err != nil {
					t.Fatal(err)
				}
				var err error
				if phase == "requests" {
					err = recorder.Reserve(ctx, in)
				} else {
					err = recorder.Complete(ctx, response)
				}
				if !errors.Is(err, fiscal.ErrConflict) {
					t.Fatal("ignored event write was not a conflict", err)
				}
				if phase == "results" {
					_, result, err := recorder.Read(ctx)
					if err != nil || result.State != "uncertain" {
						t.Fatal("failed result lost the request or fabricated evidence", err)
					}
					if err := recorder.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
						t.Fatal("failed completion allowed another send", err)
					}
				}
				if _, err := f.db.Exec(`DROP TRIGGER test_encrypted_event_ignore`); err != nil {
					t.Fatal(err)
				}
				if phase == "requests" {
					if err := recorder.Reserve(ctx, in); err != nil {
						t.Fatal("recovery changed the request", err)
					}
				}
				if err := recorder.Complete(ctx, response); err != nil {
					t.Fatal("recovery of original result", err)
				}
				if err := recorder.Complete(ctx, response); err != nil {
					t.Fatal("idempotent result", err)
				}
				original, result, err := recorder.Read(ctx)
				if err != nil || original.OperationID != in.OperationID || result.SHA256 != response.SHA256 {
					t.Fatal("original identity or result changed", err)
				}
			})
		}
	}
}
