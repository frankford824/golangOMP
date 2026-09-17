package v1migrate

import (
	"strings"
	"testing"
)

func TestMigration139AddsScopedPPStickyRulesAndBinding(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "139_pp_sticky_cost_rules.sql")
	if err != nil {
		t.Fatalf("read migration 139: %v", err)
	}
	normalized := strings.Join(strings.Fields(forward), " ")
	for _, required := range []string{
		"'常规PP背胶面积成本'",
		"'fixed_unit_price', 8.000",
		"'常规PP背胶打孔附加'",
		"'special_process_surcharge', '打孔', 1.000",
		"'常规PP背胶', '常规PP背胶', 'PP_STICKY'",
		"NOT EXISTS",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("migration 139 missing %q", required)
		}
	}
	if statements := SplitSQLStatements(forward); len(statements) != 3 {
		t.Fatalf("migration 139 split into %d statements, want 3", len(statements))
	}
}
