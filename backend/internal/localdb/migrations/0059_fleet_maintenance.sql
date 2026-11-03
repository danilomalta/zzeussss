-- Tenant/store scoped operational fleet records, immutable operation receipts and local notices.
CREATE TABLE fleet_records (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN('asset','request','trip','plan','provider','quote','expense','income','impact','profile')),
 id TEXT NOT NULL, asset_kind TEXT NOT NULL DEFAULT 'asset' CHECK(asset_kind='asset'), asset_id TEXT,
 state TEXT NOT NULL, assigned_to TEXT, revision INTEGER NOT NULL CHECK(revision>0),
 data_json TEXT NOT NULL CHECK(json_valid(data_json)),
 created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,asset_kind,asset_id) REFERENCES fleet_records(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,assigned_to) REFERENCES memberships(tenant_id,identity_id),
 CHECK((kind IN('asset','provider') AND asset_id IS NULL) OR (kind NOT IN('asset','provider') AND asset_id IS NOT NULL))
) STRICT;
CREATE INDEX fleet_records_queue ON fleet_records(tenant_id,store_id,kind,state,assigned_to,id);
CREATE INDEX fleet_records_asset ON fleet_records(tenant_id,store_id,asset_id,kind,id);
CREATE UNIQUE INDEX fleet_asset_reference ON fleet_records(tenant_id,store_id,json_extract(data_json,'$.reference') COLLATE NOCASE) WHERE kind='asset';
CREATE UNIQUE INDEX fleet_profile_asset ON fleet_records(tenant_id,store_id,asset_id) WHERE kind='profile';
CREATE UNIQUE INDEX fleet_trip_active ON fleet_records(tenant_id,store_id,asset_id) WHERE kind='trip' AND state IN('planned','departed');
CREATE TABLE fleet_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,request_hash TEXT NOT NULL,request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 response_json TEXT NOT NULL CHECK(json_valid(response_json)),restricted INTEGER NOT NULL CHECK(restricted IN(0,1)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE fleet_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,kind TEXT NOT NULL,record_id TEXT NOT NULL,revision INTEGER NOT NULL,
 device_id TEXT NOT NULL,operation_id TEXT NOT NULL,action TEXT NOT NULL,actor_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,kind,record_id,revision),
 FOREIGN KEY(tenant_id,store_id,kind,record_id) REFERENCES fleet_records(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES fleet_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TABLE fleet_notices (
 id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,request_kind TEXT NOT NULL DEFAULT 'request' CHECK(request_kind='request'),request_id TEXT NOT NULL,
 revision INTEGER NOT NULL,recipient_id TEXT,created_at TEXT NOT NULL,
 UNIQUE(tenant_id,store_id,request_id,revision),
 FOREIGN KEY(tenant_id,store_id,request_kind,request_id) REFERENCES fleet_records(tenant_id,store_id,kind,id)
) STRICT;
CREATE INDEX fleet_notices_scope ON fleet_notices(tenant_id,store_id,id);
CREATE TABLE fleet_notice_acks (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,notice_id INTEGER NOT NULL,identity_id TEXT NOT NULL,device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,notice_id,identity_id,device_id),
 FOREIGN KEY(notice_id) REFERENCES fleet_notices(id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES fleet_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
