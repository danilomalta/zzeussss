-- Planning-only audit and transactional outbox. No stock movement or SQLite DDL.
CREATE TABLE online_stock_setting_operations (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 actor_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 before_state JSONB NOT NULL,
 after_state JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_stock_setting_history ON online_stock_setting_operations(tenant_id,product_id,created_at DESC,operation_id DESC);
CREATE TABLE online_stock_setting_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type='product.stock-settings.changed'),
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_stock_setting_operations(tenant_id,operation_id)
);
