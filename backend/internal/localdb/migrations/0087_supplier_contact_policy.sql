-- Advisory only; no order, contact, payment or stock effect.
CREATE TABLE supplier_contact_policies (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0), payload_json TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,supplier_id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id)
) STRICT;
CREATE TABLE supplier_contact_policy_operations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0), request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,supplier_id,revision),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES supplier_contact_policies(tenant_id,store_id,supplier_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TRIGGER supplier_contact_operations_no_update BEFORE UPDATE ON supplier_contact_policy_operations BEGIN SELECT RAISE(ABORT,'immutable contact policy operation'); END;
CREATE TRIGGER supplier_contact_operations_no_delete BEFORE DELETE ON supplier_contact_policy_operations BEGIN SELECT RAISE(ABORT,'immutable contact policy operation'); END;
