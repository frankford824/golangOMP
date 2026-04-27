-- Migration: 030_v7_auth_frontend_minimal_upgrade.sql
-- Step 63: frontend-ready auth field expansion, configurable super admins, and department admin bootstrap metadata.

ALTER TABLE users
  ADD COLUMN department VARCHAR(64) NOT NULL DEFAULT '' AFTER display_name,
  ADD COLUMN mobile VARCHAR(32) NOT NULL DEFAULT '' AFTER department,
  ADD COLUMN email VARCHAR(128) NOT NULL DEFAULT '' AFTER mobile,
  ADD COLUMN is_config_super_admin TINYINT(1) NOT NULL DEFAULT 0 AFTER status;

ALTER TABLE users
  ADD UNIQUE KEY uq_users_mobile (mobile),
  ADD KEY idx_users_department (department),
  ADD KEY idx_users_config_super_admin (is_config_super_admin);
