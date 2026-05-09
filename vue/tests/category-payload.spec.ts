import { describe, expect, it } from 'vitest'
import { buildCategoryPatchFields } from '../src/domain/category-payload'

describe('buildCategoryPatchFields', () => {
  it('routes internal KT code to category_code', () => {
    expect(buildCategoryPatchFields('KT_STANDARD')).toEqual({ category_code: 'KT_STANDARD' })
  })

  it('routes internal OUT code to category_code', () => {
    expect(buildCategoryPatchFields('OUT_SILK_PRINT')).toEqual({ category_code: 'OUT_SILK_PRINT' })
  })

  it('routes display text to category', () => {
    expect(buildCategoryPatchFields('激光打印')).toEqual({ category: '激光打印' })
  })

  it('returns empty object for empty input', () => {
    expect(buildCategoryPatchFields('  ')).toEqual({})
    expect(buildCategoryPatchFields(null)).toEqual({})
  })
})
