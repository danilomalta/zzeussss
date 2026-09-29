-- Proposta é local. Nunca equivale a pedido enviado ao fornecedor.
CREATE TABLE restock_suggestions (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    observed_milli INTEGER NOT NULL CHECK (observed_milli >= 0),
    minimum_milli INTEGER NOT NULL CHECK (minimum_milli >= 0),
    target_milli INTEGER NOT NULL CHECK (target_milli > minimum_milli),
    policy_revision INTEGER NOT NULL CHECK (policy_revision > 0),
    recommended_milli INTEGER NOT NULL CHECK (recommended_milli >= 0),
    status TEXT NOT NULL CHECK (status IN ('not_needed','suggested','approved','rejected')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, device_id, operation_id),
    CHECK ((status = 'not_needed' AND recommended_milli = 0) OR
           (status <> 'not_needed' AND recommended_milli > 0)),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, store_id, product_id) REFERENCES restock_policies(tenant_id, store_id, product_id)
) STRICT;

CREATE INDEX idx_restock_suggestions_status ON restock_suggestions(tenant_id,store_id,status,created_at);
