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