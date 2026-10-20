-- Classify existing audited cash flows; never debit or credit the drawer twice.
CREATE TABLE payable_cash_payments (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,
 payable_id TEXT NOT NULL,installment_id TEXT NOT NULL,withdrawal_operation_id TEXT NOT NULL,
 amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),
 actor_id TEXT NOT NULL,recipient TEXT NOT NULL,reference TEXT NOT NULL,created_at TEXT NOT NULL,operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 UNIQUE(tenant_id,device_id,withdrawal_operation_id),
 UNIQUE(tenant_id,store_id,payable_id,reference),
 FOREIGN KEY(tenant_id,store_id,device_id,payable_id) REFERENCES supplier_payables(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,payable_id,installment_id) REFERENCES supplier_payable_installments(tenant_id,payable_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,withdrawal_operation_id) REFERENCES cash_adjustments(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES payable_cash_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE payable_cash_returns (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,payment_id TEXT NOT NULL,
 supply_operation_id TEXT NOT NULL,actor_id TEXT NOT NULL,reason TEXT NOT NULL,created_at TEXT NOT NULL,operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,payment_id),UNIQUE(tenant_id,device_id,supply_operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,payment_id) REFERENCES payable_cash_payments(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,supply_operation_id) REFERENCES cash_adjustments(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES payable_cash_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE payable_cash_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 payment_id TEXT NOT NULL,actor_id TEXT NOT NULL,action TEXT NOT NULL CHECK(action IN ('pay','return')),
 request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),UNIQUE(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,payment_id) REFERENCES payable_cash_payments(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE payable_cash_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,schema_version INTEGER NOT NULL CHECK(schema_version=1),payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES payable_cash_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE INDEX payable_cash_document ON payable_cash_payments(tenant_id,store_id,device_id,payable_id,created_at,id);
CREATE TRIGGER payable_cash_requires_approved BEFORE INSERT ON payable_cash_payments
WHEN NOT EXISTS(SELECT 1 FROM supplier_payables p WHERE p.tenant_id=NEW.tenant_id AND p.store_id=NEW.store_id AND p.device_id=NEW.device_id AND p.id=NEW.payable_id AND p.state='approved')
 OR EXISTS(SELECT 1 FROM cash_bags b WHERE b.tenant_id=NEW.tenant_id AND b.device_id=NEW.device_id AND b.withdrawal_operation_id=NEW.withdrawal_operation_id)
BEGIN SELECT RAISE(ABORT,'payable not approved or withdrawal already has custody'); END;
CREATE TRIGGER payable_cash_blocks_bag BEFORE INSERT ON cash_bags
WHEN EXISTS(SELECT 1 FROM payable_cash_payments p WHERE p.tenant_id=NEW.tenant_id AND p.device_id=NEW.device_id AND p.withdrawal_operation_id=NEW.withdrawal_operation_id)
BEGIN SELECT RAISE(ABORT,'withdrawal already allocated to supplier'); END;
CREATE TRIGGER payable_cash_blocks_cancel BEFORE UPDATE ON supplier_payables
WHEN NEW.state='cancelled' AND EXISTS(SELECT 1 FROM payable_cash_payments p LEFT JOIN payable_cash_returns r ON r.tenant_id=p.tenant_id AND r.payment_id=p.id WHERE p.tenant_id=NEW.tenant_id AND p.payable_id=NEW.id AND r.payment_id IS NULL)
BEGIN SELECT RAISE(ABORT,'supplier cash must be returned before cancellation'); END;
CREATE TRIGGER payable_return_not_vault BEFORE INSERT ON payable_cash_returns
WHEN EXISTS(SELECT 1 FROM cash_vault_entries v JOIN cash_adjustments a ON a.tenant_id=v.tenant_id AND a.device_id=v.device_id AND a.movement_id=v.cash_movement_id WHERE a.tenant_id=NEW.tenant_id AND a.device_id=NEW.device_id AND a.operation_id=NEW.supply_operation_id)
BEGIN SELECT RAISE(ABORT,'vault supply is not a supplier return'); END;
CREATE TRIGGER payable_return_blocks_vault BEFORE INSERT ON cash_vault_entries
WHEN NEW.kind='cash_supply' AND EXISTS(SELECT 1 FROM payable_cash_returns r JOIN cash_adjustments a ON a.tenant_id=r.tenant_id AND a.device_id=r.device_id AND a.operation_id=r.supply_operation_id WHERE a.tenant_id=NEW.tenant_id AND a.device_id=NEW.device_id AND a.movement_id=NEW.cash_movement_id)
BEGIN SELECT RAISE(ABORT,'supplier return cannot also debit vault'); END;
CREATE TRIGGER payable_cash_no_update BEFORE UPDATE ON payable_cash_payments BEGIN SELECT RAISE(ABORT,'cash payment immutable'); END;
CREATE TRIGGER payable_cash_no_delete BEFORE DELETE ON payable_cash_payments BEGIN SELECT RAISE(ABORT,'cash payment immutable'); END;
CREATE TRIGGER payable_return_no_update BEFORE UPDATE ON payable_cash_returns BEGIN SELECT RAISE(ABORT,'cash return immutable'); END;
CREATE TRIGGER payable_return_no_delete BEFORE DELETE ON payable_cash_returns BEGIN SELECT RAISE(ABORT,'cash return immutable'); END;
CREATE TRIGGER payable_cash_ops_no_update BEFORE UPDATE ON payable_cash_operations BEGIN SELECT RAISE(ABORT,'cash operation immutable'); END;
CREATE TRIGGER payable_cash_ops_no_delete BEFORE DELETE ON payable_cash_operations BEGIN SELECT RAISE(ABORT,'cash operation immutable'); END;
CREATE TRIGGER payable_cash_events_no_update BEFORE UPDATE ON payable_cash_events BEGIN SELECT RAISE(ABORT,'cash event immutable'); END;
CREATE TRIGGER payable_cash_events_no_delete BEFORE DELETE ON payable_cash_events BEGIN SELECT RAISE(ABORT,'cash event immutable'); END;
