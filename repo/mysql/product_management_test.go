package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProductManagementRefreshReadModelPreservesProductSyncStatus(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "product-management-refresh" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		required := []string{
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
