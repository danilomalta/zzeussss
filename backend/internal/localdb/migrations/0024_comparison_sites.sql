-- Configurações locais do comparador; não concedem acesso a outra empresa.
CREATE TABLE comparison_sites (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 120),
    origin TEXT NOT NULL,
    search_template TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('active','inactive')),
    revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 9007199254740991),
    changed_by TEXT NOT NULL,
    changed_at TEXT NOT NULL,
    PRIMARY KEY(tenant_id,id),
    UNIQUE(tenant_id,origin),
    FOREIGN KEY(tenant_id) REFERENCES tenants(id),
    FOREIGN KEY(tenant_id,changed_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;

-- Snapshot estável para retries; também constitui a trilha de alterações.
CREATE TABLE comparison_site_operations (
    tenant_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    input_json TEXT NOT NULL CHECK(json_valid(input_json)),
    result_json TEXT NOT NULL CHECK(json_valid(result_json)),
    recorded_at TEXT NOT NULL,
    PRIMARY KEY(tenant_id,device_id,operation_id),
    FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
    FOREIGN KEY(tenant_id,actor_identity_id) REFERENCES memberships(tenant_id,identity_id),
    FOREIGN KEY(tenant_id,site_id) REFERENCES comparison_sites(tenant_id,id)
) STRICT;
