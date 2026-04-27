//go:build integration

package task_batch_excel

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

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

func TestSAEI_Service_DownloadTemplate_PT(t *testing.T) {
	content, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypePurchaseTask)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open generated template: %v", err)
	}
	defer f.Close()
	if rows, err := f.GetRows(itemsSheet); err != nil || len(rows) < 3 {
		t.Fatalf("Items rows = %#v err=%v, want header plus sample rows", rows, err)
	}
}
