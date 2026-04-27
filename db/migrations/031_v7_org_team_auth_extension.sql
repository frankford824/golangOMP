-- Migration: 031_v7_org_team_auth_extension.sql
-- Step 64: department-team registration extension and minimal org model persistence.

ALTER TABLE users
  ADD COLUMN team VARCHAR(64) NOT NULL DEFAULT '' AFTER department;

ALTER TABLE users
  ADD KEY idx_users_department_team (department, team);
