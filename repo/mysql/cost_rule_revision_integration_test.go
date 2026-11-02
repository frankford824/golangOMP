package mysqlrepo_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

// Run against an isolated clone of the production table definitions. This
// exercises the real seed unique key, which an in-memory rule stub cannot catch.
func TestCostRuleRevisionIdentityIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_COST_RULE_REVISION_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var database string
	if err = db.QueryRow(`SELECT DATABASE()`).Scan(&database); err != nil || !strings.HasPrefix(database, "codex_cost_rule_revision_test_") {
		t.Fatal("refusing non-test database")
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM cost_rules`)
	exec(`DELETE FROM cost_rule_bindings`)
	exec(`DELETE FROM categories`)
	exec(`ALTER TABLE cost_rules DROP INDEX uq_cost_rules_seed_identity, ADD UNIQUE KEY uq_cost_rules_seed_identity(rule_name,category_code,rule_type,priority,source,special_process_keyword)`)
	exec(`INSERT INTO categories(id,category_code,category_name,display_name,remark) VALUES(1,'HSC','超透PVC','超透PVC','')`)
	exec(`INSERT INTO cost_rule_bindings(i_id_raw,normalized_i_id,rule_group,display_name,source,is_active) VALUES('超透PVC','超透PVC','HSC','超透PVC','test',1)`)
	wrapper := mysqlrepo.New(db)
	rules := mysqlrepo.NewCostRuleRepo(wrapper)
	svc := service.NewCostRuleService(rules, mysqlrepo.NewCategoryRepo(wrapper), wrapper)
	ctx := context.Background()
	original := `{"version":1,"material":"HSC","basis":"area","unit_price":10,"multiplier":1}`
	first, appErr := svc.Create(ctx, service.CreateCostRuleParams{RuleName: "超透PVC", CategoryCode: "HSC", RuleType: domain.CostRuleTypeModel, FormulaExpression: original})
	if appErr != nil {
		t.Fatal(appErr)
	}
	revised := `{"version":1,"material":"超透PVC","basis":"area","unit_price":10,"multiplier":1,"variant_prices":[{"thickness_mm":0,"normal_price":10,"irregular_price":14}]}`
	if _, appErr = svc.Patch(ctx, service.PatchCostRuleParams{RuleID: first.RuleID, FormulaExpression: &revised}); appErr == nil {
		t.Fatal("old unique key did not reproduce save failure")
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM cost_rules`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed edit partially committed: %d %v", count, err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "147_cost_rule_revision_identity.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(migration))
	second, appErr := svc.Patch(ctx, service.PatchCostRuleParams{RuleID: first.RuleID, FormulaExpression: &revised})
	if appErr != nil || second.RuleID == first.RuleID || second.RuleVersion != 2 {
		t.Fatalf("revision save failed: %+v %v", second, appErr)
	}
	thirdRaw := strings.Replace(revised, `"irregular_price":14`, `"irregular_price":16`, 1)
	third, appErr := svc.Patch(ctx, service.PatchCostRuleParams{RuleID: second.RuleID, FormulaExpression: &thirdRaw})
	if appErr != nil || third.RuleVersion != 3 {
		t.Fatalf("repeated revision save failed: %+v %v", third, appErr)
	}
	old, appErr := svc.GetByID(ctx, first.RuleID)
	if appErr != nil || old.FormulaExpression != original || old.RuleName != "超透PVC" {
		t.Fatal("original price or name was rewritten")
	}
	old, appErr = svc.GetByID(ctx, second.RuleID)
	if appErr != nil || old.FormulaExpression != revised {
		t.Fatal("second revision was rewritten")
	}
	_, appErr = svc.Patch(ctx, service.PatchCostRuleParams{RuleID: first.RuleID, FormulaExpression: &thirdRaw})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("stale edit forked model: %v", appErr)
	}
	preview, appErr := svc.Preview(ctx, domain.CostRulePreviewRequest{CategoryCode: "HSC", Input: &domain.CostInput{AreaMode: "total", AreaM2: .5, Pieces: 4, Shape: "irregular"}})
	if appErr != nil || preview.EstimatedCost == nil || *preview.EstimatedCost != 8 {
		t.Fatalf("latest revision was not used: %+v %v", preview, appErr)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM cost_rule_bindings WHERE rule_group='HSC' AND i_id_raw='超透PVC' AND is_active=1`).Scan(&count); err != nil || count != 1 {
		t.Fatal("binding changed")
	}
	// Re-applying the index-only DDL also retains every stored formula.
	exec(string(migration))
	if err = db.QueryRow(`SELECT COUNT(*) FROM cost_rules`).Scan(&count); err != nil || count != 3 {
		t.Fatal("migration rewrote rule rows")
	}
}
