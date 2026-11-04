package localapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/retail"
)

func storesSetup(t *testing.T) (*stockHTTPFixture, retail.Service) {
	t.Helper()
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	s := retailServiceFor(f.httpContractFixture)
	if e := s.Configure(context.Background(), "stores-config", retail.Config{MultistoreEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}}); e != nil {
		t.Fatal(e)
	}
	if e := s.CreateStore(context.Background(), "store-two-op", "store-two", "Synthetic second store"); e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{`INSERT INTO devices VALUES(?,'store-two','target-device','Synthetic custodian')`, `INSERT INTO stock_locations VALUES(?,'store-two','target-location','backroom','Synthetic location')`, `INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by,proof_unix,approved_by,approved_unix) SELECT tenant_id,'store-two','target-device',randomblob(32),randomblob(32),challenge_expires_unix,'approved',requested_by,proof_unix,approved_by,approved_unix FROM device_pairings WHERE tenant_id=? LIMIT 1`} {
		if _, e := f.db.Exec(q, f.owner.TenantID); e != nil {
			t.Fatal(e)
		}
	}
	return f, s
}
func transferInput(op string) retail.TransferInput {
	return retail.TransferInput{OperationID: op, DestinationStoreID: "store-two", DestinationCustodianID: "target-device", ProductID: "", FromLocationID: "", ToLocationID: "target-location", QuantityMilli: 1000, ExpectedFromMilli: 5000, ExpectedToMilli: 0, Confirmed: true}
}
func TestRetailConcurrentStoreTransfer(t *testing.T) {
	f, s := storesSetup(t)
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := transferInput(fmt.Sprintf("transfer-%d", i))
			in.ProductID = f.product
			in.FromLocationID = f.shelf
			errs <- s.Transfer(context.Background(), in)
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, retail.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	assertStockBalance(t, f, f.shelf, 4000)
	var received int64
	if e := f.db.QueryRow(`SELECT sum(quantity_milli) FROM stock_movements WHERE store_id='store-two'`).Scan(&received); e != nil || received != 1000 {
		t.Fatal(received, e)
	}
	var winner string
	f.db.QueryRow(`SELECT operation_id FROM retail_store_transfers`).Scan(&winner)
	in := transferInput(winner)
	in.ProductID = f.product
	in.FromLocationID = f.shelf
	if e := s.Transfer(context.Background(), in); e != nil {
		t.Fatal("replay", e)
	}
	in.QuantityMilli = 2000
	if e := s.Transfer(context.Background(), in); !errors.Is(e, retail.ErrConflict) {
		t.Fatal("changed replay", e)
	}
}
func TestRetailConcurrentStoreCreation(t *testing.T) {
	_, s := storesSetup(t)
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- s.CreateStore(context.Background(), fmt.Sprintf("create-%d", i), "same-store", "Explicit name")
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, retail.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
}
func TestRetailStoreTransferRollbackAndForeignScope(t *testing.T) {
	f, s := storesSetup(t)
	in := transferInput("rollback")
	in.ProductID = f.product
	in.FromLocationID = f.shelf
	if _, e := f.db.Exec(`CREATE TRIGGER abort_transfer BEFORE INSERT ON retail_store_transfers BEGIN SELECT RAISE(ABORT,'synthetic rollback'); END`); e != nil {
		t.Fatal(e)
	}
	if e := s.Transfer(context.Background(), in); e == nil {
		t.Fatal("expected rollback")
	}
	assertStockBalance(t, f, f.shelf, 5000)
	var n int
	f.db.QueryRow(`SELECT count(*) FROM stock_movements WHERE store_id='store-two'`).Scan(&n)
	if n != 0 {
		t.Fatal("partial transfer")
	}
	in.OperationID = "foreign"
	in.DestinationStoreID = "foreign-store"
	if e := s.Transfer(context.Background(), in); !errors.Is(e, identity.ErrDenied) {
		t.Fatal(e)
	}
}
func TestRetailStoreReportsPermissionsAndCurrentScope(t *testing.T) {
	f, s := storesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	var day string
	if e := f.db.QueryRow(`SELECT substr(committed_at,1,10) FROM sales WHERE id='sale-one'`).Scan(&day); e != nil {
		t.Fatal(e)
	}
	report, e := s.StoreReport(context.Background(), "", day, day, true)
	if e != nil || len(report.Totals) != 2 || report.GrossSalesCents != 3198 {
		t.Fatal(report, e)
	}
	if _, e = f.db.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	s = retailServiceFor(f.httpContractFixture)
	report, e = s.StoreReport(context.Background(), "", day, day, true)
	if e != nil || len(report.Totals) != 1 || report.Totals[0].StoreID != f.owner.StoreID {
		t.Fatal("scope leak", report, e)
	}
	if _, e = s.StoreReport(context.Background(), "store-two", day, day, false); !errors.Is(e, identity.ErrDenied) {
		t.Fatal("cross-store report", e)
	}
	in := transferInput("forbidden-target")
	in.ProductID = f.product
	in.FromLocationID = f.shelf
	in.ExpectedFromMilli = 3000
	if e = s.Transfer(context.Background(), in); !errors.Is(e, identity.ErrDenied) {
		t.Fatal("cross-store command", e)
	}
}
func TestRetailConcurrentRetirementAndCommand(t *testing.T) {
	_, s := storesSetup(t)
	const op = "uncertain-store"
	var commandErr, errorResult error
	var result retail.Resolution
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		commandErr = s.CreateStore(context.Background(), op, "uncertain-store", "Synthetic")
	}()
	go func() { defer wg.Done(); result, errorResult = s.Reconcile(context.Background(), op, true) }()
	wg.Wait()
	if errorResult != nil {
		t.Fatal(errorResult)
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM stores WHERE id='uncertain-store'`).Scan(&n)
	if result.Applied {
		if commandErr != nil || n != 1 {
			t.Fatal("applied result mismatch", commandErr, n)
		}
	} else {
		if !result.Retired || !errors.Is(commandErr, retail.ErrConflict) || n != 0 {
			t.Fatal("retired command applied", result, commandErr, n)
		}
	}
	if _, e := s.Reconcile(context.Background(), op, true); e != nil {
		t.Fatal("repeat", e)
	}
}
func TestHTTPRetailStoreAndRecoveryContracts(t *testing.T) {
	f, _ := storesSetup(t)
	day := time.Now().UTC().Format("2006-01-02")
	code, raw := request(t, f.app, "GET", "/local/v1/retail/stores/report?all_stores=true&from_on="+day+"&to_on="+day, "", f.token)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	code, raw = request(t, f.app, "POST", "/local/v1/retail/operations/delayed-store/reconcile", `{"confirmed":true}`, f.token)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	code, _ = request(t, f.app, "POST", "/local/v1/retail/stores", `{"operation_id":"delayed-store","id":"late-store","name":"Synthetic"}`, f.token)
	if code != 409 {
		t.Fatal("late HTTP command", code)
	}
}

func TestRetailStoreTransferBackupRestore(t *testing.T) {
	f, s := storesSetup(t)
	in := transferInput("restore-transfer")
	in.ProductID = f.product
	in.FromLocationID = f.shelf
	if e := s.Transfer(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "stores.tytbak")
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Restore(context.Background(), archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var balance, n int64
	if e = db.QueryRow(`SELECT sum(quantity_milli) FROM stock_movements WHERE store_id='store-two'`).Scan(&balance); e != nil || balance != 1000 {
		t.Fatal(balance, e)
	}
	if e = db.QueryRow(`SELECT count(*) FROM retail_store_transfers`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	if _, e = localauth.Resolve(context.Background(), db, f.token); !errors.Is(e, localauth.ErrDenied) {
		t.Fatal("session survived restore", e)
	}
}
func TestRetailTransferPreservesReservedStock(t *testing.T) {
	f, reserve := materialsFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Production, modules.Finance}), 200)
	s := retailServiceFor(f)
	if e := s.Configure(context.Background(), "reserved-stores-config", retail.Config{MultistoreEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}}); e != nil {
		t.Fatal(e)
	}
	if e := s.CreateStore(context.Background(), "target-store", "store-two", "Synthetic"); e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{`INSERT INTO devices VALUES(?,'store-two','target-device','Synthetic custodian')`, `INSERT INTO stock_locations VALUES(?,'store-two','target-location','backroom','Synthetic')`, `INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by,proof_unix,approved_by,approved_unix) SELECT tenant_id,'store-two','target-device',randomblob(32),randomblob(32),challenge_expires_unix,'approved',requested_by,proof_unix,approved_by,approved_unix FROM device_pairings WHERE tenant_id=? LIMIT 1`} {
		if _, e := f.db.Exec(q, f.owner.TenantID); e != nil {
			t.Fatal(e)
		}
	}
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	in := transferInput("cannot-spend-reserved")
	in.ProductID = "flour"
	in.FromLocationID = "production-room"
	in.QuantityMilli = 1000000
	in.ExpectedFromMilli = 2000000
	if e := s.Transfer(context.Background(), in); !errors.Is(e, retail.ErrConflict) {
		t.Fatal("reserved stock spent", e)
	}
}
func TestRetailLoyaltyLedgerDoesNotLeakOtherStore(t *testing.T) {
	f, s := loyaltySetup(t)
	cfg := retail.Config{Revision: 2, LoyaltyEnabled: true, MultistoreEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}, EarnCents: 1, EarnPoints: 1, RedeemPoints: 1, RedeemCents: 1, ValidDays: 30}
	if e := s.Configure(context.Background(), "loyalty-stores-policy", cfg); e != nil {
		t.Fatal(e)
	}
	if e := s.CreateStore(context.Background(), "isolated-store", "store-two", "Synthetic"); e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{`INSERT INTO devices VALUES(?,'store-two','target-device','Synthetic')`, `INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by,proof_unix,approved_by,approved_unix) SELECT tenant_id,'store-two','target-device',randomblob(32),randomblob(32),challenge_expires_unix,'approved',requested_by,proof_unix,approved_by,approved_unix FROM device_pairings WHERE tenant_id=? LIMIT 1`} {
		if _, e := f.db.Exec(q, f.owner.TenantID); e != nil {
			t.Fatal(e)
		}
	}
	target := identity.DeviceContext{TenantID: f.owner.TenantID, StoreID: "store-two", DeviceID: "target-device"}
	session, e := localauth.Login(context.Background(), f.db, target, f.owner.OwnerID, "senha-forte-de-teste-1")
	if e != nil {
		t.Fatal(e)
	}
	other := retail.Service{DB: f.db, Contracts: s.Contracts, Actor: session.Actor, Device: target, Token: session.Token}
	out, e := other.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 0 || len(out.Entries) != 0 || len(out.Benefits) != 0 {
		t.Fatal("foreign store points", out, e)
	}
	if e = other.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: "foreign-store-points", Points: 1, ExpectedBalance: 0, Confirmed: true}); !errors.Is(e, retail.ErrConflict) {
		t.Fatal(e)
	}
}
func TestRetailInventoryOnlyStoreWorkflow(t *testing.T) {
	f, s := storesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 10, []modules.ID{modules.Core, modules.Inventory}), 200)
	in := transferInput("inventory-only-transfer")
	in.ProductID = f.product
	in.FromLocationID = f.shelf
	if e := s.Transfer(context.Background(), in); e != nil {
		t.Fatal("inventory incorrectly requires Finance/POS", e)
	}
	out, e := s.Reconcile(context.Background(), in.OperationID, true)
	if e != nil || !out.Applied {
		t.Fatal(out, e)
	}
	code, _ := request(t, f.app, "GET", "/local/v1/retail/config", "", f.token)
	if code != 200 {
		t.Fatal("owner Core configuration", code)
	}
}
func TestRetailForeignActorFenceCannotBlockOwner(t *testing.T) {
	f, s := storesSetup(t)
	for _, q := range []string{`INSERT INTO identities VALUES('stock-operator','Synthetic','2026-11-03')`, `INSERT INTO memberships VALUES(?,'stock-operator','stock','active','2026-11-03')`, `INSERT INTO membership_stores VALUES(?,'stock-operator',?)`} {
		var e error
		if strings.Contains(q, "INTO identities") {
			_, e = f.db.Exec(q)
		} else if strings.Contains(q, "membership_stores") {
			_, e = f.db.Exec(q, f.owner.TenantID, f.owner.StoreID)
		} else {
			_, e = f.db.Exec(q, f.owner.TenantID)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if e := localauth.SetPassword(context.Background(), f.db, s.Actor, f.device, "stock-operator", "synthetic-stock-password"); e != nil {
		t.Fatal(e)
	}
	session, e := localauth.Login(context.Background(), f.db, f.device, "stock-operator", "synthetic-stock-password")
	if e != nil {
		t.Fatal(e)
	}
	other := retail.Service{DB: f.db, Contracts: s.Contracts, Actor: session.Actor, Device: f.device, Token: session.Token}
	if out, e := other.Reconcile(context.Background(), "shared-test-operation", true); e != nil || !out.Retired {
		t.Fatal(out, e)
	}
	if e := s.CreateStore(context.Background(), "shared-test-operation", "owner-store", "Synthetic"); e != nil {
		t.Fatal("foreign fence blocked owner", e)
	}
	if _, e = other.Reconcile(context.Background(), "shared-test-operation", true); !errors.Is(e, identity.ErrDenied) {
		t.Fatal("foreign applied operation exposed", e)
	}
}
