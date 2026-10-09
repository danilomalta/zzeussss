CREATE TABLE online_catalog_barcode_batches (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_catalog_barcode_batches_actor ON online_catalog_barcode_batches(tenant_id,actor_id,created_at DESC,operation_id DESC);
CREATE TABLE online_catalog_barcode_batch_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL CHECK(jsonb_typeof(event)='object'),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_barcode_batches(tenant_id,operation_id)
);
