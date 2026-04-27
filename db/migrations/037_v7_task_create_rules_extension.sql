-- Migration: 037_v7_task_create_rules_extension.sql
-- Adds task-type-specific creation fields for the three task types:
--   original_product_development, new_product_development, purchase_task
-- Also adds owner_team (required for all three) and related business fields.

ALTER TABLE tasks
  ADD COLUMN owner_team     VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Required team name, validated against configured teams',
  ADD COLUMN is_outsource   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Outsource flag at creation time';

ALTER TABLE task_details
  ADD COLUMN change_request      TEXT         NOT NULL COMMENT 'Original product modification requirement',
  ADD COLUMN design_requirement  TEXT         NOT NULL COMMENT 'New product design requirement',
  ADD COLUMN product_short_name  VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'New product short name',
  ADD COLUMN material_mode       VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'preset or other',
  ADD COLUMN material_other      VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Custom material when material_mode=other',
  ADD COLUMN cost_price_mode     VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'manual or template',
  ADD COLUMN base_sale_price     DECIMAL(12,2) NULL    COMMENT 'Base sale price',
  ADD COLUMN product_channel     VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Product channel for purchase task',
  ADD COLUMN reference_images_json TEXT       NOT NULL COMMENT 'JSON array of reference image URLs',
  ADD COLUMN reference_link      TEXT         NOT NULL COMMENT 'Product reference link';
