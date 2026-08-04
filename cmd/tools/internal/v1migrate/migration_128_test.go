package v1migrate

import (
	"path/filepath"
	"strings"
	"testing"
)

// migrationsDir resolves db/migrations relative to this test package
// (cmd/tools/internal/v1migrate → repo root is four levels up).
func migrationsDir() string {
	return filepath.Join("..", "..", "..", "..", "db", "migrations")
}

func TestMigration128ForwardSQLSplitsCleanly(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "128_access_operations.sql")
	if err != nil {
		t.Fatalf("read migration 128: %v", err)
	}
	stmts := SplitSQLStatements(forward)
	if len(stmts) == 0 {
		t.Fatal("migration 128 split into zero statements")
	}
	// The PREPARE guard embeds ALTER TABLE text inside single-quoted strings;
	// the splitter must not cut inside those strings.
	var prepareCount int
	for _, stmt := range stmts {
		if strings.HasPrefix(strings.ToUpper(stmt), "PREPARE STMT FROM") {
			prepareCount++
		}
	}
	if prepareCount != 1 {
		t.Fatalf("expected exactly one PREPARE statement, got %d", prepareCount)
	}
}

func TestMigration128GuardsAddColumn(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "128_access_operations.sql")
	if err != nil {
		t.Fatalf("read migration 128: %v", err)
	}
	for _, stmt := range SplitSQLStatements(forward) {
		normalized := strings.ToUpper(strings.Join(strings.Fields(stmt), " "))
		// A bare, unguarded ADD COLUMN would abort a partial-failure re-run.
		// The only ADD COLUMN must live inside an IF(...) guard string, which
		// starts with SET @sql, never as a standalone ALTER statement.
		if strings.HasPrefix(normalized, "ALTER TABLE AUTH_ROLE_PERMISSIONS ADD COLUMN TASK_TYPES") {
			t.Fatalf("migration 128 has an unguarded ADD COLUMN task_types statement: %q", stmt)
		}
	}
}

func TestMigration128DoesNotCollapseRolesOrBroadenFineGrainedCapabilities(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "128_access_operations.sql")
	if err != nil {
		t.Fatalf("read migration 128: %v", err)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(forward), " "))
	for _, forbidden := range []string{
		"access_admin' then 'super_admin",
		"asset_submitter' then 'asset",
		"planning_sku.manage",
		"workbench.admin",
		"update auth_user_role_assignments",
		"update auth_org_role_policies",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("migration 128 contains privilege-broadening fragment %q", forbidden)
		}
	}
}

func TestMigration128CopiesThenDeletesPermissionRows(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "128_access_operations.sql")
	if err != nil {
		t.Fatalf("read migration 128: %v", err)
	}
	normalized := strings.ToUpper(strings.Join(strings.Fields(forward), " "))
	insertAt := strings.Index(normalized, "INSERT IGNORE INTO AUTH_ROLE_PERMISSIONS")
	deleteAt := strings.Index(normalized, "DELETE RP FROM AUTH_ROLE_PERMISSIONS")
	if insertAt < 0 || deleteAt < 0 || insertAt >= deleteAt {
		t.Fatalf("migration must insert deduplicated replacement grants before deleting old grants")
	}
	if strings.Contains(normalized, "SET RP.PERMISSION_CODE") {
		t.Fatal("migration must not update permission primary-key values in place")
	}
}
