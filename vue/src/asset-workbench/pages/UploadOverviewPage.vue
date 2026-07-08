<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowDownAZ, ArrowUpAZ, Download, Eye, FileArchive, FileDown, FolderOpen, Inbox, Pencil, RefreshCw, Save, Trash2, X } from 'lucide-vue-next'

import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import {
  assetWorkbenchApi,
  type ArchiveVirtualFile,
  type ArchiveVirtualFolder,
  type DriveDirectoryRow,
  type DriveFileRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import { formatMoney } from '@aw/shared/format/number'
import ArchiveVirtualThumb from '@aw/shared/drive/ArchiveVirtualThumb.vue'
import { createArchiveEntryObjectUrl, downloadArchiveEntryBlob } from '@aw/shared/drive/archiveEntryBlob'
import DriveThumb from '@aw/shared/drive/DriveThumb.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'

type SortBy = 'created_at' | 'owner' | 'directory' | 'name' | 'format'
type SortDir = 'asc' | 'desc'
interface PieceworkDisplayState {
  isPrimary: boolean
  siblingCount: number
}

const session = useAssetWorkbenchSessionStore()
const route = useRoute()
const capabilities = computed(() => new Set(session.bootstrap?.capabilities ?? []))
const canManageDrive = computed(() => capabilities.value.has('asset.workbench.manage'))

const pageSize = 50
const exportLimit = 5000
const skeletonRowCount = 8

const directories = ref<DriveDirectoryRow[]>([])
const files = ref<DriveFileRow[]>([])
const selectedFile = ref<DriveFileRow | null>(null)
const selectedIds = ref<Set<number>>(new Set())
const loading = ref(false)
const actionLoading = ref(false)
const exporting = ref(false)
const error = ref('')
const notice = ref('')
const page = ref(1)
const total = ref(0)
const query = ref('')
const owner = ref('')
const createdFrom = ref('')
const createdTo = ref('')
const directory = ref('all')
const sortBy = ref<SortBy>('created_at')
const sortDir = ref<SortDir>('desc')
const moveTargetDirectoryId = ref(0)
const actionReason = ref('')
const editingFileId = ref<number | null>(null)
const editingName = ref('')
const previewOpen = ref(false)
const previewUrl = ref('')
const previewEmptyLabel = ref('')
const previewTitle = ref('')
const previewMimeType = ref('')
const previewIdentity = ref('')
const previewRows = ref<Array<[string, string]>>([])
const previewDownloadHandler = ref<(() => void | Promise<void>) | null>(null)
const archiveOpen = ref(false)
const archiveSource = ref<DriveFileRow | null>(null)
const archivePath = ref('')
const archiveFolders = ref<ArchiveVirtualFolder[]>([])
const archiveFiles = ref<ArchiveVirtualFile[]>([])
const archiveLoading = ref(false)
const archiveError = ref('')

let requestSeq = 0
let listAbortController: AbortController | null = null
let archivePreviewObjectUrl = ''
let archivePreviewRequestSeq = 0

function revokeArchivePreviewObjectUrl() {
  archivePreviewRequestSeq += 1
  if (!archivePreviewObjectUrl) return
  URL.revokeObjectURL(archivePreviewObjectUrl)
  archivePreviewObjectUrl = ''
}

function closePreviewDialog() {
  previewOpen.value = false
  previewIdentity.value = ''
  previewUrl.value = ''
  previewDownloadHandler.value = null
  revokeArchivePreviewObjectUrl()
}

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const allPageSelected = computed(() => files.value.length > 0 && files.value.every((file) => selectedIds.value.has(file.id)))
const filteredDirectoryLabel = computed(() => {
  if (directory.value === 'all') return '全部分类'
  if (directory.value === 'unassigned') return '未分类'
  const item = directories.value.find((dir) => String(dir.directory_id || '') === directory.value)
  return item?.name || '指定分类'
})
const pieceworkRows = computed(() => uniqueSubmissionItemRows(files.value))
const pieceworkDisplayByFileID = computed(() => buildPieceworkDisplayByFileID(files.value))
const totalAmount = computed(() => pieceworkRows.value.reduce((sum, file) => sum + Number(file.gross_amount || 0), 0))
const totalCount = computed(() => pieceworkRows.value.reduce((sum, file) => sum + Number(file.page_count || 0), 0))
const activeFilterCount = computed(() =>
  [query.value.trim(), owner.value.trim(), createdFrom.value, createdTo.value, directory.value !== 'all' ? directory.value : ''].filter(Boolean).length,
)
const directoryOptions = computed(() => directories.value.filter((dir) => Number(dir.directory_id || 0) > 0))
const archiveBreadcrumbs = computed(() => {
  const crumbs = [{ label: archiveSource.value ? fileDisplayName(archiveSource.value) : '压缩包', path: '' }]
  const segments = archivePath.value.split('/').filter(Boolean)
  let current = ''
  for (const segment of segments) {
    current = current ? `${current}/${segment}` : segment
    crumbs.push({ label: segment, path: current })
  }
  return crumbs
})

function directoryParams() {
  if (directory.value === 'unassigned') return { unassigned: true }
  const id = Number(directory.value)
  if (id > 0) return { dir_id: id }
  return {}
}

function formatDateTime(value?: string) {
  return formatShanghaiDateTime(value)
}

function formatSize(size?: number): string {
  if (!size) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function fileDisplayName(file: DriveFileRow): string {
  return file.display_name || file.original_filename || `文件 ${file.id}`
}

function secondaryFilename(file: DriveFileRow): string {
  const original = file.original_filename || ''
  return original && original !== fileDisplayName(file) ? original : ''
}

function fileOwnerLabel(file: DriveFileRow): string {
  return file.owner_name || file.owner_username || (file.owner_user_id ? `用户 ${file.owner_user_id}` : '—')
}

function fileOwnerSecondary(file: DriveFileRow): string {
  const secondary = file.owner_username || (file.owner_user_id ? `ID ${file.owner_user_id}` : '')
  return secondary && secondary !== fileOwnerLabel(file) ? secondary : ''
}

function filenameExt(value?: string): string {
  const normalized = String(value || '').trim().replace(/\\/g, '/').split(/[?#]/)[0] || ''
  const basename = normalized.split('/').pop() || normalized
  const dot = basename.lastIndexOf('.')
  if (dot <= 0 || dot === basename.length - 1) return ''
  return basename.slice(dot + 1).trim().replace(/^\./, '')
}

function fileExtLabel(file: DriveFileRow): string {
  const extFromName = filenameExt(file.original_filename) || filenameExt(file.display_name)
  const rawType = String(file.file_type || '').trim().replace(/^\./, '').toLowerCase()
  const mime = String(file.mime_type || '').toLowerCase()
  const genericTypes = new Set(['image', 'archive', 'design', 'pdf', 'video', 'audio', 'office', 'document', 'file'])
  const ext = extFromName || (rawType && !genericTypes.has(rawType) ? rawType : '')
  if (ext) return ext.toUpperCase()
  if (mime.includes('zip')) return 'ZIP'
  if (mime.includes('rar')) return 'RAR'
  if (mime.includes('7z')) return '7Z'
  if (mime === 'image/jpeg') return 'JPG'
  if (mime === 'image/png') return 'PNG'
  if (mime === 'image/webp') return 'WEBP'
  if (mime === 'application/pdf') return 'PDF'
  if (rawType === 'archive') return '压缩包'
  if (rawType === 'image') return '图片'
  if (rawType === 'design') return '设计'
  if (rawType === 'video') return '视频'
  if (rawType === 'office' || rawType === 'document') return '文档'
  return '文件'
}

function fileFormatKind(file: DriveFileRow): string {
  const ext = fileExtLabel(file).toLowerCase()
  const rawType = String(file.file_type || '').toLowerCase()
  const mime = String(file.mime_type || '').toLowerCase()
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext) || rawType === 'archive' || mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return '压缩包'
  if (['jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp', 'svg', 'tif', 'tiff'].includes(ext) || rawType === 'image' || mime.startsWith('image/')) return '图片'
  if (['psd', 'ai', 'cdr', 'eps'].includes(ext) || rawType === 'design') return '设计源文件'
  if (ext === 'pdf' || rawType === 'pdf' || mime === 'application/pdf') return 'PDF'
  if (['mp4', 'webm', 'mov', 'm4v'].includes(ext) || rawType === 'video' || mime.startsWith('video/')) return '视频'
  if (['doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'].includes(ext) || rawType === 'office') return '办公文档'
  return '文件'
}

function fileFormatLabel(file: DriveFileRow): string {
  const extLabel = fileExtLabel(file)
  const kind = fileFormatKind(file)
  if (extLabel && extLabel !== kind) return `${extLabel} · ${kind}`
  if (file.mime_type) return `${kind} · ${file.mime_type}`
  return kind
}

function fileFormatSecondary(file: DriveFileRow): string {
  const primary = fileExtLabel(file)
  const kind = fileFormatKind(file)
  return primary && primary !== kind ? kind : ''
}

function archiveFormatOf(file: DriveFileRow | null | undefined): string {
  if (!file) return ''
  const ext = filenameExt(file.original_filename) || filenameExt(file.display_name)
  if (['zip', 'rar', '7z'].includes(ext.toLowerCase())) return ext.toLowerCase()
  return file.file_type === 'archive' ? 'archive' : ''
}

function isArchiveFile(file: DriveFileRow | null | undefined): boolean {
  return Boolean(archiveFormatOf(file))
}

function canOpenArchive(file: DriveFileRow | null | undefined): boolean {
  return ['zip', 'rar'].includes(archiveFormatOf(file))
}

function archiveUnsupportedLabel(file: DriveFileRow): string {
  const format = archiveFormatOf(file).toUpperCase()
  return `${format || '压缩包'} 暂不能在线打开，可先下载后查看。`
}

function statusText(value?: string) {
  const map: Record<string, string> = {
    pending: '待处理',
    unpriced: '待补价',
    priced: '已计件',
    passed: '已通过',
    needs_fix: '需修',
    ready: '可预览',
    failed: '预览失败',
    processing: '处理中',
    unsettled: '未结算',
    in_batch: '批次中',
    settled: '已结算',
  }
  const normalized = String(value || '').trim()
  return map[normalized] || normalized || '—'
}

function statusToneClass(value?: string) {
  const normalized = String(value || '').trim()
  if (['priced', 'passed', 'ready', 'settled'].includes(normalized)) return 'aw-chip--success'
  if (['pending', 'unpriced', 'processing'].includes(normalized)) return 'aw-chip--warn'
  if (['needs_fix', 'failed'].includes(normalized)) return 'aw-chip--danger'
  if (normalized === 'in_batch') return 'aw-chip--info'
  return 'aw-chip--neutral'
}

function filePreviewRows(file: DriveFileRow): Array<[string, string]> {
  const rows: Array<[string, string]> = [
    ['上传人', fileOwnerLabel(file)],
    ['上传时间', formatDateTime(file.created_at)],
    ['分类', file.upload_directory_name || '未分类'],
    ['作品名称', fileDisplayName(file)],
    ['原始文件名', file.original_filename || '—'],
    ['格式', fileFormatLabel(file)],
    ['数量', fileQuantityLabel(file)],
    ['计件金额', fileAmountLabel(file)],
    ['计件状态', statusText(file.pricing_status)],
    ['文件大小', formatSize(file.file_size)],
  ]
  const note = filePieceworkNote(file)
  if (note) rows.splice(8, 0, ['计价说明', note])
  return rows
}

function uniqueSubmissionItemRows(rows: DriveFileRow[]) {
  const seen = new Set<string>()
  return rows.filter((file) => {
    const key = file.submission_item_id ? `item:${file.submission_item_id}` : `file:${file.id}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function buildPieceworkDisplayByFileID(rows: DriveFileRow[]) {
  const groups = new Map<string, DriveFileRow[]>()
  for (const file of rows) {
    const key = file.submission_item_id ? `item:${file.submission_item_id}` : `file:${file.id}`
    const group = groups.get(key) ?? []
    group.push(file)
    groups.set(key, group)
  }
  const result = new Map<number, PieceworkDisplayState>()
  for (const group of groups.values()) {
    group.forEach((file, index) => {
      result.set(file.id, { isPrimary: index === 0, siblingCount: group.length })
    })
  }
  return result
}

function filePieceworkState(file: DriveFileRow, lookup = pieceworkDisplayByFileID.value): PieceworkDisplayState {
  return lookup.get(file.id) ?? { isPrimary: true, siblingCount: 1 }
}

function filePieceworkRepeated(file: DriveFileRow, lookup = pieceworkDisplayByFileID.value) {
  const state = filePieceworkState(file, lookup)
  return state.siblingCount > 1 && !state.isPrimary
}

function fileQuantityLabel(file: DriveFileRow, lookup = pieceworkDisplayByFileID.value) {
  return filePieceworkRepeated(file, lookup) ? '—' : file.page_count ? `${file.page_count}` : '—'
}

function fileAmountLabel(file: DriveFileRow, lookup = pieceworkDisplayByFileID.value) {
  return filePieceworkRepeated(file, lookup) ? '—' : formatMoney(file.gross_amount || 0)
}

function filePieceworkNote(file: DriveFileRow, lookup = pieceworkDisplayByFileID.value) {
  const state = filePieceworkState(file, lookup)
  if (state.siblingCount <= 1) return ''
  return state.isPrimary
    ? `同一作品包含 ${state.siblingCount} 个文件，数量和金额只统计一次。`
    : '该文件属于同一作品，数量和金额已计入同作品记录。'
}

function filePieceworkInlineNote(file: DriveFileRow) {
  const state = filePieceworkState(file)
  if (state.siblingCount <= 1) return ''
  return state.isPrimary ? `文件夹作品 · 共 ${state.siblingCount} 个文件 · 只计 1 件` : '同一文件夹作品 · 金额已计入同作品'
}

function csvEscape(value: unknown) {
  const text = String(value ?? '')
  if (/[",\n\r]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}

function fileToExportRow(file: DriveFileRow, lookup: Map<number, PieceworkDisplayState>) {
  return [
    fileOwnerLabel(file),
    formatDateTime(file.created_at),
    file.upload_directory_name || '未分类',
    fileDisplayName(file),
    file.original_filename || '',
    fileFormatLabel(file),
    fileQuantityLabel(file, lookup),
    fileAmountLabel(file, lookup),
    statusText(file.pricing_status),
    formatSize(file.file_size),
    filePieceworkNote(file, lookup),
  ]
}

async function loadDirectories() {
  directories.value = await assetWorkbenchApi.driveDirectories()
}

async function loadFiles(nextPage = page.value) {
  const requestID = ++requestSeq
  listAbortController?.abort()
  listAbortController = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.driveFiles(
      {
        ...directoryParams(),
        q: query.value.trim() || undefined,
        owner: owner.value.trim() || undefined,
        created_from: createdFrom.value || undefined,
        created_to: createdTo.value || undefined,
        sort_by: sortBy.value,
        sort_dir: sortDir.value,
        page: nextPage,
        page_size: pageSize,
      },
      listAbortController.signal,
    )
    if (requestID !== requestSeq) return
    files.value = result.items
    total.value = result.total
    page.value = nextPage
    if (selectedFile.value) {
      selectedFile.value = result.items.find((file) => file.id === selectedFile.value?.id) || null
    }
    selectedIds.value = new Set([...selectedIds.value].filter((id) => result.items.some((file) => file.id === id)))
  } catch (err) {
    if ((err as DOMException)?.name === 'AbortError') return
    files.value = []
    total.value = 0
    error.value = err instanceof Error ? err.message : '上传记录加载失败'
  } finally {
    if (requestID === requestSeq) loading.value = false
  }
}

async function applyFilters() {
  page.value = 1
  selectedIds.value = new Set()
  await loadFiles(1)
}

async function resetFilters() {
  query.value = ''
  owner.value = ''
  createdFrom.value = ''
  createdTo.value = ''
  directory.value = 'all'
  await applyFilters()
}

async function changePage(delta: number) {
  const next = page.value + delta
  if (next < 1 || next > totalPages.value) return
  await loadFiles(next)
}

async function setSort(nextSort: SortBy) {
  if (sortBy.value === nextSort) sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    sortBy.value = nextSort
    sortDir.value = nextSort === 'created_at' ? 'desc' : 'asc'
  }
  await applyFilters()
}

function sortIcon(field: SortBy) {
  if (sortBy.value !== field) return null
  return sortDir.value === 'asc' ? ArrowUpAZ : ArrowDownAZ
}

function selectFile(file: DriveFileRow) {
  selectedFile.value = selectedFile.value?.id === file.id ? null : file
}

function closeDetail() {
  selectedFile.value = null
}

function toggleFile(file: DriveFileRow, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked) next.add(file.id)
  else next.delete(file.id)
  selectedIds.value = next
  selectedFile.value = file
}

function togglePageSelection(checked: boolean) {
  selectedIds.value = checked ? new Set(files.value.map((file) => file.id)) : new Set()
}

function clearSelection() {
  selectedIds.value = new Set()
}

function startEditName(file: DriveFileRow) {
  editingFileId.value = file.id
  editingName.value = fileDisplayName(file)
  selectedFile.value = file
}

function cancelEditName() {
  editingFileId.value = null
  editingName.value = ''
}

async function saveDisplayName(file: DriveFileRow) {
  const nextName = editingName.value.trim()
  if (!nextName) {
    error.value = '作品名称不能为空'
    return
  }
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const updated = await assetWorkbenchApi.updateSubmissionFile(file.id, { display_name: nextName })
    files.value = files.value.map((item) => (item.id === file.id ? { ...item, display_name: updated.display_name || nextName } : item))
    if (selectedFile.value?.id === file.id) selectedFile.value = { ...selectedFile.value, display_name: updated.display_name || nextName }
    cancelEditName()
    notice.value = '作品名称已保存'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '作品名称保存失败'
  } finally {
    actionLoading.value = false
  }
}

async function openFilePreview(file: DriveFileRow) {
  revokeArchivePreviewObjectUrl()
  if (isArchiveFile(file)) {
    await openArchiveFile(file)
    return
  }
  selectedFile.value = file
  previewOpen.value = true
  previewIdentity.value = `file:${file.id}`
  previewTitle.value = fileDisplayName(file)
  previewMimeType.value = file.mime_type || ''
  previewRows.value = filePreviewRows(file)
  previewUrl.value = ''
  previewEmptyLabel.value = '正在加载预览…'
  previewDownloadHandler.value = () => {
    void downloadFile(file)
  }
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    previewUrl.value = meta.preview_url || ''
    previewEmptyLabel.value = meta.preview_url ? '' : '暂无可展示预览，可下载原文件查看'
  } catch (err) {
    previewEmptyLabel.value = err instanceof Error ? err.message : '预览加载失败'
  }
}

async function openArchiveFile(file: DriveFileRow, path = '') {
  selectedFile.value = file
  archiveSource.value = file
  archiveOpen.value = true
  archivePath.value = path
  archiveFolders.value = []
  archiveFiles.value = []
  archiveError.value = ''
  if (!canOpenArchive(file)) {
    archiveError.value = archiveUnsupportedLabel(file)
    return
  }
  archiveLoading.value = true
  try {
    const result = await assetWorkbenchApi.browseArchiveFile(file.id, path)
    archivePath.value = result.path || ''
    archiveFolders.value = result.folders || []
    archiveFiles.value = result.files || []
  } catch (err) {
    archiveError.value = err instanceof Error ? err.message : '压缩包打开失败'
  } finally {
    archiveLoading.value = false
  }
}

function closeArchiveView() {
  archiveOpen.value = false
  archiveSource.value = null
  archivePath.value = ''
  archiveFolders.value = []
  archiveFiles.value = []
  archiveError.value = ''
  archiveLoading.value = false
}

function openArchiveFolder(path: string) {
  const source = archiveSource.value
  if (!source) return
  void openArchiveFile(source, path)
}

function canPreviewArchiveVirtualFile(file: ArchiveVirtualFile) {
  if (!file.preview_url) return false
  const mime = (file.mime_type || '').toLowerCase()
  if (mime.startsWith('image/') || mime.startsWith('video/') || mime === 'application/pdf') return true
  const ext = filenameExt(file.name).toLowerCase()
  return ['jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp', 'svg', 'tif', 'tiff', 'pdf', 'mp4', 'webm', 'mov', 'm4v'].includes(ext)
}

function archiveVirtualFileKind(file: ArchiveVirtualFile): string {
  return fileFormatKind({
    file_type: file.file_type,
    mime_type: file.mime_type,
    original_filename: file.name,
    display_name: file.name,
  } as DriveFileRow)
}

async function openArchiveVirtualFile(file: ArchiveVirtualFile) {
  revokeArchivePreviewObjectUrl()
  const canPreview = canPreviewArchiveVirtualFile(file)
  const source = archiveSource.value
  const sourceID = source?.id || 0
  const entryPath = file.path
  const previewKey = `archive:${sourceID}:${entryPath}`
  previewOpen.value = true
  previewIdentity.value = previewKey
  previewTitle.value = file.name
  previewMimeType.value = file.mime_type || ''
  previewRows.value = [
    ['所在位置', source ? fileDisplayName(source) : '压缩包'],
    ['内部路径', entryPath],
    ['格式', file.file_type || file.mime_type || '文件'],
    ['文件大小', formatSize(file.file_size)],
  ]
  previewUrl.value = ''
  previewEmptyLabel.value = canPreview ? '正在加载预览…' : '该格式暂不能在线预览，可下载后查看'
  previewDownloadHandler.value = () => {
    if (source) return downloadArchiveEntryBlob(source.id, file)
    return undefined
  }
  if (!canPreview) return
  if (!source) {
    previewEmptyLabel.value = '压缩包来源已关闭，请重新打开'
    return
  }
  try {
    const seq = ++archivePreviewRequestSeq
    const objectUrl = await createArchiveEntryObjectUrl(source.id, file)
    if (seq !== archivePreviewRequestSeq || !previewOpen.value || previewIdentity.value !== previewKey) {
      URL.revokeObjectURL(objectUrl)
      return
    }
    archivePreviewObjectUrl = objectUrl
    previewUrl.value = objectUrl
    previewEmptyLabel.value = ''
  } catch (err) {
    previewEmptyLabel.value = err instanceof Error ? err.message : '预览加载失败'
  }
}

function handlePreviewDownload() {
  const handler = previewDownloadHandler.value
  if (handler) {
    handler()
    return
  }
  if (selectedFile.value) void downloadFile(selectedFile.value)
}

async function downloadFile(file: DriveFileRow) {
  actionLoading.value = true
  error.value = ''
  try {
    const meta = await assetWorkbenchApi.getFileDownload(file.id)
    if (meta.download_url) window.open(meta.download_url, '_blank', 'noopener,noreferrer')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下载链接生成失败'
  } finally {
    actionLoading.value = false
  }
}

async function downloadSelectedFiles() {
  const ids = [...selectedIds.value]
  if (!ids.length) return
  actionLoading.value = true
  error.value = ''
  notice.value = '正在准备打包下载'
  try {
    const manifest = await assetWorkbenchApi.batchDownloadFiles(ids)
    await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.file_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `file-${item.file_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `file_id=${failure.file_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-upload-overview'),
    })
    const failureCount = manifest.failures?.length || 0
    notice.value = failureCount ? `已下载 ${manifest.items.length} 个，${failureCount} 个失败` : `已下载 ${manifest.items.length} 个文件`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量下载失败'
  } finally {
    actionLoading.value = false
  }
}

async function moveSelectedFiles() {
  const ids = [...selectedIds.value]
  if (!ids.length || !moveTargetDirectoryId.value || !canManageDrive.value) return
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.batchMoveFiles(ids, moveTargetDirectoryId.value, actionReason.value || '上传总览移动文件')
    notice.value = result.failures?.length ? `已移动 ${result.files?.length || 0} 个，${result.failures.length} 个失败` : `已移动 ${result.files?.length || 0} 个文件`
    selectedIds.value = new Set()
    await loadFiles()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '移动失败'
  } finally {
    actionLoading.value = false
  }
}

async function deleteSelectedFiles() {
  const ids = [...selectedIds.value]
  const reason = actionReason.value.trim()
  if (!ids.length || !reason || !canManageDrive.value) return
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteFiles(ids, reason)
    notice.value = result.failures?.length ? `已删除 ${result.deleted?.length || 0} 个，${result.failures.length} 个失败` : `已删除 ${result.deleted?.length || 0} 个文件`
    selectedIds.value = new Set()
    await loadFiles()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除失败'
  } finally {
    actionLoading.value = false
  }
}

async function exportCurrentFilter() {
  exporting.value = true
  error.value = ''
  notice.value = ''
  try {
    const rows: DriveFileRow[] = []
    let nextPage = 1
    for (;;) {
      const result = await assetWorkbenchApi.driveFiles({
        ...directoryParams(),
        q: query.value.trim() || undefined,
        owner: owner.value.trim() || undefined,
        created_from: createdFrom.value || undefined,
        created_to: createdTo.value || undefined,
        sort_by: sortBy.value,
        sort_dir: sortDir.value,
        page: nextPage,
        page_size: 200,
      })
      rows.push(...result.items)
      if (rows.length >= result.total || rows.length >= exportLimit || result.items.length === 0) break
      nextPage += 1
    }
    const header = ['创建人', '创建日期', '分类', '作品名称', '原始文件名', '格式', '数量', '计件金额', '状态', '文件大小', '计价说明']
    const exportPieceworkLookup = buildPieceworkDisplayByFileID(rows)
    const csv = [header, ...rows.map((file) => fileToExportRow(file, exportPieceworkLookup))].map((row) => row.map(csvEscape).join(',')).join('\n')
    const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `上传总览-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    notice.value = rows.length >= exportLimit && rows.length < total.value ? `已导出前 ${exportLimit} 条，请缩小筛选范围导出完整结果` : `已导出 ${rows.length} 条记录`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导出失败'
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  if (typeof route.query.q === 'string') query.value = route.query.q
  await Promise.all([loadDirectories(), loadFiles(1)])
})

onBeforeUnmount(() => {
  listAbortController?.abort()
  revokeArchivePreviewObjectUrl()
})
</script>

<template>
  <section class="aw-upload-ledger aw-token-scope">
    <header class="aw-upload-ledger__hero">
      <div class="aw-upload-ledger__hero-copy">
        <p class="aw-eyebrow">上传总览</p>
        <h2>全站上传台账</h2>
        <span>按创建人、创建日期、分类、作品名称和格式跟踪已上传作品。</span>
      </div>
      <dl class="aw-upload-ledger__summary" aria-label="当前筛选汇总">
        <div>
          <dt>匹配文件</dt>
          <dd>{{ total }}</dd>
        </div>
        <div>
          <dt>本页计价数量</dt>
          <dd>{{ totalCount }}</dd>
        </div>
        <div>
          <dt>本页计价金额</dt>
          <dd>{{ formatMoney(totalAmount) }}</dd>
        </div>
      </dl>
    </header>

    <section class="aw-upload-ledger__toolbar" aria-label="上传记录筛选">
      <form class="aw-upload-ledger__filters" @submit.prevent="applyFilters">
        <label>
          <span>关键词</span>
          <input v-model.trim="query" type="search" placeholder="作品名称 / 文件名 / 格式" />
        </label>
        <label>
          <span>创建人</span>
          <input v-model.trim="owner" placeholder="姓名或账号" />
        </label>
        <label>
          <span>分类</span>
          <select v-model="directory">
            <option value="all">全部分类</option>
            <option value="unassigned">未分类</option>
            <option v-for="dir in directoryOptions" :key="dir.directory_id ?? dir.name" :value="String(dir.directory_id)">
              {{ dir.name }}
            </option>
          </select>
        </label>
        <label>
          <span>开始日期</span>
          <input v-model="createdFrom" type="date" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model="createdTo" type="date" />
        </label>
        <div class="aw-upload-ledger__filter-actions">
          <button class="aw-primary-button" type="submit" :disabled="loading">查询</button>
          <button v-if="activeFilterCount" class="aw-grid-button" type="button" @click="resetFilters">重置 {{ activeFilterCount }}</button>
        </div>
      </form>
      <div class="aw-upload-ledger__tools">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadFiles()">
          <RefreshCw :size="15" aria-hidden="true" />
          刷新
        </button>
        <button class="aw-secondary-button" type="button" :disabled="exporting || loading" @click="exportCurrentFilter">
          <FileDown :size="15" aria-hidden="true" />
          {{ exporting ? '导出中' : '导出' }}
        </button>
      </div>
    </section>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error">{{ error }}</p>

    <section v-if="selectedIds.size" class="aw-upload-ledger__batch" aria-label="批量操作">
      <strong>已选择 {{ selectedIds.size }} 个作品</strong>
      <button class="aw-secondary-button" type="button" :disabled="actionLoading" @click="downloadSelectedFiles">
        <Download :size="15" aria-hidden="true" />
        下载
      </button>
      <template v-if="canManageDrive">
        <select v-model.number="moveTargetDirectoryId" aria-label="移动目标分类">
          <option :value="0">移动到分类…</option>
          <option v-for="dir in directoryOptions" :key="dir.directory_id ?? dir.name" :value="Number(dir.directory_id)">{{ dir.name }}</option>
        </select>
        <button class="aw-secondary-button" type="button" :disabled="actionLoading || !moveTargetDirectoryId" @click="moveSelectedFiles">移动</button>
        <input v-model.trim="actionReason" placeholder="删除原因或移动备注" aria-label="操作原因" />
        <button class="aw-secondary-button aw-secondary-button--danger" type="button" :disabled="actionLoading || !actionReason.trim()" @click="deleteSelectedFiles">
          <Trash2 :size="15" aria-hidden="true" />
          删除
        </button>
      </template>
      <button class="aw-grid-button" type="button" @click="clearSelection">取消选择</button>
    </section>

    <section class="aw-upload-ledger__content" :class="{ 'has-detail': Boolean(selectedFile) }">
      <div class="aw-upload-ledger__table-wrap">
        <table class="aw-upload-ledger__table">
          <colgroup>
            <col class="aw-upload-ledger__col-check" />
            <col class="aw-upload-ledger__col-thumb" />
            <col class="aw-upload-ledger__col-owner" />
            <col class="aw-upload-ledger__col-date" />
            <col class="aw-upload-ledger__col-dir" />
            <col class="aw-upload-ledger__col-name" />
            <col class="aw-upload-ledger__col-format" />
            <col class="aw-upload-ledger__col-count" />
            <col class="aw-upload-ledger__col-amount" />
            <col class="aw-upload-ledger__col-status" />
            <col class="aw-upload-ledger__col-size" />
            <col class="aw-upload-ledger__col-actions" />
          </colgroup>
          <thead>
            <tr>
              <th class="aw-upload-ledger__check">
                <input type="checkbox" :checked="allPageSelected" aria-label="选择当前页" @change="togglePageSelection(($event.target as HTMLInputElement).checked)" />
              </th>
              <th>预览</th>
              <th>
                <button type="button" @click="setSort('owner')">
                  创建人
                  <component :is="sortIcon('owner')" v-if="sortIcon('owner')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('created_at')">
                  创建时间
                  <component :is="sortIcon('created_at')" v-if="sortIcon('created_at')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('directory')">
                  分类
                  <component :is="sortIcon('directory')" v-if="sortIcon('directory')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('name')">
                  作品名称
                  <component :is="sortIcon('name')" v-if="sortIcon('name')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('format')">
                  格式
                  <component :is="sortIcon('format')" v-if="sortIcon('format')" :size="13" />
                </button>
              </th>
              <th class="aw-upload-ledger__num">数量</th>
              <th class="aw-upload-ledger__num">金额</th>
              <th>状态</th>
              <th class="aw-upload-ledger__num">大小</th>
              <th class="aw-upload-ledger__center">操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading">
              <tr v-for="i in skeletonRowCount" :key="`skeleton-${i}`" class="aw-upload-ledger__row-skeleton" aria-hidden="true">
                <td class="aw-upload-ledger__check"><span class="aw-upload-ledger__skeleton aw-upload-ledger__skeleton--dot" /></td>
                <td><span class="aw-upload-ledger__skeleton aw-upload-ledger__skeleton--thumb" /></td>
                <td colspan="10"><span class="aw-upload-ledger__skeleton" :style="{ width: `${92 - (i % 4) * 14}%` }" /></td>
              </tr>
            </template>
            <tr v-else-if="files.length === 0">
              <td colspan="12">
                <div class="aw-upload-ledger__empty">
                  <Inbox :size="30" aria-hidden="true" />
                  <strong>没有匹配的上传记录</strong>
                  <p>试试调整关键词、创建人、分类或日期范围。</p>
                  <button v-if="activeFilterCount" class="aw-grid-button" type="button" @click="resetFilters">清除全部筛选</button>
                </div>
              </td>
            </tr>
            <template v-else>
              <tr
                v-for="file in files"
                :key="file.id"
                :class="{ 'is-selected': selectedFile?.id === file.id, 'is-piecework-child': filePieceworkRepeated(file) }"
                @click="selectFile(file)"
                @dblclick="openFilePreview(file)"
              >
                <td class="aw-upload-ledger__check">
                  <input
                    type="checkbox"
                    :checked="selectedIds.has(file.id)"
                    :aria-label="`选择 ${fileDisplayName(file)}`"
                    @click.stop
                    @change="toggleFile(file, ($event.target as HTMLInputElement).checked)"
                  />
                </td>
                <td>
                  <button class="aw-upload-ledger__thumb" type="button" :aria-label="`预览 ${fileDisplayName(file)}`" @click.stop="openFilePreview(file)">
                    <FileArchive v-if="canOpenArchive(file)" :size="22" aria-hidden="true" />
                    <DriveThumb v-else :file-id="file.id" :filename="fileDisplayName(file)" :mime-type="file.mime_type" :preview-status="file.preview_status" size="sm" />
                  </button>
                </td>
                <td class="aw-upload-ledger__owner">
                  <strong :title="fileOwnerLabel(file)">{{ fileOwnerLabel(file) }}</strong>
                  <small v-if="fileOwnerSecondary(file)" :title="fileOwnerSecondary(file)">{{ fileOwnerSecondary(file) }}</small>
                </td>
                <td class="aw-upload-ledger__date">{{ formatDateTime(file.created_at) }}</td>
                <td>
                  <span class="aw-chip aw-chip--neutral" :title="file.upload_directory_name || '未分类'">{{ file.upload_directory_name || '未分类' }}</span>
                </td>
                <td class="aw-upload-ledger__name">
                  <form v-if="editingFileId === file.id" @submit.prevent="saveDisplayName(file)" @click.stop>
                    <input v-model.trim="editingName" aria-label="作品名称" />
                    <button type="submit" :disabled="actionLoading" aria-label="保存作品名称">
                      <Save :size="14" aria-hidden="true" />
                    </button>
                    <button type="button" aria-label="取消编辑" @click="cancelEditName">
                      <X :size="14" aria-hidden="true" />
                    </button>
                  </form>
                  <button v-else type="button" @click.stop="startEditName(file)">
                    <span>
                      <strong :title="fileDisplayName(file)">{{ fileDisplayName(file) }}</strong>
                      <small v-if="secondaryFilename(file)" :title="secondaryFilename(file)">{{ secondaryFilename(file) }}</small>
                      <small v-if="filePieceworkInlineNote(file)" class="aw-upload-ledger__piecework-note" :title="filePieceworkNote(file)">
                        {{ filePieceworkInlineNote(file) }}
                      </small>
                    </span>
                    <Pencil :size="14" aria-hidden="true" />
                  </button>
                </td>
                <td class="aw-upload-ledger__format">
                  <strong :title="fileFormatLabel(file)">{{ fileExtLabel(file) }}</strong>
                  <small v-if="fileFormatSecondary(file)" :title="file.mime_type || fileFormatSecondary(file)">{{ fileFormatSecondary(file) }}</small>
                </td>
                <td class="aw-upload-ledger__num">
                  <strong>{{ fileQuantityLabel(file) }}</strong>
                </td>
                <td class="aw-upload-ledger__num">
                  <strong>{{ fileAmountLabel(file) }}</strong>
                </td>
                <td>
                  <span class="aw-chip" :class="statusToneClass(file.pricing_status)">{{ statusText(file.pricing_status) }}</span>
                </td>
                <td class="aw-upload-ledger__num">{{ formatSize(file.file_size) }}</td>
                <td>
                  <div class="aw-upload-ledger__row-actions">
                    <button type="button" aria-label="预览" @click.stop="openFilePreview(file)">
                      <Eye :size="15" aria-hidden="true" />
                    </button>
                    <button type="button" aria-label="下载" @click.stop="downloadFile(file)">
                      <Download :size="15" aria-hidden="true" />
                    </button>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <aside v-if="selectedFile" class="aw-upload-ledger__detail" aria-label="当前作品详情">
        <div class="aw-upload-ledger__detail-head">
          <h3 :title="fileDisplayName(selectedFile)">{{ fileDisplayName(selectedFile) }}</h3>
          <button type="button" class="aw-upload-ledger__detail-close" aria-label="关闭详情" @click="closeDetail">
            <X :size="15" aria-hidden="true" />
          </button>
        </div>
        <div class="aw-upload-ledger__detail-thumb">
          <button
            v-if="canOpenArchive(selectedFile)"
            class="aw-upload-ledger__archive-preview"
            type="button"
            @click="openArchiveFile(selectedFile)"
          >
            <FileArchive :size="42" aria-hidden="true" />
            <strong>压缩包</strong>
            <span>点击查看里面的文件</span>
          </button>
          <DriveThumb v-else :file-id="selectedFile.id" :filename="fileDisplayName(selectedFile)" :mime-type="selectedFile.mime_type" :preview-status="selectedFile.preview_status" />
        </div>
        <dl>
          <template v-for="[label, value] in filePreviewRows(selectedFile)" :key="label">
            <dt>{{ label }}</dt>
            <dd :title="value">{{ value }}</dd>
          </template>
        </dl>
        <div class="aw-upload-ledger__detail-actions">
          <button class="aw-primary-button" type="button" @click="openFilePreview(selectedFile)">{{ canOpenArchive(selectedFile) ? '查看内容' : '打开预览' }}</button>
          <button class="aw-secondary-button" type="button" :disabled="actionLoading" @click="downloadFile(selectedFile)">
            <Download :size="15" aria-hidden="true" />
            下载
          </button>
        </div>
      </aside>
    </section>

    <footer class="aw-upload-ledger__pager">
      <span>{{ filteredDirectoryLabel }} · 共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页</span>
      <div>
        <button class="aw-grid-button" type="button" :disabled="page <= 1 || loading" @click="changePage(-1)">上一页</button>
        <button class="aw-grid-button" type="button" :disabled="page >= totalPages || loading" @click="changePage(1)">下一页</button>
      </div>
    </footer>

    <WorkbenchPreviewDialog
      :key="previewIdentity || previewTitle"
      :open="previewOpen"
      :title="previewTitle"
      eyebrow="上传作品预览"
      :preview-url="previewUrl"
      :mime-type="previewMimeType"
      :empty-label="previewEmptyLabel"
      :meta-rows="previewRows"
      @close="closePreviewDialog"
      @download="handlePreviewDownload"
    />

    <section
      v-if="archiveOpen"
      class="aw-token-scope aw-upload-ledger-archive"
      role="dialog"
      :aria-modal="previewOpen ? 'false' : 'true'"
      aria-label="压缩包内容"
    >
      <div class="aw-upload-ledger-archive__backdrop" @click="closeArchiveView" />
      <article class="aw-upload-ledger-archive__panel">
        <header class="aw-upload-ledger-archive__head">
          <div>
            <p class="aw-eyebrow">压缩包内容</p>
            <h3 :title="archiveSource ? fileDisplayName(archiveSource) : '压缩包'">{{ archiveSource ? fileDisplayName(archiveSource) : '压缩包' }}</h3>
          </div>
          <button type="button" class="aw-upload-ledger__detail-close" aria-label="关闭压缩包内容" @click="closeArchiveView">
            <X :size="15" aria-hidden="true" />
          </button>
        </header>
        <nav class="aw-upload-ledger-archive__breadcrumbs" aria-label="压缩包路径">
          <button
            v-for="(crumb, index) in archiveBreadcrumbs"
            :key="`${crumb.path || '__root__'}:${index}`"
            type="button"
            :class="{ 'is-active': index === archiveBreadcrumbs.length - 1 }"
            @click="openArchiveFolder(crumb.path)"
          >
            {{ crumb.label }}
          </button>
        </nav>
        <p v-if="archiveLoading" class="aw-drive-empty">正在读取压缩包…</p>
        <div v-else-if="archiveError" class="aw-upload-ledger-archive__empty">
          <FileArchive :size="34" aria-hidden="true" />
          <strong>{{ archiveError }}</strong>
          <button v-if="archiveSource" class="aw-secondary-button" type="button" @click="downloadFile(archiveSource)">下载压缩包</button>
        </div>
        <template v-else>
          <div class="aw-drive-archive-head">
            <div>
              <strong>{{ archivePath || '根目录' }}</strong>
              <span>{{ archiveFolders.length }} 个文件夹 · {{ archiveFiles.length }} 个文件</span>
            </div>
            <button v-if="archiveSource" class="aw-grid-button" type="button" @click="downloadFile(archiveSource)">下载压缩包</button>
          </div>
          <div v-if="archiveFolders.length" class="aw-upload-ledger-archive__grid">
            <button
              v-for="folder in archiveFolders"
              :key="folder.path"
              class="aw-upload-ledger-archive__folder"
              type="button"
              @click="openArchiveFolder(folder.path)"
            >
              <FolderOpen :size="34" aria-hidden="true" />
              <strong :title="folder.name">{{ folder.name }}</strong>
              <small>{{ folder.file_count }} 个文件</small>
            </button>
          </div>
          <div v-if="archiveFiles.length" class="aw-upload-ledger-archive__grid">
            <button
              v-for="file in archiveFiles"
              :key="file.path"
              class="aw-upload-ledger-archive__file"
              type="button"
              @click="openArchiveVirtualFile(file)"
            >
              <span>
                <ArchiveVirtualThumb v-if="archiveSource" :file-id="archiveSource.id" :file="file" />
                <FileArchive v-else :size="26" aria-hidden="true" />
              </span>
              <strong :title="file.name">{{ file.name }}</strong>
              <small>{{ archiveVirtualFileKind(file) }} · {{ formatSize(file.file_size) }}</small>
            </button>
          </div>
          <p v-if="archiveFolders.length === 0 && archiveFiles.length === 0" class="aw-drive-empty">压缩包内没有可显示的文件</p>
        </template>
      </article>
    </section>
  </section>
</template>
