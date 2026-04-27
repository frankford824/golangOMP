-- Migration: 048_v7_product_code_sequences.sql
-- Category-scoped allocator for default task product-code generation.

CREATE TABLE IF NOT EXISTS product_code_sequences (
  id BIGINT NOT NULL AUTO_INCREMENT,
  prefix VARCHAR(16) NOT NULL DEFAULT 'NS',
  category_code VARCHAR(64) NOT NULL,
  next_value BIGINT NOT NULL DEFAULT 0 COMMENT 'Next sequence to allocate (first allocation starts from 0)',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_product_code_sequences_prefix_category (prefix, category_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Category-level product-code allocator';

