-- Append-only human quality verdicts for the entire declared finished-goods lot.
CREATE TABLE production_quality_reviews (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, lot_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('passed','failed')),
 criterion TEXT NOT NULL CHECK(length(trim(criterion)) BETWEEN 1 AND 255),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255),
 lot_snapshot_json TEXT NOT NULL CHECK(json_valid(lot_snapshot_json)),
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,lot_id,revision),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,lot_id) REFERENCES production_lots(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
