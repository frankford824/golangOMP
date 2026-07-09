package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

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
		WithArgs("CGK000181", 20).
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

func TestSearchProductsFromDocumentsTextKeywordUsesFullText(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(searchRepoQueryMatcher(t)))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("schema-table").
		WithArgs("product_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-column").
		WithArgs("product_search_documents", "semantic_text").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("product-doc-text").
		WithArgs("常规kt板", 20).
		WillReturnRows(sqlmock.NewRows([]string{"erp_code", "product_name", "i_id", "category"}).
			AddRow("CGK000181", "常规kt板", "IID-1", "KT板"))

	repo := NewSearchRepo(New(db))
	items, err := repo.SearchProducts(context.Background(), "常规kt板", 20)
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
		WithArgs("常规kt板", 20).
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
			if !strings.Contains(normalized, "MATCH(d.search_text) AGAINST (? IN NATURAL LANGUAGE MODE)") {
				return fmt.Errorf("asset document query missing fulltext match: %s", normalized)
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
			if !strings.Contains(normalized, "MATCH(search_text) AGAINST (? IN NATURAL LANGUAGE MODE)") {
				return fmt.Errorf("product text query missing fulltext match: %s", normalized)
			}
			if strings.Contains(normalized, "UNION ALL") {
				return fmt.Errorf("product text query must not use code union: %s", normalized)
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
			if !strings.Contains(normalized, "MATCH(d.search_text) AGAINST (? IN NATURAL LANGUAGE MODE)") {
				return fmt.Errorf("task text query missing fulltext match: %s", normalized)
			}
			if strings.Contains(normalized, "UNION ALL") {
				return fmt.Errorf("task text query must not use code union: %s", normalized)
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})
}
