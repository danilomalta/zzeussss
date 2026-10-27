-- B02B1: intenção durável de checkout. Não registra venda nem aprovação bancária.
-- 0028–0044 pertencem à produção ba29cbd e permanecem intactas.
CREATE TABLE checkout_plans (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    checkout_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    cash_session_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    total_cents INTEGER NOT NULL CHECK (total_cents BETWEEN 1 AND 9007199254740991),
    items_json TEXT NOT NULL CHECK (json_valid(items_json)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, checkout_id),
    UNIQUE (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, store_id, cash_session_id) REFERENCES cash_sessions(tenant_id, store_id, id)
) STRICT;

CREATE TABLE checkout_plan_parts (
    tenant_id TEXT NOT NULL,
    checkout_id TEXT NOT NULL,
    part_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 19),
    method TEXT NOT NULL CHECK (method IN ('cash','debit','credit','pix','creditAccount')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents BETWEEN 1 AND 9007199254740991),
    PRIMARY KEY (tenant_id, checkout_id, part_id),
    UNIQUE (tenant_id, checkout_id, position),
    FOREIGN KEY (tenant_id, checkout_id) REFERENCES checkout_plans(tenant_id, checkout_id)
) STRICT;

CREATE TABLE checkout_plan_operations (
    tenant_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    checkout_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('created','cancelled')),
    request_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, device_id, operation_id),
    UNIQUE (tenant_id, checkout_id, action),
    FOREIGN KEY (tenant_id, checkout_id) REFERENCES checkout_plans(tenant_id, checkout_id),
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices(tenant_id, id)
) STRICT;

CREATE TABLE checkout_plan_cancellations (
    tenant_id TEXT NOT NULL,
    checkout_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    reason TEXT NOT NULL CHECK (length(trim(reason)) BETWEEN 1 AND 512),
    cancelled_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, checkout_id),
    FOREIGN KEY (tenant_id, checkout_id) REFERENCES checkout_plans(tenant_id, checkout_id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;
