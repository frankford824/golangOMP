package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProductionPackageResolvedSKUNormalizesMixedCollations(t *testing.T) {
	for _, source := range []string{
		"tsi.sku_code",
		"rr.sku_code",
		"ta.scope_sku_code",
		"t.primary_sku_code",
		"t.sku_code",
	} {
		want := "CONVERT(" + source + " USING utf8mb4) COLLATE utf8mb4_unicode_ci"
		if !strings.Contains(productionPackageResolvedSKU, want) {
			t.Fatalf("resolved SKU expression must normalize %s: %s", source, productionPackageResolvedSKU)
		}
	}
}

func TestProductionPackageManifestUsesExistingTaskAssetTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_ string, actual string) error {
		query := strings.Join(strings.Fields(actual), " ")
		if strings.Contains(query, "ta.updated_at") {
			return fmt.Errorf("task_assets has no updated_at column: %s", query)
		}
		if !strings.Contains(query, "COALESCE(ta.whole_hash, ''), ta.created_at FROM") {
			return fmt.Errorf("query must end task-asset projection with created_at: %s", query)
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	columns := []string{
		"group_id", "revision_id", "revision_mode", "finalized_at", "revision_item_id",
		"sort_order", "item_name", "task_asset_id", "task_id", "task_no", "sku_code",
		"product_name", "scope_kind", "file_name", "original_filename", "mime_type",
		"file_size", "storage_key", "whole_hash", "created_at",
	}
	mock.ExpectQuery("finalized manifest query").WillReturnRows(sqlmock.NewRows(columns).AddRow(
		int64(1), int64(2), "single", now, int64(3), 0, "最终成品图", int64(4), int64(5),
		"RW-1", "SKU-1", "Product", "sku", "final.jpg", "final.jpg", "image/jpeg",
		int64(10), "tasks/final.jpg", "", now,
	))

	items, err := NewProductionPackageRepo(New(db)).ListAllFinalizedAssets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].CreatedAt.Equal(now) {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
