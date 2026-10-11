-- Declared internal contacts; no remote-company identity, purchase or stock mutation.
CREATE TABLE online_suppliers (
 tenant_id UUID NOT NULL,
 supplier_id UUID NOT NULL,
 version BIGINT NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 active BOOLEAN NOT NULL,
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(tenant_id,supplier_id),
 FOREIGN KEY(tenant_id) REFERENCES tenants(id)
);
CREATE INDEX online_supplier_name ON online_suppliers(tenant_id,(payload->>'name') COLLATE "C",supplier_id);
CREATE TABLE online_supplier_operations (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL, supplier_id UUID NOT NULL, actor_id UUID NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('create','update')),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 receipt JSONB NOT NULL CHECK(jsonb_typeof(receipt)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,supplier_id) REFERENCES online_suppliers(tenant_id,supplier_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE online_supplier_outbox (
 tenant_id UUID NOT NULL, operation_id UUID NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('supplier.create','supplier.update')),
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_supplier_operations(tenant_id,operation_id)
);
