CREATE TABLE production_material_reservations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 order_id TEXT NOT NULL, location_id TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('active','released','consumed')),
 created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,location_id) REFERENCES stock_locations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE UNIQUE INDEX production_material_active_order ON production_material_reservations(tenant_id,store_id,order_id) WHERE status='active';
CREATE TABLE production_material_items (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, reservation_id TEXT NOT NULL,
 product_id TEXT NOT NULL, unit TEXT NOT NULL,
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(tenant_id,store_id,reservation_id,product_id),
 FOREIGN KEY(tenant_id,store_id,reservation_id) REFERENCES production_material_reservations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
) STRICT;
CREATE TABLE production_material_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, reservation_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('reserve','release','consume')),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,reservation_id) REFERENCES production_material_reservations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_material_movements (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, reservation_id TEXT NOT NULL,
 product_id TEXT NOT NULL, movement_id TEXT NOT NULL UNIQUE,
 PRIMARY KEY(tenant_id,store_id,reservation_id,product_id),
 FOREIGN KEY(tenant_id,store_id,reservation_id,product_id) REFERENCES production_material_items(tenant_id,store_id,reservation_id,product_id),
 FOREIGN KEY(movement_id) REFERENCES stock_movements(id)
) STRICT;
CREATE INDEX production_material_lookup ON production_material_items(tenant_id,store_id,product_id,reservation_id);
