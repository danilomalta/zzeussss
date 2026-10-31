package localapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
)

func fiscalConcurrentDBs(t *testing.T, f *stockHTTPFixture) []*sql.DB {
	t.Helper()
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	out := make([]*sql.DB, 2)
	for i := range out {
		db, err := localdb.Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		out[i] = db
	}
	return out
}
func TestFiscalConcurrentConfigureSingleRevision(t *testing.T) {
	f := fiscalFixture(t)
	dbs := fiscalConcurrentDBs(t, f)
	actor := numberingActor(f)
	var profile fiscal.ProfileInput
	if err := json.Unmarshal([]byte(fiscalProfileBody), &profile); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, db := range dbs {
		license := journalLicense(t, f, db)
		wg.Add(1)
		go func(i int, db *sql.DB) {
			defer wg.Done()
			<-start
			in := profile
			in.ExpectedRevision = 1
			in.OperationID = fmt.Sprintf("configure-race-%d", i)
			in.LegalName = fmt.Sprintf("Synthetic revision %d", i)
			_, errs[i] = fiscal.Configure(context.Background(), db, license, actor, f.device, in, f.token)
		}(i, db)
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, fiscal.ErrConflict) {
			t.Errorf("expected revision conflict: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("winners %d", wins)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_profiles WHERE revision=2`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate/missing revision")
	}
}
func TestFiscalConcurrentPrepareSingleIntent(t *testing.T) {
	f := fiscalFixture(t)
	dbs := fiscalConcurrentDBs(t, f)
	actor := numberingActor(f)
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, db := range dbs {
		license := journalLicense(t, f, db)
		wg.Add(1)
		go func(i int, db *sql.DB) {
			defer wg.Done()
			<-start
			_, errs[i] = fiscal.Prepare(context.Background(), db, license, actor, f.device, fiscal.IntentInput{OperationID: fmt.Sprintf("prepare-race-%d", i), SaleID: "sale-one", ProfileID: "issuer-one", ProfileRevision: 1}, f.token)
		}(i, db)
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, fiscal.ErrConflict) {
			t.Errorf("expected intent conflict: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("winners %d", wins)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_intents`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate/missing intent")
	}
}
