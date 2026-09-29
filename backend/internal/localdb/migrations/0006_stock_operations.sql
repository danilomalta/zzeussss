-- Histórico das operações. A API de estoque insere movimentos e outbox juntos.
CREATE TABLE stock_operations (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('entry', 'transfer', 'loss')),
    product_id TEXT NOT NULL,
    from_location_id TEXT,
    to_location_id TEXT,
    quantity_milli INTEGER NOT NULL CHECK (quantity_milli > 0),
    reason TEXT NOT NULL CHECK (length(trim(reason)) > 0),
    occurred_at TEXT NOT NULL,
    CHECK ((kind = 'entry' AND from_location_id IS NULL AND to_location_id IS NOT NULL)
        OR (kind = 'loss' AND from_location_id IS NOT NULL AND to_location_id IS NULL)
        OR (kind = 'transfer' AND from_location_id IS NOT NULL AND to_location_id IS NOT NULL
            AND from_location_id <> to_location_id)),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, from_location_id) REFERENCES stock_locations(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, to_location_id) REFERENCES stock_locations(tenant_id, store_id, id)
) STRICT;

CREATE TABLE stock_operation_movements (
    operation_id TEXT NOT NULL,
    movement_id TEXT NOT NULL UNIQUE,
    PRIMARY KEY (operation_id, movement_id),
    FOREIGN KEY (operation_id) REFERENCES stock_operations(id),
    FOREIGN KEY (movement_id) REFERENCES stock_movements(id)
) STRICT;

CREATE INDEX idx_stock_operations_store_product ON stock_operations(tenant_id, store_id, product_id, occurred_at);
