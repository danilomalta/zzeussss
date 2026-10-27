-- Human records of external communication; never remote supplier authentication.
CREATE TABLE purchase_supplier_decisions (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0), payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id)
) STRICT;
CREATE TABLE purchase_deliveries (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL, order_id TEXT NOT NULL,
 supplier_id TEXT NOT NULL, delivery_reference TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0), state TEXT NOT NULL CHECK(state IN('planned','loaded','arrived','received','cancelled')),
 planned_milli INTEGER NOT NULL CHECK(planned_milli BETWEEN 1 AND 9007199254740991),
 receipt_id TEXT, payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,supplier_id,delivery_reference),
 UNIQUE(tenant_id,store_id,receipt_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,receipt_id) REFERENCES purchase_receipts(tenant_id,store_id,id)
) STRICT;
CREATE INDEX purchase_delivery_order ON purchase_deliveries(tenant_id,store_id,order_id,id);
CREATE TABLE purchase_cycle_operations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN('decision','delivery','finance')),
 entity_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision>0),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,kind,entity_id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE purchase_cycle_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL, payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES purchase_cycle_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TABLE payable_receiving_origins (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, payable_id TEXT NOT NULL,
 delivery_id TEXT NOT NULL, receipt_id TEXT NOT NULL, order_id TEXT NOT NULL, supplier_id TEXT NOT NULL,
 receipt_request_json TEXT NOT NULL CHECK(json_valid(receipt_request_json)),
 amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),
 source_reference TEXT NOT NULL, operation_id TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,payable_id), UNIQUE(tenant_id,store_id,receipt_id), UNIQUE(tenant_id,store_id,delivery_id),
 FOREIGN KEY(tenant_id,store_id,device_id,payable_id) REFERENCES supplier_payables(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,delivery_id) REFERENCES purchase_deliveries(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,receipt_id) REFERENCES purchase_receipts(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES purchase_cycle_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TRIGGER purchase_decision_no_delete BEFORE DELETE ON purchase_supplier_decisions BEGIN SELECT RAISE(ABORT,'decision history immutable'); END;
CREATE TRIGGER purchase_decision_revision BEFORE UPDATE ON purchase_supplier_decisions
 WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.order_id IS NOT OLD.order_id OR NEW.revision!=OLD.revision+1
 BEGIN SELECT RAISE(ABORT,'invalid decision revision'); END;
CREATE TRIGGER purchase_delivery_no_delete BEFORE DELETE ON purchase_deliveries BEGIN SELECT RAISE(ABORT,'delivery history immutable'); END;
CREATE TRIGGER purchase_delivery_revision BEFORE UPDATE ON purchase_deliveries
 WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.id IS NOT OLD.id OR NEW.order_id IS NOT OLD.order_id OR NEW.supplier_id IS NOT OLD.supplier_id OR NEW.delivery_reference IS NOT OLD.delivery_reference OR NEW.planned_milli IS NOT OLD.planned_milli OR NEW.revision!=OLD.revision+1
 OR NOT ((OLD.state='planned' AND NEW.state IN('loaded','cancelled')) OR (OLD.state='loaded' AND NEW.state IN('arrived','cancelled')) OR (OLD.state='arrived' AND NEW.state IN('received','cancelled')))
 BEGIN SELECT RAISE(ABORT,'invalid delivery transition'); END;
CREATE TRIGGER purchase_cycle_operations_no_update BEFORE UPDATE ON purchase_cycle_operations BEGIN SELECT RAISE(ABORT,'cycle operations immutable'); END;
CREATE TRIGGER purchase_cycle_operations_no_delete BEFORE DELETE ON purchase_cycle_operations BEGIN SELECT RAISE(ABORT,'cycle operations immutable'); END;
CREATE TRIGGER purchase_cycle_events_no_update BEFORE UPDATE ON purchase_cycle_events BEGIN SELECT RAISE(ABORT,'cycle events immutable'); END;
CREATE TRIGGER purchase_cycle_events_no_delete BEFORE DELETE ON purchase_cycle_events BEGIN SELECT RAISE(ABORT,'cycle events immutable'); END;
CREATE TRIGGER payable_origin_no_update BEFORE UPDATE ON payable_receiving_origins BEGIN SELECT RAISE(ABORT,'receiving origin immutable'); END;
CREATE TRIGGER payable_origin_no_delete BEFORE DELETE ON payable_receiving_origins BEGIN SELECT RAISE(ABORT,'receiving origin immutable'); END;
