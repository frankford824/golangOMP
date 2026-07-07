import { describe, expect, it } from 'vitest'

import {
  createDriveUploadQueue,
  driveUploadRelativePath,
  filesFromDriveDrop,
  isSafeDriveUploadPath,
  withDriveUploadRelativePath,
} from './useDriveUpload'

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

  it('expands dragged directory entries and preserves file paths', async () => {
    const file = new File(['image'], 'cover.jpg', { type: 'image/jpeg' })
    let readCount = 0
    const dataTransfer = {
      items: [
        {
          webkitGetAsEntry: () => ({
            name: 'Folder',
            isFile: false,
            isDirectory: true,
            createReader: () => ({
              readEntries: (success: (entries: unknown[]) => void) => {
                readCount += 1
                success(readCount === 1
                  ? [{
                      name: 'cover.jpg',
                      isFile: true,
                      isDirectory: false,
                      file: (successFile: (value: File) => void) => successFile(file),
                    }]
                  : [])
              },
            }),
          }),
        },
      ],
    } as unknown as DataTransfer

    const [result] = await filesFromDriveDrop(dataTransfer)

    expect(driveUploadRelativePath(result)).toBe('Folder/cover.jpg')
  })

  it('rejects unsafe relative paths', () => {
    expect(isSafeDriveUploadPath('Folder/cover.jpg')).toBe(true)
    expect(isSafeDriveUploadPath('../cover.jpg')).toBe(false)
    expect(isSafeDriveUploadPath('Folder/.DS_Store')).toBe(false)
    expect(isSafeDriveUploadPath('Folder/@eaDir/cover.jpg')).toBe(false)
  })
})
