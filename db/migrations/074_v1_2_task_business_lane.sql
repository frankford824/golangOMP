-- Migration: 074_v1_2_task_business_lane.sql
-- Add formal task business lane and keep workflow-lane compatibility projection.

ALTER TABLE tasks
  ADD COLUMN business_lane VARCHAR(32) NOT NULL DEFAULT 'normal'
    COMMENT 'Task business lane: normal | customization' AFTER is_outsource;

-- Backfill historical customization-lane tasks.
-- IMPORTANT: business_lane and customization_required are not equivalent in runtime.
UPDATE tasks
SET business_lane = 'customization'
WHERE customization_required = 1
   OR task_type IN ('customer_customization', 'regular_customization');

CREATE INDEX idx_tasks_business_lane_status ON tasks (business_lane, task_status);
