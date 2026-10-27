package localapi

import (
	"reflect"
	"testing"
)

func TestHTTPProductionPlanUnitsRejectNewPlanWithoutWrites(t *testing.T) {
	for _, product := range []string{"bread", "flour", "oil"} {
		t.Run(product, func(t *testing.T) {
			f, in := orderFixture(t)
			if _, err := f.db.Exec(`UPDATE products SET unit='meter' WHERE id=?`, product); err != nil {
				t.Fatal(err)
			}
			before := traceReadState(t, f)
			requireMaterials(t, f, "POST", orderPath, in, 409)
			if !reflect.DeepEqual(before, traceReadState(t, f)) {
				t.Fatal("rejected plan writes data or clock")
			}
			orderCounts(t, f, 0, 0)
		})
	}
}
func TestHTTPProductionPlanUnitsPreserveExistingReplayAndRead(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	if _, err := f.db.Exec(`UPDATE products SET unit='meter'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", orderPath, in, 200)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID, nil, 200)
	in.OrderID, in.OperationID = "new-order", "new-op"
	requireMaterials(t, f, "POST", orderPath, in, 409)
	orderCounts(t, f, 1, 1)
}
