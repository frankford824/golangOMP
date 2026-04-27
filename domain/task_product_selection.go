package domain

// TaskProductSelectionSummary is the lightweight read-model view of original-product provenance.
// It is intended for task list, board, and procurement-summary consumption.
type TaskProductSelectionSummary struct {
	SelectedProductID      *int64                       `json:"selected_product_id,omitempty"`
	SelectedProductName    string                       `json:"selected_product_name"`
	SelectedProductSKUCode string                       `json:"selected_product_sku_code"`
	MatchedCategoryCode    string                       `json:"matched_category_code"`
	MatchedSearchEntryCode string                       `json:"matched_search_entry_code"`
	SourceProductID        *int64                       `json:"source_product_id,omitempty"`
	SourceProductName      string                       `json:"source_product_name"`
	SourceMatchType        string                       `json:"source_match_type"`
	SourceMatchRule        string                       `json:"source_match_rule"`
	SourceSearchEntryCode  string                       `json:"source_search_entry_code"`
	ERPProduct             *ERPProductSelectionSnapshot `json:"erp_product,omitempty"`
}

// TaskProductSelectionContext captures how an existing ERP product was selected for a task.
// It keeps the selected product binding and the mapped-search provenance together.
type TaskProductSelectionContext struct {
	SelectedProductID        *int64                       `json:"selected_product_id,omitempty"`
	SelectedProductName      string                       `json:"selected_product_name"`
	SelectedProductSKUCode   string                       `json:"selected_product_sku_code"`
	MatchedCategoryCode      string                       `json:"matched_category_code"`
	MatchedSearchEntryCode   string                       `json:"matched_search_entry_code"`
	MatchedMappingRule       *ProductSearchMatchedMapping `json:"matched_mapping_rule,omitempty"`
	SourceProductID          *int64                       `json:"source_product_id,omitempty"`
	SourceProductName        string                       `json:"source_product_name"`
	SourceMatchType          string                       `json:"source_match_type"`
	SourceMatchRule          string                       `json:"source_match_rule"`
	SourceSearchEntryCode    string                       `json:"source_search_entry_code"`
	ERPProduct               *ERPProductSelectionSnapshot `json:"erp_product,omitempty"`
	DeferLocalProductBinding bool                         `json:"defer_local_product_binding,omitempty"`
}
