import { describe, expect, it } from 'vitest'

import type { SettlementReportRow } from '@aw/shared/api/assetWorkbenchApi'

import { buildSettlementPayrollExportRows, buildSettlementReportExportRow, payrollRowLabel } from './settlementExport'

describe('asset workbench settlement export', () => {
  it('maps fixed two payroll row types into export labels', () => {
    const rows = buildSettlementPayrollExportRows([
      {
        payee_user_id: 1001,
        payee_name: '张三',
        worker_type: 'parttime',
        business_month: '2026-06',
        row_type: 'normal_piecework',
        item_count: 2,
        page_count: 6,
        gross_amount: 120,
        error_count: 1,
        deduction_amount: 4,
        welfare_amount: 30,
        supplement_amount: 0,
        adjustment_amount: -6,
        net_amount: 140,
      },
      {
        payee_user_id: 1001,
        payee_name: '张三',
        worker_type: 'parttime',
        business_month: '2026-06',
        row_type: 'supplement_piecework',
        item_count: 0,
        page_count: 0,
        gross_amount: 0,
        error_count: 0,
        deduction_amount: 0,
        welfare_amount: 0,
        supplement_amount: 0,
        adjustment_amount: 0,
        net_amount: 0,
      },
    ])

    expect(rows).toEqual([
      expect.objectContaining({
        payeeName: '张三',
        workerTypeLabel: '兼职',
        rowTypeLabel: '日常计件工资',
        itemCount: 2,
        grossAmount: 120,
        deductionAmount: 4,
        welfareAmount: 30,
        adjustmentAmount: -6,
        netAmount: 140,
      }),
      expect.objectContaining({
        payeeName: '张三',
        rowTypeLabel: '补录计件工资',
        supplementAmount: 0,
        netAmount: 0,
      }),
    ])
    expect(rows[0]).not.toHaveProperty('pageCount')
  })

  it('keeps unknown row types on the normal payroll side', () => {
    expect(payrollRowLabel('legacy')).toBe('日常计件工资')
  })

  it('keeps the first and last upload dates in report exports', () => {
    const row = {
      row_type: 'normal_piecework',
      payee_user_id: 1001,
      business_month: '2026-06',
      creator_name: '张三',
      job_grade: 'P1',
      created_date: '2026-06-02',
      created_date_end: '2026-06-10',
      order_count: 2,
      item_count: 2,
      page_count: 5,
      gross_amount: 50,
      error_count: 0,
      deduction_amount: 0,
      welfare_amount: 0,
      supplement_amount: 0,
      net_amount: 50,
      error_rate: 0,
      page_count_share: 1,
      error_count_share: 0,
      month_amount_share: 1,
      difficulty_metrics: [],
    } as SettlementReportRow

    expect(buildSettlementReportExportRow(row, [])).toEqual(expect.objectContaining({
      created_date: '2026-06-02',
      created_date_end: '2026-06-10',
    }))
  })
})
