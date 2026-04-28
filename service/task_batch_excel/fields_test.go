package task_batch_excel

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/service"
)

func TestFieldsForNPDUseMinimalBatchTemplate(t *testing.T) {
	fields, ok := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	if !ok {
		t.Fatal("FieldsForTaskType(new_product_development) missing")
	}
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Key)
	}
	want := []string{"product_name", "design_requirement", "product_i_id", "reference_image"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("NPD field keys = %v, want %v", got, want)
	}
}

func TestFieldsAlignWithValidateBatchTaskCreateRequest(t *testing.T) {
	cases := []struct {
		taskType domain.TaskType
		fieldKey string
	}{
		{domain.TaskTypeNewProductDevelopment, "product_name"},
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

func TestParseExcelUploadsEmbeddedReferenceImagesAndValidatesIID(t *testing.T) {
	content := testWorkbookWithImage(t, "IID-OK")
	uploader := &parseReferenceUploaderStub{}
	lookup := &parseIIDLookupStub{valid: map[string]bool{"IID-OK": true}}
	result, appErr := NewParseServiceWithDependencies(uploader, lookup).Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content), WithActorID(42))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if len(result.Preview) != 2 {
		t.Fatalf("preview len = %d, want 2", len(result.Preview))
	}
	if len(result.Preview[0].ReferenceFileRefs) != 1 {
		t.Fatalf("row 2 reference_file_refs = %+v, want 1", result.Preview[0].ReferenceFileRefs)
	}
	if uploader.calls != 1 {
		t.Fatalf("upload calls = %d, want 1", uploader.calls)
	}
	if uploader.createdBy != 42 {
		t.Fatalf("created_by = %d, want 42", uploader.createdBy)
	}
	if lookup.queries["IID-OK"] != 1 {
		t.Fatalf("iid queries = %+v, want IID-OK once", lookup.queries)
	}
}

func TestParseExcelRejectsInvalidIIDBeforeUploadingImages(t *testing.T) {
	content := testWorkbookWithImage(t, "BAD-IID")
	uploader := &parseReferenceUploaderStub{}
	lookup := &parseIIDLookupStub{valid: map[string]bool{"IID-OK": true}}
	result, appErr := NewParseServiceWithDependencies(uploader, lookup).Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content), WithActorID(42))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if !hasViolation(result.Violations, "product_i_id", "invalid_i_id") {
		t.Fatalf("violations = %+v, want invalid_i_id", result.Violations)
	}
	if uploader.calls != 0 {
		t.Fatalf("upload calls = %d, want 0 when iid invalid", uploader.calls)
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

func testWorkbookWithImage(t *testing.T, iid string) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
	}
	for row := 2; row <= 3; row++ {
		values := validRowValues(domain.TaskTypeNewProductDevelopment, row-1)
		if row == 2 {
			values["product_i_id"] = iid
		}
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(itemsSheet, cell, values[field.Key])
		}
	}
	if err := f.AddPictureFromBytes(itemsSheet, "D2", &excelize.Picture{
		Extension: ".png",
		File:      tinyPNG(),
	}); err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func validRowValues(taskType domain.TaskType, idx int) map[string]string {
	values := map[string]string{
		"product_name":       "产品" + strconv.Itoa(idx),
		"design_requirement": "出单画图",
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

type parseReferenceUploaderStub struct {
	calls     int
	createdBy int64
}

func (s *parseReferenceUploaderStub) UploadFile(_ context.Context, params service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError) {
	s.calls++
	s.createdBy = params.CreatedBy
	ref := domain.ReferenceFileRef{
		AssetID:         "asset-from-excel",
		RefID:           "asset-from-excel",
		UploadRequestID: "upload-from-excel",
		Filename:        params.Filename,
		MimeType:        params.MimeType,
		Status:          "uploaded",
		Source:          domain.ReferenceFileRefSourceTaskReferenceUpload,
	}
	ref.Normalize()
	return &ref, nil
}

type parseIIDLookupStub struct {
	valid   map[string]bool
	queries map[string]int
}

func (s *parseIIDLookupStub) ListIIDs(_ context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	if s.queries == nil {
		s.queries = map[string]int{}
	}
	s.queries[filter.Q]++
	if s.valid[filter.Q] {
		return &domain.ERPIIDListResponse{Items: []*domain.ERPIIDOption{{IID: filter.Q, Label: filter.Q}}}, nil
	}
	return &domain.ERPIIDListResponse{Items: []*domain.ERPIIDOption{}}, nil
}

func tinyPNG() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
