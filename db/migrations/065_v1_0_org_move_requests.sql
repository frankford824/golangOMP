CREATE TABLE IF NOT EXISTS org_move_requests (
  id BIGINT NOT NULL AUTO_INCREMENT,
  source_department VARCHAR(64) NOT NULL,
  target_department VARCHAR(64) NULL,
  user_id BIGINT NOT NULL,
  state VARCHAR(64) NOT NULL DEFAULT 'pending_super_admin_confirm',
  requested_by BIGINT NOT NULL,
  resolved_by BIGINT NULL,
  reason TEXT NOT NULL,
  resolved_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_org_move_requests_state_created (state, created_at),
  KEY idx_org_move_requests_user_state (user_id, state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS org_move_requests;
-- ROLLBACK-END
