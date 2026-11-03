-- Lots classify finished goods already registered by P06; no stock movements.
CREATE TABLE production_lots (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 result_id TEXT NOT NULL, product_id TEXT NOT NULL, unit TEXT NOT NULL,
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli BETWEEN 1 AND 9007199254740991),
 code TEXT NOT NULL CHECK(length(trim(code)) BETWEEN 1 AND 64),
 manufactured_on TEXT NOT NULL CHECK(length(manufactured_on)=10),
 expires_on TEXT NOT NULL CHECK(expires_on='' OR (length(expires_on)=10 AND expires_on>=manufactured_on)),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255),
 status TEXT NOT NULL CHECK(status IN ('recorded','voided')),
 revision INTEGER NOT NULL CHECK((status='recorded' AND revision=1) OR (status='voided' AND revision=2)),
 created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,code),
 FOREIGN KEY(tenant_id,store_id,result_id) REFERENCES production_results(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_lot_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, lot_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('recorded','voided')),
 revision INTEGER NOT NULL CHECK((kind='recorded' AND revision=1) OR (kind='voided' AND revision=2)),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,lot_id,revision),
 FOREIGN KEY(tenant_id,store_id,lot_id) REFERENCES production_lots(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX production_lots_result ON production_lots(tenant_id,store_id,result_id,status,created_at,id);
