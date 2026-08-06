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
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("unbound-candidates").
		WillReturnRows(sqlmock.NewRows([]string{
			"erp_i_id",
			"product_i_id",
			"normalized_i_id",
			"suggested_rule_groups",
			"suggested_group_count",
			"match_count",
			"example_sku_code",
			"example_task_no",
			"average_cost_price",
		}).AddRow("STYLE-01", "", "STYLE-01", "KT_A,KT_B", int64(2), int64(7), "SKU-01", "RW-01", 12.5))

	items, total, err := bindingRepo.ListUnboundCandidates(context.Background(), repo.UnboundCostRuleCandidateFilter{})
	if err != nil {
		t.Fatalf("ListUnboundCandidates() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("items=%d total=%d, want one result", len(items), total)
	}
	if items[0].MappingConfidence != "conflict" || items[0].SuggestedRuleGroup != "" {
		t.Fatalf("candidate = %+v, want conflict without a single suggested group", items[0])
	}
	if got := strings.Join(items[0].SuggestedRuleGroups, ","); got != "KT_A,KT_B" {
		t.Fatalf("suggested groups = %q, want KT_A,KT_B", got)
	}
	if queryCount != 2 {
		t.Fatalf("query count = %d, want 2", queryCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
