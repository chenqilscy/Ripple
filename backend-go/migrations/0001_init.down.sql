-- Ripple PG Schema · Initial Migration · DOWN
-- 危险：会丢失所有数据。仅用于开发环境重置。

BEGIN;

DROP TRIGGER IF EXISTS outbox_events_updated_at ON outbox_events;
DROP TRIGGER IF EXISTS lake_memberships_updated_at ON lake_memberships;
DROP TRIGGER IF EXISTS users_updated_at ON users;

DROP FUNCTION IF EXISTS trg_set_updated_at();

DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS lake_memberships;
DROP TABLE IF EXISTS users;

COMMIT;
