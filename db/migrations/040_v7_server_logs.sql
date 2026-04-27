-- 040: server_logs table for server log management (query, filter, clean, masking)
-- Used by GET /v1/server-logs and POST /v1/server-logs/clean

CREATE TABLE IF NOT EXISTS server_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  level VARCHAR(16) NOT NULL DEFAULT 'info',
  msg TEXT NOT NULL,
  details_json JSON,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  INDEX idx_server_logs_created (created_at),
  INDEX idx_server_logs_level (level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
