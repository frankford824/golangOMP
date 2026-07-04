import { describe, expect, it } from 'vitest'

import type { WorkbenchSpreadsheetSource } from './types'
import { buildWorkbookSnapshot, extractRowsFromSnapshot } from './workbook'

const source: WorkbenchSpreadsheetSource = {
  id: 'test-workbook',
  revision: 'r1',
  mode: 'settlement',
  title: '测试表格',
  sheets: [
    {
      id: 'payroll',
      name: '工资条',
      rowKey: 'id',
      freezeHeader: true,
      columns: [
        { key: 'id', label: 'ID', width: 72, kind: 'number', readonly: true },
        { key: 'name', label: '人员', width: 120 },
        { key: 'amount', label: '金额', width: 120, kind: 'money', align: 'right' },
      ],
      rows: [{ id: 1, name: '张三', amount: 20 }],
      validations: [{ rowKey: 1, columnKey: 'amount', tone: 'warn', message: '金额待复核' }],
    },
  ],
}

describe('asset workbench spreadsheet workbook adapter', () => {
  it('builds a Univer snapshot with frozen header and styled data cells', () => {
    const snapshot = buildWorkbookSnapshot(source)
    const sheet = snapshot.sheets?.payroll

    expect(snapshot.sheetOrder).toEqual(['payroll'])
    expect(sheet?.freeze).toEqual({ xSplit: 0, ySplit: 1, startRow: 1, startColumn: 0 })
    expect(sheet?.columnData?.[2]?.w).toBe(120)
    expect(sheet?.cellData?.[0]?.[1]?.v).toBe('人员')
    expect(sheet?.cellData?.[1]?.[2]?.v).toBe(20)
    expect(sheet?.cellData?.[1]?.[2]?.s).toBe('aw-warn')
  })

  it('extracts editable cell changes back to business rows', () => {
    const snapshot = buildWorkbookSnapshot(source)
    const sheet = snapshot.sheets?.payroll
    if (!sheet?.cellData?.[1]?.[1]) throw new Error('missing test cell')
    sheet.cellData[1][1].v = '李四'
    if (!sheet.cellData[1][2]) throw new Error('missing amount cell')
    sheet.cellData[1][2].v = 35

    const [result] = extractRowsFromSnapshot(source, snapshot)

    expect(result.rows).toEqual([{ id: 1, name: '李四', amount: 35 }])
  })
})
