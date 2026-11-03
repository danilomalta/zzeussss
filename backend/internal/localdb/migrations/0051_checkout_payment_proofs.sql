-- Manual documentary evidence is never a provider payment confirmation.
CREATE TABLE checkout_payment_proofs (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,id TEXT NOT NULL,
 checkout_id TEXT NOT NULL,part_id TEXT NOT NULL,submitter_id TEXT NOT NULL,
 method TEXT NOT NULL CHECK(method IN ('debit','credit','pix')),
 amount_cents INTEGER NOT NULL CHECK(amount_cents BETWEEN 1 AND 9007199254740991),
 provider TEXT NOT NULL COLLATE NOCASE,terminal TEXT NOT NULL COLLATE NOCASE,reference TEXT NOT NULL,occurred_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('submitted','accepted','rejected','withdrawn','revoked')),
 reviewer_id TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,last_operation_id TEXT NOT NULL,
 PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,store_id,device_id,id),
 UNIQUE(tenant_id,store_id,provider,terminal,reference),
 CHECK(reviewer_id IS NULL OR reviewer_id<>submitter_id),
 CHECK((state IN ('submitted','withdrawn') AND reviewer_id IS NULL) OR (state IN ('accepted','rejected','revoked') AND reviewer_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,checkout_id,part_id) REFERENCES checkout_plan_parts(tenant_id,checkout_id,part_id),
 FOREIGN KEY(tenant_id,submitter_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,reviewer_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,device_id,last_operation_id) REFERENCES checkout_proof_operations(tenant_id,store_id,device_id,operation_id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE UNIQUE INDEX checkout_one_active_proof ON checkout_payment_proofs(tenant_id,checkout_id,part_id) WHERE state IN ('submitted','accepted');
CREATE INDEX checkout_proof_queue ON checkout_payment_proofs(tenant_id,store_id,device_id,state,created_at,id);
CREATE TABLE checkout_proof_operations (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 proof_id TEXT NOT NULL,actor_id TEXT NOT NULL,action TEXT NOT NULL,from_state TEXT NOT NULL,to_state TEXT NOT NULL,reason TEXT NOT NULL,
 request_hash TEXT NOT NULL,result_json TEXT NOT NULL CHECK(json_valid(result_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),UNIQUE(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,proof_id) REFERENCES checkout_payment_proofs(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE checkout_proof_events (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,device_id TEXT NOT NULL,operation_id TEXT NOT NULL,
 event_type TEXT NOT NULL,schema_version INTEGER NOT NULL CHECK(schema_version=1),payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,operation_id) REFERENCES checkout_proof_operations(tenant_id,store_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER checkout_proof_no_delete BEFORE DELETE ON checkout_payment_proofs BEGIN SELECT RAISE(ABORT,'proof history immutable'); END;
CREATE TRIGGER checkout_proof_preserve BEFORE UPDATE ON checkout_payment_proofs
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.device_id IS NOT OLD.device_id OR NEW.id IS NOT OLD.id OR NEW.checkout_id IS NOT OLD.checkout_id OR NEW.part_id IS NOT OLD.part_id OR NEW.submitter_id IS NOT OLD.submitter_id OR NEW.method IS NOT OLD.method OR NEW.amount_cents IS NOT OLD.amount_cents OR NEW.provider IS NOT OLD.provider OR NEW.terminal IS NOT OLD.terminal OR NEW.reference IS NOT OLD.reference OR NEW.occurred_at IS NOT OLD.occurred_at OR NEW.created_at IS NOT OLD.created_at
 OR NOT ((OLD.state='submitted' AND NEW.state IN ('accepted','rejected','withdrawn')) OR (OLD.state='accepted' AND NEW.state='revoked'))
BEGIN SELECT RAISE(ABORT,'invalid proof mutation'); END;
CREATE TRIGGER checkout_proof_ops_no_update BEFORE UPDATE ON checkout_proof_operations BEGIN SELECT RAISE(ABORT,'proof operations immutable'); END;
CREATE TRIGGER checkout_proof_ops_no_delete BEFORE DELETE ON checkout_proof_operations BEGIN SELECT RAISE(ABORT,'proof operations immutable'); END;
CREATE TRIGGER checkout_proof_events_no_update BEFORE UPDATE ON checkout_proof_events BEGIN SELECT RAISE(ABORT,'proof events immutable'); END;
CREATE TRIGGER checkout_proof_events_no_delete BEFORE DELETE ON checkout_proof_events BEGIN SELECT RAISE(ABORT,'proof events immutable'); END;
-- A withdrawn/rejected document does not prove that external funds were refunded.
CREATE TRIGGER checkout_proof_blocks_plan_cancel BEFORE INSERT ON checkout_plan_cancellations
WHEN EXISTS(SELECT 1 FROM checkout_payment_proofs p WHERE p.tenant_id=NEW.tenant_id AND p.checkout_id=NEW.checkout_id)
BEGIN SELECT RAISE(ABORT,'external evidence requires resolution before cancellation'); END;
CREATE TRIGGER checkout_cancel_blocks_new_proof BEFORE INSERT ON checkout_payment_proofs
WHEN EXISTS(SELECT 1 FROM checkout_plan_cancellations c WHERE c.tenant_id=NEW.tenant_id AND c.checkout_id=NEW.checkout_id)
BEGIN SELECT RAISE(ABORT,'cancelled allocation cannot receive evidence'); END;
