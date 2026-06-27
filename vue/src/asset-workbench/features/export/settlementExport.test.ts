import { describe, expect, it } from 'vitest'

import { buildSettlementPayrollExportRows, payrollRowLabel } from './settlementExport'

describe('asset workbench settlement export', () => {
  it('maps fixed two payroll row types into export labels', () => {
    const rows = buildSettlementPayrollExportRows([
      {
        payee_user_id: 1001,
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
        rowTypeLabel: '正常计件工资',
        grossAmount: 120,
        deductionAmount: 4,
        welfareAmount: 30,
        adjustmentAmount: -6,
        netAmount: 140,
      }),
      expect.objectContaining({
        rowTypeLabel: '补录计件工资',
        supplementAmount: 0,
        netAmount: 0,
      }),
    ])
  })

  it('keeps unknown row types on the normal payroll side', () => {
    expect(payrollRowLabel('legacy')).toBe('正常计件工资')
  })
})
