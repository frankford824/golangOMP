import { describe, expect, it } from 'vitest'
import { sanitizeCustomizationPayload } from './customization-payload'

describe('sanitizeCustomizationPayload', () => {
  it('removes empty optional id fields so backend can use the session actor', () => {
    expect(
      sanitizeCustomizationPayload({
        reviewer_id: '',
        source_asset_id: '   ',
        customization_review_decision: 'return_to_designer',
      }),
    ).toEqual({
      customization_review_decision: 'return_to_designer',
    })
  })

  it('converts numeric strings to JSON numbers for Go int64 binding', () => {
    expect(
      sanitizeCustomizationPayload({
        reviewer_id: '303',
        current_asset_id: '42',
        operator_id: 297,
      }),
    ).toEqual({
      reviewer_id: 303,
      current_asset_id: 42,
      operator_id: 297,
    })
  })

  it('throws a user-facing error for non-empty invalid id values', () => {
    expect(() =>
      sanitizeCustomizationPayload({
        source_asset_id: 'asset-abc',
      }),
    ).toThrow('源文件资产信息格式不正确，请刷新页面后重试。')
  })
})
