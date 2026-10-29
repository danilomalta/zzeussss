-- Pending ranges are durable exclusions, including before POST and after
-- rejection/uncertainty. No automatic release or number reuse is authorized.
CREATE TABLE fiscal_invalidation_requests (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 issuer TEXT NOT NULL,
 model TEXT NOT NULL,
 series INTEGER NOT NULL,
 fiscal_year INTEGER NOT NULL CHECK(fiscal_year BETWEEN 2000 AND 2099),
 first_number INTEGER NOT NULL CHECK(first_number BETWEEN 1 AND 999999999),
 last_number INTEGER NOT NULL CHECK(last_number>=first_number AND last_number<=999999999 AND last_number-first_number<10000),
 profile_id TEXT NOT NULL,
 profile_revision INTEGER NOT NULL,
 actor_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 session_sha256 BLOB NOT NULL CHECK(length(session_sha256)=32),
 approval_reference TEXT NOT NULL,
 request_json TEXT NOT NULL,
 request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64),
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,operation_id),
 FOREIGN KEY(issuer,model,series) REFERENCES fiscal_number_series(issuer,model,series),
 FOREIGN KEY(tenant_id,store_id,profile_id,profile_revision) REFERENCES fiscal_profiles(tenant_id,store_id,profile_id,revision),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id)
) STRICT;
CREATE INDEX fiscal_invalidation_ranges ON fiscal_invalidation_requests(issuer,model,series,first_number,last_number);
CREATE TABLE fiscal_invalidation_results (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 result_json TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,operation_id) REFERENCES fiscal_invalidation_requests(tenant_id,store_id,operation_id)
) STRICT;
CREATE TRIGGER fiscal_invalidation_requests_guard BEFORE INSERT ON fiscal_invalidation_requests
WHEN EXISTS(SELECT 1 FROM fiscal_invalidation_requests i WHERE i.issuer=NEW.issuer AND i.model=NEW.model AND i.series=NEW.series AND i.first_number<=NEW.last_number AND i.last_number>=NEW.first_number)
 OR EXISTS(SELECT 1 FROM fiscal_number_allocations a WHERE a.issuer=NEW.issuer AND a.model=NEW.model AND a.series=NEW.series AND a.number BETWEEN NEW.first_number AND NEW.last_number)
 OR EXISTS(SELECT 1 FROM fiscal_transmission_requests t WHERE substr(t.access_key,7,14)=NEW.issuer AND substr(t.access_key,21,2)=NEW.model AND CAST(substr(t.access_key,23,3) AS INTEGER)=NEW.series AND CAST(substr(t.access_key,26,9) AS INTEGER) BETWEEN NEW.first_number AND NEW.last_number)
BEGIN SELECT RAISE(ABORT,'fiscal range overlaps an existing reservation or document'); END;
CREATE TRIGGER fiscal_number_allocations_invalidation_guard BEFORE INSERT ON fiscal_number_allocations
WHEN EXISTS(SELECT 1 FROM fiscal_invalidation_requests i WHERE i.issuer=NEW.issuer AND i.model=NEW.model AND i.series=NEW.series AND NEW.number BETWEEN i.first_number AND i.last_number)
BEGIN SELECT RAISE(ABORT,'fiscal number belongs to an invalidation reservation'); END;
CREATE TRIGGER fiscal_transmission_invalidation_guard BEFORE INSERT ON fiscal_transmission_requests
WHEN EXISTS(SELECT 1 FROM fiscal_invalidation_requests i WHERE i.issuer=substr(NEW.access_key,7,14) AND i.model=substr(NEW.access_key,21,2) AND i.series=CAST(substr(NEW.access_key,23,3) AS INTEGER) AND CAST(substr(NEW.access_key,26,9) AS INTEGER) BETWEEN i.first_number AND i.last_number)
BEGIN SELECT RAISE(ABORT,'fiscal document belongs to an invalidation reservation'); END;
CREATE TRIGGER fiscal_invalidation_requests_no_update BEFORE UPDATE ON fiscal_invalidation_requests
BEGIN SELECT RAISE(ABORT,'fiscal invalidation request is immutable'); END;
CREATE TRIGGER fiscal_invalidation_requests_no_delete BEFORE DELETE ON fiscal_invalidation_requests
BEGIN SELECT RAISE(ABORT,'fiscal invalidation range must not be released'); END;
CREATE TRIGGER fiscal_invalidation_results_no_update BEFORE UPDATE ON fiscal_invalidation_results
BEGIN SELECT RAISE(ABORT,'fiscal invalidation result is immutable'); END;
CREATE TRIGGER fiscal_invalidation_results_no_delete BEFORE DELETE ON fiscal_invalidation_results
BEGIN SELECT RAISE(ABORT,'fiscal invalidation evidence must be preserved'); END;
