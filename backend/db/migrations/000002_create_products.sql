BEGIN;

CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    nome VARCHAR(255) NOT NULL,
    descricao TEXT NOT NULL DEFAULT '',
    preco NUMERIC(12, 2) NOT NULL DEFAULT 0.00,
    sku VARCHAR(100) NOT NULL,
    estoque INTEGER NOT NULL DEFAULT 0,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,

    estoque_minimo INTEGER NOT NULL DEFAULT 0,
    ponto_reposicao INTEGER NOT NULL DEFAULT 0,
    demanda_media_diaria NUMERIC(12, 4) NOT NULL DEFAULT 0.00,
    variabilidade_demanda NUMERIC(12, 4) NOT NULL DEFAULT 0.00,
    tempo_reposicao_dias INTEGER NOT NULL DEFAULT 0,
    ultima_previsao_demanda_em TIMESTAMP WITH TIME ZONE,
    alerta_reposicao_ativo BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT products_tenant_id_fkey
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,

    CONSTRAINT products_preco_nonnegative
        CHECK (preco >= 0),

    CONSTRAINT products_estoque_nonnegative
        CHECK (estoque >= 0),

    CONSTRAINT products_estoque_minimo_nonnegative
        CHECK (estoque_minimo >= 0),

    CONSTRAINT products_ponto_reposicao_nonnegative
        CHECK (ponto_reposicao >= 0),

    CONSTRAINT products_tenant_sku_key
        UNIQUE (tenant_id, sku)
);

CREATE INDEX idx_products_tenant_id
    ON products(tenant_id);

COMMIT;
