package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestProductManagementRefreshReadModelPreservesProductSyncStatus(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL == "product-management-materialize" {
			return nil
		}
		if expectedSQL != "product-management-refresh" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		required := []string{
			"updated_at",
			"last_sync_error = CASE",
			"base_sync_error = CASE",
			"VALUES(updated_at) > erp_product_sync_records.updated_at",
			"WHEN VALUES(erp_sync_status) = 'pending_sync' AND erp_product_sync_records.erp_sync_status = 'failed'",
			"WHEN VALUES(base_sync_status) = 'pending_sync' AND erp_product_sync_records.base_sync_status = 'failed'",
			"WHEN erp_product_sync_records.erp_sync_status IN ('synced', 'failed')",
			"WHEN erp_product_sync_records.base_sync_status IN ('synced', 'failed')",
			"NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))",
			"latest_cost_snapshot_id = CASE",
			"latest_erp_trace_id = CASE",
			"combo_search_text = CASE",
			"cost_legacy_alias_fallback = CASE",
			"cost_area_spec_abnormal = CASE",
			"NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id)) THEN NULL",
			"NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id)) THEN 0",
			"THEN 'pending_sync'",
			"WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'",
			"WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'",
			"ELSE erp_product_sync_records.erp_sync_status",
		}
		for _, fragment := range required {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("refresh SQL missing %q", fragment)
			}
		}
		if strings.Contains(normalized, "IN ('queued', 'failed', 'cooling_down')") {
			return fmt.Errorf("refresh SQL must not preserve stale failed status over product sync success")
		}
		duplicateIndex := strings.Index(normalized, "ON DUPLICATE KEY UPDATE")
		if duplicateIndex < 0 {
			return fmt.Errorf("refresh SQL missing duplicate update")
		}
		duplicateClause := normalized[duplicateIndex:]
		statusIndex := strings.Index(duplicateClause, "erp_sync_status = CASE")
		costIndex := strings.Index(duplicateClause, "cost_price = VALUES(cost_price)")
		if statusIndex < 0 || costIndex < 0 || statusIndex > costIndex {
			return fmt.Errorf("refresh SQL must evaluate sync status before overwriting cost_price")
		}
		latestIndex := strings.Index(duplicateClause, "latest_cost_snapshot_id = CASE")
		skuUpdateIndex := strings.Index(duplicateClause, "sku_code = VALUES(sku_code)")
		if latestIndex < 0 || skuUpdateIndex < 0 || latestIndex > skuUpdateIndex {
			return fmt.Errorf("refresh SQL must invalidate materialized trace pointers before overwriting sku_code")
		}
		erpDiffIndex := strings.Index(duplicateClause, "WHEN erp_product_sync_records.erp_sync_status IN ('synced', 'failed')")
		erpSyncedIndex := strings.Index(duplicateClause, "WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'")
		if erpDiffIndex < 0 || erpSyncedIndex < 0 || erpDiffIndex > erpSyncedIndex {
			return fmt.Errorf("refresh SQL must mark changed ERP fields pending before preserving synced filing status")
		}
		baseDiffIndex := strings.Index(duplicateClause, "WHEN erp_product_sync_records.base_sync_status IN ('synced', 'failed')")
		baseSyncedIndex := strings.Index(duplicateClause, "WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'")
		if baseDiffIndex < 0 || baseSyncedIndex < 0 || baseDiffIndex > baseSyncedIndex {
			return fmt.Errorf("refresh SQL must mark changed base fields pending before preserving synced filing status")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	expectProductManagementRefreshReadModel(mock)

	repo := NewProductManagementRepo(New(db))
	if err := repo.RefreshReadModel(context.Background()); err != nil {
		t.Fatalf("RefreshReadModel() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestProductManagementRefreshReadModelFallsBackSingleSKUItemIIDToTaskPayload(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL == "product-management-materialize" {
			return nil
		}
		if expectedSQL != "product-management-refresh" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if !strings.Contains(normalized, "FROM task_sku_items tsi JOIN tasks t ON t.id = tsi.task_id") {
			return nil
		}
		required := []string{
			"JSON_EXTRACT(tsi.variant_json, '$.product_i_id')",
			"JSON_EXTRACT(tsi.variant_json, '$.i_id')",
			"COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.product_selection_snapshot_json)",
			"JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')",
			"COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.last_filing_payload_json)",
			"JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')",
			"JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')",
		}
		for _, fragment := range required {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("sku item refresh SQL missing %q", fragment)
			}
		}
		productIIDIndex := strings.Index(normalized, "JSON_EXTRACT(tsi.variant_json, '$.product_i_id')")
		taskPayloadIndex := strings.Index(normalized, "JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')")
		if productIIDIndex < 0 || taskPayloadIndex < 0 || productIIDIndex > taskPayloadIndex {
			return fmt.Errorf("sku item refresh must prefer row product_i_id before task-level filing payload")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	expectProductManagementRefreshReadModel(mock)

	repo := NewProductManagementRepo(New(db))
	if err := repo.RefreshReadModel(context.Background()); err != nil {
		t.Fatalf("RefreshReadModel() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestProductManagementClaimQueuedSyncRecordsClaimsChildSyncStatuses(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "claim-product-management-sync":
			required := []string{
				"OR base_sync_status = 'queued'",
				"OR (base_sync_status = 'cooling_down'",
				"OR (base_sync_status = 'syncing'",
				"OR image_sync_status = 'queued'",
				"OR (image_sync_status = 'cooling_down'",
				"OR (image_sync_status = 'syncing'",
				"WHEN erp_sync_status = 'queued' OR base_sync_status = 'queued' OR image_sync_status = 'queued' THEN 0",
			}
			for _, fragment := range required {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("claim SQL missing %q", fragment)
				}
			}
			return nil
		case "list-claimed-product-management-sync":
			if !strings.Contains(normalized, "WHERE pm.sync_claim_token = ?") {
				return fmt.Errorf("claimed list SQL missing claim token filter")
			}
			return nil
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("claim-product-management-sync").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("list-claimed-product-management-sync").
		WillReturnRows(sqlmock.NewRows(strings.Split(strings.ReplaceAll(productManagementSelectCols, "\n", " "), ",")))
	mock.ExpectCommit()

	repo := NewProductManagementRepo(New(db))
	if _, err := repo.ClaimQueuedSyncRecords(context.Background(), 10, "claim-token", testProductManagementNow()); err != nil {
		t.Fatalf("ClaimQueuedSyncRecords() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestProductManagementQueuePendingBaseSyncByTaskIDQueuesOnlyReadyBaseRecords(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "queue-product-management-base-sync" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		required := []string{
			"SET erp_sync_status = 'queued'",
			"base_sync_status = 'queued'",
			"last_sync_error = ''",
			"base_sync_error = ''",
			"WHERE task_id = ?",
			"COALESCE(sku_code, '') <> ''",
			"COALESCE(product_name, '') <> ''",
			"COALESCE(NULLIF(erp_i_id, ''), NULLIF(product_i_id, ''), NULLIF(product_family, ''), NULLIF(category_name, '')) IS NOT NULL",
			"base_sync_status IN ('pending_sync', 'failed')",
			"erp_sync_status NOT IN ('queued', 'cooling_down', 'syncing')",
		}
		for _, fragment := range required {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("queue SQL missing %q", fragment)
			}
		}
		if strings.Contains(normalized, "image_sync_status = 'queued'") {
			return fmt.Errorf("queue SQL must not queue image sync")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := testProductManagementNow()
	cooldownUntil := now.Add(5 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectExec("queue-product-management-base-sync").
		WithArgs(now, cooldownUntil, int64(1497)).
		WillReturnResult(sqlmock.NewResult(0, 11))
	mock.ExpectCommit()

	mysqlDB := New(db)
	var queued int64
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		var err error
		queued, err = NewProductManagementRepo(mysqlDB).QueuePendingBaseSyncByTaskID(context.Background(), tx, 1497, now, cooldownUntil)
		return err
	}); err != nil {
		t.Fatalf("QueuePendingBaseSyncByTaskID() error = %v", err)
	}
	if queued != 11 {
		t.Fatalf("queued = %d, want 11", queued)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestProductManagementMarkBaseSyncProjectionSyncedUpdatesTaskProjection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "mark-task-sku-item-base-sync-filed":
			for _, fragment := range []string{
				"UPDATE task_sku_items",
				"sku_status = 'filed'",
				"filing_status = 'filed'",
				"erp_sync_status = 'filed'",
				"erp_sync_required = 0",
				"erp_sync_version = CASE",
				"WHERE task_id = ? AND id = ?",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("sku projection SQL missing %q: %s", fragment, normalized)
				}
			}
			return nil
		case "mark-task-base-sync-filed":
			for _, fragment := range []string{
				"UPDATE task_details td",
				"filing_status = 'filed'",
				"erp_sync_required = 0",
				"last_filed_at = ?",
				"EXISTS ( SELECT 1 FROM erp_product_sync_records pm WHERE pm.task_id = td.task_id )",
				"NOT EXISTS ( SELECT 1 FROM erp_product_sync_records pm WHERE pm.task_id = td.task_id AND pm.base_sync_status <> 'synced' )",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("task projection SQL missing %q: %s", fragment, normalized)
				}
			}
			return nil
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := testProductManagementNow()
	skuItemID := int64(1492)
	mock.ExpectBegin()
	mock.ExpectExec("mark-task-sku-item-base-sync-filed").
		WithArgs(now, int64(1497), skuItemID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("mark-task-base-sync-filed").
		WithArgs(now, now, now, int64(1497)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mysqlDB := New(db)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		return NewProductManagementRepo(mysqlDB).MarkBaseSyncProjectionSynced(context.Background(), tx, 1497, &skuItemID, now)
	}); err != nil {
		t.Fatalf("MarkBaseSyncProjectionSynced() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func testProductManagementNow() time.Time {
	return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
}

func expectProductManagementRefreshReadModel(mock sqlmock.Sqlmock) {
	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 5; i++ {
		mock.ExpectExec("product-management-materialize").WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

func TestProductManagementWhereTreatsUnverifiedImageSyncedAsPending(t *testing.T) {
	where, args := buildProductManagementWhere(repo.ProductManagementListFilter{ImageSyncStatus: "synced"})
	if !strings.Contains(where, "pm.image_sync_status = ? AND pm.last_image_synced_at IS NOT NULL") {
		t.Fatalf("synced image filter where = %s", where)
	}
	if len(args) != 1 || args[0] != "synced" {
		t.Fatalf("synced image filter args = %#v", args)
	}

	where, args = buildProductManagementWhere(repo.ProductManagementListFilter{ImageSyncStatus: "pending_sync"})
	if !strings.Contains(where, "pm.image_sync_status = ? OR (pm.image_sync_status = 'synced' AND pm.last_image_synced_at IS NULL)") {
		t.Fatalf("pending image filter where = %s", where)
	}
	if len(args) != 1 || args[0] != "pending_sync" {
		t.Fatalf("pending image filter args = %#v", args)
	}

	where, _ = buildProductManagementWhere(repo.ProductManagementListFilter{IssueScope: "attention"})
	if !strings.Contains(where, "OR (pm.image_sync_status = 'synced' AND pm.last_image_synced_at IS NULL)") {
		t.Fatalf("attention filter where = %s", where)
	}
}

func TestProductManagementWhereUsesComboFullTextWhenEnabled(t *testing.T) {
	where, args := buildProductManagementWhereWithOptions(repo.ProductManagementListFilter{Keyword: "COMBO001"}, productManagementWhereOptions{UseComboFullText: true})
	if !strings.Contains(where, "MATCH(pm.combo_search_text) AGAINST (? IN NATURAL LANGUAGE MODE)") {
		t.Fatalf("where missing combo fulltext: %s", where)
	}
	if strings.Contains(where, "FROM omp_sku_combo_relations rel") {
		t.Fatalf("where must not use combo relation EXISTS on fulltext path: %s", where)
	}
	if !containsStringArg(args, "COMBO001") {
		t.Fatalf("args missing fulltext keyword: %#v", args)
	}
}

func TestProductManagementWhereSearchesComboRelations(t *testing.T) {
	where, args := buildProductManagementWhere(repo.ProductManagementListFilter{Keyword: "COMBO001"})
	for _, fragment := range []string{
		"FROM omp_sku_combo_relations rel",
		"LEFT JOIN omp_sku_combo_records rec",
		"rel.child_sku_code = pm.sku_code",
		"rel.combo_sku_code = ?",
		"COALESCE(rec.erp_i_id, '') = ?",
		"rel.combo_sku_code LIKE ?",
		"rec.name LIKE ?",
		"rec.short_name LIKE ?",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where missing %q: %s", fragment, where)
		}
	}
	if got, wantMin := len(args), 17; got < wantMin {
		t.Fatalf("args len = %d, want at least %d; args = %#v", got, wantMin, args)
	}
	if !containsStringArg(args, "COMBO001") || !containsStringArg(args, "COMBO001%") {
		t.Fatalf("args missing exact/prefix combo search values: %#v", args)
	}
}

func TestProductManagementWhereSyncedStatusSkipsAttentionScope(t *testing.T) {
	cases := []struct {
		name         string
		filter       repo.ProductManagementListFilter
		wantFragment string
	}{
		{
			name:         "overall synced",
			filter:       repo.ProductManagementListFilter{IssueScope: "attention", SyncStatus: "synced"},
			wantFragment: "pm.erp_sync_status = ?",
		},
		{
			name:         "base synced",
			filter:       repo.ProductManagementListFilter{IssueScope: "attention", BaseSyncStatus: "synced"},
			wantFragment: "pm.base_sync_status = ?",
		},
		{
			name:         "image synced",
			filter:       repo.ProductManagementListFilter{IssueScope: "attention", ImageSyncStatus: "synced"},
			wantFragment: "pm.image_sync_status = ? AND pm.last_image_synced_at IS NOT NULL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			where, args := buildProductManagementWhere(tc.filter)
			if !strings.Contains(where, tc.wantFragment) {
				t.Fatalf("where missing status fragment %q: %s", tc.wantFragment, where)
			}
			if strings.Contains(where, "cost_price IS NULL") || strings.Contains(where, "base_sync_status IN") || strings.Contains(where, "image_sync_status IN") {
				t.Fatalf("synced status filter must not include attention scope: %s", where)
			}
			if len(args) != 1 || args[0] != "synced" {
				t.Fatalf("args = %#v", args)
			}
		})
	}
}
