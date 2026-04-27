DROP INDEX IF EXISTS idx_attachments_org;
ALTER TABLE attachments DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_api_keys_org;
ALTER TABLE api_keys DROP COLUMN IF EXISTS org_id;
