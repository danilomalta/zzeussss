-- Typed NF-e 55 origins. A sale row is never reused as inbound or return.
CREATE TABLE fiscal_nfe_origins (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 origin_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('inbound_receipt','customer_return')),
 source_id TEXT NOT NULL,
 referenced_access_key TEXT NOT NULL CHECK(length(referenced_access_key)=44),
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 snapshot_sha256 TEXT NOT NULL CHECK(length(snapshot_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,origin_id),
 UNIQUE(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,kind,source_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
) STRICT;
CREATE TRIGGER fiscal_nfe_origins_no_update BEFORE UPDATE ON fiscal_nfe_origins BEGIN SELECT RAISE(ABORT,'fiscal nfe origin is immutable'); END;
CREATE TRIGGER fiscal_nfe_origins_no_delete BEFORE DELETE ON fiscal_nfe_origins BEGIN SELECT RAISE(ABORT,'fiscal nfe origin is preserved'); END;
