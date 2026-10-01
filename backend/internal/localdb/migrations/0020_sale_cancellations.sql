-- Append-only audit record and explicit links to compensating movements.
-- Original sale amounts, items and operation remain available.
CREATE UNIQUE INDEX sales_cancel_scope_key ON sales(tenant_id,store_id,device_id,id);

CREATE TABLE sale_cancellations (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL CHECK (length(operation_id) BETWEEN 1 AND 128),
    sale_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    reason TEXT NOT NULL CHECK (length(trim(reason)) BETWEEN 1 AND 500),
    request_hash TEXT NOT NULL,
    refunded_cents INTEGER NOT NULL CHECK (refunded_cents > 0),
    cash_movement_id TEXT NOT NULL UNIQUE,
    cancelled_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, sale_id),
    UNIQUE (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id, sale_id) REFERENCES sales(tenant_id, store_id, device_id, id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (cash_movement_id) REFERENCES cash_movements(id)
) STRICT;

CREATE TABLE sale_cancellation_stock (
    tenant_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    original_movement_id TEXT NOT NULL UNIQUE,
    return_movement_id TEXT NOT NULL UNIQUE,
    PRIMARY KEY (tenant_id, sale_id, item_id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sale_cancellations(tenant_id, sale_id),
    FOREIGN KEY (tenant_id, sale_id, item_id) REFERENCES sale_items(tenant_id, sale_id, id),
    FOREIGN KEY (original_movement_id) REFERENCES stock_movements(id),
    FOREIGN KEY (return_movement_id) REFERENCES stock_movements(id)
) STRICT;
