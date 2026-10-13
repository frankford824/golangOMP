import type { DriveFileRow, DriveFolderBrowseResult, SubmissionDetail, SubmissionFileRow } from '@aw/shared/api/assetWorkbenchApi'

export const workSelectionLimit = 5000
export type WorkSelectionFile = Pick<SubmissionFileRow, 'id' | 'submission_id' | 'submission_item_id'>
export interface WholeWorkSelection {
  files: SubmissionFileRow[]
  workCount: number
}
export interface PendingWorkDeletion {
  fileIds: number[]
  workCount: number
  extraCount: number
  reason: string
}

function checkLimit(count: number) {
  if (count > workSelectionLimit) throw new Error(`一次最多选择 ${workSelectionLimit} 个文件，请缩小筛选范围后重试。`)
}

/** Read every page before replacing selection; never return a partial selection. */
export async function collectFilteredFiles(fetchPage: (page: number) => Promise<{ items: DriveFileRow[]; total: number }>) {
  const files = new Map<number, DriveFileRow>()
  let expectedTotal = -1
  for (let page = 1; ; page += 1) {
    const result = await fetchPage(page)
    checkLimit(result.total)
    if (expectedTotal >= 0 && result.total !== expectedTotal) throw new Error('上传记录已变化，请刷新后重新选择。')
    expectedTotal = result.total
    const previousSize = files.size
    for (const file of result.items) files.set(file.id, file)
    checkLimit(files.size)
    if (files.size === expectedTotal) return [...files.values()]
    if (files.size <= previousSize || files.size > expectedTotal || page > workSelectionLimit) throw new Error('没有读取到全部文件，未更改选择，请刷新后重试。')
  }
}

/** Existing folder API is immediate-child only, so descend and paginate explicitly. */
export async function collectFolderFiles(path: string, fetchPage: (path: string, page: number) => Promise<DriveFolderBrowseResult>) {
  const files = new Map<number, DriveFileRow>()
  const paths = [path]
  const queued = new Set(paths)
  for (let index = 0; index < paths.length; index += 1) {
    const currentPath = paths[index]!
    let expectedTotal = -1
    const directIds = new Set<number>()
    for (let page = 1; ; page += 1) {
      const result = await fetchPage(currentPath, page)
      if (result.truncated) throw new Error('文件夹索引尚不完整，未选择部分文件；请缩小范围或联系管理员。')
      if (result.path !== currentPath || (expectedTotal >= 0 && result.total !== expectedTotal)) throw new Error('文件夹内容已变化，请刷新后重新选择。')
      expectedTotal = result.total
      checkLimit(expectedTotal)
      const previousSize = directIds.size
      for (const file of result.files) { files.set(file.id, file); directIds.add(file.id) }
      checkLimit(files.size)
      for (const folder of result.folders) {
        if (!folder.path || folder.path === currentPath || (currentPath && !folder.path.startsWith(`${currentPath}/`))) throw new Error('文件夹层级不正确，未更改选择。')
        if (!queued.has(folder.path)) { queued.add(folder.path); paths.push(folder.path); checkLimit(paths.length) }
      }
      if (directIds.size === expectedTotal) break
      if (directIds.size <= previousSize || directIds.size > expectedTotal || page > workSelectionLimit) throw new Error('没有读取到文件夹内全部文件，未更改选择。')
    }
  }
  return [...files.values()]
}

/** The priced work's item ID, not a filename or folder name, is the mutation boundary. */
export async function resolveWholeWorks(seeds: WorkSelectionFile[], fetchDetail: (id: number) => Promise<SubmissionDetail>): Promise<WholeWorkSelection> {
  const bySubmission = new Map<number, Map<number, Set<number>>>()
  for (const file of seeds) {
    if (file.id <= 0 || file.submission_id <= 0 || file.submission_item_id <= 0) throw new Error('文件作品归属不完整，请刷新后重试。')
    const works = bySubmission.get(file.submission_id) ?? new Map<number, Set<number>>()
    const ids = works.get(file.submission_item_id) ?? new Set<number>()
    ids.add(file.id); works.set(file.submission_item_id, ids); bySubmission.set(file.submission_id, works)
  }
  const files = new Map<number, SubmissionFileRow>()
  let workCount = 0
  for (const [submissionId, works] of bySubmission) {
    const detail = await fetchDetail(submissionId)
    if (detail.submission.id !== submissionId) throw new Error('作品归属已变化，请刷新后重试。')
    for (const [itemId, seedIds] of works) {
      const work = detail.items.find((entry) => entry.item.id === itemId)
      if (!work?.files.length || [...seedIds].some((id) => !work.files.some((file) => file.id === id))) throw new Error('所选文件已变化，未提交删除，请刷新后重试。')
      for (const file of work.files) {
        if (file.submission_id !== submissionId || file.submission_item_id !== itemId) throw new Error('作品文件归属不一致，未提交删除。')
        files.set(file.id, file)
      }
      checkLimit(files.size)
      workCount += 1
    }
  }
  return { files: [...files.values()], workCount }
}
