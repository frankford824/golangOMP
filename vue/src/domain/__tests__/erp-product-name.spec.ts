import { describe, expect, it } from 'vitest'

import {
  ERP_PRODUCT_NAME_MAX_LENGTH,
  erpProductNameError,
  erpProductNameHint,
  erpProductNameLength,
  isErpProductNameTooLong,
} from '@/domain/erp-product-name'

describe('erp-product-name', () => {
  it('counts visible product name length instead of UTF-8 bytes', () => {
    const exact = '名'.repeat(ERP_PRODUCT_NAME_MAX_LENGTH)
    const tooLong = exact + '字'

    expect(erpProductNameLength(exact)).toBe(40)
    expect(new TextEncoder().encode(exact).length).toBe(120)
    expect(isErpProductNameTooLong(exact)).toBe(false)
    expect(isErpProductNameTooLong(tooLong)).toBe(true)
  })

  it('shows business wording for remaining editable amount', () => {
    expect(erpProductNameHint('产品')).toContain('还可输入 38 个字')
    expect(erpProductNameHint('名'.repeat(41))).toContain('已超出 1 个字')
    expect(erpProductNameError('名'.repeat(41))).toContain('最多可填写 40 个字')
  })
})
