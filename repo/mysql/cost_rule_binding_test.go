package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestListUnboundCandidatesGroupsByNormalizedExpression(t *testing.T) {
	queryCount := 0
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "unbound-candidates" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		queryCount++
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if strings.Contains(normalized, "GROUP BY normalized_i_id") {
			return fmt.Errorf("unbound candidate SQL must not group by alias under ONLY_FULL_GROUP_BY: %s", normalized)
		}
		if !strings.Contains(normalized, "GROUP BY UPPER(REPLACE(REPLACE(TRIM(COALESCE(NULLIF(pm.erp_i_id, ''), pm.product_i_id))") {
			return fmt.Errorf("unbound candidate SQL missing normalized expression group by: %s", normalized)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	bindingRepo := NewCostRuleBindingRepo(&DB{db: db})
	mock.ExpectQuery("unbound-candidates").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery("unbound-candidates").
		WillReturnRows(sqlmock.NewRows([]string{
			"erp_i_id",
			"product_i_id",
			"normalized_i_id",
			"suggested_rule_group",
			"match_count",
			"example_sku_code",
			"example_task_no",
			"average_cost_price",
		}))

	items, total, err := bindingRepo.ListUnboundCandidates(context.Background(), repo.UnboundCostRuleCandidateFilter{})
	if err != nil {
		t.Fatalf("ListUnboundCandidates() error = %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("items=%d total=%d, want empty result", len(items), total)
	}
	if queryCount != 2 {
		t.Fatalf("query count = %d, want 2", queryCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
