import { describe, expect, it, vi } from 'vitest'
import type { DriveFileRow, DriveFolderBrowseResult, SubmissionDetail } from '@aw/shared/api/assetWorkbenchApi'
import { collectFilteredFiles, collectFolderFiles, resolveWholeWorks, workSelectionLimit } from './workSelection'

const files = Array.from({ length: 53 }, (_, index) => ({ id: index + 1, submission_id: 20, submission_item_id: 30,
  relative_path: `image/nested/deeper/${index + 1}.jpg`, original_filename: `${index + 1}.jpg` } as DriveFileRow))
const detail = { submission: { id: 20 }, items: [{ item: { id: 30 }, files }] } as unknown as SubmissionDetail
const folder = (path: string, directFiles: DriveFileRow[], total = directFiles.length, folders: string[] = []): DriveFolderBrowseResult => ({
  path, folders: folders.map((path) => ({ path, name: path.split('/').at(-1)!, file_count: 53, direct_file_count: 0 })),
  files: directFiles, total, page: 1, size: 100, truncated: false,
})

describe('complete work selection', () => {
  it('reads all 53 files across a 50-file page boundary', async () => {
    const fetchPage = vi.fn(async (page: number) => ({ items: page === 1 ? files.slice(0, 50) : files.slice(50), total: 53 }))
    expect(await collectFilteredFiles(fetchPage)).toHaveLength(53)
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })

  it('fails closed if the filter total changes or a page repeats', async () => {
    await expect(collectFilteredFiles(async (page) => ({ items: files.slice(0, 50), total: page === 1 ? 53 : 54 }))).rejects.toThrow('记录已变化')
    await expect(collectFilteredFiles(async () => ({ items: files.slice(0, 50), total: 53 }))).rejects.toThrow('没有读取到全部文件')
  })

  it('does not return an incomplete selection when a page fails', async () => {
    await expect(collectFilteredFiles(async (page) => {
      if (page === 2) throw new Error('读取失败')
      return { items: files.slice(0, 50), total: 53 }
    })).rejects.toThrow('读取失败')
  })

  it('bounds full-filter selections before loading thousands of rows', async () => {
    await expect(collectFilteredFiles(async () => ({ items: [], total: workSelectionLimit + 1 }))).rejects.toThrow('缩小筛选范围')
  })

  it('descends through nested folders and reads every page', async () => {
    const fetchPage = vi.fn(async (path: string, page: number) => {
      if (path === 'image') return folder(path, [], 0, ['image/nested'])
      if (path === 'image/nested') return folder(path, [], 0, ['image/nested/deeper'])
      return folder(path, page === 1 ? files.slice(0, 50) : files.slice(50), 53)
    })
    const result = await collectFolderFiles('image', fetchPage)
    expect(result.map((file) => file.id)).toEqual(files.map((file) => file.id))
    expect(fetchPage).toHaveBeenCalledTimes(4)
  })

  it('rejects truncated folder indexes, invalid descendant paths and missing pages', async () => {
    await expect(collectFolderFiles('image', async () => ({ ...folder('image', files), truncated: true }))).rejects.toThrow('索引尚不完整')
    await expect(collectFolderFiles('image', async () => folder('image', [], 0, ['image-other']))).rejects.toThrow('层级不正确')
    await expect(collectFolderFiles('image', async () => folder('image', [], 53))).rejects.toThrow('没有读取到')
  })

  it('expands a 50-file partial choice to exactly the 53-file work, not a sibling work', async () => {
    const unrelatedFile = { ...files[0]!, id: 900, submission_item_id: 901 }
    const fetchDetail = vi.fn(async () => ({ ...detail, items: [...detail.items, { item: { id: 901 }, files: [unrelatedFile] }] } as SubmissionDetail))
    const result = await resolveWholeWorks(files.slice(0, 50), fetchDetail)
    expect(result.files).toHaveLength(53)
    expect(result.workCount).toBe(1)
    expect(result.files.some((file) => file.id === 900)).toBe(false)
    expect(fetchDetail).toHaveBeenCalledOnce()
  })

  it('resolves same-named folders by submission and item IDs, not the path', async () => {
    const second = { ...files[0]!, id: 500, submission_id: 21, submission_item_id: 31 }
    const fetchDetail = vi.fn(async (id: number) => id === 20 ? detail : ({ submission: { id: 21 }, items: [{ item: { id: 31 }, files: [second] }] } as unknown as SubmissionDetail))
    const result = await resolveWholeWorks([files[0]!, second], fetchDetail)
    expect(result.files).toHaveLength(54)
    expect(result.workCount).toBe(2)
  })

  it('rejects missing, unauthorized or mismatched work detail without a partial result', async () => {
    await expect(resolveWholeWorks([files[0]!], async () => ({ ...detail, items: [] }))).rejects.toThrow('所选文件已变化')
    await expect(resolveWholeWorks([files[0]!], async () => { throw new Error('无权限') })).rejects.toThrow('无权限')
    await expect(resolveWholeWorks([files[0]!], async () => ({ ...detail, submission: { ...detail.submission, id: 21 } }))).rejects.toThrow('归属已变化')
    await expect(resolveWholeWorks([files[0]!], async () => ({ ...detail, items: [{ ...detail.items[0]!, files: [{ ...files[0]!, submission_item_id: 999 }] }] } as SubmissionDetail))).rejects.toThrow('归属不一致')
  })
})
