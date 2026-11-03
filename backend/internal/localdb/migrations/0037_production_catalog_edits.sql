CREATE TABLE catalog_edit_revisions (
 tenant_id TEXT NOT NULL, product_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 PRIMARY KEY(tenant_id,product_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
) STRICT;
CREATE TABLE catalog_edit_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, product_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 before_json TEXT NOT NULL CHECK(json_valid(before_json)),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
