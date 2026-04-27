package task_batch_excel

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

const (
	itemsSheet    = "Items"
	schemaSheet   = "Schema"
	enumDictSheet = "EnumDict"
)

type templateService struct{}

func (s *templateService) Generate(ctx context.Context, taskType domain.TaskType) ([]byte, *domain.AppError) {
	_ = ctx
	fields, ok := FieldsForTaskType(taskType)
	if !ok {
		return nil, unsupportedTaskTypeError(taskType)
	}

	f := excelize.NewFile()
	defer f.Close()
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != itemsSheet {
		_ = f.SetSheetName(defaultSheet, itemsSheet)
	}
	if _, err := f.NewSheet(schemaSheet); err != nil {
		return nil, excelAppError("create schema sheet", err)
	}
	if _, err := f.NewSheet(enumDictSheet); err != nil {
		return nil, excelAppError("create enum dict sheet", err)
	}
	f.SetActiveSheet(0)

	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(itemsSheet, cell, field.Column); err != nil {
			return nil, excelAppError("write template header", err)
		}
	}
	writePlaceholderRow(f, fields, taskType, 2, 0)
	writePlaceholderRow(f, fields, taskType, 3, 1)
	if appErr := writeSchemaSheet(f, fields); appErr != nil {
		return nil, appErr
	}
	if appErr := writeEnumDictSheet(f); appErr != nil {
		return nil, appErr
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, excelAppError("write template", err)
	}
	return buf.Bytes(), nil
}

func writePlaceholderRow(f *excelize.File, fields []FieldSpec, taskType domain.TaskType, row int, sampleIdx int) {
	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		_ = f.SetCellValue(itemsSheet, cell, placeholderValue(field, taskType, sampleIdx))
	}
}

func placeholderValue(field FieldSpec, taskType domain.TaskType, idx int) interface{} {
	switch field.Key {
	case "product_name":
		if taskType == domain.TaskTypePurchaseTask {
			return fmt.Sprintf("采购样品%d", idx+1)
		}
		return fmt.Sprintf("新品样品%d", idx+1)
	case "product_short_name":
		return "样品"
	case "category_code":
		return "SAMPLE_CATEGORY"
	case "material_mode":
		return string(domain.MaterialModeOther)
	case "design_requirement":
		return "出单画图"
	case "new_sku":
		return fmt.Sprintf("R5-NPD-SAMPLE-%03d", idx+1)
	case "purchase_sku":
		return fmt.Sprintf("R5-PT-SAMPLE-%03d", idx+1)
	case "cost_price_mode":
		return string(domain.CostPriceModeManual)
	case "quantity":
		return int64(1)
	case "base_sale_price":
		return float64(9.9)
	case "variant_json":
		return fmt.Sprintf(`{"sample":%d}`, idx+1)
	default:
		return ""
	}
}

func writeSchemaSheet(f *excelize.File, fields []FieldSpec) *domain.AppError {
	headers := []string{"column", "key", "required", "format", "allowed_values", "help_text"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(schemaSheet, cell, header); err != nil {
			return excelAppError("write schema header", err)
		}
	}
	for row, field := range fields {
		values := []interface{}{field.Column, field.Key, field.Required, string(field.Format), strings.Join(field.AllowedValues, ","), field.HelpText}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row+2)
			if err := f.SetCellValue(schemaSheet, cell, value); err != nil {
				return excelAppError("write schema row", err)
			}
		}
	}
	return nil
}

func writeEnumDictSheet(f *excelize.File) *domain.AppError {
	headers := []string{"enum", "value"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(enumDictSheet, cell, header); err != nil {
			return excelAppError("write enum header", err)
		}
	}
	row := 2
	for name, values := range EnumDictionary() {
		for _, value := range values {
			if err := f.SetCellValue(enumDictSheet, fmt.Sprintf("A%d", row), name); err != nil {
				return excelAppError("write enum name", err)
			}
			if err := f.SetCellValue(enumDictSheet, fmt.Sprintf("B%d", row), value); err != nil {
				return excelAppError("write enum value", err)
			}
			row++
		}
	}
	return nil
}

func unsupportedTaskTypeError(taskType domain.TaskType) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "task_type is not supported for batch Excel", map[string]interface{}{
		"violations": []map[string]interface{}{
			{
				"field":   "task_type",
				"code":    "batch_not_supported_for_task_type",
				"message": fmt.Sprintf("%s does not support batch_sku_mode=multiple", taskType),
			},
		},
	})
}

func excelAppError(action string, err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, action+": "+err.Error(), nil)
}
