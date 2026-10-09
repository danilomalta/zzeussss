CREATE TABLE online_catalog_adjustments (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 500),
 rate_basis_points INTEGER NOT NULL CHECK(rate_basis_points BETWEEN -10000 AND 10000 AND rate_basis_points<>0),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_batches(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_catalog_adjustments_history ON online_catalog_adjustments(tenant_id,created_at DESC,operation_id DESC);
CREATE TABLE online_catalog_adjustment_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL CHECK(jsonb_typeof(event)='object'),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_adjustments(tenant_id,operation_id)
);
