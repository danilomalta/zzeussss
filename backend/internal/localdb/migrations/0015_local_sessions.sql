CREATE TABLE local_passwords (
    tenant_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    password_hash BLOB NOT NULL,
    changed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, identity_id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE TABLE local_sessions (
    token_sha256 BLOB PRIMARY KEY CHECK (length(token_sha256) = 32),
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    expires_unix INTEGER NOT NULL,
    revoked_unix INTEGER,
    created_unix INTEGER NOT NULL,
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE INDEX idx_local_sessions_expiry ON local_sessions(expires_unix);
