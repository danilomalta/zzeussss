-- Append-only completion extends order states without rebuilding historical tables.
CREATE TABLE production_results (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 order_id TEXT NOT NULL, reservation_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 product_id TEXT NOT NULL, location_id TEXT NOT NULL, unit TEXT NOT NULL,
 planned_milli INTEGER NOT NULL CHECK(planned_milli BETWEEN 1 AND 9007199254740991),
 produced_milli INTEGER NOT NULL CHECK(produced_milli BETWEEN 0 AND planned_milli),
 shortfall_milli INTEGER NOT NULL CHECK(shortfall_milli=planned_milli-produced_milli),
 order_revision INTEGER NOT NULL CHECK(order_revision BETWEEN 2 AND 2147483647),
 status TEXT NOT NULL CHECK(status='completed'),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255),
 movement_id TEXT, request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,order_id), UNIQUE(tenant_id,store_id,reservation_id),
 UNIQUE(tenant_id,device_id,operation_id), UNIQUE(movement_id),
 CHECK((produced_milli=0 AND movement_id IS NULL) OR (produced_milli>0 AND movement_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,reservation_id) REFERENCES production_material_reservations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,location_id) REFERENCES stock_locations(tenant_id,store_id,id),
 FOREIGN KEY(movement_id) REFERENCES stock_movements(id)
) STRICT;
CREATE INDEX production_results_page ON production_results(tenant_id,store_id,created_at,id);
