-- Compensating movement; neither the receipt nor its original entry is deleted.
CREATE TABLE purchase_receipt_voids (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, receipt_id TEXT NOT NULL, order_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 255),
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli BETWEEN 1 AND 9007199254740991),
 unit TEXT NOT NULL CHECK(unit IN ('unit','kg','g','liter','ml','meter')),
 product_id TEXT NOT NULL, location_id TEXT NOT NULL,
 stock_operation_id TEXT NOT NULL UNIQUE, movement_id TEXT NOT NULL UNIQUE,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,receipt_id), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,receipt_id) REFERENCES purchase_receipts(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_receiving_authorizations(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(stock_operation_id) REFERENCES stock_operations(id),
 FOREIGN KEY(movement_id) REFERENCES stock_movements(id)
) STRICT;
