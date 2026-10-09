CREATE TABLE online_catalog_undos (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 source_operation_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 500),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 UNIQUE(tenant_id,source_operation_id),
 CHECK(operation_id<>source_operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_batches(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,source_operation_id) REFERENCES online_catalog_batches(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_catalog_undo_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL,
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_undos(tenant_id,operation_id)
);
