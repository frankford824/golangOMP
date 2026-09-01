package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCostRuleListActiveExcludesEffectiveSupersededRules(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"NOT EXISTS ( SELECT 1 FROM cost_rules successor",
			"successor.supersedes_rule_id = cost_rules.id",
			"successor.is_active = 1",
			"successor.effective_from IS NULL OR successor.effective_from <= ?",
			"successor.effective_to IS NULL OR successor.effective_to >= ?",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("active cost-rule query missing %q", fragment)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	asOf := time.Date(2026, 9, 1, 2, 15, 0, 0, time.UTC)
	mock.ExpectQuery("active-cost-rules").
		WithArgs("ACRYLIC", asOf, asOf, asOf, asOf).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "rule_name", "rule_version", "category_id", "category_code", "product_family", "rule_type",
			"base_price", "tax_multiplier", "min_area", "area_threshold", "surcharge_amount",
			"special_process_keyword", "special_process_price", "formula_expression", "priority", "is_active",
			"effective_from", "effective_to", "supersedes_rule_id", "superseded_by_rule_id", "governance_note", "source", "remark", "created_at", "updated_at",
		}))

	repository := NewCostRuleRepo(New(db))
	rules, err := repository.ListActiveByCategory(context.Background(), nil, "ACRYLIC", asOf)
	if err != nil {
		t.Fatalf("ListActiveByCategory() error = %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %+v, want empty mock result", rules)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
