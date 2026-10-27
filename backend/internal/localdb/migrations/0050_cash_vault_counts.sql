-- Physical counts freeze existing vaults; no money is created or adjusted.
CREATE TABLE cash_vault_counts (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,vault_id TEXT NOT NULL,
 creator_id TEXT NOT NULL,counter_id TEXT NOT NULL,reason TEXT NOT NULL,
 expected_cents INTEGER NOT NULL CHECK(expected_cents BETWEEN 0 AND 9007199254740991),
 vault_operation_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('counting','submitted','accepted','rejected','cancelled')),
 counted_cents INTEGER CHECK(counted_cents BETWEEN 0 AND 9007199254740991),
 difference_cents INTEGER,reviewer_id TEXT,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL,last_operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 CHECK(creator_id<>counter_id),CHECK(reviewer_id IS NULL OR reviewer_id<>counter_id),
 CHECK((state IN ('counting','cancelled') AND counted_cents IS NULL AND difference_cents IS NULL AND reviewer_id IS NULL)
 OR (state='submitted' AND counted_cents IS NOT NULL AND difference_cents=counted_cents-expected_cents AND reviewer_id IS NULL)
 OR (state IN ('accepted','rejected') AND counted_cents IS NOT NULL AND difference_cents=counted_cents-expected_cents AND reviewer_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id,device_id,vault_id) REFERENCES cash_vaults(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,creator_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,counter_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,reviewer_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,last_operation_id) REFERENCES cash_vault_count_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE UNIQUE INDEX cash_vault_one_open_count ON cash_vault_counts(tenant_id,store_id,device_id,vault_id) WHERE state IN ('counting','submitted');
CREATE TABLE cash_vault_count_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 count_id TEXT NOT NULL,actor_id TEXT NOT NULL,action TEXT NOT NULL,
 from_state TEXT NOT NULL,to_state TEXT NOT NULL,reason TEXT NOT NULL,
 request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),UNIQUE(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,count_id) REFERENCES cash_vault_counts(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE cash_vault_count_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,schema_version INTEGER NOT NULL CHECK(schema_version=1),payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES cash_vault_count_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER cash_vault_frozen BEFORE UPDATE ON cash_vaults
WHEN EXISTS(SELECT 1 FROM cash_vault_counts c WHERE c.tenant_id=OLD.tenant_id AND c.store_id=OLD.store_id AND c.device_id=OLD.device_id AND c.vault_id=OLD.id AND c.state IN ('counting','submitted'))
BEGIN SELECT RAISE(ABORT,'vault has an open physical count'); END;
CREATE TRIGGER cash_vault_count_no_delete BEFORE DELETE ON cash_vault_counts BEGIN SELECT RAISE(ABORT,'count history immutable'); END;
CREATE TRIGGER cash_vault_count_preserve BEFORE UPDATE ON cash_vault_counts
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id OR NEW.vault_id IS NOT OLD.vault_id OR NEW.creator_id IS NOT OLD.creator_id OR NEW.counter_id IS NOT OLD.counter_id OR NEW.reason IS NOT OLD.reason OR NEW.expected_cents IS NOT OLD.expected_cents OR NEW.vault_operation_id IS NOT OLD.vault_operation_id OR NEW.created_at IS NOT OLD.created_at
 OR NOT ((OLD.state='counting' AND NEW.state IN ('submitted','cancelled')) OR (OLD.state='submitted' AND NEW.state IN ('accepted','rejected')))
 OR (OLD.state='submitted' AND (NEW.counted_cents IS NOT OLD.counted_cents OR NEW.difference_cents IS NOT OLD.difference_cents))
BEGIN SELECT RAISE(ABORT,'invalid count mutation'); END;
CREATE TRIGGER cash_vault_count_ops_no_update BEFORE UPDATE ON cash_vault_count_operations BEGIN SELECT RAISE(ABORT,'count operations immutable'); END;
CREATE TRIGGER cash_vault_count_ops_no_delete BEFORE DELETE ON cash_vault_count_operations BEGIN SELECT RAISE(ABORT,'count operations immutable'); END;
CREATE TRIGGER cash_vault_count_events_no_update BEFORE UPDATE ON cash_vault_count_events BEGIN SELECT RAISE(ABORT,'count events immutable'); END;
CREATE TRIGGER cash_vault_count_events_no_delete BEFORE DELETE ON cash_vault_count_events BEGIN SELECT RAISE(ABORT,'count events immutable'); END;
