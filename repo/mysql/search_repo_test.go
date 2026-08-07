package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestSearchResourceGroupsAppliesEffectiveScope(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("schema-table").WithArgs("task_asset_group_search_documents").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("resource-group-scope").
		WithArgs("%x%", int64(41), int64(41), int64(41), int64(41), int64(3), int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "task_id", "task_no", "sku_code", "revision_id", "mode", "count", "file_name"}).AddRow(9, 8, "T-8", "SKU-8", 7, "set", 2, "final.png"))
	repository := NewSearchRepo(New(db)).(*searchRepo)
	items, err := repository.SearchResourceGroups(context.Background(), "x", 20, false, domain.ResourceGroupAccessFilter{ActorID: 41, Self: true, DepartmentIDs: []int64{3}, TeamIDs: []int64{7}})
	if err != nil || len(items) != 1 || items[0].ResourceGroupID != 9 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchResourceGroupsPublishedBranchPinsFinalizedRevision(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("schema-table").WithArgs("task_asset_group_search_documents").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("resource-group-published").WithArgs("%x%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "task_id", "task_no", "sku_code", "revision_id", "mode", "count", "file_name"}).AddRow(9, 8, "T-8", "SKU-8", 7, "single", 1, "final.png"))
	repository := NewSearchRepo(New(db)).(*searchRepo)
	items, err := repository.SearchResourceGroups(context.Background(), "x", 20, true, domain.ResourceGroupAccessFilter{})
	if err != nil || len(items) != 1 || items[0].FinalizedRevisionID != 7 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchTasksScopedAppliesStableOrganizationScope(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("schema-table").WithArgs("task_search_documents").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("task-doc-scoped").
		WithArgs("%x%", "%x%", "%x%", "%x%", "%x%", int64(51), int64(51), int64(51), int64(51), int64(3), int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "task_no", "title", "status", "priority", "task_type", "sku_code", "primary_sku_code", "product_i_id", "owner_department", "owner_team", "owner_org_team", "creator_id", "creator_name", "designer_id", "designer_name", "created_at", "deadline_at"}).
			AddRow(2, "T-2", "title", "InProgress", "normal", "new_product_development", "SKU", "SKU", nil, "D", "T", "OT", 51, "creator", nil, nil, nil, nil))
	repository := NewSearchRepo(New(db)).(*searchRepo)
	items, err := repository.SearchTasksScoped(context.Background(), "x", 20, domain.ResourceGroupAccessFilter{ActorID: 51, Self: true, DepartmentIDs: []int64{3}, TeamIDs: []int64{7}})
	if err != nil || len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAssetsFromDocumentsUsesFullTextForCodeKeyword(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("asset_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-column").
		WithArgs("asset_search_documents", "semantic_text").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("asset-doc-search").
		WithArgs(`"CGK000181"`, `"CGK000181"`, 20).
		WillReturnRows(sqlmock.NewRows([]string{"asset_id", "file_name", "source_module_key", "task_id", "asset_type", "flow_review_status"}).
			AddRow(int64(7), "delivery.jpg", "design", int64(3), "delivery", "approved"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchAssets(context.Background(), "CGK000181", 20)
	if err != nil {
		t.Fatalf("SearchAssets() error = %v", err)
	}
	if len(items) != 1 || items[0].AssetID != 7 {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchProductsFromDocumentsCodeKeywordUsesUnionWithoutFullText(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("product_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("product-doc-code").
		WithArgs("CGK000181", "CGK000181", "CGK000181%", "CGK000181%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"erp_code", "product_name", "i_id", "category"}).
			AddRow("CGK000181", "测试产品", "IID-1", "KT板"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchProducts(context.Background(), "CGK000181", 20)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(items) != 1 || items[0].ERPCode != "CGK000181" {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchProductsFromDocumentsTextKeywordUsesBoundedFallbackBeforeIndexReady(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("product_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-table").
		WithArgs("product_search_ngrams").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("schema-column").
		WithArgs("product_search_documents", "semantic_text").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("product-doc-text").
		WithArgs("%poster%board%", "%poster%board%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"erp_code", "product_name", "i_id", "category"}).
			AddRow("CGK000181", "poster board", "IID-1", "board"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchProducts(context.Background(), "poster board", 20)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchProductsFromDocumentsTextKeywordIncludesSemanticSubstring(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("product_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-table").
		WithArgs("product_search_ngrams").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("schema-column").
		WithArgs("product_search_documents", "semantic_text").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("product-doc-text").
		WithArgs("%rare%term%", "%rare%term%", "%rare%term%", "%rare%term%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"erp_code", "product_name", "i_id", "category"}).
			AddRow("CGK000999", "semantic match", "IID-999", "other"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchProducts(context.Background(), "rare term", 20)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(items) != 1 || items[0].ERPCode != "CGK000999" {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchProductsFromDocumentsTextKeywordUsesNgramIndexWhenReady(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("product_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-table").
		WithArgs("product_search_ngrams").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-table").
		WithArgs("product_search_index_state").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("product-index-ready").
		WithArgs(productSearchNgramIndexName, productSearchNgramIndexVersion).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-column").
		WithArgs("product_search_documents", "semantic_text").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("product-doc-ngram").
		WithArgs("医师", "师节", 2, 1000, "%医师节%", "%医师节%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"erp_code", "product_name", "i_id", "category"}).
			AddRow("CGK001543", "医师节手举牌", "IID-1543", "KT板"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchProducts(context.Background(), "医师节", 20)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(items) != 1 || items[0].ERPCode != "CGK001543" {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchTasksFromDocumentsCodeKeywordUsesUnionRecall(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	// 4 exact + 4 prefix branches, then LIMIT.
	mock.ExpectQuery("task-doc-code").
		WithArgs("CGK000181", "CGK000181", "CGK000181", "CGK000181", "CGK000181%", "CGK000181%", "CGK000181%", "CGK000181%", 20).
		WillReturnRows(searchTaskDocumentRows().AddRow(
			int64(3), "T-0003", "常规kt板", "InProgress", "high",
			"custom", "CGK000181", "CGK000181", "IID-1",
			"dept", "team", "org", int64(1), "creator", int64(2), "designer",
			nil, nil))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchTasks(context.Background(), "CGK000181", 20)
	if err != nil {
		t.Fatalf("SearchTasks() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 3 {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchTasksFromDocumentsTextKeywordUsesFullText(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("task-doc-text").
		WithArgs(`"常规kt板"`, `"常规kt板"`, 20).
		WillReturnRows(searchTaskDocumentRows().AddRow(
			int64(3), "T-0003", "常规kt板", "InProgress", "high",
			"custom", "CGK000181", "CGK000181", "IID-1",
			"dept", "team", "org", int64(1), "creator", int64(2), "designer",
			nil, nil))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchTasks(context.Background(), "常规kt板", 20)
	if err != nil {
		t.Fatalf("SearchTasks() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 3 {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func searchTaskDocumentRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"task_id", "task_no", "product_name_snapshot", "task_status", "priority",
		"task_type", "sku_code", "primary_sku_code", "product_i_id",
		"owner_department", "owner_team", "owner_org_team",
		"creator_id", "creator_name", "designer_id", "designer_name",
		"created_at", "deadline_at",
	})
}

func searchRepoQueryMatcher(t *testing.T) sqlmock.QueryMatcher {
	t.Helper()
	return sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "schema-table":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table schema SQL unexpected: %s", normalized)
			}
		case "schema-column":
			if !strings.Contains(normalized, "information_schema.columns") || !strings.Contains(normalized, "column_name = ?") {
				return fmt.Errorf("column schema SQL unexpected: %s", normalized)
			}
		case "asset-doc-search":
			if !strings.Contains(normalized, "FROM asset_search_documents d") {
				return fmt.Errorf("asset document query missing document table: %s", normalized)
			}
			if strings.Contains(normalized, "d.asset_id = ?") {
				return fmt.Errorf("asset code keyword must not add numeric id predicates: %s", normalized)
			}
			for _, fragment := range []string{
				"MATCH(search_text) AGAINST (? IN BOOLEAN MODE)",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("asset document query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "IN NATURAL LANGUAGE MODE") {
				return fmt.Errorf("asset phrase query must not run natural fallback in same SQL: %s", normalized)
			}
		case "resource-group-scope":
			for _, fragment := range []string{"FROM task_asset_group_search_documents d", "t.creator_id = ?", "t.owner_department_id IN (?)", "t.owner_team_id IN (?)"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("resource-group scope query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "asset_workbench_client_materials") {
				return fmt.Errorf("internal scope query unexpectedly uses publication gate: %s", normalized)
			}
		case "resource-group-published":
			for _, fragment := range []string{"d.final_text LIKE ?", "asset_workbench_client_materials cm", "cm.finalized_revision_id = g.finalized_revision_id", "cm.enabled = 1"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("published resource-group query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "t.creator_id = ?") || strings.Contains(normalized, "1 = 0") {
				return fmt.Errorf("published query must not inherit internal scope: %s", normalized)
			}
		case "product-doc-code":
			if !strings.Contains(normalized, "FROM product_search_documents") {
				return fmt.Errorf("product code query missing document table: %s", normalized)
			}
			if !strings.Contains(normalized, "UNION ALL") {
				return fmt.Errorf("product code query must use UNION ALL recall: %s", normalized)
			}
			if strings.Contains(normalized, "MATCH(") {
				return fmt.Errorf("product code query must not use fulltext match: %s", normalized)
			}
		case "product-doc-text":
			for _, fragment := range []string{"FROM product_search_documents", "search_text LIKE ?", "product_name LIKE ?"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("product text query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "MATCH(") {
				return fmt.Errorf("product text query must avoid the slow fulltext path: %s", normalized)
			}
		case "product-index-ready":
			for _, fragment := range []string{"FROM product_search_index_state", "index_name = ?", "index_version = ?"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("product index readiness query missing %q: %s", fragment, normalized)
				}
			}
		case "product-doc-ngram":
			for _, fragment := range []string{"FROM product_search_ngrams n", "n.term IN (?,?)", "HAVING COUNT(DISTINCT n.term) = ?", "search_text LIKE ?"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("product ngram query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "MATCH(") {
				return fmt.Errorf("product ngram query must not use fulltext: %s", normalized)
			}
		case "task-doc-scoped":
			for _, fragment := range []string{"FROM task_search_documents d JOIN tasks t", "t.creator_id = ?", "t.owner_department_id IN (?)", "t.owner_team_id IN (?)"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("scoped task query missing %q: %s", fragment, normalized)
				}
			}
		case "task-doc-code":
			if !strings.Contains(normalized, "FROM task_search_documents d") {
				return fmt.Errorf("task code query missing hydrate table: %s", normalized)
			}
			if !strings.Contains(normalized, "UNION ALL") {
				return fmt.Errorf("task code query must use UNION ALL recall: %s", normalized)
			}
			if strings.Contains(normalized, "MATCH(") {
				return fmt.Errorf("task code query must not use fulltext match: %s", normalized)
			}
		case "task-doc-text":
			for _, fragment := range []string{
				"MATCH(search_text) AGAINST (? IN BOOLEAN MODE)",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("task text query missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "IN NATURAL LANGUAGE MODE") {
				return fmt.Errorf("task phrase query must not run natural fallback in same SQL: %s", normalized)
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})
}
