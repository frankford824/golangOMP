import { describe, expect, it } from 'vitest'

import { matchesClientMaterialQuery } from './clientMaterialSearch'

describe('client material search', () => {
  it('matches a published material by task SKU fields', () => {
    const asset = {
      id: 14354,
      product_name: '真硕/定制海报/苹果迎宾牌挂布',
      original_filename: '真硕-定制海报.psd',
      sku_code: 'DZC000027',
      primary_sku_code: 'DZC000027',
    } as never

    expect(matchesClientMaterialQuery(asset, 'DZC000027')).toBe(true)
    expect(matchesClientMaterialQuery(asset, 'dzc000027')).toBe(true)
  })

  it('matches the name, resource code and path shown to users', () => {
    const asset = {
      id: 42,
      product_name: '夏季主视觉',
      resource_id: 'ext-42',
      origin_path: '/quark/海报/夏季主视觉.psd',
    } as never

    expect(matchesClientMaterialQuery(asset, '夏季')).toBe(true)
    expect(matchesClientMaterialQuery(asset, 'ext-42')).toBe(true)
    expect(matchesClientMaterialQuery(asset, '/quark/海报')).toBe(true)
    expect(matchesClientMaterialQuery(asset, '不存在')).toBe(false)
  })
})
