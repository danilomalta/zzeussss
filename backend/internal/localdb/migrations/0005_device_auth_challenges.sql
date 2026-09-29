-- Desafios de autenticação de dispositivos pareados.
CREATE TABLE device_auth_challenges (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    nonce BLOB NOT NULL CHECK (length(nonce) = 32),
    issued_unix INTEGER NOT NULL,
    expires_unix INTEGER NOT NULL,
    consumed_unix INTEGER,
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id)
) STRICT;

CREATE INDEX idx_device_challenge_expiry ON device_auth_challenges(expires_unix);
