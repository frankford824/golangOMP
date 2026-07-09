-- Optional semantic enrichment columns for search document read models.

SET @has_asset_semantic_text := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'asset_search_documents'
      AND column_name = 'semantic_text'
);
SET @sql := IF(
    @has_asset_semantic_text = 0,
    'ALTER TABLE asset_search_documents ADD COLUMN semantic_text TEXT NULL AFTER search_text, ADD COLUMN semantic_enriched_at DATETIME NULL AFTER semantic_text',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_product_semantic_text := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'product_search_documents'
      AND column_name = 'semantic_text'
);
SET @sql := IF(
    @has_product_semantic_text = 0,
    'ALTER TABLE product_search_documents ADD COLUMN semantic_text TEXT NULL AFTER search_text, ADD COLUMN semantic_enriched_at DATETIME NULL AFTER semantic_text',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_asset_semantic_ft := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'asset_search_documents'
      AND index_name = 'ft_asset_search_documents_semantic'
);
SET @sql := IF(
    @has_asset_semantic_ft = 0,
    'ALTER TABLE asset_search_documents ADD FULLTEXT KEY ft_asset_search_documents_semantic (semantic_text) WITH PARSER ngram',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_product_semantic_ft := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'product_search_documents'
      AND index_name = 'ft_product_search_documents_semantic'
);
SET @sql := IF(
    @has_product_semantic_ft = 0,
    'ALTER TABLE product_search_documents ADD FULLTEXT KEY ft_product_search_documents_semantic (semantic_text) WITH PARSER ngram',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ROLLBACK-BEGIN
ALTER TABLE product_search_documents DROP INDEX ft_product_search_documents_semantic;
ALTER TABLE asset_search_documents DROP INDEX ft_asset_search_documents_semantic;
ALTER TABLE product_search_documents DROP COLUMN semantic_enriched_at, DROP COLUMN semantic_text;
ALTER TABLE asset_search_documents DROP COLUMN semantic_enriched_at, DROP COLUMN semantic_text;
-- ROLLBACK-END
