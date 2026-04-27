-- Migration: 003_v7_warehouse_detail.sql
-- V7 Step-03: warehouse_receipts
-- Strategy: additive only; no V6/V7 table drops.
-- Engine: InnoDB, charset: utf8mb4

CREATE TABLE IF NOT EXISTS warehouse_receipts (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  task_id        BIGINT       NOT NULL,
  receipt_no     VARCHAR(64)  NOT NULL COMMENT 'System-generated warehouse receipt number',
  status         VARCHAR(32)  NOT NULL DEFAULT 'received'
                               COMMENT 'received | rejected | completed',
  receiver_id    BIGINT       NULL,
  received_at    DATETIME     NULL,
  completed_at   DATETIME     NULL,
  reject_reason  TEXT         NOT NULL,
  remark         TEXT         NOT NULL,
  created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_warehouse_receipts_receipt_no (receipt_no),
  UNIQUE KEY uq_warehouse_receipts_task_id (task_id),
  KEY idx_warehouse_receipts_status (status),
  CONSTRAINT fk_warehouse_receipts_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 warehouse receive/reject/complete record';
