package v1migrate

import (
	"strings"
	"testing"
)

func TestMigration136GrantsWarehouseOnlyGlobalAssetReadAndDownload(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "136_warehouse_global_asset_access.sql")
	if err != nil {
		t.Fatalf("read migration 136: %v", err)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(forward), " "))
	for _, required := range []string{
		"warehouse_asset_reader",
		"permission.code in ('asset.view', 'asset.download')",
		"legacy_role.role = 'warehouse'",
		"department.name = '云仓部'",
		"'global'",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("migration 136 is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"task.manage",
		"task.create",
		"asset.manage",
		"access_policy.manage",
		"planning_sku.erp_sync",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("migration 136 broadens warehouse capability with %q", forbidden)
		}
	}
}

func TestMigration136ForwardSQLSplitsCleanly(t *testing.T) {
	forward, err := ReadForwardSQL(migrationsDir(), "136_warehouse_global_asset_access.sql")
	if err != nil {
		t.Fatalf("read migration 136: %v", err)
	}
	if statements := SplitSQLStatements(forward); len(statements) < 6 {
		t.Fatalf("migration 136 split into %d statements, want at least 6", len(statements))
	}
}
