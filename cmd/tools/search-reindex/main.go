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
)

type reindexSummary struct {
	DryRun         bool          `json:"dry_run"`
	Assets         *reindexTable `json:"assets,omitempty"`
	Products       *reindexTable `json:"products,omitempty"`
	ElapsedMS      int64         `json:"elapsed_ms"`
	GeneratedAtUTC string        `json:"generated_at_utc"`
	Message        string        `json:"message,omitempty"`
}

type reindexTable struct {
	SourceRows int64 `json:"source_rows"`
	BeforeRows int64 `json:"before_rows"`
	AfterRows  int64 `json:"after_rows,omitempty"`
	Changed    bool  `json:"changed"`
}

func main() {
	var dsn string
	var assets bool
	var products bool
	var dryRun bool
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.BoolVar(&assets, "assets", true, "rebuild asset_search_documents")
	flag.BoolVar(&products, "products", true, "rebuild product_search_documents")
	flag.BoolVar(&dryRun, "dry-run", false, "count rows without rebuilding documents")
	flag.DurationVar(&timeout, "timeout", 10*time.Minute, "whole run timeout")
	flag.Parse()

	code := run(context.Background(), runOptions{
		DSN:      dsn,
		Assets:   assets,
		Products: products,
		DryRun:   dryRun,
		Timeout:  timeout,
	})
	os.Exit(code)
}

type runOptions struct {
	DSN      string
	Assets   bool
	Products bool
	DryRun   bool
	Timeout  time.Duration
}

func run(parent context.Context, opts runOptions) int {
	start := time.Now()
	if !opts.Assets && !opts.Products {
		writeError("at least one of --assets or --products must be true")
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
	if opts.Assets {
		table, err := rebuildAssets(ctx, db, opts.DryRun)
		if err != nil {
			writeError(fmt.Sprintf("rebuild assets: %v", err))
			return 1
		}
		out.Assets = table
	}
	if opts.Products {
		table, err := rebuildProducts(ctx, db, opts.DryRun)
		if err != nil {
			writeError(fmt.Sprintf("rebuild products: %v", err))
			return 1
		}
		out.Products = table
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	if opts.DryRun {
		out.Message = "dry run only; no search document rows changed"
	}
	writeJSON(out)
	return 0
}

func rebuildAssets(ctx context.Context, db *sql.DB, dryRun bool) (*reindexTable, error) {
	sourceRows, err := countRows(ctx, db, `
		SELECT COUNT(*)
		  FROM design_assets da
		  JOIN task_assets ta ON ta.id = da.current_version_id
		  JOIN tasks t ON t.id = ta.task_id
		 WHERE ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND COALESCE(ta.is_archived, 0) = 0`)
	if err != nil {
		return nil, fmt.Errorf("count asset source rows: %w", err)
	}
	beforeRows, err := countRows(ctx, db, `SELECT COUNT(*) FROM asset_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count asset documents: %w", err)
	}
	out := &reindexTable{SourceRows: sourceRows, BeforeRows: beforeRows}
	if dryRun {
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin asset transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return nil, fmt.Errorf("set group_concat_max_len: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM asset_search_documents`); err != nil {
		return nil, fmt.Errorf("delete asset documents: %w", err)
	}
	if _, err := tx.ExecContext(ctx, assetReindexSQL); err != nil {
		return nil, fmt.Errorf("insert asset documents: %w", err)
	}
	afterRows, err := countRows(ctx, tx, `SELECT COUNT(*) FROM asset_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count rebuilt asset documents: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit asset transaction: %w", err)
	}
	out.AfterRows = afterRows
	out.Changed = true
	return out, nil
}

func rebuildProducts(ctx context.Context, db *sql.DB, dryRun bool) (*reindexTable, error) {
	sourceRows, err := countRows(ctx, db, `SELECT COUNT(*) FROM products WHERE COALESCE(sku_code, '') <> ''`)
	if err != nil {
		return nil, fmt.Errorf("count product source rows: %w", err)
	}
	beforeRows, err := countRows(ctx, db, `SELECT COUNT(*) FROM product_search_documents`)
	if err != nil {
		return nil, fmt.Errorf("count product documents: %w", err)
	}
	out := &reindexTable{SourceRows: sourceRows, BeforeRows: beforeRows}
	if dryRun {
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin product transaction: %w", err)
	}
	defer tx.Rollback()
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

const assetReindexSQL = `
	INSERT INTO asset_search_documents (
	  asset_id, task_asset_id, task_id, asset_type, flow_review_status, sort_time, search_text, source_updated_at
	)
	SELECT
	  da.id,
	  ta.id,
	  ta.task_id,
	  COALESCE(ta.asset_type, ''),
	  COALESCE(ta.flow_review_status, ''),
	  COALESCE(ta.sort_time, ta.uploaded_at, ta.created_at),
	  CONCAT_WS(' ',
	    da.id, da.asset_no, ta.id, ta.file_name, ta.original_filename, ta.storage_key, ta.source_module_key,
	    t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
	    t.owner_team, t.owner_department, t.owner_org_team,
	    creator.username, creator.display_name, designer.username, designer.display_name
	  ),
	  GREATEST(ta.created_at, da.updated_at, t.updated_at)
	FROM design_assets da
	JOIN task_assets ta ON ta.id = da.current_version_id
	JOIN tasks t ON t.id = ta.task_id
	LEFT JOIN users creator ON creator.id = t.creator_id
	LEFT JOIN users designer ON designer.id = t.designer_id
	WHERE ta.deleted_at IS NULL
	  AND ta.cleaned_at IS NULL
	  AND COALESCE(ta.is_archived, 0) = 0`

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
	FROM products p
	WHERE COALESCE(p.sku_code, '') <> ''`
