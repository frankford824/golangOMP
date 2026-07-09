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

func TestSearchProductsFromDocumentsCombinesPrefixAndFullTextForCodeKeyword(t *testing.T) {
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
	mock.ExpectQuery("product-doc-search").
		WithArgs("CGK000181", "CGK000181", "CGK000181%", "CGK000181%", "CGK000181", "CGK000181", "CGK000181", "CGK000181%", 20).
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
		case "product-doc-search":
			for _, fragment := range []string{
				"FROM product_search_documents",
				"sku_code = ?",
				"sku_code LIKE ?",
				"MATCH(search_text) AGAINST (? IN NATURAL LANGUAGE MODE)",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("product document query missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})
}
