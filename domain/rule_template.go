package domain

import "time"

// RuleTemplateType identifies the rule template namespace.
type RuleTemplateType string

const (
	RuleTemplateTypeCostPricing RuleTemplateType = "cost-pricing"
	RuleTemplateTypeProductCode RuleTemplateType = "product-code"
	RuleTemplateTypeShortName   RuleTemplateType = "short-name"
)

// RuleTemplate stores config for a rule template type.
type RuleTemplate struct {
	ID           int64            `db:"id"           json:"id"`
	TemplateType RuleTemplateType `db:"template_type" json:"template_type"`
	ConfigJSON   string           `db:"config_json"  json:"config_json"`
	CreatedAt    time.Time        `db:"created_at"   json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"   json:"updated_at"`
}

// RuleTemplateConfig is the generic config envelope.
type RuleTemplateConfig struct {
	Enabled bool                   `json:"enabled"`
	Params  map[string]interface{} `json:"params,omitempty"`
}
