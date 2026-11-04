-- Existing commercial receivable, linked to a verified customer order.
-- Monetary terms are explicitly recorded by the operator, not inferred from trade v1.
CREATE TABLE receivable_customer_orders (
 tenant_id TEXT NOT NULL,store_id TEXT NOT NULL,document_id TEXT NOT NULL,
 message_id TEXT NOT NULL,decision_id TEXT NOT NULL,terms_reference TEXT NOT NULL,
 customer_tenant_id TEXT NOT NULL,customer_store_id TEXT NOT NULL,order_id TEXT NOT NULL,
 source_json TEXT NOT NULL CHECK(json_valid(source_json)),active INTEGER NOT NULL CHECK(active IN(0,1)),
 PRIMARY KEY(tenant_id,store_id,document_id),
 FOREIGN KEY(tenant_id,store_id,document_id) REFERENCES receivable_documents(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,message_id) REFERENCES company_trade_messages(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,decision_id) REFERENCES company_trade_messages(tenant_id,store_id,id)
) STRICT;
CREATE UNIQUE INDEX receivable_order_once ON receivable_customer_orders(tenant_id,store_id,customer_tenant_id,customer_store_id,order_id) WHERE active=1;
CREATE TRIGGER immutable_receivable_order_update BEFORE UPDATE ON receivable_customer_orders
WHEN NEW.tenant_id IS NOT OLD.tenant_id OR NEW.store_id IS NOT OLD.store_id OR NEW.document_id IS NOT OLD.document_id OR NEW.message_id IS NOT OLD.message_id OR NEW.decision_id IS NOT OLD.decision_id OR NEW.terms_reference IS NOT OLD.terms_reference OR NEW.customer_tenant_id IS NOT OLD.customer_tenant_id OR NEW.customer_store_id IS NOT OLD.customer_store_id OR NEW.order_id IS NOT OLD.order_id OR NEW.source_json IS NOT OLD.source_json OR OLD.active!=1 OR NEW.active!=0 OR NOT EXISTS(SELECT 1 FROM receivable_documents d WHERE d.tenant_id=OLD.tenant_id AND d.store_id=OLD.store_id AND d.id=OLD.document_id AND d.state='cancelled')
BEGIN SELECT RAISE(ABORT,'immutable billing origin');END;
CREATE TRIGGER no_receivable_order_delete BEFORE DELETE ON receivable_customer_orders BEGIN SELECT RAISE(ABORT,'immutable billing origin');END;
