-- Liga o caixa legado a identidades locais sem recriar tabelas existentes.
CREATE TABLE cash_session_operators (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, session_id),
    UNIQUE (tenant_id, store_id, session_id),
    FOREIGN KEY (tenant_id, store_id, session_id) REFERENCES cash_sessions(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

-- Um aparelho só pode manter um turno aberto em cada loja.
CREATE UNIQUE INDEX idx_one_open_cash_session
    ON cash_sessions(tenant_id, store_id, device_id) WHERE closed_at IS NULL;

CREATE TABLE cash_closures (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    declared_cents INTEGER NOT NULL CHECK (declared_cents >= 0),
    expected_cents INTEGER NOT NULL CHECK (expected_cents >= 0),
    closed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, session_id),
    UNIQUE (tenant_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, session_id) REFERENCES cash_session_operators(tenant_id, store_id, session_id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;
