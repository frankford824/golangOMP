-- Task ownership is a snapshot. Its names, stable IDs and indexes remain
-- available after the source organization is hard-deleted. User membership
-- and the department/team hierarchy still reference live organization rows.
SET @yb_previous_lock_wait_timeout := @@SESSION.lock_wait_timeout;
SET SESSION lock_wait_timeout = 5;
SET @yb_drop_department_fk := EXISTS (
  SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks'
    AND CONSTRAINT_NAME = 'fk_tasks_owner_department_id'
);
SET @yb_drop_team_fk := EXISTS (
  SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks'
    AND CONSTRAINT_NAME = 'fk_tasks_owner_team_id'
);
SET @yb_ddl := IF(@yb_drop_department_fk OR @yb_drop_team_fk,
  CONCAT('ALTER TABLE tasks ',
    IF(@yb_drop_department_fk, 'DROP FOREIGN KEY fk_tasks_owner_department_id', ''),
    IF(@yb_drop_department_fk AND @yb_drop_team_fk, ', ', ''),
    IF(@yb_drop_team_fk, 'DROP FOREIGN KEY fk_tasks_owner_team_id', ''),
    ', ALGORITHM=INPLACE, LOCK=NONE'),
  'DO 0'
);
PREPARE yb_stmt FROM @yb_ddl;
EXECUTE yb_stmt;
DEALLOCATE PREPARE yb_stmt;
SET SESSION lock_wait_timeout = @yb_previous_lock_wait_timeout;

-- ROLLBACK-BEGIN
-- Restore any deleted organization rows with their original IDs before rollback.
ALTER TABLE tasks
  ADD CONSTRAINT fk_tasks_owner_department_id FOREIGN KEY (owner_department_id) REFERENCES org_departments (id),
  ADD CONSTRAINT fk_tasks_owner_team_id FOREIGN KEY (owner_team_id) REFERENCES org_teams (id);
-- ROLLBACK-END
