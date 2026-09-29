-- Convite ligado a uma identidade já verificada por mecanismo externo.
-- Apenas o hash do código fica no banco local.
CREATE TABLE membership_invites (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    target_identity_id TEXT NOT NULL,
    issued_by TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('manager', 'cashier', 'stock', 'production', 'employee', 'supplier', 'accountant')),
    token_hash TEXT NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    expires_unix INTEGER NOT NULL,
    consumed_unix INTEGER,
    revoked_unix INTEGER,
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id),
    FOREIGN KEY (target_identity_id) REFERENCES identities(id),
    FOREIGN KEY (tenant_id, issued_by) REFERENCES memberships(tenant_id, identity_id)
) STRICT;

CREATE INDEX idx_invites_target ON membership_invites(tenant_id, target_identity_id, expires_unix);

CREATE TABLE membership_invite_events (
    id TEXT PRIMARY KEY,
    invite_id TEXT NOT NULL,
    actor_identity_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('issued', 'consumed', 'revoked')),
    occurred_unix INTEGER NOT NULL,
    FOREIGN KEY (invite_id) REFERENCES membership_invites(id),
    FOREIGN KEY (actor_identity_id) REFERENCES identities(id)
) STRICT;
