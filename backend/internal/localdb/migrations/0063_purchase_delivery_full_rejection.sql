-- Add a terminal ledger without rewriting historical delivery constraints.
CREATE TABLE purchase_delivery_full_rejections (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, delivery_id TEXT NOT NULL,
 rejection_id TEXT NOT NULL, device_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 base_revision INTEGER NOT NULL CHECK(base_revision>0),
 base_payload_json TEXT NOT NULL CHECK(json_valid(base_payload_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)),
 PRIMARY KEY(tenant_id,store_id,delivery_id),
 UNIQUE(tenant_id,store_id,rejection_id),
 FOREIGN KEY(tenant_id,store_id,delivery_id) REFERENCES purchase_deliveries(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,rejection_id) REFERENCES purchase_delivery_rejections(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES purchase_cycle_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TRIGGER purchase_full_rejection_source BEFORE INSERT ON purchase_delivery_full_rejections
 WHEN NOT EXISTS (
 SELECT 1 FROM purchase_deliveries d JOIN purchase_delivery_rejections r
 ON r.tenant_id=d.tenant_id AND r.store_id=d.store_id AND r.order_id=d.order_id
 AND r.supplier_id=d.supplier_id AND r.delivery_reference=d.delivery_reference
 WHERE d.tenant_id=NEW.tenant_id AND d.store_id=NEW.store_id AND d.id=NEW.delivery_id
 AND d.state='arrived' AND d.revision=NEW.base_revision AND d.payload_json=NEW.base_payload_json
 AND r.id=NEW.rejection_id AND r.delivered_milli<=d.planned_milli
 AND r.product_id=json_extract(d.payload_json,'$.product_id') AND r.unit=json_extract(d.payload_json,'$.unit'))
 BEGIN SELECT RAISE(ABORT,'full rejection source conflict'); END;
CREATE TRIGGER purchase_full_rejection_no_update BEFORE UPDATE ON purchase_delivery_full_rejections BEGIN SELECT RAISE(ABORT,'full rejection immutable'); END;
CREATE TRIGGER purchase_full_rejection_no_delete BEFORE DELETE ON purchase_delivery_full_rejections BEGIN SELECT RAISE(ABORT,'full rejection immutable'); END;
CREATE TRIGGER purchase_rejected_delivery_no_update BEFORE UPDATE ON purchase_deliveries
 WHEN EXISTS(SELECT 1 FROM purchase_delivery_full_rejections r WHERE r.tenant_id=OLD.tenant_id AND r.store_id=OLD.store_id AND r.delivery_id=OLD.id)
 BEGIN SELECT RAISE(ABORT,'rejected delivery terminal'); END;
