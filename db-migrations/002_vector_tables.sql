CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS policy_embeddings (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID    NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_filename VARCHAR(255),
    chunk_index     INT     NOT NULL,
    content         TEXT    NOT NULL,
    embedding       vector(3072) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_embeddings_tenant
    ON policy_embeddings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_policy_embeddings_vec
    ON policy_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

CREATE TABLE IF NOT EXISTS email_history_embeddings (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID    NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    content     TEXT    NOT NULL,
    embedding   vector(3072) NOT NULL,
    verdict     VARCHAR(10) NOT NULL CHECK (verdict IN ('CLEAN', 'LOW', 'MEDIUM', 'HIGH')),
    violations  TEXT[],
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_history_tenant
    ON email_history_embeddings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_email_history_vec
    ON email_history_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_email_history_verdict_tenant
    ON email_history_embeddings (verdict, tenant_id);
