package service

import (
	"testing"

	"workflow/domain"
)

func TestResourceGroupListShouldFlatten(t *testing.T) {
	cases := []struct {
		name   string
		params domain.ResourceGroupListParams
		want   bool
	}{
		{"default is group mode", domain.ResourceGroupListParams{}, false},
		{"explicit all format stays group", domain.ResourceGroupListParams{FormatCategory: domain.AssetFormatCategoryAll}, false},
		{"resource role forces flat", domain.ResourceGroupListParams{ResourceRole: domain.ResourceRoleFilterReference}, true},
		{"image format forces flat", domain.ResourceGroupListParams{FormatCategory: domain.AssetFormatCategoryImage}, true},
		{"document alias forces flat", domain.ResourceGroupListParams{FormatCategory: "document"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resourceGroupListShouldFlatten(tc.params); got != tc.want {
				t.Fatalf("resourceGroupListShouldFlatten(%+v) = %v, want %v", tc.params, got, tc.want)
			}
		})
	}
}

func TestNormalizeResourceListFormatCategoryDocumentAlias(t *testing.T) {
	cases := map[domain.AssetFormatCategoryFilter]domain.AssetFormatCategoryFilter{
		"document":                      domain.AssetFormatCategoryPDF,
		domain.AssetFormatCategoryPDF:   domain.AssetFormatCategoryPDF,
		"":                              domain.AssetFormatCategoryAll,
		domain.AssetFormatCategoryAll:   domain.AssetFormatCategoryAll,
		domain.AssetFormatCategoryImage: domain.AssetFormatCategoryImage,
		"totally-unknown":               domain.AssetFormatCategoryAll,
	}
	for input, want := range cases {
		if got := normalizeResourceListFormatCategory(input); got != want {
			t.Fatalf("normalizeResourceListFormatCategory(%q) = %q, want %q", input, got, want)
		}
	}
}
