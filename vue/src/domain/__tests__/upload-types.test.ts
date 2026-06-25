import { describe, expect, it } from 'vitest'
import {
  canPreviewUploadInline,
  isAllowedUploadFile,
  isBitmapDeliveryFile,
} from '../constants/upload-types'

describe('upload-types', () => {
  it('accepts newly added vector/source extensions', () => {
    expect(isAllowedUploadFile('logo.ai')).toBe(true)
    expect(isAllowedUploadFile('blueprint.CDR')).toBe(true)
    expect(isAllowedUploadFile('cutter.PLT')).toBe(true)
  })

  it('accepts delivery archive packages', () => {
    expect(isAllowedUploadFile('final-package.zip')).toBe(true)
    expect(isAllowedUploadFile('source-bundle.rar')).toBe(true)
    expect(isAllowedUploadFile('print-files.7z')).toBe(true)
  })

  it('rejects unsupported extensions', () => {
    expect(isAllowedUploadFile('movie.mp4')).toBe(false)
    expect(isAllowedUploadFile('README')).toBe(false)
  })

  it('marks inline previewability and bitmap delivery correctly', () => {
    expect(canPreviewUploadInline('preview.png')).toBe(true)
    expect(canPreviewUploadInline('source.psd')).toBe(false)
    expect(isBitmapDeliveryFile('final.webp')).toBe(true)
    expect(isBitmapDeliveryFile('vector.svg')).toBe(false)
  })
})
