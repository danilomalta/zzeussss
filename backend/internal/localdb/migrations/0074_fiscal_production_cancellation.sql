-- One explicit production cancellation event per original intent. Never release
-- the fiscal number or erase an uncertain event; no cash/stock side effects.
CREATE TABLE fiscal_production_cancellation_requests (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,intent_id TEXT NOT NULL,query_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 device_id TEXT NOT NULL,actor_id TEXT NOT NULL,session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 authorization_sha256 TEXT NOT NULL CHECK(length(authorization_sha256)=64),approval_reference TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 config_json TEXT NOT NULL CHECK(json_valid(config_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_production_allocations(tenant_id,store_id,intent_id)
) STRICT;
CREATE TABLE fiscal_production_cancellation_results (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,intent_id TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id), FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_production_cancellation_requests(tenant_id,store_id,intent_id)
) STRICT;
CREATE TRIGGER fiscal_production_cancellation_requests_no_update BEFORE UPDATE ON fiscal_production_cancellation_requests BEGIN SELECT RAISE(ABORT,'production cancellation immutable'); END;
CREATE TRIGGER fiscal_production_cancellation_requests_no_delete BEFORE DELETE ON fiscal_production_cancellation_requests BEGIN SELECT RAISE(ABORT,'uncertain production cancellation preserved'); END;
CREATE TRIGGER fiscal_production_cancellation_results_no_update BEFORE UPDATE ON fiscal_production_cancellation_results BEGIN SELECT RAISE(ABORT,'production cancellation result immutable'); END;
CREATE TRIGGER fiscal_production_cancellation_results_no_delete BEFORE DELETE ON fiscal_production_cancellation_results BEGIN SELECT RAISE(ABORT,'production cancellation result preserved'); END;
