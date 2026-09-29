-- Uma identidade pode pertencer a várias empresas. Não modificar a tabela
-- users de 0001: dados legados permanecem intactos até migração explícita.
CREATE TABLE identities (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
    created_at TEXT NOT NULL
) STRICT;

CREATE TABLE memberships (
    tenant_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'manager', 'cashier', 'stock', 'production', 'employee', 'supplier', 'accountant')),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, identity_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (identity_id) REFERENCES identities(id)
) STRICT;

CREATE TABLE membership_stores (
    tenant_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, identity_id, store_id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id)
) STRICT;

CREATE INDEX idx_membership_stores_store ON membership_stores(tenant_id, store_id);
