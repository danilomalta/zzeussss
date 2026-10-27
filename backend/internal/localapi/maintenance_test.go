package localapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fleet"
)

const mp = "/local/v1/maintenance"

func maintenanceCall(t *testing.T, f *httpContractFixture, res string, in fleet.Input, want int) fleet.MaintenanceRecord {
	t.Helper()
	p := mp + "/" + res
	if in.ExpectedRevision > 0 {
		p += "/" + in.ID + "/actions"
	}
	b := agendaCall(t, f, "POST", p, in, want)
	var out fleet.MaintenanceRecord
	if want == 200 && json.Unmarshal(b, &out) != nil {
		t.Fatal(string(b))
	}
	return out
}
func maintenanceSetup(t *testing.T) *httpContractFixture {
	t.Helper()
	f := fleetFixture(t)
	fleetCall(t, f, "assets", fleetBody("oven", 0, "create", fleet.Asset{Reference: "OVEN-01", Name: "Forno", Category: "machine", MeterBasis: "none", Location: "Padaria"}), 200)
	fleetCall(t, f, "requests", fleetBody("electric-repair", 0, "create", fleet.OpenRequest{AssetID: "oven", Issue: "Disjuntor desarma", Priority: "emergency", Location: "Padaria", Blocks: true}), 200)
	return f
}
func TestMaintenanceRegistryDirectoryDispatchAndNoCommercialEffects(t *testing.T) {
	f := maintenanceSetup(t)
	before := caseCommercialState(t, f)
	in := fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica", Description: "Instalações e máquinas"})
	r := maintenanceCall(t, f, "specialties", in, 200)
	if r.Kind != "specialty" || r.State != "active" || r.Revision != 1 {
		t.Fatal(r)
	}
	again := maintenanceCall(t, f, "specialties", in, 200)
	if !again.Repeated || again.OperationID != r.OperationID {
		t.Fatal(again)
	}
	maintenanceCall(t, f, "technicians", fleetBody(f.owner.OwnerID, 0, "create", fleet.MaintenanceTechnician{IdentityID: f.owner.OwnerID, SpecialtyIDs: []string{"electrical"}, Contact: "Ramal 20"}), 200)
	maintenanceCall(t, f, "assets", fleetBody("oven", 0, "create", fleet.MaintenanceAsset{AssetID: "oven", AssetType: "installation", Sector: "Padaria", Location: "Quadro 2"}), 200)
	var q fleet.MaintenanceQueue
	b := agendaCall(t, f, "GET", mp+"/queue", nil, 200)
	if json.Unmarshal(b, &q) != nil || q.Total != 1 || q.Emergency != 1 || q.Unassigned != 1 || q.PushDelivered || q.Items[0].Classification != nil {
		t.Fatal(string(b))
	}
	maintenanceCall(t, f, "classifications", fleetBody("electric-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "electric-repair", ExpectedRequestRevision: 1, SpecialtyID: "electrical", Note: "Conferir quadro"}), 200)
	b = agendaCall(t, f, "GET", mp+"/queue?technician_id="+f.owner.OwnerID+"&limit=1", nil, 200)
	if json.Unmarshal(b, &q) != nil || q.Total != 1 || q.Items[0].SpecialtyName != "Elétrica" || q.Items[0].AssetProfile == nil {
		t.Fatal(string(b))
	}
	b = agendaCall(t, f, "GET", mp+"/candidates", nil, 200)
	if !strings.Contains(string(b), f.owner.OwnerID) {
		t.Fatal(string(b))
	}
	b = agendaCall(t, f, "GET", mp+"/specialties/electrical/history", nil, 200)
	var h fleet.MaintenanceHistory
	if json.Unmarshal(b, &h) != nil || h.Total != 1 {
		t.Fatal(string(b))
	}
	agendaCall(t, f, "GET", mp+"/operations/"+in.OperationID, nil, 200)
	if after := caseCommercialState(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal("commercial effects", before, after)
	}
	var assigned string
	f.db.QueryRow(`SELECT COALESCE(assigned_to,'') FROM fleet_records WHERE kind='request' AND id='electric-repair'`).Scan(&assigned)
	if assigned != "" {
		t.Fatal("autoassigned", assigned)
	}
}
func TestMaintenanceRejectsStaleConflictingInvalidAndInactive(t *testing.T) {
	f := maintenanceSetup(t)
	in := fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"})
	maintenanceCall(t, f, "specialties", in, 200)
	changed := in
	changed.Data = json.RawMessage(`{"name":"Outro","description":""}`)
	maintenanceCall(t, f, "specialties", changed, 409)
	maintenanceCall(t, f, "specialties", fleetBody("other", 0, "create", fleet.MaintenanceSpecialty{Name: "elétrica"}), 409)
	maintenanceCall(t, f, "technicians", fleetBody("absent", 0, "create", fleet.MaintenanceTechnician{IdentityID: "absent", SpecialtyIDs: []string{"electrical"}}), 403)
	maintenanceCall(t, f, "technicians", fleetBody(f.owner.OwnerID, 0, "create", fleet.MaintenanceTechnician{IdentityID: f.owner.OwnerID, SpecialtyIDs: []string{"electrical", "electrical"}}), 400)
	maintenanceCall(t, f, "assets", fleetBody("absent", 0, "create", fleet.MaintenanceAsset{AssetID: "absent", AssetType: "machine", Sector: "A", Location: "B"}), 404)
	maintenanceCall(t, f, "classifications", fleetBody("electric-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "electric-repair", ExpectedRequestRevision: 2, SpecialtyID: "electrical"}), 409)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 1, "disable", map[string]any{}), 200)
	maintenanceCall(t, f, "classifications", fleetBody("electric-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "electric-repair", ExpectedRequestRevision: 1, SpecialtyID: "electrical"}), 409)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 1, "update", fleet.MaintenanceSpecialty{Name: "Stale"}), 409)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 2, "enable", map[string]any{}), 200)
	maintenanceCall(t, f, "classifications", fleetBody("electric-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "electric-repair", ExpectedRequestRevision: 1, SpecialtyID: "electrical"}), 200)
}
func TestMaintenanceFiltersAndUnknownBodies(t *testing.T) {
	f := maintenanceSetup(t)
	for _, p := range []string{"/queue?limit=0", "/queue?state=bogus", "/queue?state=", "/queue?limit=1&limit=2", "/queue?x=1", "/specialties?state=", "/specialties?offset=-1", "/candidates?x=1", "/assets/missing?x=1"} {
		t.Run(p, func(t *testing.T) { agendaCall(t, f, "GET", mp+p, nil, 400) })
	}
	agendaCall(t, f, "GET", mp+"/specialties/missing", nil, 404)
	agendaCall(t, f, "POST", mp+"/specialties", map[string]any{"operation_id": "invalid", "id": "x", "expected_revision": 0, "action": "create", "reason": "audit", "data": map[string]string{"name": "A", "description": "", "unknown": "x"}}, 400)
	status, _ := request(t, f.app, "POST", mp+"/specialties", `{"operation_id":"x","id":"x","id":"y","expected_revision":0,"action":"create","reason":"audit","data":{"name":"A","description":""}}`, f.token)
	if status != 400 {
		t.Fatal(status)
	}
}
func TestMaintenanceAtomicRollbackAndTamper(t *testing.T) {
	f := maintenanceSetup(t)
	_, e := f.db.Exec(`CREATE TRIGGER maintenance_fail BEFORE INSERT ON outbox WHEN NEW.event_type='maintenance.registry.changed' BEGIN SELECT RAISE(ABORT,'injected'); END`)
	if e != nil {
		t.Fatal(e)
	}
	in := fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"})
	maintenanceCall(t, f, "specialties", in, 409)
	for _, table := range []string{"maintenance_registry", "maintenance_registry_operations"} {
		var n int
		f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
		if n != 0 {
			t.Fatal(table, n)
		}
	}
	f.db.Exec(`DROP TRIGGER maintenance_fail`)
	maintenanceCall(t, f, "specialties", in, 200)
	_, e = f.db.Exec(`UPDATE maintenance_registry SET body_json=json_set(body_json,'$.data.name','forged') WHERE id='electrical'`)
	if e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", mp+"/specialties/electrical", nil, 409)
}
func TestMaintenanceConcurrentSameRevision(t *testing.T) {
	f := maintenanceSetup(t)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"}), 200)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, name := range []string{"A", "B"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			in := fleetBody("electrical", 1, "update", fleet.MaintenanceSpecialty{Name: name})
			in.OperationID = "update-" + name
			b, _ := json.Marshal(in)
			status, _ := request(t, f.app, "POST", mp+"/specialties/electrical/actions", string(b), f.token)
			codes <- status
		}(name)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestMaintenanceCurrentPermissionsActorReceiptsAndIsolation(t *testing.T) {
	f := maintenanceSetup(t)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"}), 200)
	worker := caseWorker(t, f, "electrician")
	other := caseWorker(t, f, "other-tech")
	manager := f.token
	f.token = worker
	agendaCall(t, f, "GET", mp+"/candidates", nil, 403)
	agendaCall(t, f, "GET", mp+"/operations/electrical-create-0", nil, 404)
	maintenanceCall(t, f, "specialties", fleetBody("forbidden", 0, "create", fleet.MaintenanceSpecialty{Name: "Not allowed"}), 403)
	fleetCall(t, f, "requests", fleetBody("worker-request", 0, "create", fleet.OpenRequest{AssetID: "oven", Issue: "Falha", Priority: "normal", Location: "Loja"}), 200)
	f.token = other
	maintenanceCall(t, f, "classifications", fleetBody("worker-request", 0, "create", fleet.MaintenanceClassification{RequestID: "worker-request", ExpectedRequestRevision: 1, SpecialtyID: "electrical"}), 403)
	f.token = worker
	maintenanceCall(t, f, "classifications", fleetBody("worker-request", 0, "create", fleet.MaintenanceClassification{RequestID: "worker-request", ExpectedRequestRevision: 1, SpecialtyID: "electrical"}), 200)
	f.token = manager
	maintenanceCall(t, f, "technicians", fleetBody("electrician", 0, "create", fleet.MaintenanceTechnician{IdentityID: "electrician", SpecialtyIDs: []string{"electrical"}}), 200)
	f.token = other
	agendaCall(t, f, "GET", mp+"/queue?technician_id=electrician", nil, 403)
	f.token = manager
	if _, e := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='electrician'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", mp+"/queue?technician_id=electrician", nil, 403)
	a := agendaActor(f)
	a.StoreID = "foreign"
	if _, e := fleet.MaintenanceList(t.Context(), f.db, a, f.device, "specialty", "", 50, 0); e == nil {
		t.Fatal("foreign store accepted")
	}
	a = agendaActor(f)
	a.TenantID = "foreign"
	if _, e := fleet.MaintenanceGet(t.Context(), f.db, a, f.device, "specialty", "electrical"); e == nil {
		t.Fatal("foreign tenant accepted")
	}
}
func TestMaintenanceBackupRestoreAndIgnoredWrites(t *testing.T) {
	f := maintenanceSetup(t)
	in := fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"})
	maintenanceCall(t, f, "specialties", in, 200)
	db := restoredPurchaseDB(t, f)
	defer db.Close()
	out, e := fleet.MaintenanceGet(t.Context(), db, agendaActor(f), f.device, "specialty", "electrical")
	if e != nil || out.ID != "electrical" {
		t.Fatal(out, e)
	}
	for _, table := range []string{"maintenance_registry", "maintenance_registry_operations", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := maintenanceSetup(t)
			if _, e := f.db.Exec("CREATE TRIGGER ignore_maintenance BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			maintenanceCall(t, f, "specialties", in, 409)
			for _, name := range []string{"maintenance_registry", "maintenance_registry_operations"} {
				var n int
				f.db.QueryRow("SELECT count(*) FROM " + name).Scan(&n)
				if n != 0 {
					t.Fatal(name, n)
				}
			}
		})
	}
}
func TestMaintenanceAssignedWorkRemainsVisibleWhenSpecialtyInactive(t *testing.T) {
	f := maintenanceSetup(t)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"}), 200)
	maintenanceCall(t, f, "technicians", fleetBody(f.owner.OwnerID, 0, "create", fleet.MaintenanceTechnician{IdentityID: f.owner.OwnerID, SpecialtyIDs: []string{"electrical"}}), 200)
	maintenanceCall(t, f, "classifications", fleetBody("electric-repair", 0, "create", fleet.MaintenanceClassification{RequestID: "electric-repair", ExpectedRequestRevision: 1, SpecialtyID: "electrical"}), 200)
	fleetCall(t, f, "requests", fleetBody("electric-repair", 1, "triage", fleet.RequestAction{}), 200)
	maintenanceCall(t, f, "specialties", fleetBody("electrical", 1, "disable", map[string]any{}), 200)
	var q fleet.MaintenanceQueue
	b := agendaCall(t, f, "GET", mp+"/queue?technician_id="+f.owner.OwnerID+"&limit=1&offset=1", nil, 200)
	if json.Unmarshal(b, &q) != nil || q.Total != 1 || len(q.Items) != 0 || q.Unassigned != 0 || q.Emergency != 1 {
		t.Fatal(string(b))
	}
	b = agendaCall(t, f, "GET", mp+"/queue?technician_id="+f.owner.OwnerID, nil, 200)
	if json.Unmarshal(b, &q) != nil || len(q.Items) != 1 {
		t.Fatal(string(b))
	}
}
func TestMaintenanceDeletedMetadataCannotBeRecreated(t *testing.T) {
	f := maintenanceSetup(t)
	in := fleetBody("electrical", 0, "create", fleet.MaintenanceSpecialty{Name: "Elétrica"})
	maintenanceCall(t, f, "specialties", in, 200)
	if _, e := f.db.Exec(`DELETE FROM maintenance_registry WHERE id='electrical'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", mp+"/specialties/electrical", nil, 409)
	in.OperationID = "new-attempt"
	maintenanceCall(t, f, "specialties", in, 409)
}
