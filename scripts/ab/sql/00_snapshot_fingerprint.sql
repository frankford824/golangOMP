SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;
SET SESSION group_concat_max_len = 16777216;

SELECT 'audit.side' AS metric, @ab_side AS value
UNION ALL SELECT 'audit.run_id', @ab_run_id
UNION ALL SELECT 'database_name', CONVERT(DATABASE() USING utf8mb4) COLLATE utf8mb4_unicode_ci
UNION ALL SELECT 'server_uuid', CONVERT(@@server_uuid USING utf8mb4) COLLATE utf8mb4_unicode_ci
UNION ALL SELECT 'base_table_count', CAST(COUNT(*) AS CHAR) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
UNION ALL SELECT 'tasks.count', CAST(COUNT(*) AS CHAR) FROM tasks
UNION ALL SELECT 'tasks.max_id', CAST(COALESCE(MAX(id), 0) AS CHAR) FROM tasks
UNION ALL SELECT 'task_assets.count', CAST(COUNT(*) AS CHAR) FROM task_assets
UNION ALL SELECT 'task_assets.max_id', CAST(COALESCE(MAX(id), 0) AS CHAR) FROM task_assets
UNION ALL SELECT 'task_event_logs.count', CAST(COUNT(*) AS CHAR) FROM task_event_logs
UNION ALL SELECT 'task_event_logs.max_sequence', CAST(COALESCE(MAX(sequence), 0) AS CHAR) FROM task_event_logs
UNION ALL SELECT 'task_module_events.count', CAST(COUNT(*) AS CHAR) FROM task_module_events
UNION ALL SELECT 'task_module_events.max_id', CAST(COALESCE(MAX(id), 0) AS CHAR) FROM task_module_events
UNION ALL SELECT 'schema_migrations.count', CAST(COUNT(*) AS CHAR) FROM schema_migrations
UNION ALL SELECT 'schema_migrations.sha256', CONVERT(SHA2(GROUP_CONCAT(CONCAT_WS(':', file_name, checksum_sha256, status) ORDER BY file_name SEPARATOR '|'), 256) USING utf8mb4) COLLATE utf8mb4_unicode_ci FROM schema_migrations
ORDER BY metric;
