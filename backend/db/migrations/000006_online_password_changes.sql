-- Additive security migration; never initialize or replace commercial data.
CREATE TABLE online_password_changes (
 id UUID PRIMARY KEY,
 tenant_id UUID NOT NULL,
 user_id UUID NOT NULL,
 actor_session_id UUID NOT NULL,
 revoked_count INTEGER NOT NULL CHECK(revoked_count > 0),
 created_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(actor_session_id, tenant_id, user_id)
  REFERENCES online_sessions(id, tenant_id, user_id)
);
CREATE INDEX online_password_changes_actor ON online_password_changes(tenant_id, user_id, created_at);
