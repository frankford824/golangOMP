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

func TestParseExcelSupportsTemplateReferenceColumns(t *testing.T) {
	content := testWorkbookWithReferenceHeaders(
		t,
		domain.TaskTypeNewProductDevelopment,
		[]string{"参考图1", "参考图2", "参考图3", "参考图4"},
	)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if len(result.Preview) != 2 {
		t.Fatalf("preview len = %d, want 2", len(result.Preview))
	}
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

func TestParseExcelAllowsSameProductStyleWithDifferentDesignRequirement(t *testing.T) {
	content := testWorkbookRows(t, domain.TaskTypeNewProductDevelopment, []map[string]string{
		{
			"product_name":       "常规海报/升学宴//5条",
			"product_i_id":       "常规海报",
			"design_requirement": "参考图1的色调，文字改成升学宴的主题，尺寸参考第二张图的",
		},
		{
			"product_name":       "常规海报/升学宴//5条",
			"product_i_id":       "常规海报",
			"design_requirement": "参考图1的色调，文字改成升学宴的主题，尺寸和参考图二一样，元素稍微改动一点",
		},
		{
			"product_name":       "常规海报/升学宴//5条",
			"product_i_id":       "常规海报",
			"design_requirement": "参考图1的色调，尺寸等比例缩小一些，元素稍微改动一点",
		},
	})
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if len(result.Preview) != 3 {
		t.Fatalf("preview len = %d, want 3", len(result.Preview))
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

func TestParseExcelAssignsReferenceImageByVisualCenterRow(t *testing.T) {
	content := testWorkbookWithImageCenteredInNextRow(t)
	uploader := &parseReferenceUploaderStub{}
	result, appErr := NewParseServiceWithDependencies(uploader, nil).Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content), WithActorID(42))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if len(result.Preview) != 2 {
		t.Fatalf("preview len = %d, want 2", len(result.Preview))
	}
	if len(result.Preview[0].ReferenceFileRefs) != 0 {
		t.Fatalf("row 2 reference_file_refs = %+v, want none", result.Preview[0].ReferenceFileRefs)
	}
	if len(result.Preview[1].ReferenceFileRefs) != 1 {
		t.Fatalf("row 3 reference_file_refs = %+v, want 1", result.Preview[1].ReferenceFileRefs)
	}
	if len(uploader.filenames) != 1 || uploader.filenames[0] != "batch-row-3-reference-1.png" {
		t.Fatalf("uploaded filenames = %+v, want batch-row-3-reference-1.png", uploader.filenames)
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

func TestParseExcelSupportsLegacyIIDColumnName(t *testing.T) {
	content := testWorkbookWithCustomHeaderColumns(
		t,
		domain.TaskTypeNewProductDevelopment,
		map[string]string{"product_i_id": "商品编码"},
		func(row map[string]string) {
			row["product_i_id"] = "IID-LEGACY-ONLY"
		},
	)
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if len(result.Preview) == 0 || strings.TrimSpace(result.Preview[0].ProductIID) != "IID-LEGACY-ONLY" {
		t.Fatalf("preview product_i_id = %+v, want IID-LEGACY-ONLY", result.Preview)
	}
}

func TestParseExcelDualIIDColumnsUseSingleNonEmptyValue(t *testing.T) {
	content := testWorkbookWithDualIIDColumns(t, "IID-ONLY-NEW", "")
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %+v, want none", result.Violations)
	}
	if got := strings.TrimSpace(result.Preview[0].ProductIID); got != "IID-ONLY-NEW" {
		t.Fatalf("preview product_i_id = %q, want IID-ONLY-NEW", got)
	}
}

func TestParseExcelDualIIDColumnsConflictReturnsRowViolation(t *testing.T) {
	content := testWorkbookWithDualIIDColumns(t, "IID-NEW", "IID-LEGACY")
	result, appErr := NewParseService().Parse(t.Context(), domain.TaskTypeNewProductDevelopment, bytes.NewReader(content))
	if appErr != nil {
		t.Fatalf("Parse appErr = %v", appErr)
	}
	if len(result.Violations) == 0 {
		t.Fatalf("violations = %+v, want conflict violation", result.Violations)
	}
	if !hasViolation(result.Violations, "product_i_id", "conflicting_product_i_id_columns") {
		t.Fatalf("violations = %+v, want conflicting_product_i_id_columns", result.Violations)
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
	expectedHeaders := expectedTemplateHeaders(taskType)
	if len(rows) == 0 || len(rows[0]) != len(expectedHeaders) {
		t.Fatalf("header row = %#v, expected headers=%d", rows, len(expectedHeaders))
	}
	for i, header := range expectedHeaders {
		if rows[0][i] != header {
			t.Fatalf("header[%d] = %q, want %q", i, rows[0][i], header)
		}
	}
}

func expectedTemplateHeaders(taskType domain.TaskType) []string {
	fields, _ := FieldsForTaskType(taskType)
	headers := make([]string, 0, len(fields)+3)
	for _, field := range fields {
		if field.Key != "reference_image" {
			headers = append(headers, field.Column)
			continue
		}
		headers = append(headers, "参考图1", "参考图2", "参考图3", "参考图4")
	}
	return headers
}

func testWorkbook(t *testing.T, taskType domain.TaskType, mutate func(map[string]string)) []byte {
	t.Helper()
	rows := make([]map[string]string, 0, 2)
	for row := 2; row <= 3; row++ {
		values := validRowValues(taskType, row-1)
		if mutate != nil {
			mutate(values)
		}
		rows = append(rows, values)
	}
	return testWorkbookRows(t, taskType, rows)
}

func testWorkbookRows(t *testing.T, taskType domain.TaskType, rowValues []map[string]string) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
	}
	for i, values := range rowValues {
		row := i + 2
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

func testWorkbookWithImageCenteredInNextRow(t *testing.T) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	_ = f.SetRowHeight(itemsSheet, 2, 17)
	_ = f.SetRowHeight(itemsSheet, 3, 17)
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
	}
	for row := 2; row <= 3; row++ {
		values := validRowValues(domain.TaskTypeNewProductDevelopment, row-1)
		for i, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(itemsSheet, cell, values[field.Key])
		}
	}
	if err := f.AddPictureFromBytes(itemsSheet, "D2", &excelize.Picture{
		Extension: ".png",
		File:      solidPNG(20, 20),
		Format: &excelize.GraphicOptions{
			OffsetY: 12,
		},
	}); err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func testWorkbookWithCustomHeaderColumns(
	t *testing.T,
	taskType domain.TaskType,
	headerOverrides map[string]string,
	mutate func(map[string]string),
) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	for i, field := range fields {
		header := field.Column
		if override, ok := headerOverrides[field.Key]; ok && strings.TrimSpace(override) != "" {
			header = strings.TrimSpace(override)
		}
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, header)
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

func testWorkbookWithReferenceHeaders(
	t *testing.T,
	taskType domain.TaskType,
	referenceHeaders []string,
) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(taskType)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)

	col := 1
	for _, field := range fields {
		if field.Key != "reference_image" {
			cell, _ := excelize.CoordinatesToCellName(col, 1)
			_ = f.SetCellValue(itemsSheet, cell, field.Column)
			col++
			continue
		}
		for _, header := range referenceHeaders {
			cell, _ := excelize.CoordinatesToCellName(col, 1)
			_ = f.SetCellValue(itemsSheet, cell, header)
			col++
		}
	}
	for row := 2; row <= 3; row++ {
		values := validRowValues(taskType, row-1)
		col = 1
		for _, field := range fields {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			_ = f.SetCellValue(itemsSheet, cell, values[field.Key])
			col++
			if field.Key == "reference_image" {
				col += len(referenceHeaders) - 1
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func testWorkbookWithDualIIDColumns(t *testing.T, primaryIID string, legacyIID string) []byte {
	t.Helper()
	fields, _ := FieldsForTaskType(domain.TaskTypeNewProductDevelopment)
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName(f.GetSheetName(0), itemsSheet)
	productIIDColumn := 0
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(itemsSheet, cell, field.Column)
		if field.Key == "product_i_id" {
			productIIDColumn = i + 1
		}
	}
	if productIIDColumn == 0 {
		t.Fatal("product_i_id column not found")
	}
	legacyColumn := len(fields) + 1
	legacyHeaderCell, _ := excelize.CoordinatesToCellName(legacyColumn, 1)
	_ = f.SetCellValue(itemsSheet, legacyHeaderCell, "商品编码")
	for row := 2; row <= 3; row++ {
		values := validRowValues(domain.TaskTypeNewProductDevelopment, row-1)
		primaryCell, _ := excelize.CoordinatesToCellName(productIIDColumn, row)
		legacyCell, _ := excelize.CoordinatesToCellName(legacyColumn, row)
		if row == 2 {
			_ = f.SetCellValue(itemsSheet, primaryCell, primaryIID)
			_ = f.SetCellValue(itemsSheet, legacyCell, legacyIID)
		} else {
			_ = f.SetCellValue(itemsSheet, primaryCell, values["product_i_id"])
			_ = f.SetCellValue(itemsSheet, legacyCell, values["product_i_id"])
		}
		for i, field := range fields {
			if field.Key == "product_i_id" {
				continue
			}
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
	filenames []string
}

func (s *parseReferenceUploaderStub) UploadFile(_ context.Context, params service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError) {
	s.calls++
	s.createdBy = params.CreatedBy
	s.filenames = append(s.filenames, params.Filename)
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
	return solidPNG(1, 1)
}

func solidPNG(width, height int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	if width > 0 && height > 0 {
		img = image.NewNRGBA(image.Rect(0, 0, width, height))
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
