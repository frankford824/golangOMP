package orders

import (
	"strings"
	"testing"
)

func TestRankingQuery(t *testing.T) {
	for _, tc := range []struct{ group, sort, order, pageOrder, groupBy, outerOrder, join string }{
		{"", "", "", "payable DESC NULLS LAST,shop_id", "GROUP BY source.shop_id", "ranked.payable DESC NULLS LAST,ranked.shop_id", ""},
		{"platform", "count", "asc", "orders ASC NULLS LAST,platform", "GROUP BY source.platform", "ranked.orders ASC NULLS LAST,ranked.platform", ""},
		{"growth", "", "", "growth_rate DESC NULLS LAST,shop_id", "GROUP BY source.shop_id", "ranked.growth_rate DESC NULLS LAST,ranked.shop_id", ""},
		{"product", "amount", "asc", "sales ASC NULLS LAST,shop_id,shop_item_id", "GROUP BY shop_id,shop_item_id", "ranked.sales ASC NULLS LAST,ranked.shop_id,ranked.shop_item_id", "LEFT ANY JOIN jst.v_listings AS identity ON ranked.shop_id=identity.shop_id AND ranked.shop_item_id=identity.shop_item_id"},
		{"person", "amount", "desc", "sales DESC NULLS LAST,person_id", "GROUP BY person_id", "ranked.sales DESC NULLS LAST,ranked.person_id", "LEFT ANY JOIN jst.v_reg_person AS identity ON ranked.person_id=identity.person_id"},
		{"sku", "quantity", "asc", "units ASC NULLS LAST,shop_id,sku_id", "GROUP BY shop_id,sku_id", "ranked.units ASC NULLS LAST,ranked.shop_id,ranked.sku_id", ""},
	} {
		t.Run(tc.group, func(t *testing.T) {
			query, err := rankingQuery(tc.group, tc.sort, tc.order, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []string{"toUInt32(count() OVER ()) AS total_rows", tc.groupBy, "ORDER BY " + tc.pageOrder + " LIMIT 101 OFFSET {offset:UInt32}) AS ranked", "ORDER BY " + tc.outerOrder} {
				if !strings.Contains(query, fragment) {
					t.Errorf("missing %q", fragment)
				}
			}
			if tc.join != "" && !strings.Contains(query, ") AS ranked "+tc.join+" ORDER BY ") {
				t.Errorf("missing post-page join %q", tc.join)
			}
		})
	}
}

func TestRankingQueryRejectsUnsupportedSort(t *testing.T) {
	for _, tc := range []struct {
		group, sort, order string
		brush              bool
	}{
		{"person", "quantity", "", false}, {"sku", "gross", "", false}, {"shop", "growth", "", false},
		{"shop", "amount", "DESC", false}, {"platform", "gross;SELECT 1", "desc", false}, {"growth", "", "", true},
		{"product", "", "desc;SELECT 1", false}, {"other", "", "", false},
	} {
		if query, err := rankingQuery(tc.group, tc.sort, tc.order, tc.brush); err == nil || query != "" {
			t.Errorf("unexpected accepted parameters %+v", tc)
		}
	}
}

func TestRankingGrowthUsesLegacyRate(t *testing.T) {
	query, err := rankingQuery("growth", "growth", "desc", false)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the legacy Float64 calculation for response and ordering parity.
	// Verified gross rate still requires its denominator to be positive.
	for _, expr := range []string{
		"toFloat64(payable-baseline)/nullIf(toFloat64(baseline),0) AS growth_rate",
		"if(verified_sales>0,divideDecimal(verified_sales-verified_cost,if(verified_sales>0,verified_sales,1),24),NULL) AS gross_rate",
	} {
		if !strings.Contains(query, expr) {
			t.Errorf("missing %q", expr)
		}
	}
}
