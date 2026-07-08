package asset_center

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

const MaxExcelPackageUploadBytes = int64(10 * 1024 * 1024)

func ParseExcelPackageRows(data []byte, filename string) ([]ExcelPackageRow, *domain.AppError) {
	if len(data) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel 文件不能为空", nil)
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	var (
		table [][]string
		err   error
	)
	switch ext {
	case ".xlsx":
		table, err = parseExcelPackageXLSX(data)
	case ".xls":
		table, err = parseExcelPackageXLS(data)
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "仅支持 .xlsx 或 .xls 模板", nil)
	}
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("解析 Excel 失败: %s", err.Error()), nil)
	}
	rows, err := buildExcelPackageRowsFromTable(table)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil)
	}
	return rows, nil
}

func parseExcelPackageXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel 文件没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func parseExcelPackageXLS(data []byte) ([][]string, error) {
	wb, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return nil, err
	}
	if wb.NumSheets() == 0 {
		return nil, fmt.Errorf("Excel 文件没有工作表")
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("Excel 文件没有工作表")
	}
	out := make([][]string, 0, int(sheet.MaxRow)+1)
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			out = append(out, nil)
			continue
		}
		last := row.LastCol()
		values := make([]string, 0, last+1)
		for j := 0; j <= last; j++ {
			values = append(values, strings.TrimSpace(row.Col(j)))
		}
		out = append(out, values)
	}
	return out, nil
}

func buildExcelPackageRowsFromTable(table [][]string) ([]ExcelPackageRow, error) {
	if len(table) < 2 {
		return nil, fmt.Errorf("Excel 至少需要表头和一行数据")
	}
	headers := make([]string, 0, len(table[0]))
	for _, value := range table[0] {
		headers = append(headers, normalizeExcelPackageHeader(value))
	}
	orderCol := resolveExcelPackageColumn(headers, []string{"订单号", "订单编号", "order_no", "order"}, 0)
	skuCol := resolveExcelPackageColumn(headers, []string{"SKU编码", "SKU", "sku_code", "商品编码"}, 1)
	skuNameCol := resolveExcelPackageColumn(headers, []string{"SKU名称", "商品名称", "sku_name", "名称"}, -1)
	quantityCol := resolveExcelPackageColumn(headers, []string{"数量", "qty", "quantity", "num"}, 2)
	addressCol := resolveExcelPackageColumn(headers, []string{"地址", "收货地址", "收件地址", "address"}, 3)
	keywordCol := resolveExcelPackageColumn(headers, []string{"匹配关键词", "关键词", "keyword", "kw"}, 4)

	rows := make([]ExcelPackageRow, 0, len(table)-1)
	for idx, raw := range table[1:] {
		row := ExcelPackageRow{
			RowNumber: idx + 2,
			OrderNo:   excelPackageTableCell(raw, orderCol),
			SKUCode:   strings.ToUpper(excelPackageTableCell(raw, skuCol)),
			SKUName:   excelPackageTableCell(raw, skuNameCol),
			Quantity:  normalizeExcelPackageQuantity(excelPackageTableCell(raw, quantityCol)),
			Address:   excelPackageTableCell(raw, addressCol),
			Keyword:   excelPackageTableCell(raw, keywordCol),
		}
		if row.OrderNo == "" && row.SKUCode == "" && row.SKUName == "" {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel 中没有可处理的数据行")
	}
	return rows, nil
}

func normalizeExcelPackageHeader(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

func resolveExcelPackageColumn(headers []string, candidates []string, fallbackIndex int) int {
	normalized := make([]string, 0, len(candidates))
	for _, item := range candidates {
		normalized = append(normalized, normalizeExcelPackageHeader(item))
	}
	for idx, header := range headers {
		for _, candidate := range normalized {
			if header == candidate {
				return idx
			}
		}
	}
	return fallbackIndex
}

func excelPackageTableCell(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	text := strings.TrimSpace(values[index])
	if strings.HasSuffix(text, ".0") {
		if n, err := strconv.ParseFloat(text, 64); err == nil && n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10)
		}
	}
	return text
}

func normalizeExcelPackageQuantity(value string) int {
	value = excelPackageTableCell([]string{value}, 0)
	if value == "" {
		return 1
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || n <= 0 {
		return 1
	}
	quantity := int(n)
	if quantity <= 0 {
		return 1
	}
	return quantity
}
