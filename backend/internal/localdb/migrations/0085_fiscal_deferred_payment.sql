-- Local immutable fiscal choice; never queued as a peer/financial command.
CREATE TABLE fiscal_deferred_payments (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 sale_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 document_id TEXT NOT NULL, payment_code TEXT NOT NULL CHECK(payment_code IN ('21','91')),
 total_cents INTEGER NOT NULL CHECK(total_cents>0), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,sale_id),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,sale_id) REFERENCES sales(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,document_id) REFERENCES receivable_documents(tenant_id,store_id,id)
) STRICT;
CREATE TRIGGER fiscal_deferred_immutable_update BEFORE UPDATE ON fiscal_deferred_payments BEGIN SELECT RAISE(ABORT,'immutable deferred fiscal choice'); END;
CREATE TRIGGER fiscal_deferred_immutable_delete BEFORE DELETE ON fiscal_deferred_payments BEGIN SELECT RAISE(ABORT,'immutable deferred fiscal choice'); END;
