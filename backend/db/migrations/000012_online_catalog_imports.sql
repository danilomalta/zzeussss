CREATE TABLE online_catalog_imports (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 500),
 source_hash TEXT NOT NULL CHECK(length(source_hash)=64),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_batches(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_catalog_import_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL CHECK(jsonb_typeof(event)='object'),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_imports(tenant_id,operation_id)
);
