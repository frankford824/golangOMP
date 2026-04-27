-- Migration: 012_v7_category_erp_mapping_skeleton.sql
-- Makes first-level ERP search-entry semantics explicit and adds category-to-ERP mapping skeleton.

ALTER TABLE categories
  ADD COLUMN search_entry_code VARCHAR(64) NOT NULL DEFAULT '' AFTER level_no,
  ADD COLUMN is_search_entry TINYINT(1) NOT NULL DEFAULT 1 AFTER search_entry_code;

UPDATE categories
SET search_entry_code = category_code,
    is_search_entry = CASE WHEN level_no = 1 THEN 1 ELSE 0 END
WHERE search_entry_code = '';

CREATE INDEX idx_categories_search_entry ON categories (search_entry_code, is_active, sort_order);

CREATE TABLE IF NOT EXISTS category_erp_mappings (
  id                        BIGINT       NOT NULL AUTO_INCREMENT,
  category_id               BIGINT       NULL,
  category_code             VARCHAR(64)  NOT NULL DEFAULT '',
  search_entry_code         VARCHAR(64)  NOT NULL DEFAULT '',
  erp_match_type            VARCHAR(64)  NOT NULL,
  erp_match_value           VARCHAR(255) NOT NULL DEFAULT '',
  secondary_condition_key   VARCHAR(64)  NOT NULL DEFAULT '',
  secondary_condition_value VARCHAR(255) NOT NULL DEFAULT '',
  tertiary_condition_key    VARCHAR(64)  NOT NULL DEFAULT '',
  tertiary_condition_value  VARCHAR(255) NOT NULL DEFAULT '',
  is_primary                TINYINT(1)   NOT NULL DEFAULT 0,
  is_active                 TINYINT(1)   NOT NULL DEFAULT 1,
  priority                  INT          NOT NULL DEFAULT 100,
  source                    VARCHAR(128) NOT NULL DEFAULT '',
  remark                    TEXT         NOT NULL,
  created_at                DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at                DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_category_erp_mapping_identity (
    category_code,
    search_entry_code,
    erp_match_type,
    erp_match_value,
    priority,
    source
  ),
  KEY idx_category_erp_mapping_lookup (search_entry_code, is_active, is_primary, priority),
  KEY idx_category_erp_mapping_category (category_code, is_active, priority)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Category-to-ERP product positioning skeleton';

INSERT IGNORE INTO category_erp_mappings (
  category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
  secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
  is_primary, is_active, priority, source, remark
)
SELECT id, category_code, search_entry_code, 'product_family',
       CASE
         WHEN category_type IN ('board', 'cloth', 'paper', 'material') THEN category_type
         ELSE category_code
       END,
       '', '', '', '',
       1, 1, 100, 'phase_022_sample',
       'Step 22 sample ERP mapping skeleton seeded from first-level category search entry.'
FROM categories
WHERE level_no = 1;
