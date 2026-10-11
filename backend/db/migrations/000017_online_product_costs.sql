-- Declared unit cost; does not rewrite price, inventory or accounting entries.
CREATE TABLE online_product_costs (
 tenant_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 cost_cents BIGINT NOT NULL CHECK(cost_cents BETWEEN 0 AND 999999999999),
 revision BIGINT NOT NULL CHECK(revision BETWEEN 1 AND 9007199254740991),
 actor_id UUID NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,product_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_cost_operations (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 actor_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 before_state JSONB NOT NULL,
 after_state JSONB NOT NULL,
 product_snapshot JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_cost_history ON online_cost_operations(tenant_id,product_id,created_at DESC,operation_id DESC);
CREATE TABLE online_cost_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type='product.cost.changed'),
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_cost_operations(tenant_id,operation_id)
);
