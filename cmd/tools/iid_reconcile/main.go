package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/domain"
)

type iidSourceRow struct {
	Source  string
	Raw     string
	Count   int64
	SKUCode string
	TaskNo  string
}

type reconcileCandidate struct {
	Raw        string
	Normalized string
	Source     string
	Count      int64
	SKUCode    string
	TaskNo     string
}

func main() {
	var dsn, outDir string
	var limit int
	flag.StringVar(&dsn, "dsn", os.Getenv("MYSQL_DSN"), "MySQL DSN; defaults to MYSQL_DSN")
	flag.StringVar(&outDir, "out", "tmp/iid_reconcile", "directory for CSV output")
	flag.IntVar(&limit, "limit", 5000, "max rows per source query")
	flag.Parse()

	if strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(os.Stderr, "MYSQL_DSN or --dsn is required")
		os.Exit(2)
	}
	if limit <= 0 {
		limit = 5000
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping db: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}

	erp, err := loadERPStyleCodes(ctx, db, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load erp iids: %v\n", err)
		os.Exit(1)
	}
	candidates, err := loadCandidateIIDs(ctx, db, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load candidate iids: %v\n", err)
		os.Exit(1)
	}
	fallback, err := loadLegacyFallbackTop(ctx, db, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load legacy fallback rows: %v\n", err)
		os.Exit(1)
	}

	exact, normalizable, dirty := classifyCandidates(candidates, erp)
	outputs := map[string][][]string{
		"exact_matches.csv":        rowsForCandidates(exact),
		"normalizable_matches.csv": rowsForCandidates(normalizable),
		"dirty_values.csv":         rowsForCandidates(dirty),
		"legacy_alias_top.csv":     rowsForFallback(fallback),
	}
	for name, rows := range outputs {
		if err := writeCSV(filepath.Join(outDir, name), rows); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	fmt.Printf("iid reconcile complete: exact=%d normalizable=%d dirty=%d legacy_alias_top=%d out=%s\n",
		len(exact), len(normalizable), len(dirty), len(fallback), outDir)
}

func loadERPStyleCodes(ctx context.Context, db *sql.DB, limit int) (map[string]map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT raw_iid, COUNT(*) AS count_value
		  FROM (
		        SELECT NULLIF(TRIM(i_id), '') AS raw_iid FROM products
		        UNION ALL
		        SELECT NULLIF(TRIM(product_i_id), '') AS raw_iid FROM erp_product_sync_records
		        UNION ALL
		        SELECT NULLIF(TRIM(erp_i_id), '') AS raw_iid FROM omp_sku_records
		       ) x
		 WHERE raw_iid IS NOT NULL
		 GROUP BY raw_iid
		 ORDER BY count_value DESC, raw_iid
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]struct{}{}
	for rows.Next() {
		var raw string
		var count int64
		if err := rows.Scan(&raw, &count); err != nil {
			return nil, err
		}
		normalized := domain.NormalizeIID(raw)
		if normalized == "" {
			continue
		}
		if out[normalized] == nil {
			out[normalized] = map[string]struct{}{}
		}
		out[normalized][strings.TrimSpace(raw)] = struct{}{}
	}
	return out, rows.Err()
}

func loadCandidateIIDs(ctx context.Context, db *sql.DB, limit int) ([]reconcileCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source_name, raw_iid, COUNT(*) AS count_value,
		       COALESCE(MIN(NULLIF(sku_code, '')), '') AS sku_code,
		       COALESCE(MIN(NULLIF(task_no, '')), '') AS task_no
		  FROM (
		        SELECT 'task_sku_items.product_i_id' AS source_name,
		               NULLIF(TRIM(product_i_id), '') AS raw_iid,
		               sku_code, '' AS task_no
		          FROM task_sku_items
		        UNION ALL
		        SELECT 'task_sku_items.variant_json.product_i_id',
		               NULLIF(TRIM(CASE WHEN JSON_VALID(variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(variant_json, '$.product_i_id')) ELSE '' END), ''),
		               sku_code, ''
		          FROM task_sku_items
		        UNION ALL
		        SELECT 'task_sku_items.variant_json.i_id',
		               NULLIF(TRIM(CASE WHEN JSON_VALID(variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(variant_json, '$.i_id')) ELSE '' END), ''),
		               sku_code, ''
		          FROM task_sku_items
		        UNION ALL
		        SELECT 'omp_sku_records.product_i_id', NULLIF(TRIM(product_i_id), ''), sku_code, task_no
		          FROM omp_sku_records
		        UNION ALL
		        SELECT 'omp_sku_records.erp_i_id', NULLIF(TRIM(erp_i_id), ''), sku_code, task_no
		          FROM omp_sku_records
		        UNION ALL
		        SELECT 'erp_product_sync_records.product_i_id', NULLIF(TRIM(product_i_id), ''), sku_code, task_no
		          FROM erp_product_sync_records
		       ) x
		 WHERE raw_iid IS NOT NULL
		 GROUP BY source_name, raw_iid
		 ORDER BY count_value DESC, source_name, raw_iid
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reconcileCandidate
	for rows.Next() {
		var row iidSourceRow
		if err := rows.Scan(&row.Source, &row.Raw, &row.Count, &row.SKUCode, &row.TaskNo); err != nil {
			return nil, err
		}
		normalized := domain.NormalizeIID(row.Raw)
		if normalized == "" {
			continue
		}
		out = append(out, reconcileCandidate{
			Raw:        strings.TrimSpace(row.Raw),
			Normalized: normalized,
			Source:     row.Source,
			Count:      row.Count,
			SKUCode:    row.SKUCode,
			TaskNo:     row.TaskNo,
		})
	}
	return out, rows.Err()
}

func loadLegacyFallbackTop(ctx context.Context, db *sql.DB, limit int) ([]reconcileCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(latest.calculation_snapshot_json, '$.erp_i_id')), ''),
		               NULLIF(JSON_UNQUOTE(JSON_EXTRACT(latest.calculation_snapshot_json, '$.product_i_id')), ''),
		               latest.product_i_id) AS raw_iid,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(latest.calculation_snapshot_json, '$.rule_group')), latest.category_code, '') AS rule_group,
		       COUNT(*) AS count_value,
		       COALESCE(MIN(NULLIF(latest.sku_code, '')), '') AS sku_code,
		       COALESCE(MIN(NULLIF(latest.task_no, '')), '') AS task_no
		  FROM omp_sku_cost_snapshots latest
		  JOIN (
		        SELECT sku_code, MAX(id) AS id
		          FROM omp_sku_cost_snapshots
		         GROUP BY sku_code
		       ) pick ON pick.id = latest.id
		 WHERE JSON_EXTRACT(latest.calculation_snapshot_json, '$.legacy_alias_fallback') = true
		    OR JSON_UNQUOTE(JSON_EXTRACT(latest.calculation_snapshot_json, '$.match_mode')) = 'legacy_alias'
		 GROUP BY raw_iid, rule_group
		 ORDER BY count_value DESC, raw_iid
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reconcileCandidate
	for rows.Next() {
		var raw, ruleGroup, skuCode, taskNo string
		var count int64
		if err := rows.Scan(&raw, &ruleGroup, &count, &skuCode, &taskNo); err != nil {
			return nil, err
		}
		normalized := domain.NormalizeIID(raw)
		if normalized == "" {
			continue
		}
		out = append(out, reconcileCandidate{
			Raw:        strings.TrimSpace(raw),
			Normalized: normalized,
			Source:     strings.TrimSpace(ruleGroup),
			Count:      count,
			SKUCode:    skuCode,
			TaskNo:     taskNo,
		})
	}
	return out, rows.Err()
}

func classifyCandidates(candidates []reconcileCandidate, erp map[string]map[string]struct{}) (exact, normalizable, dirty []reconcileCandidate) {
	for _, c := range candidates {
		rawSet := erp[c.Normalized]
		if len(rawSet) == 0 {
			dirty = append(dirty, c)
			continue
		}
		if _, ok := rawSet[c.Raw]; ok {
			exact = append(exact, c)
			continue
		}
		normalizable = append(normalizable, c)
	}
	sortCandidates(exact)
	sortCandidates(normalizable)
	sortCandidates(dirty)
	return exact, normalizable, dirty
}

func sortCandidates(items []reconcileCandidate) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Normalized < items[j].Normalized
		}
		return items[i].Count > items[j].Count
	})
}

func rowsForCandidates(items []reconcileCandidate) [][]string {
	rows := [][]string{{"raw_i_id", "normalized_i_id", "source", "count", "example_sku_code", "example_task_no"}}
	for _, item := range items {
		rows = append(rows, []string{
			item.Raw,
			item.Normalized,
			item.Source,
			strconv.FormatInt(item.Count, 10),
			item.SKUCode,
			item.TaskNo,
		})
	}
	return rows
}

func rowsForFallback(items []reconcileCandidate) [][]string {
	rows := [][]string{{"raw_i_id", "normalized_i_id", "suggested_rule_group", "match_count", "example_sku_code", "example_task_no"}}
	for _, item := range items {
		rows = append(rows, []string{
			item.Raw,
			item.Normalized,
			item.Source,
			strconv.FormatInt(item.Count, 10),
			item.SKUCode,
			item.TaskNo,
		})
	}
	return rows
}

func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return w.Error()
}
