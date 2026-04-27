-- Migration: 002_v7_audit_outsource.sql
-- V7 Step-02: audit_records, audit_handovers, outsource_orders,
--             task_event_logs, task_event_sequences
-- Strategy: additive — no V6 tables modified or dropped.
-- Engine: InnoDB, charset: utf8mb4

-- ── 1. audit_records ─────────────────────────────────────────────────────────
-- Distinct from V6 audit_actions (which is asset-version scoped).
-- One row per audit action taken against a task.
CREATE TABLE IF NOT EXISTS audit_records (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  task_id         BIGINT       NOT NULL,
  stage           VARCHAR(32)  NOT NULL COMMENT 'A | B | outsource_review',
  action          VARCHAR(32)  NOT NULL COMMENT 'claim | approve | reject | transfer | handover | takeover',
  auditor_id      BIGINT       NOT NULL,
  issue_types_json JSON        NOT NULL DEFAULT (JSON_ARRAY()),
  comment         TEXT         NOT NULL,
  affects_launch  TINYINT(1)   NOT NULL DEFAULT 0,
  need_outsource  TINYINT(1)   NOT NULL DEFAULT 0,
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_audit_records_task_id (task_id),
  KEY idx_audit_records_auditor_id (auditor_id),
  CONSTRAINT fk_audit_records_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 task-centric audit action log';

-- ── 2. audit_handovers ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_handovers (
  id                BIGINT       NOT NULL AUTO_INCREMENT,
  handover_no       VARCHAR(64)  NOT NULL COMMENT 'System-generated; UNIQUE',
  task_id           BIGINT       NOT NULL,
  from_auditor_id   BIGINT       NOT NULL,
  to_auditor_id     BIGINT       NOT NULL,
  reason            TEXT         NOT NULL,
  current_judgement TEXT         NOT NULL,
  risk_remark       TEXT         NOT NULL,
  status            VARCHAR(32)  NOT NULL DEFAULT 'pending_takeover'
                                 COMMENT 'pending_takeover | taken_over | cancelled',
  created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_audit_handovers_no (handover_no),
  KEY idx_audit_handovers_task_id (task_id),
  KEY idx_audit_handovers_status (status),
  CONSTRAINT fk_audit_handovers_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 audit shift-handover records';

-- ── 3. outsource_orders ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS outsource_orders (
  id                   BIGINT       NOT NULL AUTO_INCREMENT,
  outsource_no         VARCHAR(64)  NOT NULL COMMENT 'System-generated; UNIQUE',
  task_id              BIGINT       NOT NULL,
  vendor_name          VARCHAR(255) NOT NULL DEFAULT '',
  outsource_type       VARCHAR(64)  NOT NULL DEFAULT '',
  delivery_requirement TEXT         NOT NULL,
  settlement_note      TEXT         NOT NULL,
  status               VARCHAR(32)  NOT NULL DEFAULT 'created'
                                    COMMENT 'created | packaged | sent | in_production | returned | reviewing | approved | rejected | closed',
  returned_at          DATETIME     NULL,
  created_at           DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at           DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_outsource_orders_no (outsource_no),
  KEY idx_outsource_orders_task_id (task_id),
  KEY idx_outsource_orders_status (status),
  CONSTRAINT fk_outsource_orders_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 outsource/customisation orders';

-- ── 4. task_event_logs ───────────────────────────────────────────────────────
-- V7 task-scoped event log. Separate from V6 event_logs (SKU-scoped) by design.
-- See domain/task_event.go for design rationale.
CREATE TABLE IF NOT EXISTS task_event_logs (
  id          VARCHAR(36)  NOT NULL COMMENT 'UUID',
  task_id     BIGINT       NOT NULL,
  sequence    BIGINT       NOT NULL COMMENT 'Monotonically increasing per task_id',
  event_type  VARCHAR(128) NOT NULL,
  operator_id BIGINT       NULL,
  payload     JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_events_seq (task_id, sequence),
  KEY idx_task_event_logs_task_id (task_id),
  KEY idx_task_event_logs_event_type (event_type),
  CONSTRAINT fk_task_event_logs_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 task-scoped event log';

-- ── 5. task_event_sequences ──────────────────────────────────────────────────
-- Counter table for atomic per-task sequence generation.
-- Mirrors the sku_sequences pattern from V6.
CREATE TABLE IF NOT EXISTS task_event_sequences (
  task_id       BIGINT NOT NULL,
  last_sequence BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (task_id),
  CONSTRAINT fk_task_event_seq_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Per-task event sequence counter';
