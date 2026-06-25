package task_batch_excel

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

func TestParseExcelFallbackSheet1(t *testing.T) {
	content := testWorkbookOnSheet(t, domain.TaskTypeNewProductDevelopment, "Sheet1", nil)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Preview) != 2 || len(result.Violations) != 0 {
		t.Fatalf("Parse result = %+v, want 2 preview rows and no violations", result)
	}
}

func TestParseExcelFallbackChineseSheetName(t *testing.T) {
	content := testWorkbookOnSheet(t, domain.TaskTypeNewProductDevelopment, "明细", nil)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Preview) != 2 {
		t.Fatalf("preview len = %d, want 2", len(result.Preview))
	}
}

func TestParseExcelPrefersItemsOverSheet1(t *testing.T) {
	fields, _ := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	f := excelize.NewFile()
	defer f.Close()
	_, _ = f.NewSheet("Sheet2")
	for _, sheet := range []string{itemsSheet, "Sheet2"} {
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, field.Column)
		}
		values := validRowValues(domain.TaskTypeNewProductDevelopment, 1)
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			_ = f.SetCellValue(sheet, cell, values[field.Key])
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Preview) != 1 {
		t.Fatalf("preview len = %d, want 1 row from Items sheet", len(result.Preview))
	}
}

func TestParseExcelNoReadableSheet(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	_, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(buf.Bytes()))
	if appErr == nil || appErr.Message != errMsgNoReadableSheet {
		t.Fatalf("Parse appErr = %#v, want message %q", appErr, errMsgNoReadableSheet)
	}
}

func TestParseExcelInvalidHeaderOnFallbackSheet(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "错误表头")
	_ = f.SetCellValue(sheet, "A2", "产品1")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	_, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(buf.Bytes()))
	if appErr == nil || appErr.Message != errMsgExcelHeaderMismatch {
		t.Fatalf("Parse appErr = %#v, want message %q", appErr, errMsgExcelHeaderMismatch)
	}
}

func testWorkbookOnSheet(t *testing.T, taskType domain.TaskType, sheetName string, mutate func(map[string]string)) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), sheetName)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, field.Column)
	}
	for row := 2; row <= 3; row++ {
		values := validRowValues(taskType, row-1)
		if mutate != nil {
			mutate(values)
		}
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(sheetName, cell, values[field.Key])
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}
