package asset_center

import (
	"bytes"
	"testing"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

func TestBuildExcelPackageRowsFromEveTable(t *testing.T) {
	rows, err := buildExcelPackageRowsFromTable([][]string{
		{"订单号", "SKU", "数量", "地址", "关键词"},
		{"SO-1", "hsc12654", "2.0", "张三*地址", "主图"},
		{"SO-1", "HSC07847", "", "", ""},
		{"", "", "", "", ""},
	})
	if err != nil {
		t.Fatalf("buildExcelPackageRowsFromTable() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows)=%d, want 2", len(rows))
	}
	if rows[0].RowNumber != 2 || rows[0].OrderNo != "SO-1" || rows[0].SKUCode != "HSC12654" || rows[0].Quantity != 2 || rows[0].Address != "张三*地址" || rows[0].Keyword != "主图" {
		t.Fatalf("row[0]=%+v", rows[0])
	}
	if rows[1].RowNumber != 3 || rows[1].Quantity != 1 {
		t.Fatalf("row[1]=%+v, want row number 3 and default qty 1", rows[1])
	}
}

func TestBuildExcelPackageRowsExpandsMultiSKUCell(t *testing.T) {
	rows, err := buildExcelPackageRowsFromTable([][]string{
		{"仓库外发模板"},
		{"订单编号", "商家编码", "商品数量", "收货地址", "匹配关键词"},
		{"SO-2", "HSC34425\nHSC34426\nHSC34427", "1\n2\n3", "王五地址", "KT"},
	})
	if err != nil {
		t.Fatalf("buildExcelPackageRowsFromTable() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows)=%d, want 3", len(rows))
	}
	want := []struct {
		sku      string
		quantity int
	}{
		{sku: "HSC34425", quantity: 1},
		{sku: "HSC34426", quantity: 2},
		{sku: "HSC34427", quantity: 3},
	}
	for idx, item := range want {
		got := rows[idx]
		if got.RowNumber != 3 || got.OrderNo != "SO-2" || got.SKUCode != item.sku || got.Quantity != item.quantity || got.Address != "王五地址" || got.Keyword != "KT" {
			t.Fatalf("rows[%d]=%+v, want sku=%s quantity=%d", idx, got, item.sku, item.quantity)
		}
	}
}

func TestBuildExcelPackageRowsExtractsSKUAndInlineQuantity(t *testing.T) {
	rows, err := buildExcelPackageRowsFromTable([][]string{
		{"订单号", "商品SKU", "数量", "地址"},
		{"SO-3", "HSC04325*2，HSC06122×3", "", "赵六地址"},
	})
	if err != nil {
		t.Fatalf("buildExcelPackageRowsFromTable() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows)=%d, want 2", len(rows))
	}
	if rows[0].SKUCode != "HSC04325" || rows[0].Quantity != 2 || rows[1].SKUCode != "HSC06122" || rows[1].Quantity != 3 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestParseExcelPackageRowsXLSX(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	values := [][]interface{}{
		{"订单号", "SKU", "数量", "地址", "关键词"},
		{"SO-9", "hqt12119", 3, "李四地址", "白底"},
	}
	for rowIdx, row := range values {
		for colIdx, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName() error = %v", err)
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				t.Fatalf("SetCellValue() error = %v", err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	rows, appErr := ParseExcelPackageRows(buf.Bytes(), "eve-template.xlsx")
	if appErr != nil {
		t.Fatalf("ParseExcelPackageRows() error = %+v", appErr)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	got := rows[0]
	if got.OrderNo != "SO-9" || got.SKUCode != "HQT12119" || got.Quantity != 3 || got.Keyword != "白底" {
		t.Fatalf("row=%+v", got)
	}
}

func TestReadExcelPackageXLSRowValuesSkipsMissingRows(t *testing.T) {
	values, ok := readExcelPackageXLSRowValues(&xls.WorkSheet{MaxRow: 3}, 1)
	if ok || values != nil {
		t.Fatalf("readExcelPackageXLSRowValues() = (%v, %v), want missing row", values, ok)
	}
}
