-- Migration: 039_v8_identity_org_scope_extension.sql
-- Minimal org/user-role scope closure for account permissions without rebuilding the task domain.

ALTER TABLE users
  ADD COLUMN managed_departments_json TEXT NULL COMMENT 'JSON array of managed departments for org/data-scope management' AFTER team,
  ADD COLUMN managed_teams_json TEXT NULL COMMENT 'JSON array of managed teams for org/data-scope management' AFTER managed_departments_json;
