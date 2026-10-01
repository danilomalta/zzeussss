CREATE UNIQUE INDEX cash_session_adjustment_scope ON cash_sessions(tenant_id,store_id,device_id,id);
CREATE UNIQUE INDEX cash_movement_adjustment_scope ON cash_movements(tenant_id,store_id,cash_session_id,id);

CREATE TABLE cash_adjustments (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL CHECK(length(operation_id) BETWEEN 1 AND 128),
 session_id TEXT NOT NULL,
 actor_identity_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('withdrawal','supply')),
 amount_cents INTEGER NOT NULL CHECK(amount_cents > 0),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 500),
 movement_id TEXT NOT NULL UNIQUE,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_identity_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,session_id,movement_id) REFERENCES cash_movements(tenant_id,store_id,cash_session_id,id)
) STRICT;
