-- Index-only repair for retention and recent-run reads. Do not rewrite records.
-- Build both indexes in one online ALTER; retry only missing indexes after a
-- partial/manual installation. Bound metadata-lock waiting rather than queueing
-- indefinitely behind a long transaction.
SET @yb_previous_lock_wait_timeout := @@SESSION.lock_wait_timeout;
SET SESSION lock_wait_timeout = 5;
SET @yb_add_created := NOT EXISTS (
  SELECT 1 FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'experience_worker_runs'
    AND INDEX_NAME = 'idx_experience_worker_runs_created'
);
SET @yb_add_started := NOT EXISTS (
  SELECT 1 FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'experience_worker_runs'
    AND INDEX_NAME = 'idx_experience_worker_runs_started'
);
SET @yb_ddl := IF(@yb_add_created OR @yb_add_started,
  CONCAT('ALTER TABLE experience_worker_runs ',
    IF(@yb_add_created, 'ADD INDEX idx_experience_worker_runs_created (created_at, id)', ''),
    IF(@yb_add_created AND @yb_add_started, ', ', ''),
    IF(@yb_add_started, 'ADD INDEX idx_experience_worker_runs_started (started_at, id)', ''),
    ', ALGORITHM=INPLACE, LOCK=NONE'),
  'SELECT 1'
);
PREPARE yb_stmt FROM @yb_ddl;
EXECUTE yb_stmt;
DEALLOCATE PREPARE yb_stmt;
SET SESSION lock_wait_timeout = @yb_previous_lock_wait_timeout;

-- ROLLBACK-BEGIN
-- Index rollback only; existing data and other indexes remain intact.
ALTER TABLE experience_worker_runs
  DROP INDEX idx_experience_worker_runs_created,
  DROP INDEX idx_experience_worker_runs_started,
  ALGORITHM=INPLACE, LOCK=NONE;
