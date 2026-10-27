package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
)

func TestHTTPCustomerReturnMaintenanceCoexistenceAndBackup(t *testing.T) {
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 99, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Accounting}), 200)
	before := caseCommercialState(t, f.httpContractFixture)
	fleetCall(t, f.httpContractFixture, "assets", fleetBody("coexist-oven", 0, "create", fleet.Asset{Reference: "OVEN-TEST", Name: "Forno de teste", Category: "machine", MeterBasis: "none", Location: "Padaria"}), 200)
	fleetCall(t, f.httpContractFixture, "requests", fleetBody("coexist-repair", 0, "create", fleet.OpenRequest{AssetID: "coexist-oven", Issue: "Teste de conferencia", Priority: "emergency", Location: "Padaria", Blocks: true}), 200)
	specialty := maintenanceCall(t, f.httpContractFixture, "specialties", fleetBody("coexist-electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Eletrica de teste"}), 200)
	classification := maintenanceCall(t, f.httpContractFixture, "classifications", fleetBody("coexist-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "coexist-repair", ExpectedRequestRevision: 1, SpecialtyID: "coexist-electrical"}), 200)
	if after := caseCommercialState(t, f.httpContractFixture); !reflect.DeepEqual(before, after) {
		t.Fatal("maintenance changed commercial records", before, after)
	}
	salesRequest(t, f, salesInput(f), 201)
	refund := customerReturnRequest(t, f, customerReturnInput(t, f, "coexist-refund", 1000, 1599, "restock"), 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"coexist-close","declared_cents":2599}`, 200)
	var observed fleet.MaintenanceRecord
	raw := agendaCall(t, f.httpContractFixture, "GET", mp+"/classifications/coexist-repair", nil, 200)
	if e := json.Unmarshal(raw, &observed); e != nil || !reflect.DeepEqual(classification, observed) {
		t.Fatal(observed, e)
	}
	dir := t.TempDir()
	archive, path := filepath.Join(dir, "coexist.tytbak"), filepath.Join(dir, "restored.sqlite")
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	for _, want := range []fleet.MaintenanceRecord{specialty, classification} {
		got, e := fleet.MaintenanceGet(ctx, db, a, f.device, want.Kind, want.ID)
		if e != nil || !reflect.DeepEqual(want, got) {
			t.Fatal(got, e)
		}
	}
	got, e := sale.ReadCustomerReturnOperation(ctx, db, a, f.device, "coexist-refund")
	if e != nil || !reflect.DeepEqual(refund, got) {
		t.Fatal(got, e)
	}
	report, e := register.ReadClosedReport(ctx, db, a, f.device, "turn-one")
	if e != nil || report.Cash.ExpectedCents != 2599 || report.ReturnedSalesCents != 1599 || report.NetSalesCents != 1599 {
		t.Fatal(report, e)
	}
	var versions int
	if e := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&versions); e != nil || versions != 71 {
		t.Fatal(versions, e)
	}
}
