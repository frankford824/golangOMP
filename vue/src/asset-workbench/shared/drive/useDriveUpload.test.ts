import { beforeEach, describe, expect, it, vi } from 'vitest'

const uploadWorkbenchFileMock = vi.hoisted(() => vi.fn())
const createSubmissionMock = vi.hoisted(() => vi.fn())

vi.mock('@aw/features/upload/uploadFlow', () => ({
  uploadWorkbenchFile: uploadWorkbenchFileMock,
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: {
    createSubmission: createSubmissionMock,
  },
}))

import {
  createDriveUploadQueue,
  driveUploadRelativePath,
  filesFromDriveDrop,
  isSafeDriveUploadPath,
  uploadDriveQueue,
  withDriveUploadRelativePath,
} from './useDriveUpload'

describe('drive upload queue', () => {
  beforeEach(() => {
    uploadWorkbenchFileMock.mockReset()
    createSubmissionMock.mockReset()
  })

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

  it('creates one piecework item per uploaded folder file', async () => {
    const files = [
      withDriveUploadRelativePath(new File(['a'], 'a.jpg', { type: 'image/jpeg' }), 'Folder/a.jpg'),
      withDriveUploadRelativePath(new File(['b'], 'b.jpg', { type: 'image/jpeg' }), 'Folder/b.jpg'),
    ]
    const queue = createDriveUploadQueue(files)
    uploadWorkbenchFileMock.mockImplementation(async (file: File, options: { onProgress?: (progress: { percent: number }) => void }) => {
      options.onProgress?.({ percent: 100 })
      return { sessionId: `session:${driveUploadRelativePath(file)}` }
    })
    createSubmissionMock.mockResolvedValue({ items: [] })

    const count = await uploadDriveQueue(queue, { directoryId: 11, difficultyClass: 'A' })

    expect(count).toBe(2)
    expect(createSubmissionMock).toHaveBeenCalledTimes(1)
    expect(createSubmissionMock.mock.calls[0][0].items).toEqual([
      {
        difficulty_class: 'A',
        finalized: true,
        page_count: 1,
        item_count: 1,
        upload_session_ids: ['session:Folder/a.jpg'],
      },
      {
        difficulty_class: 'A',
        finalized: true,
        page_count: 1,
        item_count: 1,
        upload_session_ids: ['session:Folder/b.jpg'],
      },
    ])
  })
})
