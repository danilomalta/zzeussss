-- Additive internal workshop evidence. No inventory, money or payroll postings.
CREATE TABLE fleet_workshop_records (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN('template','binding','inspection','labor')),id TEXT NOT NULL,
 request_kind TEXT NOT NULL DEFAULT 'request' CHECK(request_kind='request'),request_id TEXT,
 revision INTEGER NOT NULL CHECK(revision>0),state TEXT NOT NULL,body_json TEXT NOT NULL CHECK(json_valid(body_json)),
 PRIMARY KEY(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,request_kind,request_id) REFERENCES fleet_records(tenant_id,store_id,kind,id),
 CHECK((kind='template' AND request_id IS NULL) OR(kind<>'template' AND request_id IS NOT NULL)),
 CHECK(kind<>'binding' OR id=request_id)
) STRICT;
CREATE INDEX fleet_workshop_queue ON fleet_workshop_records(tenant_id,store_id,kind,request_id,state,id);
CREATE TABLE fleet_workshop_operations (
 seq INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,actor_id TEXT NOT NULL,
 kind TEXT NOT NULL,id TEXT NOT NULL,revision INTEGER NOT NULL,request_hash TEXT NOT NULL,request_json TEXT NOT NULL CHECK(json_valid(request_json)),response_json TEXT NOT NULL CHECK(json_valid(response_json)),
 UNIQUE(tenant_id,store_id,device_id,operation_id),UNIQUE(tenant_id,store_id,kind,id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX fleet_workshop_history ON fleet_workshop_operations(tenant_id,store_id,kind,id,revision);
CREATE INDEX fleet_workshop_labor_worker ON fleet_workshop_records(tenant_id,store_id,json_extract(body_json,'$.data.worker_id'),state,id) WHERE kind='labor';
