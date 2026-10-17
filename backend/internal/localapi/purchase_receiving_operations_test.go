package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const receivingOperationsPath = "/local/v1/purchase-receiving/operations/"

func receivingOriginal(t *testing.T, f *httpContractFixture, kind, op string) []byte {
	t.Helper()
	return requireMaterials(t, f, "GET", receivingOperationsPath+kind+"/"+op+"/receipt", nil, 200)
}
func TestHTTPPurchaseReceivingOperationsOriginalResultsRestoreAndNoWrites(t *testing.T) {
	f := receivingFixture(t)
	authorized := receivingOriginal(t, f, "authorize", "receiving-auth")
	original := receivedResponse(t, f, receivingBody(), 201)
	receipt := receivingOriginal(t, f, "receipt", "receipt-op")
	rejected := map[string]any{"operation_id": "reject-original", "delivery_reference": "reject-ref", "unit": "unit", "delivered_milli": 1000, "reason": "Avaria total"}
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders/order/rejections", rejected, 201)
	rejection := receivingOriginal(t, f, "reject", "reject-original")
	requireMaterials(t, f, "POST", receiptVoidPath(original.Receipt.ID), receiptVoidBody(), 200)
	correction := receivingOriginal(t, f, "void", "void-receipt")
	next := receivingBody()
	next["operation_id"] = "later-receipt"
	next["delivery_reference"] = "later-ref"
	receivedResponse(t, f, next, 201)
	checks := []struct {
		kind, op string
		body     []byte
	}{{"authorize", "receiving-auth", authorized}, {"receipt", "receipt-op", receipt}, {"reject", "reject-original", rejection}, {"void", "void-receipt", correction}}
	before := traceReadState(t, f)
	for _, c := range checks {
		b := receivingOriginal(t, f, c.kind, c.op)
		if string(b) != string(c.body) {
			t.Fatal("original changed", c.kind)
		}
		var out purchases.ReceivingOperationReceipt
		if json.Unmarshal(b, &out) != nil || out.Kind != c.kind {
			t.Fatal(string(b))
		}
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("GET wrote data")
	}
	db := restoredPurchaseDB(t, f)
	for _, c := range checks {
		out, e := purchases.ReceivingOperation(context.Background(), db, materialsActor(f), f.device, c.kind, c.op)
		if e != nil {
			t.Fatal(e)
		}
		data, _ := json.Marshal(out)
		if string(data) != string(c.body) {
			t.Fatal("restore changed original", c.kind)
		}
	}
	// Explicit replays keep their original data after later actions, not the live totals.
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	receivedResponse(t, f, receivingBody(), 200)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders/order/rejections", rejected, 200)
	requireMaterials(t, f, "POST", receiptVoidPath(original.Receipt.ID), receiptVoidBody(), 200)
	materialCount(t, f, "purchase_receiving_authorizations", 1)
	materialCount(t, f, "purchase_receipts", 2)
	materialCount(t, f, "purchase_delivery_rejections", 1)
	materialCount(t, f, "purchase_receipt_voids", 1)
}
func TestHTTPPurchaseReceivingOperationsScopeAndMalformedAudits(t *testing.T) {
	for _, kind := range []string{"authorize", "receipt", "reject", "void"} {
		t.Run(kind, func(t *testing.T) {
			f := receivingFixture(t)
			op := "receiving-auth"
			table := "purchase_receiving_authorizations"
			if kind == "receipt" || kind == "void" {
				out := receivedResponse(t, f, receivingBody(), 201)
				op = "receipt-op"
				table = "purchase_receipts"
				if kind == "void" {
					requireMaterials(t, f, "POST", receiptVoidPath(out.Receipt.ID), receiptVoidBody(), 200)
					op = "void-receipt"
					table = "purchase_receipt_voids"
				}
			}
			if kind == "reject" {
				requireMaterials(t, f, "POST", "/local/v1/purchase-orders/order/rejections", map[string]any{"operation_id": "reject-op", "delivery_reference": "reject-ref", "unit": "unit", "delivered_milli": 1000, "reason": "Avaria"}, 201)
				op = "reject-op"
				table = "purchase_delivery_rejections"
			}
			path := receivingOperationsPath + kind + "/" + op + "/receipt"
			receivingOriginal(t, f, kind, op)
			requireMaterials(t, f, "GET", path+"?tenant_id=other", nil, 400)
			requireMaterials(t, f, "GET", receivingOperationsPath+kind+"/missing/receipt", nil, 404)
			if _, e := f.db.Exec(`PRAGMA foreign_keys=OFF`); e != nil {
				t.Fatal(e)
			}
			a := materialsActor(f)
			for _, field := range []string{"actor_id", "device_id", "store_id", "tenant_id"} {
				original := map[string]string{"actor_id": a.IdentityID, "device_id": f.device.DeviceID, "store_id": a.StoreID, "tenant_id": a.TenantID}[field]
				if _, e := f.db.Exec("UPDATE "+table+" SET "+field+"='foreign' WHERE operation_id=?", op); e != nil {
					t.Fatal(e)
				}
				requireMaterials(t, f, "GET", path, nil, 404)
				if _, e := f.db.Exec("UPDATE "+table+" SET "+field+"=? WHERE operation_id=?", original, op); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := f.db.Exec("UPDATE "+table+" SET request_json='{}' WHERE operation_id=?", op); e != nil {
				t.Fatal(e)
			}
			requireMaterials(t, f, "GET", path, nil, 409)
		})
	}
}
func TestHTTPPurchaseReceivingOperationsReadRolesAndKinds(t *testing.T) {
	f := receivingFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	requireMaterials(t, f, "GET", receivingOperationsPath+"future/receiving-auth/receipt", nil, 400)
	a := materialsActor(f)
	if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, a.TenantID, a.IdentityID, a.StoreID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
		t.Fatal(e)
	}
	requireMaterials(t, f, "GET", receivingOperationsPath+"authorize/receiving-auth/receipt", nil, 403)
	requireMaterials(t, f, "GET", receivingOperationsPath+"receipt/receipt-op/receipt", nil, 403)
	for _, field := range []string{"tenant", "store", "actor"} {
		foreign := a
		switch field {
		case "tenant":
			foreign.TenantID = "foreign"
		case "store":
			foreign.StoreID = "foreign"
		case "actor":
			foreign.IdentityID = "foreign"
		}
		if _, e := purchases.ReceivingOperation(context.Background(), f.db, foreign, f.device, "receipt", "receipt-op"); e == nil {
			t.Fatal("foreign read", field)
		}
	}
}
