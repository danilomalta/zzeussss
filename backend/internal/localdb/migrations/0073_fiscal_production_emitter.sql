-- Production authority is independent of the homologation series/journal.
-- No historical document or uncertain operation is removed or overwritten.
CREATE TABLE fiscal_production_series (
 issuer TEXT NOT NULL, model TEXT NOT NULL CHECK(model IN ('55','65')), series INTEGER NOT NULL CHECK(series BETWEEN 0 AND 889),
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, profile_id TEXT NOT NULL,
 initial_number INTEGER NOT NULL CHECK(initial_number BETWEEN 1 AND 999999999),
 authority_reference TEXT NOT NULL, actor_id TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(issuer,model,series)
) STRICT;
CREATE TABLE fiscal_production_allocations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, intent_id TEXT NOT NULL,
 issuer TEXT NOT NULL, model TEXT NOT NULL, series INTEGER NOT NULL,
 number INTEGER NOT NULL CHECK(number BETWEEN 1 AND 999999999), access_key TEXT NOT NULL CHECK(length(access_key)=44),
 issued_at TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id), UNIQUE(issuer,model,series,number), UNIQUE(access_key),
 FOREIGN KEY(issuer,model,series) REFERENCES fiscal_production_series(issuer,model,series),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_intents(tenant_id,store_id,id)
) STRICT;
CREATE TABLE fiscal_production_preparations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, intent_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision>0),
 device_id TEXT NOT NULL, actor_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 config_sha256 TEXT NOT NULL CHECK(length(config_sha256)=64), config_json TEXT NOT NULL CHECK(json_valid(config_json)), session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 invoice_json TEXT NOT NULL CHECK(json_valid(invoice_json)), invoice_sha256 TEXT NOT NULL CHECK(length(invoice_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id,revision), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_production_allocations(tenant_id,store_id,intent_id)
) STRICT;
CREATE TABLE fiscal_production_approvals (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, intent_id TEXT NOT NULL, revision INTEGER NOT NULL,
 device_id TEXT NOT NULL, actor_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 tax_reference TEXT NOT NULL, invoice_sha256 TEXT NOT NULL CHECK(length(invoice_sha256)=64), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id,revision), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id,revision) REFERENCES fiscal_production_preparations(tenant_id,store_id,intent_id,revision)
) STRICT;
CREATE TABLE fiscal_production_requests (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, intent_id TEXT NOT NULL, revision INTEGER NOT NULL,
 device_id TEXT NOT NULL, actor_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('issue','query')), config_json TEXT NOT NULL CHECK(json_valid(config_json)), request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,intent_id,revision) REFERENCES fiscal_production_preparations(tenant_id,store_id,intent_id,revision)
) STRICT;
CREATE UNIQUE INDEX fiscal_production_issue_once ON fiscal_production_requests(tenant_id,store_id,intent_id,revision) WHERE kind='issue';
CREATE TABLE fiscal_production_results (
 tenant_id TEXT NOT NULL, device_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,device_id,operation_id) REFERENCES fiscal_production_requests(tenant_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER fiscal_production_series_no_update BEFORE UPDATE ON fiscal_production_series BEGIN SELECT RAISE(ABORT,'production numbering origin immutable'); END;
CREATE TRIGGER fiscal_production_series_no_delete BEFORE DELETE ON fiscal_production_series BEGIN SELECT RAISE(ABORT,'production numbering origin preserved'); END;
CREATE TRIGGER fiscal_production_allocations_no_update BEFORE UPDATE ON fiscal_production_allocations BEGIN SELECT RAISE(ABORT,'production allocation immutable'); END;
CREATE TRIGGER fiscal_production_allocations_no_delete BEFORE DELETE ON fiscal_production_allocations BEGIN SELECT RAISE(ABORT,'production number must not be reused'); END;
CREATE TRIGGER fiscal_production_preparations_no_update BEFORE UPDATE ON fiscal_production_preparations BEGIN SELECT RAISE(ABORT,'production preparation immutable'); END;
CREATE TRIGGER fiscal_production_preparations_no_delete BEFORE DELETE ON fiscal_production_preparations BEGIN SELECT RAISE(ABORT,'production preparation preserved'); END;
CREATE TRIGGER fiscal_production_approvals_no_update BEFORE UPDATE ON fiscal_production_approvals BEGIN SELECT RAISE(ABORT,'production approval immutable'); END;
CREATE TRIGGER fiscal_production_approvals_no_delete BEFORE DELETE ON fiscal_production_approvals BEGIN SELECT RAISE(ABORT,'production approval preserved'); END;
CREATE TRIGGER fiscal_production_requests_no_update BEFORE UPDATE ON fiscal_production_requests BEGIN SELECT RAISE(ABORT,'production request immutable'); END;
CREATE TRIGGER fiscal_production_requests_no_delete BEFORE DELETE ON fiscal_production_requests BEGIN SELECT RAISE(ABORT,'uncertain production request preserved'); END;
CREATE TRIGGER fiscal_production_results_no_update BEFORE UPDATE ON fiscal_production_results BEGIN SELECT RAISE(ABORT,'production result immutable'); END;
CREATE TRIGGER fiscal_production_results_no_delete BEFORE DELETE ON fiscal_production_results BEGIN SELECT RAISE(ABORT,'production result preserved'); END;
