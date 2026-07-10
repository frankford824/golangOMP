package asset_center

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"

	"workflow/domain"
)

const MaxExcelPackageUploadBytes = int64(10 * 1024 * 1024)

var (
	excelPackageSKUCodePattern        = regexp.MustCompile(`(?i)\b[A-Z]{2,}[A-Z0-9_-]*\d{2,}[A-Z0-9_-]*\b`)
	excelPackageInlineQuantityPattern = regexp.MustCompile(`^\s*[*xX×]\s*(\d+)`)
)

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

func parseExcelPackageXLS(data []byte) (table [][]string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			table = nil
			err = fmt.Errorf("解析 .xls 工作表失败")
		}
	}()
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
		values, ok := readExcelPackageXLSRowValues(sheet, i)
		if !ok {
			out = append(out, nil)
			continue
		}
		out = append(out, values)
	}
	return out, nil
}

func readExcelPackageXLSRowValues(sheet *xls.WorkSheet, index int) (values []string, ok bool) {
	if sheet == nil {
		return nil, false
	}
	defer func() {
		if recover() != nil {
			values = nil
			ok = false
		}
	}()
	row := sheet.Row(index)
	if row == nil {
		return nil, false
	}
	last := row.LastCol()
	values = make([]string, 0, last+1)
	for j := 0; j <= last; j++ {
		values = append(values, strings.TrimSpace(row.Col(j)))
	}
	return values, true
}

func buildExcelPackageRowsFromTable(table [][]string) ([]ExcelPackageRow, error) {
	if len(table) < 2 {
		return nil, fmt.Errorf("Excel 至少需要表头和一行数据")
	}
	headerIndex := resolveExcelPackageHeaderRow(table)
	headers := make([]string, 0, len(table[0]))
	for _, value := range table[headerIndex] {
		headers = append(headers, normalizeExcelPackageHeader(value))
	}
	orderCol := resolveExcelPackageColumn(headers, excelPackageOrderHeaders(), -1)
	skuCol := resolveExcelPackageColumn(headers, excelPackageSKUHeaders(), -1)
	skuNameCol := resolveExcelPackageColumn(headers, excelPackageSKUNameHeaders(), -1)
	quantityCol := resolveExcelPackageColumn(headers, excelPackageQuantityHeaders(), -1)
	addressCol := resolveExcelPackageColumn(headers, excelPackageAddressHeaders(), -1)
	keywordCol := resolveExcelPackageColumn(headers, excelPackageKeywordHeaders(), -1)
	if excelPackageHeaderScore(headers) == 0 {
		orderCol, skuCol, quantityCol, addressCol, keywordCol = 0, 1, 2, 3, 4
	} else if skuCol < 0 {
		// Keep the original positional SKU fallback only when the template has
		// recognizable headers but uses a non-standard SKU title.
		skuCol = 1
	}

	rows := make([]ExcelPackageRow, 0, len(table)-headerIndex-1)
	for idx, raw := range table[headerIndex+1:] {
		rowNumber := headerIndex + idx + 2
		expanded := buildExcelPackageRowsFromRaw(raw, rowNumber, orderCol, skuCol, skuNameCol, quantityCol, addressCol, keywordCol)
		if len(expanded) == 0 {
			continue
		}
		rows = append(rows, expanded...)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel 中没有可处理的数据行")
	}
	return rows, nil
}

func buildExcelPackageRowsFromRaw(raw []string, rowNumber, orderCol, skuCol, skuNameCol, quantityCol, addressCol, keywordCol int) []ExcelPackageRow {
	orderNo := excelPackageTableCell(raw, orderCol)
	skuCell := excelPackageTableCell(raw, skuCol)
	skuName := excelPackageTableCell(raw, skuNameCol)
	quantityCell := excelPackageTableCell(raw, quantityCol)
	address := excelPackageTableCell(raw, addressCol)
	keyword := excelPackageTableCell(raw, keywordCol)
	skuCodes := extractExcelPackageSKUCodes(skuCell)
	if len(skuCodes) == 0 && skuCell != "" {
		skuCodes = []string{strings.ToUpper(strings.TrimSpace(skuCell))}
	}
	if len(skuCodes) == 0 {
		row := ExcelPackageRow{
			RowNumber: rowNumber,
			OrderNo:   orderNo,
			SKUName:   skuName,
			Quantity:  normalizeExcelPackageQuantity(quantityCell),
			Address:   address,
			Keyword:   keyword,
		}
		if row.OrderNo == "" && row.SKUName == "" {
			return nil
		}
		return []ExcelPackageRow{row}
	}

	quantities := resolveExcelPackageQuantities(skuCell, quantityCell, skuCodes)
	rows := make([]ExcelPackageRow, 0, len(skuCodes))
	for idx, sku := range skuCodes {
		row := ExcelPackageRow{
			RowNumber: rowNumber,
			OrderNo:   firstNonEmptyExcelPackage(orderNo, sku),
			SKUCode:   sku,
			SKUName:   skuName,
			Quantity:  quantities[idx],
			Address:   address,
			Keyword:   keyword,
		}
		if row.OrderNo == "" && row.SKUCode == "" && row.SKUName == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func resolveExcelPackageHeaderRow(table [][]string) int {
	limit := len(table)
	if limit > 10 {
		limit = 10
	}
	bestIndex := 0
	bestScore := -1
	for idx := 0; idx < limit; idx++ {
		headers := make([]string, 0, len(table[idx]))
		for _, value := range table[idx] {
			headers = append(headers, normalizeExcelPackageHeader(value))
		}
		score := excelPackageHeaderScore(headers)
		if score > bestScore {
			bestScore = score
			bestIndex = idx
		}
	}
	return bestIndex
}

func excelPackageHeaderScore(headers []string) int {
	score := 0
	for _, candidates := range [][]string{
		excelPackageOrderHeaders(),
		excelPackageSKUHeaders(),
		excelPackageSKUNameHeaders(),
		excelPackageQuantityHeaders(),
		excelPackageAddressHeaders(),
		excelPackageKeywordHeaders(),
	} {
		if resolveExcelPackageColumn(headers, candidates, -1) >= 0 {
			score++
		}
	}
	return score
}

func excelPackageOrderHeaders() []string {
	return []string{"订单号", "订单编号", "订单", "订单ID", "order_no", "order"}
}

func excelPackageSKUHeaders() []string {
	return []string{"SKU编码", "SKU", "sku_code", "商品编码", "商家编码", "商家SKU", "商品SKU", "产品编码", "款式编码", "编码"}
}

func excelPackageSKUNameHeaders() []string {
	return []string{"SKU名称", "商品名称", "品名", "产品名称", "sku_name", "名称"}
}

func excelPackageQuantityHeaders() []string {
	return []string{"数量", "件数", "商品数量", "订购数量", "qty", "quantity", "num"}
}

func excelPackageAddressHeaders() []string {
	return []string{"地址", "收货地址", "收件地址", "详细地址", "address"}
}

func excelPackageKeywordHeaders() []string {
	return []string{"匹配关键词", "关键词", "搜索关键词", "keyword", "kw"}
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

func extractExcelPackageSKUCodes(value string) []string {
	matches := excelPackageSKUCodePattern.FindAllString(strings.ToUpper(value), -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.Trim(match, "-_")
		if match != "" {
			out = append(out, match)
		}
	}
	return out
}

func resolveExcelPackageQuantities(skuCell, quantityCell string, skuCodes []string) []int {
	defaultQuantity := normalizeExcelPackageQuantity(quantityCell)
	quantities := make([]int, len(skuCodes))
	for idx := range quantities {
		quantities[idx] = defaultQuantity
	}
	if len(skuCodes) == 0 {
		return quantities
	}
	if inline := extractInlineExcelPackageQuantities(skuCell, skuCodes); len(inline) == len(skuCodes) {
		return inline
	}
	parts := splitExcelPackageMultiValueCell(quantityCell)
	if len(parts) == len(skuCodes) {
		for idx, part := range parts {
			quantities[idx] = normalizeExcelPackageQuantity(part)
		}
	}
	return quantities
}

func extractInlineExcelPackageQuantities(skuCell string, skuCodes []string) []int {
	quantities := make([]int, 0, len(skuCodes))
	upper := strings.ToUpper(skuCell)
	for _, sku := range skuCodes {
		idx := strings.Index(upper, sku)
		if idx < 0 {
			return nil
		}
		after := upper[idx+len(sku):]
		if len([]rune(after)) > 12 {
			after = string([]rune(after)[:12])
		}
		quantity := 0
		matches := excelPackageInlineQuantityPattern.FindStringSubmatch(after)
		if len(matches) == 2 {
			quantity = normalizeExcelPackageQuantity(matches[1])
		}
		if quantity <= 0 {
			return nil
		}
		quantities = append(quantities, quantity)
	}
	return quantities
}

func splitExcelPackageMultiValueCell(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case '\n', '\r', ',', '，', ';', '；', '/', '|', '、':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
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
