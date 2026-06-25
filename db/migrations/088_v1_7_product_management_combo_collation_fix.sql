-- Migration: 088_v1_7_product_management_combo_collation_fix.sql
-- Keep combo parent cache collation aligned with the existing relation table
-- from 079 so JOIN and keyword search do not fail on MySQL 8 deployments.

ALTER TABLE omp_sku_combo_records
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

ALTER TABLE omp_sku_combo_sync_state
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- ROLLBACK-BEGIN
ALTER TABLE omp_sku_combo_records
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

ALTER TABLE omp_sku_combo_sync_state
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- ROLLBACK-END
