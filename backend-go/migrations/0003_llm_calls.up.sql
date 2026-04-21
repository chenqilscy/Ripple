-- llm_calls · LLM 调用审计与计费
CREATE TABLE IF NOT EXISTS llm_calls (
    id            BIGSERIAL PRIMARY KEY,
    provider      VARCHAR(32) NOT NULL,
    modality      VARCHAR(16) NOT NULL,
    prompt_hash   VARCHAR(64) NOT NULL,
    candidates_n  INT NOT NULL DEFAULT 0,
    cost_tokens   BIGINT NOT NULL DEFAULT 0,
    latency_ms    INT NOT NULL DEFAULT 0,
    status        VARCHAR(16) NOT NULL,           -- ok | error
    error_message TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_calls_created_at ON llm_calls (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_calls_provider_status ON llm_calls (provider, status);
