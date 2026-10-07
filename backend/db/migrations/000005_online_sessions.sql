-- Incremental only. Never run 000001_init.sql on an existing installation.
CREATE UNIQUE INDEX online_users_tenant_identity ON users(tenant_id, id);
CREATE TABLE online_sessions (
 id UUID PRIMARY KEY,
 tenant_id UUID NOT NULL,
 user_id UUID NOT NULL,
 role VARCHAR(50) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 UNIQUE(id, tenant_id, user_id),
 FOREIGN KEY(tenant_id, user_id) REFERENCES users(tenant_id, id),
 CHECK(expires_at > created_at)
);
CREATE INDEX online_sessions_actor ON online_sessions(tenant_id, user_id, created_at);
CREATE TABLE online_refresh_tokens (
 digest CHAR(64) PRIMARY KEY,
 session_id UUID NOT NULL REFERENCES online_sessions(id),
 created_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 CHECK(length(digest) = 64)
);
CREATE INDEX online_refresh_session ON online_refresh_tokens(session_id);
CREATE TABLE online_session_audit (
 id UUID PRIMARY KEY,
 tenant_id UUID NOT NULL,
 user_id UUID NOT NULL,
 session_id UUID NOT NULL,
 event VARCHAR(30) NOT NULL CHECK(event IN ('created','rotated','replay_revoked','logout','revoked','others_revoked')),
 created_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(session_id, tenant_id, user_id) REFERENCES online_sessions(id, tenant_id, user_id)
);
CREATE INDEX online_session_audit_actor ON online_session_audit(tenant_id, user_id, created_at);
