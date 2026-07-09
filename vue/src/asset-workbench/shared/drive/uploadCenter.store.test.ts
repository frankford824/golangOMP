// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { withDriveUploadRelativePath } from './useDriveUpload'
import { useUploadCenterStore } from './uploadCenter.store'

describe('asset workbench upload center store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps upload tasks available outside the upload page', () => {
    const store = useUploadCenterStore()
    const file = withDriveUploadRelativePath(new File(['image'], 'cover.jpg', { type: 'image/jpeg' }), 'A/cover.jpg')

    const [item] = store.addItems([file], {
      source: 'upload-page',
      uploadDirectoryId: 7,
      uploadDirectoryName: '海报目录',
      difficultyClass: 'A',
    })

    expect(store.hasItems).toBe(true)
    expect(store.panelOpen).toBe(true)
    expect(store.uploadPageItems).toHaveLength(1)
    expect(item.displayName).toBe('A/cover.jpg')

    store.updateItem(item.id, { status: 'uploading', progress: 45 })
    expect(store.hasActive).toBe(true)
    expect(store.overallProgress).toBe(45)

    store.updateItem(item.id, { status: 'uploaded', progress: 100 })
    expect(store.pendingRecordItems).toHaveLength(1)
    store.clearFinished()
    expect(store.uploadPageItems).toHaveLength(1)

    store.updateItem(item.id, { status: 'submitted', progress: 100 })
    expect(store.finishedItems).toHaveLength(1)
    expect(store.uploadPageItems).toHaveLength(0)
    expect(store.summaryText).toBe('已完成 1 个')
  })

  it('clears failed tasks without touching active uploads', () => {
    const store = useUploadCenterStore()
    const failed = store.addItems([new File(['a'], 'failed.jpg', { type: 'image/jpeg' })], { source: 'upload-page' })[0]
    const active = store.addItems([new File(['b'], 'active.jpg', { type: 'image/jpeg' })], { source: 'upload-page' })[0]

    store.updateItem(failed.id, { status: 'failed', error: '连接中断' })
    store.updateItem(active.id, { status: 'uploading', progress: 20 })
    store.clearFailed()

    expect(store.failedItems).toHaveLength(0)
    expect(store.items.map((item) => item.id)).toEqual([active.id])
  })
})
