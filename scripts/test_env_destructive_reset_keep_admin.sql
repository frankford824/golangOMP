-- High-risk destructive test reset.
-- Goal: clear test business data while keeping Admin/SuperAdmin login capability.
-- Scope: data-only reset (no schema or migration changes).

SET SESSION sql_safe_updates = 0;
SET FOREIGN_KEY_CHECKS = 0;

-- 1) Resolve and lock keep-admin user set.
DROP TEMPORARY TABLE IF EXISTS tmp_keep_admin_users;
CREATE TEMPORARY TABLE tmp_keep_admin_users (
  id BIGINT PRIMARY KEY
);

INSERT INTO tmp_keep_admin_users (id)
SELECT u.id
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
GROUP BY u.id, u.username, u.is_config_super_admin
HAVING SUM(CASE WHEN ur.role IN ('Admin', 'SuperAdmin') THEN 1 ELSE 0 END) > 0
   OR u.is_config_super_admin = 1
   OR LOWER(u.username) = 'admin';

SET @keep_admin_count := (SELECT COUNT(*) FROM tmp_keep_admin_users);
SET @guard_sql := IF(
  @keep_admin_count > 0,
  'SELECT "ADMIN_GUARD_OK" AS guard_status',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''Admin guard failed: keep-admin set is empty'''
);
PREPARE guard_stmt FROM @guard_sql;
EXECUTE guard_stmt;
DEALLOCATE PREPARE guard_stmt;

-- 2) Clear session/auth runtime traces (keep users, clear sessions).
DELETE FROM user_sessions;

-- 3) Clear task / asset / procurement / workflow / customization traces.
DELETE FROM task_event_logs;
DELETE FROM task_event_sequences;
DELETE FROM procurement_record_items;
DELETE FROM procurement_records;
DELETE FROM warehouse_receipts;
DELETE FROM outsource_orders;
DELETE FROM audit_handovers;
DELETE FROM audit_records;

-- Customization job business data (pricing rules are master config, kept).
SET @has_customization_jobs := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 'customization_jobs'
);
SET @cj_sql := IF(@has_customization_jobs > 0, 'DELETE FROM customization_jobs', 'SELECT "SKIP customization_jobs (missing)"');
PREPARE cj_stmt FROM @cj_sql;
EXECUTE cj_stmt;
DEALLOCATE PREPARE cj_stmt;
DELETE FROM cost_override_finance_flags;
DELETE FROM cost_override_reviews;
DELETE FROM cost_override_events;
DELETE FROM cost_override_event_sequences;
DELETE FROM task_assets;
DELETE FROM design_assets;

-- Optional task design/read-model table in some live envs (skip cleanly if absent).
SET @has_asset_versions := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_versions'
);
SET @asset_versions_sql := IF(@has_asset_versions > 0, 'DELETE FROM asset_versions', 'SELECT "SKIP asset_versions (missing)"');
PREPARE asset_versions_stmt FROM @asset_versions_sql;
EXECUTE asset_versions_stmt;
DEALLOCATE PREPARE asset_versions_stmt;
SET @asset_versions_count := NULL;
SET @asset_versions_count_sql := IF(
  @has_asset_versions > 0,
  'SELECT COUNT(*) INTO @asset_versions_count FROM asset_versions',
  'SELECT NULL INTO @asset_versions_count'
);
PREPARE asset_versions_count_stmt FROM @asset_versions_count_sql;
EXECUTE asset_versions_count_stmt;
DEALLOCATE PREPARE asset_versions_count_stmt;

DELETE FROM task_sku_items;
DELETE FROM task_details;
DELETE FROM tasks;

-- 4) Clear task-linked upload/storage metadata and related logs.
DELETE FROM asset_storage_refs;
DELETE FROM upload_requests;
DELETE FROM integration_call_executions;
DELETE FROM integration_call_logs;
DELETE FROM export_job_attempts;
DELETE FROM export_job_dispatches;
DELETE FROM export_job_events;
DELETE FROM export_job_event_sequences;
DELETE FROM export_jobs;
DELETE FROM permission_logs;
DELETE FROM server_logs;
DELETE FROM erp_sync_runs;

-- Optional runtime log table in live envs (skip cleanly if absent).
SET @has_erp_sync_log := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 'erp_sync_log'
);
SET @erp_sync_log_sql := IF(@has_erp_sync_log > 0, 'DELETE FROM erp_sync_log', 'SELECT "SKIP erp_sync_log (missing)"');
PREPARE erp_sync_log_stmt FROM @erp_sync_log_sql;
EXECUTE erp_sync_log_stmt;
DEALLOCATE PREPARE erp_sync_log_stmt;

DELETE FROM distribution_jobs;
DELETE FROM job_attempts;
DELETE FROM event_logs;
DELETE FROM sku_sequences;
DELETE FROM workbench_preferences;

-- 5) Keep admin users; remove non-admin/non-superadmin test users.
DELETE ur
FROM user_roles ur
LEFT JOIN tmp_keep_admin_users ka ON ka.id = ur.user_id
WHERE ka.id IS NULL;

DELETE u
FROM users u
LEFT JOIN tmp_keep_admin_users ka ON ka.id = u.id
WHERE ka.id IS NULL;

SET FOREIGN_KEY_CHECKS = 1;

-- 6) Summary output for audit trail.
SELECT 'keep_admin_count' AS metric, @keep_admin_count AS value
UNION ALL
SELECT 'tasks_after_reset', COUNT(*) FROM tasks
UNION ALL
SELECT 'task_details_after_reset', COUNT(*) FROM task_details
UNION ALL
SELECT 'task_assets_after_reset', COUNT(*) FROM task_assets
UNION ALL
SELECT 'design_assets_after_reset', COUNT(*) FROM design_assets
UNION ALL
SELECT 'asset_versions_after_reset', @asset_versions_count
UNION ALL
SELECT 'procurement_records_after_reset', COUNT(*) FROM procurement_records
UNION ALL
SELECT 'task_event_logs_after_reset', COUNT(*) FROM task_event_logs
UNION ALL
SELECT 'upload_requests_after_reset', COUNT(*) FROM upload_requests
UNION ALL
SELECT 'asset_storage_refs_after_reset', COUNT(*) FROM asset_storage_refs
UNION ALL
SELECT 'permission_logs_after_reset', COUNT(*) FROM permission_logs
UNION ALL
SELECT 'integration_call_logs_after_reset', COUNT(*) FROM integration_call_logs
UNION ALL
SELECT 'users_after_reset', COUNT(*) FROM users
UNION ALL
SELECT 'user_sessions_after_reset', COUNT(*) FROM user_sessions
UNION ALL
SELECT 'customization_jobs_after_reset', COALESCE((SELECT COUNT(*) FROM customization_jobs), 0);
