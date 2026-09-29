-- Pedido de pareamento: a chave privada jamais entra neste banco.
CREATE TABLE device_pairings (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    public_key BLOB NOT NULL CHECK (length(public_key) = 32),
    challenge BLOB NOT NULL CHECK (length(challenge) = 32),
    challenge_expires_unix INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'approved', 'revoked')),
    requested_by TEXT NOT NULL,
    proof_unix INTEGER,
    approved_by TEXT,
    approved_unix INTEGER,
    revoked_by TEXT,
    revoked_unix INTEGER,
    PRIMARY KEY (tenant_id, device_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, requested_by) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES memberships(tenant_id, identity_id),
    FOREIGN KEY (tenant_id, revoked_by) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE TABLE device_pairing_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    actor_ref TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('requested', 'proved', 'approved', 'revoked')),
    occurred_unix INTEGER NOT NULL,
    FOREIGN KEY (tenant_id, device_id) REFERENCES device_pairings(tenant_id, device_id)
) STRICT;
