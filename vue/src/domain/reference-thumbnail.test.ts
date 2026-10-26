import { describe, expect, it } from 'vitest'
import { referenceThumbnailURL, withTaskReferencePreview } from './reference-thumbnail'

describe('server reference thumbnail selection', () => {
  it('uses the explicit thumbnail for task and SKU UUID references', () => {
    expect(referenceThumbnailURL({ asset_id: 'uuid', thumbnail_url: ' https://objects.example/thumb.webp ' })).toBe('https://objects.example/thumb.webp')
  })
  it('does not substitute an original download, label, or legacy URL', () => {
    expect(referenceThumbnailURL({ download_url: 'https://objects.example/original.jpg', preview_url: '/legacy.jpg' })).toBeUndefined()
    for (const input of [null, undefined, {}, { thumbnail_url: '' }, { thumbnail_url: 123 }]) expect(referenceThumbnailURL(input)).toBeUndefined()
  })
  it('preserves the controlled path for bound revision references', () => {
    expect(referenceThumbnailURL({ formal_task_asset_id: 201, thumbnail_url: 'https://objects.example/stale.webp' })).toBeUndefined()
  })

  it('renews legacy per-SKU locators from the same stable task reference without changing business fields', () => {
    const sku = { asset_id:'uuid',filename:'SKU-specific.png',download_url:'/expired-original',cost_price:23.5 }
    const root = { ref_id:'uuid',filename:'image.png',thumbnail_url:'/fresh-thumb',download_url:'/fresh-original' }
    const got = withTaskReferencePreview(sku,[root])
    expect(got).toMatchObject({filename:'SKU-specific.png',thumbnail_url:'/fresh-thumb',preview_url:'/fresh-original',download_url:'/fresh-original',cost_price:23.5})
    expect(sku.download_url).toBe('/expired-original')
  })

  it('never matches by filename, never guesses an ambiguous ID, and never replaces a controlled binding', () => {
    const root = {ref_id:'uuid',filename:'image.png',thumbnail_url:'/fresh',download_url:'/fresh-original'}
    const other = {asset_id:'other',filename:'image.png'}
    expect(withTaskReferencePreview(other,[root])).toBe(other)
    const sku = {asset_id:'uuid'}
    expect(withTaskReferencePreview(sku,[root,{...root,thumbnail_url:'/different'}])).toBe(sku)
    const bound = {asset_id:'uuid',formal_task_asset_id:201}
    expect(withTaskReferencePreview(bound,[root])).toBe(bound)
  })

  it('does not revive upload-time locators when the current task reference has no access URL', () => {
    expect(withTaskReferencePreview({asset_id:'uuid',download_url:'/expired',preview_url:'/expired',thumbnail_url:'/old-thumb'},[{ref_id:'uuid'}])).toMatchObject({download_url:undefined,preview_url:undefined,thumbnail_url:undefined})
  })
})
