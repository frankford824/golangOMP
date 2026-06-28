CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  endpoint_hash CHAR(64) NOT NULL,
  endpoint TEXT NOT NULL,
  p256dh VARCHAR(255) NOT NULL,
  auth VARCHAR(255) NOT NULL,
  user_agent VARCHAR(512) NOT NULL DEFAULT '',
  platform VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  vapid_key_hash CHAR(64) NOT NULL DEFAULT '',
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  disabled_at DATETIME NULL,
  disabled_reason VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_web_push_subscriptions_endpoint_hash (endpoint_hash),
  KEY idx_web_push_subscriptions_user_status (user_id, status, updated_at),
  KEY idx_web_push_subscriptions_status_key (status, vapid_key_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notification_delivery_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  notification_id BIGINT NOT NULL,
  subscription_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL DEFAULT 'web_push',
  payload JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  attempt_count INT NOT NULL DEFAULT 0,
  next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  lease_until DATETIME NULL,
  claim_token VARCHAR(128) NOT NULL DEFAULT '',
  last_error TEXT NULL,
  provider_status_code INT NULL,
  sent_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_notification_delivery_target (notification_id, subscription_id, channel),
  KEY idx_notification_delivery_claim (channel, status, next_attempt_at, lease_until, id),
  KEY idx_notification_delivery_user (user_id, status, created_at),
  KEY idx_notification_delivery_subscription (subscription_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notification_dedupe_claims (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  notification_type VARCHAR(64) NOT NULL,
  dedupe_scope VARCHAR(255) NOT NULL,
  dedupe_key VARCHAR(255) NOT NULL,
  notification_id BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_notification_dedupe_user_type_key (user_id, notification_type, dedupe_key),
  KEY idx_notification_dedupe_scope (dedupe_scope),
  KEY idx_notification_dedupe_notification (notification_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notification_preferences (
  user_id BIGINT NOT NULL,
  web_push_enabled TINYINT(1) NOT NULL DEFAULT 0,
  last_test_sent_at DATETIME NULL,
  vapid_key_hash CHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notification_dedupe_claims;
DROP TABLE IF EXISTS notification_delivery_outbox;
DROP TABLE IF EXISTS web_push_subscriptions;
-- ROLLBACK-END
