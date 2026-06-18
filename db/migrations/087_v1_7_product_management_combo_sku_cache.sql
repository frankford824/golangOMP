-- Migration: 087_v1_7_product_management_combo_sku_cache.sql
-- Cache JST combine SKU parents and sync cursors for product-management hierarchy views.

CREATE TABLE IF NOT EXISTS omp_sku_combo_records (
  combo_sku_code VARCHAR(64) NOT NULL,
  name VARCHAR(512) NOT NULL DEFAULT '',
  short_name VARCHAR(255) NOT NULL DEFAULT '',
  erp_i_id VARCHAR(128) NOT NULL DEFAULT '',
  enabled TINYINT(1) NULL,
  cost_price DECIMAL(12,3) NULL,
  sale_price DECIMAL(12,3) NULL,
  modified_at DATETIME NULL,
  source VARCHAR(64) NOT NULL DEFAULT 'jst_openweb_combine_sku_query',
  raw_payload_json LONGTEXT NULL,
  last_synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (combo_sku_code),
  KEY idx_omp_sku_combo_records_iid (erp_i_id),
  KEY idx_omp_sku_combo_records_modified (modified_at),
  KEY idx_omp_sku_combo_records_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='JST combine SKU parent cache for OMP product-management hierarchy';

CREATE TABLE IF NOT EXISTS omp_sku_combo_sync_state (
  id BIGINT NOT NULL AUTO_INCREMENT,
  window_begin DATETIME NOT NULL,
  window_end DATETIME NOT NULL,
  page_index INT NOT NULL DEFAULT 1,
  page_size INT NOT NULL DEFAULT 50,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_success_at DATETIME NULL,
  next_retry_at DATETIME NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  processed_items INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_omp_sku_combo_sync_window (window_begin, window_end),
  KEY idx_omp_sku_combo_sync_status_retry (status, next_retry_at),
  KEY idx_omp_sku_combo_sync_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='JST combine SKU window sync cursor';

ALTER TABLE omp_sku_combo_relations
  ADD KEY idx_omp_sku_combo_combo (combo_sku_code);

-- ROLLBACK-BEGIN
ALTER TABLE omp_sku_combo_relations DROP KEY idx_omp_sku_combo_combo;
DROP TABLE IF EXISTS omp_sku_combo_sync_state;
DROP TABLE IF EXISTS omp_sku_combo_records;
-- ROLLBACK-END
