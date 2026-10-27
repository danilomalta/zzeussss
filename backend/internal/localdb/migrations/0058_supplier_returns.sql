-- Local collection/return records; supplier observations are explicitly manual.
CREATE TABLE purchase_supplier_returns (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,id TEXT NOT NULL,case_id TEXT NOT NULL,receipt_id TEXT NOT NULL,
 order_id TEXT NOT NULL,supplier_id TEXT NOT NULL,product_id TEXT NOT NULL,unit TEXT NOT NULL,
 requested_milli INTEGER NOT NULL CHECK(requested_milli BETWEEN 1 AND 9007199254740991),
 authorized_milli INTEGER NOT NULL CHECK(authorized_milli BETWEEN 0 AND requested_milli),
 dispatched_milli INTEGER NOT NULL CHECK(dispatched_milli BETWEEN 0 AND authorized_milli),
 supplier_received_milli INTEGER NOT NULL CHECK(supplier_received_milli BETWEEN 0 AND dispatched_milli),
 recovered_milli INTEGER NOT NULL CHECK(recovered_milli BETWEEN 0 AND dispatched_milli AND recovered_milli+supplier_received_milli<=dispatched_milli),
 state TEXT NOT NULL CHECK(state IN('requested','approved','scheduled','dispatched','completed','declined','cancelled')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 4096),payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,case_id) REFERENCES purchase_receiving_cases(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,receipt_id) REFERENCES purchase_receipts(tenant_id,store_id,id),
 CHECK((state IN('requested','approved','scheduled','declined','cancelled') AND dispatched_milli=0 AND supplier_received_milli=0 AND recovered_milli=0)
 OR (state='dispatched' AND dispatched_milli>supplier_received_milli+recovered_milli)
 OR (state='completed' AND dispatched_milli>0 AND dispatched_milli=supplier_received_milli+recovered_milli))
) STRICT;
CREATE INDEX purchase_supplier_returns_receipt ON purchase_supplier_returns(tenant_id,store_id,receipt_id,id);
CREATE INDEX purchase_supplier_returns_case ON purchase_supplier_returns(tenant_id,store_id,case_id,id);
CREATE TABLE purchase_supplier_return_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 return_id TEXT NOT NULL,revision INTEGER NOT NULL,actor_id TEXT NOT NULL,action TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),UNIQUE(tenant_id,store_id,return_id,revision),
 FOREIGN KEY(tenant_id,store_id,return_id) REFERENCES purchase_supplier_returns(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE purchase_supplier_return_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES purchase_supplier_return_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER supplier_return_no_delete BEFORE DELETE ON purchase_supplier_returns BEGIN SELECT RAISE(ABORT,'supplier return history immutable'); END;
CREATE TRIGGER supplier_return_preserve BEFORE UPDATE ON purchase_supplier_returns
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.id IS NOT OLD.id OR NEW.case_id IS NOT OLD.case_id
 OR NEW.receipt_id IS NOT OLD.receipt_id OR NEW.order_id IS NOT OLD.order_id OR NEW.supplier_id IS NOT OLD.supplier_id OR NEW.product_id IS NOT OLD.product_id OR NEW.unit IS NOT OLD.unit OR NEW.requested_milli IS NOT OLD.requested_milli OR NEW.revision!=OLD.revision+1
 OR NOT ((OLD.state='requested' AND NEW.state IN('approved','declined','cancelled')) OR (OLD.state='approved' AND NEW.state IN('scheduled','cancelled')) OR (OLD.state='scheduled' AND NEW.state IN('scheduled','dispatched','cancelled')) OR (OLD.state='dispatched' AND NEW.state IN('dispatched','completed')))
BEGIN SELECT RAISE(ABORT,'invalid supplier return mutation'); END;
CREATE TRIGGER supplier_return_ops_no_update BEFORE UPDATE ON purchase_supplier_return_operations BEGIN SELECT RAISE(ABORT,'supplier return operations immutable'); END;
CREATE TRIGGER supplier_return_ops_no_delete BEFORE DELETE ON purchase_supplier_return_operations BEGIN SELECT RAISE(ABORT,'supplier return operations immutable'); END;
CREATE TRIGGER supplier_return_events_no_update BEFORE UPDATE ON purchase_supplier_return_events BEGIN SELECT RAISE(ABORT,'supplier return events immutable'); END;
CREATE TRIGGER supplier_return_events_no_delete BEFORE DELETE ON purchase_supplier_return_events BEGIN SELECT RAISE(ABORT,'supplier return events immutable'); END;
