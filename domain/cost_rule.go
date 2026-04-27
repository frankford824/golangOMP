package domain

import "time"

type CostRuleType string

const (
	CostRuleTypeFixedUnitPrice         CostRuleType = "fixed_unit_price"
	CostRuleTypeAreaThresholdSurcharge CostRuleType = "area_threshold_surcharge"
	CostRuleTypeMinimumBillableArea    CostRuleType = "minimum_billable_area"
	CostRuleTypeSizeBasedFormula       CostRuleType = "size_based_formula"
	CostRuleTypeManualQuote            CostRuleType = "manual_quote"
	CostRuleTypeSpecialProcessPrice    CostRuleType = "special_process_surcharge"
)

func (t CostRuleType) Valid() bool {
	switch t {
	case CostRuleTypeFixedUnitPrice,
		CostRuleTypeAreaThresholdSurcharge,
		CostRuleTypeMinimumBillableArea,
		CostRuleTypeSizeBasedFormula,
		CostRuleTypeManualQuote,
		CostRuleTypeSpecialProcessPrice:
		return true
	default:
		return false
	}
}

type CostRule struct {
	RuleID                int64                        `db:"id"                      json:"rule_id"`
	RuleName              string                       `db:"rule_name"               json:"rule_name"`
	RuleVersion           int                          `db:"rule_version"            json:"rule_version"`
	CategoryID            *int64                       `db:"category_id"             json:"category_id,omitempty"`
	CategoryCode          string                       `db:"category_code"           json:"category_code"`
	ProductFamily         string                       `db:"product_family"          json:"product_family"`
	RuleType              CostRuleType                 `db:"rule_type"               json:"rule_type"`
	BasePrice             *float64                     `db:"base_price"              json:"base_price,omitempty"`
	TaxMultiplier         *float64                     `db:"tax_multiplier"          json:"tax_multiplier,omitempty"`
	MinArea               *float64                     `db:"min_area"                json:"min_area,omitempty"`
	AreaThreshold         *float64                     `db:"area_threshold"          json:"area_threshold,omitempty"`
	SurchargeAmount       *float64                     `db:"surcharge_amount"        json:"surcharge_amount,omitempty"`
	SpecialProcessKeyword string                       `db:"special_process_keyword" json:"special_process_keyword"`
	SpecialProcessPrice   *float64                     `db:"special_process_price"   json:"special_process_price,omitempty"`
	FormulaExpression     string                       `db:"formula_expression"      json:"formula_expression"`
	Priority              int                          `db:"priority"                json:"priority"`
	IsActive              bool                         `db:"is_active"               json:"is_active"`
	EffectiveFrom         *time.Time                   `db:"effective_from"          json:"effective_from,omitempty"`
	EffectiveTo           *time.Time                   `db:"effective_to"            json:"effective_to,omitempty"`
	SupersedesRuleID      *int64                       `db:"supersedes_rule_id"      json:"supersedes_rule_id,omitempty"`
	SupersededByRuleID    *int64                       `db:"superseded_by_rule_id"   json:"superseded_by_rule_id,omitempty"`
	GovernanceNote        string                       `db:"governance_note"         json:"governance_note"`
	GovernanceStatus      CostRuleGovernanceStatus     `db:"-"           json:"governance_status"`
	VersionChainSummary   *CostRuleVersionChainSummary `db:"-"       json:"version_chain_summary,omitempty"`
	PreviousVersion       *CostRuleVersionRef          `db:"-"         json:"previous_version,omitempty"`
	NextVersion           *CostRuleVersionRef          `db:"-"         json:"next_version,omitempty"`
	SupersessionDepth     int                          `db:"-"         json:"supersession_depth"`
	Source                string                       `db:"source"                  json:"source"`
	Remark                string                       `db:"remark"                  json:"remark"`
	CreatedAt             time.Time                    `db:"created_at"              json:"created_at"`
	UpdatedAt             time.Time                    `db:"updated_at"              json:"updated_at"`
}

type CostRuleGovernanceStatus string

const (
	CostRuleGovernanceStatusInactive  CostRuleGovernanceStatus = "inactive"
	CostRuleGovernanceStatusScheduled CostRuleGovernanceStatus = "scheduled"
	CostRuleGovernanceStatusEffective CostRuleGovernanceStatus = "effective"
	CostRuleGovernanceStatusExpired   CostRuleGovernanceStatus = "expired"
	CostRuleGovernanceStatusNoMatch   CostRuleGovernanceStatus = "no_match"
)

type CostRuleVersionRef struct {
	RuleID           int64                    `json:"rule_id"`
	RuleName         string                   `json:"rule_name"`
	RuleVersion      int                      `json:"rule_version"`
	GovernanceStatus CostRuleGovernanceStatus `json:"governance_status"`
	EffectiveFrom    *time.Time               `json:"effective_from,omitempty"`
	EffectiveTo      *time.Time               `json:"effective_to,omitempty"`
	Source           string                   `json:"source"`
}

type CostRuleVersionChainSummary struct {
	RootRuleID        int64 `json:"root_rule_id"`
	RootRuleVersion   int   `json:"root_rule_version"`
	LatestRuleID      int64 `json:"latest_rule_id"`
	LatestRuleVersion int   `json:"latest_rule_version"`
	TotalVersions     int   `json:"total_versions"`
	SupersessionDepth int   `json:"supersession_depth"`
	IsLatestVersion   bool  `json:"is_latest_version"`
}

type CostRuleHistoryReadModel struct {
	Rule         *CostRule             `json:"rule"`
	VersionChain []*CostRuleVersionRef `json:"version_chain"`
	CurrentRule  *CostRuleVersionRef   `json:"current_rule,omitempty"`
}

func (r *CostRule) GovernanceStatusAt(asOf time.Time) CostRuleGovernanceStatus {
	if r == nil {
		return CostRuleGovernanceStatusNoMatch
	}
	if !r.IsActive {
		return CostRuleGovernanceStatusInactive
	}
	if r.EffectiveFrom != nil && r.EffectiveFrom.After(asOf) {
		return CostRuleGovernanceStatusScheduled
	}
	if r.EffectiveTo != nil && r.EffectiveTo.Before(asOf) {
		return CostRuleGovernanceStatusExpired
	}
	return CostRuleGovernanceStatusEffective
}

type CostRulePreviewRequest struct {
	CategoryID   *int64   `json:"category_id,omitempty"`
	CategoryCode string   `json:"category_code,omitempty"`
	Width        *float64 `json:"width,omitempty"`
	Height       *float64 `json:"height,omitempty"`
	Area         *float64 `json:"area,omitempty"`
	Quantity     *int64   `json:"quantity,omitempty"`
	Process      string   `json:"process,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

type CostRulePreviewMatch struct {
	RuleID           int64                    `json:"rule_id"`
	RuleName         string                   `json:"rule_name"`
	RuleVersion      int                      `json:"rule_version"`
	RuleType         CostRuleType             `json:"rule_type"`
	Priority         int                      `json:"priority"`
	Source           string                   `json:"source"`
	GovernanceStatus CostRuleGovernanceStatus `json:"governance_status"`
}

type CostRulePreviewResponse struct {
	MatchedRule          *CostRulePreviewMatch    `json:"matched_rule,omitempty"`
	MatchedRuleID        *int64                   `json:"matched_rule_id,omitempty"`
	MatchedRuleVersion   *int                     `json:"matched_rule_version,omitempty"`
	AppliedRules         []CostRulePreviewMatch   `json:"applied_rules"`
	EstimatedCost        *float64                 `json:"estimated_cost,omitempty"`
	RuleSource           string                   `json:"rule_source"`
	GovernanceStatus     CostRuleGovernanceStatus `json:"governance_status"`
	RequiresManualReview bool                     `json:"requires_manual_review"`
	Explanation          string                   `json:"explanation"`
}
