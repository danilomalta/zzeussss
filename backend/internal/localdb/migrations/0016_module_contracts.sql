-- Declaracoes assinadas: conteudo historico separado do ponteiro vigente.
-- Nenhum modulo e concedido automaticamente a empresas existentes.
CREATE TABLE module_contract_history (
    tenant_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision > 0),
    key_id TEXT NOT NULL,
    payload BLOB NOT NULL,
    signature BLOB NOT NULL CHECK (length(signature) = 64),
    installed_by TEXT NOT NULL,
    installed_at_unix INTEGER NOT NULL CHECK (installed_at_unix > 0),
    PRIMARY KEY (tenant_id, revision),
    FOREIGN KEY (tenant_id, installed_by) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE TABLE module_contract_state (
    tenant_id TEXT PRIMARY KEY,
    revision INTEGER NOT NULL CHECK (revision > 0),
    last_observed_unix INTEGER NOT NULL CHECK (last_observed_unix > 0),
    FOREIGN KEY (tenant_id, revision) REFERENCES module_contract_history(tenant_id, revision)
) STRICT;
