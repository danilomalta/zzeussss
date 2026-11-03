-- Local human follow-up. Never an external agreement, refund or stock receipt.
CREATE TABLE purchase_receiving_cases (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,id TEXT NOT NULL,order_id TEXT NOT NULL,supplier_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN('shortage','damage','wrong_product','other')),
 requested_outcome TEXT NOT NULL CHECK(requested_outcome IN('review','replacement','credit','refund','collection','other')),
 reference TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN('open','in_progress','resolved','cancelled')),
 assigned_to TEXT,due_at TEXT,revision INTEGER NOT NULL CHECK(revision>0),
 opening_evidence_json TEXT NOT NULL CHECK(json_valid(opening_evidence_json)),
 created_by TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,last_reason TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,assigned_to) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX purchase_receiving_cases_queue ON purchase_receiving_cases(tenant_id,store_id,status,due_at,id);
CREATE INDEX purchase_receiving_cases_order ON purchase_receiving_cases(tenant_id,store_id,order_id,id);
CREATE TABLE purchase_receiving_case_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,request_hash TEXT NOT NULL,request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 response_json TEXT NOT NULL CHECK(json_valid(response_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE purchase_receiving_case_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,case_id TEXT NOT NULL,revision INTEGER NOT NULL,
 action TEXT NOT NULL,actor_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,case_id,revision),
 FOREIGN KEY(tenant_id,store_id,case_id) REFERENCES purchase_receiving_cases(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES purchase_receiving_case_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
