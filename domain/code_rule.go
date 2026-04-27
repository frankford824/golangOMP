package domain

import "time"

// CodeRule stores a versioned numbering rule configuration (spec V7 §5.3).
type CodeRule struct {
	ID         int64        `db:"id"          json:"id"`
	RuleType   CodeRuleType `db:"rule_type"   json:"rule_type"`
	RuleName   string       `db:"rule_name"   json:"rule_name"`
	Prefix     string       `db:"prefix"      json:"prefix"`
	DateFormat string       `db:"date_format" json:"date_format"`
	SiteCode   string       `db:"site_code"   json:"site_code"`
	BizCode    string       `db:"biz_code"    json:"biz_code"`
	SeqLength  int          `db:"seq_length"  json:"seq_length"`
	ResetCycle ResetCycle   `db:"reset_cycle" json:"reset_cycle"`
	IsEnabled  bool         `db:"is_enabled"  json:"is_enabled"`
	ConfigJSON string       `db:"config_json" json:"config_json"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"  json:"updated_at"`
}

// CodePreview is the response from the preview endpoint.
// IsPreview is always true; the sequence is not incremented.
type CodePreview struct {
	RuleID    int64  `json:"rule_id"`
	Preview   string `json:"preview"`
	IsPreview bool   `json:"is_preview"`
}
