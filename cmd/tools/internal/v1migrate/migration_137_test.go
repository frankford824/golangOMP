package v1migrate

import (
	"strings"
	"testing"
)

func TestMigration137CreatesImmutableCostChangeTriggers(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "137_jst_cost_change_stream.sql")
	if err != nil {
		t.Fatalf("ReadForwardSQL() error = %v", err)
	}
	statements := SplitSQLStatements(forward)
	if len(statements) != 5 {
		t.Fatalf("statement count = %d, want 5: %#v", len(statements), statements)
	}
	normalized := strings.Join(strings.Fields(forward), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS jst_cost_changes",
		"CREATE TRIGGER trg_jst_inventory_cost_insert AFTER INSERT ON jst_inventory",
		"WHERE NEW.cost_price IS NOT NULL",
		"CREATE TRIGGER trg_jst_inventory_cost_update AFTER UPDATE ON jst_inventory",
		"WHERE NOT (OLD.cost_price <=> NEW.cost_price)",
		"source_modified_at",
		"changed_at DATETIME(3)",
	} {
		if !strings.Contains(normalized, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
