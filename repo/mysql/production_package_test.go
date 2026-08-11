package mysqlrepo

import (
	"strings"
	"testing"
)

func TestProductionPackageResolvedSKUNormalizesMixedCollations(t *testing.T) {
	for _, source := range []string{
		"tsi.sku_code",
		"rr.sku_code",
		"ta.scope_sku_code",
		"t.primary_sku_code",
		"t.sku_code",
	} {
		want := "CONVERT(" + source + " USING utf8mb4) COLLATE utf8mb4_unicode_ci"
		if !strings.Contains(productionPackageResolvedSKU, want) {
			t.Fatalf("resolved SKU expression must normalize %s: %s", source, productionPackageResolvedSKU)
		}
	}
}
