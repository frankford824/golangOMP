import { describe, expect, it } from 'vitest'

import { createDriveUploadQueue, withDriveUploadRelativePath } from './useDriveUpload'

describe('drive upload queue', () => {
  it('uses relative path captured from dragged folders', () => {
    const file = withDriveUploadRelativePath(new File(['image'], 'cover.jpg', { type: 'image/jpeg' }), 'Campaign/A/cover.jpg')

    const [item] = createDriveUploadQueue([file])

    expect(item.relativePath).toBe('Campaign/A/cover.jpg')
  })

  it('falls back to webkitRelativePath for folder picker files', () => {
    const file = new File(['image'], 'cover.jpg', { type: 'image/jpeg' }) as File & { webkitRelativePath?: string }
    Object.defineProperty(file, 'webkitRelativePath', {
      value: 'Picker/A/cover.jpg',
      configurable: true,
    })

    const [item] = createDriveUploadQueue([file])

    expect(item.relativePath).toBe('Picker/A/cover.jpg')
  })
})
