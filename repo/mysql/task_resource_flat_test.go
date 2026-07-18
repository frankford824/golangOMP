package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestListFlatResourceItemsFiltersCountsAndPagesFilesWithSameScope(t *testing.T) {
	matched := 0
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected != "flat-resource-filter" {
			return fmt.Errorf("unexpected expectation %q", expected)
		}
		for _, token := range []string{
			"WITH flat_resources AS",
			"task_asset_group_revision_references rr",
			"JOIN task_asset_group_revisions rev",
			"JOIN task_asset_group_revision_items ri",
			"flat.resource_role = ?",
			"flat.file_name LIKE ?",
			"LOWER(flat.file_name) LIKE ?",
			"t.owner_department_id IN (?)",
			"ta.binding_state = 'bound'",
			"ta.access_revoked_at IS NULL",
		} {
			if !strings.Contains(actual, token) {
				return fmt.Errorf("query missing %q: %s", token, actual)
			}
		}
		matched++
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	mock.ExpectQuery("flat-resource-filter").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery("flat-resource-filter").WillReturnRows(sqlmock.NewRows([]string{
		"group_id", "task_id", "task_no", "sku_code", "resource_role", "file_name", "mime_type", "storage_key",
	}).AddRow(8, 3, "RW-008", "SKU-008", "source", "source.psd", "image/vnd.adobe.photoshop", "tasks/3/source.psd"))
	items, total, err := repository.ListFlatResourceItems(context.Background(), domain.ResourceGroupListParams{
		ResourceRole:   domain.ResourceRoleFilterSource,
		Query:          "source",
		FormatCategory: domain.AssetFormatCategoryDesign,
		Page:           3, PageSize: 2,
		Access: domain.ResourceGroupAccessFilter{DepartmentIDs: []int64{101}},
	})
	if err != nil {
		t.Fatalf("ListFlatResourceItems() error = %v", err)
	}
	if total != 5 || len(items) != 1 || items[0].FileName != "source.psd" || items[0].StorageKey == "" {
		t.Fatalf("items/total = %+v/%d", items, total)
	}
	if matched != 2 {
		t.Fatalf("matched query count = %d, want 2", matched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
