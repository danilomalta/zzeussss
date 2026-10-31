package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func journalKey(t *testing.T) string {
	t.Helper()
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	return key
}
func journalLicense(t *testing.T, f *stockHTTPFixture, db *sql.DB) *entitlementstore.Store {
	t.Helper()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	return license
}
func fiscalJournal(t *testing.T, f *stockHTTPFixture, db *sql.DB, intent fiscal.Intent, token string) *fiscal.HomologationRecorder {
	t.Helper()
	a := identity.Scope{TenantID: f.device.TenantID, StoreID: f.device.StoreID, IdentityID: f.owner.OwnerID}
	r, err := fiscal.NewHomologationRecorder(context.Background(), db, journalLicense(t, f, db), a, f.device, intent.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", journalKey(t), token)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func fiscalRecorded(t *testing.T, r *fiscal.HomologationRecorder) fiscalcore.RecordedRequest {
	t.Helper()
	body := []byte("synthetic technical request; not an invoice")
	sum := sha256.Sum256(body)
	return fiscalcore.RecordedRequest{OperationID: "transmit-one", Scope: r.Scope(), URL: "https://homologacao.example.gov.br/NFeAutorizacao4", Issuer: "12ABC34501DE35", Environment: "2", Key: journalKey(t), RequestSHA256: hex.EncodeToString(sum[:]), Body: body}
}
func TestFiscalTransmissionConcurrentDurableReservationAndImmutableResult(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Reserve(ctx, in) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, fiscalcore.ErrRecorded) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
	req, result, err := r.Read(ctx)
	if err != nil || req.OperationID != in.OperationID || result.State != "uncertain" {
		t.Fatal(err, result.State)
	}
	if err = r.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal(err)
	}
	for _, table := range []string{"fiscal_transmission_requests", "fiscal_intent_session_bindings"} {
		if _, err = f.db.Exec(`DELETE FROM ` + table); err == nil {
			t.Fatal("immutable record removed", table)
		}
	}
	response := fiscalcore.ExchangeResult{OperationID: in.OperationID, State: "response_received", HTTPStatus: 200, Body: []byte("technical response; not authorization")}
	sum := sha256.Sum256(response.Body)
	response.SHA256 = hex.EncodeToString(sum[:])
	if err = r.Complete(ctx, response); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete(ctx, response); err != nil {
		t.Fatal("idempotent completion", err)
	}
	changed := response
	changed.HTTPStatus = 500
	if err = r.Complete(ctx, changed); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("overwritten result", err)
	}
	_, result, err = r.Read(ctx)
	if err != nil || result.HTTPStatus != 200 || string(result.Body) != string(response.Body) {
		t.Fatal("original response lost", err)
	}
	if intent.Readiness.OfficialAuthorization {
		t.Fatal("fabricated fiscal authorization")
	}
}
func TestFiscalTransmissionRejectsWrongBindingBeforeReservation(t *testing.T) {
	for _, field := range []string{"scope", "issuer", "environment", "url", "key", "hash", "empty", "operation"} {
		t.Run(field, func(t *testing.T) {
			f := fiscalFixture(t)
			intent := fiscalIntent(t, f, fiscalIntentBody, 201)
			r := fiscalJournal(t, f, f.db, intent, f.token)
			in := fiscalRecorded(t, r)
			switch field {
			case "scope":
				in.Scope = "another-store"
			case "issuer":
				in.Issuer = "foreign"
			case "environment":
				in.Environment = "1"
			case "url":
				in.URL = "https://other.gov.br"
			case "key":
				in.Key = "foreign"
			case "hash":
				in.RequestSHA256 = strings.Repeat("0", 64)
			case "empty":
				in.Body = nil
			case "operation":
				in.OperationID = "invalid/id"
			}
			if err := r.Reserve(context.Background(), in); !errors.Is(err, fiscal.ErrInvalid) {
				t.Fatal(err)
			}
			var n int
			if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&n); err != nil || n != 0 {
				t.Fatal(n, err)
			}
		})
	}
}
func TestFiscalTransmissionLogoutAndNewSessionNeverResumeOriginalIntent(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.Reserve(ctx, in); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("old session used", err)
	}
	token := loginToken(t, f.app, f.owner.OwnerID)
	next := fiscalJournal(t, f, f.db, intent, token)
	if err := next.Reserve(ctx, in); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("new session resumed old attempt", err)
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
func TestFiscalTransmissionLogoutPreservesResponseAndRecoveryWithoutPost(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
	}
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, fiscalcore.ExchangeResult{OperationID: in.OperationID, State: "uncertain"}); err != nil {
		t.Fatal("response proof lost on logout", err)
	}
	token := loginToken(t, f.app, f.owner.OwnerID)
	next := fiscalJournal(t, f, f.db, intent, token)
	req, res, err := next.Read(ctx)
	if err != nil || req.RequestSHA256 != in.RequestSHA256 || res.State != "uncertain" {
		t.Fatal(err)
	}
	if err := next.Reserve(ctx, in); err == nil {
		t.Fatal("recovered by new POST")
	}
}
func TestFiscalTransmissionProfileRevisionBlocksOldPreparedAttempt(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	changed := strings.Replace(fiscalProfileBody, `"issuer-create"`, `"issuer-next"`, 1)
	changed = strings.Replace(changed, `"expected_revision":0`, `"expected_revision":1`, 1)
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", changed, f.token)
	if code != 201 {
		t.Fatal(code)
	}
	if err := r.Reserve(context.Background(), in); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal(err)
	}
}
func TestFiscalTransmissionBackupRestorePreservesOriginalAndBlocksUnrecordedAttempt(t *testing.T) {
	for _, state := range []string{"snapshot-before-reservation", "snapshot-after-reservation", "snapshot-after-response"} {
		t.Run(state, func(t *testing.T) {
			reserve := state != "snapshot-before-reservation"
			expectedState := "uncertain"
			expectedResponseHash := ""
			f := fiscalFixture(t)
			intent := fiscalIntent(t, f, fiscalIntentBody, 201)
			r := fiscalJournal(t, f, f.db, intent, f.token)
			in := fiscalRecorded(t, r)
			ctx := context.Background()
			if reserve {
				if err := r.Reserve(ctx, in); err != nil {
					t.Fatal(err)
				}
			}
			if state == "snapshot-after-response" {
				response := fiscalcore.ExchangeResult{OperationID: in.OperationID, State: "response_received", HTTPStatus: 200, Body: []byte("synthetic stored response, not authorization")}
				digest := sha256.Sum256(response.Body)
				response.SHA256 = hex.EncodeToString(digest[:])
				expectedState = response.State
				expectedResponseHash = response.SHA256
				if err := r.Complete(ctx, response); err != nil {
					t.Fatal(err)
				}
			}
			key := make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			archive := filepath.Join(dir, "backup.tytbak")
			dest := filepath.Join(dir, "restored.sqlite")
			if *pilotBackupCLI != "" {
				station := pilotPrepareCLIStation(t, f)
				var seq int
				var name, path string
				if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
					t.Fatal(err)
				}
				dest = pilotCLIBackupRoundTrip(t, f, path, station)
			} else {
				if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
					t.Fatal(err)
				}
				if err := backup.Verify(ctx, archive, f.device, key); err != nil {
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
			next := fiscalJournal(t, f, db, intent, token)
			if err := next.Reserve(ctx, in); err == nil {
				t.Fatal("restored attempt transmitted")
			}
			var blocked int
			if err := db.QueryRow(`SELECT count(*) FROM fiscal_restore_barriers`).Scan(&blocked); err != nil || blocked != 1 {
				t.Fatal(blocked, err)
			}
			if reserve {
				req, res, err := next.Read(ctx)
				if err != nil || req.OperationID != in.OperationID || req.RequestSHA256 != in.RequestSHA256 || res.State != expectedState || res.SHA256 != expectedResponseHash {
					t.Fatal("restored proof differs", err)
				}
			}
			// Fresh post-restore session AND fresh intent still cannot transmit.
			copyHTTP := *f.httpContractFixture
			copyHTTP.db = db
			copyHTTP.app = app
			copyHTTP.token = token
			restoredF := *f
			restoredF.httpContractFixture = &copyHTTP
			nextSale := salesInput(&restoredF)
			nextSale.SaleID = "sale-after-restore"
			nextSale.OperationID = "sale-op-after-restore"
			salesRequest(t, &restoredF, nextSale, 201)
			nextBody := strings.Replace(fiscalIntentBody, "fiscal-one", "fiscal-after-restore", 1)
			nextBody = strings.Replace(nextBody, "sale-one", "sale-after-restore", 1)
			nextIntent := fiscalIntent(t, &restoredF, nextBody, 201)
			fresh := fiscalJournal(t, &restoredF, db, nextIntent, token)
			if err := fresh.Reserve(ctx, fiscalRecorded(t, fresh)); !errors.Is(err, fiscal.ErrRestored) {
				t.Fatal("fresh intent bypassed restore fence", err)
			}
			if _, err := db.Exec(`DELETE FROM fiscal_restore_barriers`); err == nil {
				t.Fatal("restore barrier deleted")
			}
		})
	}
}

func TestFiscalTransmissionRestartPreservesOriginalUncertainRequest(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	ctx := context.Background()
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
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
	next := fiscalJournal(t, f, db, intent, f.token)
	req, result, err := next.Read(ctx)
	if err != nil || req.OperationID != in.OperationID || req.RequestSHA256 != in.RequestSHA256 || result.State != "uncertain" {
		t.Fatal("restart lost original", err)
	}
	if err = next.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("restart caused second POST", err)
	}
}
func TestFiscalTransmissionProductionConstructorRemainsUnavailable(t *testing.T) {
	f := fiscalFixture(t)
	profile := strings.Replace(fiscalProfileBody, "issuer-create", "production-create", 1)
	profile = strings.Replace(profile, "issuer-one", "production-profile", 1)
	profile = strings.Replace(profile, "homologation", "production", 1)
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", profile, f.token)
	if code != 201 {
		t.Fatal(code)
	}
	body := strings.Replace(fiscalIntentBody, "fiscal-one", "production-intent", 1)
	body = strings.Replace(body, "issuer-one", "production-profile", 1)
	intent := fiscalIntent(t, f, body, 201)
	a := identity.Scope{TenantID: f.device.TenantID, StoreID: f.device.StoreID, IdentityID: f.owner.OwnerID}
	_, err := fiscal.NewHomologationRecorder(context.Background(), f.db, journalLicense(t, f, f.db), a, f.device, intent.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", journalKey(t), f.token)
	if !errors.Is(err, fiscal.ErrInvalid) {
		t.Fatal("production enabled without homologation", err)
	}
}

func TestFiscalTransmissionNumberCannotMoveToAnotherSaleOrKey(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	ctx := context.Background()
	if err := r.Reserve(ctx, fiscalRecorded(t, r)); err != nil {
		t.Fatal(err)
	}
	nextSale := salesInput(f)
	nextSale.SaleID = "sale-another"
	nextSale.OperationID = "sale-op-another"
	salesRequest(t, f, nextSale, 201)
	body := strings.Replace(fiscalIntentBody, "fiscal-one", "fiscal-another", 1)
	body = strings.Replace(body, "sale-one", "sale-another", 1)
	next := fiscalIntent(t, f, body, 201)
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "87654321"})
	if err != nil {
		t.Fatal(err)
	}
	a := identity.Scope{TenantID: f.device.TenantID, StoreID: f.device.StoreID, IdentityID: f.owner.OwnerID}
	recorder, err := fiscal.NewHomologationRecorder(ctx, f.db, journalLicense(t, f, f.db), a, f.device, next.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", key, f.token)
	if err != nil {
		t.Fatal(err)
	}
	in := fiscalRecorded(t, recorder)
	in.Key = key
	in.OperationID = "transmit-another"
	if err = recorder.Reserve(ctx, in); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("same number changed sale or cNF", err)
	}
}
func TestFiscalTransmissionContractRevocationBlocksReserveButPreservesProof(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	ctx := context.Background()
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
	if err := r.Reserve(ctx, fiscalRecorded(t, r)); err == nil {
		t.Fatal("missing Fiscal contract allowed reservation")
	}
}
func TestFiscalTransmissionContextMismatchAndMalformedCallback(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	ctx := context.Background()
	a := identity.Scope{TenantID: f.device.TenantID, StoreID: f.device.StoreID, IdentityID: f.owner.OwnerID}
	for _, field := range []string{"tenant", "store", "device", "actor"} {
		t.Run(field, func(t *testing.T) {
			actor, device := a, f.device
			switch field {
			case "tenant":
				actor.TenantID = "foreign"
			case "store":
				actor.StoreID = "foreign"
			case "device":
				device.DeviceID = "foreign"
			case "actor":
				actor.IdentityID = "foreign"
			}
			_, err := fiscal.NewHomologationRecorder(ctx, f.db, journalLicense(t, f, f.db), actor, device, intent.ID, "https://homologacao.example.gov.br/NFeAutorizacao4", journalKey(t), f.token)
			if err == nil {
				t.Fatal("foreign context accepted")
			}
		})
	}
	r := fiscalJournal(t, f, f.db, intent, f.token)
	in := fiscalRecorded(t, r)
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
	}
	for _, res := range []fiscalcore.ExchangeResult{{OperationID: in.OperationID, State: "authorized"}, {OperationID: in.OperationID, State: "response_received", HTTPStatus: 200, SHA256: "wrong"}, {OperationID: in.OperationID, State: "uncertain", Body: []byte("untrusted")}, {OperationID: "foreign", State: "uncertain"}} {
		if err := r.Complete(ctx, res); err == nil {
			t.Fatal("invalid callback accepted")
		}
	}
	_, res, err := r.Read(ctx)
	if err != nil || res.State != "uncertain" {
		t.Fatal("original uncertain request lost", err)
	}
}

func TestFiscalTransmissionLogoutBetweenReservationAndSendIsBlocked(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	ctx := context.Background()
	in := fiscalRecorded(t, r)
	if err := r.Reserve(ctx, in); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, in.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, in.OperationID); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("old authorization reached send", err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&count); err != nil || count != 1 {
		t.Fatal("lost original reservation", err)
	}
}
