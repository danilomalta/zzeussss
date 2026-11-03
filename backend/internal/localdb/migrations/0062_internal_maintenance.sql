-- Internal maintenance directory/classification; no sales, payments or inventory effects.
CREATE TABLE maintenance_registry (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN('specialty','technician','asset','request')),id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0),state TEXT NOT NULL CHECK(state IN('active','inactive')),body_json TEXT NOT NULL CHECK(json_valid(body_json)),
 asset_kind TEXT NOT NULL DEFAULT 'asset' CHECK(asset_kind='asset'),asset_id TEXT,
 request_kind TEXT NOT NULL DEFAULT 'request' CHECK(request_kind='request'),request_id TEXT,identity_id TEXT,
 PRIMARY KEY(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,asset_kind,asset_id) REFERENCES fleet_records(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,store_id,request_kind,request_id) REFERENCES fleet_records(tenant_id,store_id,kind,id),
 FOREIGN KEY(tenant_id,identity_id) REFERENCES memberships(tenant_id,identity_id),
 CHECK((kind='asset' AND asset_id IS NOT NULL AND asset_id=id) OR(kind<>'asset' AND asset_id IS NULL)),
 CHECK((kind='request' AND request_id IS NOT NULL AND request_id=id) OR(kind<>'request' AND request_id IS NULL)),
 CHECK((kind='technician' AND identity_id IS NOT NULL AND identity_id=id) OR(kind<>'technician' AND identity_id IS NULL))
) STRICT;
CREATE UNIQUE INDEX maintenance_specialty_name ON maintenance_registry(tenant_id,store_id,json_extract(body_json,'$.data.name') COLLATE NOCASE) WHERE kind='specialty';
CREATE INDEX maintenance_registry_state ON maintenance_registry(tenant_id,store_id,kind,state,id);
CREATE TABLE maintenance_registry_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,actor_id TEXT NOT NULL,
 kind TEXT NOT NULL,id TEXT NOT NULL,revision INTEGER NOT NULL,request_hash TEXT NOT NULL,request_json TEXT NOT NULL CHECK(json_valid(request_json)),response_json TEXT NOT NULL CHECK(json_valid(response_json)),
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),UNIQUE(tenant_id,store_id,kind,id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
