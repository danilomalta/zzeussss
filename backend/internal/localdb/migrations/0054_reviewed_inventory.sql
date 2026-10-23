-- Pacote de inventario revisado. O aplicador usa a proxima versao da base verificada.
CREATE TABLE reviewed_inventory_sessions (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 location_id TEXT NOT NULL, title TEXT NOT NULL CHECK(length(title) BETWEEN 1 AND 160),
 status TEXT NOT NULL CHECK(status IN ('open','submitted','approved','rejected','cancelled')),
 revision INTEGER NOT NULL CHECK(revision > 0), created_by TEXT NOT NULL,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,location_id) REFERENCES stock_locations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX reviewed_inventory_list ON reviewed_inventory_sessions(tenant_id,store_id,status,created_at,id);
CREATE TABLE reviewed_inventory_lines (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, session_id TEXT NOT NULL,
 product_id TEXT NOT NULL, unit TEXT NOT NULL,
 counted_milli INTEGER CHECK(counted_milli >= 0), previous_milli INTEGER,
 movement_fingerprint TEXT, counted_by TEXT, counted_at TEXT,
 movement_id TEXT,
 PRIMARY KEY(tenant_id,store_id,session_id,product_id),
 FOREIGN KEY(tenant_id,store_id,session_id) REFERENCES reviewed_inventory_sessions(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,counted_by) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(movement_id) REFERENCES stock_movements(id)
) STRICT;
CREATE TABLE reviewed_inventory_receipts (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, session_id TEXT NOT NULL,
 request_hash TEXT NOT NULL, request_json TEXT NOT NULL CHECK(json_valid(request_json)), response_json TEXT NOT NULL CHECK(json_valid(response_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,session_id) REFERENCES reviewed_inventory_sessions(tenant_id,store_id,id)
) STRICT;
CREATE TABLE reviewed_inventory_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, session_id TEXT NOT NULL,
 revision INTEGER NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 device_id TEXT NOT NULL, action TEXT NOT NULL, reason TEXT NOT NULL, occurred_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,session_id,revision),
 FOREIGN KEY(tenant_id,store_id,session_id) REFERENCES reviewed_inventory_sessions(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id)
) STRICT;
