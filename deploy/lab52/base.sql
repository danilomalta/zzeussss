-- Fresh isolated lab only; no historical destructive migration is executed.
BEGIN;
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    document VARCHAR(18) NOT NULL UNIQUE,
    business_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE tenant_modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    module_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_modules_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT uq_tenant_module UNIQUE (tenant_id, module_name)
);

-- ── 2. MODULO: AUTH ─────────────────────────────────────────────────────────
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    sync_code VARCHAR(100) NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE CASCADE
);

COMMIT;
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

BEGIN;

-- Permite uma referência composta que garante que o produto
-- e a sugestão pertencem à mesma empresa.
CREATE UNIQUE INDEX products_tenant_id_id_key
    ON public.products (tenant_id, id);

CREATE TABLE public.discount_suggestions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    product_id BIGINT NOT NULL,
    suggested_discount NUMERIC(5, 2) NOT NULL,
    suggested_range TEXT NOT NULL,
    reason TEXT NOT NULL,
    criteria TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    reviewed_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT discount_suggestions_product_tenant_fkey
        FOREIGN KEY (tenant_id, product_id)
        REFERENCES public.products (tenant_id, id),

    CONSTRAINT discount_suggestions_discount_check
        CHECK (suggested_discount >= 0 AND suggested_discount <= 100),

    CONSTRAINT discount_suggestions_status_check
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),

    CONSTRAINT discount_suggestions_reviewer_check
        CHECK (
            (status = 'PENDING' AND reviewed_by = '')
            OR
            (status IN ('APPROVED', 'REJECTED')
             AND length(trim(reviewed_by)) > 0)
        )
);

CREATE INDEX idx_discount_suggestions_tenant_status
    ON public.discount_suggestions (tenant_id, status);

CREATE UNIQUE INDEX uq_discount_suggestions_pending_product
    ON public.discount_suggestions (tenant_id, product_id)
    WHERE status = 'PENDING' AND deleted_at IS NULL;

COMMIT;
BEGIN;

CREATE TABLE public.sped_jobs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    job_id VARCHAR(100) NOT NULL,
    type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    requested_by TEXT NOT NULL,
    file_url TEXT NOT NULL DEFAULT '',
    error_msg TEXT NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT sped_jobs_tenant_id_fkey
        FOREIGN KEY (tenant_id)
        REFERENCES public.tenants(id)
        ON DELETE CASCADE,

    CONSTRAINT sped_jobs_job_id_key
        UNIQUE (job_id),

    CONSTRAINT sped_jobs_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED')),

    CONSTRAINT sped_jobs_dates_check
        CHECK (end_date >= start_date)
);

CREATE INDEX idx_sped_jobs_tenant_status
    ON public.sped_jobs (tenant_id, status);

CREATE INDEX idx_sped_jobs_tenant_job_id
    ON public.sped_jobs (tenant_id, job_id);

CREATE TABLE public.indicacaos (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    id_externo VARCHAR(255) NOT NULL,
    indicador_id BIGINT,
    indicador_perfil VARCHAR(50) NOT NULL,
    email_indicado VARCHAR(255) NOT NULL DEFAULT '',
    telefone_indicado VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'registrada',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT indicacaos_tenant_id_fkey
        FOREIGN KEY (tenant_id)
        REFERENCES public.tenants(id)
        ON DELETE CASCADE,

    CONSTRAINT indicacaos_tenant_id_id_key
        UNIQUE (tenant_id, id),

    CONSTRAINT indicacaos_tenant_id_externo_key
        UNIQUE (tenant_id, id_externo),

    CONSTRAINT indicacaos_contact_check
        CHECK (
            length(trim(email_indicado)) > 0
            OR length(trim(telefone_indicado)) > 0
        )
);

CREATE INDEX idx_indicacaos_tenant_status
    ON public.indicacaos (tenant_id, status);

CREATE TABLE public.recompensa_indicacaos (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    id_externo VARCHAR(255) NOT NULL,
    indicacao_id BIGINT NOT NULL,
    valor_centavos BIGINT NOT NULL,
    moeda VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL,
    motivo VARCHAR(50) NOT NULL,
    concedida_em TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT recompensa_indicacaos_tenant_id_fkey
        FOREIGN KEY (tenant_id)
        REFERENCES public.tenants(id)
        ON DELETE CASCADE,

    CONSTRAINT recompensa_indicacaos_indicacao_fkey
        FOREIGN KEY (tenant_id, indicacao_id)
        REFERENCES public.indicacaos (tenant_id, id)
        ON DELETE CASCADE,

    CONSTRAINT recompensa_indicacaos_tenant_id_externo_key
        UNIQUE (tenant_id, id_externo),

    CONSTRAINT recompensa_indicacaos_valor_check
        CHECK (valor_centavos > 0),

    CONSTRAINT recompensa_indicacaos_status_check
        CHECK (status IN ('pendente', 'aprovada', 'estornada')),

    CONSTRAINT recompensa_indicacaos_motivo_check
        CHECK (motivo IN ('conta_aprovada', 'acao_menor'))
);

CREATE INDEX idx_recompensa_indicacaos_tenant_status
    ON public.recompensa_indicacaos (tenant_id, status);

COMMIT;
