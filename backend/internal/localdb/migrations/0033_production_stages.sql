-- Optional, immutable stage definitions attached to a frozen production order.
CREATE TABLE production_stage_plans (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 stage_count INTEGER NOT NULL CHECK(stage_count BETWEEN 1 AND 20),
 created_by TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_stages (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL, id TEXT NOT NULL,
 position INTEGER NOT NULL CHECK(position BETWEEN 1 AND 20),
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 120), responsible_id TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','running','completed')),
 revision INTEGER NOT NULL CHECK((status='pending' AND revision=1) OR (status='running' AND revision=2) OR (status='completed' AND revision=3)),
 updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id,id),
 UNIQUE(tenant_id,store_id,order_id,position),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_stage_plans(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,responsible_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_stage_events (
 sequence INTEGER PRIMARY KEY,
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('configured','running','completed')),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES production_stage_plans(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX production_stage_events_order ON production_stage_events(tenant_id,store_id,order_id,sequence);
