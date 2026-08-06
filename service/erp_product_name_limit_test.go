package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"workflow/domain"
)

func TestValidateERPProductUpsertNameLength(t *testing.T) {
	okName := strings.Repeat("名", ERPProductNameMaxLength)
	if appErr := validateERPProductUpsertNameLength(domain.ERPProductUpsertPayload{Name: okName}); appErr != nil {
		t.Fatalf("exact max length rejected: %+v", appErr)
	}

	tooLong := okName + "字"
	appErr := validateERPProductUpsertNameLength(domain.ERPProductUpsertPayload{Name: tooLong})
	if appErr == nil {
		t.Fatal("expected long ERP product name to be rejected")
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details = %#v, want map", appErr.Details)
	}
	if got := details["code"]; got != "erp_product_name_too_long" {
		t.Fatalf("code = %#v, want erp_product_name_too_long", got)
	}

	longShortName := strings.Repeat("短", 14)
	normalized := normalizeERPProductUpsertPayload(domain.ERPProductUpsertPayload{Name: "正常产品", ShortName: longShortName})
	if normalized.ShortName != "正常产品" {
		t.Fatalf("ShortName = %q, want product name", normalized.ShortName)
	}
	appErr = validateERPProductUpsertNameLength(normalized)
	if appErr != nil {
		t.Fatalf("normalized ERP product short name should pass validation: %+v", appErr)
	}
}

func TestNormalizeERPProductUpsertPayloadForcesProductNameAsShortName(t *testing.T) {
	normalized := normalizeERPProductUpsertPayload(domain.ERPProductUpsertPayload{
		SKUID:            "SKU-NAME-001",
		Name:             "统一名称",
		ProductName:      "统一名称",
		ShortName:        "旧简称",
		ProductShortName: "另一个旧简称",
	})

	if normalized.ShortName != "统一名称" {
		t.Fatalf("ShortName = %q, want product name", normalized.ShortName)
	}
	if normalized.ProductShortName != "统一名称" {
		t.Fatalf("ProductShortName = %q, want product name", normalized.ProductShortName)
	}
}

func TestNormalizeERPProductUpsertPayloadTruncatesHistoricalNameForERPOnly(t *testing.T) {
	historicalName := strings.Repeat("旧", ERPProductNameMaxLength+7)
	normalized := normalizeERPProductUpsertPayload(domain.ERPProductUpsertPayload{
		SKUID: "SKU-HISTORICAL-001",
		Name:  historicalName,
	})
	if got := utf8.RuneCountInString(normalized.Name); got != ERPProductNameMaxLength {
		t.Fatalf("normalized name length = %d, want %d", got, ERPProductNameMaxLength)
	}
	if normalized.Name != normalized.ShortName || normalized.Name != normalized.ProductName ||
		normalized.Name != normalized.ProductShortName {
		t.Fatalf("normalized ERP names diverged: %+v", normalized)
	}
}

func TestTruncateERPShortNameIsRuneSafe(t *testing.T) {
	got := truncateERPShortName(strings.Repeat("中", 50), ERPProductShortNameMaxLength)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateERPShortName() returned invalid UTF-8: %q", got)
	}
	if utf8.RuneCountInString(got) > ERPProductShortNameMaxLength {
		t.Fatalf("rune length = %d, want <= %d", utf8.RuneCountInString(got), ERPProductShortNameMaxLength)
	}
	if utf8.RuneCountInString(got) != ERPProductShortNameMaxLength {
		t.Fatalf("rune length = %d, want %d", utf8.RuneCountInString(got), ERPProductShortNameMaxLength)
	}
}

func TestValidateCreateTaskERPProductNameLengthBatch(t *testing.T) {
	tooLong := strings.Repeat("批", ERPProductNameMaxLength+1)
	appErr := validateCreateTaskERPProductNameLength(CreateTaskParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		SourceMode:   domain.TaskSourceModeNewProduct,
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{ProductName: "正常产品"},
			{ProductName: tooLong},
		},
	})
	if appErr == nil {
		t.Fatal("expected long batch item product name to be rejected")
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok || details["violations"] == nil {
		t.Fatalf("missing violations in app error: %+v", appErr.Details)
	}
}
