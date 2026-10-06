-- Never store passwords, password hashes or session tokens in audit payloads.
CREATE TABLE account_security_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    identity_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('password_changed', 'sessions_revoked')),
    affected_sessions INTEGER NOT NULL CHECK (affected_sessions >= 0),
    created_unix INTEGER NOT NULL,
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, identity_id) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE INDEX idx_account_security_actor ON account_security_events(tenant_id,identity_id,created_unix);
