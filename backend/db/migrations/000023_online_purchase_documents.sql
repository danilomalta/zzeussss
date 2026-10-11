-- Stable internal documents. No transport or supplier acceptance is created.
CREATE TABLE online_purchase_documents (
 tenant_id UUID NOT NULL, document_id UUID NOT NULL, purchase_id UUID NOT NULL,
 revision BIGINT NOT NULL CHECK(revision BETWEEN 1 AND 2),
 state TEXT NOT NULL CHECK(state IN ('prepared','cancelled')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(tenant_id,document_id), UNIQUE(tenant_id,purchase_id),
 FOREIGN KEY(tenant_id,purchase_id) REFERENCES online_purchase_drafts(tenant_id,purchase_id)
);
CREATE TABLE online_purchase_document_operations (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL, document_id UUID NOT NULL, actor_id UUID NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('prepare','cancel')),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),receipt JSONB NOT NULL CHECK(jsonb_typeof(receipt)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,document_id) REFERENCES online_purchase_documents(tenant_id,document_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_purchase_document_outbox (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('purchase.document.prepare','purchase.document.cancel')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_purchase_document_operations(tenant_id,operation_id)
);
