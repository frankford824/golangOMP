package mysqlrepo

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"workflow/repo"
)

func TestBoundSKUQueryUsesLiveBindingAndIncludesUnfiledSKUs(t *testing.T) {
	if strings.Count(costBoundSKUFrom, "(") != strings.Count(costBoundSKUFrom, ")") {
		t.Fatal("unbalanced query parentheses")
	}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, query string) error {
		for _, required := range []string{"FROM task_sku_items s", "eb.normalized_i_id_active =", "pb.normalized_i_id_active =", "COALESCE(eb.rule_group,pb.rule_group) = ?", "cr.effective_from", "cr.effective_to", "s.sku_code LIKE ?"} {
			if !strings.Contains(query, required) {
				return fmt.Errorf("missing %s", required)
			}
		}
		if strings.Contains(query, "cost_snapshot") || strings.Contains(query, "erp_product_sync_records") {
			return fmt.Errorf("historical/ERP projection must not control SKU membership")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("count").WithArgs("KT", sqlmock.AnyArg(), sqlmock.AnyArg(), "%CGK%", "%CGK%", "%CGK%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(21))
	mock.ExpectQuery("rows").WithArgs("KT", sqlmock.AnyArg(), sqlmock.AnyArg(), "%CGK%", "%CGK%", "%CGK%", 20, 20).WillReturnRows(sqlmock.NewRows([]string{"id", "sku_code", "product_name_snapshot", "style", "cost_price", "sku_status"}).AddRow(4, "CGK4", "测试", "款式", nil, "generated"))
	r := &costRuleBindingRepo{db: &DB{db: db}}
	rows, total, err := r.ListBoundSKUs(context.Background(), repo.CostRuleBindingListFilter{RuleGroup: "KT", Keyword: "CGK", Page: 2, PageSize: 20})
	if err != nil || total != 21 || len(rows) != 1 || rows[0].CostPrice != nil {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
