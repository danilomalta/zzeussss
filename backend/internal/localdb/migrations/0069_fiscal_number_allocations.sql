-- Homologation numbering authority for this database only. No production route.
-- A write is acquired before authorization reads, so independent SQLite handles
-- serialize the allocation instead of racing deferred read-to-write upgrades.
CREATE TABLE fiscal_number_lock (id INTEGER PRIMARY KEY CHECK(id=1), marker INTEGER NOT NULL CHECK(marker=0)) STRICT;
INSERT INTO fiscal_number_lock VALUES(1,0);
CREATE TABLE fiscal_number_series (
 issuer TEXT NOT NULL,
 model TEXT NOT NULL CHECK(model IN ('55','65')),
 series INTEGER NOT NULL CHECK(series BETWEEN 0 AND 889),
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 profile_id TEXT NOT NULL,
 profile_revision INTEGER NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 initial_number INTEGER NOT NULL CHECK(initial_number BETWEEN 1 AND 999999999),
 reconciliation_reference TEXT NOT NULL CHECK(length(reconciliation_reference) BETWEEN 8 AND 255),
 created_at TEXT NOT NULL,
 PRIMARY KEY(issuer,model,series),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,profile_id,profile_revision) REFERENCES fiscal_profiles(tenant_id,store_id,profile_id,revision)
) STRICT;
CREATE TABLE fiscal_number_allocations (
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 intent_id TEXT NOT NULL,
 issuer TEXT NOT NULL,
 model TEXT NOT NULL,
 series INTEGER NOT NULL,
 number INTEGER NOT NULL CHECK(number BETWEEN 1 AND 999999999),
 access_key TEXT NOT NULL UNIQUE CHECK(length(access_key)=44),
 issued_at TEXT NOT NULL,
 device_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,intent_id),
 UNIQUE(issuer,model,series,number),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(issuer,model,series) REFERENCES fiscal_number_series(issuer,model,series),
 FOREIGN KEY(tenant_id,store_id,intent_id) REFERENCES fiscal_intents(tenant_id,store_id,id)
) STRICT;
CREATE TRIGGER fiscal_number_series_no_update BEFORE UPDATE ON fiscal_number_series
BEGIN SELECT RAISE(ABORT,'fiscal series origin is immutable'); END;
CREATE TRIGGER fiscal_number_series_no_delete BEFORE DELETE ON fiscal_number_series
BEGIN SELECT RAISE(ABORT,'fiscal series origin must be preserved'); END;
CREATE TRIGGER fiscal_number_allocations_no_update BEFORE UPDATE ON fiscal_number_allocations
BEGIN SELECT RAISE(ABORT,'fiscal number allocation is immutable'); END;
CREATE TRIGGER fiscal_number_allocations_no_delete BEFORE DELETE ON fiscal_number_allocations
BEGIN SELECT RAISE(ABORT,'fiscal allocated number must not be reused'); END;
