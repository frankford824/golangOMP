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
  groupDriveUploadPieceworkItems,
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

  it('groups uploaded folder files as one piecework item', async () => {
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
        upload_session_ids: ['session:Folder/a.jpg', 'session:Folder/b.jpg'],
      },
    ])
  })

  it('does not retry failed files during a normal upload run', async () => {
    const files = [
      new File(['a'], 'new.jpg', { type: 'image/jpeg' }),
      new File(['b'], 'failed.jpg', { type: 'image/jpeg' }),
    ]
    const queue = createDriveUploadQueue(files)
    queue[1].status = 'failed'
    queue[1].error = '上次上传失败'
    uploadWorkbenchFileMock.mockImplementation(async (file: File) => ({ sessionId: `session:${file.name}` }))
    createSubmissionMock.mockResolvedValue({ items: [] })

    const count = await uploadDriveQueue(queue, { directoryId: 11, difficultyClass: 'A' })

    expect(count).toBe(1)
    expect(uploadWorkbenchFileMock).toHaveBeenCalledTimes(1)
    expect(uploadWorkbenchFileMock.mock.calls[0][0].name).toBe('new.jpg')
    expect(queue[1].status).toBe('failed')
  })

  it('retries failed files only when includeFailed is explicit', async () => {
    const queue = createDriveUploadQueue([new File(['a'], 'failed.jpg', { type: 'image/jpeg' })])
    queue[0].status = 'failed'
    queue[0].error = '上次上传失败'
    uploadWorkbenchFileMock.mockImplementation(async (file: File) => ({ sessionId: `session:${file.name}` }))
    createSubmissionMock.mockResolvedValue({ items: [] })

    const count = await uploadDriveQueue(queue, { directoryId: 11, difficultyClass: 'A', includeFailed: true })

    expect(count).toBe(1)
    expect(uploadWorkbenchFileMock).toHaveBeenCalledTimes(1)
    expect(queue[0].status).toBe('uploaded')
  })

  it('keeps loose files as separate piecework items', () => {
    const groups = groupDriveUploadPieceworkItems([
      { id: 'a', relativePath: 'a.jpg', sessionId: 'session:a.jpg' },
      { id: 'b', relativePath: 'b.zip', sessionId: 'session:b.zip' },
    ])

    expect(groups).toHaveLength(2)
    expect(groups.map((group) => group.items.map((item) => item.sessionId))).toEqual([
      ['session:a.jpg'],
      ['session:b.zip'],
    ])
    expect(groups.every((group) => !group.isFolder)).toBe(true)
  })
})
