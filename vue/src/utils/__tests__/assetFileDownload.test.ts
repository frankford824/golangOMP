import { describe, expect, it } from 'vitest'
import {
  isPreviewDerivativeFilename,
  parseNumericAssetId,
  resolveAssetSaveFilename,
} from '@/utils/assetFileDownload'

describe('parseNumericAssetId', () => {
  it('accepts positive integer strings', () => {
    expect(parseNumericAssetId('601')).toBe('601')
    expect(parseNumericAssetId(501)).toBe('501')
  })

  it('rejects non-numeric ref ids', () => {
    expect(parseNumericAssetId('ref-1')).toBeUndefined()
    expect(parseNumericAssetId('')).toBeUndefined()
  })
})

describe('resolveAssetSaveFilename', () => {
  it('prefers operator filename over preview.webp from meta', () => {
    expect(resolveAssetSaveFilename('banner.psd', 'preview.webp')).toBe('banner.psd')
  })

  it('uses meta when preferred is empty', () => {
    expect(resolveAssetSaveFilename('', '运营上传图.jpg')).toBe('运营上传图.jpg')
  })

  it('sanitizes unsafe characters', () => {
    expect(resolveAssetSaveFilename('a/b:c?.png', '')).toBe('a_b_c_.png')
  })
})

describe('isPreviewDerivativeFilename', () => {
  it('detects preview derivative basenames', () => {
    expect(isPreviewDerivativeFilename('preview.webp')).toBe(true)
    expect(isPreviewDerivativeFilename('path/design-thumb.webp')).toBe(true)
    expect(isPreviewDerivativeFilename('真实素材.psd')).toBe(false)
  })
})
