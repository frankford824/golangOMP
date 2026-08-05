package mysqlrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestListFlatResourceItemsFiltersCountsAndPagesFilesWithSameScope(t *testing.T) {
	matched := 0
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected == "flat-resource-integrity" {
			for _, token := range []string{
				"SELECT violation_code, entity_id",
				"source_ownership",
				"final_ownership",
				"reference_ownership",
				"ta.bound_group_id IS NULL OR ta.bound_group_id <> g.id",
				"ta.bound_role IS NULL OR ta.bound_role <> 'source'",
				"ta.bound_role IS NULL OR ta.bound_role <> 'final'",
				"COALESCE(ta.is_archived, 0) <> 0",
				"ta.storage_ref_id IS NULL OR asr.ref_id IS NULL",
				"COALESCE(asr.status, '') IN ('archived', 'historical_unavailable')",
				"rr.ref_id_snapshot <> f.ref_id",
				"rr.scope_snapshot <> CASE",
				"CONCAT('retouch_requirement:', f.retouch_requirement_id)",
				"CONCAT('sku:', f.sku_item_id)",
				"COALESCE(formal_ta.is_archived, 0) <> 0",
				"formal_ta.asset_type <> 'reference'",
				"formal_ta.bound_role IS NOT NULL AND TRIM(formal_ta.bound_role) <> ''",
				"formal_ta.storage_ref_id <> rr.ref_id_snapshot",
				"formal_ta.storage_ref_id IS NULL",
				"formal_asr.ref_id IS NULL",
				"COALESCE(formal_asr.status, '') IN ('archived', 'historical_unavailable')",
			} {
				if !strings.Contains(actual, token) {
					return fmt.Errorf("integrity query missing %q: %s", token, actual)
				}
			}
			if strings.Contains(actual, "'superseded'") {
				return fmt.Errorf("integrity query must not reject superseded storage refs: %s", actual)
			}
			matched++
			return nil
		}
		if expected != "flat-resource-filter" {
			return fmt.Errorf("unexpected expectation %q", expected)
		}
		for _, token := range []string{
			"WITH flat_resources AS",
			"task_asset_group_revision_references rr",
			"JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id",
			"JOIN task_asset_group_revisions rev",
			"JOIN task_asset_group_revision_items ri",
			"flat.resource_role = ?",
			"flat.resource_owner_id = ?",
			"flat.resource_created_at >= ?",
			"flat.resource_created_at <= ?",
			"flat.file_name LIKE ?",
			"LOWER(flat.file_name) LIKE ?",
			"t.task_type = ?",
			"t.owner_department_id IN (?)",
			"f.task_id = g.task_id",
			"rr.ref_id_snapshot = f.ref_id",
			"rr.scope_snapshot = CASE",
			"ta.task_id = g.task_id AND ta.asset_type = 'source'",
			"ta.task_id = g.task_id AND ta.asset_type = 'delivery'",
			"ta.binding_state = 'bound'",
			"ta.bound_group_id = g.id AND ta.bound_role = 'source'",
			"ta.bound_group_id = g.id AND ta.bound_role = 'final'",
			"COALESCE(ta.is_archived, 0) = 0",
			"ta.access_revoked_at IS NULL",
			"ta.storage_ref_id IS NOT NULL AND asr.ref_id IS NOT NULL",
			"COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')",
			"formal_asr.ref_id IS NOT NULL",
			"formal_ta.asset_type = 'reference'",
			"formal_ta.bound_role IS NULL OR TRIM(formal_ta.bound_role) = ''",
			"formal_ta.storage_ref_id = rr.ref_id_snapshot",
			"COALESCE(formal_asr.status, '') NOT IN ('archived', 'historical_unavailable')",
		} {
			if !strings.Contains(actual, token) {
				return fmt.Errorf("query missing %q: %s", token, actual)
			}
		}
		if strings.Contains(actual, "'superseded'") {
			return fmt.Errorf("flat query must not reject superseded storage refs: %s", actual)
		}
		matched++
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	mock.ExpectQuery("flat-resource-integrity").
		WillReturnRows(sqlmock.NewRows([]string{"violation_code", "entity_id"}))
	mock.ExpectQuery("flat-resource-filter").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery("flat-resource-filter").WillReturnRows(sqlmock.NewRows([]string{
		"group_id", "task_id", "task_no", "task_type", "sku_code", "resource_role", "file_name", "mime_type",
		"resource_owner_id", "resource_owner_name", "resource_created_at", "storage_key", "task_asset_id",
	}).AddRow(8, 3, "RW-008", "new_product_development", "SKU-008", "source", "source.psd",
		"image/vnd.adobe.photoshop", 42, "设计师", time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC), "tasks/3/source.psd", 88))
	items, total, err := repository.ListFlatResourceItems(context.Background(), domain.ResourceGroupListParams{
		ResourceRole:   domain.ResourceRoleFilterSource,
		Query:          "source",
		FormatCategory: domain.AssetFormatCategoryDesign,
		FileFormat:     "psd",
		ResourceOwnerID: int64Ptr(42),
		ResourceCreatedFrom: timePtr(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)),
		ResourceCreatedTo:   timePtr(time.Date(2026, 8, 5, 23, 59, 59, 0, time.UTC)),
		TaskType:       domain.TaskTypeNewProductDevelopment,
		Page:           3, PageSize: 2,
		Access: domain.ResourceGroupAccessFilter{DepartmentIDs: []int64{101}},
	})
	if err != nil {
		t.Fatalf("ListFlatResourceItems() error = %v", err)
	}
	if total != 5 || len(items) != 1 || items[0].TaskAssetID != 88 || items[0].FileName != "source.psd" || items[0].StorageKey == "" {
		t.Fatalf("items/total = %+v/%d", items, total)
	}
	if matched != 3 {
		t.Fatalf("matched query count = %d, want 3", matched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func TestListFlatResourceItemsFailsClosedOnIntegrityViolation(t *testing.T) {
	tests := []struct {
		name          string
		violationCode string
		entityID      int64
	}{
		{name: "source null bound group", violationCode: "source_ownership", entityID: 71},
		{name: "source null bound role", violationCode: "source_ownership", entityID: 72},
		{name: "final null bound group", violationCode: "final_ownership", entityID: 73},
		{name: "final null bound role", violationCode: "final_ownership", entityID: 74},
		{name: "task asset missing storage ref", violationCode: "final_ownership", entityID: 75},
		{name: "task asset storage row missing", violationCode: "source_ownership", entityID: 76},
		{name: "task asset archived flag", violationCode: "final_ownership", entityID: 77},
		{name: "task asset archived storage", violationCode: "source_ownership", entityID: 78},
		{name: "task asset historical unavailable storage", violationCode: "final_ownership", entityID: 79},
		{name: "reference storage row missing", violationCode: "reference_ownership", entityID: 80},
		{name: "formal asset storage ref missing", violationCode: "reference_ownership", entityID: 81},
		{name: "formal asset storage row missing", violationCode: "reference_ownership", entityID: 82},
		{name: "formal asset archived flag", violationCode: "reference_ownership", entityID: 83},
		{name: "reference archived storage", violationCode: "reference_ownership", entityID: 84},
		{name: "formal historical unavailable storage", violationCode: "reference_ownership", entityID: 85},
		{name: "reference cross scope", violationCode: "reference_ownership", entityID: 86},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			mock.ExpectQuery(`SELECT violation_code, entity_id`).
				WillReturnRows(sqlmock.NewRows([]string{"violation_code", "entity_id"}).
					AddRow(test.violationCode, test.entityID))

			items, total, err := repository.ListFlatResourceItems(
				context.Background(),
				domain.ResourceGroupListParams{
					ResourceRole: domain.ResourceRoleFilterFinal,
					Page:         1,
					PageSize:     20,
					Access:       domain.ResourceGroupAccessFilter{Global: true},
				},
			)
			if items != nil || total != 0 || !errors.Is(err, repo.ErrDataIntegrity) {
				t.Fatalf("items/total/error = %+v/%d/%v", items, total, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
