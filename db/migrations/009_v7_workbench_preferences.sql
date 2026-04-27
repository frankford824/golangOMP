-- Migration: 009_v7_workbench_preferences.sql
-- Lightweight saved workbench preferences scoped by placeholder actor identity.

CREATE TABLE IF NOT EXISTS workbench_preferences (
  id                BIGINT       NOT NULL AUTO_INCREMENT,
  actor_id          BIGINT       NOT NULL,
  actor_roles_key   VARCHAR(255) NOT NULL DEFAULT '',
  auth_mode         VARCHAR(64)  NOT NULL DEFAULT 'placeholder_no_enforcement',
  default_queue_key VARCHAR(64)  NOT NULL DEFAULT '',
  pinned_queue_keys JSON         NOT NULL DEFAULT (JSON_ARRAY()),
  default_filters   JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  default_page_size INT          NOT NULL DEFAULT 0,
  default_sort      VARCHAR(64)  NOT NULL DEFAULT '',
  created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_workbench_preferences_actor_scope (actor_id, actor_roles_key, auth_mode)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Placeholder-actor-scoped saved workbench preferences';
