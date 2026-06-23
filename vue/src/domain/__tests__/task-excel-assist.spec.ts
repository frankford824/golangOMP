import { describe, expect, it } from 'vitest'

import { mapExcelPreviewToBatchItems } from '@/domain/task-excel-assist'

describe('task-excel-assist', () => {
  it('keeps customization sku code type for batch rows', () => {
    const rows = [
      {
        source_row: 2,
        product_name: '定制批量 A',
        design_requirement: '定制要求 A',
        product_i_id: 'KT_STANDARD',
      },
      {
        source_row: 3,
        product_name: '定制批量 B',
        design_requirement: '定制要求 B',
        product_i_id: 'KT_STANDARD',
      },
    ]

    const items = mapExcelPreviewToBatchItems('new_product_development', rows, {
      skuCodeType: 'customization',
    })

    expect(items).toHaveLength(2)
    expect(items.every((item) => item.skuCodeType === 'customization')).toBe(true)
  })
})
