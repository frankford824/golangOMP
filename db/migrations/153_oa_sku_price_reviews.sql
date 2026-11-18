-- OA operator confirmation and immutable per-SKU evidence.
CREATE TABLE IF NOT EXISTS oa_sku_price_reviews (
    id BIGINT NOT NULL AUTO_INCREMENT,
    batch_id BIGINT NULL,
    sku_code VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL,
    previous_review_id BIGINT NULL,
    evidence_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    operator_id BIGINT NULL,
    operator_name VARCHAR(255) NULL,
    request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    reason VARCHAR(200) NULL,
    evidence JSON NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_oa_price_review_evidence (sku_code, evidence_hash),
    UNIQUE KEY uq_oa_price_review_successor (sku_code, previous_review_id),
    KEY idx_oa_price_review_sku (sku_code, id),
    UNIQUE KEY uq_oa_price_review_request (operator_id, request_id),
    KEY idx_oa_price_review_batch (batch_id),
    CONSTRAINT fk_oa_price_review_batch FOREIGN KEY (batch_id) REFERENCES oa_sku_price_reviews (id),
    CONSTRAINT chk_oa_price_review_record CHECK (
        (batch_id IS NULL AND sku_code IS NULL AND previous_review_id IS NULL AND evidence_hash IS NULL AND operator_id IS NOT NULL AND operator_name IS NOT NULL
            AND request_id IS NOT NULL AND request_hash IS NOT NULL AND reason IS NOT NULL AND evidence IS NULL)
        OR
        (batch_id IS NOT NULL AND sku_code IS NOT NULL AND previous_review_id IS NOT NULL AND previous_review_id >= 0 AND evidence IS NOT NULL AND evidence_hash IS NOT NULL
            AND operator_id IS NULL AND operator_name IS NULL AND request_id IS NULL AND request_hash IS NULL AND reason IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
