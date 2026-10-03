-- Referencias comerciais locais. Nao sao contas ou vinculos entre empresas.
CREATE UNIQUE INDEX restock_suggestion_store_key ON restock_suggestions(tenant_id,store_id,id);
CREATE UNIQUE INDEX restock_review_store_key ON restock_reviews(tenant_id,store_id,suggestion_id);
CREATE TABLE purchase_suppliers (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, created_by TEXT NOT NULL,
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 255),
 status TEXT NOT NULL CHECK(status IN ('active','inactive')), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE purchase_orders (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, created_by TEXT NOT NULL,
 supplier_id TEXT NOT NULL, supplier_name TEXT NOT NULL, suggestion_id TEXT NOT NULL,
 approved_by TEXT NOT NULL, approved_at TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status='local_not_sent'), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,device_id,operation_id),
 UNIQUE(tenant_id,suggestion_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,approved_by) REFERENCES memberships(tenant_id,identity_id),
 FOREIGN KEY(tenant_id,store_id,supplier_id) REFERENCES purchase_suppliers(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,suggestion_id) REFERENCES restock_suggestions(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,suggestion_id) REFERENCES restock_reviews(tenant_id,store_id,suggestion_id)
) STRICT;
CREATE TABLE purchase_order_items (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, order_id TEXT NOT NULL,
 product_id TEXT NOT NULL, sku TEXT NOT NULL, name TEXT NOT NULL, unit TEXT NOT NULL,
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(tenant_id,store_id,order_id,product_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES purchase_orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
) STRICT;
CREATE TABLE purchase_audit (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('supplier.created','purchase.created')),
 aggregate_id TEXT NOT NULL, actor_id TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id,kind),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX purchase_order_history ON purchase_orders(tenant_id,store_id,created_at,id);
