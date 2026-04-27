-- Migration: 032_v7_identity_user_profile_extension.sql
-- Step 65: user profile and contact fields for auth/org integration and frontend access.

ALTER TABLE users
  ADD COLUMN department VARCHAR(64) NOT NULL DEFAULT '' AFTER display_name,
  ADD COLUMN mobile VARCHAR(32) NOT NULL DEFAULT '' AFTER department,
  ADD COLUMN email VARCHAR(128) NOT NULL DEFAULT '' AFTER mobile,
  ADD COLUMN team VARCHAR(64) NOT NULL DEFAULT '' AFTER department,
  ADD COLUMN is_config_super_admin TINYINT(1) NOT NULL DEFAULT 0 AFTER status;

