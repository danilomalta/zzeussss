-- Complementa produtos locais sem recriar a tabela nem perder cadastros.
ALTER TABLE products ADD COLUMN unit TEXT NOT NULL DEFAULT 'unit'
    CHECK (unit IN ('unit', 'kg', 'g', 'liter', 'ml', 'meter'));

ALTER TABLE products ADD COLUMN barcode TEXT;

CREATE UNIQUE INDEX idx_products_tenant_barcode ON products(tenant_id, barcode)
    WHERE barcode IS NOT NULL;
