package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"workflow/domain"
)

type costPreviewComputation struct {
	Response    *domain.CostRulePreviewResponse
	MatchedRule *domain.CostRule
}

func previewCostRules(req domain.CostRulePreviewRequest, rules []*domain.CostRule) costPreviewComputation {
	req = withTextDerivedCostRuleDimensions(req)
	sortedRules := make([]*domain.CostRule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		sortedRules = append(sortedRules, rule)
	}
	sort.SliceStable(sortedRules, func(i, j int) bool {
		if sortedRules[i].Priority == sortedRules[j].Priority {
			if sortedRules[i].RuleVersion == sortedRules[j].RuleVersion {
				return sortedRules[i].RuleID < sortedRules[j].RuleID
			}
			return sortedRules[i].RuleVersion > sortedRules[j].RuleVersion
		}
		return sortedRules[i].Priority < sortedRules[j].Priority
	})

	area := previewArea(req)
	areaThresholdBasis := previewAreaThresholdBasis(req, area)
	quantity := previewQuantity(req.Quantity)
	ruleSource := ""
	manualReview := false
	var estimated float64
	fixedUnitTaxMultiplier := 1.0
	hasFixedUnitPrice := false
	var matchedRule *domain.CostRule
	applied := make([]domain.CostRulePreviewMatch, 0, len(sortedRules))
	explanations := make([]string, 0, len(sortedRules))

	for _, rule := range sortedRules {
		match := domain.CostRulePreviewMatch{
			RuleID:           rule.RuleID,
			RuleName:         rule.RuleName,
			RuleVersion:      rule.RuleVersion,
			RuleType:         rule.RuleType,
			Priority:         rule.Priority,
			Source:           rule.Source,
			GovernanceStatus: rule.GovernanceStatusAt(time.Now()),
		}
		if matchedRule == nil {
			matchedRule = rule
			ruleSource = rule.Source
		}

		switch rule.RuleType {
		case domain.CostRuleTypeMinimumBillableArea:
			switch {
			case rule.MinArea == nil:
				continue
			case area <= 0:
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s requires width/height/area input before minimum billable area can be applied", rule.RuleName))
			case area < *rule.MinArea:
				area = *rule.MinArea
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s adjusted billable area to %.4f", rule.RuleName, area))
			}
		case domain.CostRuleTypeFixedUnitPrice:
			baseCharge, ok := applyFixedUnitPrice(rule, area, quantity)
			if !ok {
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s requires width/height/area input before fixed-price estimation can run", rule.RuleName))
				continue
			}
			estimated += *baseCharge
			hasFixedUnitPrice = true
			fixedUnitTaxMultiplier = taxMultiplierOrOne(rule.TaxMultiplier)
			applied = append(applied, match)
			explanations = append(explanations, fmt.Sprintf("%s applied fixed price %.3f", rule.RuleName, *baseCharge))
		case domain.CostRuleTypeAreaThresholdSurcharge:
			switch {
			case rule.AreaThreshold == nil || rule.SurchargeAmount == nil:
				continue
			case area <= 0 || areaThresholdBasis <= 0:
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s requires width/height/area input before threshold surcharge can be evaluated", rule.RuleName))
			case areaThresholdBasis < *rule.AreaThreshold:
				extra := (*rule.SurchargeAmount) * area * float64(quantity)
				if rule.TaxMultiplier != nil && *rule.TaxMultiplier > 0 {
					extra = extra * (*rule.TaxMultiplier)
				} else if hasFixedUnitPrice {
					extra = extra * fixedUnitTaxMultiplier
				}
				estimated += extra
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s increased unit price by %.3f and applied %.3f because threshold area %.4f < %.4f", rule.RuleName, *rule.SurchargeAmount, extra, areaThresholdBasis, *rule.AreaThreshold))
			}
		case domain.CostRuleTypeSpecialProcessPrice:
			if rule.SpecialProcessPrice != nil && containsProcessKeyword(req.Process, strings.Join(nonEmptyStrings(req.CategoryCode, req.Notes), " "), rule.SpecialProcessKeyword) {
				extra := (*rule.SpecialProcessPrice) * float64(quantity)
				estimated += extra
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s added process surcharge %.3f", rule.RuleName, extra))
			}
		case domain.CostRuleTypeSizeBasedFormula:
			calculated, explanation, ok := applySizeBasedFormula(rule, quantity, req.Process, req.Notes)
			if ok {
				estimated += calculated
				applied = append(applied, match)
				explanations = append(explanations, explanation)
			} else {
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s requires manual review because the size-based formula skeleton is not fully executable yet", rule.RuleName))
			}
		case domain.CostRuleTypeManualQuote:
			manualReview = true
			applied = append(applied, match)
			explanations = append(explanations, fmt.Sprintf("%s is marked as manual_quote", rule.RuleName))
		}
	}

	var estimatedPtr *float64
	if len(applied) > 0 && (!manualReview || estimated > 0) {
		estimatedCopy := roundCostAmount(estimated)
		estimatedPtr = &estimatedCopy
	}
	if len(applied) == 0 {
		manualReview = true
		explanations = append(explanations, "No active cost rule matched the requested category and input snapshot.")
	}

	return costPreviewComputation{
		Response: &domain.CostRulePreviewResponse{
			MatchedRule:          previewMatchFromRule(matchedRule),
			MatchedRuleID:        previewMatchRuleID(matchedRule),
			MatchedRuleVersion:   previewMatchRuleVersion(matchedRule),
			AppliedRules:         applied,
			EstimatedCost:        estimatedPtr,
			RuleSource:           ruleSource,
			GovernanceStatus:     previewGovernanceStatus(matchedRule),
			RequiresManualReview: manualReview,
			Explanation:          strings.Join(explanations, " "),
		},
		MatchedRule: matchedRule,
	}
}

func roundCostAmount(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func taxMultiplierOrOne(value *float64) float64 {
	if value == nil || *value <= 0 {
		return 1
	}
	return *value
}

func withTextDerivedCostRuleDimensions(req domain.CostRulePreviewRequest) domain.CostRulePreviewRequest {
	extracted := extractCostDimensionsFromText(req.Notes)
	if extracted.WidthM != nil {
		req.Width = cloneFloat64Ptr(extracted.WidthM)
	}
	if extracted.HeightM != nil {
		req.Height = cloneFloat64Ptr(extracted.HeightM)
	}
	if extracted.AreaM2 != nil {
		req.Area = cloneFloat64Ptr(extracted.AreaM2)
	}
	return req
}

func previewMatchFromRule(rule *domain.CostRule) *domain.CostRulePreviewMatch {
	if rule == nil {
		return nil
	}
	return &domain.CostRulePreviewMatch{
		RuleID:           rule.RuleID,
		RuleName:         rule.RuleName,
		RuleVersion:      rule.RuleVersion,
		RuleType:         rule.RuleType,
		Priority:         rule.Priority,
		Source:           rule.Source,
		GovernanceStatus: rule.GovernanceStatusAt(time.Now()),
	}
}

func previewMatchRuleID(rule *domain.CostRule) *int64 {
	if rule == nil {
		return nil
	}
	id := rule.RuleID
	return &id
}

func previewMatchRuleVersion(rule *domain.CostRule) *int {
	if rule == nil {
		return nil
	}
	version := rule.RuleVersion
	return &version
}

func previewGovernanceStatus(rule *domain.CostRule) domain.CostRuleGovernanceStatus {
	if rule == nil {
		return domain.CostRuleGovernanceStatusNoMatch
	}
	return rule.GovernanceStatusAt(time.Now())
}

func previewAreaThresholdBasis(req domain.CostRulePreviewRequest, billableArea float64) float64 {
	if costPreviewUsesBillableAreaForThreshold(req) {
		return billableArea
	}
	if req.Width != nil && req.Height != nil && *req.Width > 0 && *req.Height > 0 {
		return (*req.Width) * (*req.Height)
	}
	return billableArea
}

func costPreviewUsesBillableAreaForThreshold(req domain.CostRulePreviewRequest) bool {
	return containsCostBoxLayoutKeyword(strings.Join(nonEmptyStrings(req.Process, req.Notes), " "))
}
