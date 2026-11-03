-- Physical customer returns and cash refunds; original sales stay immutable.
CREATE TABLE customer_returns (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 return_id TEXT NOT NULL, operation_id TEXT NOT NULL, sale_id TEXT NOT NULL,
 cash_session_id TEXT NOT NULL, actor_identity_id TEXT NOT NULL,
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 500),
 refund_cents INTEGER NOT NULL CHECK(refund_cents>0 AND refund_cents<=9007199254740991),
 cash_movement_id TEXT NOT NULL UNIQUE, request_hash TEXT NOT NULL,
 input_json TEXT NOT NULL CHECK(json_valid(input_json)), created_at TEXT NOT NULL,
 UNIQUE(tenant_id,return_id), UNIQUE(tenant_id,return_id,sale_id),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id,sale_id) REFERENCES sales(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id,cash_session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,actor_identity_id) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,cash_session_id,cash_movement_id) REFERENCES cash_movements(tenant_id,store_id,cash_session_id,id)
) STRICT;
CREATE TABLE customer_return_items (
 tenant_id TEXT NOT NULL, return_id TEXT NOT NULL, sale_id TEXT NOT NULL,
 sale_item_id TEXT NOT NULL, product_id TEXT NOT NULL, location_id TEXT NOT NULL,
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli>0 AND quantity_milli<=9007199254740991),
 refund_cents INTEGER NOT NULL CHECK(refund_cents>=0 AND refund_cents<=9007199254740991),
 disposition TEXT NOT NULL CHECK(disposition IN ('restock','no_restock')),
 original_movement_id TEXT NOT NULL, return_movement_id TEXT UNIQUE,
 PRIMARY KEY(tenant_id,return_id,sale_item_id),
 CHECK((disposition='restock' AND return_movement_id IS NOT NULL) OR (disposition='no_restock' AND return_movement_id IS NULL)),
 FOREIGN KEY(tenant_id,return_id,sale_id) REFERENCES customer_returns(tenant_id,return_id,sale_id),
 FOREIGN KEY(tenant_id,sale_id,sale_item_id) REFERENCES sale_items(tenant_id,sale_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(original_movement_id) REFERENCES stock_movements(id),
 FOREIGN KEY(return_movement_id) REFERENCES stock_movements(id)
) STRICT;
CREATE INDEX customer_returns_sale ON customer_returns(tenant_id,store_id,device_id,sale_id,sequence);
CREATE TRIGGER customer_returns_no_update BEFORE UPDATE ON customer_returns BEGIN SELECT RAISE(ABORT,'customer return is immutable'); END;
CREATE TRIGGER customer_returns_no_delete BEFORE DELETE ON customer_returns BEGIN SELECT RAISE(ABORT,'customer return is immutable'); END;
CREATE TRIGGER customer_return_items_no_update BEFORE UPDATE ON customer_return_items BEGIN SELECT RAISE(ABORT,'customer return item is immutable'); END;
CREATE TRIGGER customer_return_items_no_delete BEFORE DELETE ON customer_return_items BEGIN SELECT RAISE(ABORT,'customer return item is immutable'); END;
-- Prevent whole-sale cancellation from refunding quantities already returned.
CREATE TRIGGER customer_returns_no_double_cancel BEFORE INSERT ON sale_cancellations
WHEN EXISTS(SELECT 1 FROM customer_returns r WHERE r.tenant_id=NEW.tenant_id AND r.sale_id=NEW.sale_id)
BEGIN SELECT RAISE(ABORT,'sale already has customer returns'); END;
CREATE TRIGGER customer_returns_no_cancelled_sale BEFORE INSERT ON customer_returns
WHEN NOT EXISTS(SELECT 1 FROM sales s WHERE s.tenant_id=NEW.tenant_id AND s.id=NEW.sale_id AND s.status='committed')
BEGIN SELECT RAISE(ABORT,'sale is not committed'); END;

-- Extend the derived source view; no old journal row or SQL is rewritten.
DROP VIEW cash_journal_sources;
CREATE VIEW cash_journal_sources AS
WITH movements AS (
 SELECT m.tenant_id,m.store_id,c.device_id,m.cash_session_id AS session_id,
        'movement:'||m.id AS entry_id,'movement' AS source_kind,m.id AS source_id,
        m.id AS movement_id,m.amount_cents AS cash_delta_cents,m.created_at AS occurred_at,
        CASE
         WHEN ((a.movement_id IS NOT NULL)+(r.cash_movement_id IS NOT NULL)+(o.sale_id IS NOT NULL)+(u.return_id IS NOT NULL))<>1 THEN 'unclassified'
         WHEN a.kind='supply' AND m.amount_cents=a.amount_cents THEN 'supply'
         WHEN a.kind='withdrawal' AND m.amount_cents=-a.amount_cents THEN 'withdrawal'
         WHEN r.sale_id IS NOT NULL AND m.amount_cents=-r.refunded_cents THEN 'sale_reversal'
         WHEN u.return_id IS NOT NULL AND m.amount_cents=-u.refund_cents THEN 'sale_reversal'
         WHEN o.sale_id IS NOT NULL AND m.amount_cents=s.total_cents THEN 'sale'
         ELSE 'unclassified' END AS category,
        a.operation_id AS adjustment_operation,a.actor_identity_id AS adjustment_actor,
        COALESCE(r.operation_id,u.operation_id) AS reversal_operation,COALESCE(r.actor_identity_id,u.actor_identity_id) AS reversal_actor,
        o.operation_id AS sale_operation,o.actor_identity_id AS sale_actor
 FROM cash_movements m
 JOIN cash_sessions c ON c.tenant_id=m.tenant_id AND c.store_id=m.store_id AND c.id=m.cash_session_id
 LEFT JOIN cash_adjustments a ON a.tenant_id=m.tenant_id AND a.store_id=m.store_id AND a.device_id=c.device_id AND a.session_id=m.cash_session_id AND a.movement_id=m.id
 LEFT JOIN sale_cancellations r ON r.tenant_id=m.tenant_id AND r.store_id=m.store_id AND r.device_id=c.device_id AND r.cash_movement_id=m.id
 LEFT JOIN customer_returns u ON u.tenant_id=m.tenant_id AND u.store_id=m.store_id AND u.device_id=c.device_id AND u.cash_session_id=c.id AND u.cash_movement_id=m.id
 LEFT JOIN sale_operations o ON o.tenant_id=m.tenant_id AND o.store_id=m.store_id AND o.device_id=c.device_id AND m.reason='venda:'||o.sale_id
 LEFT JOIN sales s ON s.tenant_id=o.tenant_id AND s.id=o.sale_id AND s.cash_session_id=c.id AND s.store_id=c.store_id AND s.device_id=c.device_id
), sources AS (
 SELECT c.tenant_id,c.store_id,c.device_id,c.id AS session_id,
        'opening:'||c.id AS entry_id,'opening' AS source_kind,c.id AS source_id,
        NULL AS movement_id,c.opening_cents AS cash_delta_cents,c.opened_at AS occurred_at,
        'opening' AS category,
        CASE WHEN b.identity_id IS NOT NULL THEN c.id ELSE NULL END AS operation_id,
        b.identity_id AS actor_identity_id
 FROM cash_sessions c LEFT JOIN cash_session_operators b
 ON b.tenant_id=c.tenant_id AND b.store_id=c.store_id AND b.session_id=c.id
 UNION ALL
 SELECT tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,
        movement_id,cash_delta_cents,occurred_at,category,
        CASE category WHEN 'supply' THEN adjustment_operation WHEN 'withdrawal' THEN adjustment_operation WHEN 'sale' THEN sale_operation WHEN 'sale_reversal' THEN reversal_operation ELSE NULL END,
        CASE category WHEN 'supply' THEN adjustment_actor WHEN 'withdrawal' THEN adjustment_actor WHEN 'sale' THEN sale_actor WHEN 'sale_reversal' THEN reversal_actor ELSE NULL END
 FROM movements
)
SELECT *,CASE category
 WHEN 'opening' THEN 'opening_control' WHEN 'sale' THEN 'cash_sales_control'
 WHEN 'sale_reversal' THEN 'cash_sales_control' WHEN 'supply' THEN 'supply_control'
 WHEN 'withdrawal' THEN 'withdrawal_control' ELSE 'unclassified_control' END AS counter_account
FROM sources;
