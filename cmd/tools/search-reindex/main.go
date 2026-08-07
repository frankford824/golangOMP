package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/config"
	mysqlrepo "workflow/repo/mysql"
)

type reindexSummary struct {
	DryRun         bool                                 `json:"dry_run"`
	Tasks          *reindexTable                        `json:"tasks,omitempty"`
	Assets         *reindexTable                        `json:"assets,omitempty"`
	Products       *reindexTable                        `json:"products,omitempty"`
	ProductIndex   *mysqlrepo.ProductSearchNgramRebuild `json:"product_index,omitempty"`
	ElapsedMS      int64                                `json:"elapsed_ms"`
	GeneratedAtUTC string                               `json:"generated_at_utc"`
	Message        string                               `json:"message,omitempty"`
}

type reindexTable struct {
	SourceRows int64 `json:"source_rows"`
	BeforeRows int64 `json:"before_rows"`
	AfterRows  int64 `json:"after_rows,omitempty"`
	Changed    bool  `json:"changed"`
}

func main() {
	var dsn string
	var tasks bool
	var assets bool
	var products bool
	var ensureProductIndex bool
	var dryRun bool
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.BoolVar(&tasks, "tasks", false, "rebuild task_search_documents through the canonical task projection")
	flag.BoolVar(&assets, "assets", true, "rebuild task_asset_group_search_documents")
	flag.BoolVar(&products, "products", true, "rebuild product_search_documents")
	flag.BoolVar(&ensureProductIndex, "ensure-product-index", false, "build the scalable product ngram index only when it is not ready")
	flag.BoolVar(&dryRun, "dry-run", false, "count rows without rebuilding documents")
	flag.DurationVar(&timeout, "timeout", 10*time.Minute, "whole run timeout")
	flag.Parse()

	code := run(context.Background(), runOptions{
		DSN:                dsn,
		Tasks:              tasks,
		Assets:             assets,
		Products:           products,
		EnsureProductIndex: ensureProductIndex,
		DryRun:             dryRun,
		Timeout:            timeout,
	})
	os.Exit(code)
}

type runOptions struct {
	DSN                string
	Tasks              bool
	Assets             bool
	Products           bool
	EnsureProductIndex bool
	DryRun             bool
	Timeout            time.Duration
}

func run(parent context.Context, opts runOptions) int {
	start := time.Now()
	if !opts.Tasks && !opts.Assets && !opts.Products && !opts.EnsureProductIndex {
		writeError("at least one rebuild target or --ensure-product-index must be selected")
		return 2
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		writeError(fmt.Sprintf("load config: %v", err))
		return 1
	}
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		dsn = cfg.MySQL.DSN
	}
	if strings.TrimSpace(dsn) == "" {
		writeError("mysql dsn is required")
		return 2
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		writeError(fmt.Sprintf("open mysql: %v", err))
		return 1
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		writeError(fmt.Sprintf("ping mysql: %v", err))
		return 1
	}

	out := reindexSummary{
		DryRun:         opts.DryRun,
		GeneratedAtUTC: start.UTC().Format(time.RFC3339),
	}
	if opts.Tasks {
		rebuilt, err := mysqlrepo.RebuildAllTaskSearchDocumentProjections(ctx, db, opts.DryRun)
		if err != nil {
			writeRunError(&out, start, fmt.Sprintf("rebuild tasks: %v", err))
			return 1
		}
		out.Tasks = &reindexTable{SourceRows: rebuilt.SourceRows, BeforeRows: rebuilt.BeforeRows,
			AfterRows: rebuilt.AfterRows, Changed: rebuilt.Changed}
	}
	if opts.Assets {
		table, err := rebuildAssets(ctx, db, opts.DryRun)
		if err != nil {
			writeRunError(&out, start, fmt.Sprintf("rebuild assets: %v", err))
			return 1
		}
		out.Assets = table
	}
	if opts.Products {
		table, err := rebuildProducts(ctx, db, opts.DryRun)
		if err != nil {
			writeRunError(&out, start, fmt.Sprintf("rebuild products: %v", err))
			return 1
		}
		out.Products = table
		if !opts.DryRun {
			index, err := mysqlrepo.RebuildProductSearchNgramIndex(ctx, db)
			if err != nil {
				writeRunError(&out, start, fmt.Sprintf("rebuild product ngram index: %v", err))
				return 1
			}
			out.ProductIndex = &index
		}
	} else if opts.EnsureProductIndex && !opts.DryRun {
		index, err := mysqlrepo.EnsureProductSearchNgramIndex(ctx, db)
		if err != nil {
			writeRunError(&out, start, fmt.Sprintf("ensure product ngram index: %v", err))
			return 1
		}
		out.ProductIndex = &index
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	if opts.DryRun {
		out.Message = "dry run only; no search document rows changed"
	}
	writeJSON(out)
	return 0
}

func rebuildAssets(ctx context.Context, db *sql.DB, dryRun bool) (*reindexTable, error) {
	countSource := func(q rowCounter) (int64, error) {
		return countRows(ctx, q, `
		SELECT COUNT(*)
		  FROM task_asset_groups g
		 WHERE g.finalized_revision_id IS NOT NULL`)
	}
	out := &reindexTable{}
	if dryRun {
		var err error
		out.SourceRows, err = countSource(db)
		if err != nil {
			return nil, fmt.Errorf("count asset source rows: %w", err)
		}
		out.BeforeRows, err = countRows(ctx, db, `SELECT COUNT(*) FROM task_asset_group_search_documents`)
		if err != nil {
			return nil, fmt.Errorf("count asset documents: %w", err)
		}
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin asset transaction: %w", err)
	}
	defer tx.Rollback()
	out.SourceRows, err = countSource(tx)
	if err != nil {
		return nil, fmt.Errorf("count asset source rows: %w", err)
	}
	out.BeforeRows, err = countRows(ctx, tx, `SELECT COUNT(*) FROM task_asset_group_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count asset documents: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return nil, fmt.Errorf("set group_concat_max_len: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_asset_group_search_documents`); err != nil {
		return nil, fmt.Errorf("delete asset documents: %w", err)
	}
	if _, err := tx.ExecContext(ctx, assetReindexSQL); err != nil {
		return nil, fmt.Errorf("insert asset documents: %w", err)
	}
	afterRows, err := countRows(ctx, tx, `SELECT COUNT(*) FROM task_asset_group_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count rebuilt asset documents: %w", err)
	}
	if afterRows != out.SourceRows {
		return nil, fmt.Errorf("asset document row mismatch: source=%d after=%d", out.SourceRows, afterRows)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit asset transaction: %w", err)
	}
	out.AfterRows = afterRows
	out.Changed = true
	return out, nil
}

func rebuildProducts(ctx context.Context, db *sql.DB, dryRun bool) (*reindexTable, error) {
	countSource := func(q rowCounter) (int64, error) {
		return countRows(ctx, q, `SELECT COUNT(DISTINCT sku_code) FROM products WHERE COALESCE(sku_code, '') <> ''`)
	}
	out := &reindexTable{}
	if dryRun {
		var err error
		out.SourceRows, err = countSource(db)
		if err != nil {
			return nil, fmt.Errorf("count product source rows: %w", err)
		}
		out.BeforeRows, err = countRows(ctx, db, `SELECT COUNT(*) FROM product_search_documents`)
		if err != nil {
			return nil, fmt.Errorf("count product documents: %w", err)
		}
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin product transaction: %w", err)
	}
	defer tx.Rollback()
	out.SourceRows, err = countSource(tx)
	if err != nil {
		return nil, fmt.Errorf("count product source rows: %w", err)
	}
	out.BeforeRows, err = countRows(ctx, tx, `SELECT COUNT(*) FROM product_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count product documents: %w", err)
	}
	if err := mysqlrepo.InvalidateProductSearchNgramIndexTx(ctx, tx); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM product_search_documents`); err != nil {
		return nil, fmt.Errorf("delete product documents: %w", err)
	}
	if _, err := tx.ExecContext(ctx, productReindexSQL); err != nil {
		return nil, fmt.Errorf("insert product documents: %w", err)
	}
	afterRows, err := countRows(ctx, tx, `SELECT COUNT(*) FROM product_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count rebuilt product documents: %w", err)
	}
	if afterRows != out.SourceRows {
		return nil, fmt.Errorf("product document row mismatch: source=%d after=%d", out.SourceRows, afterRows)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit product transaction: %w", err)
	}
	out.AfterRows = afterRows
	out.Changed = true
	return out, nil
}

type rowCounter interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func countRows(ctx context.Context, q rowCounter, query string) (int64, error) {
	var count int64
	if err := q.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func writeJSON(out reindexSummary) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode output: %v\n", err)
		os.Exit(1)
	}
}

func writeError(message string) {
	out := reindexSummary{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Message:        message,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func writeRunError(out *reindexSummary, start time.Time, message string) {
	out.ElapsedMS = time.Since(start).Milliseconds()
	out.Message = message + "; any earlier table shown with changed=true was committed atomically and is safe to rebuild again"
	writeJSON(*out)
}

const assetReindexSQL = `
	INSERT INTO task_asset_group_search_documents (
	  group_id, task_id, finalized_revision_id, internal_text, final_text
	)
	SELECT
	  g.id,
	  g.task_id,
	  g.finalized_revision_id,
	  CONCAT_WS(' ',
	    t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
	    COALESCE(tsi.sku_code, ''), COALESCE(source.file_name, ''),
	    COALESCE((SELECT GROUP_CONCAT(ref.file_name_snapshot ORDER BY ref.sort_order SEPARATOR ' ')
	              FROM task_asset_group_revision_references ref WHERE ref.revision_id = revision.id), ''),
	    COALESCE((SELECT GROUP_CONCAT(ta.file_name ORDER BY item.sort_order SEPARATOR ' ')
	              FROM task_asset_group_revision_items item
	              JOIN task_assets ta ON ta.id = item.task_asset_id
	              WHERE item.revision_id = revision.id), '')
	  ),
	  CONCAT_WS(' ',
	    t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
	    COALESCE(tsi.sku_code, ''),
	    COALESCE((SELECT GROUP_CONCAT(ta.file_name ORDER BY item.sort_order SEPARATOR ' ')
	              FROM task_asset_group_revision_items item
	              JOIN task_assets ta ON ta.id = item.task_asset_id
	              WHERE item.revision_id = revision.id), '')
	  )
	FROM task_asset_groups g
	JOIN tasks t ON t.id = g.task_id
	JOIN task_asset_group_revisions revision ON revision.id = g.finalized_revision_id
	LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
	LEFT JOIN task_assets source ON source.id = revision.source_task_asset_id
	WHERE g.finalized_revision_id IS NOT NULL`

const productReindexSQL = `
	INSERT INTO product_search_documents (
	  sku_code, product_name, i_id, category, search_text, source_updated_at
	)
SELECT
  p.sku_code,
  COALESCE(p.product_name, ''),
  COALESCE(p.i_id_gen, ''),
	  COALESCE(NULLIF(CASE WHEN JSON_VALID(p.spec_json) THEN JSON_UNQUOTE(JSON_EXTRACT(p.spec_json, '$.category_name')) ELSE '' END, ''), NULLIF(p.category, ''), ''),
	  CONCAT_WS(' ',
	    p.sku_code,
	    p.product_name,
	    p.category,
	    p.i_id_gen,
	    CASE WHEN JSON_VALID(p.spec_json) THEN JSON_UNQUOTE(JSON_EXTRACT(p.spec_json, '$.category_name')) ELSE '' END
  ),
  p.updated_at
	FROM (
	  SELECT ranked.*
	    FROM (
	      SELECT p.*,
	             ROW_NUMBER() OVER (PARTITION BY p.sku_code ORDER BY p.updated_at DESC, p.id DESC) AS rn
	        FROM products p
	       WHERE COALESCE(p.sku_code, '') <> ''
	    ) ranked
	   WHERE ranked.rn = 1
	) p
	ON DUPLICATE KEY UPDATE
	  product_name = VALUES(product_name),
	  i_id = VALUES(i_id),
	  category = VALUES(category),
	  search_text = VALUES(search_text),
	  source_updated_at = VALUES(source_updated_at)`
