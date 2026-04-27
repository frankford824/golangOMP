package task_batch_excel

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/service"
)

func TestFieldsAlignWithCreateTaskBatchSKUItemParams(t *testing.T) {
	got := map[string]bool{}
	for _, taskType := range []domain.TaskType{domain.TaskTypeNewProductDevelopment, domain.TaskTypePurchaseTask} {
		fields, ok := FieldsForTaskType(taskType)
		if !ok {
			t.Fatalf("FieldsForTaskType(%s) missing", taskType)
		}
		for _, field := range fields {
			got[field.Key] = true
		}
	}
	want := map[string]bool{}
	rt := reflect.TypeOf(service.CreateTaskBatchSKUItemParams{})
	for i := 0; i < rt.NumField(); i++ {
		key := lowerSnake(rt.Field(i).Name)
		if key == "reference_file_refs" {
			continue
		}
		want[key] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("field keys = %#v, want %#v", got, want)
	}
}

func TestFieldsAlignWithValidateBatchTaskCreateRequest(t *testing.T) {
	cases := []struct {
		taskType domain.TaskType
		fieldKey string
	}{
		{domain.TaskTypeNewProductDevelopment, "product_name"},
		{domain.TaskTypeNewProductDevelopment, "product_short_name"},
		{domain.TaskTypeNewProductDevelopment, "category_code"},
		{domain.TaskTypeNewProductDevelopment, "material_mode"},
		{domain.TaskTypeNewProductDevelopment, "design_requirement"},
		{domain.TaskTypePurchaseTask, "product_name"},
		{domain.TaskTypePurchaseTask, "category_code"},
		{domain.TaskTypePurchaseTask, "cost_price_mode"},
		{domain.TaskTypePurchaseTask, "quantity"},
		{domain.TaskTypePurchaseTask, "base_sale_price"},
	}
	for _, tc := range cases {
		t.Run(string(tc.taskType)+"/"+tc.fieldKey, func(t *testing.T) {
			content := testWorkbook(t, tc.taskType, func(row map[string]string) {
				row[tc.fieldKey] = ""
			})
			result, appErr := NewParseService().Parse(t.Context(), tc.taskType, bytes.NewReader(content))
			if appErr != nil {
				t.Fatalf("Parse appErr = %v", appErr)
			}
			if !hasViolation(result.Violations, tc.fieldKey, "missing_required_field") {
				t.Fatalf("violations = %#v, want missing_required_field for %s", result.Violations, tc.fieldKey)
			}
		})
	}
}

func TestViolationCodeDictionaryAligns(t *testing.T) {
	for _, taskType := range []domain.TaskType{domain.TaskTypeNewProductDevelopment, domain.TaskTypePurchaseTask} {
		fields, _ := FieldsForTaskType(taskType)
		for _, field := range fields {
			if field.Required && field.ViolationCodes.Missing != "missing_required_field" {
				t.Fatalf("%s/%s missing code = %s", taskType, field.Key, field.ViolationCodes.Missing)
			}
			if field.Key == "material_mode" && field.ViolationCodes.Invalid != "invalid_material_mode" {
				t.Fatalf("material_mode invalid code = %s", field.ViolationCodes.Invalid)
			}
			if field.Key == "cost_price_mode" && field.ViolationCodes.Invalid != "invalid_cost_price_mode" {
				t.Fatalf("cost_price_mode invalid code = %s", field.ViolationCodes.Invalid)
			}
		}
	}
}

func TestTemplateGenerateNPD(t *testing.T) {
	assertTemplateHeaders(t, domain.TaskTypeNewProductDevelopment)
}

func TestTemplateGeneratePT(t *testing.T) {
	assertTemplateHeaders(t, domain.TaskTypePurchaseTask)
}

func TestParseValidExcel(t *testing.T) {
	content := testWorkbook(t, domain.TaskTypeNewProductDevelopment, nil)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Preview) != 2 || len(result.Violations) != 0 {
		t.Fatalf("Parse result = %+v, want 2 preview rows and no violations", result)
	}
}

func TestParseMissingRequired(t *testing.T) {
	content := testWorkbook(t, domain.TaskTypeNewProductDevelopment, func(row map[string]string) {
		row["product_name"] = ""
	})
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolation(result.Violations, "product_name", "missing_required_field") {
		t.Fatalf("violations = %#v, want product_name missing", result.Violations)
	}
}

func TestParseInvalidEnum(t *testing.T) {
	content := testWorkbook(t, domain.TaskTypeNewProductDevelopment, func(row map[string]string) {
		row["material_mode"] = "foo"
	})
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolation(result.Violations, "material_mode", "invalid_material_mode") {
		t.Fatalf("violations = %#v, want material_mode invalid", result.Violations)
	}
}

func TestParseDuplicateBatchSKU(t *testing.T) {
	content := testWorkbook(t, domain.TaskTypeNewProductDevelopment, func(row map[string]string) {
		row["new_sku"] = "DUP-SKU"
	})
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolation(result.Violations, "new_sku", "duplicate_batch_sku") {
		t.Fatalf("violations = %#v, want duplicate_batch_sku", result.Violations)
	}
}

func TestParseTaskTypeNotSupported(t *testing.T) {
	content := testWorkbook(t, domain.TaskTypeNewProductDevelopment, nil)
	_, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeOriginalProductDevelopment, bytes.NewReader(content))
	if appErr == nil || !appErrorHasCode(appErr, "batch_not_supported_for_task_type") {
		t.Fatalf("Parse appErr = %#v, want batch_not_supported_for_task_type", appErr)
	}
}

func assertTemplateHeaders(t *testing.T, taskType domain.TaskType) {
	t.Helper()
	content, appErr := NewTemplateService().Generate(t.Context(), taskType)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open generated template: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(itemsSheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	fields, _ := FieldsForTaskType(taskType)
	if len(rows) == 0 || len(rows[0]) != len(fields) {
		t.Fatalf("header row = %#v, fields=%d", rows, len(fields))
	}
	for i, field := range fields {
		if rows[0][i] != field.Column {
			t.Fatalf("header[%d] = %q, want %q", i, rows[0][i], field.Column)
		}
	}
}

func testWorkbook(t *testing.T, taskType domain.TaskType, mutate func(map[string]string)) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
	}
	for row := 2; row <= 3; row++ {
		values := validRowValues(taskType, row-1)
		if mutate != nil {
			mutate(values)
		}
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(itemsSheet, cell, values[field.Key])
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func validRowValues(taskType domain.TaskType, idx int) map[string]string {
	values := map[string]string{
		"product_name":       "产品",
		"product_short_name": "简称",
		"category_code":      "CAT",
		"material_mode":      string(domain.MaterialModeOther),
		"design_requirement": "出单画图",
		"new_sku":            "NPD-SKU-" + strconv.Itoa(idx),
		"variant_json":       `{"idx":` + strconv.Itoa(idx) + `}`,
	}
	if taskType == domain.TaskTypePurchaseTask {
		values = map[string]string{
			"product_name":    "采购产品",
			"category_code":   "CAT",
			"cost_price_mode": string(domain.CostPriceModeManual),
			"quantity":        "2",
			"base_sale_price": "10.5",
			"purchase_sku":    "PT-SKU-" + strconv.Itoa(idx),
			"variant_json":    `{"idx":` + strconv.Itoa(idx) + `}`,
		}
	}
	return values
}

func hasViolation(violations []ParseViolation, fieldKey string, code string) bool {
	fields, _ := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	fields = append(fields, ptFields...)
	columns := map[string]bool{}
	for _, field := range fields {
		if field.Key == fieldKey {
			columns[field.Column] = true
		}
	}
	for _, violation := range violations {
		if violation.Code == code && columns[violation.Column] {
			return true
		}
	}
	return false
}

func appErrorHasCode(appErr *domain.AppError, code string) bool {
	for _, violation := range extractViolations(appErr.Details) {
		if violation["code"] == code {
			return true
		}
	}
	return false
}

func lowerSnake(s string) string {
	switch s {
	case "NewSKU":
		return "new_sku"
	case "PurchaseSKU":
		return "purchase_sku"
	case "VariantJSON":
		return "variant_json"
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
