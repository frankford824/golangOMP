//go:build integration

package task_batch_excel

import (
	"bytes"
	"testing"

	"workflow/domain"
)

func TestSAEI_Service_ParseUpload_HappyPath_NPD(t *testing.T) {
	content, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypeNewProductDevelopment)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Preview) != 2 || len(result.Violations) != 0 {
		t.Fatalf("Parse result = %+v, want 2 preview rows and no violations", result)
	}
}

func TestSAEI_Service_PurchaseTemplateRetired(t *testing.T) {
	_, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypePurchaseTask)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Generate appErr = %#v, want invalid request", appErr)
	}
}
