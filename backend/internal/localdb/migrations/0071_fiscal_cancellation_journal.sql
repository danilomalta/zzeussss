-- One immutable cancellation110111/sequence1 per original fiscal intent.
-- This is independent of sale refunds and stock movements.
CREATE INDEX fiscal_query_latest_reservation ON fiscal_query_requests(tenant_id,store_id,intent_id);
CREATE TABLE fiscal_cancellation_requests (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 query_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 authorization_sha256 TEXT NOT NULL CHECK(length(authorization_sha256)=64),
 approval_reference TEXT NOT NULL CHECK(length(approval_reference) BETWEEN 1 AND 128),
 request_json TEXT NOT NULL,
 request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),
 UNIQUE(tenant_id,store_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_transmission_requests(tenant_id,store_id,intent_id),
 FOREIGN KEY(tenant_id,store_id,query_id) REFERENCES fiscal_query_requests(tenant_id,store_id,query_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE fiscal_cancellation_results (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 result_json TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_cancellation_requests(tenant_id,store_id,intent_id)
) STRICT;
CREATE TRIGGER fiscal_cancellation_requests_no_update BEFORE UPDATE ON fiscal_cancellation_requests
BEGIN SELECT RAISE(ABORT,'fiscal cancellation request is immutable'); END;
CREATE TRIGGER fiscal_cancellation_requests_no_delete BEFORE DELETE ON fiscal_cancellation_requests
BEGIN SELECT RAISE(ABORT,'fiscal cancellation uncertainty must be preserved'); END;
CREATE TRIGGER fiscal_cancellation_results_no_update BEFORE UPDATE ON fiscal_cancellation_results
BEGIN SELECT RAISE(ABORT,'fiscal cancellation result is immutable'); END;
CREATE TRIGGER fiscal_cancellation_results_no_delete BEFORE DELETE ON fiscal_cancellation_results
BEGIN SELECT RAISE(ABORT,'fiscal cancellation evidence must be preserved'); END;
