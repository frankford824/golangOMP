-- Migration: 025_v7_identity_auth_minimal.sql
-- Step 52: real user register/login, session tokens, role assignment, permission logs.

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  username      VARCHAR(64)  NOT NULL,
  display_name  VARCHAR(128) NOT NULL DEFAULT '',
  password_hash VARCHAR(255) NOT NULL,
  status        VARCHAR(16)  NOT NULL DEFAULT 'active',
  last_login_at DATETIME     NULL,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_username (username),
  KEY idx_users_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Workflow users for minimal real login/session phase';

CREATE TABLE IF NOT EXISTS user_roles (
  user_id    BIGINT      NOT NULL,
  role       VARCHAR(32) NOT NULL,
  created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, role),
  CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Minimal role assignment table for workflow users';

CREATE TABLE IF NOT EXISTS user_sessions (
  session_id   VARCHAR(64) NOT NULL,
  user_id      BIGINT      NOT NULL,
  token_hash   VARCHAR(64) NOT NULL,
  expires_at   DATETIME    NOT NULL,
  last_seen_at DATETIME    NULL,
  revoked_at   DATETIME    NULL,
  created_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (session_id),
  UNIQUE KEY uq_user_sessions_token_hash (token_hash),
  KEY idx_user_sessions_user_id (user_id),
  KEY idx_user_sessions_expires_at (expires_at),
  CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Session-token records for minimal real login phase';

CREATE TABLE IF NOT EXISTS permission_logs (
  id                  BIGINT       NOT NULL AUTO_INCREMENT,
  actor_id            BIGINT       NULL,
  actor_username      VARCHAR(64)  NOT NULL DEFAULT '',
  actor_source        VARCHAR(64)  NOT NULL DEFAULT '',
  auth_mode           VARCHAR(64)  NOT NULL DEFAULT '',
  actor_roles_json    JSON         NOT NULL DEFAULT (JSON_ARRAY()),
  method              VARCHAR(16)  NOT NULL,
  route_path          VARCHAR(255) NOT NULL,
  required_roles_json JSON         NOT NULL DEFAULT (JSON_ARRAY()),
  granted             TINYINT(1)   NOT NULL DEFAULT 0,
  reason              VARCHAR(255) NOT NULL DEFAULT '',
  created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_permission_logs_actor_id (actor_id),
  KEY idx_permission_logs_granted (granted),
  KEY idx_permission_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Route-level permission decision log for minimal auth phase';
