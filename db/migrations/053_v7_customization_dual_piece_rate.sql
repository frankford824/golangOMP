-- Migration: 053_v7_customization_dual_piece_rate.sql
-- Add minimal dual piece-rate pricing support for customization operators.

ALTER TABLE users
  ADD COLUMN employment_type VARCHAR(32) NOT NULL DEFAULT 'full_time' COMMENT 'full_time | part_time';

ALTER TABLE customization_jobs
  ADD COLUMN pricing_worker_type VARCHAR(32) NULL COMMENT 'pricing snapshot worker type: full_time | part_time' AFTER last_operator_id;

CREATE TABLE IF NOT EXISTS customization_pricing_rules (
  id BIGINT NOT NULL AUTO_INCREMENT,
  customization_level_code VARCHAR(64) NOT NULL,
  employment_type VARCHAR(32) NOT NULL COMMENT 'full_time | part_time',
  unit_price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  weight_factor DECIMAL(10,4) NOT NULL DEFAULT 1.0000,
  is_enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_customization_pricing_rules_level_worker (customization_level_code, employment_type),
  KEY idx_customization_pricing_rules_enabled (is_enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Customization dual piece-rate pricing rules by level and employment type';
