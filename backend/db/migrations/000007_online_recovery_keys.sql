-- Personal recovery keys. No email address is treated as verified identity.
CREATE TABLE online_recovery_keys (
 tenant_id UUID NOT NULL,
 user_id UUID NOT NULL,
 id UUID NOT NULL UNIQUE,
 digest CHAR(64) NOT NULL,
 credential_digest CHAR(64) NOT NULL,
 role VARCHAR(50) NOT NULL,
 issued_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 PRIMARY KEY(tenant_id, user_id),
 FOREIGN KEY(tenant_id, user_id) REFERENCES users(tenant_id, id),
 CHECK(expires_at > issued_at)
);
CREATE TABLE online_recovery_audit (
 id UUID PRIMARY KEY,
 tenant_id UUID NOT NULL,
 user_id UUID NOT NULL,
 key_id UUID NOT NULL,
 event VARCHAR(20) NOT NULL CHECK(event IN ('issued','recovered')),
 revoked_count INTEGER NOT NULL CHECK(revoked_count >= 0),
 created_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(tenant_id, user_id) REFERENCES users(tenant_id, id)
);
CREATE INDEX online_recovery_audit_actor ON online_recovery_audit(tenant_id, user_id, created_at);
