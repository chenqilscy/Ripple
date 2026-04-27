-- P14.2: org scope for quota-enforced resources
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys (org_id) WHERE org_id <> '' AND revoked_at IS NULL;

ALTER TABLE attachments ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_attachments_org ON attachments (org_id) WHERE org_id <> '';
