-- Política vigente e trilha de alterações por loja e produto.
CREATE TABLE restock_policies (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    minimum_milli INTEGER NOT NULL CHECK (minimum_milli >= 0),
    target_milli INTEGER NOT NULL CHECK (target_milli > minimum_milli),
    revision INTEGER NOT NULL CHECK (revision > 0),
    changed_by TEXT NOT NULL,
    changed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, store_id, product_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id),
    FOREIGN KEY (tenant_id, changed_by) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE TABLE restock_policy_changes (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    minimum_milli INTEGER NOT NULL CHECK (minimum_milli >= 0),
    target_milli INTEGER NOT NULL CHECK (target_milli > minimum_milli),
    revision INTEGER NOT NULL CHECK (revision > 0),
    changed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, store_id, product_id) REFERENCES restock_policies(tenant_id, store_id, product_id)
) STRICT;
