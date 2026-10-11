CREATE UNIQUE INDEX discount_suggestions_tenant_id_id_key ON discount_suggestions(tenant_id,id);
CREATE TABLE online_discount_reviews (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 suggestion_id BIGINT NOT NULL,
 actor_id UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('approve','reject')),
 reason TEXT NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 before_state JSONB NOT NULL,
 after_state JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 UNIQUE(tenant_id,suggestion_id),
 FOREIGN KEY(tenant_id,suggestion_id) REFERENCES discount_suggestions(tenant_id,id)
);
CREATE INDEX online_discount_reviews_actor ON online_discount_reviews(tenant_id,actor_id,created_at,operation_id);
CREATE TABLE online_discount_review_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type='discount.reviewed'),
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_discount_reviews(tenant_id,operation_id)
);
