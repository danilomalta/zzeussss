-- Accepted quantities alone enter physical stock. Rejected quantities are evidence.
CREATE TABLE purchase_receipts (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL, order_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 delivery_reference TEXT NOT NULL CHECK(length(delivery_reference) BETWEEN 1 AND 128),
 product_id TEXT NOT NULL, unit TEXT NOT NULL CHECK(unit IN ('unit','kg','g','liter','ml','meter')),
 location_id TEXT NOT NULL,
 delivered_milli INTEGER NOT NULL CHECK(delivered_milli BETWEEN 1 AND 9007199254740991),
 accepted_milli INTEGER NOT NULL CHECK(accepted_milli BETWEEN 1 AND delivered_milli),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 255),
 stock_operation_id TEXT NOT NULL UNIQUE, movement_id TEXT NOT NULL UNIQUE,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,supplier_id,delivery_reference),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_receiving_authorizations(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,location_id) REFERENCES stock_locations(tenant_id,store_id,id),
 FOREIGN KEY(stock_operation_id) REFERENCES stock_operations(id),
 FOREIGN KEY(movement_id) REFERENCES stock_movements(id)
) STRICT;
CREATE INDEX idx_purchase_receipts_order ON purchase_receipts(tenant_id,store_id,order_id,created_at,id);
