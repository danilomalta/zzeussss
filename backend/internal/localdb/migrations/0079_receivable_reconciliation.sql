-- An explicit, scoped financial fence; never a cancellation/refund of a receipt.
CREATE TABLE receivable_reconciliations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,document_id TEXT NOT NULL,action TEXT NOT NULL CHECK(action IN('create','receive_cash','cancel')),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TRIGGER receivable_reconciliation_no_operation BEFORE INSERT ON receivable_reconciliations
WHEN EXISTS(SELECT 1 FROM receivable_operations o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.device_id=NEW.device_id AND o.operation_id=NEW.operation_id)
BEGIN SELECT RAISE(ABORT,'receivable operation already applied'); END;
CREATE TRIGGER receivable_operation_not_retired BEFORE INSERT ON receivable_operations
WHEN EXISTS(SELECT 1 FROM receivable_reconciliations o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.device_id=NEW.device_id AND o.operation_id=NEW.operation_id)
BEGIN SELECT RAISE(ABORT,'receivable operation retired'); END;
CREATE TRIGGER immutable_receivable_reconciliation_update BEFORE UPDATE ON receivable_reconciliations BEGIN SELECT RAISE(ABORT,'immutable reconciliation'); END;
CREATE TRIGGER immutable_receivable_reconciliation_delete BEFORE DELETE ON receivable_reconciliations BEGIN SELECT RAISE(ABORT,'immutable reconciliation'); END;
