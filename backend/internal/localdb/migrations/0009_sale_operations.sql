-- A operação é imutável e identifica uma venda local concluída.
CREATE TABLE sale_operations (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    PRIMARY KEY (tenant_id, device_id, operation_id),
    UNIQUE (tenant_id, sale_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales(tenant_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE TABLE sale_item_stock (
    tenant_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    movement_id TEXT NOT NULL UNIQUE,
    PRIMARY KEY (tenant_id, sale_id, item_id),
    FOREIGN KEY (tenant_id, sale_id, item_id) REFERENCES sale_items(tenant_id, sale_id, id),
    FOREIGN KEY (movement_id) REFERENCES stock_movements(id)
) STRICT;
