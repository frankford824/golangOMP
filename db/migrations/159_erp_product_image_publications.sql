CREATE TABLE IF NOT EXISTS erp_product_image_checks (
    sku_code VARCHAR(64) NOT NULL,
    next_check_at DATETIME(6) NOT NULL,
    lease_token VARCHAR(64) NOT NULL DEFAULT '',
    leased_until DATETIME(6) NULL,
    checked_at DATETIME(6) NULL,
    requested_task_id BIGINT NULL,
    requested_ref_id VARCHAR(64) NULL,
    last_error VARCHAR(512) NOT NULL DEFAULT '',
    PRIMARY KEY (sku_code),
    KEY idx_erp_product_image_checks_due (next_check_at, sku_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS erp_product_image_references (
    sku_code VARCHAR(64) NOT NULL,
    asset_version_id BIGINT NOT NULL,
    expires_at DATETIME(6) NULL,
    source_version_id BIGINT NULL,
    storage_ref_id VARCHAR(64) NULL,
    PRIMARY KEY (sku_code, asset_version_id),
    KEY idx_erp_product_image_references_version (asset_version_id),
    KEY idx_erp_product_image_references_source (source_version_id),
    KEY idx_erp_product_image_references_storage (storage_ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Seed the known historical selections conservatively. The first authoritative
-- readback replaces these pins with the exact references returned by ERP.
INSERT IGNORE INTO erp_product_image_references (sku_code, asset_version_id, source_version_id, storage_ref_id)
SELECT p.sku_code, a.id, a.source_asset_version_id, a.storage_ref_id
FROM erp_product_sync_records p JOIN task_assets a ON a.id = p.image_asset_version_id
WHERE p.sku_code <> ''
  AND (p.last_image_synced_at IS NOT NULL OR p.image_sync_status IN ('synced', 'syncing', 'cooling_down'));

-- Previously published OSS objects that predate versioned task assets retain a
-- short, signed proxy identity without creating a new task asset or revision.
CREATE TABLE IF NOT EXISTS erp_product_image_legacy_objects (
    id BIGINT NOT NULL AUTO_INCREMENT,
    sku_code VARCHAR(64) NOT NULL,
    object_key VARCHAR(1024) NOT NULL,
    object_key_hash BINARY(32) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_erp_product_image_legacy_key (sku_code, object_key_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
