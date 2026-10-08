package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"workflow/domain"
)

const LegacyCostRestoreSource = "production_legacy_restore_20261007"

type LegacyCostBindingEvidence struct {
	IID    string   `json:"i_id"`
	Groups []string `json:"historical_groups"`
}

type LegacyCostBindingDecision struct {
	IID           string `json:"i_id"`
	NormalizedIID string `json:"normalized_i_id"`
	Group         string `json:"rule_group,omitempty"`
	Reason        string `json:"reason"`
}

// PlanLegacyCostBindings resolves only style identities, never arbitrary product
// titles. Existing explicit bindings win. Former text-based cross-material
// matches cannot override the existing deterministic style-name classification.
func PlanLegacyCostBindings(rules []*domain.CostRule, existing []*domain.CostRuleBinding, evidence []LegacyCostBindingEvidence) []LegacyCostBindingDecision {
	groups := map[string]bool{}
	bound := map[string]string{}
	for _, r := range rules {
		if r.IsActive {
			groups[r.CategoryCode] = true
		}
	}
	for _, b := range existing {
		if b.IsActive {
			bound[b.NormalizedIID] = b.RuleGroup
		}
	}
	byIID := map[string]LegacyCostBindingEvidence{}
	for _, e := range evidence {
		n := domain.NormalizeIID(e.IID)
		if n == "" {
			continue
		}
		old := byIID[n]
		old.IID = strings.TrimSpace(e.IID)
		old.Groups = append(old.Groups, e.Groups...)
		byIID[n] = old
	}
	keys := make([]string, 0, len(byIID))
	for n := range byIID {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	out := make([]LegacyCostBindingDecision, 0, len(keys))
	for _, n := range keys {
		e := byIID[n]
		d := LegacyCostBindingDecision{IID: e.IID, NormalizedIID: n}
		if g := bound[n]; g != "" {
			d.Group = g
			d.Reason = "existing_binding"
			out = append(out, d)
			continue
		}
		aliases := costCategoryAliasesFromText("", e.IID)
		if len(aliases) == 1 && groups[aliases[0]] {
			d.Group = aliases[0]
			d.Reason = "existing_style_classifier"
		} else if len(aliases) > 0 {
			d.Reason = "missing_rule_group"
		} else {
			unique := map[string]bool{}
			for _, g := range e.Groups {
				if groups[g] {
					unique[g] = true
				}
			}
			// Only named material families can use this fallback. An accidental
			// match on an opaque purchase/style code must not become a tariff.
			known := map[string]string{"常规模切": "DIECUT_STICKER", "定制模切": "DIECUT_STICKER", "常规车缝": "FLAG_CLOTH_SEWED", "A3纸打印": "A3_PRINT", "A4纸打印": "A4_PRINT"}
			if expected := known[n]; expected != "" && unique[expected] {
				d.Group = expected
				d.Reason = "named_historical_family"
			} else {
				d.Reason = "unresolved_historical_binding"
			}
		}
		out = append(out, d)
	}
	return out
}

// A successor preserves every price, threshold, formula and process term while
// recording restoration provenance. Historical snapshots keep their old IDs.
func RestoredLegacyCostRule(r *domain.CostRule) *domain.CostRule {
	if r == nil || !isUncertifiedSampleCostRule(r) {
		return nil
	}
	c := *r
	c.RuleID = 0
	c.RuleVersion = r.RuleVersion + 1
	c.SupersedesRuleID = &r.RuleID
	c.Source = LegacyCostRestoreSource
	c.GovernanceNote = fmt.Sprintf("恢复生产既有计价项，原规则%d版本%d；价格、公式、工艺不变", r.RuleID, r.RuleVersion)
	return &c
}

func LegacyCostPlanDigest(value interface{}) string {
	b, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Used by the operator recovery command to preview through the same SKU
// dimension and rule-binding path as ordinary task creation.
func PreviewRestoredLegacyCost(req domain.CostRulePreviewRequest, rules []*domain.CostRule) *domain.CostRulePreviewResponse {
	restored := make([]*domain.CostRule, 0, len(rules))
	for _, r := range rules {
		if c := RestoredLegacyCostRule(r); c != nil {
			restored = append(restored, c)
		} else {
			restored = append(restored, r)
		}
	}
	return previewCostRulesResolvedDimensions(req, appendVirtualCostRules(restored, req.CategoryCode, req.Notes)).Response
}
