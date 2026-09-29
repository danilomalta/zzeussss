-- Uma proposta recebe uma única decisão, com a identidade do gerente.
CREATE TABLE restock_reviews (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    suggestion_id TEXT NOT NULL,
    reviewer_identity_id TEXT NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('approved','rejected')),
    reason TEXT NOT NULL CHECK (length(trim(reason)) > 0 AND length(reason) <= 255),
    decided_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, suggestion_id),
    UNIQUE (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, suggestion_id) REFERENCES restock_suggestions(tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, reviewer_identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;
