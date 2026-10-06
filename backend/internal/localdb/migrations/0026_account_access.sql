CREATE TABLE owner_recovery_keys (
    tenant_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    token_sha256 BLOB NOT NULL CHECK (length(token_sha256) = 32),
    issued_unix INTEGER NOT NULL,
    expires_unix INTEGER NOT NULL,
    consumed_unix INTEGER,
    PRIMARY KEY (tenant_id, identity_id, device_id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id)
) STRICT;

-- Only non-secret request metadata is recorded. No password, hash of a password,
-- session token or recovery secret is stored in the operation history.
CREATE TABLE account_access_operations (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('password_reset', 'sessions_revoked', 'recovery_issued', 'recovery_revoked', 'owner_recovered')),
    reason TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    affected_sessions INTEGER NOT NULL CHECK (affected_sessions >= 0),
    created_unix INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, device_id, operation_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, target_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE INDEX idx_account_access_target ON account_access_operations(tenant_id,target_id,created_unix);
