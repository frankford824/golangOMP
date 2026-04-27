-- Migration: 058_v1_0_org_team_department_scoped_uniqueness.sql
-- Purpose: allow the v1.0 official org baseline to reuse the same team name
-- under multiple departments (for example `默认组` under 设计研发部 / 定制美工部 /
-- 云仓部). The original backendized org master schema added both:
--   - uq_org_teams_name (name)
--   - uq_org_teams_department_name (department_id, name)
-- The global name-unique index blocks the official baseline, so this
-- migration drops only the redundant global index and keeps the
-- department-scoped uniqueness guardrail intact.

SET @has_uq_org_teams_name := (
  SELECT COUNT(*)
    FROM information_schema.STATISTICS
   WHERE TABLE_SCHEMA = DATABASE()
     AND TABLE_NAME = 'org_teams'
     AND INDEX_NAME = 'uq_org_teams_name'
);

SET @drop_uq_org_teams_name_sql := IF(
  @has_uq_org_teams_name > 0,
  'ALTER TABLE org_teams DROP INDEX uq_org_teams_name',
  'SELECT 1'
);

PREPARE drop_uq_org_teams_name_stmt FROM @drop_uq_org_teams_name_sql;
EXECUTE drop_uq_org_teams_name_stmt;
DEALLOCATE PREPARE drop_uq_org_teams_name_stmt;
