-- M3-S1 rollback: spaces + space_members
BEGIN;
DROP TRIGGER IF EXISTS space_members_updated_at ON space_members;
DROP TRIGGER IF EXISTS spaces_updated_at ON spaces;
DROP TABLE IF EXISTS space_members;
DROP TABLE IF EXISTS spaces;
COMMIT;
