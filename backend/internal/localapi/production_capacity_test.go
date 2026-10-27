package localapi

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
)

const capacityPath = "/local/v1/production/capacity"

func capacityFixture(t *testing.T) (*httpContractFixture, production.PublishInput) {
	t.Helper()
	f, in := recipeFixture(t)
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("publish %d %s", status, b)
	}
	for _, id := range []string{"production-room", "other-room"} {
		if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,?,'production',?)`, f.owner.TenantID, f.owner.StoreID, id, id); e != nil {
			t.Fatal(e)
		}
	}
	addCapacityMovement(t, f, "flour-stock", "flour", "production-room", 4000000)
	addCapacityMovement(t, f, "oil-stock", "oil", "production-room", 500005)
	addCapacityMovement(t, f, "flour-other", "flour", "other-room", 9000000)
	addCapacityMovement(t, f, "oil-other", "oil", "other-room", 9000000)
	return f, in
}

func addCapacityMovement(t *testing.T, f *httpContractFixture, id, product, location string, quantity int64) {
	t.Helper()
	if _, e := f.db.Exec(`INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, id, f.owner.TenantID, f.owner.StoreID, f.device.DeviceID, product, location, quantity, "capacity-test", "now"); e != nil {
		t.Fatal(e)
	}
}

func readCapacity(t *testing.T, f *httpContractFixture, version string) production.CapacityResult {
	t.Helper()
	status, body := request(t, f.app, "GET", capacityPath+"?version_id="+version+"&location_id=production-room", "", f.token)
	if status != 200 {
		t.Fatalf("capacity %d %s", status, body)
	}
	var v production.CapacityResult
	if e := json.Unmarshal(body, &v); e != nil {
		t.Fatal(e)
	}
	return v
}

func TestHTTPProductionCapacityExactLocationAndLimitingMaterial(t *testing.T) {
	f, in := capacityFixture(t)
	v := readCapacity(t, f, in.VersionID)
	if len(v.Alternatives) != 1 || v.LocationID != "production-room" || !v.AlternativesIndependent || v.SimultaneousTotalAvailable {
		t.Fatalf("scope %+v", v)
	}
	c := v.Alternatives[0]
	if c.PossibleBatches != 5 || c.PossibleOutputMilli != 50000 || len(c.LimitingProductIDs) != 1 || c.LimitingProductIDs[0] != "oil" || c.Materials[0].StockMilli != 4000000 || c.Materials[1].RequiredMilli != 100001 {
		t.Fatalf("exact capacity %+v", c)
	}
	// Two materials tied at five complete batches must BOTH be identified.
	addCapacityMovement(t, f, "flour-used", "flour", "production-room", -1500000)
	v = readCapacity(t, f, in.VersionID)
	if len(v.Alternatives[0].LimitingProductIDs) != 2 {
		t.Fatalf("tie %+v", v)
	}
	// Any positive fraction smaller than the full recipe requirement is zero
	// batches, without falsely rounding up to one unit of output.
	addCapacityMovement(t, f, "oil-used", "oil", "production-room", -400005)
	v = readCapacity(t, f, in.VersionID)
	if v.Alternatives[0].PossibleBatches != 0 || v.Alternatives[0].PossibleOutputMilli != 0 {
		t.Fatalf("fraction rounded up %+v", v)
	}
}

func TestHTTPProductionCapacityIndependentHistoricalAlternativesAndNoWrites(t *testing.T) {
	f, v1 := capacityFixture(t)
	v2 := v1
	v2.OperationID, v2.VersionID, v2.ExpectedRevision, v2.YieldMilli = "capacity-v2-op", "bread-v2", 1, 20000
	status, body := request(t, f.app, "POST", recipePath, recipeBody(t, v2), f.token)
	if status != 201 {
		t.Fatalf("v2 %d %s", status, body)
	}
	tables := []string{"stock_movements", "production_recipe_audit", "outbox", "stock_operations", "production_recipe_versions"}
	before := map[string]int{}
	for _, table := range tables {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		before[table] = n
	}
	var clock int64
	if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); e != nil {
		t.Fatal(e)
	}
	bodyIn := `{"location_id":"production-room","version_ids":["bread-v1","bread-v2"]}`
	for i := 0; i < 2; i++ {
		status, body = request(t, f.app, "POST", capacityPath+"/alternatives", bodyIn, f.token)
		var v production.CapacityResult
		if e := json.Unmarshal(body, &v); e != nil || status != 200 {
			t.Fatalf("alternatives %d %s %v", status, body, e)
		}
		if len(v.Alternatives) != 2 || v.Alternatives[0].PossibleOutputMilli != 50000 || v.Alternatives[1].PossibleOutputMilli != 100000 || v.SimultaneousTotalAvailable || !v.AlternativesIndependent {
			t.Fatalf("shared alternatives %+v", v)
		}
		if v.Alternatives[0].Materials[0].StockMilli != v.Alternatives[1].Materials[0].StockMilli {
			t.Fatal("alternatives consumed material")
		}
	}
	for _, table := range tables {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != before[table] {
			t.Fatalf("read wrote %s: %d %v", table, n, e)
		}
	}
	var afterClock int64
	if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&afterClock); e != nil || afterClock != clock {
		t.Fatal("read changed license clock", e)
	}
}

func TestHTTPProductionCapacityRejectsAmbiguityAndUnknownRecords(t *testing.T) {
	f, _ := capacityFixture(t)
	for _, in := range []string{`null`, `[]`, `{"location_id":"production-room","version_ids":null}`, `{"location_id":"production-room","version_ids":[]}`, `{"location_id":"production-room","version_ids":["bread-v1","bread-v1"]}`, `{"location_id":"production-room","location_id":"other-room","version_ids":["bread-v1"]}`, `{"location_id":"production-room","version_ids":["bread-v1"],"tenant_id":"foreign"}`, `{"location_id":"production-room","version_ids":[1]}`, `{"location_id":"production-room","version_ids":["bread-v1"]}{}`} {
		status, b := request(t, f.app, "POST", capacityPath+"/alternatives", in, f.token)
		if status != 400 {
			t.Fatalf("invalid %d %s for %s", status, b, in)
		}
	}
	for _, query := range []string{"version_id=bread-v1", "version_id=bread-v1&location_id=production-room&tenant_id=other", "version_id=bread-v1&location_id=production-room&location_id=other-room", "version_id=&location_id=production-room"} {
		status, b := request(t, f.app, "GET", capacityPath+"?"+query, "", f.token)
		if status != 400 {
			t.Fatalf("query %d %s", status, b)
		}
	}
	for _, query := range []string{"version_id=missing&location_id=production-room", "version_id=bread-v1&location_id=missing"} {
		status, _ := request(t, f.app, "GET", capacityPath+"?"+query, "", f.token)
		if status != 404 {
			t.Fatal(query, status)
		}
	}
	in := production.CapacityInput{LocationID: "production-room", VersionIDs: make([]string, 21)}
	b, _ := json.Marshal(in)
	status, _ := request(t, f.app, "POST", capacityPath+"/alternatives", string(b), f.token)
	if status != 400 {
		t.Fatal("unbounded alternatives", status)
	}
}

func TestHTTPProductionCapacityRejectsInvalidBalanceUnitsAndOverflow(t *testing.T) {
	for _, kind := range []string{"negative", "balance-overflow", "output-overflow", "unit-change", "dimension-change", "integer-sum"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := capacityFixture(t)
			want := 409
			switch kind {
			case "negative":
				addCapacityMovement(t, f, "bad", "flour", "production-room", -4000001)
			case "balance-overflow":
				addCapacityMovement(t, f, "bad", "flour", "production-room", production.MaxQuantity)
			case "output-overflow":
				if _, e := f.db.Exec(`UPDATE production_recipe_versions SET request_json=json_set(request_json,'$.yield_milli',?)`, production.MaxQuantity); e != nil {
					t.Fatal(e)
				}
			case "unit-change":
				if _, e := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='flour'`); e != nil {
					t.Fatal(e)
				}
			case "dimension-change":
				if _, e := f.db.Exec(`UPDATE products SET unit='ml' WHERE id='flour'`); e != nil {
					t.Fatal(e)
				}
			case "integer-sum":
				// Intermediate SUM would overflow int64. Exact cancellation
				// produces the original final balance, independent of row order.
				addCapacityMovement(t, f, "huge-plus-1", "flour", "production-room", math.MaxInt64)
				addCapacityMovement(t, f, "huge-plus-2", "flour", "production-room", math.MaxInt64)
				addCapacityMovement(t, f, "huge-minus-1", "flour", "production-room", -math.MaxInt64)
				addCapacityMovement(t, f, "huge-minus-2", "flour", "production-room", -math.MaxInt64)
				want = 200
			}
			status, body := request(t, f.app, "GET", capacityPath+"?version_id=bread-v1&location_id=production-room", "", f.token)
			if status != want {
				t.Fatalf("%d want %d %s", status, want, body)
			}
		})
	}
}

func TestHTTPProductionCapacityAuthorizationAndIsolation(t *testing.T) {
	for _, kind := range []string{"anonymous", "cashier", "revoked", "device", "foreign-location", "foreign-tenant"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := capacityFixture(t)
			token, location, want := f.token, "production-room", 403
			switch kind {
			case "anonymous":
				token, want = "", 401
			case "cashier":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
					t.Fatal(e)
				}
			case "revoked":
				if _, e := f.db.Exec(`UPDATE memberships SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "device":
				if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "foreign-location":
				if _, e := f.db.Exec(`INSERT INTO stores VALUES(?,'other','Other')`, f.owner.TenantID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,'other','foreign','production','Other')`, f.owner.TenantID); e != nil {
					t.Fatal(e)
				}
				location, want = "foreign", 404
			case "foreign-tenant":
				actor := identity.Scope{TenantID: "foreign", StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
				if _, e := production.Capacities(context.Background(), f.db, actor, f.device, production.CapacityInput{LocationID: location, VersionIDs: []string{"bread-v1"}}); e == nil {
					t.Fatal("foreign tenant permitted")
				}
				return
			}
			status, b := request(t, f.app, "GET", capacityPath+"?version_id=bread-v1&location_id="+location, "", token)
			if status != want {
				t.Fatalf("%d want %d %s", status, want, b)
			}
		})
	}
}

func TestHTTPProductionCapacityConcurrentReadsAndReopen(t *testing.T) {
	f, in := capacityFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := readCapacity(t, f, in.VersionID)
			if v.Alternatives[0].PossibleBatches != 5 {
				t.Errorf("concurrent %+v", v)
			}
		}()
	}
	wg.Wait()
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	v, e := production.Capacities(context.Background(), db, actor, f.device, production.CapacityInput{LocationID: "production-room", VersionIDs: []string{in.VersionID}})
	if e != nil || v.Alternatives[0].PossibleBatches != 5 {
		t.Fatalf("reopen %+v %v", v, e)
	}
}
