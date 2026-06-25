package task_single_excel

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

type parseService struct {
	erpLookup ExcelAssistERPLookup
}

func (s *parseService) Parse(ctx context.Context, taskType domain.TaskType, mode string, file io.Reader, opts ...ParseOption) (*ParseResult, *domain.AppError) {
	if mode != AssistModeSingle {
		return nil, domain.NewAppError("invalid_excel_assist_mode", "Excel assist mode must be single", nil)
	}
	fields, ok := FieldsForTaskType(taskType, mode)
	if !ok {
		return nil, unsupportedTaskTypeError(taskType)
	}
	options := ParseOptions{ERPLookup: s.erpLookup}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid Excel file", nil)
	}
	defer f.Close()

	_, rows, appErr := resolveDataSheet(f)
	if appErr != nil {
		return nil, appErr
	}
	columnIndex, appErr := parseHeader(rows[0], fields)
	if appErr != nil {
		return nil, appErr
	}

	type dataRow struct {
		rowNumber int
		cells     []string
	}
	var dataRows []dataRow
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if rowIsEmpty(row) {
			continue
		}
		dataRows = append(dataRows, dataRow{rowNumber: rowIdx + 1, cells: row})
	}

	base := &ParseResult{TaskType: taskType, Mode: AssistModeSingle, Violations: []ParseViolation{}}

	if len(dataRows) > 1 {
		base.Violations = append(base.Violations, ParseViolation{
			Row:     0,
			Column:  "",
			Code:    "multiple_rows_not_allowed",
			Message: "单任务 Excel 仅允许 1 行有效数据，请删除多余行后重新上传",
		})
		return base, nil
	}

	if len(dataRows) == 0 {
		base.Violations = missingRequiredViolations(2, fields, columnIndex)
		return base, nil
	}

	dr := dataRows[0]
	draft, violations := parseDataRow(dr.cells, fields, columnIndex, dr.rowNumber)
	base.Draft = &draft
	base.Violations = append(base.Violations, violations...)
	if len(base.Violations) > 0 {
		return base, nil
	}

	switch taskType {
	case domain.TaskTypeOriginalProductDevelopment:
		erpViolations, appErr := s.enrichOriginalDraftFromERP(ctx, &draft, dr.rowNumber, fields, options.ERPLookup)
		if appErr != nil {
			return nil, appErr
		}
		if len(erpViolations) > 0 {
			base.Violations = append(base.Violations, erpViolations...)
		}
	default:
		if iidViolations, appErr := s.validateProductIID(ctx, draft.ProductIID, dr.rowNumber, fields, options.ERPLookup); appErr != nil {
			return nil, appErr
		} else if len(iidViolations) > 0 {
			base.Violations = append(base.Violations, iidViolations...)
		}
	}
	return base, nil
}

func parseHeader(header []string, fields []FieldSpec) (map[string]int, *domain.AppError) {
	index := make(map[string]int, len(fields))
	byColumn := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		byColumn[strings.TrimSpace(field.Column)] = field
	}
	legacyProductIIDColumn := "商品编码"
	for i, raw := range header {
		column := strings.TrimSpace(raw)
		if field, ok := byColumn[column]; ok {
			index[field.Key] = i
			continue
		}
		if column == legacyProductIIDColumn {
			if _, exists := index["product_i_id"]; !exists {
				index["product_i_id"] = i
			}
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := index[field.Key]; !ok {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, errMsgExcelHeaderMismatch, map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     1,
						Column:  field.Column,
						Code:    "invalid_header",
						Message: errMsgExcelHeaderMismatch,
					}},
				})
			}
		}
	}
	return index, nil
}

func parseDataRow(row []string, fields []FieldSpec, columnIndex map[string]int, rowNumber int) (SingleTaskDraft, []ParseViolation) {
	var draft SingleTaskDraft
	var violations []ParseViolation
	byKey := fieldByKey(fields)

	for _, field := range fields {
		idx, ok := columnIndex[field.Key]
		if !ok || idx >= len(row) {
			if field.Required {
				violations = append(violations, violationForField(rowNumber, field, field.ViolationCodes.Missing, "missing required value"))
			}
			continue
		}
		value := strings.TrimSpace(row[idx])
		if value == "" {
			if field.Required {
				violations = append(violations, violationForField(rowNumber, field, field.ViolationCodes.Missing, "missing required value"))
			}
			continue
		}
		switch field.Key {
		case "product_i_id":
			draft.ProductIID = value
		case "product_name":
			draft.ProductName = value
		case "design_requirement":
			draft.DesignRequirement = value
		case "spec_text":
			draft.SpecText = value
		case "quantity":
			parsed, qtyViolations := parseQuantityField(rowNumber, field, value)
			violations = append(violations, qtyViolations...)
			if parsed != nil {
				draft.Quantity = parsed
			}
		case "material":
			draft.Material = value
		case "material_other":
			draft.MaterialOther = value
		case "remark":
			draft.Remark = value
		case "sku_code":
			draft.SKUCode = value
		case "change_request":
			draft.ChangeRequest = value
		default:
			_ = byKey
		}
	}
	return draft, violations
}

func missingRequiredViolations(rowNumber int, fields []FieldSpec, columnIndex map[string]int) []ParseViolation {
	var violations []ParseViolation
	for _, field := range fields {
		if !field.Required {
			continue
		}
		if _, ok := columnIndex[field.Key]; !ok {
			continue
		}
		violations = append(violations, violationForField(rowNumber, field, field.ViolationCodes.Missing, "missing required value"))
	}
	return violations
}

func parseQuantityField(rowNumber int, field FieldSpec, value string) (*int64, []ParseViolation) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		code := field.ViolationCodes.Invalid
		if code == "" {
			code = "invalid_quantity"
		}
		return nil, []ParseViolation{violationForField(rowNumber, field, code, "quantity must be a positive integer")}
	}
	return &parsed, nil
}

func violationForField(rowNumber int, field FieldSpec, code, message string) ParseViolation {
	if code == "" {
		code = "missing_required_field"
	}
	return ParseViolation{
		Row:     rowNumber,
		Column:  field.Column,
		Code:    code,
		Message: message,
	}
}

func (s *parseService) validateProductIID(ctx context.Context, iid string, rowNumber int, fields []FieldSpec, lookup ERPIIDLookup) ([]ParseViolation, *domain.AppError) {
	iid = strings.TrimSpace(iid)
	if iid == "" || lookup == nil {
		return nil, nil
	}
	field := fieldByKey(fields)["product_i_id"]
	resp, appErr := lookup.ListIIDs(ctx, domain.ERPIIDListFilter{Q: iid, Page: 1, PageSize: 50})
	if appErr != nil {
		return nil, appErr
	}
	found := false
	if resp != nil {
		for _, option := range resp.Items {
			if option != nil && strings.EqualFold(strings.TrimSpace(option.IID), iid) {
				found = true
				break
			}
		}
	}
	if found {
		return nil, nil
	}
	code := field.ViolationCodes.Invalid
	if code == "" {
		code = "invalid_i_id"
	}
	return []ParseViolation{{
		Row:     rowNumber,
		Column:  field.Column,
		Code:    code,
		Message: "product_i_id must be selected from ERP product i_id options",
	}}, nil
}
