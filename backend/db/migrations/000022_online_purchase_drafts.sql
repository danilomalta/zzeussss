-- Internal purchasing drafts only: no supplier transmission, acceptance or stock movement.
CREATE TABLE online_purchase_drafts (
 tenant_id UUID NOT NULL, purchase_id UUID NOT NULL, proposal_id UUID NOT NULL, offer_id UUID NOT NULL,
 revision BIGINT NOT NULL CHECK(revision BETWEEN 1 AND 9007199254740991),
 state TEXT NOT NULL CHECK(state IN ('draft','approved','cancelled')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(tenant_id,purchase_id),
 UNIQUE(tenant_id,proposal_id),
 FOREIGN KEY(tenant_id,proposal_id) REFERENCES online_replenishment_proposals(tenant_id,proposal_id),
 FOREIGN KEY(tenant_id,offer_id) REFERENCES online_supplier_offers(tenant_id,offer_id)
);
CREATE TABLE online_purchase_operations (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL, purchase_id UUID NOT NULL, actor_id UUID NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('create','approve','cancel')),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64), receipt JSONB NOT NULL CHECK(jsonb_typeof(receipt)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,purchase_id) REFERENCES online_purchase_drafts(tenant_id,purchase_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_purchase_outbox (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('purchase.internal.create','purchase.internal.approve','purchase.internal.cancel')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_purchase_operations(tenant_id,operation_id)
);
