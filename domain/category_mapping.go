package domain

import "time"

type CategoryERPMatchType string

const (
	CategoryERPMatchTypeCategoryCode  CategoryERPMatchType = "category_code"
	CategoryERPMatchTypeProductFamily CategoryERPMatchType = "product_family"
	CategoryERPMatchTypeSKUPrefix     CategoryERPMatchType = "sku_prefix"
	CategoryERPMatchTypeKeyword       CategoryERPMatchType = "keyword"
	CategoryERPMatchTypeExternalID    CategoryERPMatchType = "external_id"
)

func (t CategoryERPMatchType) Valid() bool {
	switch t {
	case CategoryERPMatchTypeCategoryCode,
		CategoryERPMatchTypeProductFamily,
		CategoryERPMatchTypeSKUPrefix,
		CategoryERPMatchTypeKeyword,
		CategoryERPMatchTypeExternalID:
		return true
	default:
		return false
	}
}

// CategoryERPMapping is the skeleton bridge from category center to ERP product positioning.
// `search_entry_code` is the explicit first-level ERP search entry. Later second/third-level
// search conditions can be overlaid through the reserved secondary/tertiary condition fields.
type CategoryERPMapping struct {
	MappingID               int64                `db:"id"                      json:"mapping_id"`
	CategoryID              *int64               `db:"category_id"             json:"category_id,omitempty"`
	CategoryCode            string               `db:"category_code"           json:"category_code"`
	SearchEntryCode         string               `db:"search_entry_code"       json:"search_entry_code"`
	ERPMatchType            CategoryERPMatchType `db:"erp_match_type"          json:"erp_match_type"`
	ERPMatchValue           string               `db:"erp_match_value"         json:"erp_match_value"`
	SecondaryConditionKey   string               `db:"secondary_condition_key" json:"secondary_condition_key"`
	SecondaryConditionValue string               `db:"secondary_condition_value" json:"secondary_condition_value"`
	TertiaryConditionKey    string               `db:"tertiary_condition_key"  json:"tertiary_condition_key"`
	TertiaryConditionValue  string               `db:"tertiary_condition_value" json:"tertiary_condition_value"`
	IsPrimary               bool                 `db:"is_primary"              json:"is_primary"`
	IsActive                bool                 `db:"is_active"               json:"is_active"`
	Priority                int                  `db:"priority"                json:"priority"`
	Source                  string               `db:"source"                  json:"source"`
	Remark                  string               `db:"remark"                  json:"remark"`
	CreatedAt               time.Time            `db:"created_at"              json:"created_at"`
	UpdatedAt               time.Time            `db:"updated_at"              json:"updated_at"`
}
