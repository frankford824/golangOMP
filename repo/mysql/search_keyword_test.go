package mysqlrepo

import "testing"

func TestNormalizeSearchKeywordClassifiesBusinessIdentifiers(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantCode  bool
		wantTask  bool
		wantSKU   bool
		wantInt64 bool
	}{
		{name: "task no", raw: "RW-20260610-A-001324", wantCode: true, wantTask: true},
		{name: "sku", raw: "CGK000081", wantCode: true, wantSKU: true},
		{name: "asset id", raw: "1697", wantInt64: true},
		{name: "product name", raw: "常规kt板/毕业手举牌", wantCode: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSearchKeyword(tc.raw)
			if got.IsCode != tc.wantCode || got.IsTaskNo != tc.wantTask || got.IsSKUCode != tc.wantSKU || got.HasInt64 != tc.wantInt64 {
				t.Fatalf("normalizeSearchKeyword(%q) = %+v", tc.raw, got)
			}
			if got.Like == "" || got.Prefix == "" {
				t.Fatalf("normalizeSearchKeyword(%q) missing like/prefix: %+v", tc.raw, got)
			}
		})
	}
}
