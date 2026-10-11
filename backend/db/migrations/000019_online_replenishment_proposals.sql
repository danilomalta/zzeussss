-- Durable human approval only. No stock movement, supplier order or SQLite change.
CREATE TABLE online_replenishment_proposals (
 tenant_id UUID NOT NULL,
 proposal_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 revision BIGINT NOT NULL CHECK(revision BETWEEN 1 AND 9007199254740991),
 state TEXT NOT NULL CHECK(state IN ('planned','approved','cancelled')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(tenant_id,proposal_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
);
CREATE TABLE online_replenishment_operations (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 proposal_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('create','approve','cancel')),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 receipt JSONB NOT NULL CHECK(jsonb_typeof(receipt)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,proposal_id) REFERENCES online_replenishment_proposals(tenant_id,proposal_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_replenishment_proposal_audit ON online_replenishment_operations(tenant_id,proposal_id,created_at,operation_id);
CREATE TABLE online_replenishment_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('replenishment.proposal.create','replenishment.proposal.approve','replenishment.proposal.cancel')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_replenishment_operations(tenant_id,operation_id)
);
