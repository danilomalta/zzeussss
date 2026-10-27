-- Durable local carts. No payments or stock reservations are created by saving.
CREATE TABLE pos_carts (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,
 actor_id TEXT NOT NULL,state TEXT NOT NULL CHECK(state IN('suspended','claimed','completed','cancelled')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 4096),sale_id TEXT NOT NULL,payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,id),UNIQUE(tenant_id,sale_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX pos_carts_owner ON pos_carts(tenant_id,store_id,device_id,actor_id,state,updated_at,id);
CREATE TABLE pos_cart_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 cart_id TEXT NOT NULL,actor_id TEXT NOT NULL,revision INTEGER NOT NULL,action TEXT NOT NULL,
 request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),UNIQUE(tenant_id,store_id,device_id,cart_id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id,cart_id) REFERENCES pos_carts(tenant_id,store_id,device_id,id)
) STRICT;
CREATE TABLE pos_cart_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES pos_cart_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER pos_cart_no_delete BEFORE DELETE ON pos_carts BEGIN SELECT RAISE(ABORT,'cart history immutable'); END;
CREATE TRIGGER pos_cart_preserve BEFORE UPDATE ON pos_carts
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id OR NEW.actor_id IS NOT OLD.actor_id OR NEW.sale_id IS NOT OLD.sale_id OR NEW.revision!=OLD.revision+1
 OR NOT ((OLD.state='suspended' AND NEW.state IN('suspended','claimed','cancelled')) OR (OLD.state='claimed' AND NEW.state IN('suspended','completed')))
BEGIN SELECT RAISE(ABORT,'invalid cart mutation'); END;
CREATE TRIGGER pos_cart_ops_no_update BEFORE UPDATE ON pos_cart_operations BEGIN SELECT RAISE(ABORT,'cart operations immutable'); END;
CREATE TRIGGER pos_cart_ops_no_delete BEFORE DELETE ON pos_cart_operations BEGIN SELECT RAISE(ABORT,'cart operations immutable'); END;
CREATE TRIGGER pos_cart_events_no_update BEFORE UPDATE ON pos_cart_events BEGIN SELECT RAISE(ABORT,'cart events immutable'); END;
CREATE TRIGGER pos_cart_events_no_delete BEFORE DELETE ON pos_cart_events BEGIN SELECT RAISE(ABORT,'cart events immutable'); END;
