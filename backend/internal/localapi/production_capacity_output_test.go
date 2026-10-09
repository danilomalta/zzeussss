package localapi

import (
	"reflect"
	"testing"
)

func TestHTTPProductionCapacityOutputUnitRejectsUnsafeInterpretation(t *testing.T) {
	f, in := capacityFixture(t)
	readCapacity(t, f, in.VersionID)
	if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	before := traceReadState(t, f)
	requireMaterials(t, f, "GET", capacityPath+"?version_id="+in.VersionID+"&location_id=production-room", nil, 409)
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("capacity writes data or clock")
	}
	// Historic recipe remains readable; capacity cannot reinterpret current stock.
	requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 200)
	if _, err := f.db.Exec(`UPDATE products SET unit='unit' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	readCapacity(t, f, in.VersionID)
}
