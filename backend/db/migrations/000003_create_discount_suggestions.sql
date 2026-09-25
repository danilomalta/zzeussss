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