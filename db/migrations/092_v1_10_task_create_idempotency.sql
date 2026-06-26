CREATE TABLE IF NOT EXISTS task_create_requests (
  id BIGINT NOT NULL AUTO_INCREMENT,
  client_create_id VARCHAR(128) NOT NULL,
  actor_id BIGINT NOT NULL,
  payload_hash VARCHAR(128) NOT NULL,
  request_payload_json LONGTEXT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'in_progress',
  task_id BIGINT NULL,
  error_message VARCHAR(500) NOT NULL DEFAULT '',
  expires_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_create_requests_actor_client (actor_id, client_create_id),
  KEY idx_task_create_requests_task_id (task_id),
  KEY idx_task_create_requests_payload_hash (payload_hash),
  KEY idx_task_create_requests_status_expires (status, expires_at),
  CONSTRAINT fk_task_create_requests_task
    FOREIGN KEY (task_id) REFERENCES tasks(id)
    ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
