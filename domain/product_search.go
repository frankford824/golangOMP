package domain

type ProductMappingMatchMode string

const (
	ProductMappingMatchPrimary ProductMappingMatchMode = "primary"
	ProductMappingMatchAll     ProductMappingMatchMode = "all"
)

func (m ProductMappingMatchMode) Valid() bool {
	switch m {
	case "", ProductMappingMatchPrimary, ProductMappingMatchAll:
		return true
	default:
		return false
	}
}

type ProductSearchMatchedMapping struct {
	MappingID               int64                `json:"mapping_id"`
	CategoryCode            string               `json:"category_code"`
	SearchEntryCode         string               `json:"search_entry_code"`
	ERPMatchType            CategoryERPMatchType `json:"erp_match_type"`
	ERPMatchValue           string               `json:"erp_match_value"`
	SecondaryConditionKey   string               `json:"secondary_condition_key"`
	SecondaryConditionValue string               `json:"secondary_condition_value"`
	TertiaryConditionKey    string               `json:"tertiary_condition_key"`
	TertiaryConditionValue  string               `json:"tertiary_condition_value"`
	IsPrimary               bool                 `json:"is_primary"`
	Priority                int                  `json:"priority"`
}

type ProductSearchResult struct {
	Product
	MatchedCategoryCode    string                       `json:"matched_category_code,omitempty"`
	MatchedSearchEntryCode string                       `json:"matched_search_entry_code,omitempty"`
	MatchedMappingRule     *ProductSearchMatchedMapping `json:"matched_mapping_rule,omitempty"`
}
