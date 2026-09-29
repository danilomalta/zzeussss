-- Banco de um dispositivo. IDs são gerados localmente em Go.
-- Todos os vínculos operacionais levam tenant_id e, quando aplicável, store_id.
CREATE TABLE tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    created_at TEXT NOT NULL
) STRICT;

CREATE TABLE stores (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) STRICT;

CREATE TABLE devices (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id)
) STRICT;

CREATE TABLE users (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) STRICT;

CREATE TABLE products (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
    cost_cents INTEGER NOT NULL DEFAULT 0 CHECK (cost_cents >= 0),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, sku),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) STRICT;

CREATE TABLE stock_locations (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('shelf', 'backroom', 'receiving', 'production')),
    name TEXT NOT NULL,
    PRIMARY KEY (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id)
) STRICT;

CREATE TABLE stock_movements (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    location_id TEXT NOT NULL,
    quantity_milli INTEGER NOT NULL CHECK (quantity_milli <> 0),
    reason TEXT NOT NULL CHECK (length(trim(reason)) > 0),
    created_at TEXT NOT NULL,
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores(tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, location_id) REFERENCES stock_locations(tenant_id, store_id, id)
) STRICT;

CREATE INDEX idx_stock_movements_balance ON stock_movements(tenant_id, store_id, product_id, location_id, created_at);

CREATE TABLE cash_sessions (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    id TEXT NOT NULL,
    operator_id TEXT NOT NULL,
    opened_at TEXT NOT NULL,
    closed_at TEXT,
    opening_cents INTEGER NOT NULL CHECK (opening_cents >= 0),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id)
) STRICT;

CREATE TABLE sales (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    id TEXT NOT NULL,
    cash_session_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('committed', 'cancelled')),
    total_cents INTEGER NOT NULL CHECK (total_cents >= 0),
    committed_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, cash_session_id) REFERENCES cash_sessions(tenant_id, store_id, id)
) STRICT;

CREATE TABLE sale_items (
    tenant_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity_milli INTEGER NOT NULL CHECK (quantity_milli > 0),
    unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents >= 0),
    total_cents INTEGER NOT NULL CHECK (total_cents >= 0),
    PRIMARY KEY (tenant_id, sale_id, id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales(tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id)
) STRICT;

CREATE TABLE sale_payments (
    tenant_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    id TEXT NOT NULL,
    method TEXT NOT NULL CHECK (method IN ('cash', 'card', 'pix', 'other')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'reversed')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    PRIMARY KEY (tenant_id, sale_id, id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales(tenant_id, id)
) STRICT;

CREATE TABLE cash_movements (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    cash_session_id TEXT NOT NULL,
    amount_cents INTEGER NOT NULL CHECK (amount_cents <> 0),
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (tenant_id, store_id, cash_session_id) REFERENCES cash_sessions(tenant_id, store_id, id)
) STRICT;

CREATE TABLE outbox (
    event_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'acked')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TEXT NOT NULL,
    acked_at TEXT,
    UNIQUE (tenant_id, device_id, operation_id, event_type),
    FOREIGN KEY (tenant_id, store_id, device_id) REFERENCES devices(tenant_id, store_id, id)
) STRICT;

CREATE INDEX idx_outbox_pending ON outbox(status, created_at);
