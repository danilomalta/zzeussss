-- Immutable scoped policies and calculation evidence. No preloaded legal rates.
CREATE TABLE fiscal_tax_policies (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, policy_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision>0),
 device_id TEXT NOT NULL, actor_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 profile_id TEXT NOT NULL, profile_revision INTEGER NOT NULL,
 active INTEGER NOT NULL CHECK(active IN(0,1)),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,policy_id,revision),UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,profile_id,profile_revision) REFERENCES fiscal_profiles(tenant_id,store_id,profile_id,revision)
) STRICT;
CREATE TABLE fiscal_tax_quotes (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 device_id TEXT NOT NULL,actor_id TEXT NOT NULL,policy_id TEXT NOT NULL,policy_revision INTEGER NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 policy_sha256 TEXT NOT NULL CHECK(length(policy_sha256)=64),result_json TEXT NOT NULL CHECK(json_valid(result_json)),result_sha256 TEXT NOT NULL CHECK(length(result_sha256)=64),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,policy_id,policy_revision) REFERENCES fiscal_tax_policies(tenant_id,store_id,policy_id,revision)
) STRICT;
CREATE TRIGGER fiscal_tax_policies_no_update BEFORE UPDATE ON fiscal_tax_policies BEGIN SELECT RAISE(ABORT,'fiscal tax policy immutable'); END;
CREATE TRIGGER fiscal_tax_policies_no_delete BEFORE DELETE ON fiscal_tax_policies BEGIN SELECT RAISE(ABORT,'fiscal tax policy history preserved'); END;
CREATE TRIGGER fiscal_tax_quotes_no_update BEFORE UPDATE ON fiscal_tax_quotes BEGIN SELECT RAISE(ABORT,'fiscal tax quote immutable'); END;
CREATE TRIGGER fiscal_tax_quotes_no_delete BEFORE DELETE ON fiscal_tax_quotes BEGIN SELECT RAISE(ABORT,'fiscal tax quote evidence preserved'); END;
