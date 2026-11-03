-- Finalização em dinheiro do plano B02B1, na linha isolada do backend.
-- Recebido e troco são preservados; no caixa entra somente o total da venda.
CREATE TABLE checkout_plan_sales (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    checkout_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    received_cents INTEGER NOT NULL CHECK (received_cents BETWEEN 1 AND 9007199254740991),
    change_cents INTEGER NOT NULL CHECK (change_cents BETWEEN 0 AND 9007199254740991),
    request_hash TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, checkout_id),
    UNIQUE (tenant_id, device_id, operation_id),
    UNIQUE (tenant_id, sale_id),
    FOREIGN KEY (tenant_id, checkout_id) REFERENCES checkout_plans(tenant_id, checkout_id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales(tenant_id, id),
    FOREIGN KEY (tenant_id, device_id, operation_id) REFERENCES sale_operations(tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id)
) STRICT;
