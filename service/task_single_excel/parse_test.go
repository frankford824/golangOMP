package task_single_excel

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

type mockERPLookup struct {
	validIIDs   map[string]bool
	skuProducts map[string][]*domain.ERPProduct
	searchErr   *domain.AppError
}

func (m *mockERPLookup) ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	_ = ctx
	if m != nil && m.validIIDs[strings.TrimSpace(filter.Q)] {
		return &domain.ERPIIDListResponse{
			Items: []*domain.ERPIIDOption{{IID: filter.Q}},
		}, nil
	}
	return &domain.ERPIIDListResponse{Items: nil}, nil
}

func (m *mockERPLookup) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	_ = ctx
	if m == nil {
		return &domain.ERPProductListResponse{Items: nil}, nil
	}
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	sku := strings.TrimSpace(filter.SKUCode)
	if sku == "" {
		sku = strings.TrimSpace(filter.Q)
	}
	if m.skuProducts != nil {
		if items, ok := m.skuProducts[sku]; ok {
			return &domain.ERPProductListResponse{Items: items}, nil
		}
	}
	return &domain.ERPProductListResponse{Items: nil}, nil
}

func TestTemplateGenerate_NPD_Single(t *testing.T) {
	content, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypeNewProductDevelopment, AssistModeSingle)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(itemsSheet)
	if err != nil || len(rows) < 1 {
		t.Fatalf("Items rows = %#v err=%v", rows, err)
	}
	wantHeaders := []string{"产品款式编码", "产品名称", "设计要求", "规格尺寸", "材质", "材质备注", "备注"}
	if len(rows[0]) < len(wantHeaders) {
		t.Fatalf("header = %v, want at least %v", rows[0], wantHeaders)
	}
	for i, want := range wantHeaders {
		if rows[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}
	if len(rows) > 2 {
		t.Fatalf("template data rows = %d, want header only (no sample rows)", len(rows)-1)
	}
}

func TestParse_HappyPath_OneRow(t *testing.T) {
	content := testSingleWorkbook(t, map[string]string{
		"product_i_id":       "IID-001",
		"product_name":       "测试产品",
		"design_requirement": "设计要求文案",
		"spec_text":          "10x10cm",
		"material":           "棉",
		"material_other":     "备注材质",
		"remark":             "行备注",
	})
	lookup := &mockERPLookup{validIIDs: map[string]bool{"IID-001": true}}
	result, appErr := NewParseServiceWithDependencies(lookup).Parse(
		t.Context(),
		domain.TaskTypeNewProductDevelopment,
		AssistModeSingle,
		bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v", result.Violations)
	}
	if result.Draft == nil {
		t.Fatal("draft is nil")
	}
	if result.Draft.ProductIID != "IID-001" || result.Draft.ProductName != "测试产品" {
		t.Fatalf("draft = %+v", result.Draft)
	}
	if result.Mode != AssistModeSingle {
		t.Fatalf("mode = %q", result.Mode)
	}
}

func TestParse_MultipleRows_NotAllowed(t *testing.T) {
	content := testSingleWorkbookRows(t,
		map[string]string{
			"product_i_id":       "IID-001",
			"product_name":       "产品A",
			"design_requirement": "要求A",
		},
		map[string]string{
			"product_i_id":       "IID-002",
			"product_name":       "产品B",
			"design_requirement": "要求B",
		},
	)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, AssistModeSingle, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "multiple_rows_not_allowed") {
		t.Fatalf("violations = %+v, want multiple_rows_not_allowed", result.Violations)
	}
}

func TestParse_MissingRequiredField(t *testing.T) {
	content := testSingleWorkbook(t, map[string]string{
		"product_i_id":       "IID-001",
		"product_name":       "",
		"design_requirement": "要求",
	})
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, AssistModeSingle, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "missing_required_field") {
		t.Fatalf("violations = %+v, want missing_required_field", result.Violations)
	}
}

func TestParse_InvalidIID(t *testing.T) {
	content := testSingleWorkbook(t, map[string]string{
		"product_i_id":       "UNKNOWN-IID",
		"product_name":       "产品",
		"design_requirement": "要求",
	})
	lookup := &mockERPLookup{validIIDs: map[string]bool{}}
	result, appErr := NewParseServiceWithDependencies(lookup).Parse(
		t.Context(),
		domain.TaskTypeNewProductDevelopment,
		AssistModeSingle,
		bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "invalid_i_id") {
		t.Fatalf("violations = %+v, want invalid_i_id", result.Violations)
	}
}

func TestParse_InvalidMode(t *testing.T) {
	_, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, "multiple", bytes.NewReader([]byte{}))
	if appErr == nil || appErr.Code != "invalid_excel_assist_mode" {
		t.Fatalf("appErr = %#v, want invalid_excel_assist_mode", appErr)
	}
}

func TestParse_UnsupportedTaskType(t *testing.T) {
	_, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeRetouchTask, AssistModeSingle, bytes.NewReader([]byte{}))
	if appErr == nil || appErr.Code != "excel_assist_task_type_not_supported" {
		t.Fatalf("appErr = %#v, want excel_assist_task_type_not_supported", appErr)
	}
}

func TestPurchaseTaskExcelAssistIsRetired(t *testing.T) {
	_, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypePurchaseTask, AssistModeSingle)
	if appErr == nil || appErr.Code != "excel_assist_task_type_not_supported" {
		t.Fatalf("Generate appErr = %#v, want excel_assist_task_type_not_supported", appErr)
	}
}

func hasViolationCode(violations []ParseViolation, code string) bool {
	for _, v := range violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func testSingleWorkbook(t *testing.T, row map[string]string) []byte {
	t.Helper()
	return testSingleWorkbookForTaskType(t, domain.TaskTypeNewProductDevelopment, row)
}

func testSingleWorkbookForTaskType(t *testing.T, taskType domain.TaskType, row map[string]string) []byte {
	t.Helper()
	return testSingleWorkbookRowsForTaskType(t, taskType, row)
}

func testSingleWorkbookRows(t *testing.T, rows ...map[string]string) []byte {
	t.Helper()
	return testSingleWorkbookRowsForTaskType(t, domain.TaskTypeNewProductDevelopment, rows...)
}

func testSingleWorkbookRowsForTaskType(t *testing.T, taskType domain.TaskType, rows ...map[string]string) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType, AssistModeSingle)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
	}
	for rowIdx, values := range rows {
		for _, field := range fields {
			col, _ := excelize.ColumnNumberToName(fieldIndex(fields, field.Key) + 1)
			cell := col + strconv.Itoa(rowIdx+2)
			if v, ok := values[field.Key]; ok && v != "" {
				_ = f.SetCellValue(itemsSheet, cell, v)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func fieldIndex(fields []FieldSpec, key string) int {
	for i, field := range fields {
		if field.Key == key {
			return i
		}
	}
	return 0
}

func TestTemplateGenerate_Original_Single(t *testing.T) {
	content, appErr := NewTemplateService().Generate(t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(itemsSheet)
	if err != nil || len(rows) < 1 {
		t.Fatalf("Items rows = %#v err=%v", rows, err)
	}
	wantHeaders := []string{"SKU编码", "修改要求", "规格尺寸", "备注"}
	for i, want := range wantHeaders {
		if rows[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}
}

func TestParse_Original_HappyPath_EnrichesERP(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "SKU-ORIG-001",
		"change_request": "改款要求文案",
		"spec_text":      "10x10cm",
		"remark":         "行备注",
	})
	lookup := &mockERPLookup{
		skuProducts: map[string][]*domain.ERPProduct{
			"SKU-ORIG-001": {{
				ProductID:    "ERP-9001",
				SKUID:        "SKU-ORIG-001",
				SKUCode:      "SKU-ORIG-001",
				ProductName:  "原款商品A",
				CategoryCode: "CAT-01",
				CategoryName: "分类A",
				ImageURL:     "https://example.com/a.jpg",
			}},
		},
	}
	result, appErr := NewParseServiceWithDependencies(lookup).Parse(
		t.Context(),
		domain.TaskTypeOriginalProductDevelopment,
		AssistModeSingle,
		bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v", result.Violations)
	}
	if result.Draft == nil {
		t.Fatal("draft is nil")
	}
	if result.Draft.SKUCode != "SKU-ORIG-001" || result.Draft.ChangeRequest != "改款要求文案" {
		t.Fatalf("draft = %+v", result.Draft)
	}
	if result.Draft.ProductID != "ERP-9001" || result.Draft.ProductName != "原款商品A" {
		t.Fatalf("enriched draft = %+v", result.Draft)
	}
	if result.Draft.ERPProduct == nil || result.Draft.ERPProduct.SKUCode != "SKU-ORIG-001" {
		t.Fatalf("erp_product = %+v", result.Draft.ERPProduct)
	}
	if hasViolationCode(result.Violations, "invalid_i_id") {
		t.Fatal("original parse must not emit invalid_i_id")
	}
}

func TestParse_Original_MissingSKU(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "",
		"change_request": "要求",
	})
	result, appErr := NewParseServiceWithDependencies(&mockERPLookup{}).Parse(
		t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "missing_required_field") {
		t.Fatalf("violations = %+v", result.Violations)
	}
}

func TestParse_Original_MissingChangeRequest(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "SKU-ORIG-001",
		"change_request": "",
	})
	result, appErr := NewParseServiceWithDependencies(&mockERPLookup{}).Parse(
		t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "missing_required_field") {
		t.Fatalf("violations = %+v", result.Violations)
	}
}

func TestParse_Original_MultipleRows_NotAllowed(t *testing.T) {
	content := testSingleWorkbookRowsForTaskType(t, domain.TaskTypeOriginalProductDevelopment,
		map[string]string{"sku_code": "SKU-A", "change_request": "要求A"},
		map[string]string{"sku_code": "SKU-B", "change_request": "要求B"},
	)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "multiple_rows_not_allowed") {
		t.Fatalf("violations = %+v", result.Violations)
	}
}

func TestParse_Original_ProductNotFound(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "MISSING-SKU",
		"change_request": "要求",
	})
	result, appErr := NewParseServiceWithDependencies(&mockERPLookup{}).Parse(
		t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "product_not_found") {
		t.Fatalf("violations = %+v", result.Violations)
	}
	if hasViolationCode(result.Violations, "invalid_i_id") {
		t.Fatal("must not use invalid_i_id for original")
	}
}

func TestParse_Original_AmbiguousProduct(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "DUP-SKU",
		"change_request": "要求",
	})
	lookup := &mockERPLookup{
		skuProducts: map[string][]*domain.ERPProduct{
			"DUP-SKU": {
				{ProductID: "ERP-1", SKUCode: "DUP-SKU", ProductName: "A"},
				{ProductID: "ERP-2", SKUCode: "DUP-SKU", ProductName: "B"},
			},
		},
	}
	result, appErr := NewParseServiceWithDependencies(lookup).Parse(
		t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "ambiguous_product") {
		t.Fatalf("violations = %+v", result.Violations)
	}
}

func TestParse_Original_ERPLookupFailed(t *testing.T) {
	content := testSingleWorkbookForTaskType(t, domain.TaskTypeOriginalProductDevelopment, map[string]string{
		"sku_code":       "SKU-ERR",
		"change_request": "要求",
	})
	lookup := &mockERPLookup{
		searchErr: domain.NewAppError("erp_unavailable", "erp bridge down", nil),
	}
	result, appErr := NewParseServiceWithDependencies(lookup).Parse(
		t.Context(), domain.TaskTypeOriginalProductDevelopment, AssistModeSingle, bytes.NewReader(content),
	)
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolationCode(result.Violations, "erp_lookup_failed") {
		t.Fatalf("violations = %+v", result.Violations)
	}
}
