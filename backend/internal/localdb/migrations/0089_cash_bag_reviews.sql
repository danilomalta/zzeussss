-- Append-only investigation ledger. Never changes cash, vault or original bag counts.
CREATE TABLE cash_bag_reviews (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, bag_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>0), operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('investigate','recount','resolve')),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 500),
 counted_cents INTEGER NOT NULL CHECK(counted_cents BETWEEN 0 AND 9007199254740991),
 delta_cents INTEGER NOT NULL CHECK(delta_cents BETWEEN -9007199254740991 AND 9007199254740991),
 request_hash TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,device_id,bag_id,revision),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,bag_id) REFERENCES cash_bags(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE cash_bag_review_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL, bag_id TEXT NOT NULL,
 revision INTEGER NOT NULL, payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
 PRIMARY KEY(tenant_id,store_id,device_id,bag_id,revision),
 FOREIGN KEY(tenant_id,store_id,device_id,bag_id,revision) REFERENCES cash_bag_reviews(tenant_id,store_id,device_id,bag_id,revision)
) STRICT;
CREATE TRIGGER cash_bag_reviews_no_update BEFORE UPDATE ON cash_bag_reviews BEGIN SELECT RAISE(ABORT,'custody review immutable'); END;
CREATE TRIGGER cash_bag_reviews_no_delete BEFORE DELETE ON cash_bag_reviews BEGIN SELECT RAISE(ABORT,'custody review immutable'); END;
CREATE TRIGGER cash_bag_review_events_no_update BEFORE UPDATE ON cash_bag_review_events BEGIN SELECT RAISE(ABORT,'custody review event immutable'); END;
CREATE TRIGGER cash_bag_review_events_no_delete BEFORE DELETE ON cash_bag_review_events BEGIN SELECT RAISE(ABORT,'custody review event immutable'); END;
