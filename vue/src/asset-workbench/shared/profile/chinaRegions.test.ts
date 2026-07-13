import { describe, expect, it } from 'vitest'

import { cityOptions, provinceOptions } from './chinaRegions'

describe('china region options', () => {
  it('provides the expected province and linked city choices', () => {
    expect(provinceOptions()).toEqual(expect.arrayContaining(['江苏', '上海', '安徽']))
    expect(cityOptions('江苏')).toEqual(expect.arrayContaining(['南京', '苏州']))
    expect(cityOptions('安徽')).toContain('合肥')
    expect(cityOptions('上海')).toEqual(['上海市'])
  })

  it('preserves legacy values that are not in the standard option list', () => {
    expect(provinceOptions('海外')).toContain('海外')
    expect(cityOptions('海外', '新加坡')).toEqual(['新加坡'])
  })
})
