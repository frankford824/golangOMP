package service

import (
	"strings"
	"testing"

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
