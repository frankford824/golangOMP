package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	productSearchNgramIndexName       = "product_ngram_v1"
	productSearchNgramIndexVersion    = 1
	productSearchLexicalTermLimit     = 256
	productSearchSemanticTermLimit    = 64
	productSearchQueryTermLimit       = 12
	productSearchNgramInsertBatchSize = 200
)

// ProductSearchNgramRebuild describes one atomic shadow-table rebuild.
type ProductSearchNgramRebuild struct {
	SourceDocuments int64 `json:"source_documents"`
	BeforeTerms     int64 `json:"before_terms"`
	AfterTerms      int64 `json:"after_terms"`
	Changed         bool  `json:"changed"`
}

// EnsureProductSearchNgramIndex is a no-op after a complete index generation
// has been activated. A missing readiness marker triggers a full shadow build.
func EnsureProductSearchNgramIndex(ctx context.Context, db *sql.DB) (ProductSearchNgramRebuild, error) {
	out := ProductSearchNgramRebuild{}
	if db == nil {
		return out, fmt.Errorf("database is required")
	}
	if productSearchNgramIndexReady(ctx, db) {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_search_documents`).Scan(&out.SourceDocuments); err != nil {
			return out, fmt.Errorf("count product search documents: %w", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_search_ngrams`).Scan(&out.BeforeTerms); err != nil {
			return out, fmt.Errorf("count product search ngrams: %w", err)
		}
		out.AfterTerms = out.BeforeTerms
		return out, nil
	}
	return RebuildProductSearchNgramIndex(ctx, db)
}

func productSearchNgramTableExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlTableExists(ctx, q, "product_search_ngrams")
}

func productSearchNgramIndexReady(ctx context.Context, q taskSearchDocumentSQL) bool {
	if !productSearchNgramTableExists(ctx, q) || !mysqlTableExists(ctx, q, "product_search_index_state") {
		return false
	}
	key := mysqlSchemaCacheKey{kind: "search-index-ready", table: productSearchNgramIndexName}
	if ready, ok := loadMySQLSchemaPresenceCache(key); ok {
		return ready
	}
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	var ready int
	err := q.QueryRowContext(queryCtx, `
		SELECT COUNT(*)
		  FROM product_search_index_state
		 WHERE index_name = ?
		   AND index_version = ?`, productSearchNgramIndexName, productSearchNgramIndexVersion).Scan(&ready)
	isReady := err == nil && ready == 1
	storeMySQLSchemaPresenceCache(key, isReady)
	return isReady
}

// InvalidateProductSearchNgramIndexTx makes readers use the bounded fallback
// while a complete product document rebuild is in progress.
func InvalidateProductSearchNgramIndexTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("product search ngram transaction is required")
	}
	if !mysqlTableExists(ctx, tx, "product_search_index_state") {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM product_search_index_state WHERE index_name = ?`, productSearchNgramIndexName); err != nil {
		return fmt.Errorf("invalidate product search ngram index: %w", err)
	}
	mysqlSchemaPresenceCache.Delete(mysqlSchemaCacheKey{kind: "search-index-ready", table: productSearchNgramIndexName})
	return nil
}

func productSearchQueryNgrams(raw string) []string {
	all := productSearchNgrams(raw, 0)
	if len(all) <= productSearchQueryTermLimit {
		return all
	}
	// Sample across the complete query instead of keeping only a prefix. This
	// bounds SQL placeholders while retaining evidence from every query region.
	out := make([]string, 0, productSearchQueryTermLimit)
	seen := make(map[string]struct{}, productSearchQueryTermLimit)
	for i := 0; i < productSearchQueryTermLimit; i++ {
		index := i * (len(all) - 1) / (productSearchQueryTermLimit - 1)
		term := all[index]
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	return out
}

func productSearchDocumentNgrams(searchText, semanticText string) []string {
	lexical := productSearchNgrams(searchText, productSearchLexicalTermLimit)
	semantic := productSearchNgrams(semanticText, productSearchSemanticTermLimit)
	seen := make(map[string]struct{}, len(lexical)+len(semantic))
	out := make([]string, 0, len(lexical)+len(semantic))
	for _, terms := range [][]string{lexical, semantic} {
		for _, term := range terms {
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			out = append(out, term)
		}
	}
	sort.Strings(out)
	return out
}

func productSearchNgrams(raw string, limit int) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 32)
	segment := make([]rune, 0, 32)
	flush := func() bool {
		for i := 0; i+1 < len(segment); i++ {
			term := string(segment[i : i+2])
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			out = append(out, term)
			if limit > 0 && len(out) >= limit {
				segment = segment[:0]
				return true
			}
		}
		segment = segment[:0]
		return false
	}
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			segment = append(segment, unicode.ToLower(r))
			continue
		}
		if flush() {
			return out
		}
	}
	flush()
	return out
}

func reindexProductSearchNgrams(ctx context.Context, q taskSearchDocumentSQL, skuCode string) error {
	skuCode = strings.TrimSpace(skuCode)
	if skuCode == "" || !productSearchNgramTableExists(ctx, q) {
		return nil
	}
	hasSemanticText := productSearchDocumentsSemanticTextExists(ctx, q)
	query := `SELECT COALESCE(search_text, ''), '' FROM product_search_documents WHERE sku_code = ?`
	if hasSemanticText {
		query = `SELECT COALESCE(search_text, ''), COALESCE(semantic_text, '') FROM product_search_documents WHERE sku_code = ?`
	}
	var searchText, semanticText string
	err := q.QueryRowContext(ctx, query, skuCode).Scan(&searchText, &semanticText)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load product search document for ngram index: %w", err)
	}
	if _, deleteErr := q.ExecContext(ctx, `DELETE FROM product_search_ngrams WHERE sku_code = ?`, skuCode); deleteErr != nil {
		return fmt.Errorf("delete product search ngrams: %w", deleteErr)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if err := insertProductSearchNgrams(ctx, q, "product_search_ngrams", skuCode, productSearchDocumentNgrams(searchText, semanticText)); err != nil {
		return fmt.Errorf("insert product search ngrams: %w", err)
	}
	return nil
}

// ReindexProductSearchNgramsTx refreshes the product term index in the same
// transaction as a semantic-text update.
func ReindexProductSearchNgramsTx(ctx context.Context, tx *sql.Tx, skuCode string) error {
	if tx == nil {
		return fmt.Errorf("product search ngram transaction is required")
	}
	return reindexProductSearchNgrams(ctx, tx, skuCode)
}

func insertProductSearchNgrams(ctx context.Context, q taskSearchDocumentSQL, table, skuCode string, terms []string) error {
	for start := 0; start < len(terms); start += productSearchNgramInsertBatchSize {
		end := start + productSearchNgramInsertBatchSize
		if end > len(terms) {
			end = len(terms)
		}
		values := make([]string, 0, end-start)
		args := make([]interface{}, 0, (end-start)*2)
		for _, term := range terms[start:end] {
			values = append(values, "(?, ?)")
			args = append(args, term, skuCode)
		}
		if _, err := q.ExecContext(ctx,
			"INSERT IGNORE INTO "+table+" (term, sku_code) VALUES "+strings.Join(values, ", "),
			args...,
		); err != nil {
			return err
		}
	}
	return nil
}

// RebuildProductSearchNgramIndex populates a shadow table, validates it, and
// swaps it into service with one atomic RENAME TABLE statement. Callers must
// hold the product-search write freeze while the rebuild is running.
func RebuildProductSearchNgramIndex(ctx context.Context, db *sql.DB) (ProductSearchNgramRebuild, error) {
	out := ProductSearchNgramRebuild{}
	if db == nil {
		return out, fmt.Errorf("database is required")
	}
	if !productSearchNgramTableExists(ctx, db) {
		return out, fmt.Errorf("product_search_ngrams table is missing")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_search_documents`).Scan(&out.SourceDocuments); err != nil {
		return out, fmt.Errorf("count product search documents: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_search_ngrams`).Scan(&out.BeforeTerms); err != nil {
		return out, fmt.Errorf("count product search ngrams: %w", err)
	}
	const buildTable = "product_search_ngrams_build"
	const staleTable = "product_search_ngrams_stale"
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS `+buildTable); err != nil {
		return out, fmt.Errorf("drop stale product ngram build table: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE `+buildTable+` LIKE product_search_ngrams`); err != nil {
		return out, fmt.Errorf("create product ngram build table: %w", err)
	}
	cleanupBuild := true
	defer func() {
		if cleanupBuild {
			_, _ = db.ExecContext(context.Background(), `DROP TABLE IF EXISTS `+buildTable)
		}
	}()

	hasSemanticText := productSearchDocumentsSemanticTextExists(ctx, db)
	query := `SELECT sku_code, COALESCE(search_text, ''), '' FROM product_search_documents ORDER BY sku_code`
	if hasSemanticText {
		query = `SELECT sku_code, COALESCE(search_text, ''), COALESCE(semantic_text, '') FROM product_search_documents ORDER BY sku_code`
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return out, fmt.Errorf("stream product search documents: %w", err)
	}
	for rows.Next() {
		var skuCode, searchText, semanticText string
		if err := rows.Scan(&skuCode, &searchText, &semanticText); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan product search document: %w", err)
		}
		if err := insertProductSearchNgrams(ctx, db, buildTable, skuCode, productSearchDocumentNgrams(searchText, semanticText)); err != nil {
			rows.Close()
			return out, fmt.Errorf("populate product ngram build table: %w", err)
		}
	}
	if err := rows.Close(); err != nil {
		return out, fmt.Errorf("close product search document stream: %w", err)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iterate product search documents: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+buildTable).Scan(&out.AfterTerms); err != nil {
		return out, fmt.Errorf("count rebuilt product search ngrams: %w", err)
	}
	if out.SourceDocuments > 0 && out.AfterTerms == 0 {
		return out, fmt.Errorf("rebuilt product ngram index is empty for %d documents", out.SourceDocuments)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS `+staleTable); err != nil {
		return out, fmt.Errorf("drop stale product ngram table: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		RENAME TABLE
		  product_search_ngrams TO `+staleTable+`,
		  `+buildTable+` TO product_search_ngrams`); err != nil {
		return out, fmt.Errorf("activate rebuilt product ngram index: %w", err)
	}
	cleanupBuild = false
	if _, err := db.ExecContext(ctx, `DROP TABLE `+staleTable); err != nil {
		return out, fmt.Errorf("drop previous product ngram index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO product_search_index_state (
		  index_name, index_version, document_count, term_count, built_at
		) VALUES (?, ?, ?, ?, UTC_TIMESTAMP())
		ON DUPLICATE KEY UPDATE
		  index_version = VALUES(index_version),
		  document_count = VALUES(document_count),
		  term_count = VALUES(term_count),
		  built_at = VALUES(built_at)`,
		productSearchNgramIndexName, productSearchNgramIndexVersion, out.SourceDocuments, out.AfterTerms,
	); err != nil {
		return out, fmt.Errorf("mark product ngram index ready: %w", err)
	}
	mysqlSchemaPresenceCache.Delete(mysqlSchemaCacheKey{kind: "search-index-ready", table: productSearchNgramIndexName})
	out.Changed = true
	return out, nil
}
