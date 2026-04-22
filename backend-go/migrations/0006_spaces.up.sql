-- M3-S1: spaces + space_members
-- 引用：docs/system-design/M3-设计白皮书.md §2.1
-- Lake↔Space 绑定留至 S1.5（涉及 Neo4j 端，单独 PR）

BEGIN;

CREATE TABLE IF NOT EXISTS spaces (
  id                      UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id                UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name                    TEXT         NOT NULL,
  description             TEXT         NOT NULL DEFAULT '',
  llm_quota_monthly       INT          NOT NULL DEFAULT 10000,
  llm_used_current_month  INT          NOT NULL DEFAULT 0,
  created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_spaces_owner ON spaces(owner_id);

CREATE TABLE IF NOT EXISTS space_members (
  space_id    UUID         NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role        VARCHAR(16)  NOT NULL CHECK (role IN ('OWNER','EDITOR','VIEWER')),
  joined_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (space_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_space_members_user ON space_members(user_id);

DROP TRIGGER IF EXISTS spaces_updated_at ON spaces;
CREATE TRIGGER spaces_updated_at BEFORE UPDATE ON spaces
  FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

DROP TRIGGER IF EXISTS space_members_updated_at ON space_members;
CREATE TRIGGER space_members_updated_at BEFORE UPDATE ON space_members
  FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

COMMIT;
