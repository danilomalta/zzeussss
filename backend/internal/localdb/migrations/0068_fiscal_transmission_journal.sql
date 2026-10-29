CREATE TABLE fiscal_intent_session_bindings (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 PRIMARY KEY(tenant_id,store_id,intent_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_intents(tenant_id,store_id,id)
) STRICT;
CREATE TRIGGER fiscal_intent_session_bindings_no_update BEFORE UPDATE ON fiscal_intent_session_bindings
BEGIN SELECT RAISE(ABORT,'original fiscal session binding is immutable'); END;
CREATE TRIGGER fiscal_intent_session_bindings_no_delete BEFORE DELETE ON fiscal_intent_session_bindings
BEGIN SELECT RAISE(ABORT,'original fiscal session binding must be preserved'); END;
-- Technical homologation journal. This does not enable fiscal authorization.
CREATE TABLE fiscal_transmission_requests (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 access_key TEXT NOT NULL CHECK(length(access_key)=44),
 request_json TEXT NOT NULL,
 request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_intents(tenant_id,store_id,id)
) STRICT;
-- The same issuer/model/series/number cannot be used by another intent,
-- including when its caller changes cNF and therefore changes the full key.
CREATE UNIQUE INDEX fiscal_transmission_number ON fiscal_transmission_requests(
 substr(access_key,7,14),substr(access_key,21,2),substr(access_key,23,3),substr(access_key,26,9)
);
CREATE TABLE fiscal_transmission_results (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 result_json TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_transmission_requests(tenant_id,store_id,intent_id)
) STRICT;
-- A restored snapshot is not authority to send, even for records absent in it.
CREATE TABLE fiscal_restore_barriers (
 tenant_id TEXT PRIMARY KEY REFERENCES tenants(id),
 restored_at TEXT NOT NULL
) STRICT;
CREATE TRIGGER fiscal_transmission_requests_no_update BEFORE UPDATE ON fiscal_transmission_requests
BEGIN SELECT RAISE(ABORT,'fiscal transmission requests are immutable'); END;
CREATE TRIGGER fiscal_transmission_requests_no_delete BEFORE DELETE ON fiscal_transmission_requests
BEGIN SELECT RAISE(ABORT,'fiscal uncertain operations must be preserved'); END;
CREATE TRIGGER fiscal_transmission_results_no_update BEFORE UPDATE ON fiscal_transmission_results
BEGIN SELECT RAISE(ABORT,'fiscal transmission results are immutable'); END;
CREATE TRIGGER fiscal_transmission_results_no_delete BEFORE DELETE ON fiscal_transmission_results
BEGIN SELECT RAISE(ABORT,'fiscal transmission results must be preserved'); END;
CREATE TRIGGER fiscal_restore_barriers_no_update BEFORE UPDATE ON fiscal_restore_barriers
BEGIN SELECT RAISE(ABORT,'fiscal restore barrier requires reconciliation'); END;
CREATE TRIGGER fiscal_restore_barriers_no_delete BEFORE DELETE ON fiscal_restore_barriers
BEGIN SELECT RAISE(ABORT,'fiscal restore barrier requires reconciliation'); END;
