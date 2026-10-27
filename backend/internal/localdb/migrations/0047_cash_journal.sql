-- Journal operacional do dinheiro. Contrapartidas de controle não são um
-- plano de contas fiscal nem comprovam liquidação bancária.
CREATE VIEW cash_journal_sources AS
WITH movements AS (
 SELECT m.tenant_id,m.store_id,c.device_id,m.cash_session_id AS session_id,
        'movement:'||m.id AS entry_id,'movement' AS source_kind,m.id AS source_id,
        m.id AS movement_id,m.amount_cents AS cash_delta_cents,m.created_at AS occurred_at,
        CASE
         WHEN ((a.movement_id IS NOT NULL)+(r.cash_movement_id IS NOT NULL)+(o.sale_id IS NOT NULL))<>1 THEN 'unclassified'
         WHEN a.kind='supply' AND m.amount_cents=a.amount_cents THEN 'supply'
         WHEN a.kind='withdrawal' AND m.amount_cents=-a.amount_cents THEN 'withdrawal'
         WHEN r.sale_id IS NOT NULL AND m.amount_cents=-r.refunded_cents THEN 'sale_reversal'
         WHEN o.sale_id IS NOT NULL AND m.amount_cents=s.total_cents THEN 'sale'
         ELSE 'unclassified' END AS category,
        a.operation_id AS adjustment_operation,a.actor_identity_id AS adjustment_actor,
        r.operation_id AS reversal_operation,r.actor_identity_id AS reversal_actor,
        o.operation_id AS sale_operation,o.actor_identity_id AS sale_actor
 FROM cash_movements m
 JOIN cash_sessions c ON c.tenant_id=m.tenant_id AND c.store_id=m.store_id AND c.id=m.cash_session_id
 LEFT JOIN cash_adjustments a ON a.tenant_id=m.tenant_id AND a.store_id=m.store_id AND a.device_id=c.device_id AND a.session_id=m.cash_session_id AND a.movement_id=m.id
 LEFT JOIN sale_cancellations r ON r.tenant_id=m.tenant_id AND r.store_id=m.store_id AND r.device_id=c.device_id AND r.cash_movement_id=m.id
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

CREATE TABLE cash_journal_entries (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 device_id TEXT NOT NULL,
 session_id TEXT NOT NULL,
 entry_id TEXT NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN ('opening','movement')),
 source_id TEXT NOT NULL,
 movement_id TEXT,
 cash_delta_cents INTEGER NOT NULL,
 occurred_at TEXT NOT NULL,
 category TEXT NOT NULL CHECK(category IN ('opening','sale','sale_reversal','supply','withdrawal','unclassified')),
 operation_id TEXT,
 actor_identity_id TEXT,
 counter_account TEXT NOT NULL,
 UNIQUE(tenant_id,entry_id),
 UNIQUE(tenant_id,source_kind,source_id),
 CHECK((source_kind='opening' AND entry_id='opening:'||source_id AND source_id=session_id AND movement_id IS NULL AND category='opening' AND cash_delta_cents>=0)
    OR (source_kind='movement' AND entry_id='movement:'||source_id AND movement_id=source_id AND cash_delta_cents<>0 AND category<>'opening')),
 CHECK(counter_account=CASE category WHEN 'opening' THEN 'opening_control' WHEN 'sale' THEN 'cash_sales_control' WHEN 'sale_reversal' THEN 'cash_sales_control' WHEN 'supply' THEN 'supply_control' WHEN 'withdrawal' THEN 'withdrawal_control' ELSE 'unclassified_control' END),
 FOREIGN KEY(tenant_id,store_id,device_id,session_id) REFERENCES cash_sessions(tenant_id,store_id,device_id,id),
 FOREIGN KEY(tenant_id,store_id,session_id,movement_id) REFERENCES cash_movements(tenant_id,store_id,cash_session_id,id),
 FOREIGN KEY(tenant_id,actor_identity_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;

INSERT INTO cash_journal_entries(tenant_id,store_id,device_id,session_id,entry_id,
 source_kind,source_id,movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account)
SELECT tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,
 movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account
FROM cash_journal_sources ORDER BY occurred_at,tenant_id,entry_id;

CREATE INDEX cash_journal_session ON cash_journal_entries(tenant_id,store_id,device_id,session_id,sequence);
CREATE TRIGGER cash_journal_no_update BEFORE UPDATE ON cash_journal_entries
BEGIN SELECT RAISE(ABORT,'cash journal is append only'); END;
CREATE TRIGGER cash_journal_no_delete BEFORE DELETE ON cash_journal_entries
BEGIN SELECT RAISE(ABORT,'cash journal is append only'); END;
