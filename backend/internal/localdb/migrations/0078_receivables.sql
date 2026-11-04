-- Manual external commercial documents. Not tax invoices or a second sale.
CREATE TABLE receivable_documents (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 customer_reference TEXT NOT NULL, document_reference TEXT NOT NULL, description TEXT NOT NULL,
 issued_on TEXT NOT NULL,total_cents INTEGER NOT NULL CHECK(total_cents BETWEEN 1 AND 9007199254740991),
 state TEXT NOT NULL CHECK(state IN ('open','cancelled')),created_by TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,customer_reference,document_reference),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE receivable_installments (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,document_id TEXT NOT NULL,id TEXT NOT NULL,
 due_on TEXT NOT NULL,amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(tenant_id,store_id,document_id,id),
 FOREIGN KEY(tenant_id,store_id,document_id) REFERENCES receivable_documents(tenant_id,store_id,id)
) STRICT;
CREATE TABLE receivable_cash_receipts (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,document_id TEXT NOT NULL,installment_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,movement_operation_id TEXT NOT NULL,movement_id TEXT NOT NULL,session_id TEXT NOT NULL,
 amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),actor_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),UNIQUE(tenant_id,device_id,movement_operation_id),
 FOREIGN KEY(tenant_id,store_id,document_id,installment_id) REFERENCES receivable_installments(tenant_id,store_id,document_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,movement_operation_id) REFERENCES cash_adjustments(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES receivable_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE receivable_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,actor_id TEXT NOT NULL,
 document_id TEXT NOT NULL,action TEXT NOT NULL,reason TEXT NOT NULL,request_sha256 TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,document_id) REFERENCES receivable_documents(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
-- An incoming physical cash movement cannot pay a receivable AND represent a supplier refund.
CREATE TRIGGER receivable_cash_excludes_supplier_return BEFORE INSERT ON receivable_cash_receipts
WHEN EXISTS(SELECT 1 FROM payable_cash_returns p WHERE p.tenant_id=NEW.tenant_id AND p.device_id=NEW.device_id AND p.supply_operation_id=NEW.movement_operation_id)
BEGIN SELECT RAISE(ABORT,'cash source already classified'); END;
CREATE TRIGGER supplier_return_excludes_receivable_cash BEFORE INSERT ON payable_cash_returns
WHEN EXISTS(SELECT 1 FROM receivable_cash_receipts p WHERE p.tenant_id=NEW.tenant_id AND p.device_id=NEW.device_id AND p.movement_operation_id=NEW.supply_operation_id)
BEGIN SELECT RAISE(ABORT,'cash source already classified'); END;
CREATE INDEX receivable_by_store ON receivable_documents(tenant_id,store_id,created_at,id);
CREATE TRIGGER immutable_receivable_installments_update BEFORE UPDATE ON receivable_installments BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_installments_delete BEFORE DELETE ON receivable_installments BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_cash_receipts_update BEFORE UPDATE ON receivable_cash_receipts BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_cash_receipts_delete BEFORE DELETE ON receivable_cash_receipts BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_operations_update BEFORE UPDATE ON receivable_operations BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_operations_delete BEFORE DELETE ON receivable_operations BEGIN SELECT RAISE(ABORT,'immutable receivable evidence'); END;
CREATE TRIGGER immutable_receivable_document BEFORE UPDATE ON receivable_documents WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.id IS NOT OLD.id OR NEW.customer_reference IS NOT OLD.customer_reference OR NEW.document_reference IS NOT OLD.document_reference OR NEW.description IS NOT OLD.description OR NEW.issued_on IS NOT OLD.issued_on OR NEW.total_cents IS NOT OLD.total_cents OR NEW.created_by IS NOT OLD.created_by OR NEW.created_at IS NOT OLD.created_at OR OLD.state!='open' OR NEW.state!='cancelled' BEGIN SELECT RAISE(ABORT,'immutable receivable document'); END;
CREATE TRIGGER no_receivable_delete BEFORE DELETE ON receivable_documents BEGIN SELECT RAISE(ABORT,'immutable receivable document'); END;

CREATE TABLE receivable_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES receivable_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER immutable_receivable_events_update BEFORE UPDATE ON receivable_events BEGIN SELECT RAISE(ABORT,'immutable receivable event'); END;
CREATE TRIGGER immutable_receivable_events_delete BEFORE DELETE ON receivable_events BEGIN SELECT RAISE(ABORT,'immutable receivable event'); END;
