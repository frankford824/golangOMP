import { describe, expect, it } from 'vitest'

import { buildImportReviewSource } from './excelReview'

async function qualityWorkbook(headers: string[], values: Array<string | number>) {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  const worksheet = workbook.addWorksheet('出错记录')
  worksheet.addRow(headers)
  worksheet.addRow(values)
  const buffer = await workbook.xlsx.writeBuffer()
  return new File([buffer as BlobPart], 'quality-errors.xlsx', {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
}

describe('asset workbench quality error Excel review', () => {
  it('accepts the business-facing required headers without an order number', async () => {
    const file = await qualityWorkbook(
      ['日期', '出错人', '出错分类', '出错张数', '问题描述'],
      ['2026-07-09', '张三', 'C类', 2, '尺寸错误'],
    )

    const source = await buildImportReviewSource('error-deduction', [file], 'r1')

    expect(source.sheets).toHaveLength(1)
    expect(source.sheets[0]?.validations).toEqual([])
  })

  it('marks a missing error count column as a blocking review issue', async () => {
    const file = await qualityWorkbook(
      ['发生日期', '姓名', '难度类', '问题描述'],
      ['2026-07-09', '李四', 'B类', '文字错误'],
    )

    const source = await buildImportReviewSource('error-deduction', [file], 'r2')

    expect(source.sheets[0]?.validations).toEqual(
      expect.arrayContaining([expect.objectContaining({ tone: 'danger', message: '缺少必要列：出错张数' })]),
    )
  })
})
