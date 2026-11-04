CREATE TABLE retail_point_awards(tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,sale_id TEXT NOT NULL,customer_id TEXT NOT NULL,points INTEGER NOT NULL CHECK(points>0),total_cents INTEGER NOT NULL,earn_cents INTEGER NOT NULL,earn_points INTEGER NOT NULL,expires_unix INTEGER NOT NULL,PRIMARY KEY(tenant_id,sale_id)) STRICT;
CREATE TABLE retail_points(tenant_id TEXT NOT NULL,customer_id TEXT NOT NULL,sale_id TEXT NOT NULL,entry_id TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN('earn','redeem','reverse')),delta INTEGER NOT NULL CHECK(delta<>0),expires_unix INTEGER NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(tenant_id,entry_id,sale_id)) STRICT;
CREATE TABLE retail_point_redemptions(tenant_id TEXT NOT NULL,customer_id TEXT NOT NULL,operation_id TEXT NOT NULL,points INTEGER NOT NULL CHECK(points>0),benefit_cents INTEGER NOT NULL CHECK(benefit_cents>0),PRIMARY KEY(tenant_id,operation_id)) STRICT;
CREATE TRIGGER retail_points_no_update BEFORE UPDATE ON retail_points BEGIN SELECT RAISE(ABORT,'immutable points ledger'); END;
CREATE TRIGGER retail_points_no_delete BEFORE DELETE ON retail_points BEGIN SELECT RAISE(ABORT,'immutable points ledger'); END;
CREATE TRIGGER retail_awards_no_update BEFORE UPDATE ON retail_point_awards BEGIN SELECT RAISE(ABORT,'immutable award source'); END;
CREATE TRIGGER retail_awards_no_delete BEFORE DELETE ON retail_point_awards BEGIN SELECT RAISE(ABORT,'immutable award source'); END;
CREATE TRIGGER retail_redemptions_no_update BEFORE UPDATE ON retail_point_redemptions BEGIN SELECT RAISE(ABORT,'immutable redemption'); END;
CREATE TRIGGER retail_redemptions_no_delete BEFORE DELETE ON retail_point_redemptions BEGIN SELECT RAISE(ABORT,'immutable redemption'); END;
