-- Departments group people within ONE store. They do not partition sales/stock.
CREATE TABLE access_departments (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 255),
 status TEXT NOT NULL CHECK(status IN ('active','inactive')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TABLE access_department_members (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, identity_id TEXT NOT NULL,
 department_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision > 0),
 PRIMARY KEY(tenant_id,store_id,identity_id),
 FOREIGN KEY(tenant_id,identity_id,store_id) REFERENCES membership_stores(tenant_id,identity_id,store_id),
 FOREIGN KEY(tenant_id,store_id,department_id) REFERENCES access_departments(tenant_id,store_id,id)
) STRICT;
CREATE TABLE access_rules (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL,
 target_kind TEXT NOT NULL CHECK(target_kind IN ('member','department')),
 target_id TEXT NOT NULL, permission TEXT NOT NULL,
 effect TEXT NOT NULL CHECK(effect IN ('allow','deny','inherit')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 PRIMARY KEY(tenant_id,store_id,target_kind,target_id,permission),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TABLE access_delegations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, department_id TEXT NOT NULL,
 identity_id TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN ('active','revoked')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 PRIMARY KEY(tenant_id,store_id,department_id,identity_id),
 FOREIGN KEY(tenant_id,identity_id,store_id) REFERENCES membership_stores(tenant_id,identity_id,store_id),
 FOREIGN KEY(tenant_id,store_id,department_id) REFERENCES access_departments(tenant_id,store_id,id)
) STRICT;
CREATE TABLE access_policy_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('department','member','rule','delegation')),
 department_id TEXT NOT NULL, target_id TEXT NOT NULL,
 permission TEXT NOT NULL, before_value TEXT NOT NULL, after_value TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0), reason TEXT NOT NULL,
 request_hash TEXT NOT NULL, created_unix INTEGER NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX access_policy_events_page ON access_policy_events(tenant_id,store_id,created_unix,operation_id);
CREATE INDEX access_department_members_department ON access_department_members(tenant_id,store_id,department_id,identity_id);
