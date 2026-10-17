-- Preserve original order, item and approval; cancellation is a separate decision.
CREATE TABLE purchase_order_cancellations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
