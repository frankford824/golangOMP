package task_batch_excel

import (
	"strings"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

const (
	errMsgNoReadableSheet   = "Excel 文件中没有可解析的数据，请使用系统下载的模板填写后上传。"
	errMsgExcelHeaderMismatch = "Excel 表头不符合模板要求，请使用系统下载的模板，或确认第一行表头未被修改。"
)

var itemsSheetAliases = []string{
	"明细",
	"数据",
	"商品明细",
	"批量明细",
	"SKU明细",
}

// resolveDataSheet picks the workbook sheet that carries batch item rows.
// Priority: Items → known Chinese aliases → first non-empty sheet (excluding Schema/EnumDict).
func resolveDataSheet(f *excelize.File) (sheetName string, rows [][]string, appErr *domain.AppError) {
	candidates := append([]string{itemsSheet}, itemsSheetAliases...)
	for _, name := range candidates {
		if rows, ok := trySheetRows(f, name); ok {
			return name, rows, nil
		}
	}
	for _, name := range f.GetSheetList() {
		if isMetaSheet(name) {
			continue
		}
		if rows, ok := trySheetRows(f, name); ok {
			return name, rows, nil
		}
	}
	return "", nil, domain.NewAppError(domain.ErrCodeInvalidRequest, errMsgNoReadableSheet, nil)
}

func trySheetRows(f *excelize.File, name string) ([][]string, bool) {
	rows, err := f.GetRows(name)
	if err != nil {
		return nil, false
	}
	if !sheetHasContent(rows) {
		return nil, false
	}
	return rows, true
}

func sheetHasContent(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				return true
			}
		}
	}
	return false
}

func isMetaSheet(name string) bool {
	switch strings.TrimSpace(name) {
	case schemaSheet, enumDictSheet:
		return true
	default:
		return false
	}
}
