-- Migration: 010_v7_category_cost_rule_skeleton.sql
-- Adds category center skeleton, cost-rule skeleton, and task-detail linkage fields.

CREATE TABLE IF NOT EXISTS categories (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  category_code VARCHAR(64)  NOT NULL,
  category_name VARCHAR(255) NOT NULL DEFAULT '',
  display_name  VARCHAR(255) NOT NULL DEFAULT '',
  parent_id     BIGINT       NULL,
  level_no      INT          NOT NULL DEFAULT 1,
  category_type VARCHAR(64)  NOT NULL DEFAULT 'other',
  is_active     TINYINT(1)   NOT NULL DEFAULT 1,
  sort_order    INT          NOT NULL DEFAULT 0,
  source        VARCHAR(128) NOT NULL DEFAULT '',
  remark        TEXT         NOT NULL,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_categories_code (category_code),
  KEY idx_categories_parent (parent_id),
  KEY idx_categories_type_active_sort (category_type, is_active, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Configurable first-level category center skeleton';

CREATE TABLE IF NOT EXISTS cost_rules (
  id                      BIGINT         NOT NULL AUTO_INCREMENT,
  rule_name               VARCHAR(255)   NOT NULL,
  category_id             BIGINT         NULL,
  category_code           VARCHAR(64)    NOT NULL DEFAULT '',
  product_family          VARCHAR(128)   NOT NULL DEFAULT '',
  rule_type               VARCHAR(64)    NOT NULL,
  base_price              DECIMAL(10,2)  NULL,
  tax_multiplier          DECIMAL(10,4)  NULL,
  min_area                DECIMAL(10,4)  NULL,
  area_threshold          DECIMAL(10,4)  NULL,
  surcharge_amount        DECIMAL(10,2)  NULL,
  special_process_keyword VARCHAR(128)   NOT NULL DEFAULT '',
  special_process_price   DECIMAL(10,2)  NULL,
  formula_expression      VARCHAR(512)   NOT NULL DEFAULT '',
  priority                INT            NOT NULL DEFAULT 100,
  is_active               TINYINT(1)     NOT NULL DEFAULT 1,
  effective_from          DATETIME       NULL,
  effective_to            DATETIME       NULL,
  source                  VARCHAR(128)   NOT NULL DEFAULT '',
  remark                  TEXT           NOT NULL,
  created_at              DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at              DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_cost_rules_seed_identity (rule_name, category_code, rule_type, priority, source, special_process_keyword),
  KEY idx_cost_rules_category_active_priority (category_code, is_active, priority),
  KEY idx_cost_rules_type (rule_type),
  KEY idx_cost_rules_effective_window (effective_from, effective_to)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Configurable cost-rule skeleton';

ALTER TABLE task_details
  ADD COLUMN category_id BIGINT NULL AFTER category,
  ADD COLUMN category_code VARCHAR(64) NOT NULL DEFAULT '' AFTER category_id,
  ADD COLUMN category_name VARCHAR(255) NOT NULL DEFAULT '' AFTER category_code,
  ADD COLUMN cost_rule_id BIGINT NULL AFTER cost_price,
  ADD COLUMN cost_rule_name VARCHAR(255) NOT NULL DEFAULT '' AFTER cost_rule_id,
  ADD COLUMN cost_rule_source VARCHAR(255) NOT NULL DEFAULT '' AFTER cost_rule_name;

CREATE INDEX idx_task_details_category_code ON task_details (category_code);
CREATE INDEX idx_task_details_cost_rule_id ON task_details (cost_rule_id);

INSERT IGNORE INTO categories (
  category_code, category_name, display_name, parent_id, level_no, category_type, is_active, sort_order, source, remark
) VALUES
  ('KT_STANDARD', '常规kt板', '常规kt板', NULL, 1, 'board', 1, 10, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_CUSTOM', '定制kt板', '定制kt板', NULL, 1, 'board', 1, 20, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_HENAN', '河南kt板', '河南kt板', NULL, 1, 'board', 1, 30, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_RED', '红色kt板', '红色kt板', NULL, 1, 'board', 1, 40, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_GOLD', '金色kt板', '金色kt板', NULL, 1, 'board', 1, 50, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_STANDARD_FILM', '常规kt板(覆膜)', '常规kt板(覆膜)', NULL, 1, 'board', 1, 60, 'phase_020_sample', '一级总分类入口样例'),
  ('KT_CUSTOM_FILM', '定制kt板(覆膜)', '定制kt板(覆膜)', NULL, 1, 'board', 1, 70, 'phase_020_sample', '一级总分类入口样例'),
  ('PHOTO_CLOTH_STANDARD', '常规写真布', '常规写真布', NULL, 1, 'cloth', 1, 80, 'phase_020_sample', '一级总分类入口样例'),
  ('PHOTO_CLOTH_CUSTOM', '定制写真布', '定制写真布', NULL, 1, 'cloth', 1, 90, 'phase_020_sample', '一级总分类入口样例'),
  ('FLAG_CLOTH_STANDARD', '常规旗帜布', '常规旗帜布', NULL, 1, 'cloth', 1, 100, 'phase_020_sample', '一级总分类入口样例'),
  ('FLAG_CLOTH_SEWED', '车缝旗帜布', '车缝旗帜布', NULL, 1, 'cloth', 1, 110, 'phase_020_sample', '一级总分类入口样例'),
  ('SPRAY_CLOTH_STANDARD', '常规喷绘布', '常规喷绘布', NULL, 1, 'cloth', 1, 120, 'phase_020_sample', '一级总分类入口样例'),
  ('SPRAY_CLOTH_CUSTOM', '定制喷绘布', '定制喷绘布', NULL, 1, 'cloth', 1, 130, 'phase_020_sample', '一级总分类入口样例'),
  ('PENNANT_CUSTOM', '定制锦旗', '定制锦旗', NULL, 1, 'cloth', 1, 140, 'phase_020_sample', '一级总分类入口样例'),
  ('A3_PRINT', 'A3纸打印', 'A3纸打印', NULL, 1, 'paper', 1, 150, 'phase_020_sample', '一级总分类入口样例'),
  ('A4_PRINT', 'A4纸打印', 'A4纸打印', NULL, 1, 'paper', 1, 160, 'phase_020_sample', '一级总分类入口样例'),
  ('COPPER_PAPER', '铜版纸', '铜版纸', NULL, 1, 'paper', 1, 170, 'phase_020_sample', '一级总分类入口样例'),
  ('WHITE_CARD', '白卡纸', '白卡纸', NULL, 1, 'paper', 1, 180, 'phase_020_sample', '一级总分类入口样例'),
  ('DIECUT_STICKER', '模切不干胶', '模切不干胶', NULL, 1, 'paper', 1, 190, 'phase_020_sample', '一级总分类入口样例'),
  ('PP_STICKY', 'PP纸背胶', 'PP纸背胶', NULL, 1, 'paper', 1, 200, 'phase_020_sample', '一级总分类入口样例'),
  ('PP_PLAIN', 'PP纸无背胶', 'PP纸无背胶', NULL, 1, 'paper', 1, 210, 'phase_020_sample', '一级总分类入口样例'),
  ('ACRYLIC', '亚克力', '亚克力', NULL, 1, 'material', 1, 220, 'phase_020_sample', '人工报价材料类'),
  ('LEAF_CARVING', '叶雕', '叶雕', NULL, 1, 'material', 1, 230, 'phase_020_sample', '人工报价材料类'),
  ('HBJ', 'HBJ', 'HBJ', NULL, 1, 'coded_style', 1, 240, 'phase_020_sample', '一级总分类编码入口'),
  ('HBZ', 'HBZ', 'HBZ', NULL, 1, 'coded_style', 1, 250, 'phase_020_sample', '一级总分类编码入口'),
  ('HCP', 'HCP', 'HCP', NULL, 1, 'coded_style', 1, 260, 'phase_020_sample', '一级总分类编码入口'),
  ('HLZ', 'HLZ', 'HLZ', NULL, 1, 'coded_style', 1, 270, 'phase_020_sample', '一级总分类编码入口'),
  ('HPJ', 'HPJ', 'HPJ', NULL, 1, 'coded_style', 1, 280, 'phase_020_sample', '一级总分类编码入口'),
  ('HQT', 'HQT', 'HQT', NULL, 1, 'coded_style', 1, 290, 'phase_020_sample', '一级总分类编码入口'),
  ('HSC', 'HSC', 'HSC', NULL, 1, 'coded_style', 1, 300, 'phase_020_sample', '一级总分类编码入口'),
  ('HZS', 'HZS', 'HZS', NULL, 1, 'coded_style', 1, 310, 'phase_020_sample', '一级总分类编码入口');

INSERT IGNORE INTO cost_rules (
  rule_name, category_code, product_family, rule_type, base_price, tax_multiplier, min_area,
  area_threshold, surcharge_amount, special_process_keyword, special_process_price, formula_expression,
  priority, is_active, source, remark
) VALUES
  ('常规KT板基础单价', 'KT_STANDARD', 'board', 'fixed_unit_price', 11.00, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '按面积试算的基础单价样例'),
  ('常规KT板小面积附加', 'KT_STANDARD', 'board', 'area_threshold_surcharge', NULL, NULL, NULL, 0.1500, 3.00, '', NULL, '', 20, 1, 'phase_020_sample', '面积低于阈值加价'),
  ('常规KT板开槽拼接加价', 'KT_STANDARD', 'board', 'special_process_surcharge', NULL, NULL, NULL, NULL, NULL, '开槽拼接', 1.00, '', 30, 1, 'phase_020_sample', '特殊工艺加价'),
  ('定制KT板基础单价', 'KT_CUSTOM', 'board', 'fixed_unit_price', 11.00, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '按面积试算的基础单价样例'),
  ('定制KT板小面积附加', 'KT_CUSTOM', 'board', 'area_threshold_surcharge', NULL, NULL, NULL, 0.1500, 3.00, '', NULL, '', 20, 1, 'phase_020_sample', '面积低于阈值加价'),
  ('河南KT板最小计费面积', 'KT_HENAN', 'board', 'minimum_billable_area', NULL, 1.1000, 3.0000, NULL, NULL, '', NULL, '', 5, 1, 'phase_020_sample', '最小计费面积样例'),
  ('河南KT板基础单价', 'KT_HENAN', 'board', 'fixed_unit_price', 11.00, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '最小面积调整后再按面积计价'),
  ('红色KT板基础单价', 'KT_RED', 'board', 'fixed_unit_price', 4.50, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '固定单价样例'),
  ('金色KT板基础单价', 'KT_GOLD', 'board', 'fixed_unit_price', 6.50, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '固定单价样例'),
  ('常规覆膜KT板基础单价', 'KT_STANDARD_FILM', 'board', 'fixed_unit_price', 13.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '覆膜基础单价'),
  ('常规覆膜KT板小面积保底', 'KT_STANDARD_FILM', 'board', 'minimum_billable_area', NULL, NULL, 0.1500, NULL, NULL, '', NULL, '', 5, 1, 'phase_020_sample', '覆膜小面积保底骨架'),
  ('定制覆膜KT板基础单价', 'KT_CUSTOM_FILM', 'board', 'fixed_unit_price', 13.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '覆膜基础单价'),
  ('定制覆膜KT板小面积保底', 'KT_CUSTOM_FILM', 'board', 'minimum_billable_area', NULL, NULL, 0.1500, NULL, NULL, '', NULL, '', 5, 1, 'phase_020_sample', '覆膜小面积保底骨架'),
  ('常规写真布基础单价', 'PHOTO_CLOTH_STANDARD', 'cloth', 'fixed_unit_price', 5.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '布类基础单价'),
  ('常规写真布小面积附加', 'PHOTO_CLOTH_STANDARD', 'cloth', 'area_threshold_surcharge', NULL, NULL, NULL, 0.1500, 3.00, '', NULL, '', 20, 1, 'phase_020_sample', '面积低于阈值加价'),
  ('定制写真布基础单价', 'PHOTO_CLOTH_CUSTOM', 'cloth', 'fixed_unit_price', 5.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '布类基础单价'),
  ('定制写真布小面积附加', 'PHOTO_CLOTH_CUSTOM', 'cloth', 'area_threshold_surcharge', NULL, NULL, NULL, 0.1500, 3.00, '', NULL, '', 20, 1, 'phase_020_sample', '面积低于阈值加价'),
  ('常规旗帜布基础单价', 'FLAG_CLOTH_STANDARD', 'cloth', 'fixed_unit_price', 4.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '固定单价样例'),
  ('车缝旗帜布基础单价', 'FLAG_CLOTH_SEWED', 'cloth', 'fixed_unit_price', 6.00, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '固定单价样例'),
  ('A3打印单双面规则', 'A3_PRINT', 'paper', 'size_based_formula', NULL, NULL, NULL, NULL, NULL, '', NULL, 'print_side:single=0.5,double=0.6', 10, 1, 'phase_020_sample', '窄范围公式样例'),
  ('A4打印单双面规则', 'A4_PRINT', 'paper', 'size_based_formula', NULL, NULL, NULL, NULL, NULL, '', NULL, 'print_side:single=0.3,double=0.4', 10, 1, 'phase_020_sample', '窄范围公式样例'),
  ('铜版纸尺寸规则骨架', 'COPPER_PAPER', 'paper', 'size_based_formula', NULL, NULL, NULL, NULL, NULL, '', NULL, 'size_lookup_required', 10, 1, 'phase_020_sample', '尺寸判定骨架，当前仍需人工补充'),
  ('白卡纸尺寸规则骨架', 'WHITE_CARD', 'paper', 'size_based_formula', NULL, NULL, NULL, NULL, NULL, '', NULL, 'size_lookup_required', 10, 1, 'phase_020_sample', '尺寸判定骨架，当前仍需人工补充'),
  ('模切不干胶人工报价', 'DIECUT_STICKER', 'paper', 'manual_quote', NULL, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '规则待补前先归入人工报价'),
  ('亚克力人工报价', 'ACRYLIC', 'material', 'manual_quote', NULL, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '人工判定类'),
  ('叶雕人工报价', 'LEAF_CARVING', 'material', 'manual_quote', NULL, NULL, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '人工判定类');
