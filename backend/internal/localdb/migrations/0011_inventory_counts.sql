-- Contagem física e ajuste guardados como um único fato auditável.
CREATE TABLE inventory_counts (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    location_id TEXT NOT NULL,
    counted_milli INTEGER NOT NULL CHECK (counted_milli >= 0),
    previous_milli INTEGER NOT NULL CHECK (previous_milli >= 0),
    difference_milli INTEGER NOT NULL,
    movement_id TEXT UNIQUE,
    counted_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, location_id) REFERENCES stock_locations(tenant_id, store_id, id),
    FOREIGN KEY (movement_id) REFERENCES stock_movements(id)
) STRICT;
