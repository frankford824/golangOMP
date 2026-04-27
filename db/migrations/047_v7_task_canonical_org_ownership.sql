-- Migration: 047_v7_task_canonical_org_ownership.sql
-- Add canonical task org ownership fields while keeping legacy owner_team intact.

ALTER TABLE tasks
  ADD COLUMN owner_department VARCHAR(64) NULL COMMENT 'Canonical task owner department name',
  ADD COLUMN owner_org_team VARCHAR(64) NULL COMMENT 'Canonical task owner org-team name';

CREATE INDEX idx_tasks_owner_department ON tasks (owner_department);
CREATE INDEX idx_tasks_owner_org_team ON tasks (owner_org_team);

-- Safe minimal backfill only where department mapping from legacy owner_team is unique.
-- owner_org_team remains NULL for historical legacy rows because reverse team mapping is ambiguous.
UPDATE tasks
SET owner_department = '设计部'
WHERE owner_department IS NULL
  AND owner_team = '设计组';

UPDATE tasks
SET owner_department = '运营部'
WHERE owner_department IS NULL
  AND owner_team = '内贸运营组';
