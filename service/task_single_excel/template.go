package task_single_excel

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

type templateService struct{}

func (s *templateService) Generate(ctx context.Context, taskType domain.TaskType, mode string) ([]byte, *domain.AppError) {
	_ = ctx
	fields, ok := FieldsForTaskType(taskType, mode)
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
	f.SetActiveSheet(0)

	for i, field := range fields {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(itemsSheet, cell, field.Column); err != nil {
			return nil, excelAppError("write template header", err)
		}
	}
	if appErr := writeSchemaSheet(f, fields); appErr != nil {
		return nil, appErr
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, excelAppError("write template", err)
	}
	return buf.Bytes(), nil
}

func writeSchemaSheet(f *excelize.File, fields []FieldSpec) *domain.AppError {
	headers := []string{"列名", "字段键", "必填", "说明"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(schemaSheet, cell, header); err != nil {
			return excelAppError("write schema header", err)
		}
	}
	row := 2
	for _, field := range fields {
		required := "否"
		if field.Required {
			required = "是"
		}
		values := []interface{}{field.Column, field.Key, required, field.HelpText}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			if err := f.SetCellValue(schemaSheet, cell, value); err != nil {
				return excelAppError("write schema row", err)
			}
		}
		row++
	}
	return nil
}

func unsupportedTaskTypeError(taskType domain.TaskType) *domain.AppError {
	return domain.NewAppError("excel_assist_task_type_not_supported", fmt.Sprintf("%s is not supported for single-task Excel assist", taskType), nil)
}

func excelAppError(action string, err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, action+": "+err.Error(), nil)
}
