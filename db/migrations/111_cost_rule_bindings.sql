-- Cost rule bindings: canonical ERP i_id -> internal cost rule group.

CREATE TABLE IF NOT EXISTS cost_rule_bindings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  i_id_raw VARCHAR(255) NOT NULL DEFAULT '',
  normalized_i_id VARCHAR(255) NOT NULL,
  normalized_i_id_active VARCHAR(255)
    GENERATED ALWAYS AS (IF(is_active = 1, normalized_i_id, NULL)) STORED,
  rule_group VARCHAR(64) NOT NULL,
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  source VARCHAR(64) NOT NULL DEFAULT 'manual',
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  created_by BIGINT NULL,
  updated_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_cost_rule_bindings_active_iid (normalized_i_id_active),
  KEY idx_cost_rule_bindings_rule_group (rule_group),
  KEY idx_cost_rule_bindings_active_group (is_active, rule_group),
  KEY idx_cost_rule_bindings_created_by (created_by),
  KEY idx_cost_rule_bindings_updated_by (updated_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Canonical ERP i_id bindings to existing cost_rules.category_code rule groups';
