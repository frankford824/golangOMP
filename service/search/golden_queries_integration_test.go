//go:build integration

package search

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"workflow/domain"
)

type goldenQueryCase struct {
	Name             string   `json:"name"`
	Query            string   `json:"query"`
	Scope            string   `json:"scope"`
	ExpectedTasks    []int64  `json:"expected_tasks"`
	ExpectedProducts []string `json:"expected_products"`
	ExpectedAssets   []int64  `json:"expected_assets"`
}

func TestSearchGoldenQueriesCoverage(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	seedGoldenSearchDocuments(t, db)
	t.Cleanup(func() { cleanupGoldenSearchDocuments(t, db) })

	cases := loadGoldenQueryCases(t)
	ctx, cancel := sadCtx(t)
	defer cancel()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, appErr := svc.Search(ctx, sadActor(50021, domain.RoleSuperAdmin), tc.Query, tc.Scope, 10)
			if appErr != nil {
				t.Fatalf("Search(%q, %q) appErr = %+v", tc.Query, tc.Scope, appErr)
			}
			for _, taskID := range tc.ExpectedTasks {
				if !searchTasksContain(got.Tasks, taskID) {
					t.Fatalf("Search(%q, %q) missing task %d in top results: %+v", tc.Query, tc.Scope, taskID, got.Tasks)
				}
			}
			for _, sku := range tc.ExpectedProducts {
				if !searchProductsContain(got.Products, sku) {
					t.Fatalf("Search(%q, %q) missing product %s in top results: %+v", tc.Query, tc.Scope, sku, got.Products)
				}
			}
			for _, assetID := range tc.ExpectedAssets {
				if !searchAssetsContain(got.Assets, assetID) {
					t.Fatalf("Search(%q, %q) missing asset %d in top results: %+v", tc.Query, tc.Scope, assetID, got.Assets)
				}
			}
		})
	}
}

func loadGoldenQueryCases(t *testing.T) []goldenQueryCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden_queries.json")
	if err != nil {
		t.Fatalf("read golden queries: %v", err)
	}
	var cases []goldenQueryCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parse golden queries: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("golden query case list is empty")
	}
	return cases
}

func seedGoldenSearchDocuments(t *testing.T, db *sql.DB) {
	t.Helper()
	cleanupGoldenSearchDocuments(t, db)
	now := time.Now().UTC()
	insertGoldenTaskDocument(t, db, 50021, "RW-GOLD-50021", "Golden 编码任务", "CGK000733", "IID-GOLD-733", "RW-GOLD-50021 CGK000733 IID-GOLD-733 Golden 编码任务", now)
	insertGoldenTaskDocument(t, db, 50022, "RW-GOLD-50022", "常规kt板 展示任务", "CGK000734", "IID-GOLD-734", "RW-GOLD-50022 CGK000734 IID-GOLD-734 常规kt板 展示牌 覆膜", now.Add(time.Second))
	insertGoldenProductDocument(t, db, "CGK000733", "竹夹子 20厘米", "IID-GOLD-733", "夹子", "CGK000733 IID-GOLD-733 竹夹子 20厘米", now)
	insertGoldenProductDocument(t, db, "CGK000734", "常规kt板 展示牌", "IID-GOLD-734", "KT板", "CGK000734 IID-GOLD-734 常规kt板 展示牌 覆膜", now.Add(time.Second))
	sadInsertTaskAsset(t, db, 50023, "RW-GOLD-50023", "CGK000735", "交付图常规kt板.png")
	insertGoldenAssetDocument(t, db, 50023, "交付图常规kt板.png RW-GOLD-50023 CGK000735 常规kt板", now.Add(2*time.Second))
}

func cleanupGoldenSearchDocuments(t *testing.T, db *sql.DB) {
	t.Helper()
	_, _ = db.Exec(`DELETE FROM asset_search_documents WHERE asset_id BETWEEN 50020 AND 50029 OR task_id BETWEEN 50020 AND 50029`)
	_, _ = db.Exec(`DELETE FROM product_search_documents WHERE sku_code IN ('CGK000733', 'CGK000734')`)
	_, _ = db.Exec(`DELETE FROM task_search_documents WHERE task_id BETWEEN 50020 AND 50029`)
	sadCleanup(t, db, []int64{50021, 50022, 50023}, nil)
}

func insertGoldenTaskDocument(t *testing.T, db *sql.DB, taskID int64, taskNo, title, sku, iid, searchText string, updatedAt time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO task_search_documents
		  (task_id, task_no, product_name_snapshot, sku_code, primary_sku_code, product_i_id,
		   task_type, task_status, priority, owner_department, owner_team, owner_org_team,
		   created_at, updated_at, search_text)
		VALUES (?, ?, ?, ?, ?, ?, 'original_product_development', 'InProgress', 'normal',
		        '运营部', 'golden', 'operations_golden', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  task_no = VALUES(task_no),
		  product_name_snapshot = VALUES(product_name_snapshot),
		  sku_code = VALUES(sku_code),
		  primary_sku_code = VALUES(primary_sku_code),
		  product_i_id = VALUES(product_i_id),
		  updated_at = VALUES(updated_at),
		  search_text = VALUES(search_text)`,
		taskID, taskNo, title, sku, sku, iid, updatedAt, updatedAt, searchText)
	if err != nil {
		t.Fatalf("insert golden task document %d: %v", taskID, err)
	}
}

func insertGoldenProductDocument(t *testing.T, db *sql.DB, sku, name, iid, category, searchText string, updatedAt time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO product_search_documents
		  (sku_code, product_name, i_id, category, search_text, source_updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  product_name = VALUES(product_name),
		  i_id = VALUES(i_id),
		  category = VALUES(category),
		  search_text = VALUES(search_text),
		  source_updated_at = VALUES(source_updated_at)`,
		sku, name, iid, category, searchText, updatedAt)
	if err != nil {
		t.Fatalf("insert golden product document %s: %v", sku, err)
	}
}

func insertGoldenAssetDocument(t *testing.T, db *sql.DB, taskID int64, searchText string, updatedAt time.Time) {
	t.Helper()
	var taskAssetID int64
	if err := db.QueryRow(`SELECT id FROM task_assets WHERE task_id = ? ORDER BY id DESC LIMIT 1`, taskID).Scan(&taskAssetID); err != nil {
		t.Fatalf("find golden task asset for task %d: %v", taskID, err)
	}
	_, err := db.Exec(`
		INSERT INTO asset_search_documents
		  (asset_id, task_asset_id, task_id, asset_type, flow_review_status, sort_time, search_text, source_updated_at)
		VALUES (?, ?, ?, 'delivery', 'approved', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  task_asset_id = VALUES(task_asset_id),
		  task_id = VALUES(task_id),
		  asset_type = VALUES(asset_type),
		  flow_review_status = VALUES(flow_review_status),
		  sort_time = VALUES(sort_time),
		  search_text = VALUES(search_text),
		  source_updated_at = VALUES(source_updated_at)`,
		taskID, taskAssetID, taskID, updatedAt, searchText, updatedAt)
	if err != nil {
		t.Fatalf("insert golden asset document %d: %v", taskID, err)
	}
}

func searchTasksContain(items []domain.SearchTask, taskID int64) bool {
	for _, item := range items {
		if item.ID == taskID {
			return true
		}
	}
	return false
}

func searchProductsContain(items []domain.SearchProduct, sku string) bool {
	for _, item := range items {
		if item.ERPCode == sku {
			return true
		}
	}
	return false
}

func searchAssetsContain(items []domain.SearchAsset, assetID int64) bool {
	for _, item := range items {
		if item.AssetID == assetID {
			return true
		}
	}
	return false
}
