-- Local supplier obligations only. Approval never executes or proves payment.
CREATE TABLE supplier_payables (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,
 supplier_id TEXT NOT NULL,supplier_name TEXT NOT NULL,document_reference TEXT NOT NULL COLLATE NOCASE,
 description TEXT NOT NULL,issued_on TEXT NOT NULL,currency TEXT NOT NULL CHECK(currency='BRL'),
 total_cents INTEGER NOT NULL CHECK(total_cents BETWEEN 1 AND 9007199254740991),
 creator_id TEXT NOT NULL,state TEXT NOT NULL CHECK(state IN ('pending','approved','rejected','cancelled')),
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL,last_operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 UNIQUE(tenant_id,store_id,supplier_id,document_reference),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,creator_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,last_operation_id) REFERENCES supplier_payable_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE supplier_payable_installments (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,payable_id TEXT NOT NULL,id TEXT NOT NULL,
 position INTEGER NOT NULL CHECK(position BETWEEN 1 AND 120),due_on TEXT NOT NULL,
 amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(tenant_id,payable_id,id),UNIQUE(tenant_id,payable_id,position),
 FOREIGN KEY(tenant_id,store_id,device_id,payable_id) REFERENCES supplier_payables(tenant_id,store_id,device_id,id)
) STRICT;
CREATE TABLE supplier_payable_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 payable_id TEXT NOT NULL,actor_id TEXT NOT NULL,action TEXT NOT NULL,from_state TEXT NOT NULL,to_state TEXT NOT NULL,
 reason TEXT NOT NULL,request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),UNIQUE(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,payable_id) REFERENCES supplier_payables(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
-- Private durable events; not sent by the legacy commercial wire protocol.
CREATE TABLE supplier_payable_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,schema_version INTEGER NOT NULL CHECK(schema_version=1),payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES supplier_payable_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE INDEX supplier_payable_queue ON supplier_payables(tenant_id,store_id,device_id,created_at,id);
CREATE TRIGGER supplier_payable_no_delete BEFORE DELETE ON supplier_payables BEGIN SELECT RAISE(ABORT,'payable history immutable'); END;
CREATE TRIGGER supplier_payable_preserve BEFORE UPDATE ON supplier_payables
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id OR NEW.supplier_id IS NOT OLD.supplier_id OR NEW.supplier_name IS NOT OLD.supplier_name OR NEW.document_reference IS NOT OLD.document_reference OR NEW.description IS NOT OLD.description OR NEW.issued_on IS NOT OLD.issued_on OR NEW.currency IS NOT OLD.currency OR NEW.total_cents IS NOT OLD.total_cents OR NEW.creator_id IS NOT OLD.creator_id OR NEW.created_at IS NOT OLD.created_at
 OR NOT ((OLD.state='pending' AND NEW.state IN ('approved','rejected','cancelled')) OR (OLD.state='approved' AND NEW.state='cancelled'))
BEGIN SELECT RAISE(ABORT,'invalid payable mutation'); END;
CREATE TRIGGER supplier_payable_part_no_update BEFORE UPDATE ON supplier_payable_installments BEGIN SELECT RAISE(ABORT,'installments immutable'); END;
CREATE TRIGGER supplier_payable_part_no_delete BEFORE DELETE ON supplier_payable_installments BEGIN SELECT RAISE(ABORT,'installments immutable'); END;
CREATE TRIGGER supplier_payable_ops_no_update BEFORE UPDATE ON supplier_payable_operations BEGIN SELECT RAISE(ABORT,'operations immutable'); END;
CREATE TRIGGER supplier_payable_ops_no_delete BEFORE DELETE ON supplier_payable_operations BEGIN SELECT RAISE(ABORT,'operations immutable'); END;
CREATE TRIGGER supplier_payable_events_no_update BEFORE UPDATE ON supplier_payable_events BEGIN SELECT RAISE(ABORT,'events immutable'); END;
CREATE TRIGGER supplier_payable_events_no_delete BEFORE DELETE ON supplier_payable_events BEGIN SELECT RAISE(ABORT,'events immutable'); END;
