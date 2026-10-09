ALTER TABLE products ADD COLUMN catalog_version BIGINT NOT NULL DEFAULT 1 CHECK (catalog_version BETWEEN 1 AND 9007199254740991);
CREATE TABLE online_catalog_operations (
 tenant_id UUID NOT NULL REFERENCES tenants(id),
 operation_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 actor_id UUID NOT NULL,
 action TEXT NOT NULL CHECK (action IN ('details','price','active')),
 payload_hash TEXT NOT NULL CHECK (length(payload_hash)=64),
 before_state JSONB NOT NULL,
 after_state JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,operation_id),
 FOREIGN KEY (tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY (tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_catalog_history ON online_catalog_operations(tenant_id,product_id,created_at DESC,operation_id DESC);
-- Durable publication intent only. No transport or acknowledgement is claimed.
CREATE TABLE online_catalog_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_operations(tenant_id,operation_id)
);
