CREATE TABLE online_catalog_batches (
 tenant_id UUID NOT NULL REFERENCES tenants(id),
 operation_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_catalog_batches_history ON online_catalog_batches(tenant_id,created_at DESC,operation_id DESC);
