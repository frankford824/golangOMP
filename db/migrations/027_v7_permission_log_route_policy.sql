-- Migration: 027_v7_permission_log_route_policy.sql
-- Step 56: persist route-policy context on permission logs for Step B auditing.

ALTER TABLE permission_logs
  ADD COLUMN readiness VARCHAR(64) NOT NULL DEFAULT '' AFTER auth_mode,
  ADD COLUMN session_required TINYINT(1) NOT NULL DEFAULT 0 AFTER readiness,
  ADD COLUMN debug_compatible TINYINT(1) NOT NULL DEFAULT 0 AFTER session_required;

CREATE INDEX idx_permission_logs_actor_username ON permission_logs (actor_username);
CREATE INDEX idx_permission_logs_method ON permission_logs (method);
