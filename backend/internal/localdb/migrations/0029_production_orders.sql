-- Orders plan work only. No reservation or stock movement is created here.
CREATE TABLE production_orders (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 version_id TEXT NOT NULL, location_id TEXT NOT NULL, responsible_id TEXT NOT NULL,
 planned_batches INTEGER NOT NULL CHECK(planned_batches BETWEEN 1 AND 9007199254740991),
 planned_output_milli INTEGER NOT NULL CHECK(planned_output_milli BETWEEN 1 AND 9007199254740991),
 recipe_json TEXT NOT NULL CHECK(json_valid(recipe_json)),
 status TEXT NOT NULL CHECK(status IN ('planned','approved','cancelled')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,version_id) REFERENCES production_recipe_versions(tenant_id,store_id,version_id),
 FOREIGN KEY(tenant_id,store_id,location_id) REFERENCES stock_locations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,responsible_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_order_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, order_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('created','state')),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 before_status TEXT NOT NULL, after_status TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0), reason TEXT NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX production_orders_page ON production_orders(tenant_id,store_id,status,created_at,id);
CREATE INDEX production_order_events_history ON production_order_events(tenant_id,store_id,order_id,revision);
