-- 0002_cloud down
DROP INDEX IF EXISTS ix_cloud_tasks_owner_created;
DROP INDEX IF EXISTS ix_cloud_tasks_status_created;
DROP TABLE IF EXISTS cloud_tasks;
