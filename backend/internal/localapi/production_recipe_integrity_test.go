package localapi

import (
	"encoding/json"
	"reflect"
	"testing"

	"titansystem-backend/internal/localdb/production"
)

func TestHTTPProductionRecipeIntegrityRejectsMalformedStoredSnapshot(t *testing.T) {
	for _, kind := range []string{"yield", "revision", "unit", "duplicate", "identity"} {
		t.Run(kind, func(t *testing.T) {
			f, in := recipeFixture(t)
			requireMaterials(t, f, "POST", recipePath, in, 201)
			bad := in
			switch kind {
			case "yield":
				bad.YieldMilli = 0
			case "revision":
				bad.ExpectedRevision = 2
			case "unit":
				bad.OutputUnit = "unknown"
			case "duplicate":
				bad.Ingredients = append(bad.Ingredients, bad.Ingredients[0])
			case "identity":
				bad.VersionID = "other-version"
			}
			b, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.db.Exec(`UPDATE production_recipe_versions SET request_json=?`, string(b)); err != nil {
				t.Fatal(err)
			}
			before := traceReadState(t, f)
			requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 409)
			if kind != "identity" {
				requireMaterials(t, f, "GET", recipePath, nil, 409)
			}
			if !reflect.DeepEqual(before, traceReadState(t, f)) {
				t.Fatal("read writes data or clock")
			}
		})
	}
}
func TestHTTPProductionRecipeIntegrityKeepsHistoricVersions(t *testing.T) {
	f, in := recipeFixture(t)
	requireMaterials(t, f, "POST", recipePath, in, 201)
	v2 := in
	v2.VersionID, v2.OperationID, v2.ExpectedRevision = "next", "next-op", 1
	requireMaterials(t, f, "POST", recipePath, v2, 201)
	if _, err := f.db.Exec(`UPDATE products SET unit='meter',name='Changed'`); err != nil {
		t.Fatal(err)
	}
	b := requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 200)
	var got production.Version
	if err := json.Unmarshal(b, &got); err != nil || got.VersionID != in.VersionID || got.OutputUnit != in.OutputUnit || got.YieldMilli != in.YieldMilli {
		t.Fatal(got, err)
	}
	requireMaterials(t, f, "GET", recipePath, nil, 200)
}
