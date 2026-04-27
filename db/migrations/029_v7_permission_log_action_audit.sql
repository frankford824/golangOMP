-- Migration: 029_v7_permission_log_action_audit.sql
-- Step 62: extend permission logs to cover register/login/role-assignment audit actions.

ALTER TABLE permission_logs
  ADD COLUMN action_type VARCHAR(64) NOT NULL DEFAULT 'route_access' AFTER debug_compatible,
  ADD COLUMN target_user_id BIGINT NULL AFTER actor_roles_json,
  ADD COLUMN target_username VARCHAR(64) NOT NULL DEFAULT '' AFTER target_user_id,
  ADD COLUMN target_roles_json JSON NOT NULL DEFAULT (JSON_ARRAY()) AFTER target_username;

CREATE INDEX idx_permission_logs_action_type ON permission_logs (action_type);
CREATE INDEX idx_permission_logs_target_user_id ON permission_logs (target_user_id);
