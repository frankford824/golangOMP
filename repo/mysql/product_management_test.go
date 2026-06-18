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
			"WHEN erp_product_sync_records.erp_sync_status = 'synced'",
			"NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))",
			"THEN 'pending_sync'",
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
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))

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
			}
			for _, fragment := range required {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("claim SQL missing %q", fragment)
				}
			}
			return nil
		case "list-claimed-product-management-sync":
			if !strings.Contains(normalized, "WHERE sync_claim_token = ?") {
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

func testProductManagementNow() time.Time {
	return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
}

func TestProductManagementWhereTreatsUnverifiedImageSyncedAsPending(t *testing.T) {
	where, args := buildProductManagementWhere(repo.ProductManagementListFilter{ImageSyncStatus: "synced"})
	if !strings.Contains(where, "image_sync_status = ? AND last_image_synced_at IS NOT NULL") {
		t.Fatalf("synced image filter where = %s", where)
	}
	if len(args) != 1 || args[0] != "synced" {
		t.Fatalf("synced image filter args = %#v", args)
	}

	where, args = buildProductManagementWhere(repo.ProductManagementListFilter{ImageSyncStatus: "pending_sync"})
	if !strings.Contains(where, "image_sync_status = ? OR (image_sync_status = 'synced' AND last_image_synced_at IS NULL)") {
		t.Fatalf("pending image filter where = %s", where)
	}
	if len(args) != 1 || args[0] != "pending_sync" {
		t.Fatalf("pending image filter args = %#v", args)
	}

	where, _ = buildProductManagementWhere(repo.ProductManagementListFilter{IssueScope: "attention"})
	if !strings.Contains(where, "OR (image_sync_status = 'synced' AND last_image_synced_at IS NULL)") {
		t.Fatalf("attention filter where = %s", where)
	}
}

func TestProductManagementWhereSearchesComboRelations(t *testing.T) {
	where, args := buildProductManagementWhere(repo.ProductManagementListFilter{Keyword: "COMBO001"})
	for _, fragment := range []string{
		"FROM omp_sku_combo_relations rel",
		"LEFT JOIN omp_sku_combo_records rec",
		"rel.child_sku_code = erp_product_sync_records.sku_code COLLATE utf8mb4_0900_ai_ci",
		"rel.combo_sku_code LIKE ?",
		"rec.name LIKE ?",
		"rec.short_name LIKE ?",
		"rec.erp_i_id LIKE ?",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where missing %q: %s", fragment, where)
		}
	}
	if len(args) != 12 {
		t.Fatalf("args len = %d, want 12; args = %#v", len(args), args)
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
			wantFragment: "erp_sync_status = ?",
		},
		{
			name:         "base synced",
			filter:       repo.ProductManagementListFilter{IssueScope: "attention", BaseSyncStatus: "synced"},
			wantFragment: "base_sync_status = ?",
		},
		{
			name:         "image synced",
			filter:       repo.ProductManagementListFilter{IssueScope: "attention", ImageSyncStatus: "synced"},
			wantFragment: "image_sync_status = ? AND last_image_synced_at IS NOT NULL",
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
