CREATE TABLE company_registrations (
 tenant_id TEXT NOT NULL REFERENCES tenants(id), revision INTEGER NOT NULL CHECK(revision>0),
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,revision), UNIQUE(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id)
) STRICT;
CREATE TRIGGER company_registrations_no_update BEFORE UPDATE ON company_registrations BEGIN SELECT RAISE(ABORT,'registration history immutable'); END;
CREATE TRIGGER company_registrations_no_delete BEFORE DELETE ON company_registrations BEGIN SELECT RAISE(ABORT,'registration history preserved'); END;
