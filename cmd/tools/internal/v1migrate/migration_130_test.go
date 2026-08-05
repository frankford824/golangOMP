package v1migrate

import (
	"strings"
	"testing"
)

func TestMigration130ForwardSQLSplitsCleanly(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "130_activate_legacy_purchase_sku_planning_rule.sql")
	if err != nil {
		t.Fatalf("read migration 130: %v", err)
	}
	stmts := SplitSQLStatements(forward)
	if len(stmts) < 10 {
		t.Fatalf("migration 130 split into only %d statements", len(stmts))
	}
	var prepareCount int
	for _, stmt := range stmts {
		if strings.HasPrefix(strings.ToUpper(stmt), "PREPARE ") {
			prepareCount++
		}
	}
	if prepareCount != 2 {
		t.Fatalf("expected two fail-closed PREPARE guards, got %d", prepareCount)
	}
}

func TestMigration130KeepsRevisionOneImmutableAndUsesLegacySequenceStore(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "130_activate_legacy_purchase_sku_planning_rule.sql")
	if err != nil {
		t.Fatalf("read migration 130: %v", err)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(forward), " "))
	for _, required := range []string{
		"version_no, prefix",
		"'legacy_task_product_code_v1'",
		"'regular', 'cg'",
		"'customization', 'dz'",
		"'sequence_store', 'product_code_sequences'",
		"'supersedes_rule_revision_id', @planning_previous_revision_id",
		"active_revision_id = @planning_revision_id",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("migration 130 is missing required fragment %q", required)
		}
	}
	if strings.Contains(normalized, "update code_rule_revisions") {
		t.Fatal("migration 130 must not mutate an immutable code-rule revision")
	}
	if strings.Contains(normalized, "update task_sku_items") {
		t.Fatal("migration 130 must not rewrite historical SKU identities")
	}
	if strings.Contains(normalized, "'supersedes_rule_revision_id', 9") {
		t.Fatal("migration 130 must not assume a production revision id")
	}
}
