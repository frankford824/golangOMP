-- Migration: 001_v7_tables.sql
-- V7 new tables: products, tasks, task_details, code_rules, code_rule_sequences
-- Strategy: additive — does NOT drop any V6 tables.
-- Engine: InnoDB, charset: utf8mb4 (per project spec)

-- ── 1. products ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS products (
  id               BIGINT       NOT NULL AUTO_INCREMENT,
  erp_product_id   VARCHAR(64)  NOT NULL COMMENT 'ERP primary key; UNIQUE to prevent duplicates',
  sku_code         VARCHAR(64)  NOT NULL,
  product_name     VARCHAR(255) NOT NULL,
  category         VARCHAR(128) NOT NULL DEFAULT '',
  spec_json        JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  status           VARCHAR(32)  NOT NULL DEFAULT 'active',
  source_updated_at DATETIME    NULL,
  sync_time        DATETIME     NULL,
  created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_products_erp_product_id (erp_product_id),
  KEY idx_products_sku_code (sku_code),
  KEY idx_products_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ERP product master data (synced every 5 min)';

-- ── 2. tasks ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tasks (
  id                    BIGINT       NOT NULL AUTO_INCREMENT,
  task_no               VARCHAR(64)  NOT NULL COMMENT 'System-generated unique task number',
  source_mode           VARCHAR(32)  NOT NULL COMMENT 'existing_product | new_product',
  product_id            BIGINT       NULL     COMMENT 'FK → products.id (NULL for new_product mode)',
  sku_code              VARCHAR(64)  NOT NULL COMMENT 'Must be set at task creation time',
  product_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  task_type             VARCHAR(32)  NOT NULL DEFAULT 'regular',
  operator_group_id     BIGINT       NULL,
  creator_id            BIGINT       NOT NULL,
  designer_id           BIGINT       NULL,
  current_handler_id    BIGINT       NULL,
  task_status           VARCHAR(64)  NOT NULL DEFAULT 'Draft',
  priority              VARCHAR(16)  NOT NULL DEFAULT 'normal',
  deadline_at           DATETIME     NULL,
  need_outsource        TINYINT(1)   NOT NULL DEFAULT 0,
  created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_tasks_task_no (task_no),
  KEY idx_tasks_sku_code (sku_code),
  KEY idx_tasks_task_status (task_status),
  KEY idx_tasks_creator_id (creator_id),
  KEY idx_tasks_designer_id (designer_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 business aggregate root';

-- ── 3. task_details ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS task_details (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  task_id         BIGINT       NOT NULL,
  demand_text     TEXT         NOT NULL,
  copy_text       TEXT         NOT NULL,
  style_keywords  VARCHAR(512) NOT NULL DEFAULT '',
  remark          TEXT         NOT NULL,
  risk_flags_json JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_details_task_id (task_id),
  CONSTRAINT fk_task_details_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ── 4. code_rules ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS code_rules (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  rule_type   VARCHAR(32)  NOT NULL COMMENT 'task_no | new_sku | outsource_no | handover_no',
  rule_name   VARCHAR(128) NOT NULL,
  prefix      VARCHAR(32)  NOT NULL DEFAULT '',
  date_format VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'Go time layout e.g. 20060102',
  site_code   VARCHAR(16)  NOT NULL DEFAULT '',
  biz_code    VARCHAR(16)  NOT NULL DEFAULT '',
  seq_length  INT          NOT NULL DEFAULT 6,
  reset_cycle VARCHAR(16)  NOT NULL DEFAULT 'none' COMMENT 'none | daily | monthly',
  is_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
  config_json JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_code_rules_rule_type (rule_type),
  KEY idx_code_rules_enabled (is_enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Numbering rule configuration (V7 §5)';

-- ── 5. code_rule_sequences ───────────────────────────────────────────────────
-- Counter table for atomic sequence generation; uses SELECT FOR UPDATE inside TX.
CREATE TABLE IF NOT EXISTS code_rule_sequences (
  rule_id  BIGINT NOT NULL,
  last_seq BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (rule_id),
  CONSTRAINT fk_crs_rule FOREIGN KEY (rule_id) REFERENCES code_rules (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Per-rule sequence counter';

-- ── 6. Seed: default code rules ──────────────────────────────────────────────
INSERT IGNORE INTO code_rules (rule_type, rule_name, prefix, date_format, site_code, seq_length, reset_cycle, is_enabled, config_json)
VALUES
  ('task_no',     'Default Task No Rule',     'RW',   '20060102', 'A', 6, 'daily',   1, '{}'),
  ('new_sku',     'Default New SKU Rule',      'SKU',  '',         '',  6, 'none',    1, '{}'),
  ('outsource_no','Default Outsource No Rule', 'WB',   '20060102', 'B', 6, 'daily',   1, '{}'),
  ('handover_no', 'Default Handover No Rule',  'HD',   '20060102', 'A', 6, 'daily',   1, '{}');
