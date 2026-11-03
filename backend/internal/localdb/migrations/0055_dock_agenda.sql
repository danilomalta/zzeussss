-- Local dock agenda. No supplier confirmation, stock movement or payment.
CREATE TABLE dock_agenda_resources (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,id TEXT NOT NULL,
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 120),
 enabled INTEGER NOT NULL CHECK(enabled IN(0,1)),revision INTEGER NOT NULL CHECK(revision>0),
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TABLE dock_agenda_appointments (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,id TEXT NOT NULL,dock_id TEXT NOT NULL,
 order_id TEXT NOT NULL,supplier_id TEXT NOT NULL,
 starts_unix INTEGER NOT NULL,ends_unix INTEGER NOT NULL CHECK(ends_unix>starts_unix),
 status TEXT NOT NULL CHECK(status IN('reserved','arrived','unloading','completed','cancelled','no_show')),
 revision INTEGER NOT NULL CHECK(revision>0),created_by TEXT NOT NULL,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
 arrived_at TEXT,unloading_at TEXT,completed_at TEXT,
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,dock_id) REFERENCES dock_agenda_resources(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX dock_agenda_windows ON dock_agenda_appointments(tenant_id,store_id,dock_id,status,starts_unix,ends_unix);
CREATE UNIQUE INDEX dock_agenda_one_order ON dock_agenda_appointments(tenant_id,store_id,order_id) WHERE status IN('reserved','arrived','unloading');
CREATE UNIQUE INDEX dock_agenda_one_unloading ON dock_agenda_appointments(tenant_id,store_id,dock_id) WHERE status='unloading';
-- Overlap checked inside the authorized write transaction before INSERT/UPDATE.
CREATE TABLE dock_agenda_receipts (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,request_hash TEXT NOT NULL,request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 response_json TEXT NOT NULL CHECK(json_valid(response_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE dock_agenda_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,entity_kind TEXT NOT NULL CHECK(entity_kind IN('dock','appointment')),
 entity_id TEXT NOT NULL,revision INTEGER NOT NULL CHECK(revision>0),action TEXT NOT NULL,
 device_id TEXT NOT NULL,operation_id TEXT NOT NULL,actor_id TEXT NOT NULL,occurred_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,entity_kind,entity_id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES dock_agenda_receipts(tenant_id,store_id,device_id,operation_id)
) STRICT;
