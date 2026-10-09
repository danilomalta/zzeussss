package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/stock"
)

func receiptVoidBody() map[string]any {
	return map[string]any{"operation_id": "void-receipt", "reason": "corrigir recebimento lancado por engano"}
}
func receiptVoidPath(id string) string { return receivingPath + "/" + id + "/void" }
func TestHTTPPurchaseReceiptVoidRestoresPendingPreservesOriginalAndBackup(t *testing.T) {
	f := receivingFixture(t)
	original := receivedResponse(t, f, receivingBody(), 201)
	path := receiptVoidPath(original.Receipt.ID)
	first := requireMaterials(t, f, "POST", path, receiptVoidBody(), 200)
	repeat := requireMaterials(t, f, "POST", path, receiptVoidBody(), 200)
	var a, b purchases.ReceiptVoidResult
	if json.Unmarshal(first, &a) != nil || json.Unmarshal(repeat, &b) != nil || a.Repeated || !b.Repeated || a.Void.QuantityMilli != 2000 || a.Void.StockOperationID != b.Void.StockOperationID {
		t.Fatal(string(first), string(repeat))
	}
	replay := receivedResponse(t, f, receivingBody(), 200)
	replay.Repeated = false
	if !reflect.DeepEqual(original, replay) {
		t.Fatal("original replay changed", replay)
	}
	trace := receivingTraceResponse(t, f, receivingTracePath)
	if trace.AcceptedMilli != 0 || trace.RemainingMilli != 5000 || trace.VoidedCount != 1 || trace.VoidedAcceptedMilliExact != "2000" || trace.Items[0].Void == nil {
		t.Fatal(trace)
	}
	in := receivingBody()
	in["operation_id"] = "replacement"
	requireMaterials(t, f, "POST", receivingPath, in, 409)
	in["delivery_reference"] = "correction-reference"
	receivedResponse(t, f, in, 201)
	trace = receivingTraceResponse(t, f, receivingTracePath)
	if trace.AcceptedMilli != 2000 || trace.VoidedCount != 1 {
		t.Fatal(trace)
	}
	changed := receiptVoidBody()
	changed["operation_id"] = "void-again"
	requireMaterials(t, f, "POST", path, changed, 409)
	changed = receiptVoidBody()
	changed["reason"] = "changed"
	requireMaterials(t, f, "POST", path, changed, 409)
	db := restoredPurchaseDB(t, f)
	after, err := purchases.ReceivingTrace(context.Background(), db, materialsActor(f), f.device, "order", 0)
	if err != nil || !reflect.DeepEqual(trace, after) {
		t.Fatal(after, err)
	}
	var total int64
	if err = db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements`).Scan(&total); err != nil || total != 2000 {
		t.Fatal(total, err)
	}
	materialCount(t, f, "purchase_receipts", 2)
	materialCount(t, f, "purchase_receipt_voids", 1)
}
func TestHTTPPurchaseReceiptVoidRollbackScopeAndUnits(t *testing.T) {
	for _, table := range []string{"stock_operations", "stock_movements", "stock_operation_movements", "purchase_receipt_voids", "purchase_receiving_exception_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := receivingFixture(t)
			receipt := receivedResponse(t, f, receivingBody(), 201)
			if _, err := f.db.Exec("CREATE TRIGGER ignore_void BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", receiptVoidPath(receipt.Receipt.ID), receiptVoidBody(), 409)
			materialCount(t, f, "purchase_receipt_voids", 0)
			materialCount(t, f, "stock_movements", 1)
		})
	}
	f := receivingFixture(t)
	receipt := receivedResponse(t, f, receivingBody(), 201)
	path := receiptVoidPath(receipt.Receipt.ID)
	in := receiptVoidBody()
	in["quantity_milli"] = 2000
	requireMaterials(t, f, "POST", path, in, 400)
	requireMaterials(t, f, "POST", path+"?tenant_id=other", receiptVoidBody(), 400)
	requireMaterials(t, f, "POST", receiptVoidPath("missing"), receiptVoidBody(), 404)
	if _, err := f.db.Exec(`UPDATE products SET unit='kg'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", path, receiptVoidBody(), 409)
	if _, err := f.db.Exec(`UPDATE products SET unit='unit'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", path, receiptVoidBody(), 403)
}
func TestHTTPPurchaseReceiptVoidConcurrentWithdrawalCannotMakeNegativeStock(t *testing.T) {
	f := receivingFixture(t)
	receipt := receivedResponse(t, f, receivingBody(), 201)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, _ := request(t, f.app, "POST", receiptVoidPath(receipt.Receipt.ID), orderBody(t, receiptVoidBody()), f.token)
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		_, err := stock.Record(context.Background(), f.db, materialsActor(f), f.device, stock.Input{OperationID: "competing-loss", Kind: "loss", ProductID: receipt.Receipt.ProductID, FromLocationID: "receiving-room", QuantityMilli: 1500, Reason: "baixa concorrente"})
		if err == nil {
			statuses <- 200
		} else if err == stock.ErrInsufficientStock {
			statuses <- 409
		} else {
			t.Errorf("%v", err)
			statuses <- 500
		}
	}()
	wg.Wait()
	close(statuses)
	successes, conflicts := 0, 0
	for status := range statuses {
		if status == 200 {
			successes++
		} else if status == 409 {
			conflicts++
		} else {
			t.Fatal(status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal(successes, conflicts)
	}
	var balance int64
	if err := f.db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements`).Scan(&balance); err != nil || balance < 0 {
		t.Fatal(balance, err)
	}
}
