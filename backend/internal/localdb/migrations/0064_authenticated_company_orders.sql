-- Isolated cross-company trust. Never grants same-company device pairing.
CREATE TABLE company_trade_peers (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 remote_tenant TEXT NOT NULL, remote_store TEXT NOT NULL, remote_device TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN('active','revoked')),
 revision INTEGER NOT NULL CHECK(revision IN(1,2)),
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,remote_tenant,remote_store,remote_device),
 CHECK(tenant_id!=remote_tenant),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TABLE company_trade_messages (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 peer_id TEXT NOT NULL, direction TEXT NOT NULL CHECK(direction IN('sent','received')),
 kind TEXT NOT NULL CHECK(kind IN('order','decision')), order_id TEXT NOT NULL,
 offer_sha256 TEXT NOT NULL, envelope_json TEXT NOT NULL CHECK(json_valid(envelope_json)),
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,peer_id) REFERENCES company_trade_peers(tenant_id,store_id,id)
) STRICT;
CREATE UNIQUE INDEX company_trade_export_once ON company_trade_messages(tenant_id,store_id,order_id)
 WHERE direction='sent' AND kind='order';
CREATE UNIQUE INDEX company_trade_decision_once ON company_trade_messages(tenant_id,store_id,direction,offer_sha256)
 WHERE kind='decision';
CREATE TABLE company_trade_applied (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 message_id TEXT NOT NULL, envelope_sha256 TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision=1),
 PRIMARY KEY(tenant_id,store_id,order_id), UNIQUE(tenant_id,store_id,message_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,message_id) REFERENCES company_trade_messages(tenant_id,store_id,id)
) STRICT;
CREATE TABLE company_trade_operations (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, operation_id TEXT NOT NULL,
 actor_id TEXT NOT NULL, request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TRIGGER company_trade_peer_no_delete BEFORE DELETE ON company_trade_peers BEGIN SELECT RAISE(ABORT,'trade trust history immutable'); END;
CREATE TRIGGER company_trade_peer_revision BEFORE UPDATE ON company_trade_peers
 WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.id IS NOT OLD.id
 OR NEW.remote_tenant IS NOT OLD.remote_tenant OR NEW.remote_store IS NOT OLD.remote_store OR NEW.remote_device IS NOT OLD.remote_device
 OR OLD.status!='active' OR NEW.status!='revoked' OR NEW.revision!=OLD.revision+1
 OR json_set(NEW.payload_json,'$.status','active','$.revision',1)!=OLD.payload_json
 BEGIN SELECT RAISE(ABORT,'only durable trade revocation allowed'); END;
CREATE TRIGGER company_trade_messages_no_update BEFORE UPDATE ON company_trade_messages BEGIN SELECT RAISE(ABORT,'trade message immutable'); END;
CREATE TRIGGER company_trade_messages_no_delete BEFORE DELETE ON company_trade_messages BEGIN SELECT RAISE(ABORT,'trade message immutable'); END;
CREATE TRIGGER company_trade_applied_no_update BEFORE UPDATE ON company_trade_applied BEGIN SELECT RAISE(ABORT,'trade decision immutable'); END;
CREATE TRIGGER company_trade_applied_no_delete BEFORE DELETE ON company_trade_applied BEGIN SELECT RAISE(ABORT,'trade decision immutable'); END;
CREATE TRIGGER company_trade_operations_no_update BEFORE UPDATE ON company_trade_operations BEGIN SELECT RAISE(ABORT,'trade receipt immutable'); END;
CREATE TRIGGER company_trade_operations_no_delete BEFORE DELETE ON company_trade_operations BEGIN SELECT RAISE(ABORT,'trade receipt immutable'); END;

-- A restored archive may predate revocations. Block new exchanges, not history.
CREATE TABLE company_trade_recovery (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, generation TEXT NOT NULL,
 required INTEGER NOT NULL CHECK(required IN(0,1)), approved_by TEXT NOT NULL,
 verification_reference TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TRIGGER company_trade_recovery_no_delete BEFORE DELETE ON company_trade_recovery BEGIN SELECT RAISE(ABORT,'trade recovery gate cannot be removed'); END;
CREATE TRIGGER company_trade_recovery_transition BEFORE UPDATE ON company_trade_recovery
 WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id
 OR NOT ((NEW.generation!=OLD.generation AND NEW.required=1 AND NEW.approved_by='' AND NEW.verification_reference='')
 OR (NEW.generation=OLD.generation AND OLD.required=1 AND NEW.required=0 AND length(NEW.approved_by)>0 AND length(NEW.verification_reference)>0))
 BEGIN SELECT RAISE(ABORT,'invalid trade recovery transition'); END;
