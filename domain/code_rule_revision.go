package domain

import "time"

type CodeRuleDimensionMode string

const (
	CodeRuleDimensionNone         CodeRuleDimensionMode = "none"
	CodeRuleDimensionCategoryCode CodeRuleDimensionMode = "category_code"
)

type CodeRuleRevision struct {
	ID             int64                 `json:"id"`
	RuleID         int64                 `json:"rule_id"`
	VersionNo      int                   `json:"version_no"`
	Prefix         string                `json:"prefix"`
	DateFormat     string                `json:"date_format,omitempty"`
	SiteCode       string                `json:"site_code,omitempty"`
	BizCode        string                `json:"biz_code,omitempty"`
	Separator      string                `json:"separator,omitempty"`
	SequenceLength int                   `json:"seq_length"`
	ResetCycle     ResetCycle            `json:"reset_cycle"`
	DimensionMode  CodeRuleDimensionMode `json:"dimension_mode"`
	CreatedAt      time.Time             `json:"created_at"`
}
