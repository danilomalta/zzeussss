package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
)

func numberingActor(f *stockHTTPFixture) identity.Scope {
	return identity.Scope{TenantID: f.device.TenantID, StoreID: f.device.StoreID, IdentityID: f.owner.OwnerID}
}
func numberingInput(intent fiscal.Intent) fiscal.NumberInput {
	return fiscal.NumberInput{OperationID: "number-" + intent.ID, IssuedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
}
func numberingOrigin(first int64) fiscal.SeriesOriginInput {
	return fiscal.SeriesOriginInput{OperationID: "series-origin-one", ProfileID: "issuer-one", ProfileRevision: 1, InitialNumber: first, ReconciliationReference: "synthetic-homologation-series-only"}
}
func numberingPrepare(t *testing.T, f *stockHTTPFixture, intent fiscal.Intent, first int64) fiscal.NumberAssignment {
	t.Helper()
	ctx := context.Background()
	license := journalLicense(t, f, f.db)
	a := numberingActor(f)
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, numberingOrigin(first), f.token); err != nil {
		t.Fatal(err)
	}
	out, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, intent.ID, numberingInput(intent), f.token)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func numberedFiscalJournal(t *testing.T, f *stockHTTPFixture, db *sql.DB, intent fiscal.Intent, token string, number fiscal.NumberAssignment) *fiscal.HomologationRecorder {
	t.Helper()
	r, err := fiscal.NewHomologationRecorder(context.Background(), db, journalLicense(t, f, db), numberingActor(f), f.device, intent.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", number.AccessKey, token)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func numberingNextIntent(t *testing.T, f *stockHTTPFixture, index int) fiscal.Intent {
	t.Helper()
	in := salesInput(f)
	in.SaleID = fmt.Sprintf("number-sale-%d", index)
	in.OperationID = fmt.Sprintf("number-sale-op-%d", index)
	stockRequest(t, f, stockEntry(f, fmt.Sprintf("number-entry-%d", index), 1000), 201)
	stockRequest(t, f, stock.Input{OperationID: fmt.Sprintf("number-transfer-%d", index), Kind: "transfer", ProductID: f.product, FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 1000, Reason: "Synthetic numbering test stock"}, 201)
	in.Items[0].QuantityMilli = 1000
	in.Payments[0].AmountCents = 1599
	salesRequest(t, f, in, 201)
	body := strings.Replace(fiscalIntentBody, "fiscal-one", fmt.Sprintf("number-intent-%d", index), 1)
	body = strings.Replace(body, "sale-one", in.SaleID, 1)
	return fiscalIntent(t, f, body, 201)
}

func TestFiscalNumberingExplicitOriginReplayAndImmutableKey(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	ctx := context.Background()
	a := numberingActor(f)
	license := journalLicense(t, f, f.db)
	if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, intent.ID, numberingInput(intent), f.token); !errors.Is(err, fiscal.ErrNotFound) {
		t.Fatal("implicit start at one", err)
	}
	number := numberingPrepare(t, f, intent, 41)
	if number.Number != 41 || !fiscalcore.ValidAccessKey(number.AccessKey) {
		t.Fatal(number.Number)
	}
	repeated, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, intent.ID, numberingInput(intent), f.token)
	if err != nil || !repeated.Repeated || repeated.AccessKey != number.AccessKey || repeated.IssuedAt != number.IssuedAt || repeated.OperationID != number.OperationID {
		t.Fatal("replay changed original", err)
	}
	altered := numberingInput(intent)
	altered.IssuedAt = altered.IssuedAt.Add(time.Second)
	if _, err = fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, intent.ID, altered, f.token); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("rewrote issue time", err)
	}
	origin := numberingOrigin(41)
	if err = fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, origin, f.token); err != nil {
		t.Fatal("origin replay", err)
	}
	origin.InitialNumber = 1
	if err = fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, origin, f.token); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("reset series", err)
	}
	for _, q := range []string{`UPDATE fiscal_number_allocations SET number=1`, `DELETE FROM fiscal_number_allocations`, `UPDATE fiscal_number_series SET initial_number=1`, `DELETE FROM fiscal_number_series`} {
		if _, err = f.db.Exec(q); err == nil {
			t.Fatal("immutable fiscal number changed", q)
		}
	}
	var n int
	if err = f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&n); err != nil || n != 0 {
		t.Fatal("number reservation sent a document", err)
	}
}

func TestFiscalNumberingConcurrentHandlesAllocateWithoutDuplicates(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	numberingPrepare(t, f, intent, 41)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	other, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	intents := make([]fiscal.Intent, 8)
	for i := range intents {
		intents[i] = numberingNextIntent(t, f, i)
	}
	type outcome struct {
		number fiscal.NumberAssignment
		err    error
	}
	results := make(chan outcome, len(intents))
	var wg sync.WaitGroup
	handles := []*sql.DB{f.db, other}
	for i, intent := range intents {
		db := handles[i%2]
		license := journalLicense(t, f, db)
		wg.Add(1)
		go func(intent fiscal.Intent) {
			defer wg.Done()
			out, err := fiscal.AllocateHomologationNumber(context.Background(), db, license, numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
			results <- outcome{out, err}
		}(intent)
	}
	wg.Wait()
	close(results)
	seen := map[int64]bool{}
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		n := result.number.Number
		if n < 42 || n > 49 || seen[n] {
			t.Fatal("duplicate or out-of-range allocation", n)
		}
		seen[n] = true
	}
	if len(seen) != 8 {
		t.Fatal("lost allocations", len(seen))
	}
}

func TestFiscalNumberingConcurrentReplayReservesOnlyOneKey(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	ctx := context.Background()
	license := journalLicense(t, f, f.db)
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, numberingActor(f), f.device, numberingOrigin(1), f.token); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		out fiscal.NumberAssignment
		err error
	}
	results := make(chan outcome, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
			results <- outcome{out, err}
		}()
	}
	wg.Wait()
	close(results)
	var original string
	created := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if original == "" {
			original = result.out.AccessKey
		}
		if result.out.AccessKey != original || result.out.Number != 1 {
			t.Fatal("replay changed key")
		}
		if !result.out.Repeated {
			created++
		}
	}
	if created != 1 {
		t.Fatal("more than one fresh allocation", created)
	}
}

func TestFiscalNumberingLogoutRecoveryAndRestart(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	original := numberingPrepare(t, f, intent, 1)
	ctx := context.Background()
	a := numberingActor(f)
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	nextToken := loginToken(t, f.app, f.owner.OwnerID)
	for _, token := range []string{f.token, nextToken} {
		if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, journalLicense(t, f, f.db), a, f.device, intent.ID, numberingInput(intent), token); !errors.Is(err, localauth.ErrDenied) {
			t.Fatal("session took over original allocation", err)
		}
	}
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recovered, err := fiscal.ReadHomologationNumber(ctx, db, a, f.device, intent.ID, nextToken)
	if err != nil || recovered.AccessKey != original.AccessKey || recovered.CreatedAt != original.CreatedAt || recovered.IssuedAt != original.IssuedAt {
		t.Fatal("restart lost original", err)
	}
}

func TestFiscalNumberingLegacyRequestsCannotStealOrReassignNumbers(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	number := numberingPrepare(t, f, intent, 1)
	next := numberingNextIntent(t, f, 1)
	manual := numberedFiscalJournal(t, f, f.db, next, f.token, number)
	in := fiscalRecorded(t, manual)
	in.Key = number.AccessKey
	if err := manual.Reserve(context.Background(), in); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("stole assigned number", err)
	}
	own := numberedFiscalJournal(t, f, f.db, intent, f.token, number)
	in = fiscalRecorded(t, own)
	in.Key = number.AccessKey
	if err := own.Reserve(context.Background(), in); err != nil {
		t.Fatal("assigned number refused", err)
	}
	old, err := fiscal.AllocateHomologationNumber(context.Background(), f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, intent.ID, numberingInput(intent), f.token)
	if err != nil || old.AccessKey != number.AccessKey {
		t.Fatal("journal changed allocation", err)
	}
}

func TestFiscalNumberingSeedCannotGoBehindLegacyJournal(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	ctx := context.Background()
	r := fiscalJournal(t, f, f.db, intent, f.token)
	if err := r.Reserve(ctx, fiscalRecorded(t, r)); err != nil {
		t.Fatal(err)
	}
	license := journalLicense(t, f, f.db)
	a := numberingActor(f)
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, numberingOrigin(1), f.token); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("seed ignored legacy request", err)
	}
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, numberingOrigin(2), f.token); err != nil {
		t.Fatal(err)
	}
	if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, intent.ID, numberingInput(intent), f.token); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("legacy intent assigned new key", err)
	}
	next := numberingNextIntent(t, f, 1)
	n, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, f.device, next.ID, numberingInput(next), f.token)
	if err != nil || n.Number != 2 {
		t.Fatal("next number after legacy request", n.Number, err)
	}
}

func TestFiscalNumberingExhaustionNeverWraps(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	number := numberingPrepare(t, f, intent, 999999999)
	if number.Number != 999999999 {
		t.Fatal(number.Number)
	}
	next := numberingNextIntent(t, f, 1)
	if _, err := fiscal.AllocateHomologationNumber(context.Background(), f.db, journalLicense(t, f, f.db), numberingActor(f), f.device, next.ID, numberingInput(next), f.token); !errors.Is(err, fiscal.ErrNumberExhausted) {
		t.Fatal("wrapped exhausted series", err)
	}
}

func TestFiscalNumberingRestorePreservesOriginalAndBlocksFreshAllocation(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	original := numberingPrepare(t, f, intent, 1)
	ctx := context.Background()
	dir := t.TempDir()
	var dest string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, f)
		var seq int
		var name, path string
		if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
			t.Fatal(err)
		}
		dest = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(dir, "numbering.tytbak")
		dest = filepath.Join(dir, "restored.sqlite")
		if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err := backup.Restore(ctx, archive, dest, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	db, err := localdb.Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	a := numberingActor(f)
	read, err := fiscal.ReadHomologationNumber(ctx, db, a, f.device, intent.ID, token)
	if err != nil || read.AccessKey != original.AccessKey || read.Number != original.Number {
		t.Fatal("restored original changed", err)
	}
	if err = fiscal.ConfigureHomologationSeries(ctx, db, journalLicense(t, f, db), a, f.device, numberingOrigin(1), token); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("restored series reconfigured", err)
	}
	// A fresh intent after restore also cannot advance the sequence.
	restored := *f
	copiedHTTP := *f.httpContractFixture
	restored.httpContractFixture = &copiedHTTP
	restored.db = db
	restored.app = app
	restored.token = token
	next := numberingNextIntent(t, &restored, 1)
	if _, err = fiscal.AllocateHomologationNumber(ctx, db, journalLicense(t, f, db), a, f.device, next.ID, numberingInput(next), token); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("restored installation allocated", err)
	}
}

func TestFiscalNumberingDeniedContextsAndRevokedContractDoNotAdvance(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	original := numberingPrepare(t, f, intent, 1)
	next := numberingNextIntent(t, f, 1)
	ctx := context.Background()
	license := journalLicense(t, f, f.db)
	for _, dimension := range []string{"tenant", "store", "actor", "device"} {
		t.Run(dimension, func(t *testing.T) {
			a, d := numberingActor(f), f.device
			switch dimension {
			case "tenant":
				a.TenantID = "foreign"
				d.TenantID = "foreign"
			case "store":
				a.StoreID = "foreign"
				d.StoreID = "foreign"
			case "actor":
				a.IdentityID = "foreign"
			case "device":
				d.DeviceID = "foreign"
			}
			if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, a, d, next.ID, numberingInput(next), f.token); err == nil {
				t.Fatal("foreign context allocated number")
			}
		})
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	if _, err := fiscal.AllocateHomologationNumber(ctx, f.db, license, numberingActor(f), f.device, next.ID, numberingInput(next), f.token); err == nil {
		t.Fatal("revoked fiscal contract allocated")
	}
	recovered, err := fiscal.ReadHomologationNumber(ctx, f.db, numberingActor(f), f.device, intent.ID, f.token)
	if err != nil || recovered.AccessKey != original.AccessKey {
		t.Fatal("revocation hid historical assignment", err)
	}
	var count int
	if err = f.db.QueryRow(`SELECT count(*) FROM fiscal_number_allocations`).Scan(&count); err != nil || count != 1 {
		t.Fatal("denied operation advanced sequence", count, err)
	}
}

func TestFiscalNumberingOnlyOwnerMayEstablishOrigin(t *testing.T) {
	f := fiscalFixture(t)
	ctx := context.Background()
	license := journalLicense(t, f, f.db)
	for _, role := range []string{"manager", "accountant", "cashier"} {
		t.Run(role, func(t *testing.T) {
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"numbering-create-`+role+`","identity_id":"numbering-`+role+`","name":"Synthetic staff","role":"`+role+`","password":"synthetic-staff-password-only"}`, f.token)
			if code != 201 {
				t.Fatal(code)
			}
			code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"numbering-`+role+`","password":"synthetic-staff-password-only"}`, "")
			if code != 200 {
				t.Fatal(code)
			}
			var auth struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(body, &auth); err != nil {
				t.Fatal(err)
			}
			a := numberingActor(f)
			a.IdentityID = "numbering-" + role
			if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, numberingOrigin(1), auth.Token); err == nil {
				t.Fatal("non-owner configured origin")
			}
		})
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_number_series`).Scan(&count); err != nil || count != 0 {
		t.Fatal("denial created series", count, err)
	}
}

func TestFiscalNumberingOriginInputAndProductionFailClosed(t *testing.T) {
	f := fiscalFixture(t)
	ctx := context.Background()
	license := journalLicense(t, f, f.db)
	a := numberingActor(f)
	for _, mutate := range []func(*fiscal.SeriesOriginInput){func(in *fiscal.SeriesOriginInput) { in.InitialNumber = 0 }, func(in *fiscal.SeriesOriginInput) { in.InitialNumber = 1000000000 }, func(in *fiscal.SeriesOriginInput) { in.ReconciliationReference = "" }, func(in *fiscal.SeriesOriginInput) { in.ReconciliationReference = "bad\nreference" }, func(in *fiscal.SeriesOriginInput) { in.ProfileRevision = 0 }} {
		in := numberingOrigin(1)
		mutate(&in)
		if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, in, f.token); !errors.Is(err, fiscal.ErrInvalid) {
			t.Fatal(err)
		}
	}
	revised := strings.Replace(fiscalProfileBody, "issuer-create", "issuer-production", 1)
	revised = strings.Replace(revised, `"expected_revision":0`, `"expected_revision":1`, 1)
	revised = strings.Replace(revised, `"homologation"`, `"production"`, 1)
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", revised, f.token)
	if code != 201 {
		t.Fatal(code)
	}
	in := numberingOrigin(1)
	in.ProfileRevision = 2
	if err := fiscal.ConfigureHomologationSeries(ctx, f.db, license, a, f.device, in, f.token); !errors.Is(err, fiscalcore.ErrUnsupported) {
		t.Fatal("production numbered automatically", err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_number_series`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid origin created state", count, err)
	}
}
