package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestDriveListFilesWithoutOrderListsDirectoryByUploadTime(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		switch expectedSQL {
		case "drive-list-files-count", "drive-list-files-select":
			if strings.Contains(actualSQL, "i.order_no = ?") {
				return fmt.Errorf("DriveListFiles should not filter order_no when omitted:\n%s", actualSQL)
			}
			if expectedSQL == "drive-list-files-select" && !strings.Contains(actualSQL, "ORDER BY f.created_at DESC, f.id DESC") {
				return fmt.Errorf("DriveListFiles should sort directory files by upload time:\n%s", actualSQL)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("drive-list-files-count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("drive-list-files-select").
		WithArgs(60, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "submission_id", "submission_item_id", "submission_no", "owner_user_id",
			"owner_name", "owner_username",
			"upload_directory_id", "upload_directory_name", "difficulty_class", "order_no",
			"original_filename", "display_name", "relative_path", "upload_batch_id", "is_folder_upload",
			"file_type", "mime_type", "file_size", "preview_status",
			"qc_status", "pricing_status", "settlement_status", "page_count",
			"gross_amount", "business_month", "created_at",
		}).AddRow(
			int64(42), int64(11), int64(21), "SUB-001", int64(7),
			"张三", "zhangsan",
			int64(3), "挂布", "A", "AWF20260703080000ABCDEF12",
			"sample.jpg", "sample.jpg", "folder/sample.jpg", "batch-1", true,
			"jpg", "image/jpeg", int64(1024), "ready",
			"passed", "priced", "pending", 1,
			12.5, "2026-07", now,
		))

	workbenchRepo := NewAssetWorkbenchRepo(New(db))
	items, total, err := workbenchRepo.DriveListFiles(context.Background(), repo.AssetWorkbenchDriveFilter{Page: 1, PageSize: 60})
	if err != nil {
		t.Fatalf("DriveListFiles() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != 42 {
		t.Fatalf("DriveListFiles() total=%d items=%+v, want one file 42", total, items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDriveListFilesAppliesUploadOverviewFilters(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		switch expectedSQL {
		case "drive-list-files-count", "drive-list-files-select":
			checks := []string{
				"f.owner_user_id = ?",
				"f.original_filename LIKE ?",
				"COALESCE(p.real_name, '') LIKE ?",
				"COALESCE(u.display_name, '') LIKE ?",
				"COALESCE(u.username, '') LIKE ?",
				"f.created_at >= ?",
				"f.created_at <= ?",
				"LEFT JOIN users u ON u.id = f.owner_user_id",
				"LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id",
			}
			for _, check := range checks {
				if !strings.Contains(actualSQL, check) {
					return fmt.Errorf("DriveListFiles query missing %q:\n%s", check, actualSQL)
				}
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	ownerID := int64(7)
	createdFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	createdTo := time.Date(2026, 7, 3, 23, 59, 59, 0, time.UTC)
	args := []driver.Value{
		ownerID,
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%海报%",
		"%张三%",
		"%张三%",
		"%张三%",
		createdFrom,
		createdTo,
	}
	mock.ExpectQuery("drive-list-files-count").
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery("drive-list-files-select").
		WithArgs(append(args, 25, 25)...).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "submission_id", "submission_item_id", "submission_no", "owner_user_id",
			"owner_name", "owner_username",
			"upload_directory_id", "upload_directory_name", "difficulty_class", "order_no",
			"original_filename", "display_name", "relative_path", "upload_batch_id", "is_folder_upload",
			"file_type", "mime_type", "file_size", "preview_status",
			"qc_status", "pricing_status", "settlement_status", "page_count",
			"gross_amount", "business_month", "created_at",
		}))

	workbenchRepo := NewAssetWorkbenchRepo(New(db))
	_, total, err := workbenchRepo.DriveListFiles(context.Background(), repo.AssetWorkbenchDriveFilter{
		OwnerUserID:  &ownerID,
		Keyword:      " 海报 ",
		OwnerKeyword: " 张三 ",
		CreatedFrom:  &createdFrom,
		CreatedTo:    &createdTo,
		Page:         2,
		PageSize:     25,
	})
	if err != nil {
		t.Fatalf("DriveListFiles() error = %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestBuildDriveSearchBaseUsesFullTextWhenPreferred(t *testing.T) {
	base, args := buildDriveSearchBase(repo.AssetWorkbenchDriveFilter{Keyword: "海报"}, true)

	if !strings.Contains(base, "WHERE f.id IN (") || !strings.Contains(base, "UNION DISTINCT") {
		t.Fatalf("drive search base should anchor fulltext branches through an id union: %s", base)
	}
	if !strings.Contains(base, "MATCH(f.original_filename, f.display_name, f.relative_path, f.file_type, f.mime_type, f.upload_directory_name) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("drive search base should use file fulltext: %s", base)
	}
	if !strings.Contains(base, "MATCH(i.order_no, i.template_name_snapshot, i.category_snapshot, i.difficulty_class) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("drive search base should use item fulltext: %s", base)
	}
	if !strings.Contains(base, "MATCH(s.submission_no, s.notes) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("drive search base should use submission fulltext: %s", base)
	}
	if len(args) != 3 || args[0] != "+海报*" || args[1] != "+海报*" || args[2] != "+海报*" {
		t.Fatalf("args = %#v, want three fulltext args", args)
	}
}

func TestBuildDriveSearchBaseFallbackUsesContainsLike(t *testing.T) {
	base, args := buildDriveSearchBase(repo.AssetWorkbenchDriveFilter{Keyword: "海报"}, false)

	if strings.Contains(base, "MATCH(") {
		t.Fatalf("fallback drive search should not use fulltext: %s", base)
	}
	if !strings.Contains(base, "f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR i.order_no LIKE ? OR s.submission_no LIKE ? OR COALESCE(p.real_name, '') LIKE ? OR COALESCE(u.display_name, '') LIKE ? OR COALESCE(u.username, '') LIKE ?") {
		t.Fatalf("fallback drive search should use contains LIKE including uploader identity: %s", base)
	}
	if len(args) != 8 {
		t.Fatalf("args = %#v, want eight LIKE args", args)
	}
	for i, arg := range args {
		if arg != "%海报%" {
			t.Fatalf("args[%d] = %#v, want LIKE keyword", i, arg)
		}
	}
}

func TestDriveLocateFileFiltersVoidedItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		switch expectedSQL {
		case "drive-locate-file":
			if !strings.Contains(actualSQL, "i.id = f.submission_item_id AND i.voided_at IS NULL") {
				return fmt.Errorf("DriveLocateFile query does not filter voided items:\n%s", actualSQL)
			}
		case "drive-locate-page":
			if !strings.Contains(actualSQL, "f.upload_directory_id = ?") || !strings.Contains(actualSQL, "f.created_at > ? OR (f.created_at = ? AND f.id > ?)") {
				return fmt.Errorf("DriveLocateFile page query does not count the current directory rank:\n%s", actualSQL)
			}
		default:
			return nil
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("drive-locate-file").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "submission_id", "submission_item_id", "submission_no", "owner_user_id",
			"owner_name", "owner_username",
			"upload_directory_id", "upload_directory_name", "difficulty_class", "order_no",
			"original_filename", "display_name", "relative_path", "upload_batch_id", "is_folder_upload",
			"file_type", "mime_type", "file_size", "preview_status",
			"qc_status", "pricing_status", "settlement_status", "page_count",
			"gross_amount", "business_month", "created_at",
		}).AddRow(
			int64(42), int64(11), int64(21), "SUB-001", int64(7),
			"张三", "zhangsan",
			int64(3), "挂布", "A", "ORDER-001",
			"sample.jpg", "sample.jpg", "folder/sample.jpg", "batch-1", true,
			"jpg", "image/jpeg", int64(1024), "ready",
			"passed", "priced", "pending", 1,
			12.5, "2026-07", now,
		))
	mock.ExpectQuery("drive-locate-page").
		WithArgs(int64(3), now, now, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(120)))

	workbenchRepo := NewAssetWorkbenchRepo(New(db))
	file, err := workbenchRepo.DriveLocateFile(context.Background(), repo.AssetWorkbenchDriveFilter{}, 42)
	if err != nil {
		t.Fatalf("DriveLocateFile() error = %v", err)
	}
	if file.ID != 42 || file.OrderNo != "ORDER-001" {
		t.Fatalf("DriveLocateFile() = %+v, want file 42 ORDER-001", file)
	}
	if file.LocatePage != 3 || file.LocatePageSize != 60 {
		t.Fatalf("locate page = %d/%d, want 3/60", file.LocatePage, file.LocatePageSize)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
