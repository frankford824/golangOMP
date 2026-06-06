package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"workflow/domain"
)

func TestValidateERPProductUpsertNameLength(t *testing.T) {
	okName := strings.Repeat("测", ERPProductNameMaxRunes)
	if appErr := validateERPProductUpsertNameLength(domain.ERPProductUpsertPayload{Name: okName}); appErr != nil {
		t.Fatalf("exact max length rejected: %+v", appErr)
	}

	tooLong := okName + "试"
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
	appErr = validateERPProductUpsertNameLength(domain.ERPProductUpsertPayload{Name: "正常产品", ShortName: longShortName})
	if appErr != nil {
		t.Fatalf("long ERP product short name should no longer be rejected: %+v", appErr)
	}
}

func TestTruncateERPShortNameIsRuneSafe(t *testing.T) {
	got := truncateERPShortName(strings.Repeat("中", 20), ERPProductShortNameMaxBytes)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateERPShortName() returned invalid UTF-8: %q", got)
	}
	if len(got) > ERPProductShortNameMaxBytes {
		t.Fatalf("byte length = %d, want <= %d", len(got), ERPProductShortNameMaxBytes)
	}
	if utf8.RuneCountInString(got) != 13 {
		t.Fatalf("rune length = %d, want 13", utf8.RuneCountInString(got))
	}
}

func TestValidateCreateTaskERPProductNameLengthBatch(t *testing.T) {
	tooLong := strings.Repeat("批", ERPProductNameMaxRunes+1)
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
