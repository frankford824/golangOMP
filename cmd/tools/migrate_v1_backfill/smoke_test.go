//go:build integration

package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

func TestR2BackfillSmoke(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is not set")
	}
	db, err := v1migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	assertBasicInfoCoverage(ctx, t, db)
	assertPendingAuditCoverage(ctx, t, db)
	assertAssetSourceCoverage(ctx, t, db)
	assertReferenceCoverage(ctx, t, db)
	assertBackfillIdempotent(ctx, t, db)
}

func assertBasicInfoCoverage(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var total, covered int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_modules WHERE module_key='basic_info'`).Scan(&covered); err != nil {
		t.Fatal(err)
	}
	if covered != total {
		t.Fatalf("basic_info coverage mismatch: tasks=%d modules=%d", total, covered)
	}
}

func assertPendingAuditCoverage(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var total, covered int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE task_status='PendingAuditA'`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM tasks t
		  JOIN task_modules tm ON tm.task_id=t.id AND tm.module_key='audit'
		 WHERE t.task_status='PendingAuditA'`).Scan(&covered); err != nil {
		t.Fatal(err)
	}
	if covered != total {
		t.Fatalf("PendingAuditA audit module mismatch: tasks=%d modules=%d", total, covered)
	}
}

func assertAssetSourceCoverage(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var missing int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_assets WHERE source_module_key IS NULL OR source_module_key=''`).Scan(&missing); err != nil {
		t.Fatal(err)
	}
	if missing != 0 {
		t.Fatalf("task_assets with empty source_module_key: %d", missing)
	}
}

func assertReferenceCoverage(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var expected, actual int64
	if err := db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM task_details WHERE reference_file_refs_json IS NOT NULL AND reference_file_refs_json<>'' AND reference_file_refs_json<>'[]')
		  +
		  (SELECT COUNT(*) FROM task_sku_items WHERE reference_file_refs_json IS NOT NULL AND reference_file_refs_json<>'' AND reference_file_refs_json<>'[]')`).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_file_refs`).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	min := int64(float64(expected) * 0.995)
	if actual < min {
		t.Fatalf("reference flatten coverage too low: actual=%d expected_min=%d json_rows=%d", actual, min, expected)
	}
}

func assertBackfillIdempotent(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	before := tableCounts(ctx, t, db)
	if _, err := runBackfill(ctx, db, BackfillOptions{BatchSize: 1000}); err != nil {
		t.Fatal(err)
	}
	after := tableCounts(ctx, t, db)
	for table, beforeCount := range before {
		if after[table] != beforeCount {
			t.Fatalf("idempotency count changed for %s: before=%d after=%d", table, beforeCount, after[table])
		}
	}
}

func tableCounts(ctx context.Context, t *testing.T, db *sql.DB) map[string]int64 {
	t.Helper()
	tables := []string{"task_modules", "task_module_events", "reference_file_refs", "task_customization_orders"}
	out := make(map[string]int64, len(tables))
	for _, table := range tables {
		var n int64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[table] = n
	}
	return out
}
