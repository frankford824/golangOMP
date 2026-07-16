package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestListResourceGroupsUsesSameLaneFormatAndScopeForCountAndPage(t *testing.T) {
	matched := 0
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected != "resource-group-filter" {
			return fmt.Errorf("unexpected expectation %q", expected)
		}
		for _, token := range []string{
			"g.finalized_revision_id IS NOT NULL",
			"t.business_lane = ?",
			"task_asset_group_revisions gr",
			"task_asset_group_revision_items ri",
			"gr.source_task_asset_id",
			"ri.revision_id = g.finalized_revision_id",
			"LOWER(ta.file_name) LIKE ?",
			"t.owner_department_id IN (?)",
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
	mock.ExpectQuery("resource-group-filter").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("resource-group-filter").WillReturnRows(sqlmock.NewRows([]string{
		"id", "task_id", "scope_kind", "task_sku_item_id", "retouch_requirement_id", "working_revision_id", "finalized_revision_id", "lock_version",
		"migration_incomplete", "migration_issue", "created_at", "updated_at", "task_no", "sku_code", "business_lane",
	}))
	items, total, err := repository.ListResourceGroups(context.Background(), domain.ResourceGroupListParams{
		BusinessLane: domain.TaskBusinessLaneCustomization, FormatCategory: domain.AssetFormatCategoryDesign,
		Page: 2, PageSize: 25, Access: domain.ResourceGroupAccessFilter{DepartmentIDs: []int64{101}},
	})
	if err != nil {
		t.Fatalf("ListResourceGroups() error = %v", err)
	}
	if len(items) != 0 || total != 0 || matched != 2 {
		t.Fatalf("items/total/matched = %d/%d/%d", len(items), total, matched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
