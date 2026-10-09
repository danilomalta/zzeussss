CREATE TABLE online_catalog_barcodes (
 tenant_id UUID NOT NULL,
 id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 code TEXT NOT NULL CHECK(code ~ '^([0-9]{8}|[0-9]{12}|[0-9]{13}|[0-9]{14})$'),
 canonical_code TEXT NOT NULL CHECK(canonical_code ~ '^[0-9]{14}$'),
 ativo BOOLEAN NOT NULL DEFAULT TRUE,
 code_version BIGINT NOT NULL DEFAULT 1 CHECK(code_version BETWEEN 1 AND 9007199254740991),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,canonical_code),
 UNIQUE(tenant_id,product_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
);
CREATE INDEX online_catalog_barcodes_product ON online_catalog_barcodes(tenant_id,product_id,created_at,id);
CREATE TABLE online_catalog_barcode_operations (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 product_id BIGINT NOT NULL,
 barcode_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('add','active')),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 500),
 payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,product_id,barcode_id) REFERENCES online_catalog_barcodes(tenant_id,product_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE INDEX online_catalog_barcode_history ON online_catalog_barcode_operations(tenant_id,product_id,created_at DESC,operation_id DESC);
CREATE TABLE online_catalog_barcode_outbox (
 tenant_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 event JSONB NOT NULL CHECK(jsonb_typeof(event)='object'),
 PRIMARY KEY(tenant_id,operation_id),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES online_catalog_barcode_operations(tenant_id,operation_id)
);
