-- Scalable lexical index for product global search.
--
-- The table intentionally starts empty. The packaged search_reindex tool builds
-- a shadow index after pending migrations and before live symlinks are switched.
-- Until the ready marker is written, the backend retains the bounded legacy
-- search path, so applying this additive migration is backward compatible.

CREATE TABLE IF NOT EXISTS product_search_ngrams (
  term VARCHAR(8) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  sku_code VARCHAR(64) NOT NULL,
  PRIMARY KEY (term, sku_code),
  KEY idx_product_search_ngrams_sku (sku_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='BTREE bigram inverted index for product global search';

CREATE TABLE IF NOT EXISTS product_search_index_state (
  index_name VARCHAR(64) NOT NULL,
  index_version INT NOT NULL,
  document_count BIGINT NOT NULL DEFAULT 0,
  term_count BIGINT NOT NULL DEFAULT 0,
  built_at DATETIME NOT NULL,
  PRIMARY KEY (index_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Readiness marker for atomically rebuilt search indexes';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS product_search_ngrams_build;
DROP TABLE IF EXISTS product_search_ngrams_stale;
DROP TABLE IF EXISTS product_search_index_state;
DROP TABLE IF EXISTS product_search_ngrams;
-- ROLLBACK-END
