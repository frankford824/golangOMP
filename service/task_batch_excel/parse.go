package task_batch_excel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/service"
)

type parseService struct{}

var batchFieldPathRE = regexp.MustCompile(`^batch_items\[(\d+)\](?:\.(.+))?$`)

func (s *parseService) Parse(ctx context.Context, taskType domain.TaskType, file io.Reader) (*ParseResult, *domain.AppError) {
	_ = ctx
	fields, ok := FieldsForTaskType(taskType)
	if !ok {
		return nil, unsupportedTaskTypeError(taskType)
	}
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid Excel file", nil)
	}
	defer f.Close()

	rows, err := f.GetRows(itemsSheet)
	if err != nil {
		return nil, excelAppError("read Items sheet", err)
	}
	if len(rows) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing header row", nil)
	}
	columnIndex, appErr := parseHeader(rows[0], fields)
	if appErr != nil {
		return nil, appErr
	}

	items := make([]service.CreateTaskBatchSKUItemParams, 0, len(rows)-1)
	preview := make([]BatchItem, 0, len(rows)-1)
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if rowIsEmpty(row) {
			continue
		}
		item, parseViolations := parseItemRow(row, fields, columnIndex)
		items = append(items, item)
		preview = append(preview, batchItemFromService(item))
		if len(parseViolations) > 0 {
			parseViolations = parseViolationsForRow(rowIdx+1, parseViolations)
			return &ParseResult{TaskType: taskType, Preview: preview, Violations: parseViolations}, nil
		}
	}

	params := service.CreateTaskParams{
		TaskType:     taskType,
		SourceMode:   domain.TaskSourceModeNewProduct,
		BatchSKUMode: "multiple",
		BatchItems:   items,
	}
	if appErr := service.ValidateBatchTaskCreateRequest(params); appErr != nil {
		return &ParseResult{
			TaskType:   taskType,
			Preview:    preview,
			Violations: mapValidationViolations(appErr, fields),
		}, nil
	}
	return &ParseResult{TaskType: taskType, Preview: preview, Violations: []ParseViolation{}}, nil
}

func parseHeader(header []string, fields []FieldSpec) (map[string]int, *domain.AppError) {
	index := make(map[string]int, len(fields))
	byColumn := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		byColumn[strings.TrimSpace(field.Column)] = field
	}
	for i, raw := range header {
		column := strings.TrimSpace(raw)
		if field, ok := byColumn[column]; ok {
			index[field.Key] = i
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := index[field.Key]; !ok {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel header is missing required column", map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     1,
						Column:  field.Column,
						Code:    "missing_required_field",
						Message: "missing required column " + field.Column,
					}},
				})
			}
		}
	}
	return index, nil
}

func parseItemRow(row []string, fields []FieldSpec, columnIndex map[string]int) (service.CreateTaskBatchSKUItemParams, []ParseViolation) {
	var item service.CreateTaskBatchSKUItemParams
	var violations []ParseViolation
	for _, field := range fields {
		idx, ok := columnIndex[field.Key]
		if !ok || idx >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[idx])
		if value == "" {
			continue
		}
		switch field.Key {
		case "product_name":
			item.ProductName = value
		case "product_short_name":
			item.ProductShortName = value
		case "category_code":
			item.CategoryCode = value
		case "material_mode":
			item.MaterialMode = value
		case "design_requirement":
			item.DesignRequirement = value
		case "new_sku":
			item.NewSKU = value
		case "purchase_sku":
			item.PurchaseSKU = value
		case "cost_price_mode":
			item.CostPriceMode = value
		case "quantity":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "missing_required_field", Message: "batch_items[].quantity is required and must be greater than 0"})
				continue
			}
			item.Quantity = &parsed
		case "base_sale_price":
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "missing_required_field", Message: "batch_items[].base_sale_price is required"})
				continue
			}
			item.BaseSalePrice = &parsed
		case "variant_json":
			if !json.Valid([]byte(value)) {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "invalid_variant_json", Message: "batch_items[].variant_json must be valid JSON"})
				continue
			}
			item.VariantJSON = json.RawMessage(value)
		}
	}
	return item, violations
}

func mapValidationViolations(appErr *domain.AppError, fields []FieldSpec) []ParseViolation {
	rawViolations := extractViolations(appErr.Details)
	out := make([]ParseViolation, 0, len(rawViolations))
	byKey := fieldByKey(fields)
	for _, raw := range rawViolations {
		fieldPath, _ := raw["field"].(string)
		code, _ := raw["code"].(string)
		message, _ := raw["message"].(string)
		row, key := rowAndKeyFromFieldPath(fieldPath)
		if key == "sku_code" {
			key = skuColumnKey(fields)
		}
		column := ""
		if field, ok := byKey[key]; ok {
			column = field.Column
		}
		out = append(out, ParseViolation{
			Row:     row,
			Column:  column,
			Code:    code,
			Message: message,
		})
	}
	return out
}

func skuColumnKey(fields []FieldSpec) string {
	for _, field := range fields {
		if field.Key == "new_sku" || field.Key == "purchase_sku" {
			return field.Key
		}
	}
	return "sku_code"
}

func extractViolations(details interface{}) []map[string]interface{} {
	detailMap, ok := details.(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := detailMap["violations"]
	if !ok {
		return nil
	}
	switch violations := raw.(type) {
	case []map[string]interface{}:
		return violations
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(violations))
		for _, item := range violations {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func rowAndKeyFromFieldPath(fieldPath string) (int, string) {
	matches := batchFieldPathRE.FindStringSubmatch(fieldPath)
	if len(matches) == 0 {
		return 0, fieldPath
	}
	idx, _ := strconv.Atoi(matches[1])
	return idx + 2, matches[2]
}

func rowIsEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func batchItemFromService(item service.CreateTaskBatchSKUItemParams) BatchItem {
	return BatchItem{
		ProductName:       item.ProductName,
		ProductShortName:  item.ProductShortName,
		CategoryCode:      item.CategoryCode,
		MaterialMode:      item.MaterialMode,
		DesignRequirement: item.DesignRequirement,
		NewSKU:            item.NewSKU,
		PurchaseSKU:       item.PurchaseSKU,
		CostPriceMode:     item.CostPriceMode,
		Quantity:          item.Quantity,
		BaseSalePrice:     item.BaseSalePrice,
		VariantJSON:       item.VariantJSON,
	}
}

func parseViolationForRow(row int, violation ParseViolation) ParseViolation {
	violation.Row = row
	return violation
}

func parseViolationsForRow(row int, violations []ParseViolation) []ParseViolation {
	for i := range violations {
		violations[i] = parseViolationForRow(row, violations[i])
	}
	return violations
}

func invalidCellMessage(field FieldSpec, value string) string {
	return fmt.Sprintf("%s has invalid value %q", field.Key, value)
}
