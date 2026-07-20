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
	MatchTrace  *domain.CostRuleMatchTrace
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
				explanations = append(explanations, fmt.Sprintf("%s：需要先填写宽、高或面积，才能应用最低计价面积。", rule.RuleName))
			case area < *rule.MinArea:
				area = *rule.MinArea
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s：本次按最低计价面积 %.4f ㎡计算。", rule.RuleName, area))
			}
		case domain.CostRuleTypeFixedUnitPrice:
			baseCharge, ok := applyFixedUnitPrice(rule, area, quantity)
			if !ok {
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s：需要先填写宽、高或面积，才能计算固定单价。", rule.RuleName))
				continue
			}
			estimated += *baseCharge
			hasFixedUnitPrice = true
			fixedUnitTaxMultiplier = taxMultiplierOrOne(rule.TaxMultiplier)
			applied = append(applied, match)
			explanations = append(explanations, fmt.Sprintf("%s：固定单价部分为 ¥%.3f。", rule.RuleName, *baseCharge))
		case domain.CostRuleTypeAreaThresholdSurcharge:
			switch {
			case rule.AreaThreshold == nil || rule.SurchargeAmount == nil:
				continue
			case area <= 0 || areaThresholdBasis <= 0:
				manualReview = true
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s：需要先填写宽、高或面积，才能判断是否产生小面积附加费。", rule.RuleName))
			case areaThresholdBasis < *rule.AreaThreshold:
				extra := (*rule.SurchargeAmount) * area * float64(quantity)
				if rule.TaxMultiplier != nil && *rule.TaxMultiplier > 0 {
					extra = extra * (*rule.TaxMultiplier)
				} else if hasFixedUnitPrice {
					extra = extra * fixedUnitTaxMultiplier
				}
				estimated += extra
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s：单价增加 ¥%.3f，本次附加 ¥%.3f（实际面积 %.4f ㎡，低于 %.4f ㎡阈值）。", rule.RuleName, *rule.SurchargeAmount, extra, areaThresholdBasis, *rule.AreaThreshold))
			}
		case domain.CostRuleTypeSpecialProcessPrice:
			if rule.SpecialProcessPrice != nil && containsProcessKeyword(req.Process, strings.Join(nonEmptyStrings(req.CategoryCode, req.Notes), " "), rule.SpecialProcessKeyword) {
				extra := (*rule.SpecialProcessPrice) * float64(quantity)
				estimated += extra
				applied = append(applied, match)
				explanations = append(explanations, fmt.Sprintf("%s：特殊工艺附加 ¥%.3f。", rule.RuleName, extra))
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
				explanations = append(explanations, fmt.Sprintf("%s：当前尺寸公式还不能自动完成计算，需要人工确认。", rule.RuleName))
			}
		case domain.CostRuleTypeManualQuote:
			manualReview = true
			applied = append(applied, match)
			explanations = append(explanations, fmt.Sprintf("%s：此规则要求人工报价。", rule.RuleName))
		}
	}

	var estimatedPtr *float64
	if len(applied) > 0 && (!manualReview || estimated > 0) {
		estimatedCopy := roundCostAmount(estimated)
		estimatedPtr = &estimatedCopy
	}
	if len(applied) == 0 {
		manualReview = true
		explanations = append(explanations, "没有找到可用于当前款式和输入条件的成本规则，需要人工确认。")
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
			ERPIID:               strings.TrimSpace(req.ERPIID),
			ProductIID:           strings.TrimSpace(req.ProductIID),
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
