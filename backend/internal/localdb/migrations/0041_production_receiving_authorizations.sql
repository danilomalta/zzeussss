-- Human local decision; not a supplier acknowledgement or transmission receipt.
CREATE TABLE purchase_receiving_authorizations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 reference TEXT NOT NULL CHECK(length(reference) BETWEEN 1 AND 128),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 255),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE purchase_receiving_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('authorized','received')),
 order_id TEXT NOT NULL, actor_id TEXT NOT NULL, request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 created_at TEXT NOT NULL, PRIMARY KEY(tenant_id,device_id,operation_id,kind),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
