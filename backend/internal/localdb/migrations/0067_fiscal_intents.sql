-- Fiscal preparation is not authorization. No number/key/XML is allocated here.
CREATE TABLE fiscal_profiles (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 profile_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0),
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 request_json TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,profile_id,revision),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
) STRICT;

CREATE TABLE fiscal_intents (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 sale_id TEXT NOT NULL,
 profile_id TEXT NOT NULL,
 profile_revision INTEGER NOT NULL,
 model TEXT NOT NULL CHECK(model IN ('55','65')),
 environment TEXT NOT NULL CHECK(environment IN ('homologation','production')),
 request_json TEXT NOT NULL,
 snapshot_json TEXT NOT NULL,
 snapshot_sha256 TEXT NOT NULL CHECK(length(snapshot_sha256)=64),
 status TEXT NOT NULL CHECK(status='blocked_configuration'),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,sale_id,model,environment),
 FOREIGN KEY(tenant_id,sale_id) REFERENCES sales(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,profile_id,profile_revision) REFERENCES fiscal_profiles(tenant_id,store_id,profile_id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
) STRICT;
CREATE INDEX fiscal_intents_page ON fiscal_intents(tenant_id,store_id,created_at,id);

CREATE TRIGGER fiscal_profiles_immutable_update BEFORE UPDATE ON fiscal_profiles
BEGIN SELECT RAISE(ABORT,'fiscal profile revisions are immutable'); END;
CREATE TRIGGER fiscal_profiles_immutable_delete BEFORE DELETE ON fiscal_profiles
BEGIN SELECT RAISE(ABORT,'fiscal profile revisions are immutable'); END;
CREATE TRIGGER fiscal_intents_immutable_update BEFORE UPDATE ON fiscal_intents
BEGIN SELECT RAISE(ABORT,'fiscal intents require a reviewed lifecycle migration'); END;
CREATE TRIGGER fiscal_intents_immutable_delete BEFORE DELETE ON fiscal_intents
BEGIN SELECT RAISE(ABORT,'fiscal intents preserve uncertain operations'); END;
