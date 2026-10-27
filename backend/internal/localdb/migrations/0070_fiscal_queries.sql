-- Status lookups are independent of the original one-attempt issuance.
-- They never release numbers or grant authority to repeat the original POST.
CREATE TABLE fiscal_query_requests (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 query_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 original_request_sha256 TEXT NOT NULL CHECK(length(original_request_sha256)=64),
 request_json TEXT NOT NULL,
 request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,query_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_transmission_requests(tenant_id,store_id,intent_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX fiscal_query_intent ON fiscal_query_requests(tenant_id,store_id,intent_id,created_at,query_id);
CREATE TABLE fiscal_query_results (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 query_id TEXT NOT NULL,
 result_json TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,query_id),
 FOREIGN KEY(tenant_id,store_id,query_id) REFERENCES fiscal_query_requests(tenant_id,store_id,query_id)
) STRICT;
CREATE TRIGGER fiscal_query_requests_no_update BEFORE UPDATE ON fiscal_query_requests
BEGIN SELECT RAISE(ABORT,'fiscal query requests are immutable'); END;
CREATE TRIGGER fiscal_query_requests_no_delete BEFORE DELETE ON fiscal_query_requests
BEGIN SELECT RAISE(ABORT,'fiscal query evidence must be preserved'); END;
CREATE TRIGGER fiscal_query_results_no_update BEFORE UPDATE ON fiscal_query_results
BEGIN SELECT RAISE(ABORT,'fiscal query results are immutable'); END;
CREATE TRIGGER fiscal_query_results_no_delete BEFORE DELETE ON fiscal_query_results
BEGIN SELECT RAISE(ABORT,'fiscal query evidence must be preserved'); END;
