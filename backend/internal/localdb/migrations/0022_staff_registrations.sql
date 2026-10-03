-- Audit of local staff creation only; passwords never appear in this record.
CREATE TABLE staff_registrations (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 identity_id TEXT NOT NULL,
 created_by TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY (tenant_id,device_id,operation_id),
 UNIQUE (tenant_id,identity_id),
 FOREIGN KEY (tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,identity_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY (tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
