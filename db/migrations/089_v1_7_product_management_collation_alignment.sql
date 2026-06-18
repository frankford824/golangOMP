-- Migration: 089_v1_7_product_management_collation_alignment.sql
-- Align product-management read model text columns with combo relation cache.
-- Product center keyword search joins erp_product_sync_records.sku_code to
-- omp_sku_combo_relations.child_sku_code, so both tables must use the same
-- MySQL 8 collation to avoid Error 1267 during COUNT/LIST queries.

ALTER TABLE erp_product_sync_records
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- ROLLBACK-BEGIN
ALTER TABLE erp_product_sync_records
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- ROLLBACK-END
