-- Recibos de aceite preservados mesmo depois da marcação de envio.
CREATE TABLE outbox_receipts (
    event_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    receipt_id TEXT NOT NULL,
    payload_sha256 TEXT NOT NULL CHECK (length(payload_sha256) = 64),
    confirmed_at TEXT NOT NULL,
    UNIQUE (tenant_id, device_id, receipt_id),
    FOREIGN KEY (event_id) REFERENCES outbox(event_id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id)
) STRICT;
