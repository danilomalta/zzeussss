-- Local custody only. A bag references an existing audited withdrawal;
-- bag transitions NEVER insert another cash/stock/journal movement.
CREATE UNIQUE INDEX cash_adjustment_bag_scope ON cash_adjustments(tenant_id,store_id,device_id,operation_id);
CREATE TABLE cash_bags (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 id TEXT NOT NULL,
 withdrawal_operation_id TEXT NOT NULL,
 movement_id TEXT NOT NULL,
 session_id TEXT NOT NULL,
 operator_id TEXT NOT NULL,
 sender_id TEXT NOT NULL,
 seal_id TEXT NOT NULL,
 declared_cents INTEGER NOT NULL CHECK(declared_cents BETWEEN 1 AND 9007199254740991),
 state TEXT NOT NULL CHECK(state IN ('prepared','in_transit','received','rejected','cancelled')),
 counted_cents INTEGER,
 difference_cents INTEGER,
 receiver_id TEXT,
 received_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,store_id,device_id,id),
 UNIQUE(tenant_id,store_id,device_id,seal_id),
 CHECK((state='received' AND counted_cents IS NOT NULL AND difference_cents IS NOT NULL AND counted_cents BETWEEN 0 AND 9007199254740991 AND difference_cents=counted_cents-declared_cents AND receiver_id IS NOT NULL AND received_at IS NOT NULL AND receiver_id<>sender_id AND receiver_id<>operator_id)
    OR (state<>'received' AND counted_cents IS NULL AND difference_cents IS NULL AND receiver_id IS NULL AND received_at IS NULL)),
 FOREIGN KEY(tenant_id,store_id,device_id,withdrawal_operation_id) REFERENCES cash_adjustments(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,session_id,movement_id) REFERENCES cash_movements(tenant_id,store_id,cash_session_id,id),
 FOREIGN KEY(tenant_id,sender_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,operator_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,receiver_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE UNIQUE INDEX cash_bags_active_withdrawal ON cash_bags(tenant_id,device_id,withdrawal_operation_id) WHERE state NOT IN ('cancelled','rejected');
CREATE INDEX cash_bags_session ON cash_bags(tenant_id,store_id,device_id,session_id,state,id);
CREATE INDEX cash_bags_list ON cash_bags(tenant_id,store_id,device_id,state,created_at,id);

CREATE TABLE cash_bag_operations (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 bag_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('create','dispatch','receive','reject','cancel')),
 from_state TEXT NOT NULL,
 to_state TEXT NOT NULL,
 reason TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)),
 created_at TEXT NOT NULL,
 UNIQUE(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,device_id,operation_id,bag_id),
 FOREIGN KEY(tenant_id,store_id,device_id,bag_id) REFERENCES cash_bags(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX cash_bag_history ON cash_bag_operations(tenant_id,store_id,device_id,bag_id,sequence);
-- The legacy wire protocol does not support cash.bag.*. Keep custody events
-- durable without feeding unsupported types into its shared pending queue.
CREATE TABLE cash_bag_outbox (
 event_id TEXT PRIMARY KEY,
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 aggregate_id TEXT NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('cash.bag.create','cash.bag.dispatch','cash.bag.receive','cash.bag.reject','cash.bag.cancel')),
 schema_version INTEGER NOT NULL CHECK(schema_version=1),
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 created_at TEXT NOT NULL,
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,aggregate_id) REFERENCES cash_bags(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id,aggregate_id) REFERENCES cash_bag_operations(tenant_id,store_id,device_id,operation_id,bag_id)
) STRICT;
CREATE TRIGGER cash_bag_outbox_no_update BEFORE UPDATE ON cash_bag_outbox BEGIN SELECT RAISE(ABORT,'custody events are immutable'); END;
CREATE TRIGGER cash_bag_outbox_no_delete BEFORE DELETE ON cash_bag_outbox BEGIN SELECT RAISE(ABORT,'custody events cannot be erased'); END;
CREATE TRIGGER cash_bag_ops_no_update BEFORE UPDATE ON cash_bag_operations BEGIN SELECT RAISE(ABORT,'bag history is append only'); END;
CREATE TRIGGER cash_bag_ops_no_delete BEFORE DELETE ON cash_bag_operations BEGIN SELECT RAISE(ABORT,'bag history is append only'); END;
CREATE TRIGGER cash_bags_no_delete BEFORE DELETE ON cash_bags BEGIN SELECT RAISE(ABORT,'bag history cannot be erased'); END;
CREATE TRIGGER cash_bags_preserve_source BEFORE UPDATE ON cash_bags
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id
 OR NEW.withdrawal_operation_id IS NOT OLD.withdrawal_operation_id OR NEW.movement_id IS NOT OLD.movement_id OR NEW.session_id IS NOT OLD.session_id
 OR NEW.operator_id IS NOT OLD.operator_id OR NEW.sender_id IS NOT OLD.sender_id OR NEW.seal_id IS NOT OLD.seal_id OR NEW.declared_cents IS NOT OLD.declared_cents OR NEW.created_at IS NOT OLD.created_at
 OR NOT ((OLD.state='prepared' AND NEW.state IN ('in_transit','cancelled')) OR (OLD.state='in_transit' AND NEW.state IN ('received','rejected')))
BEGIN SELECT RAISE(ABORT,'invalid bag source or transition'); END;
