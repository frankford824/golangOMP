<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Download,
  FileDown,
  FileArchive,
  HardDrive,
  ImageDown,
  Pencil,
  Table2,
  Upload,
} from 'lucide-vue-next'

import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import {
  assetWorkbenchApi,
  type AssetWorkbenchBatchJob,
  type ArchiveVirtualFile,
  type ArchiveVirtualFolder,
  type ClientMaterialRow,
  type DifficultyClassRow,
  type DriveDirectoryRow,
  type DriveFileRow,
  type DriveFolderRow,
  type MaterialBusinessLane,
  type MaterialFormatCategory,
  type MaterialFolderRow,
  type MaterialSourceFilter,
  type OverviewSearchRow,
  type SystemAssetRow,
  type SystemAssetPreviewMeta,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import ArchiveVirtualThumb from '@aw/shared/drive/ArchiveVirtualThumb.vue'
import { batchMutationFailureMessage } from '@aw/shared/drive/batchMutationFeedback'
import { createArchiveEntryObjectUrl, downloadArchiveEntryBlob } from '@aw/shared/drive/archiveEntryBlob'
import DriveThumb from '@aw/shared/drive/DriveThumb.vue'
import DriveUploadDialog from '@aw/shared/drive/DriveUploadDialog.vue'
import MaterialListThumb from '@aw/shared/materials/MaterialListThumb.vue'
import IconfontActionIcon from '@aw/shared/icons/IconfontActionIcon.vue'
import WorkbenchFolderIcon from '@aw/shared/icons/WorkbenchFolderIcon.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import { formatMoney } from '@aw/shared/format/number'
import { previewIsPreparing, waitForPreparedPreview } from '@aw/shared/download/preparedDownload'
import { useGlobalDownload } from '@aw/shared/download/useGlobalDownload'
import SpreadsheetWorkbench from '@aw/shared/spreadsheet/SpreadsheetWorkbench.vue'
import type {
  WorkbenchSpreadsheetActionPayload,
  WorkbenchSpreadsheetSource,
  WorkbenchSpreadsheetValidation,
} from '@aw/shared/spreadsheet/types'
import { canAttemptSystemAssetPreview, materialAssetKey, resolvedSystemAssetThumbnailUrl } from '@aw/shared/materials/systemAssetPreview'

type DriveMode = 'uploads' | 'directories' | 'operational'
type SearchScope = 'all' | 'operational' | 'files' | 'orders'
type ClientMaterialFilter = 'all' | 'enabled' | 'disabled'
type ContextMenuState =
  | { kind: 'directory'; x: number; y: number; dir: DriveDirectoryRow }
  | { kind: 'file'; x: number; y: number; file: DriveFileRow }
type ContextMenuInput =
  | { kind: 'directory'; dir: DriveDirectoryRow }
  | { kind: 'file'; file: DriveFileRow }
interface MaterialDirectoryNode {
  path: string
  name: string
  depth: number
  file_count: number
  direct_file_count: number
}
interface VisibleMaterialDirectoryNode extends MaterialDirectoryNode {
  has_children: boolean
  expanded: boolean
}
interface MaterialFolderEntry {
  path: string
  name: string
  file_count: number
  direct_file_count: number
  source_type?: string
}

const session = useAssetWorkbenchSessionStore()
const route = useRoute()
const { queueDriveFile, queueMaterial } = useGlobalDownload()

const UNASSIGNED_KEY = 'unassigned'
const pageSize = 60
const materialPageSize = 100
const searchDebounceMs = 250
const searchPreviewPrefetchLimit = 12
const QUARK_VISIBLE_ROOTS = new Set(['电视投屏', '海报', 'kt板', '闲置kt板'].map((item) => item.toLowerCase()))
const QUARK_VISIBLE_ACTUAL_BASE = '/quark/我的备份/来自：ASUS Administrator 电脑备份'

const activeMode = ref<DriveMode>('directories')
const driveSpreadsheetOpen = ref(false)
const capabilities = computed(() => new Set(session.bootstrap?.capabilities ?? []))
const canManageDrive = computed(() => capabilities.value.has('asset.workbench.manage'))
const canMaintainItems = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.settlement'))
const canListUploadDirectories = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.submit'))
const canUseOperational = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.material.download'))
const canDeleteDriveFiles = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.submit'))

interface SelectedDir {
  key: string
  id: number | null
  name: string
  unassigned: boolean
}

const directories = ref<DriveDirectoryRow[]>([])
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const difficultyClasses = ref<DifficultyClassRow[]>([])
const dirLoading = ref(false)
const dirError = ref('')
const selectedDir = ref<SelectedDir | null>(null)
const driveFolderPath = ref('')
const driveFolders = ref<DriveFolderRow[]>([])
const driveFolderTruncated = ref(false)
const files = ref<DriveFileRow[]>([])
const filesLoading = ref(false)
const filesError = ref('')
const fileTotal = ref(0)
const filePage = ref(1)
const selectedFile = ref<DriveFileRow | null>(null)
const selectedFileIds = ref<Set<number>>(new Set())
const fileMutationLoading = ref(false)
const highlightFileId = ref<number | null>(null)
const uploadOverviewQuery = ref('')
const uploadOverviewOwner = ref('')
const uploadOverviewFrom = ref('')
const uploadOverviewTo = ref('')
const uploadOverviewDirectory = ref('all')

const searchQuery = ref('')
const searchScope = ref<SearchScope>('all')
const searchActive = ref(false)
const searchLoading = ref(false)
const searchError = ref('')
const searchResults = ref<OverviewSearchRow[]>([])
const searchTotal = ref(0)

const materialQuery = ref('')
const materialSourceFilter = ref<MaterialSourceFilter>('all')
const materialFormatFilter = ref<MaterialFormatCategory>('all')
const materialBusinessLaneFilter = ref<MaterialBusinessLane>('all')
const materialLoading = ref(false)
const materialLoadingMore = ref(false)
const materialError = ref('')
const materialItems = ref<SystemAssetRow[]>([])
const materialKnownFolders = ref<Record<string, MaterialFolderEntry>>({})
const materialFileTotal = ref(0)
const materialPage = ref(1)
const clientMaterials = ref<ClientMaterialRow[]>([])
const clientMaterialManagerOpen = ref(false)
const clientMaterialLoading = ref(false)
const clientMaterialError = ref('')
const batchJobPanelOpen = ref(false)
const batchJobs = ref<AssetWorkbenchBatchJob[]>([])
const batchJobsLoading = ref(false)
const batchJobsError = ref('')
const selectedMaterialIds = ref<Set<string>>(new Set())
const materialPreviewUrls = ref<Record<string, string>>({})
const materialPreviewLoadingIds = ref<Set<string>>(new Set())
const activeMaterial = shallowRef<SystemAssetRow | null>(null)
const publishingClientMaterial = ref(false)
const batchUpdatingClientMaterials = ref(false)
const suppressMaterialAutoload = ref(false)
const selectedMaterialFolderPath = ref('')
const expandedMaterialFolderPaths = ref<Set<string>>(new Set(['']))
const clientMaterialFilter = ref<ClientMaterialFilter>('all')
let batchJobPollTimer: number | null = null

const materialSourceOptions: Array<{ value: MaterialSourceFilter; label: string }> = [
  { value: 'all', label: '全部来源' },
  { value: 'system', label: '系统资源' },
  { value: 'external', label: '外部资源' },
]
const materialFormatOptions: Array<{ value: MaterialFormatCategory; label: string }> = [
  { value: 'all', label: '全部格式' },
  { value: 'image', label: '图片' },
  { value: 'design', label: '设计源文件' },
  { value: 'pdf', label: 'PDF' },
  { value: 'video', label: '视频' },
  { value: 'archive', label: '压缩包' },
]
const materialBusinessLaneOptions: Array<{ value: MaterialBusinessLane; label: string }> = [
  { value: 'all', label: '全部分类' },
  { value: 'customization', label: '定制' },
  { value: 'normal', label: '常规' },
]

const previewOpen = ref(false)
const previewTitle = ref('')
const previewEyebrow = ref('素材预览')
const previewUrl = ref('')
const previewMimeType = ref('')
const previewFilename = ref('')
const previewEmptyLabel = ref('')
const previewRows = ref<Array<[string, string]>>([])
const previewDownload = shallowRef<(() => void | Promise<void>) | null>(null)

const uploadOpen = ref(false)
const uploadDialogKey = ref(0)
const uploadInitialFiles = ref<File[]>([])
const contextMenu = ref<ContextMenuState | null>(null)
const contextMenuRef = ref<HTMLElement | null>(null)
const archiveView = ref<{
  source: DriveFileRow
  path: string
  format: string
  folders: ArchiveVirtualFolder[]
  files: ArchiveVirtualFile[]
} | null>(null)
const archiveLoading = ref(false)
const archiveError = ref('')
const notice = ref('')
const actionError = ref('')

let directoryAbortController: AbortController | null = null
let filesAbortController: AbortController | null = null
let searchAbortController: AbortController | null = null
let materialAbortController: AbortController | null = null
let directoryRequestSeq = 0
let filesRequestSeq = 0
let searchRequestSeq = 0
let materialRequestSeq = 0
let directoryClickTimer: number | null = null
let searchDebounceTimer: number | null = null
let archivePreviewObjectUrl = ''
let archivePreviewRequestSeq = 0
const searchPreviewLoadingKeys = new Set<string>()

function revokeArchivePreviewObjectUrl() {
  archivePreviewRequestSeq += 1
  if (!archivePreviewObjectUrl) return
  URL.revokeObjectURL(archivePreviewObjectUrl)
  archivePreviewObjectUrl = ''
}

const creatingDirectory = ref(false)
const newDirectoryName = ref('')
const newDirectoryDifficulty = ref('')
const newDirectoryFileTypes = ref('')
const editingDirectoryKey = ref('')
const directoryEditForm = ref({
  name: '',
  difficulty_class: '',
  allowed_file_types: '',
  enabled: true,
})

const moveTargetDirectoryId = ref(0)
const deleteReason = ref('')
const maintenanceReason = ref('')
const itemEditForm = ref({
  difficulty_class: '',
  page_count: 1,
})

const currentDirRow = computed(() =>
  selectedDir.value ? directories.value.find((item) => dirKey(item) === selectedDir.value?.key) ?? null : null,
)
const directoryOptions = computed(() => uploadDirectories.value.filter((item) => item.enabled))
const difficultyOptions = computed(() => difficultyClasses.value.filter((item) => item.enabled).map((item) => item.code))
const totalPages = computed(() => Math.max(1, Math.ceil(fileTotal.value / pageSize)))
const uploadOverviewFilterActive = computed(() =>
  Boolean(uploadOverviewQuery.value.trim() || uploadOverviewOwner.value.trim() || uploadOverviewFrom.value || uploadOverviewTo.value || uploadOverviewDirectory.value !== 'all'),
)
const driveFolderBreadcrumbs = computed(() => {
  const parts = pathSegments(driveFolderPath.value)
  return [
    { path: '', name: selectedDir.value?.name || '上传目录' },
    ...parts.map((part, index) => ({ path: drivePathFromSegments(parts.slice(0, index + 1)), name: part })),
  ]
})
const selectedFiles = computed(() => files.value.filter((file) => selectedFileIds.value.has(file.id)))
const selectedFileActionIds = computed(() => {
  const ids = selectedFiles.value.map((file) => file.id)
  if (ids.length) return ids
  return selectedFile.value ? [selectedFile.value.id] : []
})
const selectedFileActionLabel = computed(() => {
  const count = selectedFileActionIds.value.length
  if (count === 0) return '未选择文件'
  return count === 1 ? '已选 1 个文件' : `已选 ${count} 个文件`
})
const archiveBreadcrumbs = computed(() => {
  const view = archiveView.value
  if (!view) return []
  const parts = pathSegments(view.path)
  return [
    { path: '', name: fileDisplayName(view.source) },
    ...parts.map((part, index) => ({ path: drivePathFromSegments(parts.slice(0, index + 1)), name: part })),
  ]
})
const allMaterialDirectoryNodes = computed<MaterialDirectoryNode[]>(() => {
  const nodes = new Map<string, MaterialDirectoryNode>()
  const ensure = (path: string) => {
    const normalized = normalizeVirtualPath(path)
    let node = nodes.get(normalized)
    if (!node) {
      node = {
        path: normalized,
        name: materialFolderLabel(normalized),
        depth: pathSegments(normalized).length,
        file_count: 0,
        direct_file_count: 0,
      }
      nodes.set(normalized, node)
    }
    return node
  }
  ensure('')
  for (const folder of Object.values(materialKnownFolders.value)) {
    if (!operationalMaterialPathVisible(folder.path) || !materialPathMatchesSourceFilter(folder.path)) continue
    const node = ensure(folder.path)
    node.name = folder.name || materialFolderLabel(folder.path)
    node.file_count = folder.file_count
    node.direct_file_count = folder.direct_file_count
  }
  const selectedParts = pathSegments(selectedMaterialFolderPath.value)
  for (let index = 1; index <= selectedParts.length; index += 1) {
    ensure(pathFromSegments(selectedParts.slice(0, index)))
  }
  if (materialQuery.value.trim()) {
    for (const asset of materialItems.value) {
      if (!materialMatchesActiveFilters(asset)) continue
      const dirParts = pathSegments(materialDirectoryPath(asset))
      ensure('').file_count += 1
      for (let index = 1; index <= dirParts.length; index += 1) {
        ensure(pathFromSegments(dirParts.slice(0, index))).file_count += 1
      }
      ensure(pathFromSegments(dirParts)).direct_file_count += 1
    }
  }
  return [...nodes.values()].sort((left, right) => {
    if (left.path === '') return -1
    if (right.path === '') return 1
    return left.path.localeCompare(right.path, 'zh-CN')
  })
})
const materialFolderPathsWithChildren = computed(() => {
  const paths = new Set<string>()
  for (const node of allMaterialDirectoryNodes.value) {
    const parts = pathSegments(node.path)
    if (parts.length === 0) continue
    paths.add(pathFromSegments(parts.slice(0, -1)))
  }
  return paths
})
const materialDirectoryNodes = computed<MaterialDirectoryNode[]>(() => {
  if (materialQuery.value.trim()) return allMaterialDirectoryNodes.value
  return allMaterialDirectoryNodes.value.filter((node) => materialFolderAncestorsExpanded(node.path))
})
const visibleMaterialDirectoryNodes = computed<VisibleMaterialDirectoryNode[]>(() =>
  materialDirectoryNodes.value.map((node) => ({
    ...node,
    has_children: materialFolderPathsWithChildren.value.has(node.path),
    expanded: expandedMaterialFolderPaths.value.has(node.path),
  })),
)
const materialFolderBreadcrumbs = computed(() => {
  const parts = pathSegments(selectedMaterialFolderPath.value)
  return [
    { path: '', name: '运营素材' },
    ...parts.map((_, index) => {
      const path = pathFromSegments(parts.slice(0, index + 1))
      return { path, name: materialFolderLabel(path) }
    }),
  ]
})
const selectedMaterialFolderNode = computed(() =>
  allMaterialDirectoryNodes.value.find((node) => node.path === selectedMaterialFolderPath.value) ?? allMaterialDirectoryNodes.value[0] ?? null,
)
const visibleMaterialFolders = computed<MaterialFolderEntry[]>(() => {
  const selectedParts = pathSegments(selectedMaterialFolderPath.value)
  return allMaterialDirectoryNodes.value
    .filter((node) => {
      if (!operationalMaterialPathVisible(node.path)) return false
      if (!materialPathMatchesSourceFilter(node.path)) return false
      if (!node.path || node.path === selectedMaterialFolderPath.value) return false
      const parts = pathSegments(node.path)
      return parts.length === selectedParts.length + 1 && selectedParts.every((part, index) => parts[index] === part)
    })
    .map((node) => ({ path: node.path, name: node.name, file_count: node.file_count, direct_file_count: node.direct_file_count }))
})
const visibleMaterialFiles = computed(() =>
  materialQuery.value.trim()
    ? materialItems.value.filter(materialMatchesActiveFilters)
    : materialItems.value.filter((asset) => materialMatchesActiveFilters(asset) && materialDirectoryPath(asset) === selectedMaterialFolderPath.value),
)
const selectedMaterialAssets = computed(() => visibleMaterialFiles.value.filter((asset) => selectedMaterialIds.value.has(materialAssetKey(asset))))
const selectedMaterialCount = computed(() => selectedMaterialAssets.value.length)
const driveSpreadsheetSource = computed<WorkbenchSpreadsheetSource>(() =>
  activeMode.value === 'operational'
    ? {
        id: 'asset-drive-material-spreadsheet',
        revision: `materials:${selectedMaterialFolderPath.value}:${visibleMaterialFiles.value.map((asset) => materialAssetKey(asset)).join('|')}`,
        mode: 'drive',
        title: '运营素材清单模式',
        description: '用于批量核对素材标题、目录、SKU 和来源；预览、发布和下载仍在右侧详情与原操作栏完成。',
        readonly: true,
        actions: [
          { key: 'refresh_materials', label: '刷新素材', tone: 'neutral', disabled: materialLoading.value },
        ],
        sheets: [
          {
            id: 'materials',
            name: '运营素材',
            rowKey: 'row_id',
            readonly: true,
            freezeHeader: true,
            columns: [
              { key: 'row_id', label: 'ID', width: 160, readonly: true },
              { key: 'title', label: '标题', width: 260, readonly: true },
              { key: 'directory', label: '目录', width: 220, readonly: true },
              { key: 'sku', label: 'SKU', width: 140, readonly: true },
              { key: 'source', label: '来源', width: 110, kind: 'status', readonly: true },
              { key: 'filename', label: '文件名', width: 240, readonly: true },
              { key: 'size', label: '大小', width: 110, readonly: true },
            ],
            rows: visibleMaterialFiles.value.map((asset) => ({
              row_id: materialAssetKey(asset),
              title: materialDisplayTitle(asset),
              directory: materialDirectoryPath(asset) || '根目录',
              sku: asset.scope_sku_code || asset.sku_code || asset.primary_sku_code || '',
              source: sourceLabelOf(asset),
              filename: asset.original_filename || '',
              size: '—',
            })),
            validations: materialSpreadsheetValidations.value,
          },
        ],
      }
    : {
        id: activeMode.value === 'uploads' ? 'asset-drive-upload-overview-spreadsheet' : 'asset-drive-file-spreadsheet',
        revision: `files:${activeMode.value}:${selectedDir.value?.key ?? 'all'}:${driveFolderPath.value}:${driveFolders.value.map((folder) => folder.path).join('|')}:${files.value.map((file) => file.id).join('|')}`,
        mode: 'drive',
        title: activeMode.value === 'uploads' ? '上传总览清单模式' : '上传素材清单模式',
        description: activeMode.value === 'uploads'
          ? '按上传人、时间、目录、格式、数量和计件金额核对全站上传记录；预览、下载和维护仍在右侧详情与操作栏完成。'
          : '用于批量核对当前目录文件、文件夹、相对路径、质检和计件状态；移动、删除和下载仍走原 Drive 操作。',
        readonly: true,
        actions: [
          { key: 'refresh_drive', label: '刷新', tone: 'neutral', disabled: filesLoading.value },
          { key: 'select_all_files', label: activeMode.value === 'uploads' ? '全选当前页' : '全选当前文件', tone: 'neutral', disabled: activeMode.value !== 'uploads' && !selectedDir.value || files.value.length === 0 },
          { key: 'download_selected', label: '下载所选', tone: 'success', disabled: selectedFileActionIds.value.length === 0 },
          { key: 'open_drive_upload', label: '上传到此处', tone: 'success', disabled: activeMode.value === 'uploads' || !selectedDir.value },
        ],
        sheets: [
          {
            id: 'files',
            name: activeMode.value === 'uploads' ? '上传总览' : selectedDir.value?.name ? `${selectedDir.value.name} 清单` : '目录清单',
            rowKey: 'row_id',
            readonly: true,
            freezeHeader: true,
            columns: [
              { key: 'row_id', label: 'ID', width: 132, readonly: true },
              { key: 'item_type', label: '类型', width: 88, kind: 'status', readonly: true },
              { key: 'name', label: '名称', width: 260, readonly: true },
              { key: 'relative_path', label: '相对路径', width: 220, readonly: true },
              { key: 'directory', label: '上传目录', width: 160, readonly: true },
              { key: 'owner_name', label: '上传人', width: 120, readonly: true },
              { key: 'format', label: '格式', width: 96, readonly: true },
              { key: 'business_month', label: '结算月', width: 104, readonly: true },
              { key: 'difficulty_class', label: '难度', width: 96, readonly: true },
              { key: 'page_count', label: '数量', width: 88, readonly: true },
              { key: 'gross_amount', label: '计件金额', width: 110, readonly: true },
              { key: 'qc_status', label: '质检', width: 96, kind: 'status', readonly: true },
              { key: 'pricing_status', label: '计件', width: 96, kind: 'status', readonly: true },
              { key: 'size', label: '大小', width: 110, readonly: true },
              { key: 'created_at', label: '上传时间', width: 150, readonly: true },
            ],
            rows: [
              ...driveFolders.value.map((folder) => ({
                row_id: `folder:${folder.path}`,
                item_type: '文件夹',
                name: folder.name,
                relative_path: folder.path,
                directory: selectedDir.value?.name || '',
                owner_name: '',
                format: '',
                business_month: '',
                difficulty_class: '',
                page_count: '',
                gross_amount: '',
                qc_status: '',
                pricing_status: '',
                size: `${folder.file_count} 个文件`,
                created_at: '',
              })),
              ...files.value.map((file) => ({
                row_id: `file:${file.id}`,
                item_type: '文件',
                name: filePathLabel(file),
                relative_path: file.relative_path || '',
                directory: file.upload_directory_name || selectedDir.value?.name || '',
                owner_name: fileOwnerLabel(file),
                format: fileFormatLabel(file),
                business_month: file.business_month || '',
                difficulty_class: file.difficulty_class || '',
                page_count: file.page_count || '',
                gross_amount: formatMoney(file.gross_amount || 0),
                qc_status: statusText(file.qc_status),
                pricing_status: statusText(file.pricing_status),
                size: formatSize(file.file_size),
                created_at: formatDateTime(file.created_at),
              })),
            ],
            validations: fileSpreadsheetValidations.value,
          },
        ],
      },
)
const fileSpreadsheetValidations = computed<WorkbenchSpreadsheetValidation[]>(() =>
  files.value.flatMap((file) => {
    const validations: WorkbenchSpreadsheetValidation[] = []
    if (file.qc_status === 'needs_fix') {
      validations.push({ rowKey: `file:${file.id}`, columnKey: 'qc_status', tone: 'warn', message: `${fileDisplayName(file)} 质检标记为需修` })
    }
    if (!file.relative_path && driveFolderPath.value) {
      validations.push({ rowKey: `file:${file.id}`, columnKey: 'relative_path', tone: 'info', message: `${fileDisplayName(file)} 位于当前虚拟目录根部` })
    }
    return validations
  }),
)
const materialSpreadsheetValidations = computed<WorkbenchSpreadsheetValidation[]>(() =>
  visibleMaterialFiles.value
    .filter((asset) => !(asset.scope_sku_code || asset.sku_code || asset.primary_sku_code))
    .map((asset) => ({ rowKey: materialAssetKey(asset), columnKey: 'sku', tone: 'warn', message: `${materialDisplayTitle(asset)} 缺少 SKU` })),
)
const materialCanLoadMore = computed(() =>
  canManageDrive.value &&
  !materialLoading.value &&
  !materialLoadingMore.value &&
  materialItems.value.length < materialFileTotal.value,
)
const selectedMaterialFolderParent = computed(() => {
  const parts = pathSegments(selectedMaterialFolderPath.value)
  if (parts.length === 0) return ''
  return pathFromSegments(parts.slice(0, -1))
})

function hasQueryValue(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== ''
}

function routeQueryString(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) {
    return value.find((item): item is string => typeof item === 'string' && item.trim() !== '') || ''
  }
  return ''
}

function dirKey(dir: DriveDirectoryRow): string {
  return dir.directory_id == null ? UNASSIGNED_KEY : String(dir.directory_id)
}

function titleOf(asset: SystemAssetRow) {
  return asset.product_name || asset.original_filename || asset.file_name || asset.task_no || `素材 ${asset.resource_id || asset.id}`
}

function pathSegments(path: string): string[] {
  return path
    .replace(/\\/g, '/')
    .split('/')
    .map((part) => part.trim())
    .filter((part) => part && part !== '.')
}

function pathFromSegments(parts: string[]): string {
  return parts.length ? `/${parts.join('/')}` : ''
}

function drivePathFromSegments(parts: string[]): string {
  return parts.join('/')
}

function normalizeDriveFolderPath(path?: string): string {
  return drivePathFromSegments(pathSegments(path || ''))
}

function driveFileParentPath(file: DriveFileRow): string {
  const parts = pathSegments(file.relative_path || file.display_name || file.original_filename || '')
  parts.pop()
  return drivePathFromSegments(parts)
}

function fileNameFromPath(path?: string): string {
  const parts = pathSegments(path || '')
  return parts.at(-1) || ''
}

function normalizeVirtualPath(path?: string): string {
  const parts = pathSegments(path || '')
  return pathFromSegments(parts)
}

function materialPathActualToVirtual(path?: string): string {
  const normalized = normalizeVirtualPath(path)
  if (!normalized) return ''
  const baseParts = pathSegments(QUARK_VISIBLE_ACTUAL_BASE)
  const parts = pathSegments(normalized)
  if (parts.length <= baseParts.length) return normalized
  const underBase = baseParts.every((part, index) => part.toLowerCase() === parts[index]?.toLowerCase())
  if (!underBase) return normalized
  const folder = parts[baseParts.length]
  if (!QUARK_VISIBLE_ROOTS.has(folder.toLowerCase())) return normalized
  return pathFromSegments(['quark', folder, ...parts.slice(baseParts.length + 1)])
}

function operationalMaterialPathVisible(path?: string): boolean {
  const parts = pathSegments(materialPathActualToVirtual(path))
  if (parts.length === 0) return true
  if (parts[0].toLowerCase() !== 'quark') return true
  if (parts.length === 1) return true
  return QUARK_VISIBLE_ROOTS.has(parts[1].toLowerCase())
}

function operationalMaterialAssetVisible(asset: SystemAssetRow): boolean {
  return operationalMaterialPathVisible(materialVirtualFilePath(asset))
}

function normalizeMaterialSourceFilter(value: unknown): MaterialSourceFilter {
  return value === 'system' || value === 'external' ? value : 'all'
}

function normalizeMaterialFormatFilter(value: unknown): MaterialFormatCategory {
  return value === 'image' || value === 'design' || value === 'pdf' || value === 'video' || value === 'archive' ? value : 'all'
}

function normalizeMaterialBusinessLaneFilter(value: unknown): MaterialBusinessLane {
  return value === 'customization' || value === 'normal' ? value : 'all'
}

function materialSourceForPath(path?: string): MaterialSourceFilter | '' {
  const parts = pathSegments(path || '')
  if (parts.length === 0) return ''
  if (parts[0] === '系统资源') return 'system'
  return 'external'
}

function materialDefaultFolderPathForSource(source = materialSourceFilter.value): string {
  if (materialBusinessLaneFilter.value !== 'all') return '/系统资源'
  if (source === 'system') return '/系统资源'
  if (source === 'external') return '/quark'
  return ''
}

function materialRequestSourceForPath(path?: string): MaterialSourceFilter {
  return normalizeMaterialSourceFilter(materialSourceForPath(path) || materialSourceFilter.value)
}

function materialRequestBusinessLane(): MaterialBusinessLane {
  return normalizeMaterialBusinessLaneFilter(materialBusinessLaneFilter.value)
}

function materialPathMatchesSourceFilter(path?: string): boolean {
  if (materialSourceFilter.value === 'all') return true
  const source = materialSourceForPath(path)
  return source === '' || source === materialSourceFilter.value
}

function materialAssetSource(asset: SystemAssetRow): MaterialSourceFilter {
  return asset.source_type === 'external' ? 'external' : 'system'
}

function materialBusinessLaneOf(asset: SystemAssetRow): MaterialBusinessLane {
  const lane = String(asset.business_lane || '').trim()
  return lane === 'customization' || lane === 'normal' ? lane : 'all'
}

function materialBusinessLaneLabel(asset: SystemAssetRow): string {
  const lane = materialBusinessLaneOf(asset)
  if (lane === 'customization') return '定制'
  if (lane === 'normal') return '常规'
  return asset.source_type === 'external' ? '外部资源' : '未标记分类'
}

function materialAssetFilenameForFormat(asset: SystemAssetRow): string {
  return [asset.original_filename, asset.file_name, asset.origin_path, asset.product_name]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
}

function materialFormatCategoryOf(asset: SystemAssetRow): MaterialFormatCategory | 'other' {
  const mime = (asset.mime_type || '').toLowerCase()
  const name = materialAssetFilenameForFormat(asset)
  if (mime.includes('photoshop') || mime.includes('illustrator') || mime.includes('postscript') || /\.(psd|psb|ai|cdr|eps|indd)\b/i.test(name)) return 'design'
  if (mime.includes('pdf') || /\.pdf\b/i.test(name)) return 'pdf'
  if (mime.startsWith('video/') || /\.(mp4|mov|m4v|avi|mkv|webm)\b/i.test(name)) return 'video'
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z') || mime.includes('tar') || mime.includes('gzip') || /\.(zip|rar|7z|tar|gz|tgz)\b/i.test(name)) return 'archive'
  if (mime.startsWith('image/') || /\.(jpe?g|png|webp|gif|bmp|svg|tiff?)\b/i.test(name)) return 'image'
  return 'other'
}

function materialMatchesActiveFilters(asset: SystemAssetRow): boolean {
  if (!operationalMaterialAssetVisible(asset)) return false
  if (materialSourceFilter.value !== 'all' && materialAssetSource(asset) !== materialSourceFilter.value) return false
  if (materialBusinessLaneFilter.value !== 'all') {
    if (materialAssetSource(asset) !== 'system') return false
    if (materialBusinessLaneOf(asset) !== materialBusinessLaneFilter.value) return false
  }
  if (materialFormatFilter.value !== 'all' && materialFormatCategoryOf(asset) !== materialFormatFilter.value) return false
  return true
}

function materialDisplayTitle(asset: SystemAssetRow): string {
  const title = titleOf(asset)
  if (asset.source_type === 'external') {
    return fileNameFromPath(asset.origin_path || title) || title
  }
  return title
}

function materialVirtualFilePath(asset: SystemAssetRow): string {
  const externalPath = normalizeVirtualPath(asset.origin_path || (asset.source_type === 'external' ? titleOf(asset) : ''))
  if (asset.source_type === 'external' && externalPath) return materialPathActualToVirtual(externalPath)
  return normalizeVirtualPath(`/系统资源/${asset.original_filename || asset.file_name || titleOf(asset)}`)
}

function materialDirectoryPath(asset: SystemAssetRow): string {
  const parts = pathSegments(materialVirtualFilePath(asset))
  parts.pop()
  return pathFromSegments(parts)
}

function materialFolderFileName(asset: SystemAssetRow): string {
  return fileNameFromPath(materialVirtualFilePath(asset)) || materialDisplayTitle(asset)
}

function materialFolderLabel(path: string): string {
  return fileNameFromPath(path) || '全部素材'
}

function rememberMaterialFolder(folder: MaterialFolderRow | MaterialFolderEntry) {
  const path = normalizeVirtualPath(folder.path)
  if (!path) return
  if (!operationalMaterialPathVisible(path)) return
  materialKnownFolders.value = {
    ...materialKnownFolders.value,
    [path]: {
      path,
      name: folder.name || materialFolderLabel(path),
      file_count: Number(folder.file_count || 0),
      direct_file_count: Number(folder.direct_file_count || 0),
      source_type: folder.source_type,
    },
  }
  rememberMaterialPath(path)
}

function rememberMaterialPath(path: string) {
  const parts = pathSegments(path)
  if (!operationalMaterialPathVisible(pathFromSegments(parts))) return
  const next = { ...materialKnownFolders.value }
  for (let index = 1; index <= parts.length; index += 1) {
    const current = pathFromSegments(parts.slice(0, index))
    if (!next[current]) {
      next[current] = {
        path: current,
        name: materialFolderLabel(current),
        file_count: 0,
        direct_file_count: 0,
      }
    }
  }
  materialKnownFolders.value = next
}

function rememberMaterialFolders(folders: MaterialFolderRow[]) {
  const next = { ...materialKnownFolders.value }
  for (const folder of folders) {
    const path = normalizeVirtualPath(folder.path)
    if (!path) continue
    if (!operationalMaterialPathVisible(path)) continue
    next[path] = {
      path,
      name: folder.name || materialFolderLabel(path),
      file_count: Number(folder.file_count || 0),
      direct_file_count: Number(folder.direct_file_count || 0),
      source_type: folder.source_type,
    }
    const parts = pathSegments(path)
    for (let index = 1; index < parts.length; index += 1) {
      const ancestor = pathFromSegments(parts.slice(0, index))
      if (!operationalMaterialPathVisible(ancestor)) continue
      if (!next[ancestor]) {
        next[ancestor] = {
          path: ancestor,
          name: materialFolderLabel(ancestor),
          file_count: 0,
          direct_file_count: 0,
        }
      }
    }
  }
  materialKnownFolders.value = next
}

function materialFolderDescendantOf(path: string, parentPath: string) {
  const normalized = normalizeVirtualPath(path)
  const parent = normalizeVirtualPath(parentPath)
  if (!parent) return !!normalized
  return normalized.startsWith(`${parent}/`)
}

function materialFolderAncestorsExpanded(path: string) {
  const parts = pathSegments(path)
  if (parts.length === 0) return true
  for (let index = 0; index < parts.length - 1; index += 1) {
    const ancestor = pathFromSegments(parts.slice(0, index + 1))
    if (!expandedMaterialFolderPaths.value.has(ancestor)) return false
  }
  return true
}

function materialFolderHasChildren(path: string) {
  return materialFolderPathsWithChildren.value.has(normalizeVirtualPath(path))
}

function isMaterialFolderExpanded(path: string) {
  return expandedMaterialFolderPaths.value.has(normalizeVirtualPath(path))
}

function setMaterialFolderExpanded(path: string, expanded: boolean) {
  const normalized = normalizeVirtualPath(path)
  const next = new Set(expandedMaterialFolderPaths.value)
  next.add('')
  if (expanded) {
    next.add(normalized)
  } else if (normalized) {
    for (const candidate of [...next]) {
      if (candidate === normalized || materialFolderDescendantOf(candidate, normalized)) {
        next.delete(candidate)
      }
    }
  }
  expandedMaterialFolderPaths.value = next
}

function expandMaterialFolderTreePath(path: string) {
  const next = new Set(expandedMaterialFolderPaths.value)
  next.add('')
  const parts = pathSegments(path)
  for (let index = 1; index <= parts.length; index += 1) {
    next.add(pathFromSegments(parts.slice(0, index)))
  }
  expandedMaterialFolderPaths.value = next
}

function sourceLabelOf(asset: SystemAssetRow) {
  return asset.source_label || (asset.source_type === 'external' ? '外部资源' : '系统资源')
}

function stringFromMeta(row: OverviewSearchRow, key: string): string {
  const value = row.meta_json?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

function numberFromUnknown(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value.trim())
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function boolFromMeta(row: OverviewSearchRow, key: string): boolean | undefined {
  const value = row.meta_json?.[key]
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true') return true
    if (normalized === 'false') return false
  }
  return undefined
}

function externalIDFromResourceID(value?: string): number {
  const raw = String(value || '').trim()
  const match = raw.match(/^(?:external:|ext-)(\d+)$/i)
  return match ? Number(match[1]) : 0
}

function normalizeMaterialSourceType(sourceType?: string, resourceID?: string): 'system' | 'external' {
  const normalized = String(sourceType || '').trim().toLowerCase()
  if (normalized === 'external' || externalIDFromResourceID(resourceID) > 0) return 'external'
  return 'system'
}

function materialResourceID(asset: SystemAssetRow): string {
  if (asset.resource_id) return asset.resource_id
  if (asset.source_type === 'external') return `ext-${asset.id}`
  return String(asset.id)
}

function clientMaterialResourceID(material: ClientMaterialRow): string {
  return material.resource_id || material.source_ref || (material.source_type === 'external' ? `ext-${material.asset_id}` : String(material.asset_id))
}

function addIdentityKey(keys: Set<string>, value?: string | number) {
  const raw = String(value ?? '').trim()
  if (!raw) return
  keys.add(raw)
}

function materialIdentityKeys(asset: SystemAssetRow): Set<string> {
  const keys = new Set<string>()
  const sourceType = normalizeMaterialSourceType(asset.source_type, asset.resource_id)
  const resourceID = materialResourceID(asset)
  addIdentityKey(keys, materialAssetKey(asset))
  addIdentityKey(keys, resourceID)
  addIdentityKey(keys, `${sourceType}:${resourceID}`)
  addIdentityKey(keys, `${sourceType}:${asset.id}`)
  if (asset.material_id) addIdentityKey(keys, `client:${asset.material_id}`)
  if (sourceType === 'external') {
    const externalID = externalIDFromResourceID(resourceID) || asset.id
    if (externalID > 0) {
      addIdentityKey(keys, `ext-${externalID}`)
      addIdentityKey(keys, `external:ext-${externalID}`)
      addIdentityKey(keys, `external:${externalID}`)
    }
  } else if (asset.id > 0) {
    addIdentityKey(keys, String(asset.id))
    addIdentityKey(keys, `system:${asset.id}`)
  }
  return keys
}

function clientMaterialIdentityKeys(material: ClientMaterialRow): Set<string> {
  const keys = new Set<string>()
  const resourceID = clientMaterialResourceID(material)
  const sourceType = normalizeMaterialSourceType(material.source_type, resourceID)
  addIdentityKey(keys, `client:${material.id}`)
  addIdentityKey(keys, resourceID)
  addIdentityKey(keys, material.source_ref)
  addIdentityKey(keys, `${sourceType}:${resourceID}`)
  addIdentityKey(keys, `${sourceType}:${material.asset_id}`)
  if (sourceType === 'external') {
    const externalID = externalIDFromResourceID(resourceID) || material.asset_id
    if (externalID > 0) {
      addIdentityKey(keys, `ext-${externalID}`)
      addIdentityKey(keys, `external:ext-${externalID}`)
      addIdentityKey(keys, `external:${externalID}`)
    }
  } else if (material.asset_id > 0) {
    addIdentityKey(keys, String(material.asset_id))
    addIdentityKey(keys, `system:${material.asset_id}`)
  }
  return keys
}

function overviewMaterialIdentityKeys(row: OverviewSearchRow): Set<string> {
  const keys = new Set<string>()
  const resourceID = row.locate?.resource_id || row.locate?.source_ref || stringFromMeta(row, 'resource_id') || stringFromMeta(row, 'source_ref') || row.primary_code
  const sourceType = normalizeMaterialSourceType(row.locate?.source_type || stringFromMeta(row, 'source_type'), resourceID)
  const assetID = overviewAssetID(row)
  addIdentityKey(keys, resourceID)
  addIdentityKey(keys, `${sourceType}:${resourceID}`)
  if (row.locate?.material_id) addIdentityKey(keys, `client:${row.locate.material_id}`)
  const materialID = numberFromUnknown(row.meta_json?.material_id)
  if (materialID > 0) addIdentityKey(keys, `client:${materialID}`)
  if (assetID > 0) {
    addIdentityKey(keys, `${sourceType}:${assetID}`)
    if (sourceType === 'system') {
      addIdentityKey(keys, String(assetID))
      addIdentityKey(keys, `system:${assetID}`)
    } else {
      addIdentityKey(keys, `ext-${assetID}`)
      addIdentityKey(keys, `external:ext-${assetID}`)
      addIdentityKey(keys, `external:${assetID}`)
    }
  }
  return keys
}

function clientMaterialForAsset(asset: SystemAssetRow): ClientMaterialRow | null {
  const assetKeys = materialIdentityKeys(asset)
  return clientMaterials.value.find((material) => hasSharedIdentity(assetKeys, clientMaterialIdentityKeys(material))) || null
}

function materialWithCurrentClientPublication(asset: SystemAssetRow): SystemAssetRow {
  const material = clientMaterialForAsset(asset)
  if (material) return materialWithClientPublication(asset, material)
  return { ...asset, material_id: undefined }
}

function syncActiveMaterialPublication() {
  if (!activeMaterial.value) return
  activeMaterial.value = materialWithCurrentClientPublication(activeMaterial.value)
}

function materialClientStatus(asset: SystemAssetRow) {
  const material = clientMaterialForAsset(asset)
  if (!material) {
    return { label: '未上架到客户端', chipClass: 'aw-chip aw-chip--neutral' }
  }
  if (material.enabled) {
    return { label: '客户端已上架', chipClass: 'aw-chip aw-chip--success' }
  }
  return { label: '客户端已停用', chipClass: 'aw-chip aw-chip--warn' }
}

function materialClientStatusLabel(asset: SystemAssetRow) {
  return materialClientStatus(asset).label
}

function materialClientStatusClass(asset: SystemAssetRow) {
  return materialClientStatus(asset).chipClass
}

function hasSharedIdentity(left: Set<string>, right: Set<string>): boolean {
  for (const key of left) {
    if (right.has(key)) return true
  }
  return false
}

function materialMatchesOverviewRow(asset: SystemAssetRow, row: OverviewSearchRow): boolean {
  return hasSharedIdentity(materialIdentityKeys(asset), overviewMaterialIdentityKeys(row))
}

function overviewAssetID(row: OverviewSearchRow): number {
  const metaAssetID = numberFromUnknown(row.meta_json?.asset_id)
  if (metaAssetID > 0) return metaAssetID
  const resourceID = row.locate?.resource_id || row.locate?.source_ref || stringFromMeta(row, 'resource_id') || stringFromMeta(row, 'source_ref') || row.primary_code
  const externalID = externalIDFromResourceID(resourceID)
  if (externalID > 0) return externalID
  if (row.source === 'client_material') return numberFromUnknown(row.locate?.source_ref || stringFromMeta(row, 'source_ref'))
  return row.id
}

function materialFromOverview(row: OverviewSearchRow): SystemAssetRow {
  const resourceID = row.locate?.resource_id || row.locate?.source_ref || stringFromMeta(row, 'resource_id') || stringFromMeta(row, 'source_ref') || row.primary_code || String(row.id)
  const sourceType = normalizeMaterialSourceType(row.locate?.source_type || stringFromMeta(row, 'source_type'), resourceID)
  const filename = stringFromMeta(row, 'original_filename') || stringFromMeta(row, 'file_name') || stringFromMeta(row, 'filename') || row.title
  const materialID = Number(row.locate?.material_id || numberFromUnknown(row.meta_json?.material_id) || 0)
  return {
    id: overviewAssetID(row),
    material_id: materialID > 0 ? materialID : undefined,
    resource_id: resourceID,
    source_type: sourceType,
    source_label: stringFromMeta(row, 'source_label') || row.source_label || (sourceType === 'external' ? '外部资源' : '系统资源'),
    asset_no: stringFromMeta(row, 'asset_no') || (row.primary_code && row.primary_code !== resourceID ? row.primary_code : ''),
    scope_sku_code: stringFromMeta(row, 'scope_sku_code') || row.secondary_code || '',
    sku_code: stringFromMeta(row, 'sku_code'),
    primary_sku_code: stringFromMeta(row, 'primary_sku_code'),
    file_name: filename,
    original_filename: filename,
    mime_type: stringFromMeta(row, 'mime_type'),
    product_name: stringFromMeta(row, 'product_name') || row.title,
    task_no: stringFromMeta(row, 'task_no') || row.order_no || '',
    preview_url: stringFromMeta(row, 'preview_url'),
    download_url: stringFromMeta(row, 'download_url'),
    created_by_name: stringFromMeta(row, 'created_by_name'),
    created_by_username: stringFromMeta(row, 'created_by_username'),
    task_creator_name: stringFromMeta(row, 'task_creator_name'),
    preview_available: boolFromMeta(row, 'preview_available') ?? false,
    origin_path: materialPathActualToVirtual(stringFromMeta(row, 'origin_path') || (row.title.startsWith('/') ? row.title : '')),
    created_at: row.created_at,
    updated_at: row.updated_at,
  }
}

function upsertMaterialItem(asset: SystemAssetRow) {
  const nextKeySet = materialIdentityKeys(asset)
  materialItems.value = [
    asset,
    ...materialItems.value.filter((item) => !hasSharedIdentity(materialIdentityKeys(item), nextKeySet)),
  ]
}

function mergeMaterialItems(current: SystemAssetRow[], incoming: SystemAssetRow[]) {
  const seen = new Set<string>()
  const merged: SystemAssetRow[] = []
  for (const item of [...current, ...incoming]) {
    const key = materialAssetKey(item)
    if (seen.has(key)) continue
    seen.add(key)
    merged.push(item)
  }
  return merged
}

function isAbortError(err: unknown) {
  if (!err || typeof err !== 'object') return false
  const maybe = err as { name?: string; code?: string }
  return maybe.name === 'AbortError' || maybe.code === 'ERR_CANCELED'
}

function abortDriveRequests() {
  directoryAbortController?.abort()
  filesAbortController?.abort()
  searchAbortController?.abort()
  materialAbortController?.abort()
}

function formatSize(size?: number): string {
  if (!size) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function formatDateTime(value?: string) {
  return formatShanghaiDateTime(value)
}

function fileFormatLabel(file: DriveFileRow): string {
  const rawType = file.file_type?.trim().replace(/^\./, '')
  const name = file.display_name || file.original_filename
  const suffix = name.includes('.') ? name.split('.').pop()?.trim() : ''
  const ext = (rawType || suffix || '').replace(/^\./, '')
  const extLabel = ext ? ext.toUpperCase() : ''
  if (file.mime_type) return extLabel ? `${extLabel} · ${file.mime_type}` : file.mime_type
  return extLabel || '—'
}

function extensionFromFilename(value?: string): string {
  const name = fileNameFromPath(value || '')
  const index = name.lastIndexOf('.')
  if (index < 0 || index === name.length - 1) return ''
  return name.slice(index + 1).trim().replace(/^\./, '').toLowerCase()
}

function normalizeExtensionLabel(value?: string): string {
  const ext = (value || '').trim().replace(/^\./, '').toLowerCase()
  if (!ext) return ''
  const map: Record<string, string> = {
    jpeg: 'JPG',
    jpg: 'JPG',
    tiff: 'TIF',
    tif: 'TIF',
    svgz: 'SVG',
    psb: 'PSD',
    ai: 'AI',
    eps: 'EPS',
    cdr: 'CDR',
    zip: 'ZIP',
    rar: 'RAR',
    '7z': '7Z',
  }
  return map[ext] || ext.toUpperCase()
}

function formatLabelFromMimeType(value?: string): string {
  const mime = (value || '').trim().toLowerCase()
  if (!mime || mime === 'application/octet-stream') return ''
  if (mime.includes('photoshop')) return 'PSD'
  if (mime.includes('illustrator') || mime.includes('postscript')) return 'AI'
  if (mime.includes('coreldraw')) return 'CDR'
  if (mime.includes('pdf')) return 'PDF'
  if (mime.includes('zip')) return 'ZIP'
  if (mime.includes('rar')) return 'RAR'
  if (mime.includes('7z') || mime.includes('7-zip')) return '7Z'
  if (mime.startsWith('image/')) return normalizeExtensionLabel(mime.replace('image/', ''))
  if (mime.startsWith('video/')) return normalizeExtensionLabel(mime.replace('video/', ''))
  if (mime.startsWith('audio/')) return normalizeExtensionLabel(mime.replace('audio/', ''))
  return ''
}

function searchHitFilename(row: OverviewSearchRow): string {
  return (
    stringFromMeta(row, 'display_name') ||
    stringFromMeta(row, 'original_filename') ||
    stringFromMeta(row, 'file_name') ||
    stringFromMeta(row, 'filename') ||
    fileNameFromPath(stringFromMeta(row, 'relative_path')) ||
    fileNameFromPath(stringFromMeta(row, 'origin_path')) ||
    ''
  )
}

function searchHitFormatLabel(row: OverviewSearchRow): string {
  const metaFileType = stringFromMeta(row, 'file_type')
  const filename = searchHitFilename(row) || row.title || row.primary_code
  return (
    normalizeExtensionLabel(metaFileType) ||
    formatLabelFromMimeType(stringFromMeta(row, 'mime_type')) ||
    normalizeExtensionLabel(extensionFromFilename(filename)) ||
    '文件'
  )
}

function searchHitSourceLabel(row: OverviewSearchRow): string {
  if (row.source_label) return row.source_label
  if (row.source === 'system_asset') return '运营素材'
  if (row.source === 'client_material') return '客户端素材'
  if (row.source === 'submission_file') return '上传文件'
  if (row.source === 'piecework_item') return '计件记录'
  if (row.source === 'submission') return '提交记录'
  return '搜索结果'
}

function searchHitContextLabel(row: OverviewSearchRow): string {
  const parts = [searchHitSourceLabel(row)]
  if (row.source === 'submission_file') {
    const directory = stringFromMeta(row, 'upload_directory_name')
    if (directory) parts.push(directory)
    else if (row.business_month) parts.push(row.business_month)
  } else if (row.scope === 'operational' || row.source === 'system_asset' || row.source === 'client_material') {
    const sku = row.secondary_code || stringFromMeta(row, 'scope_sku_code') || stringFromMeta(row, 'sku_code') || stringFromMeta(row, 'primary_sku_code')
    if (sku) parts.push(`SKU ${sku}`)
  } else if (row.creator_name) {
    parts.push(row.creator_name)
  }
  return parts.filter(Boolean).join(' · ')
}

function searchHitSecondaryLabel(row: OverviewSearchRow): string {
  const filename = searchHitFilename(row)
  const title = (row.title || '').trim()
  const parts: string[] = []
  if (filename && filename !== title) {
    parts.push(`文件 ${filename}`)
  } else if (row.primary_code && row.primary_code !== title) {
    parts.push(`编码 ${row.primary_code}`)
  }
  if (row.creator_name) {
    parts.push(row.source === 'submission_file' ? `上传人 ${row.creator_name}` : `创建人 ${row.creator_name}`)
  }
  return parts.join(' · ') || '可在网盘中定位'
}

function isOperationalSearchHit(row: OverviewSearchRow): boolean {
  return row.scope === 'operational' || row.source === 'system_asset' || row.source === 'client_material'
}

function searchHitMaterial(row: OverviewSearchRow): SystemAssetRow {
  return materialFromOverview(row)
}

function searchHitDriveFileID(row: OverviewSearchRow): number {
  if (row.source !== 'submission_file') return 0
  const fileID = Number(row.locate?.file_id || numberFromUnknown(row.meta_json?.file_id) || row.id || 0)
  return Number.isFinite(fileID) && fileID > 0 ? fileID : 0
}

function searchHitMimeType(row: OverviewSearchRow): string {
  return stringFromMeta(row, 'mime_type')
}

function searchHitPreviewStatus(row: OverviewSearchRow): string {
  return stringFromMeta(row, 'preview_status') || row.status || ''
}

function fileDisplayName(file: DriveFileRow): string {
  return file.display_name || file.original_filename || `文件 ${file.id}`
}

function filePathLabel(file: DriveFileRow): string {
  return file.relative_path || fileDisplayName(file)
}

function fileOwnerLabel(file: DriveFileRow): string {
  return file.owner_name || file.owner_username || (file.owner_user_id ? `用户 ${file.owner_user_id}` : '—')
}

function statusText(value?: string) {
  const normalized = (value || '').trim()
  const map: Record<string, string> = {
    submitted: '已提交',
    checked: '已通过',
    needs_fix: '需修',
    pending: '待处理',
    pending_grade: '待定级',
    unpriced: '待补价',
    priced: '已计件',
    voided: '已作废',
    unsettled: '未结算',
    in_batch: '批次中',
    settled: '已结算',
    reversed: '已调整',
    processing: '处理中',
    ready: '可预览',
    failed: '失败',
    not_applicable: '不适用',
    ready_for_use: '可直接使用',
    enabled: '上架中',
    disabled: '已停用',
  }
  return map[normalized] || normalized || '—'
}

function normalizeScope(value: unknown): SearchScope {
  if (value === 'operational' || value === 'files' || value === 'orders') return value
  return 'all'
}

function makeDirectoryPrefix(name: string) {
  const ascii = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return ascii || `dir-${Date.now().toString(36)}`
}

function parseFileTypeInput(raw: string): string[] {
  const values = raw
    .split(/[,\s，、]+/)
    .map((value) => value.trim().toLowerCase().replace(/^\.+/, ''))
    .filter(Boolean)
  return [...new Set(values)]
}

function fileTypesInputValue(values?: string[]): string {
  return values?.length ? values.join(', ') : ''
}

function allowedFileTypesLabel(values?: string[]): string {
  return values?.length ? values.join('、') : '全部格式'
}

function driveDirectoryFromUpload(row: UploadDirectoryRow, count?: DriveDirectoryRow): DriveDirectoryRow {
  return {
    directory_id: row.id,
    name: row.name,
    prefix: row.oss_prefix,
    difficulty_class: row.difficulty_class,
    allowed_file_types: row.allowed_file_types,
    description: row.description,
    enabled: row.enabled,
    sort_order: row.sort_order,
    file_count: count?.file_count ?? 0,
    order_count: count?.order_count ?? 0,
  }
}

function materialFromClient(row: ClientMaterialRow): SystemAssetRow {
  return {
    id: row.asset_id,
    material_id: row.id,
    resource_id: row.resource_id || row.source_ref || String(row.asset_id),
    source_type: row.source_type || 'system',
    source_label: row.source_label || (row.source_type === 'external' ? '外部资源' : '系统资源'),
    scope_sku_code: row.scope_sku_code,
    sku_code: row.sku_code,
    primary_sku_code: row.primary_sku_code,
    file_name: row.filename_snapshot,
    original_filename: row.filename_snapshot,
    mime_type: row.mime_type_snapshot,
    product_name: row.title,
    preview_available: row.preview_available,
  }
}

function materialWithClientPublication(asset: SystemAssetRow, row: ClientMaterialRow): SystemAssetRow {
  const published = materialFromClient(row)
  return {
    ...published,
    asset_no: asset.asset_no,
    product_name: row.title || asset.product_name || published.product_name,
    file_name: published.file_name || asset.file_name,
    original_filename: published.original_filename || asset.original_filename,
    mime_type: published.mime_type || asset.mime_type,
    preview_url: asset.preview_url,
    download_url: asset.download_url,
    origin_path: asset.origin_path,
    task_no: asset.task_no,
    created_by_name: asset.created_by_name,
    created_by_username: asset.created_by_username,
    task_creator_name: asset.task_creator_name,
    task_creator_username: asset.task_creator_username,
    created_at: asset.created_at,
    updated_at: asset.updated_at,
  }
}

const activeClientMaterial = computed(() => {
  const asset = activeMaterial.value
  if (!asset) return null
  return clientMaterialForAsset(asset)
})
const enabledClientMaterialCount = computed(() => clientMaterials.value.filter((material) => material.enabled).length)
const disabledClientMaterialCount = computed(() => clientMaterials.value.length - enabledClientMaterialCount.value)
const activeBatchJobCount = computed(() => batchJobs.value.filter((job) => job.status === 'queued' || job.status === 'running').length)
const visibleClientMaterials = computed(() => {
  switch (clientMaterialFilter.value) {
    case 'enabled':
      return clientMaterials.value.filter((material) => material.enabled)
    case 'disabled':
      return clientMaterials.value.filter((material) => !material.enabled)
    default:
      return clientMaterials.value
  }
})

async function loadDifficultyClasses() {
  try {
    difficultyClasses.value = await assetWorkbenchApi.listDifficultyClasses()
    if (!newDirectoryDifficulty.value) newDirectoryDifficulty.value = difficultyOptions.value[0] || ''
  } catch {
    difficultyClasses.value = []
  }
}

async function loadDirectories() {
  dirLoading.value = true
  dirError.value = ''
  try {
    const [driveRows, uploadRows] = await Promise.all([
      assetWorkbenchApi.driveDirectories(),
      canManageDrive.value
        ? assetWorkbenchApi.listUploadDirectoriesAdmin()
        : canListUploadDirectories.value
          ? assetWorkbenchApi.listUploadDirectories()
          : Promise.resolve([] as UploadDirectoryRow[]),
    ])
    uploadDirectories.value = uploadRows
    const countByID = new Map<string, DriveDirectoryRow>()
    for (const row of driveRows) countByID.set(dirKey(row), row)
    const merged = uploadRows.map((row) => driveDirectoryFromUpload(row, countByID.get(String(row.id))))
    const uploadIDs = new Set(uploadRows.map((row) => String(row.id)))
    for (const row of driveRows) {
      const key = dirKey(row)
      if (key === UNASSIGNED_KEY || !uploadIDs.has(key)) merged.push(row)
    }
    directories.value = merged.sort((a, b) => {
      const aSort = a.sort_order ?? 9999
      const bSort = b.sort_order ?? 9999
      if (aSort !== bSort) return aSort - bSort
      return a.name.localeCompare(b.name, 'zh-CN')
    })
  } catch (err) {
    dirError.value = err instanceof Error ? err.message : '目录加载失败'
  } finally {
    dirLoading.value = false
  }
}

async function selectDir(dir: DriveDirectoryRow, keepFile = false, initialPage = 1, initialFolderPath = '') {
  const requestID = ++directoryRequestSeq
  directoryAbortController?.abort()
  directoryAbortController = new AbortController()
  closeContextMenu()
  closeArchiveView()
  const next: SelectedDir = {
    key: dirKey(dir),
    id: dir.directory_id ?? null,
    name: dir.name,
    unassigned: dir.directory_id == null,
  }
  selectedDir.value = next
  driveFolderPath.value = normalizeDriveFolderPath(initialFolderPath)
  driveFolders.value = []
  driveFolderTruncated.value = false
  files.value = []
  fileTotal.value = 0
  filesError.value = ''
  selectedFileIds.value = new Set()
  if (!keepFile) selectedFile.value = null
  filePage.value = Math.max(1, Math.floor(initialPage || 1))
  await loadFiles(directoryAbortController.signal, requestID)
}

function uploadOverviewDirectoryParams() {
  if (uploadOverviewDirectory.value === 'unassigned') return { unassigned: true }
  const directoryID = Number(uploadOverviewDirectory.value)
  if (Number.isFinite(directoryID) && directoryID > 0) return { dir_id: directoryID }
  return {}
}

async function loadUploadOverview(signal?: AbortSignal) {
  const params = {
    ...uploadOverviewDirectoryParams(),
    q: uploadOverviewQuery.value.trim() || undefined,
    owner: uploadOverviewOwner.value.trim() || undefined,
    created_from: uploadOverviewFrom.value || undefined,
    created_to: uploadOverviewTo.value || undefined,
    page: filePage.value,
    page_size: pageSize,
  }
  const result = await assetWorkbenchApi.driveFiles(params, signal)
  driveFolderPath.value = ''
  driveFolders.value = []
  driveFolderTruncated.value = false
  files.value = result.items
  fileTotal.value = result.total
}

async function loadFiles(signal?: AbortSignal, parentRequestID?: number) {
  if (activeMode.value !== 'uploads' && !selectedDir.value) return
  const requestID = ++filesRequestSeq
  if (!signal) {
    filesAbortController?.abort()
    filesAbortController = new AbortController()
    signal = filesAbortController.signal
  }
  filesLoading.value = true
  filesError.value = ''
  try {
    if (activeMode.value === 'uploads') {
      await loadUploadOverview(signal)
      if (requestID !== filesRequestSeq) return
      return
    }
    if (!selectedDir.value) return
    const result = await assetWorkbenchApi.driveFolder({
      dir_id: selectedDir.value.unassigned ? undefined : selectedDir.value.id ?? undefined,
      unassigned: selectedDir.value.unassigned,
      path: driveFolderPath.value,
      page: filePage.value,
      page_size: pageSize,
    }, signal)
    if (requestID !== filesRequestSeq || (parentRequestID && parentRequestID !== directoryRequestSeq)) return
    driveFolderPath.value = normalizeDriveFolderPath(result.path)
    driveFolders.value = result.folders || []
    driveFolderTruncated.value = Boolean(result.truncated)
    files.value = result.files || []
    fileTotal.value = result.total
  } catch (err) {
    if (requestID !== filesRequestSeq || isAbortError(err)) return
    files.value = []
    driveFolders.value = []
    driveFolderTruncated.value = false
    fileTotal.value = 0
    filesError.value = err instanceof Error ? err.message : '文件列表加载失败'
  } finally {
    if (requestID === filesRequestSeq) filesLoading.value = false
  }
}

async function openUploadOverview() {
  activeMode.value = 'uploads'
  closeArchiveView()
  selectedDir.value = null
  driveFolderPath.value = ''
  driveFolders.value = []
  driveFolderTruncated.value = false
  selectedFile.value = null
  selectedFileIds.value = new Set()
  filePage.value = 1
  await loadFiles()
}

async function applyUploadOverviewFilters() {
  activeMode.value = 'uploads'
  closeArchiveView()
  selectedFile.value = null
  selectedFileIds.value = new Set()
  filePage.value = 1
  await loadFiles()
}

async function clearUploadOverviewFilters() {
  uploadOverviewQuery.value = ''
  uploadOverviewOwner.value = ''
  uploadOverviewFrom.value = ''
  uploadOverviewTo.value = ''
  uploadOverviewDirectory.value = 'all'
  await applyUploadOverviewFilters()
}

async function refreshCurrentDrive() {
  if (activeMode.value === 'uploads') {
    await loadDirectories()
    await loadFiles()
    return
  }
  const prevKey = selectedDir.value?.key ?? null
  const prevFolderPath = driveFolderPath.value
  await loadDirectories()
  if (!prevKey) return
  const dir = directories.value.find((item) => dirKey(item) === prevKey)
  if (!dir) return
  await selectDir(dir, true, filePage.value, prevFolderPath)
}

async function changePage(delta: number) {
  const next = filePage.value + delta
  if (next < 1 || next > totalPages.value) return
  filePage.value = next
  await loadFiles()
}

function selectFile(file: DriveFileRow, additive = false) {
  selectedFile.value = file
  highlightFileId.value = file.id
  const next = additive ? new Set(selectedFileIds.value) : new Set<number>()
  next.add(file.id)
  selectedFileIds.value = next
}

function toggleFile(file: DriveFileRow, checked: boolean) {
  const next = new Set(selectedFileIds.value)
  if (checked) next.add(file.id)
  else next.delete(file.id)
  selectedFileIds.value = next
  selectedFile.value = file
}

function selectAllFilesInDirectory() {
  selectedFileIds.value = new Set(files.value.map((file) => file.id))
  if (!selectedFile.value && files.value[0]) selectedFile.value = files.value[0]
}

function clearSelection() {
  selectedFileIds.value = new Set()
}

function resetToRoot() {
  closeArchiveView()
  selectedDir.value = null
  driveFolderPath.value = ''
  driveFolders.value = []
  driveFolderTruncated.value = false
  files.value = []
  fileTotal.value = 0
  filesError.value = ''
  selectedFile.value = null
  selectedFileIds.value = new Set()
}

async function openDriveFolder(path: string) {
  closeArchiveView()
  driveFolderPath.value = normalizeDriveFolderPath(path)
  filePage.value = 1
  selectedFile.value = null
  selectedFileIds.value = new Set()
  await loadFiles()
}

function upsertDriveFile(file: DriveFileRow) {
  if (files.value.some((item) => item.id === file.id)) return
  files.value = [file, ...files.value]
}

function openDirectory(dir: DriveDirectoryRow) {
  activeMode.value = 'directories'
  void selectDir(dir)
}

function queueOpenDirectory(dir: DriveDirectoryRow) {
  if (directoryClickTimer) window.clearTimeout(directoryClickTimer)
  directoryClickTimer = window.setTimeout(() => {
    directoryClickTimer = null
    openDirectory(dir)
  }, 180)
}

function editDirectoryFromDoubleClick(dir: DriveDirectoryRow) {
  if (directoryClickTimer) {
    window.clearTimeout(directoryClickTimer)
    directoryClickTimer = null
  }
  startDirectoryEdit(dir)
}

function goDrivesHome() {
  activeMode.value = 'directories'
  resetToRoot()
}

function openOperational() {
  activeMode.value = 'operational'
  selectedFile.value = null
}

function openMaterialFolder(path: string) {
  void loadMaterialFolder(path, { expandTree: true })
}

function toggleMaterialFolderNode(path: string) {
  const normalized = normalizeVirtualPath(path)
  const canCollapse = normalized && isMaterialFolderExpanded(normalized) && materialFolderHasChildren(normalized)
  if (canCollapse) {
    setMaterialFolderExpanded(normalized, false)
    if (selectedMaterialFolderPath.value === normalized || materialFolderDescendantOf(selectedMaterialFolderPath.value, normalized)) {
      void loadMaterialFolder(normalized, { expandTree: false })
    }
    return
  }
  setMaterialFolderExpanded(normalized, true)
  void loadMaterialFolder(normalized, { expandTree: true })
}

function openMaterialFolderParent() {
  openMaterialFolder(selectedMaterialFolderParent.value)
}

async function revealMaterialInFolder(asset: SystemAssetRow) {
  await loadMaterialFolder(materialDirectoryPath(asset), { expandTree: true })
  upsertMaterialItem(asset)
  selectMaterial(asset)
}

const detailOpen = computed(() =>
  activeMode.value === 'operational' ? !!activeMaterial.value : !!selectedFile.value,
)

function closeDetail() {
  if (activeMode.value === 'operational') {
    activeMaterial.value = null
  } else {
    selectedFile.value = null
    selectedFileIds.value = new Set()
  }
}

async function revealFile(file: DriveFileRow) {
  searchActive.value = false
  activeMode.value = 'directories'
  const targetKey = file.upload_directory_id == null ? UNASSIGNED_KEY : String(file.upload_directory_id)
  let dir = directories.value.find((item) => dirKey(item) === targetKey)
  if (!dir) {
    await loadDirectories()
    dir = directories.value.find((item) => dirKey(item) === targetKey)
  }
  if (!dir) return
  const parentPath = driveFileParentPath(file)
  await selectDir(dir, true, parentPath ? 1 : file.locate_page || 1, parentPath)
  upsertDriveFile(file)
  selectFile(file)
  window.setTimeout(() => {
    if (highlightFileId.value === file.id) highlightFileId.value = null
  }, 2400)
}

async function runUnifiedSearch() {
  cancelSearchDebounce()
  const q = searchQuery.value.trim()
  if (!q) {
    clearSearch()
    return
  }
  const requestID = ++searchRequestSeq
  searchAbortController?.abort()
  searchAbortController = new AbortController()
  searchActive.value = true
  searchLoading.value = true
  searchError.value = ''
  searchResults.value = []
  searchTotal.value = 0
  try {
    const result = await assetWorkbenchApi.overviewSearch({ q, scope: searchScope.value, page: 1, page_size: 60 }, searchAbortController.signal)
    if (requestID !== searchRequestSeq) return
    searchResults.value = result.items
    searchTotal.value = result.total
    void prefetchSearchResultPreviews(result.items)
  } catch (err) {
    if (requestID !== searchRequestSeq || isAbortError(err)) return
    searchResults.value = []
    searchTotal.value = 0
    searchError.value = err instanceof Error ? err.message : '统一检索失败'
  } finally {
    if (requestID === searchRequestSeq) searchLoading.value = false
  }
}

async function prefetchSearchResultPreviews(rows: OverviewSearchRow[]) {
  const candidates = rows
    .filter(isOperationalSearchHit)
    .map(searchHitMaterial)
    .filter((asset) => canAttemptSystemAssetPreview(asset))
    .slice(0, searchPreviewPrefetchLimit)
  await Promise.allSettled(candidates.map((asset) => ensureSearchResultMaterialPreview(asset)))
}

async function ensureSearchResultMaterialPreview(asset: SystemAssetRow) {
  const key = materialAssetKey(asset)
  if (!key || materialPreviewUrls.value[key] || searchPreviewLoadingKeys.has(key)) return
  const inline = resolvedSystemAssetThumbnailUrl(asset)
  if (inline) {
    cacheMaterialPreview(key, inline)
    return
  }
  searchPreviewLoadingKeys.add(key)
  try {
    const meta = await previewMaterial(asset)
    const url = meta.preview_url || ''
    if (url) cacheMaterialPreview(key, url)
  } catch {
    /* Search thumbnails are opportunistic; detail preview still reports errors. */
  } finally {
    searchPreviewLoadingKeys.delete(key)
  }
}

function clearSearch() {
  cancelSearchDebounce()
  resetSearchState(false)
}

function resetSearchState(keepQuery: boolean) {
  searchAbortController?.abort()
  searchRequestSeq += 1
  searchActive.value = false
  if (!keepQuery) searchQuery.value = ''
  searchError.value = ''
  searchResults.value = []
  searchTotal.value = 0
  searchLoading.value = false
}

function cancelSearchDebounce() {
  if (!searchDebounceTimer) return
  window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = null
}

function scheduleUnifiedSearch() {
  cancelSearchDebounce()
  const q = searchQuery.value.trim()
  if (!q) {
    resetSearchState(false)
    return
  }
  if (q.length < 2) {
    resetSearchState(true)
    return
  }
  searchDebounceTimer = window.setTimeout(() => {
    searchDebounceTimer = null
    void runUnifiedSearch()
  }, searchDebounceMs)
}

async function locateSearchRow(row: OverviewSearchRow) {
  if (row.scope === 'operational' || row.source === 'system_asset' || row.source === 'client_material') {
    suppressMaterialAutoload.value = true
    try {
      activeMode.value = 'operational'
      await loadMaterials(row.title || row.primary_code || materialQuery.value)
      const found = materialItems.value.find((asset) => materialMatchesOverviewRow(asset, row))
      const target = found || materialFromOverview(row)
      if (!found) upsertMaterialItem(target)
      await revealMaterialInFolder(target)
      searchActive.value = false
      notice.value = `已定位目录：${materialDirectoryPath(target) || '全部素材'} · ${materialFolderFileName(target)}`
    } finally {
      suppressMaterialAutoload.value = false
    }
    return
  }
  const fileID = Number(row.locate?.file_id || 0)
  if (fileID > 0) {
    try {
      const file = await assetWorkbenchApi.driveLocate(fileID)
      await revealFile(file)
      return
    } catch {
      /* fall through to keyword search */
    }
  }
  activeMode.value = 'directories'
  searchQuery.value = row.order_no || row.primary_code || row.title
  searchScope.value = 'files'
  await runUnifiedSearch()
}

function openPreviewDialog(args: {
  title: string
  eyebrow: string
  url?: string
  mimeType?: string
  filename?: string
  emptyLabel?: string
  rows?: Array<[string, string]>
  download?: () => void | Promise<void>
}) {
  revokeArchivePreviewObjectUrl()
  previewTitle.value = args.title
  previewEyebrow.value = args.eyebrow
  previewUrl.value = args.url || ''
  previewMimeType.value = args.mimeType || ''
  previewFilename.value = args.filename || args.title
  previewEmptyLabel.value = args.emptyLabel || ''
  previewRows.value = args.rows ?? []
  previewDownload.value = args.download ?? null
  previewOpen.value = true
}

function closePreview() {
  previewOpen.value = false
  previewUrl.value = ''
  previewDownload.value = null
  revokeArchivePreviewObjectUrl()
}

async function openFilePreview(file: DriveFileRow) {
  if (canOpenArchive(file)) {
    await openArchiveFile(file)
    return
  }
  selectFile(file, true)
  openPreviewDialog({
    title: fileDisplayName(file),
    eyebrow: '文件预览',
    emptyLabel: '正在加载预览…',
    mimeType: file.mime_type,
    filename: fileDisplayName(file),
    rows: filePreviewRows(file),
    download: () => downloadFile(file),
  })
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    previewUrl.value = meta.preview_url || ''
    previewEmptyLabel.value = meta.preview_url ? '' : '暂无可展示预览'
  } catch (err) {
    previewEmptyLabel.value = err instanceof Error ? err.message : '预览加载失败'
  }
}

function filePreviewRows(file: DriveFileRow): Array<[string, string]> {
  return [
    ['所在目录', file.upload_directory_name],
    ['文件夹', file.relative_path || '—'],
    ['上传人', fileOwnerLabel(file)],
    ['格式', fileFormatLabel(file)],
    ['上传时间', formatDateTime(file.created_at)],
    ['结算月份', file.business_month || '—'],
    ['难度', file.difficulty_class || '—'],
    ['数量', file.page_count ? `${file.page_count}` : '—'],
    ['计件金额', formatMoney(file.gross_amount || 0)],
    ['质检', statusText(file.qc_status)],
    ['计件', statusText(file.pricing_status)],
    ['大小', formatSize(file.file_size)],
  ]
}

function downloadFile(file: DriveFileRow) {
  actionError.value = ''
  const result = queueDriveFile(file)
  notice.value = result.duplicate
    ? '这个文件已在下载中心，无需重复点击'
    : '已加入下载中心，可以继续使用其他页面'
}

async function downloadSelectedFiles() {
  const ids = selectedFileActionIds.value
  if (!ids.length) return
  notice.value = '正在准备下载'
  actionError.value = ''
  try {
    const manifest = await assetWorkbenchApi.batchDownloadFiles(ids)
    const result = await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.file_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `file-${item.file_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `file_id=${failure.file_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-drive-files'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已下载 ${result.writtenCount} 个文件，失败 ${result.failureCount} 个`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '下载失败'
  }
}

function openUpload(files: File[] = []) {
  if (!selectedDir.value) return
  uploadInitialFiles.value = files
  uploadDialogKey.value += 1
  uploadOpen.value = true
}

async function handleDriveSpreadsheetAction(payload: WorkbenchSpreadsheetActionPayload) {
  if (payload.action.key === 'refresh_drive') {
    await loadFiles()
    return
  }
  if (payload.action.key === 'select_all_files') {
    selectAllFilesInDirectory()
    return
  }
  if (payload.action.key === 'download_selected') {
    await downloadSelectedFiles()
    return
  }
  if (payload.action.key === 'open_drive_upload') {
    openUpload()
    return
  }
  if (payload.action.key === 'refresh_materials') {
    await loadMaterials()
  }
}

function filesFromDrop(event: DragEvent) {
  return Array.from(event.dataTransfer?.files ?? []).filter((file) => file.size > 0)
}

function dropOnDirectory(event: DragEvent, dir: DriveDirectoryRow) {
  const dropped = filesFromDrop(event)
  if (!dropped.length) return
  void selectDir(dir, true).then(() => openUpload(dropped))
}

function dropOnCurrentDirectory(event: DragEvent) {
  const dropped = filesFromDrop(event)
  if (!dropped.length || !selectedDir.value) return
  openUpload(dropped)
}

async function onUploaded() {
  uploadOpen.value = false
  uploadInitialFiles.value = []
  await refreshCurrentDrive()
}

async function createDirectory() {
  if (!canManageDrive.value) return
  const name = newDirectoryName.value.trim()
  if (!name) return
  const difficulty = newDirectoryDifficulty.value || difficultyOptions.value[0] || 'A'
  notice.value = ''
  actionError.value = ''
  try {
    const created = await assetWorkbenchApi.createUploadDirectory({
      name,
      oss_prefix: makeDirectoryPrefix(name),
      difficulty_class: difficulty,
      allowed_file_types: parseFileTypeInput(newDirectoryFileTypes.value),
      enabled: true,
      sort_order: uploadDirectories.value.length + 1,
    })
    creatingDirectory.value = false
    newDirectoryName.value = ''
    newDirectoryFileTypes.value = ''
    await loadDirectories()
    const dir = directories.value.find((item) => item.directory_id === created.id)
    if (dir) await selectDir(dir)
    notice.value = `已创建上传目录：${created.name}`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '目录创建失败'
  }
}

function startDirectoryEdit(dir: DriveDirectoryRow) {
  if (!canManageDrive.value || dir.directory_id == null) return
  editingDirectoryKey.value = dirKey(dir)
  directoryEditForm.value = {
    name: dir.name,
    difficulty_class: dir.difficulty_class || difficultyOptions.value[0] || '',
    allowed_file_types: fileTypesInputValue(dir.allowed_file_types),
    enabled: dir.enabled !== false,
  }
  closeContextMenu()
}

async function saveDirectoryEdit(dir: DriveDirectoryRow) {
  if (!canManageDrive.value || dir.directory_id == null) return
  try {
    await assetWorkbenchApi.updateUploadDirectory(dir.directory_id, {
      name: directoryEditForm.value.name,
      difficulty_class: directoryEditForm.value.difficulty_class,
      allowed_file_types: parseFileTypeInput(directoryEditForm.value.allowed_file_types),
      enabled: directoryEditForm.value.enabled,
    })
    editingDirectoryKey.value = ''
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '目录保存失败'
  }
}

async function setDirectoryEnabled(dir: DriveDirectoryRow, enabled: boolean) {
  if (!canManageDrive.value || dir.directory_id == null) return
  try {
    await assetWorkbenchApi.updateUploadDirectory(dir.directory_id, { enabled })
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '目录状态更新失败'
  } finally {
    closeContextMenu()
  }
}

async function saveSelectedItemEdit() {
  const file = selectedFile.value
  if (!file || !canMaintainItems.value) return
  try {
    await assetWorkbenchApi.updateSubmissionItem(file.submission_item_id, {
      difficulty_class: itemEditForm.value.difficulty_class,
      page_count: itemEditForm.value.page_count,
      finalized: true,
      reason: maintenanceReason.value || '素材网盘文件维护',
    })
    notice.value = `已更新 ${fileDisplayName(file)}`
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '文件维护保存失败'
  }
}

async function setSelectedQC(qcStatus: string) {
  const file = selectedFile.value
  if (!file || !canMaintainItems.value) return
  try {
    await assetWorkbenchApi.updateSubmissionItemQC(file.submission_item_id, {
      qc_status: qcStatus,
      reason: maintenanceReason.value || undefined,
    })
    notice.value = qcStatus === 'checked' ? '已标记通过' : '已标记需修'
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '质检状态更新失败'
  }
}

async function repriceSelectedItem() {
  const file = selectedFile.value
  if (!file || !canMaintainItems.value) return
  try {
    const updated = await assetWorkbenchApi.repriceSubmissionItem(file.submission_item_id, maintenanceReason.value || '素材网盘重新计件')
    notice.value = `已重新计件：${statusText(updated.pricing_status)}`
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '重新计件失败'
  }
}

async function moveSelectedFiles() {
  if (!canManageDrive.value || fileMutationLoading.value) return
  const ids = selectedFileActionIds.value
  if (!ids.length || !moveTargetDirectoryId.value) return
  fileMutationLoading.value = true
  actionError.value = ''
  try {
    const result = await assetWorkbenchApi.batchMoveFiles(ids, moveTargetDirectoryId.value, maintenanceReason.value || '素材网盘移动文件')
    const movedCount = result.files?.length ?? 0
    notice.value = movedCount ? `已移动 ${movedCount} 个文件` : ''
    actionError.value = batchMutationFailureMessage('移动', result.failures)
    selectedFileIds.value = new Set((result.failures ?? []).map((failure) => failure.file_id))
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '移动文件失败'
  } finally {
    fileMutationLoading.value = false
  }
}

async function deleteSelectedFiles() {
  if (!canDeleteDriveFiles.value || fileMutationLoading.value) return
  const ids = selectedFileActionIds.value
  if (!ids.length || !deleteReason.value.trim()) return
  fileMutationLoading.value = true
  actionError.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteFiles(ids, deleteReason.value.trim())
    const deletedCount = result.deleted?.length ?? 0
    notice.value = deletedCount ? `已删除 ${deletedCount} 个文件` : ''
    actionError.value = batchMutationFailureMessage('删除', result.failures)
    selectedFileIds.value = new Set((result.failures ?? []).map((failure) => failure.file_id))
    if (!result.failures?.length) deleteReason.value = ''
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除文件失败'
  } finally {
    fileMutationLoading.value = false
  }
}

function archiveFormatOf(file: DriveFileRow | null | undefined) {
  if (!file) return ''
  const name = (file.display_name || file.original_filename || '').toLowerCase()
  const ext = name.split('.').pop() || ''
  if (['zip', 'rar', '7z'].includes(ext)) return ext
  if (file.file_type === 'archive') return ext || 'archive'
  return ''
}

function canOpenArchive(file: DriveFileRow | null | undefined) {
  return ['zip', 'rar'].includes(archiveFormatOf(file))
}

async function openArchiveFile(file: DriveFileRow | null | undefined, path = '') {
  if (!file) return
  if (!canOpenArchive(file)) {
    actionError.value = '该压缩包暂不支持在线打开，可下载后查看'
    return
  }
  closeContextMenu()
  archiveLoading.value = true
  archiveError.value = ''
  try {
    const result = await assetWorkbenchApi.browseArchiveFile(file.id, path)
    archiveView.value = {
      source: file,
      path: result.path || '',
      format: result.format,
      folders: result.folders || [],
      files: result.files || [],
    }
    selectedFile.value = file
  } catch (err) {
    archiveError.value = err instanceof Error ? err.message : '压缩包打开失败'
  } finally {
    archiveLoading.value = false
  }
}

function closeArchiveView() {
  archiveView.value = null
  archiveError.value = ''
  archiveLoading.value = false
}

function openArchiveFolder(path: string) {
  const source = archiveView.value?.source
  if (!source) return
  void openArchiveFile(source, path)
}

async function openArchiveVirtualFile(file: ArchiveVirtualFile) {
  const canPreview = canPreviewArchiveVirtualFile(file)
  openPreviewDialog({
    title: file.name,
    eyebrow: '压缩包内容',
    url: '',
    mimeType: file.mime_type,
    filename: file.name,
    rows: [
      ['路径', file.path],
      ['格式', file.file_type || file.mime_type || '文件'],
      ['大小', formatSize(file.file_size)],
    ],
    emptyLabel: canPreview ? '正在加载预览…' : '该格式暂不能在线预览，可下载后查看',
    download: () => downloadArchiveVirtualFile(file),
  })
  if (!canPreview) return
  const source = archiveView.value?.source
  if (!source) {
    previewEmptyLabel.value = '压缩包来源已关闭，请重新打开'
    return
  }
  try {
    const seq = ++archivePreviewRequestSeq
    const objectUrl = await createArchiveEntryObjectUrl(source.id, file)
    if (seq !== archivePreviewRequestSeq || !previewOpen.value) {
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

async function downloadArchiveVirtualFile(file: ArchiveVirtualFile) {
  const source = archiveView.value?.source
  if (!source) return
  await downloadArchiveEntryBlob(source.id, file)
}

function canPreviewArchiveVirtualFile(file: ArchiveVirtualFile) {
  if (!file.preview_url) return false
  const mime = (file.mime_type || '').toLowerCase()
  if (mime.startsWith('image/') || mime.startsWith('video/') || mime === 'application/pdf') return true
  const ext = (file.name.split('.').pop() || '').toLowerCase()
  return ['jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp', 'svg', 'tif', 'tiff', 'pdf', 'mp4', 'webm', 'mov', 'm4v'].includes(ext)
}

function clearMaterialResultSnapshot(options: { clearFolders?: boolean } = {}) {
  materialItems.value = []
  materialFileTotal.value = 0
  materialPage.value = 1
  selectedMaterialIds.value = new Set()
  activeMaterial.value = null
  if (!options.clearFolders) return
  materialKnownFolders.value = {}
  selectedMaterialFolderPath.value = ''
  expandedMaterialFolderPaths.value = new Set([''])
}

function resetMaterialScopeSnapshot() {
  materialAbortController?.abort()
  materialAbortController = null
  materialRequestSeq += 1
  materialLoading.value = false
  materialLoadingMore.value = false
  materialError.value = ''
  clearMaterialResultSnapshot({ clearFolders: true })
}

async function loadMaterials(query = materialQuery.value, options: { append?: boolean } = {}) {
  if (!canUseOperational.value) return
  const nextQuery = query.trim()
  if (materialBusinessLaneFilter.value !== 'all' && materialSourceFilter.value !== 'system') {
    materialSourceFilter.value = 'system'
  }
  if (canManageDrive.value && !nextQuery) {
    await loadMaterialFolder(materialDefaultFolderPathForSource())
    return
  }
  const append = options.append === true
  const requestID = ++materialRequestSeq
  materialAbortController?.abort()
  materialAbortController = new AbortController()
  if (append) materialLoadingMore.value = true
  else materialLoading.value = true
  materialError.value = ''
  materialQuery.value = nextQuery
  const page = append ? materialPage.value + 1 : 1
  if (!append) {
    clearMaterialResultSnapshot({ clearFolders: true })
  }
  try {
    if (canManageDrive.value) {
      const [systemResult, published] = await Promise.all([
        assetWorkbenchApi.systemSearch({
          q: materialQuery.value,
          source: materialSourceFilter.value,
          format_category: materialFormatFilter.value,
          business_lane: materialRequestBusinessLane(),
          page,
          page_size: materialPageSize,
        }, materialAbortController.signal),
        assetWorkbenchApi.listClientMaterials(true, materialAbortController.signal),
      ])
      if (requestID !== materialRequestSeq) return
      const visibleItems = systemResult.items.filter(materialMatchesActiveFilters)
      materialItems.value = append ? mergeMaterialItems(materialItems.value, visibleItems) : visibleItems
      materialFileTotal.value = Number(systemResult.total || visibleItems.length)
      materialPage.value = systemResult.page || page
      clientMaterials.value = published
      for (const asset of visibleItems) {
        rememberMaterialPath(materialDirectoryPath(asset))
      }
    } else {
      const published = await assetWorkbenchApi.listClientMaterials(false)
      clientMaterials.value = published
      const q = materialQuery.value.toLowerCase()
      materialItems.value = published.map(materialFromClient).filter((asset) => {
        if (!materialMatchesActiveFilters(asset)) return false
        if (!q) return true
        return [titleOf(asset), asset.original_filename, asset.resource_id, asset.scope_sku_code, asset.source_label]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()
          .includes(q)
      })
      materialFileTotal.value = materialItems.value.length
      materialPage.value = 1
    }
  } catch (err) {
    if (requestID !== materialRequestSeq || isAbortError(err)) return
    if (!append) {
      materialItems.value = []
      materialFileTotal.value = 0
    }
    const message = err instanceof Error ? err.message : '运营素材加载失败'
    if (append) actionError.value = message
    else materialError.value = message
  } finally {
    if (requestID === materialRequestSeq) {
      materialLoading.value = false
      materialLoadingMore.value = false
    }
  }
}

async function loadMaterialFolder(path = selectedMaterialFolderPath.value, options: { expandTree?: boolean; append?: boolean } = {}) {
  if (!canUseOperational.value) return
  const expandTree = options.expandTree !== false
  const append = options.append === true
  const normalized = normalizeVirtualPath(path)
  if (!operationalMaterialPathVisible(normalized)) {
    await loadMaterialFolder('/quark', { expandTree, append: false })
    return
  }
  const requestID = ++materialRequestSeq
  materialAbortController?.abort()
  materialAbortController = new AbortController()
  if (append) materialLoadingMore.value = true
  else materialLoading.value = true
  materialError.value = ''
  const page = append ? materialPage.value + 1 : 1
  if (!append) {
    clearMaterialResultSnapshot()
    materialQuery.value = ''
    selectedMaterialFolderPath.value = normalized
    materialSourceFilter.value = materialRequestSourceForPath(normalized)
  }
  rememberMaterialPath(normalized)
  if (expandTree) expandMaterialFolderTreePath(normalized)
  try {
    if (canManageDrive.value) {
      const source = materialRequestSourceForPath(normalized)
      const [browse, published] = await Promise.all([
        assetWorkbenchApi.browseMaterials({
          path: normalized,
          source,
          format_category: materialFormatFilter.value,
          business_lane: materialRequestBusinessLane(),
          page,
          page_size: materialPageSize,
        }, materialAbortController.signal),
        assetWorkbenchApi.listClientMaterials(true, materialAbortController.signal),
      ])
      if (requestID !== materialRequestSeq) return
      selectedMaterialFolderPath.value = normalizeVirtualPath(browse.path || normalized)
      rememberMaterialFolders(browse.folders || [])
      if (expandTree) expandMaterialFolderTreePath(selectedMaterialFolderPath.value)
      if (selectedMaterialFolderPath.value) {
        const childCount = (browse.folders || []).reduce((sum, folder) => sum + Number(folder.file_count || 0), 0)
        rememberMaterialFolder({
          path: selectedMaterialFolderPath.value,
          name: materialFolderLabel(selectedMaterialFolderPath.value),
          file_count: Number(browse.total || 0) + childCount,
          direct_file_count: Number(browse.total || 0),
        })
      }
      const visibleFiles = (browse.files || []).filter(materialMatchesActiveFilters)
      materialItems.value = append ? mergeMaterialItems(materialItems.value, visibleFiles) : visibleFiles
      materialFileTotal.value = Number(browse.total || visibleFiles.length)
      materialPage.value = browse.page || page
      clientMaterials.value = published
    } else {
      const published = await assetWorkbenchApi.listClientMaterials(false, materialAbortController.signal)
      if (requestID !== materialRequestSeq) return
      clientMaterials.value = published
      materialItems.value = published.map(materialFromClient).filter(materialMatchesActiveFilters)
      materialFileTotal.value = materialItems.value.length
      materialPage.value = 1
    }
  } catch (err) {
    if (requestID !== materialRequestSeq || isAbortError(err)) return
    if (!append) {
      materialItems.value = []
      materialFileTotal.value = 0
    }
    const message = err instanceof Error ? err.message : '运营素材加载失败'
    if (append) actionError.value = message
    else materialError.value = message
  } finally {
    if (requestID === materialRequestSeq) {
      materialLoading.value = false
      materialLoadingMore.value = false
    }
  }
}

function loadMoreMaterials() {
  if (!materialCanLoadMore.value) return
  if (materialQuery.value.trim()) {
    void loadMaterials(materialQuery.value, { append: true })
    return
  }
  void loadMaterialFolder(selectedMaterialFolderPath.value, { expandTree: false, append: true })
}

function refreshMaterialsForFilters() {
  materialSourceFilter.value = normalizeMaterialSourceFilter(materialSourceFilter.value)
  materialFormatFilter.value = normalizeMaterialFormatFilter(materialFormatFilter.value)
  materialBusinessLaneFilter.value = normalizeMaterialBusinessLaneFilter(materialBusinessLaneFilter.value)
  if (materialBusinessLaneFilter.value !== 'all' && materialSourceFilter.value !== 'system') {
    materialSourceFilter.value = 'system'
  }
  resetMaterialScopeSnapshot()
  if (materialQuery.value.trim()) {
    void loadMaterials(materialQuery.value)
    return
  }
  void loadMaterialFolder(materialDefaultFolderPathForSource())
}

function clearMaterialSearch() {
  materialQuery.value = ''
  resetMaterialScopeSnapshot()
  void loadMaterialFolder(materialDefaultFolderPathForSource())
}

async function openMaterialPreview(asset: SystemAssetRow) {
  activeMaterial.value = asset
  const key = materialAssetKey(asset)
  const loading = new Set(materialPreviewLoadingIds.value)
  loading.add(key)
  materialPreviewLoadingIds.value = loading
  openPreviewDialog({
    title: materialDisplayTitle(asset),
    eyebrow: sourceLabelOf(asset),
    emptyLabel: '正在加载预览…',
    mimeType: asset.mime_type,
    filename: asset.original_filename || asset.file_name,
    rows: materialPreviewRows(asset, null),
    download: () => downloadMaterial(asset),
  })
  try {
    let meta = await previewMaterial(asset)
    if (asset.material_id && previewIsPreparing(meta)) {
      previewEmptyLabel.value = '正在生成预览，完成后会自动显示'
      meta = await waitForPreparedPreview(meta, () => assetWorkbenchApi.previewClientMaterial(asset.material_id as number))
    }
    const url = meta.preview_url || meta.download_url || ''
    if (url) {
      materialPreviewUrls.value = { ...materialPreviewUrls.value, [key]: url }
      previewUrl.value = url
      previewEmptyLabel.value = ''
    } else {
      previewEmptyLabel.value = '当前素材只能下载'
    }
    previewRows.value = materialPreviewRows(asset, meta)
  } catch (err) {
    previewEmptyLabel.value = err instanceof Error ? err.message : '预览加载失败'
  } finally {
    const next = new Set(materialPreviewLoadingIds.value)
    next.delete(key)
    materialPreviewLoadingIds.value = next
  }
}

async function previewMaterial(asset: SystemAssetRow): Promise<SystemAssetPreviewMeta> {
  if (asset.material_id) return assetWorkbenchApi.previewClientMaterial(asset.material_id)
  return assetWorkbenchApi.previewMaterialAsset(asset)
}

function downloadMaterial(asset: SystemAssetRow) {
  actionError.value = ''
  const result = queueMaterial(asset)
  notice.value = result.duplicate
    ? '这个素材已在下载中心，无需重复点击'
    : '已加入下载中心，准备完成后会自动下载'
}

function materialPreviewRows(asset: SystemAssetRow, meta: SystemAssetPreviewMeta | null): Array<[string, string]> {
  return [
    ['来源', sourceLabelOf(asset)],
    ['资源ID', asset.resource_id || String(asset.id)],
    ['SKU', asset.scope_sku_code || asset.sku_code || asset.primary_sku_code || '—'],
    ['文件名', meta?.filename || asset.original_filename || asset.file_name || '—'],
    ['类型', meta?.mime_type || asset.mime_type || '—'],
  ]
}

function selectMaterial(asset: SystemAssetRow) {
  activeMaterial.value = asset
  void ensureMaterialPreview(asset)
}

function selectClientMaterial(material: ClientMaterialRow) {
  const materialKeys = clientMaterialIdentityKeys(material)
  const existing = materialItems.value.find((asset) => hasSharedIdentity(materialIdentityKeys(asset), materialKeys))
  searchActive.value = false
  activeMode.value = 'operational'
  clientMaterialManagerOpen.value = false
  selectMaterial(existing ? materialWithClientPublication(existing, material) : materialFromClient(material))
}

async function refreshClientMaterials(options: { silent?: boolean } = {}) {
  if (!canManageDrive.value || (clientMaterialLoading.value && !options.silent)) return
  if (!options.silent) clientMaterialLoading.value = true
  clientMaterialError.value = ''
  try {
    clientMaterials.value = await assetWorkbenchApi.listClientMaterials(true)
    syncActiveMaterialPublication()
  } catch (err) {
    clientMaterialError.value = err instanceof Error ? err.message : '客户端素材加载失败'
  } finally {
    if (!options.silent) clientMaterialLoading.value = false
  }
}

function openClientMaterialManager() {
  clientMaterialManagerOpen.value = true
  void refreshClientMaterials()
}

function batchJobStatusLabel(status?: string): string {
  switch (status) {
    case 'queued':
      return '排队中'
    case 'running':
      return '处理中'
    case 'succeeded':
      return '已完成'
    case 'failed':
      return '处理失败'
    case 'cancelled':
      return '已取消'
    default:
      return '未知状态'
  }
}

function batchJobActionLabel(action?: string): string {
  switch (action) {
    case 'publish':
      return '批量上架'
    case 'enable':
      return '批量启用'
    case 'disable':
      return '批量停用'
    case 'remove':
      return '批量下架'
    default:
      return '批量任务'
  }
}

function batchJobProgressLabel(job: AssetWorkbenchBatchJob): string {
  const total = job.total_count || job.processed_count || 0
  const processed = job.processed_count || (job.status === 'succeeded' ? total : 0)
  if (!total) return batchJobStatusLabel(job.status)
  return `${processed}/${total}`
}

function batchJobSummary(job: AssetWorkbenchBatchJob): string {
  if (job.status === 'queued') return '任务已提交，等待后台处理'
  if (job.status === 'running') return `正在处理 ${batchJobProgressLabel(job)}`
  if (job.status === 'failed') return job.error_message || '处理失败，请稍后重试'
  const success = (job.created_count || 0) + (job.updated_count || 0) + (job.removed_count || 0)
  return `成功 ${success} 项，跳过 ${job.skipped_count || 0} 项，失败 ${job.failed_count || 0} 项`
}

async function refreshBatchJobs(silent = false) {
  if (!canManageDrive.value) return
  if (!silent) batchJobsLoading.value = true
  batchJobsError.value = ''
  const hadActiveJobs = activeBatchJobCount.value > 0
  try {
    const result = await assetWorkbenchApi.listBatchJobs({ page: 1, page_size: 20 })
    batchJobs.value = result.items || []
    if (activeBatchJobCount.value > 0) scheduleBatchJobPolling()
    else {
      stopBatchJobPolling()
      if (hadActiveJobs) {
        await refreshClientMaterials({ silent: true })
      }
    }
  } catch (err) {
    batchJobsError.value = err instanceof Error ? err.message : '批量任务加载失败'
  } finally {
    if (!silent) batchJobsLoading.value = false
  }
}

function openBatchJobPanel() {
  batchJobPanelOpen.value = true
  void refreshBatchJobs(false)
}

function scheduleBatchJobPolling() {
  if (batchJobPollTimer !== null) return
  batchJobPollTimer = window.setTimeout(async () => {
    batchJobPollTimer = null
    if (!batchJobPanelOpen.value && activeBatchJobCount.value === 0) return
    await refreshBatchJobs(true)
  }, 3000)
}

function stopBatchJobPolling() {
  if (batchJobPollTimer === null) return
  window.clearTimeout(batchJobPollTimer)
  batchJobPollTimer = null
}

function cacheMaterialPreview(key: string, url: string) {
  if (!key || !url || materialPreviewUrls.value[key]) return
  materialPreviewUrls.value = { ...materialPreviewUrls.value, [key]: url }
}

function ensureMaterialPreview(asset: SystemAssetRow) {
  const key = materialAssetKey(asset)
  if (materialPreviewUrls.value[key]) return
  const inline = resolvedSystemAssetThumbnailUrl(asset)
  if (inline) {
    cacheMaterialPreview(key, inline)
  }
}

const activeMaterialPreviewUrl = computed(() => {
  const asset = activeMaterial.value
  if (!asset) return ''
  return materialPreviewUrls.value[materialAssetKey(asset)] || resolvedSystemAssetThumbnailUrl(asset) || ''
})

const activeMaterialPreviewLoading = computed(() => {
  const asset = activeMaterial.value
  if (!asset) return false
  return materialPreviewLoadingIds.value.has(materialAssetKey(asset)) && !activeMaterialPreviewUrl.value
})

function materialCodeOf(asset: SystemAssetRow): string {
  if (asset.source_type === 'external') return asset.resource_id || `ext-${asset.id}`
  const sku = asset.scope_sku_code || asset.sku_code || asset.primary_sku_code
  return sku ? `SKU ${sku}` : asset.resource_id || `素材 ${asset.id}`
}

function materialTypeLabel(asset: SystemAssetRow): string {
  const mime = (asset.mime_type || '').toLowerCase()
  if (mime.includes('photoshop') || mime.includes('vnd.adobe')) return 'PSD'
  if (mime.includes('pdf')) return 'PDF'
  if (mime.startsWith('image/')) return mime.replace('image/', '').toUpperCase()
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return '压缩包'
  return asset.mime_type || '文件'
}

function toggleMaterial(asset: SystemAssetRow, checked: boolean) {
  const key = materialAssetKey(asset)
  const next = new Set(selectedMaterialIds.value)
  if (checked) next.add(key)
  else next.delete(key)
  selectedMaterialIds.value = next
  activeMaterial.value = asset
}

function publishPayloadForMaterial(asset: SystemAssetRow) {
  const resourceID = materialResourceID(asset)
  const sourceType = normalizeMaterialSourceType(asset.source_type, resourceID)
  const payload = {
    asset_id: asset.id,
    source_type: sourceType,
    title: materialDisplayTitle(asset),
    description: '',
    enabled: true,
    sort_order: clientMaterials.value.length + 1,
  } as const
  if (sourceType === 'external') {
    return {
      ...payload,
      source_ref: resourceID,
      resource_id: resourceID,
    }
  }
  return {
    ...payload,
    source_ref: String(asset.id),
    resource_id: String(asset.id),
  }
}

async function publishClientMaterial(asset: SystemAssetRow) {
  if (publishingClientMaterial.value) return
  publishingClientMaterial.value = true
  actionError.value = ''
  try {
    const created = await assetWorkbenchApi.createClientMaterial(publishPayloadForMaterial(asset))
    clientMaterials.value = [
      created,
      ...clientMaterials.value.filter((material) => material.id !== created.id),
    ]
    const publishedAsset = materialWithClientPublication(asset, created)
    upsertMaterialItem(publishedAsset)
    activeMaterial.value = publishedAsset
    notice.value = `已上架到客户端：${created.title || created.filename_snapshot || materialDisplayTitle(asset)}`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '客户端素材上架失败'
  } finally {
    publishingClientMaterial.value = false
  }
}

function selectAllVisibleMaterials() {
  selectedMaterialIds.value = new Set(visibleMaterialFiles.value.map((asset) => materialAssetKey(asset)))
}

function clearSelectedMaterials() {
  selectedMaterialIds.value = new Set()
}

async function batchUpdateSelectedClientMaterials(action: 'publish' | 'disable' | 'remove') {
  if (batchUpdatingClientMaterials.value || selectedMaterialAssets.value.length === 0) return
  batchUpdatingClientMaterials.value = true
  actionError.value = ''
  try {
    const result = await assetWorkbenchApi.batchUpdateClientMaterials({
      action,
      items: selectedMaterialAssets.value.map(publishPayloadForMaterial),
      selection_scope: 'selected',
    })
    await finishClientMaterialBatch(action, result)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '批量处理客户端素材失败'
  } finally {
    batchUpdatingClientMaterials.value = false
  }
}

async function batchPublishCurrentMaterialFolder(includeChildren: boolean) {
  if (batchUpdatingClientMaterials.value) return
  batchUpdatingClientMaterials.value = true
  actionError.value = ''
  try {
    const result = await assetWorkbenchApi.batchUpdateClientMaterials({
      action: 'publish',
      folders: [{
        path: selectedMaterialFolderPath.value,
        source: materialRequestSourceForPath(selectedMaterialFolderPath.value),
        format_category: materialFormatFilter.value,
        business_lane: materialRequestBusinessLane(),
        include_children: includeChildren,
      }],
      format_category: materialFormatFilter.value,
      business_lane: materialRequestBusinessLane(),
      selection_scope: includeChildren ? 'current_folder_recursive' : 'current_folder',
    })
    await finishClientMaterialBatch('publish', result)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '批量上架当前目录失败'
  } finally {
    batchUpdatingClientMaterials.value = false
  }
}

async function finishClientMaterialBatch(action: 'publish' | 'disable' | 'remove', result: Awaited<ReturnType<typeof assetWorkbenchApi.batchUpdateClientMaterials>>) {
  if (result.async_required) {
    if (result.job) {
      batchJobs.value = [result.job, ...batchJobs.value.filter((job) => job.job_id !== result.job?.job_id)]
    }
    batchJobPanelOpen.value = true
    notice.value = result.message || '选择范围较大，已转入批量任务中心后台处理。'
    actionError.value = ''
    await refreshBatchJobs(true)
    scheduleBatchJobPolling()
    return
  }
  clientMaterials.value = await assetWorkbenchApi.listClientMaterials(true)
  syncActiveMaterialPublication()
  if (action === 'remove') {
    activeMaterial.value = activeMaterial.value ? { ...activeMaterial.value, material_id: undefined } : null
  } else {
    syncActiveMaterialPublication()
  }
  const actionLabel = action === 'publish' ? '上架' : action === 'disable' ? '停用' : '下架'
  notice.value = `${actionLabel}完成：请求 ${result.requested} 项，成功 ${result.created + result.updated + result.removed} 项，跳过 ${result.skipped} 项，失败 ${result.failed} 项。`
  if (result.failed > 0) {
    actionError.value = `有 ${result.failed} 项未处理成功，请检查素材是否仍存在或是否已经上架。`
  }
  clearSelectedMaterials()
}

async function toggleClientMaterial(material: ClientMaterialRow) {
  try {
    const updated = await assetWorkbenchApi.updateClientMaterial(material.id, { enabled: !material.enabled })
    clientMaterials.value = clientMaterials.value.map((row) => (row.id === updated.id ? updated : row))
    if (activeMaterial.value && hasSharedIdentity(materialIdentityKeys(activeMaterial.value), clientMaterialIdentityKeys(updated))) {
      activeMaterial.value = materialWithClientPublication(activeMaterial.value, updated)
    }
    notice.value = `${updated.enabled ? '已启用' : '已停用'}客户端素材：${updated.title || updated.filename_snapshot}`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '客户端素材状态更新失败'
  }
}

async function removeClientMaterial(material: ClientMaterialRow) {
  try {
    const wasActive = activeMaterial.value
      ? hasSharedIdentity(materialIdentityKeys(activeMaterial.value), clientMaterialIdentityKeys(material))
      : false
    await assetWorkbenchApi.deleteClientMaterial(material.id)
    clientMaterials.value = clientMaterials.value.filter((row) => row.id !== material.id)
    if (wasActive) {
      activeMaterial.value = activeMaterial.value ? { ...activeMaterial.value, material_id: undefined } : { ...materialFromClient(material), material_id: undefined }
    }
    notice.value = `已从客户端下架：${material.title || material.filename_snapshot}`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '客户端素材下架失败'
  }
}

function openContextMenu(event: MouseEvent, state: ContextMenuInput) {
  contextMenu.value = { ...state, x: event.clientX, y: event.clientY } as ContextMenuState
  void nextTick(() => positionContextMenu(event.clientX, event.clientY))
}

function positionContextMenu(anchorX: number, anchorY: number) {
  const menu = contextMenuRef.value
  if (!contextMenu.value || !menu) return
  const pad = 8
  const rect = menu.getBoundingClientRect()
  let x = anchorX
  let y = anchorY
  if (x + rect.width + pad > window.innerWidth) x = Math.max(pad, window.innerWidth - rect.width - pad)
  if (y + rect.height + pad > window.innerHeight) y = Math.max(pad, window.innerHeight - rect.height - pad)
  contextMenu.value = { ...contextMenu.value, x, y } as ContextMenuState
}

function closeContextMenu() {
  contextMenu.value = null
}

function fileFromContext() {
  return contextMenu.value?.kind === 'file' ? contextMenu.value.file : null
}

function directoryFromContext() {
  return contextMenu.value?.kind === 'directory' ? contextMenu.value.dir : null
}

function previewContextFile() {
  const file = fileFromContext()
  if (!file) return
  closeContextMenu()
  void openFilePreview(file)
}

function downloadContextFile() {
  const file = fileFromContext()
  if (!file) return
  closeContextMenu()
  void downloadFile(file)
}

function selectContextFile() {
  const file = fileFromContext()
  if (!file) return
  selectFile(file, true)
  closeContextMenu()
}

function downloadContextSelection() {
  const file = fileFromContext()
  if (!file) return
  selectFile(file, true)
  closeContextMenu()
  void downloadSelectedFiles()
}

function openContextArchiveFile() {
  const file = fileFromContext()
  if (!file) return
  void openArchiveFile(file)
}

function selectContextFileForDelete() {
  const file = fileFromContext()
  if (!file) return
  selectFile(file, true)
  notice.value = '已加入多选，可在上方操作栏删除'
  closeContextMenu()
}

function syncEditForm(file: DriveFileRow | null) {
  itemEditForm.value = {
    difficulty_class: file?.difficulty_class || currentDirRow.value?.difficulty_class || difficultyOptions.value[0] || '',
    page_count: file?.page_count || 1,
  }
}

watch(selectedFile, syncEditForm)
watch(activeMode, (mode, previousMode) => {
  if (mode !== 'operational' || previousMode === 'operational' || suppressMaterialAutoload.value) return
  resetMaterialScopeSnapshot()
  if (materialQuery.value.trim()) {
    void loadMaterials(materialQuery.value)
    return
  }
  void loadMaterialFolder(materialDefaultFolderPathForSource())
})
watch(
  () => [route.query.q, route.query.scope] as const,
  ([nextQuery, nextScope], [previousQuery, previousScope]) => {
    if (nextQuery === previousQuery && nextScope === previousScope) return
    const q = routeQueryString(nextQuery).trim()
    if (!q) {
      if (searchActive.value || searchQuery.value) resetSearchState(false)
      return
    }
    searchQuery.value = q
    searchScope.value = normalizeScope(nextScope)
    void runUnifiedSearch()
  },
)

onMounted(async () => {
  await Promise.all([loadDifficultyClasses(), loadDirectories()])
  if (hasQueryValue(route.query.scope) && normalizeScope(route.query.scope) === 'operational') {
    activeMode.value = 'operational'
  }
  const locateId = Number(route.query.file_id || route.query.locate)
  if (locateId > 0) {
    try {
      const file = await assetWorkbenchApi.driveLocate(locateId)
      await revealFile(file)
    } catch {
      /* ignore stale locate links */
    }
  }
  const initialQuery = routeQueryString(route.query.q).trim()
  if (initialQuery) {
    searchQuery.value = initialQuery
    searchScope.value = normalizeScope(route.query.scope)
    await runUnifiedSearch()
  }
  if (activeMode.value === 'operational') await loadMaterials(initialQuery)
  window.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
  abortDriveRequests()
  cancelSearchDebounce()
  stopBatchJobPolling()
  revokeArchivePreviewObjectUrl()
  if (directoryClickTimer) {
    window.clearTimeout(directoryClickTimer)
    directoryClickTimer = null
  }
  window.removeEventListener('click', closeContextMenu)
})
</script>

<template>
  <section class="aw-drive aw-token-scope" @click="closeContextMenu">
    <header class="aw-drive__toolbar">
      <div class="aw-drive__title">
        <HardDrive :size="20" aria-hidden="true" />
        <div>
          <p class="aw-eyebrow">素材网盘</p>
          <h2>素材网盘</h2>
        </div>
      </div>
      <form class="aw-drive__search aw-drive__search--global" @submit.prevent="runUnifiedSearch">
        <span class="aw-drive__search-icon" aria-hidden="true">
          <IconfontActionIcon name="search" :size="17" />
        </span>
        <span class="aw-drive__scope-select">
          <select v-model="searchScope" aria-label="搜索范围" @change="scheduleUnifiedSearch">
            <option value="all">全部</option>
            <option value="operational">运营素材</option>
            <option value="files">上传文件</option>
          </select>
          <ChevronDown :size="15" aria-hidden="true" />
        </span>
        <span class="aw-drive__search-divider" aria-hidden="true"></span>
        <input
          v-model="searchQuery"
          type="search"
          placeholder="搜索运营素材、文件名、上传目录"
          @input="scheduleUnifiedSearch"
        />
        <span v-if="searchLoading" class="aw-drive__search-state">搜索中</span>
        <button v-if="searchActive" class="aw-drive__search-clear" type="button" aria-label="清除搜索" @click="clearSearch">
          <IconfontActionIcon name="close" :size="14" />
        </button>
      </form>
      <div v-if="canManageDrive" class="aw-drive__toolbar-actions">
        <button class="aw-secondary-button aw-client-material-button" type="button" @click="openBatchJobPanel">
          <HardDrive :size="16" aria-hidden="true" />
          <span>批量任务</span>
          <span v-if="activeBatchJobCount" class="aw-client-material-button__count">{{ activeBatchJobCount }} 处理中</span>
        </button>
        <button class="aw-secondary-button aw-client-material-button" type="button" @click="openClientMaterialManager">
          <ImageDown :size="16" aria-hidden="true" />
          <span>客户端素材</span>
          <span class="aw-client-material-button__count">{{ enabledClientMaterialCount }}/{{ clientMaterials.length }}</span>
        </button>
      </div>
    </header>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="actionError" class="aw-inline-alert aw-inline-alert--error">{{ actionError }}</p>

    <div
      v-if="batchJobPanelOpen"
      class="aw-client-material-manager"
      role="dialog"
      aria-modal="true"
      aria-label="批量任务"
      @click.self="batchJobPanelOpen = false"
    >
      <section class="aw-client-material-manager__panel">
        <header class="aw-client-material-manager__head">
          <div>
            <p class="aw-eyebrow">批量任务</p>
            <h3>后台处理进度</h3>
            <span>{{ activeBatchJobCount ? `${activeBatchJobCount} 个任务正在处理` : '暂无正在处理的任务' }}</span>
          </div>
          <button class="aw-drive-mini-button" type="button" aria-label="关闭批量任务" @click="batchJobPanelOpen = false">
            <IconfontActionIcon name="close" :size="14" />
          </button>
        </header>
        <div class="aw-client-material-manager__tools">
          <p>大批量上架、停用或下架会在这里持续更新，完成后可刷新客户端素材查看结果。</p>
          <button class="aw-grid-button" type="button" :disabled="batchJobsLoading" @click="refreshBatchJobs(false)">
            {{ batchJobsLoading ? '刷新中…' : '刷新' }}
          </button>
        </div>
        <p v-if="batchJobsError" class="aw-inline-alert aw-inline-alert--error">{{ batchJobsError }}</p>
        <p v-else-if="batchJobsLoading && batchJobs.length === 0" class="aw-drive-empty">正在加载批量任务…</p>
        <div v-else-if="batchJobs.length" class="aw-client-material-manager__list">
          <article v-for="job in batchJobs" :key="job.job_id" class="aw-client-material-row">
            <div class="aw-client-material-row__main">
              <strong>{{ batchJobActionLabel(job.action) }}</strong>
              <span>{{ batchJobSummary(job) }}</span>
              <small>{{ batchJobProgressLabel(job) }} · {{ formatDateTime(job.created_at) }}</small>
            </div>
            <span class="aw-chip" :class="{ 'aw-chip--success': job.status === 'succeeded', 'aw-chip--danger': job.status === 'failed' }">
              {{ batchJobStatusLabel(job.status) }}
            </span>
          </article>
        </div>
        <p v-else class="aw-drive-empty">暂无批量任务。</p>
      </section>
    </div>

    <div
      v-if="clientMaterialManagerOpen"
      class="aw-client-material-manager"
      role="dialog"
      aria-modal="true"
      aria-label="客户端素材管理"
      @click.self="clientMaterialManagerOpen = false"
    >
      <section class="aw-client-material-manager__panel">
        <header class="aw-client-material-manager__head">
          <div>
            <p class="aw-eyebrow">客户端素材</p>
            <h3>发布管理</h3>
            <span>{{ enabledClientMaterialCount }} 个上架中 · {{ disabledClientMaterialCount }} 个已停用</span>
          </div>
          <button class="aw-drive-mini-button" type="button" aria-label="关闭客户端素材管理" @click="clientMaterialManagerOpen = false">
            <IconfontActionIcon name="close" :size="14" />
          </button>
        </header>
        <div class="aw-client-material-manager__toolbar">
          <div class="aw-segmented-control" aria-label="客户端素材状态筛选">
            <button
              type="button"
              :class="{ 'is-active': clientMaterialFilter === 'all' }"
              @click="clientMaterialFilter = 'all'"
            >
              全部 {{ clientMaterials.length }}
            </button>
            <button
              type="button"
              :class="{ 'is-active': clientMaterialFilter === 'enabled' }"
              @click="clientMaterialFilter = 'enabled'"
            >
              上架中 {{ enabledClientMaterialCount }}
            </button>
            <button
              type="button"
              :class="{ 'is-active': clientMaterialFilter === 'disabled' }"
              @click="clientMaterialFilter = 'disabled'"
            >
              已停用 {{ disabledClientMaterialCount }}
            </button>
          </div>
          <button class="aw-grid-button" type="button" :disabled="clientMaterialLoading" @click="() => refreshClientMaterials()">
            {{ clientMaterialLoading ? '刷新中…' : '刷新' }}
          </button>
        </div>
        <p v-if="clientMaterialError" class="aw-inline-alert aw-inline-alert--error">{{ clientMaterialError }}</p>
        <p v-else-if="clientMaterialLoading && clientMaterials.length === 0" class="aw-drive-empty">正在加载客户端素材…</p>
        <div v-else-if="visibleClientMaterials.length" class="aw-client-materials-list">
          <article v-for="material in visibleClientMaterials" :key="material.id" class="aw-client-material-row">
            <div class="aw-client-material-row__main">
              <strong :title="material.title || material.filename_snapshot">{{ material.title || material.filename_snapshot }}</strong>
              <span>{{ material.source_label || material.source_type || '系统资源' }} · {{ material.resource_id || material.source_ref || material.asset_id }}</span>
            </div>
            <span class="aw-chip" :class="material.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">
              {{ material.enabled ? '上架中' : '已停用' }}
            </span>
            <div class="aw-inline-actions aw-inline-actions--compact">
              <button class="aw-grid-button" type="button" @click="selectClientMaterial(material)">查看</button>
              <button class="aw-grid-button" type="button" @click="toggleClientMaterial(material)">{{ material.enabled ? '停用' : '启用' }}</button>
              <button class="aw-grid-button" type="button" @click="removeClientMaterial(material)">下架</button>
            </div>
          </article>
        </div>
        <p v-else class="aw-copy">
          {{ clientMaterials.length ? '当前筛选下没有客户端素材。' : '还没有发布给客户端的素材。' }}
        </p>
      </section>
    </div>

    <section v-if="searchActive" class="aw-drive-search-results">
      <div class="aw-drive-search-results__head">
        <span>统一检索「{{ searchQuery }}」</span>
        <span class="aw-drive-search-results__count">{{ searchLoading ? '搜索中…' : `共 ${searchTotal} 条` }}</span>
      </div>
      <p v-if="searchError" class="aw-drive-empty">{{ searchError }}</p>
      <p v-else-if="!searchLoading && searchResults.length === 0" class="aw-drive-empty">没有匹配内容</p>
      <ul v-else class="aw-drive-hit-list">
        <li v-for="hit in searchResults" :key="`${hit.source}-${hit.id}`" class="aw-drive-hit">
          <button
            class="aw-drive-hit__thumb"
            type="button"
            :aria-label="`定位 ${hit.title || hit.primary_code || '搜索结果'}`"
            @click="locateSearchRow(hit)"
          >
            <MaterialListThumb
              v-if="isOperationalSearchHit(hit)"
              :asset="searchHitMaterial(hit)"
              :cached-url="materialPreviewUrls[materialAssetKey(searchHitMaterial(hit))]"
            />
            <DriveThumb
              v-else-if="searchHitDriveFileID(hit)"
              :file-id="searchHitDriveFileID(hit)"
              :filename="searchHitFilename(hit) || hit.title || hit.primary_code"
              :mime-type="searchHitMimeType(hit)"
              :preview-status="searchHitPreviewStatus(hit)"
            />
            <ImageDown v-else-if="hit.scope === 'operational'" :size="24" aria-hidden="true" />
            <FileDown v-else :size="24" aria-hidden="true" />
          </button>
          <div class="aw-drive-hit__body">
            <strong class="aw-drive-hit__title">
              <span class="aw-drive-hit__title-text">{{ hit.title || hit.primary_code }}</span>
              <span class="aw-drive-hit__format-chip">{{ searchHitFormatLabel(hit) }}</span>
            </strong>
            <span class="aw-drive-hit__path">
              {{ searchHitContextLabel(hit) }}
            </span>
            <small>{{ searchHitSecondaryLabel(hit) }}</small>
          </div>
          <div class="aw-drive-hit__actions">
            <button class="aw-secondary-button" type="button" @click="locateSearchRow(hit)">
              <WorkbenchFolderIcon :size="14" variant="starFilled" />
              在网盘中定位
            </button>
          </div>
        </li>
      </ul>
    </section>

    <div v-show="!searchActive" class="aw-drive__shell" :class="{ 'has-drawer': detailOpen }">
      <nav class="aw-drive-side" aria-label="网盘导航">
        <section class="aw-drive-side__group">
          <p class="aw-drive-side__label">
            <WorkbenchFolderIcon :size="15" variant="star" />
            <span>上传目录</span>
            <button v-if="canManageDrive" class="aw-drive-mini-button" type="button" aria-label="新建上传目录" @click="creatingDirectory = true">
              <IconfontActionIcon name="add" :size="14" />
            </button>
          </p>
          <form v-if="creatingDirectory" class="aw-drive-inline-form" @submit.prevent="createDirectory">
            <input v-model.trim="newDirectoryName" placeholder="目录名称" aria-label="目录名称" />
            <select v-model="newDirectoryDifficulty" aria-label="难度">
              <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
            </select>
            <input v-model.trim="newDirectoryFileTypes" placeholder="允许格式，不填则不限" aria-label="允许上传格式" />
            <div class="aw-drive-inline-form__actions">
              <button class="aw-grid-button aw-grid-button--strong" type="submit">创建</button>
              <button class="aw-grid-button" type="button" @click="creatingDirectory = false">取消</button>
            </div>
          </form>
          <div class="aw-drive-side__list">
            <p v-if="dirLoading" class="aw-drive-empty">加载中…</p>
            <p v-else-if="dirError" class="aw-drive-empty">{{ dirError }}</p>
            <p v-else-if="directories.length === 0" class="aw-drive-empty">暂无上传目录</p>
            <div
              v-for="dir in directories"
              :key="dirKey(dir)"
              class="aw-drive-column__item-wrap"
              @dragover.prevent
              @drop.prevent="dropOnDirectory($event, dir)"
            >
              <form v-if="editingDirectoryKey === dirKey(dir)" class="aw-drive-inline-form" @submit.prevent="saveDirectoryEdit(dir)">
                <input v-model.trim="directoryEditForm.name" aria-label="编辑目录名称" />
                <select v-model="directoryEditForm.difficulty_class" aria-label="编辑难度">
                  <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
                </select>
                <input v-model.trim="directoryEditForm.allowed_file_types" placeholder="允许格式，不填则不限" aria-label="编辑允许上传格式" />
                <label class="aw-inline-check">
                  <input v-model="directoryEditForm.enabled" type="checkbox" />
                  <span>启用</span>
                </label>
                <div class="aw-drive-inline-form__actions">
                  <button class="aw-grid-button aw-grid-button--strong" type="submit">保存</button>
                  <button class="aw-grid-button" type="button" @click="editingDirectoryKey = ''">取消</button>
                </div>
              </form>
              <button
                v-else
                class="aw-drive-side__item"
                :class="{ 'is-active': activeMode === 'directories' && selectedDir?.key === dirKey(dir), 'is-disabled': dir.enabled === false }"
                type="button"
                @dblclick.stop.prevent="editDirectoryFromDoubleClick(dir)"
                @click="queueOpenDirectory(dir)"
                @contextmenu.prevent.stop="openContextMenu($event, { kind: 'directory', dir })"
              >
                <WorkbenchFolderIcon
                  :size="16"
                  :variant="activeMode === 'directories' && selectedDir?.key === dirKey(dir) ? 'starFilled' : 'star'"
                />
                <span class="aw-drive-side__name">{{ dir.name }}</span>
                <span
                  v-if="dir.allowed_file_types?.length"
                  class="aw-chip aw-chip--subtle aw-drive-column__ext"
                  :title="allowedFileTypesLabel(dir.allowed_file_types)"
                >{{ allowedFileTypesLabel(dir.allowed_file_types) }}</span>
                <span class="aw-chip aw-chip--neutral aw-drive-column__count">{{ dir.file_count }}</span>
              </button>
            </div>
          </div>
        </section>

        <section v-if="canUseOperational" class="aw-drive-side__group">
          <p class="aw-drive-side__label">
            <ImageDown :size="15" aria-hidden="true" />
            <span>运营素材</span>
          </p>
          <button
            class="aw-drive-side__item"
            :class="{ 'is-active': activeMode === 'operational' }"
            type="button"
            @click="openOperational"
          >
            <WorkbenchFolderIcon :size="16" variant="heart" />
            <span class="aw-drive-side__name">全部运营素材</span>
          </button>
        </section>
      </nav>

      <main class="aw-drive-main">
        <div class="aw-drive-main__bar" :class="{ 'aw-drive-main__bar--operational': activeMode === 'operational' }">
          <nav class="aw-drive__breadcrumb" aria-label="路径">
            <template v-if="activeMode === 'uploads'">
              <button class="aw-drive__crumb" :class="{ 'is-active': true }" type="button" @click="openUploadOverview">上传总览</button>
            </template>
            <template v-else-if="activeMode === 'directories'">
              <button class="aw-drive__crumb" type="button" :class="{ 'is-active': !selectedDir }" @click="goDrivesHome">全部目录</button>
              <template v-if="selectedDir">
                <template v-for="(crumb, index) in driveFolderBreadcrumbs" :key="crumb.path || '__drive_root__'">
                  <ChevronRight :size="14" aria-hidden="true" />
                  <button
                    class="aw-drive__crumb"
                    type="button"
                    :class="{ 'is-active': index === driveFolderBreadcrumbs.length - 1 }"
                    @click="openDriveFolder(crumb.path)"
                  >
                    {{ crumb.name }}
                  </button>
                </template>
                <template v-if="archiveView">
                  <template v-for="(crumb, index) in archiveBreadcrumbs" :key="`archive:${crumb.path || '__archive_root__'}`">
                    <ChevronRight :size="14" aria-hidden="true" />
                    <button
                      class="aw-drive__crumb"
                      type="button"
                      :class="{ 'is-active': index === archiveBreadcrumbs.length - 1 }"
                      @click="openArchiveFolder(crumb.path)"
                    >
                      {{ crumb.name }}
                    </button>
                  </template>
                </template>
              </template>
            </template>
            <template v-else>
              <template v-for="(crumb, index) in materialFolderBreadcrumbs" :key="crumb.path || '__material_root__'">
                <ChevronRight v-if="index > 0" :size="14" aria-hidden="true" />
                <button
                  class="aw-drive__crumb"
                  type="button"
                  :class="{ 'is-active': index === materialFolderBreadcrumbs.length - 1 }"
                  @click="openMaterialFolder(crumb.path)"
                >
                  {{ crumb.name }}
                </button>
              </template>
            </template>
          </nav>
          <div class="aw-drive-main__tools">
            <template v-if="activeMode === 'uploads'">
              <form class="aw-upload-overview-filters" @submit.prevent="applyUploadOverviewFilters">
                <input v-model.trim="uploadOverviewQuery" type="search" placeholder="文件名 / 格式 / 上传目录" aria-label="上传文件关键词" />
                <input v-model.trim="uploadOverviewOwner" placeholder="上传人" aria-label="上传人" />
                <input v-model="uploadOverviewFrom" type="date" aria-label="开始日期" />
                <input v-model="uploadOverviewTo" type="date" aria-label="结束日期" />
                <select v-model="uploadOverviewDirectory" aria-label="上传目录">
                  <option value="all">全部目录</option>
                  <option value="unassigned">未分类</option>
                  <option v-for="dir in uploadDirectories" :key="dir.id" :value="String(dir.id)">{{ dir.name }}</option>
                </select>
                <button class="aw-drive__search-submit" type="submit">筛选</button>
                <button v-if="uploadOverviewFilterActive" class="aw-grid-button" type="button" @click="clearUploadOverviewFilters">重置</button>
              </form>
              <button class="aw-secondary-button" type="button" @click="driveSpreadsheetOpen = !driveSpreadsheetOpen">
                <Table2 :size="16" aria-hidden="true" />
                {{ driveSpreadsheetOpen ? '收起清单模式' : '清单模式' }}
              </button>
            </template>
            <template v-else-if="activeMode === 'directories'">
              <button v-if="selectedDir && files.length" class="aw-secondary-button" type="button" @click="selectAllFilesInDirectory">全选</button>
              <button class="aw-secondary-button" type="button" @click="driveSpreadsheetOpen = !driveSpreadsheetOpen">
                <Table2 :size="16" aria-hidden="true" />
                {{ driveSpreadsheetOpen ? '收起清单模式' : '清单模式' }}
              </button>
              <button
                class="aw-primary-button"
                type="button"
                :disabled="!selectedDir"
                :title="selectedDir ? '上传到当前位置' : '先进入一个上传目录'"
                @click="openUpload()"
              >
                <Upload :size="16" aria-hidden="true" />
                上传到此处
              </button>
            </template>
            <template v-else>
              <form class="aw-material-toolbar" @submit.prevent="loadMaterials()">
                <div class="aw-material-toolbar__query">
                  <span class="aw-material-toolbar__search-icon" aria-hidden="true">
                    <IconfontActionIcon name="search" :size="18" />
                  </span>
                  <input v-model="materialQuery" type="search" aria-label="搜索运营素材" placeholder="搜索名称、编码、SKU 或路径" />
                  <button v-if="materialQuery" class="aw-drive__search-clear" type="button" aria-label="清除搜索" @click="clearMaterialSearch">
                    <IconfontActionIcon name="close" :size="14" />
                  </button>
                  <button class="aw-material-toolbar__submit" type="submit">搜索</button>
                </div>
                <div class="aw-material-toolbar__filters" aria-label="素材筛选条件">
                  <label class="aw-material-toolbar__filter">
                    <span>来源</span>
                    <select v-model="materialSourceFilter" aria-label="素材来源" @change="refreshMaterialsForFilters">
                      <option v-for="option in materialSourceOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </label>
                  <label class="aw-material-toolbar__filter">
                    <span>分类</span>
                    <select v-model="materialBusinessLaneFilter" aria-label="素材分类" @change="refreshMaterialsForFilters">
                      <option v-for="option in materialBusinessLaneOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </label>
                  <label class="aw-material-toolbar__filter">
                    <span>格式</span>
                    <select v-model="materialFormatFilter" aria-label="素材格式" @change="refreshMaterialsForFilters">
                      <option v-for="option in materialFormatOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </label>
                </div>
              </form>
              <button class="aw-secondary-button" type="button" @click="driveSpreadsheetOpen = !driveSpreadsheetOpen">
                <Table2 :size="16" aria-hidden="true" />
                {{ driveSpreadsheetOpen ? '收起清单模式' : '清单模式' }}
              </button>
            </template>
          </div>
        </div>

        <div v-if="activeMode !== 'operational' && selectedFileActionIds.length" class="aw-drive-batch-bar">
          <strong>{{ selectedFileActionLabel }}</strong>
          <button class="aw-secondary-button" type="button" @click="downloadSelectedFiles">
            <Download :size="15" aria-hidden="true" />
            下载所选
          </button>
          <template v-if="canManageDrive">
            <select v-model.number="moveTargetDirectoryId" aria-label="移动目标目录">
              <option :value="0">移动到…</option>
              <option v-for="dir in directoryOptions" :key="dir.id" :value="dir.id">{{ dir.name }}</option>
            </select>
            <button class="aw-secondary-button" type="button" :disabled="fileMutationLoading || !moveTargetDirectoryId" @click="moveSelectedFiles">
              {{ fileMutationLoading ? '处理中…' : '移动' }}
            </button>
          </template>
          <template v-if="canDeleteDriveFiles">
            <input v-model.trim="deleteReason" placeholder="删除原因" aria-label="删除原因" />
            <button class="aw-secondary-button aw-secondary-button--danger" type="button" :disabled="fileMutationLoading || !deleteReason.trim()" @click="deleteSelectedFiles">
              <IconfontActionIcon name="delete" :size="15" />
              删除
            </button>
          </template>
          <button class="aw-grid-button" type="button" @click="clearSelection">取消多选</button>
        </div>

        <SpreadsheetWorkbench
          v-if="driveSpreadsheetOpen"
          :source="driveSpreadsheetSource"
          :height="460"
          @close="driveSpreadsheetOpen = false"
          @action="handleDriveSpreadsheetAction"
        />

        <div class="aw-drive-main__content">
          <template v-if="activeMode === 'uploads'">
            <p v-if="filesLoading" class="aw-drive-empty">正在加载上传记录…</p>
            <p v-else-if="filesError" class="aw-drive-empty">{{ filesError }}</p>
            <p v-else-if="files.length === 0" class="aw-drive-empty">没有匹配的上传记录</p>
            <template v-else>
              <div class="aw-upload-overview-table" role="table" aria-label="上传总览">
                <div class="aw-upload-overview-table__head" role="row">
                  <span>文件</span>
                  <span>上传人</span>
                  <span>上传时间</span>
                  <span>目录</span>
                  <span>格式</span>
                  <span>数量</span>
                  <span>计件金额</span>
                  <span>状态</span>
                </div>
                <article
                  v-for="file in files"
                  :key="file.id"
                  class="aw-upload-overview-row"
                  :class="{ 'is-selected': selectedFile?.id === file.id, 'is-highlight': highlightFileId === file.id }"
                  role="row"
                  @click="selectFile(file)"
                  @contextmenu.prevent.stop="openContextMenu($event, { kind: 'file', file })"
                >
                  <label class="aw-upload-overview-row__check" @click.stop>
                    <input
                      type="checkbox"
                      :checked="selectedFileIds.has(file.id)"
                      :aria-label="`选择 ${fileDisplayName(file)}`"
                      @change="toggleFile(file, ($event.target as HTMLInputElement).checked)"
                    />
                  </label>
                  <button class="aw-upload-overview-row__file" type="button" @click.stop="selectFile(file)" @dblclick.stop="canOpenArchive(file) ? openArchiveFile(file) : openFilePreview(file)">
                    <span class="aw-upload-overview-row__thumb">
                      <FileArchive v-if="canOpenArchive(file)" :size="22" aria-hidden="true" />
                      <DriveThumb v-else :file-id="file.id" :filename="fileDisplayName(file)" :mime-type="file.mime_type" :preview-status="file.preview_status" />
                    </span>
                    <span>
                      <strong :title="filePathLabel(file)">{{ filePathLabel(file) }}</strong>
                      <small>{{ formatSize(file.file_size) }}</small>
                    </span>
                  </button>
                  <span>{{ fileOwnerLabel(file) }}</span>
                  <span>{{ formatDateTime(file.created_at) }}</span>
                  <span>{{ file.upload_directory_name || '未分类' }}</span>
                  <span>{{ fileFormatLabel(file) }}</span>
                  <span>{{ file.page_count || '—' }}</span>
                  <span>{{ formatMoney(file.gross_amount || 0) }}</span>
                  <span>{{ statusText(file.pricing_status) }}</span>
                </article>
              </div>
              <div v-if="fileTotal > 0" class="aw-drive-pager">
                <button class="aw-grid-button" type="button" :disabled="filePage <= 1" @click="changePage(-1)">上一页</button>
                <span>{{ filePage }} / {{ totalPages }} · 共 {{ fileTotal }} 个</span>
                <button class="aw-grid-button" type="button" :disabled="filePage >= totalPages" @click="changePage(1)">下一页</button>
              </div>
            </template>
          </template>
          <template v-else-if="activeMode === 'directories'">
            <template v-if="!selectedDir">
              <p v-if="dirLoading" class="aw-drive-empty">加载中…</p>
              <p v-else-if="dirError" class="aw-drive-empty">{{ dirError }}</p>
              <p v-else-if="directories.length === 0" class="aw-drive-empty">暂无上传目录，点击左侧「+」创建</p>
              <div v-else class="aw-drive-tiles">
                <button
                  v-for="dir in directories"
                  :key="dirKey(dir)"
                  class="aw-drive-tile"
                  :class="{ 'is-disabled': dir.enabled === false }"
                  type="button"
                  @click="queueOpenDirectory(dir)"
                  @dblclick.stop.prevent="editDirectoryFromDoubleClick(dir)"
                  @dragover.prevent
                  @drop.prevent="dropOnDirectory($event, dir)"
                  @contextmenu.prevent.stop="openContextMenu($event, { kind: 'directory', dir })"
                >
                  <WorkbenchFolderIcon class="aw-drive-tile__icon" :size="46" variant="star" />
                  <strong class="aw-drive-tile__name" :title="dir.name">{{ dir.name }}</strong>
                  <small class="aw-drive-tile__meta">{{ dir.file_count }} 个文件</small>
                </button>
              </div>
            </template>

            <template v-else>
              <template v-if="archiveView">
                <p v-if="archiveLoading" class="aw-drive-empty">正在读取压缩包…</p>
                <p v-else-if="archiveError" class="aw-drive-empty">{{ archiveError }}</p>
                <template v-else>
                  <div class="aw-drive-archive-head">
                    <div>
                      <strong>压缩包内容</strong>
                      <span>{{ archiveView.folders.length }} 个文件夹 · {{ archiveView.files.length }} 个文件</span>
                    </div>
                    <button class="aw-grid-button" type="button" @click="closeArchiveView">返回所在目录</button>
                  </div>
                  <div v-if="archiveView.folders.length" class="aw-drive-tiles aw-drive-tiles--folders">
                    <button
                      v-for="folder in archiveView.folders"
                      :key="folder.path"
                      class="aw-drive-tile aw-drive-tile--folder"
                      type="button"
                      @click="openArchiveFolder(folder.path)"
                    >
                      <WorkbenchFolderIcon class="aw-drive-tile__icon" :size="46" variant="starFilled" />
                      <strong class="aw-drive-tile__name" :title="folder.name">{{ folder.name }}</strong>
                      <small class="aw-drive-tile__meta">{{ folder.file_count }} 个文件</small>
                    </button>
                  </div>
                  <div v-if="archiveView.files.length" class="aw-drive-files aw-drive-files--roomy">
                    <article v-for="file in archiveView.files" :key="file.path" class="aw-drive-file-card">
                      <button class="aw-drive-file-card__button" type="button" @click="openArchiveVirtualFile(file)" @dblclick="openArchiveVirtualFile(file)">
                        <span class="aw-drive-file-card__media">
                          <ArchiveVirtualThumb :file-id="archiveView.source.id" :file="file" />
                        </span>
                        <span class="aw-drive-file-card__name">{{ file.name }}</span>
                      </button>
                    </article>
                  </div>
                  <p v-if="archiveView.folders.length === 0 && archiveView.files.length === 0" class="aw-drive-empty">压缩包内没有可显示的文件</p>
                </template>
              </template>
              <p v-else-if="filesLoading" class="aw-drive-empty">加载中…</p>
              <p v-else-if="filesError" class="aw-drive-empty">{{ filesError }}</p>
              <div v-else-if="driveFolders.length === 0 && files.length === 0" class="aw-drive-drop" @dragover.prevent @drop.prevent="dropOnCurrentDirectory">
                <Upload :size="26" aria-hidden="true" />
                <span>该目录暂无文件，可拖拽上传到这里</span>
              </div>
              <template v-else>
                <p v-if="driveFolderTruncated" class="aw-drive-inline-note">当前目录文件量较大，已优先展示前 10000 个文件生成的文件夹索引。</p>
                <div v-if="driveFolders.length" class="aw-drive-tiles aw-drive-tiles--folders">
                  <button
                    v-for="folder in driveFolders"
                    :key="folder.path"
                    class="aw-drive-tile aw-drive-tile--folder"
                    type="button"
                    @click="openDriveFolder(folder.path)"
                  >
                    <WorkbenchFolderIcon class="aw-drive-tile__icon" :size="46" variant="starFilled" />
                    <strong class="aw-drive-tile__name" :title="folder.name">{{ folder.name }}</strong>
                    <small class="aw-drive-tile__meta">{{ folder.file_count }} 个文件</small>
                  </button>
                </div>
                <div v-if="files.length" class="aw-drive-files aw-drive-files--roomy" @dragover.prevent @drop.prevent="dropOnCurrentDirectory">
                  <article
                    v-for="file in files"
                    :key="file.id"
                    class="aw-drive-file-card"
                    :class="{ 'is-selected': selectedFile?.id === file.id, 'is-highlight': highlightFileId === file.id }"
                    @contextmenu.prevent.stop="openContextMenu($event, { kind: 'file', file })"
                  >
                    <label class="aw-drive-file-card__check">
                      <input
                        type="checkbox"
                        :checked="selectedFileIds.has(file.id)"
                        :aria-label="`选择 ${fileDisplayName(file)}`"
                        @change="toggleFile(file, ($event.target as HTMLInputElement).checked)"
                      />
                    </label>
                    <button class="aw-drive-file-card__button" type="button" @click="selectFile(file)" @dblclick="canOpenArchive(file) ? openArchiveFile(file) : openFilePreview(file)">
                      <span class="aw-drive-file-card__media">
                        <FileArchive v-if="canOpenArchive(file)" :size="28" aria-hidden="true" />
                        <DriveThumb v-else :file-id="file.id" :filename="fileDisplayName(file)" :mime-type="file.mime_type" :preview-status="file.preview_status" />
                      </span>
                      <span class="aw-drive-file-card__name">{{ filePathLabel(file) }}</span>
                    </button>
                  </article>
                </div>
                <div v-else class="aw-drive-drop" @dragover.prevent @drop.prevent="dropOnCurrentDirectory">
                  <Upload :size="26" aria-hidden="true" />
                  <span>当前文件夹暂无直接文件，可继续打开子文件夹</span>
                </div>
                <div v-if="fileTotal > 0" class="aw-drive-pager">
                  <button class="aw-grid-button" type="button" :disabled="filePage <= 1" @click="changePage(-1)">上一页</button>
                  <span>{{ filePage }} / {{ totalPages }} · 共 {{ fileTotal }} 个</span>
                  <button class="aw-grid-button" type="button" :disabled="filePage >= totalPages" @click="changePage(1)">下一页</button>
                </div>
              </template>
            </template>
          </template>

          <template v-else>
            <p v-if="materialLoading" class="aw-drive-empty">正在检索素材…</p>
            <p v-else-if="materialError" class="aw-drive-empty">{{ materialError }}</p>
            <p v-else-if="visibleMaterialFolders.length === 0 && visibleMaterialFiles.length === 0" class="aw-drive-empty">没有可见素材，调整关键词后再试</p>
            <div v-else class="aw-material-drive">
              <aside class="aw-material-drive__folders" aria-label="全部素材目录">
                <div class="aw-material-drive__head">
                  <strong>全部素材目录</strong>
                  <span>{{ materialQuery ? '搜索结果' : '目录浏览' }}</span>
                </div>
                <div class="aw-material-drive__tree">
                  <button
                    v-for="node in visibleMaterialDirectoryNodes"
                    :key="node.path || '__material_root__'"
                    class="aw-material-folder-node"
                    :class="{
                      'is-active': selectedMaterialFolderPath === node.path,
                      'is-expanded': node.expanded,
                      'has-children': node.has_children,
                    }"
                    type="button"
                    :aria-expanded="node.has_children ? node.expanded : undefined"
                    :style="{ paddingLeft: `${8 + node.depth * 14}px` }"
                    @click="toggleMaterialFolderNode(node.path)"
                  >
                    <ChevronRight
                      v-if="node.has_children"
                      :size="13"
                      class="aw-material-folder-node__chevron"
                      aria-hidden="true"
                    />
                    <span v-else class="aw-material-folder-node__spacer" aria-hidden="true" />
                    <WorkbenchFolderIcon
                      :size="15"
                      :variant="selectedMaterialFolderPath === node.path ? 'heart' : 'star'"
                    />
                    <span>{{ node.name }}</span>
                    <small>{{ node.file_count }}</small>
                  </button>
                </div>
              </aside>

              <section class="aw-material-drive__files" aria-label="当前素材目录">
                <div class="aw-material-drive__summary">
                  <div>
                    <strong>{{ selectedMaterialFolderNode?.name || '全部素材' }}</strong>
                    <span>{{ visibleMaterialFolders.length }} 个子目录 · {{ visibleMaterialFiles.length }} / {{ materialFileTotal }} 个素材</span>
                  </div>
                  <button
                    v-if="selectedMaterialFolderPath"
                    class="aw-grid-button"
                    type="button"
                    @click="openMaterialFolderParent"
                  >
                    上一级
                  </button>
                </div>

                <div
                  v-if="canManageDrive && visibleMaterialFiles.length"
                  class="aw-material-batch-toolbar"
                  :class="{ 'has-selection': selectedMaterialCount > 0 }"
                >
                  <div class="aw-material-batch-toolbar__head">
                    <div>
                      <strong>{{ selectedMaterialCount > 0 ? `已选 ${selectedMaterialCount} 个素材` : '批量管理' }}</strong>
                      <span>{{ selectedMaterialCount > 0 ? '以下操作仅处理已勾选的素材' : '先勾选素材，或直接处理当前目录' }}</span>
                    </div>
                    <div class="aw-material-batch-toolbar__selection">
                      <button class="aw-grid-button" type="button" :disabled="batchUpdatingClientMaterials" @click="selectAllVisibleMaterials">全选当前列表</button>
                      <button v-if="selectedMaterialCount > 0" class="aw-grid-button" type="button" :disabled="batchUpdatingClientMaterials" @click="clearSelectedMaterials">取消全选</button>
                    </div>
                  </div>
                  <div class="aw-material-batch-toolbar__groups">
                    <div v-if="selectedMaterialCount > 0" class="aw-material-batch-toolbar__group">
                      <span>已选素材</span>
                      <div>
                        <button class="aw-primary-button" type="button" :disabled="batchUpdatingClientMaterials" @click="batchUpdateSelectedClientMaterials('publish')">
                          {{ batchUpdatingClientMaterials ? '处理中…' : '上架所选' }}
                        </button>
                        <button class="aw-secondary-button" type="button" :disabled="batchUpdatingClientMaterials" @click="batchUpdateSelectedClientMaterials('disable')">停用所选</button>
                        <button class="aw-secondary-button" type="button" :disabled="batchUpdatingClientMaterials" @click="batchUpdateSelectedClientMaterials('remove')">下架所选</button>
                      </div>
                    </div>
                    <div class="aw-material-batch-toolbar__group aw-material-batch-toolbar__group--directory">
                      <span>当前目录</span>
                      <div>
                        <button class="aw-secondary-button" type="button" :disabled="batchUpdatingClientMaterials" @click="batchPublishCurrentMaterialFolder(false)">上架本目录</button>
                        <button class="aw-secondary-button" type="button" :disabled="batchUpdatingClientMaterials" @click="batchPublishCurrentMaterialFolder(true)">含子目录上架</button>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-if="visibleMaterialFolders.length" class="aw-material-folder-grid">
                  <button
                    v-for="folder in visibleMaterialFolders"
                    :key="folder.path"
                    class="aw-material-folder-card"
                    type="button"
                    @click="openMaterialFolder(folder.path)"
                  >
                    <WorkbenchFolderIcon :size="28" variant="heart" />
                    <strong :title="folder.name">{{ folder.name }}</strong>
                    <small>{{ folder.file_count }} 个素材</small>
                  </button>
                </div>

                <p v-if="visibleMaterialFolders.length === 0 && visibleMaterialFiles.length === 0" class="aw-drive-empty">
                  当前目录没有直接素材，选择上级或其他子目录
                </p>
                <div v-if="visibleMaterialFiles.length" class="aw-drive-ops-list">
                  <div
                    v-for="asset in visibleMaterialFiles"
                    :key="materialAssetKey(asset)"
                    class="aw-material-row"
                    :class="{
                      'is-active': activeMaterial && materialAssetKey(activeMaterial) === materialAssetKey(asset),
                      'is-selected': selectedMaterialIds.has(materialAssetKey(asset)),
                    }"
                    @contextmenu.prevent.stop
                  >
                    <label class="aw-material-row__check" @click.stop>
                      <input
                        type="checkbox"
                        :checked="selectedMaterialIds.has(materialAssetKey(asset))"
                        :aria-label="`选择 ${materialDisplayTitle(asset)}`"
                        @change="toggleMaterial(asset, ($event.target as HTMLInputElement).checked)"
                      />
                    </label>
                    <button class="aw-material-row__button" type="button" @click="selectMaterial(asset)" @dblclick="openMaterialPreview(asset)">
                      <span class="aw-material-row__thumb">
                        <MaterialListThumb :asset="asset" :cached-url="materialPreviewUrls[materialAssetKey(asset)]" />
                      </span>
                      <span class="aw-material-row__body">
                        <strong :title="titleOf(asset)">{{ materialFolderFileName(asset) }}</strong>
                        <span class="aw-material-row__meta">
                          <small class="aw-material-row__code" :title="materialCodeOf(asset)">
                            <span>编码</span>{{ materialCodeOf(asset) }}
                          </small>
                          <small>{{ materialBusinessLaneLabel(asset) }} · {{ materialTypeLabel(asset) }}</small>
                        </span>
                      </span>
                      <span class="aw-material-row__badges">
                        <span class="aw-chip aw-chip--subtle aw-material-row__source">{{ sourceLabelOf(asset) }}</span>
                        <span :class="materialClientStatusClass(asset)">{{ materialClientStatusLabel(asset) }}</span>
                      </span>
                    </button>
                  </div>
                </div>
                <div v-if="materialCanLoadMore" class="aw-drive-pager">
                  <button class="aw-grid-button" type="button" :disabled="materialLoadingMore" @click="loadMoreMaterials">
                    {{ materialLoadingMore ? '加载中…' : '加载更多素材' }}
                  </button>
                  <span>已显示 {{ visibleMaterialFiles.length }} / {{ materialFileTotal }} 个</span>
                </div>

              </section>
            </div>
          </template>
        </div>
      </main>

      <aside v-if="detailOpen" class="aw-drive-drawer" aria-label="详情">
        <template v-if="activeMode !== 'operational' && selectedFile">
          <div class="aw-drive-drawer__head">
            <p class="aw-eyebrow">文件详情</p>
            <button class="aw-drive-mini-button" type="button" aria-label="关闭" @click="closeDetail">
              <IconfontActionIcon name="close" :size="14" />
            </button>
          </div>
          <button
            v-if="canOpenArchive(selectedFile)"
            class="aw-drive__detail-preview aw-drive__detail-preview--archive"
            type="button"
            @click="openArchiveFile(selectedFile)"
          >
            <FileArchive :size="46" aria-hidden="true" />
            <strong>压缩包</strong>
            <span class="aw-drive__detail-hint">点击查看内容</span>
          </button>
          <button v-else class="aw-drive__detail-preview" type="button" @click="openFilePreview(selectedFile)">
            <DriveThumb :file-id="selectedFile.id" :filename="fileDisplayName(selectedFile)" :mime-type="selectedFile.mime_type" :preview-status="selectedFile.preview_status" />
            <span class="aw-drive__detail-hint">点击预览</span>
          </button>
          <h3 class="aw-drive__detail-name">{{ fileDisplayName(selectedFile) }}</h3>
          <dl class="aw-material-detail__list">
            <div><dt>目录</dt><dd>{{ selectedFile.upload_directory_name }}</dd></div>
            <div><dt>文件夹</dt><dd>{{ selectedFile.relative_path || '—' }}</dd></div>
            <div><dt>上传人</dt><dd>{{ fileOwnerLabel(selectedFile) }}</dd></div>
            <div><dt>格式</dt><dd>{{ fileFormatLabel(selectedFile) }}</dd></div>
            <div><dt>上传时间</dt><dd>{{ formatDateTime(selectedFile.created_at) }}</dd></div>
            <div><dt>结算月份</dt><dd>{{ selectedFile.business_month || '—' }}</dd></div>
            <div><dt>难度</dt><dd>{{ selectedFile.difficulty_class || '—' }}</dd></div>
            <div><dt>数量</dt><dd>{{ selectedFile.page_count || '—' }}</dd></div>
            <div><dt>计件金额</dt><dd>{{ formatMoney(selectedFile.gross_amount || 0) }}</dd></div>
            <div><dt>质检</dt><dd>{{ statusText(selectedFile.qc_status) }}</dd></div>
            <div><dt>计件</dt><dd>{{ statusText(selectedFile.pricing_status) }}</dd></div>
            <div><dt>大小</dt><dd>{{ formatSize(selectedFile.file_size) }}</dd></div>
          </dl>
          <div class="aw-drive__detail-actions">
            <button class="aw-primary-button" type="button" @click="downloadFile(selectedFile)">
              <Download :size="16" aria-hidden="true" />
              下载
            </button>
            <button class="aw-secondary-button" type="button" @click="downloadSelectedFiles">下载所选</button>
            <button
              v-if="canOpenArchive(selectedFile)"
              class="aw-secondary-button"
              type="button"
              @click="openArchiveFile(selectedFile)"
            >
              <FileArchive :size="16" aria-hidden="true" />
              查看压缩包内容
            </button>
          </div>
          <div v-if="canMaintainItems || canDeleteDriveFiles" class="aw-drive-maintenance">
            <template v-if="canMaintainItems">
              <p class="aw-eyebrow">文件维护</p>
              <label class="aw-field">
                <span>难度</span>
                <select v-model="itemEditForm.difficulty_class">
                  <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
                </select>
              </label>
              <label class="aw-field">
                <span>页数</span>
                <input v-model.number="itemEditForm.page_count" min="1" type="number" />
              </label>
              <label class="aw-field">
                <span>原因</span>
                <input v-model.trim="maintenanceReason" placeholder="可选，维护记录原因" />
              </label>
              <div class="aw-inline-actions">
                <button class="aw-secondary-button" type="button" @click="saveSelectedItemEdit">保存计件</button>
                <button class="aw-secondary-button" type="button" @click="setSelectedQC('checked')">
                  <CheckCircle2 :size="15" aria-hidden="true" />
                  质检通过
                </button>
                <button class="aw-secondary-button" type="button" @click="setSelectedQC('needs_fix')">标记需修</button>
                <button class="aw-secondary-button" type="button" @click="repriceSelectedItem">重新计件</button>
              </div>
            </template>
            <template v-if="canManageDrive">
              <label class="aw-field">
                <span>移动到目录</span>
                <select v-model.number="moveTargetDirectoryId">
                  <option :value="0">选择目录</option>
                  <option v-for="dir in directoryOptions" :key="dir.id" :value="dir.id">{{ dir.name }}</option>
                </select>
              </label>
              <div class="aw-inline-actions">
                <button class="aw-secondary-button" type="button" :disabled="fileMutationLoading || !moveTargetDirectoryId" @click="moveSelectedFiles">
                  {{ fileMutationLoading ? '处理中…' : '移动文件' }}
                </button>
              </div>
            </template>
            <template v-if="canDeleteDriveFiles">
              <label class="aw-field">
                <span>删除原因</span>
                <input v-model.trim="deleteReason" placeholder="删除必须填写原因" />
              </label>
              <button class="aw-secondary-button" type="button" :disabled="fileMutationLoading || !deleteReason.trim()" @click="deleteSelectedFiles">
                <IconfontActionIcon name="delete" :size="15" />
                删除文件
              </button>
            </template>
          </div>
        </template>

        <template v-else-if="activeMode === 'operational' && activeMaterial">
          <div class="aw-drive-drawer__head">
            <p class="aw-eyebrow">素材详情</p>
            <button class="aw-drive-mini-button" type="button" aria-label="关闭" @click="closeDetail">
              <IconfontActionIcon name="close" :size="14" />
            </button>
          </div>
          <button class="aw-drive__detail-preview" type="button" @click="openMaterialPreview(activeMaterial)">
            <img v-if="activeMaterialPreviewUrl" :src="activeMaterialPreviewUrl" :alt="materialDisplayTitle(activeMaterial)" loading="lazy" />
            <span v-else-if="activeMaterialPreviewLoading" class="aw-drive-thumb__ph" aria-hidden="true" />
            <ImageDown v-else :size="30" aria-hidden="true" />
            <span class="aw-drive__detail-hint">点击预览</span>
          </button>
          <h3 class="aw-drive__detail-name">{{ materialDisplayTitle(activeMaterial) }}</h3>
          <dl class="aw-material-detail__list">
            <div><dt>来源</dt><dd>{{ sourceLabelOf(activeMaterial) }}</dd></div>
            <div><dt>SKU</dt><dd>{{ activeMaterial.scope_sku_code || activeMaterial.sku_code || activeMaterial.primary_sku_code || '—' }}</dd></div>
            <div><dt>文件</dt><dd>{{ activeMaterial.original_filename || activeMaterial.file_name || '—' }}</dd></div>
            <div><dt>类型</dt><dd>{{ materialTypeLabel(activeMaterial) }}</dd></div>
            <div><dt>路径</dt><dd>{{ materialVirtualFilePath(activeMaterial) || '—' }}</dd></div>
            <div><dt>客户端</dt><dd>{{ materialClientStatusLabel(activeMaterial) }}</dd></div>
          </dl>
          <div class="aw-drive__detail-actions">
            <button class="aw-primary-button" type="button" @click="openMaterialPreview(activeMaterial)">
              <ImageDown :size="16" aria-hidden="true" />
              预览
            </button>
            <button class="aw-secondary-button" type="button" @click="downloadMaterial(activeMaterial)">
              <Download :size="16" aria-hidden="true" />
              下载
            </button>
          </div>
          <section v-if="canManageDrive" class="aw-drive-maintenance">
            <p class="aw-eyebrow">当前素材发布</p>
            <div class="aw-compact-list__item">
              <div>
                <strong>{{ activeClientMaterial ? '客户端已收录' : '未上架到客户端' }}</strong>
                <span>
                  {{ activeClientMaterial
                    ? `${activeClientMaterial.enabled ? '上架中' : '已停用'} · ${activeClientMaterial.resource_id || activeClientMaterial.source_ref || activeClientMaterial.asset_id}`
                    : '上架后用户端素材库可预览和下载'
                  }}
                </span>
              </div>
              <div class="aw-inline-actions aw-inline-actions--compact">
                <button
                  v-if="activeClientMaterial"
                  class="aw-grid-button"
                  type="button"
                  @click="toggleClientMaterial(activeClientMaterial)"
                >
                  {{ activeClientMaterial.enabled ? '停用' : '启用' }}
                </button>
                <button
                  v-if="activeClientMaterial"
                  class="aw-grid-button"
                  type="button"
                  @click="removeClientMaterial(activeClientMaterial)"
                >
                  下架
                </button>
                <button
                  v-else
                  class="aw-grid-button aw-grid-button--strong"
                  type="button"
                  :disabled="publishingClientMaterial"
                  @click="publishClientMaterial(activeMaterial)"
                >
                  <IconfontActionIcon name="add" :size="14" />
                  {{ publishingClientMaterial ? '上架中' : '上架' }}
                </button>
              </div>
            </div>
          </section>
        </template>
      </aside>
    </div>

    <div
      v-if="contextMenu"
      ref="contextMenuRef"
      class="aw-drive-context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @click.stop
    >
      <template v-if="contextMenu.kind === 'directory'">
        <button type="button" @click="directoryFromContext() && startDirectoryEdit(directoryFromContext()!)">
          <Pencil :size="14" aria-hidden="true" />
          重命名/改难度
        </button>
        <button
          v-if="directoryFromContext()?.enabled !== false"
          type="button"
          @click="directoryFromContext() && setDirectoryEnabled(directoryFromContext()!, false)"
        >
          停用目录
        </button>
        <button v-else type="button" @click="directoryFromContext() && setDirectoryEnabled(directoryFromContext()!, true)">启用目录</button>
      </template>
      <template v-else>
        <button type="button" @click="previewContextFile">预览</button>
        <button type="button" @click="downloadContextFile">下载</button>
        <button type="button" @click="selectContextFile">加入多选</button>
        <button type="button" @click="downloadContextSelection">下载所选</button>
        <button v-if="canOpenArchive(fileFromContext())" type="button" @click="openContextArchiveFile">
          查看压缩包内容
        </button>
        <button v-if="canDeleteDriveFiles" type="button" @click="selectContextFileForDelete">
          删除…
        </button>
      </template>
    </div>

    <DriveUploadDialog
      :key="uploadDialogKey"
      :open="uploadOpen"
      :directory-id="selectedDir && !selectedDir.unassigned ? selectedDir.id ?? undefined : undefined"
      :directory-name="selectedDir?.name ?? ''"
      :difficulty-class="currentDirRow?.difficulty_class"
      :initial-files="uploadInitialFiles"
      :allowed-file-types="currentDirRow?.allowed_file_types"
      @close="uploadOpen = false"
      @uploaded="onUploaded"
    />

    <WorkbenchPreviewDialog
      :open="previewOpen"
      :title="previewTitle"
      :eyebrow="previewEyebrow"
      :preview-url="previewUrl"
      :mime-type="previewMimeType"
      :filename="previewFilename"
      :meta-rows="previewRows"
      :empty-label="previewEmptyLabel"
      download-label="下载"
      windowed
      @close="closePreview"
      @download="previewDownload && previewDownload()"
    />
  </section>
</template>
