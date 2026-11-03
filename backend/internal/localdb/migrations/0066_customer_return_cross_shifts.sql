-- Add provenance for a cash refund paid in a later explicitly chosen turn.
-- Existing sales, closures, journal rows and SQL1-65 are not rewritten.
CREATE TABLE customer_return_cross_shifts (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 return_id TEXT NOT NULL, original_cash_session_id TEXT NOT NULL,
 refund_cash_session_id TEXT NOT NULL, actor_identity_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,return_id),
 CHECK(original_cash_session_id<>refund_cash_session_id),
 FOREIGN KEY(tenant_id,return_id) REFERENCES customer_returns(tenant_id,return_id),
 FOREIGN KEY(tenant_id,store_id,device_id,original_cash_session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,refund_cash_session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_identity_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX customer_return_cross_shift_drawer ON customer_return_cross_shifts(tenant_id,store_id,device_id,refund_cash_session_id);
CREATE TRIGGER customer_return_cross_shifts_no_update BEFORE UPDATE ON customer_return_cross_shifts BEGIN SELECT RAISE(ABORT,'refund drawer provenance is immutable'); END;
CREATE TRIGGER customer_return_cross_shifts_no_delete BEFORE DELETE ON customer_return_cross_shifts BEGIN SELECT RAISE(ABORT,'refund drawer provenance is immutable'); END;
CREATE TRIGGER customer_return_cross_shifts_consistent BEFORE INSERT ON customer_return_cross_shifts
WHEN NOT EXISTS(
 SELECT 1 FROM customer_returns r
 JOIN sales s ON s.tenant_id=r.tenant_id AND s.id=r.sale_id AND s.store_id=r.store_id AND s.device_id=r.device_id
 JOIN cash_sessions old ON old.tenant_id=s.tenant_id AND old.store_id=s.store_id AND old.device_id=s.device_id AND old.id=s.cash_session_id
 JOIN cash_sessions dest ON dest.tenant_id=r.tenant_id AND dest.store_id=r.store_id AND dest.device_id=r.device_id AND dest.id=r.cash_session_id
 JOIN cash_session_operators op ON op.tenant_id=dest.tenant_id AND op.store_id=dest.store_id AND op.session_id=dest.id
 WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id AND r.device_id=NEW.device_id AND r.return_id=NEW.return_id
 AND s.cash_session_id=NEW.original_cash_session_id AND r.cash_session_id=NEW.refund_cash_session_id
 AND r.actor_identity_id=NEW.actor_identity_id AND r.created_at=NEW.created_at
 AND old.closed_at IS NOT NULL AND dest.closed_at IS NULL AND dest.operator_id=NEW.actor_identity_id AND op.identity_id=NEW.actor_identity_id
 AND json_extract(r.input_json,'$.refund_cash_session_id')=NEW.refund_cash_session_id
)
BEGIN SELECT RAISE(ABORT,'refund drawer provenance does not match'); END;
