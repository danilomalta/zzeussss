-- Local physical cash, not bank accounts. No opening money is fabricated and
-- previously received bags are NOT automatically assigned to a destination.
CREATE TABLE cash_vaults (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,
 name TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('active','archived')),
 balance_cents INTEGER NOT NULL CHECK(balance_cents BETWEEN 0 AND 9007199254740991),
 created_by TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
 creation_operation_id TEXT NOT NULL,last_operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 CHECK(state<>'archived' OR balance_cents=0),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,device_id,creation_operation_id) REFERENCES cash_vault_operations(tenant_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(tenant_id,device_id,last_operation_id) REFERENCES cash_vault_operations(tenant_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE cash_vault_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN ('create','deposit','transfer','supply','archive')),
 reason TEXT NOT NULL,request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE cash_vault_entries (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,vault_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('deposit','transfer_out','transfer_in','cash_supply')),
 delta_cents INTEGER NOT NULL CHECK(delta_cents BETWEEN -9007199254740991 AND 9007199254740991),
 bag_id TEXT,cash_session_id TEXT,cash_movement_id TEXT,
 created_at TEXT NOT NULL,
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 UNIQUE(tenant_id,device_id,operation_id,vault_id,kind),
 CHECK((kind='deposit' AND delta_cents>=0 AND bag_id IS NOT NULL AND cash_session_id IS NULL AND cash_movement_id IS NULL)
 OR (kind IN ('transfer_out','transfer_in') AND bag_id IS NULL AND cash_session_id IS NULL AND cash_movement_id IS NULL AND ((kind='transfer_in' AND delta_cents>0) OR (kind='transfer_out' AND delta_cents<0)))
 OR (kind='cash_supply' AND delta_cents<0 AND bag_id IS NULL AND cash_session_id IS NOT NULL AND cash_movement_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id,device_id,vault_id) REFERENCES cash_vaults(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES cash_vault_operations(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,bag_id) REFERENCES cash_bags(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,cash_session_id,cash_movement_id) REFERENCES cash_movements(tenant_id,store_id,cash_session_id,id)
) STRICT;
CREATE INDEX cash_vault_statement ON cash_vault_entries(tenant_id,store_id,device_id,vault_id,sequence);
CREATE UNIQUE INDEX cash_vault_supply_once ON cash_vault_entries(tenant_id,device_id,cash_movement_id) WHERE cash_movement_id IS NOT NULL;
CREATE TABLE cash_vault_deposits (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,bag_id TEXT NOT NULL,vault_id TEXT NOT NULL,entry_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,bag_id),
 FOREIGN KEY(tenant_id,store_id,device_id,bag_id) REFERENCES cash_bags(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,vault_id) REFERENCES cash_vaults(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,entry_id) REFERENCES cash_vault_entries(tenant_id,store_id,device_id,id)
) STRICT;
-- Dedicated immutable local events: the legacy wire protocol has no vault contract.
CREATE TABLE cash_vault_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,schema_version INTEGER NOT NULL CHECK(schema_version=1),
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES cash_vault_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER cash_vault_no_delete BEFORE DELETE ON cash_vaults BEGIN SELECT RAISE(ABORT,'vault history cannot be erased'); END;
CREATE TRIGGER cash_vault_preserve BEFORE UPDATE ON cash_vaults
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id OR NEW.name IS NOT OLD.name OR NEW.created_by IS NOT OLD.created_by OR NEW.created_at IS NOT OLD.created_at OR NEW.creation_operation_id IS NOT OLD.creation_operation_id OR OLD.state='archived'
BEGIN SELECT RAISE(ABORT,'vault metadata is immutable'); END;
CREATE TRIGGER cash_vault_ops_no_update BEFORE UPDATE ON cash_vault_operations BEGIN SELECT RAISE(ABORT,'vault operations immutable'); END;
CREATE TRIGGER cash_vault_ops_no_delete BEFORE DELETE ON cash_vault_operations BEGIN SELECT RAISE(ABORT,'vault operations immutable'); END;
CREATE TRIGGER cash_vault_entries_no_update BEFORE UPDATE ON cash_vault_entries BEGIN SELECT RAISE(ABORT,'vault entries immutable'); END;
CREATE TRIGGER cash_vault_entries_no_delete BEFORE DELETE ON cash_vault_entries BEGIN SELECT RAISE(ABORT,'vault entries immutable'); END;
CREATE TRIGGER cash_vault_deposit_no_update BEFORE UPDATE ON cash_vault_deposits BEGIN SELECT RAISE(ABORT,'bag credit immutable'); END;
CREATE TRIGGER cash_vault_deposit_no_delete BEFORE DELETE ON cash_vault_deposits BEGIN SELECT RAISE(ABORT,'bag credit immutable'); END;
CREATE TRIGGER cash_vault_events_no_update BEFORE UPDATE ON cash_vault_events BEGIN SELECT RAISE(ABORT,'vault events immutable'); END;
CREATE TRIGGER cash_vault_events_no_delete BEFORE DELETE ON cash_vault_events BEGIN SELECT RAISE(ABORT,'vault events immutable'); END;
