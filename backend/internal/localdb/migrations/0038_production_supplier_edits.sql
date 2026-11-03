-- Creation retry payload remains immutable after supplier maintenance.
CREATE TABLE purchase_supplier_originals (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, created_by TEXT NOT NULL,
 name TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN ('active','inactive')), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
INSERT INTO purchase_supplier_originals SELECT * FROM purchase_suppliers;
CREATE TABLE purchase_supplier_revisions (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 PRIMARY KEY(tenant_id,store_id,supplier_id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id)
) STRICT;
CREATE TABLE purchase_supplier_edits (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 before_name TEXT NOT NULL, before_status TEXT NOT NULL CHECK(before_status IN ('active','inactive')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id), UNIQUE(tenant_id,store_id,supplier_id,revision),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
