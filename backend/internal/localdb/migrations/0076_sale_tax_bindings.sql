-- Opt-in price evidence; old merchandise rows and sales stay unchanged.
CREATE TABLE sale_tax_bindings (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 sale_id TEXT NOT NULL, item_id TEXT NOT NULL, quote_operation_id TEXT NOT NULL,
 binding_json TEXT NOT NULL CHECK(json_valid(binding_json)),
 PRIMARY KEY(tenant_id,sale_id,item_id),
 FOREIGN KEY(tenant_id,sale_id,item_id) REFERENCES sale_items(tenant_id,sale_id,id),
 FOREIGN KEY(tenant_id,device_id,quote_operation_id) REFERENCES fiscal_tax_quotes(tenant_id,device_id,operation_id)
) STRICT;
CREATE TRIGGER sale_tax_bindings_no_update BEFORE UPDATE ON sale_tax_bindings BEGIN SELECT RAISE(ABORT,'sale tax evidence immutable'); END;
CREATE TRIGGER sale_tax_bindings_no_delete BEFORE DELETE ON sale_tax_bindings BEGIN SELECT RAISE(ABORT,'sale tax evidence preserved'); END;
-- Old quotes remain readable; an empty historical unit cannot authorize new
-- tax-priced sales. Capture a fresh quote rather than infer historical units.
ALTER TABLE fiscal_tax_quotes ADD COLUMN catalog_unit TEXT NOT NULL DEFAULT '';
