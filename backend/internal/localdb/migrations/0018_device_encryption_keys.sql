-- Somente chaves PUBLICAS e assinaturas. Privadas ficam em arquivo separado.
CREATE TABLE device_encryption_keys (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision>0),
    public_key BLOB NOT NULL CHECK (length(public_key)=32),
    signing_public_key BLOB NOT NULL CHECK (length(signing_public_key)=32),
    signature BLOB NOT NULL CHECK (length(signature)=64),
    approved_by TEXT NOT NULL,
    PRIMARY KEY (tenant_id,store_id,device_id),
    FOREIGN KEY (tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,approved_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;

CREATE TABLE device_encryption_key_audit (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    public_key_sha256 TEXT NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TEXT NOT NULL,
    FOREIGN KEY (tenant_id,store_id,device_id) REFERENCES device_encryption_keys(tenant_id,store_id,device_id),
    FOREIGN KEY (tenant_id,approved_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
