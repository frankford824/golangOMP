package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestNewPlanningTaskDetailUsesValidRiskFlagsJSON(t *testing.T) {
	detail := newPlanningTaskDetail([]domain.PlanningSKUItemInput{{Quantity: 2}, {Quantity: 3}})

	if !json.Valid([]byte(detail.RiskFlagsJSON)) {
		t.Fatalf("RiskFlagsJSON = %q, want valid JSON", detail.RiskFlagsJSON)
	}
	if detail.Quantity == nil || *detail.Quantity != 5 {
		t.Fatalf("Quantity = %v, want 5", detail.Quantity)
	}
}

type planningSequenceStub struct {
	next map[string]int64
}

func (s *planningSequenceStub) AllocateRange(_ context.Context, _ repo.Tx, prefix, categoryShortCode string, count int) (int64, error) {
	if count <= 0 {
		return 0, fmt.Errorf("count must be positive")
	}
	key := prefix + categoryShortCode
	start := s.next[key]
	s.next[key] = start + int64(count)
	return start, nil
}

func TestPlanningSKUCodeRuleMatchesLatestLegacyPurchaseFormat(t *testing.T) {
	rule := domain.CodeRuleRevision{
		ID:             10,
		Separator:      "",
		SequenceLength: 6,
		ResetCycle:     domain.ResetCycleNone,
		DimensionMode:  domain.CodeRuleDimensionCategoryCode,
		ConfigJSON: `{
			"strategy":"legacy_task_product_code_v1",
			"prefixes":{"regular":"CG","customization":"DZ"},
			"category_short_code_length":1,
			"sequence_length":6
		}`,
	}
	config, appErr := parsePlanningSKUCodeRule(rule)
	if appErr != nil {
		t.Fatalf("parse rule: %v", appErr)
	}
	sequences := &planningSequenceStub{next: map[string]int64{"CGH": 21, "DZC": 7}}
	codes, err := allocatePlanningSKUCodes(context.Background(), nil, sequences, []domain.PlanningSKUItemInput{
		{CategoryCode: "HZS", SKUCodeType: domain.TaskSKUCodeTypeRegular},
		{CategoryCode: "婚庆", SKUCodeType: domain.TaskSKUCodeTypeRegular},
		{CategoryCode: "HQT", SKUCodeType: domain.TaskSKUCodeTypeRegular},
		{CategoryCode: "定制海报", SKUCodeType: domain.TaskSKUCodeTypeCustomization},
	}, config)
	if err != nil {
		t.Fatalf("allocate planning codes: %v", err)
	}
	want := []string{"CGH000021", "CGA000000", "CGH000022", "DZC000007"}
	for index := range want {
		if codes[index] != want[index] {
			t.Fatalf("codes[%d] = %q, want %q", index, codes[index], want[index])
		}
	}
}

func TestValidatePlanningSKUItemRequiresCategoryAndValidCodeType(t *testing.T) {
	base := domain.PlanningSKUItemInput{DescriptionSpec: "产品", Quantity: 1}
	if appErr := validatePlanningSKUItem(base, domain.PlanningSKUERPSyncNone, 0, true); appErr == nil {
		t.Fatal("expected category_code validation error")
	}
	base.CategoryCode = "HZS"
	base.SKUCodeType = domain.TaskSKUCodeType("other")
	if appErr := validatePlanningSKUItem(base, domain.PlanningSKUERPSyncNone, 0, true); appErr == nil {
		t.Fatal("expected sku_code_type validation error")
	}
	base.SKUCodeType = domain.TaskSKUCodeTypeCustomization
	if appErr := validatePlanningSKUItem(base, domain.PlanningSKUERPSyncNone, 0, true); appErr != nil {
		t.Fatalf("valid planning row rejected: %v", appErr)
	}
}

func TestPlanningSKUDerivesERPProductNameFromDescription(t *testing.T) {
	item := domain.PlanningSKUItemInput{
		CategoryCode:    "HZS",
		DescriptionSpec: "亚克力立牌 20cm",
		Quantity:        1,
		ERPProductIID:   "HQT",
	}
	item.ERPProductName = planningERPProductName(item)

	if appErr := validatePlanningSKUItem(item, domain.PlanningSKUERPSyncAsync, 0, true); appErr != nil {
		t.Fatalf("derived ERP product name was rejected: %v", appErr)
	}
	if item.ERPProductName != item.DescriptionSpec {
		t.Fatalf("ERPProductName = %q, want description fallback %q", item.ERPProductName, item.DescriptionSpec)
	}
	headers := planningExcelHeaders(true)
	if got := headers[0]; got != "款式编码" {
		t.Fatalf("first template header = %q, want the single visible style code", got)
	}
	for _, header := range headers {
		if header == "ERP 产品名称" {
			t.Fatal("new planning template still asks users to repeat the ERP product name")
		}
	}
}

func TestPlanningSKUERPProductNameFallbackRespectsERPMaxLength(t *testing.T) {
	item := domain.PlanningSKUItemInput{DescriptionSpec: strings.Repeat("长", ERPProductNameMaxLength+8)}
	if got := erpProductNameLength(planningERPProductName(item)); got != ERPProductNameMaxLength {
		t.Fatalf("fallback ERP product name length = %d, want %d", got, ERPProductNameMaxLength)
	}
}
