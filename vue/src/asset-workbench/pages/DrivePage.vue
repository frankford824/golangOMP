<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  CheckCircle2,
  ChevronRight,
  Download,
  FileDown,
  Folder,
  FolderOpen,
  HardDrive,
  ImageDown,
  Pencil,
  Plus,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-vue-next'

import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import {
  assetWorkbenchApi,
  type ClientMaterialRow,
  type DifficultyClassRow,
  type DriveDirectoryRow,
  type DriveFileRow,
  type DriveOrderRow,
  type MaterialFolderRow,
  type OverviewSearchRow,
  type SystemAssetRow,
  type SystemAssetPreviewMeta,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import DriveThumb from '@aw/shared/drive/DriveThumb.vue'
import DriveUploadDialog from '@aw/shared/drive/DriveUploadDialog.vue'
import MaterialListThumb from '@aw/shared/materials/MaterialListThumb.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import { canAttemptSystemAssetPreview, materialAssetKey, resolvedSystemAssetThumbnailUrl } from '@aw/shared/materials/systemAssetPreview'

type DriveMode = 'directories' | 'operational'
type SearchScope = 'all' | 'operational' | 'files' | 'orders'
type ClientMaterialFilter = 'all' | 'enabled' | 'disabled'
type ContextMenuState =
  | { kind: 'directory'; x: number; y: number; dir: DriveDirectoryRow }
  | { kind: 'order'; x: number; y: number; order: DriveOrderRow }
  | { kind: 'file'; x: number; y: number; file: DriveFileRow }
type ContextMenuInput =
  | { kind: 'directory'; dir: DriveDirectoryRow }
  | { kind: 'order'; order: DriveOrderRow }
  | { kind: 'file'; file: DriveFileRow }
interface MaterialDirectoryNode {
  path: string
  name: string
  depth: number
  file_count: number
  direct_file_count: number
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

const UNASSIGNED_KEY = 'unassigned'
const pageSize = 60

const activeMode = ref<DriveMode>('directories')
const capabilities = computed(() => new Set(session.bootstrap?.capabilities ?? []))
const canManageDrive = computed(() => capabilities.value.has('asset.workbench.manage'))
const canMaintainItems = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.settlement'))
const canListUploadDirectories = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.submit'))
const canUseOperational = computed(() => canManageDrive.value || capabilities.value.has('asset.workbench.material.download'))

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
const orders = ref<DriveOrderRow[]>([])
const ordersLoading = ref(false)
const selectedOrder = ref<string | null>(null)
const files = ref<DriveFileRow[]>([])
const filesLoading = ref(false)
const fileTotal = ref(0)
const filePage = ref(1)
const selectedFile = ref<DriveFileRow | null>(null)
const selectedFileIds = ref<Set<number>>(new Set())
const highlightFileId = ref<number | null>(null)

const searchQuery = ref('')
const searchScope = ref<SearchScope>('all')
const searchActive = ref(false)
const searchLoading = ref(false)
const searchResults = ref<OverviewSearchRow[]>([])
const searchTotal = ref(0)

const materialQuery = ref('')
const materialLoading = ref(false)
const materialError = ref('')
const materialItems = ref<SystemAssetRow[]>([])
const materialKnownFolders = ref<Record<string, MaterialFolderEntry>>({})
const materialFileTotal = ref(0)
const clientMaterials = ref<ClientMaterialRow[]>([])
const selectedMaterialIds = ref<Set<string>>(new Set())
const materialPreviewUrls = ref<Record<string, string>>({})
const materialPreviewLoadingIds = ref<Set<string>>(new Set())
const activeMaterial = shallowRef<SystemAssetRow | null>(null)
const publishingClientMaterial = ref(false)
const suppressMaterialAutoload = ref(false)
const selectedMaterialFolderPath = ref('')
const expandedMaterialFolderPaths = ref<Set<string>>(new Set(['']))
const clientMaterialFilter = ref<ClientMaterialFilter>('all')

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
const uploadDefaultOrderNo = ref('')
const contextMenu = ref<ContextMenuState | null>(null)
const notice = ref('')
const actionError = ref('')

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
  order_no: '',
  difficulty_class: '',
  page_count: 1,
})

const currentDirRow = computed(() =>
  selectedDir.value ? directories.value.find((item) => dirKey(item) === selectedDir.value?.key) ?? null : null,
)
const directoryOptions = computed(() => uploadDirectories.value.filter((item) => item.enabled))
const difficultyOptions = computed(() => difficultyClasses.value.filter((item) => item.enabled).map((item) => item.code))
const totalPages = computed(() => Math.max(1, Math.ceil(fileTotal.value / pageSize)))
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
const materialDirectoryNodes = computed<MaterialDirectoryNode[]>(() => {
  if (materialQuery.value.trim()) return allMaterialDirectoryNodes.value
  return allMaterialDirectoryNodes.value.filter((node) => materialFolderAncestorsExpanded(node.path))
})
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
      if (!node.path || node.path === selectedMaterialFolderPath.value) return false
      const parts = pathSegments(node.path)
      return parts.length === selectedParts.length + 1 && selectedParts.every((part, index) => parts[index] === part)
    })
    .map((node) => ({ path: node.path, name: node.name, file_count: node.file_count, direct_file_count: node.direct_file_count }))
})
const visibleMaterialFiles = computed(() =>
  materialQuery.value.trim()
    ? materialItems.value
    : materialItems.value.filter((asset) => materialDirectoryPath(asset) === selectedMaterialFolderPath.value),
)
const selectedMaterialFolderParent = computed(() => {
  const parts = pathSegments(selectedMaterialFolderPath.value)
  if (parts.length === 0) return ''
  return pathFromSegments(parts.slice(0, -1))
})

function hasQueryValue(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== ''
}

function dirKey(dir: DriveDirectoryRow): string {
  return dir.directory_id == null ? UNASSIGNED_KEY : String(dir.directory_id)
}

function orderLabel(orderNo: string): string {
  return orderNo && orderNo.trim() ? orderNo : '无订单号'
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

function fileNameFromPath(path?: string): string {
  const parts = pathSegments(path || '')
  return parts.at(-1) || ''
}

function normalizeVirtualPath(path?: string): string {
  const parts = pathSegments(path || '')
  return pathFromSegments(parts)
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
  if (asset.source_type === 'external' && externalPath) return externalPath
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
  const normalized = normalizeVirtualPath(path)
  const depth = pathSegments(normalized).length
  return allMaterialDirectoryNodes.value.some((node) => {
    if (!node.path || node.path === normalized) return false
    const parts = pathSegments(node.path)
    if (parts.length !== depth + 1) return false
    if (!normalized) return parts.length === 1
    return node.path.startsWith(`${normalized}/`)
  })
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
    created_by_name: stringFromMeta(row, 'created_by_name'),
    created_by_username: stringFromMeta(row, 'created_by_username'),
    task_creator_name: stringFromMeta(row, 'task_creator_name'),
    preview_available: boolFromMeta(row, 'preview_available') ?? false,
    origin_path: stringFromMeta(row, 'origin_path') || (row.title.startsWith('/') ? row.title : ''),
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

function formatSize(size?: number): string {
  if (!size) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function fileFormatLabel(file: DriveFileRow): string {
  const rawType = file.file_type?.trim().replace(/^\./, '')
  const suffix = file.original_filename.includes('.') ? file.original_filename.split('.').pop()?.trim() : ''
  const ext = (rawType || suffix || '').replace(/^\./, '')
  const extLabel = ext ? ext.toUpperCase() : ''
  if (file.mime_type) return extLabel ? `${extLabel} · ${file.mime_type}` : file.mime_type
  return extLabel || '—'
}

function statusText(value?: string) {
  const normalized = (value || '').trim()
  const map: Record<string, string> = {
    checked: '已通过',
    needs_fix: '需修',
    pending: '待处理',
    pending_grade: '待定级',
    priced: '已计价',
    unsettled: '未结算',
    settled: '已结算',
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
  const assetKeys = materialIdentityKeys(asset)
  return clientMaterials.value.find((material) => hasSharedIdentity(assetKeys, clientMaterialIdentityKeys(material))) || null
})
const enabledClientMaterialCount = computed(() => clientMaterials.value.filter((material) => material.enabled).length)
const disabledClientMaterialCount = computed(() => clientMaterials.value.length - enabledClientMaterialCount.value)
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

async function selectDir(dir: DriveDirectoryRow, keepFile = false) {
  closeContextMenu()
  const next: SelectedDir = {
    key: dirKey(dir),
    id: dir.directory_id ?? null,
    name: dir.name,
    unassigned: dir.directory_id == null,
  }
  selectedDir.value = next
  selectedOrder.value = null
  files.value = []
  fileTotal.value = 0
  selectedFileIds.value = new Set()
  if (!keepFile) selectedFile.value = null
  ordersLoading.value = true
  try {
    orders.value = await assetWorkbenchApi.driveOrders(
      next.unassigned ? { unassigned: true } : { dir_id: next.id ?? undefined },
    )
  } catch {
    orders.value = []
  } finally {
    ordersLoading.value = false
  }
}

async function selectOrder(orderNo: string, keepFile = false) {
  closeContextMenu()
  selectedOrder.value = orderNo
  filePage.value = 1
  selectedFileIds.value = new Set()
  if (!keepFile) selectedFile.value = null
  await loadFiles()
}

async function loadFiles() {
  if (!selectedDir.value || selectedOrder.value == null) return
  filesLoading.value = true
  try {
    const result = await assetWorkbenchApi.driveFiles({
      dir_id: selectedDir.value.unassigned ? undefined : selectedDir.value.id ?? undefined,
      unassigned: selectedDir.value.unassigned,
      order_no: selectedOrder.value,
      page: filePage.value,
      page_size: pageSize,
    })
    files.value = result.items
    fileTotal.value = result.total
  } catch {
    files.value = []
    fileTotal.value = 0
  } finally {
    filesLoading.value = false
  }
}

async function refreshCurrentDrive() {
  const prevKey = selectedDir.value?.key ?? null
  const prevOrder = selectedOrder.value
  await loadDirectories()
  if (!prevKey) return
  const dir = directories.value.find((item) => dirKey(item) === prevKey)
  if (!dir) return
  await selectDir(dir, true)
  if (prevOrder != null) await selectOrder(prevOrder, true)
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

function selectAllFilesInOrder() {
  selectedFileIds.value = new Set(files.value.map((file) => file.id))
  if (!selectedFile.value && files.value[0]) selectedFile.value = files.value[0]
}

async function editOrderFolder(order: DriveOrderRow) {
  await selectOrder(order.order_no, true)
  const firstFile = files.value[0]
  if (!firstFile) {
    notice.value = '该订单暂无文件，可先上传作品'
    return
  }
  selectFile(firstFile)
  notice.value = `已打开 ${orderLabel(order.order_no)} 的首个文件，可在右侧维护订单信息`
}

function openUploadForOrder(order: DriveOrderRow) {
  if (!selectedDir.value) return
  uploadInitialFiles.value = []
  uploadDefaultOrderNo.value = order.order_no
  uploadDialogKey.value += 1
  uploadOpen.value = true
  closeContextMenu()
}

async function selectAllFilesForOrder(order: DriveOrderRow) {
  await selectOrder(order.order_no, true)
  selectAllFilesInOrder()
  closeContextMenu()
}

function resetToRoot() {
  selectedDir.value = null
  selectedOrder.value = null
  orders.value = []
  files.value = []
  fileTotal.value = 0
  selectedFile.value = null
  selectedFileIds.value = new Set()
}

function backToDir() {
  selectedOrder.value = null
  files.value = []
  fileTotal.value = 0
  selectedFile.value = null
  selectedFileIds.value = new Set()
}

function openDirectory(dir: DriveDirectoryRow) {
  activeMode.value = 'directories'
  void selectDir(dir)
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
  await selectDir(dir, true)
  await selectOrder(file.order_no, true)
  selectFile(file)
  window.setTimeout(() => {
    if (highlightFileId.value === file.id) highlightFileId.value = null
  }, 2400)
}

async function runUnifiedSearch() {
  const q = searchQuery.value.trim()
  if (!q) {
    clearSearch()
    return
  }
  searchActive.value = true
  searchLoading.value = true
  try {
    const result = await assetWorkbenchApi.overviewSearch({ q, scope: searchScope.value, page: 1, page_size: 60 })
    searchResults.value = result.items
    searchTotal.value = result.total
  } catch {
    searchResults.value = []
    searchTotal.value = 0
  } finally {
    searchLoading.value = false
  }
}

function clearSearch() {
  searchActive.value = false
  searchQuery.value = ''
  searchResults.value = []
  searchTotal.value = 0
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
}

async function openFilePreview(file: DriveFileRow) {
  selectFile(file, true)
  openPreviewDialog({
    title: file.original_filename || `文件 ${file.id}`,
    eyebrow: '交稿预览',
    emptyLabel: '正在加载预览…',
    mimeType: file.mime_type,
    filename: file.original_filename,
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
    ['订单号', orderLabel(file.order_no)],
    ['所在目录', file.upload_directory_name],
    ['格式', fileFormatLabel(file)],
    ['上传时间', formatDateTime(file.created_at)],
    ['业务月', file.business_month || '—'],
    ['难度', file.difficulty_class || '—'],
    ['QC', statusText(file.qc_status)],
    ['计价', statusText(file.pricing_status)],
    ['大小', formatSize(file.file_size)],
  ]
}

async function downloadFile(file: DriveFileRow) {
  try {
    const meta = await assetWorkbenchApi.getFileDownload(file.id)
    if (meta.download_url) window.open(meta.download_url, '_blank', 'noopener,noreferrer')
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '下载链接生成失败'
  }
}

async function downloadSelectedFiles() {
  const ids = selectedFileActionIds.value
  if (!ids.length) return
  notice.value = '正在生成文件包'
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
    notice.value = `已打包 ${result.writtenCount} 个文件，失败 ${result.failureCount} 个`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '批量下载失败'
  }
}

function openUpload(files: File[] = []) {
  if (!selectedDir.value) return
  uploadInitialFiles.value = files
  uploadDefaultOrderNo.value = selectedOrder.value ?? ''
  uploadDialogKey.value += 1
  uploadOpen.value = true
}

function filesFromDrop(event: DragEvent) {
  return Array.from(event.dataTransfer?.files ?? []).filter((file) => file.size > 0)
}

function dropOnDirectory(event: DragEvent, dir: DriveDirectoryRow) {
  const dropped = filesFromDrop(event)
  if (!dropped.length) return
  void selectDir(dir, true).then(() => openUpload(dropped))
}

function dropOnOrder(event: DragEvent, order: DriveOrderRow) {
  const dropped = filesFromDrop(event)
  if (!dropped.length) return
  void selectOrder(order.order_no, true).then(() => openUpload(dropped))
}

function dropOnCurrentOrder(event: DragEvent) {
  const dropped = filesFromDrop(event)
  if (!dropped.length || !selectedOrder.value) return
  openUpload(dropped)
}

async function onUploaded() {
  uploadOpen.value = false
  uploadInitialFiles.value = []
  uploadDefaultOrderNo.value = ''
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
    const updated = await assetWorkbenchApi.updateSubmissionItem(file.submission_item_id, {
      order_no: itemEditForm.value.order_no,
      difficulty_class: itemEditForm.value.difficulty_class,
      page_count: itemEditForm.value.page_count,
      finalized: true,
      reason: maintenanceReason.value || '素材网盘内联维护',
    })
    notice.value = `已更新 ${updated.order_no}`
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '订单维护保存失败'
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
    actionError.value = err instanceof Error ? err.message : 'QC 状态更新失败'
  }
}

async function repriceSelectedItem() {
  const file = selectedFile.value
  if (!file || !canMaintainItems.value) return
  try {
    const updated = await assetWorkbenchApi.repriceSubmissionItem(file.submission_item_id, maintenanceReason.value || '素材网盘重计价')
    notice.value = `已重计价：${statusText(updated.pricing_status)}`
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '重计价失败'
  }
}

async function moveSelectedFiles() {
  if (!canManageDrive.value) return
  const ids = selectedFileActionIds.value
  if (!ids.length || !moveTargetDirectoryId.value) return
  try {
    const result = await assetWorkbenchApi.batchMoveFiles(ids, moveTargetDirectoryId.value, maintenanceReason.value || '素材网盘移动文件')
    notice.value = `已移动 ${result.files?.length ?? 0} 个文件，失败 ${result.failures?.length ?? 0} 个`
    selectedFileIds.value = new Set()
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '移动文件失败'
  }
}

async function deleteSelectedFiles() {
  if (!canManageDrive.value) return
  const ids = selectedFileActionIds.value
  if (!ids.length || !deleteReason.value.trim()) return
  try {
    const result = await assetWorkbenchApi.batchDeleteFiles(ids, deleteReason.value.trim())
    notice.value = `已删除 ${result.deleted?.length ?? 0} 个文件，失败 ${result.failures?.length ?? 0} 个`
    selectedFileIds.value = new Set()
    deleteReason.value = ''
    await refreshCurrentDrive()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除文件失败'
  }
}

async function loadMaterials(query = materialQuery.value) {
  if (!canUseOperational.value) return
  const nextQuery = query.trim()
  if (canManageDrive.value && !nextQuery) {
    await loadMaterialFolder('')
    return
  }
  materialLoading.value = true
  materialError.value = ''
  materialQuery.value = nextQuery
  selectedMaterialFolderPath.value = ''
  activeMaterial.value = null
  try {
    if (canManageDrive.value) {
      const [systemResult, published] = await Promise.all([
        assetWorkbenchApi.systemSearch({ q: materialQuery.value, source: 'all', page: 1, page_size: 80 }),
        assetWorkbenchApi.listClientMaterials(true),
      ])
      materialItems.value = systemResult.items
      materialFileTotal.value = systemResult.total
      clientMaterials.value = published
      for (const asset of systemResult.items) {
        rememberMaterialPath(materialDirectoryPath(asset))
      }
    } else {
      const published = await assetWorkbenchApi.listClientMaterials(false)
      clientMaterials.value = published
      const q = materialQuery.value.toLowerCase()
      materialItems.value = published.map(materialFromClient).filter((asset) => {
        if (!q) return true
        return [titleOf(asset), asset.original_filename, asset.resource_id, asset.scope_sku_code, asset.source_label]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()
          .includes(q)
      })
      materialFileTotal.value = materialItems.value.length
    }
  } catch (err) {
    materialItems.value = []
    materialFileTotal.value = 0
    materialError.value = err instanceof Error ? err.message : '运营素材加载失败'
  } finally {
    materialLoading.value = false
  }
}

async function loadMaterialFolder(path = selectedMaterialFolderPath.value, options: { expandTree?: boolean } = {}) {
  if (!canUseOperational.value) return
  const expandTree = options.expandTree !== false
  const normalized = normalizeVirtualPath(path)
  materialLoading.value = true
  materialError.value = ''
  materialQuery.value = ''
  selectedMaterialFolderPath.value = normalized
  activeMaterial.value = null
  rememberMaterialPath(normalized)
  if (expandTree) expandMaterialFolderTreePath(normalized)
  try {
    if (canManageDrive.value) {
      const [browse, published] = await Promise.all([
        assetWorkbenchApi.browseMaterials({ path: normalized, source: 'all', page: 1, page_size: 100 }),
        assetWorkbenchApi.listClientMaterials(true),
      ])
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
      materialItems.value = browse.files || []
      materialFileTotal.value = browse.total || 0
      clientMaterials.value = published
    } else {
      const published = await assetWorkbenchApi.listClientMaterials(false)
      clientMaterials.value = published
      materialItems.value = published.map(materialFromClient)
      materialFileTotal.value = materialItems.value.length
    }
  } catch (err) {
    materialItems.value = []
    materialFileTotal.value = 0
    materialError.value = err instanceof Error ? err.message : '运营素材加载失败'
  } finally {
    materialLoading.value = false
  }
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
    const meta = await previewMaterial(asset)
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

async function downloadMaterial(asset: SystemAssetRow) {
  try {
    const meta = asset.material_id
      ? await assetWorkbenchApi.downloadClientMaterial(asset.material_id)
      : await assetWorkbenchApi.downloadMaterialAsset(asset)
    if (meta.download_url) window.open(meta.download_url, '_blank', 'noopener,noreferrer')
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '素材下载失败'
  }
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
  selectMaterial(existing ? materialWithClientPublication(existing, material) : materialFromClient(material))
}

function cacheMaterialPreview(key: string, url: string) {
  if (!key || !url || materialPreviewUrls.value[key]) return
  materialPreviewUrls.value = { ...materialPreviewUrls.value, [key]: url }
}

async function ensureMaterialPreview(asset: SystemAssetRow) {
  const key = materialAssetKey(asset)
  if (materialPreviewUrls.value[key]) return
  const inline = resolvedSystemAssetThumbnailUrl(asset)
  if (inline) {
    cacheMaterialPreview(key, inline)
    return
  }
  if (!canAttemptSystemAssetPreview(asset) || materialPreviewLoadingIds.value.has(key)) return
  const loading = new Set(materialPreviewLoadingIds.value)
  loading.add(key)
  materialPreviewLoadingIds.value = loading
  try {
    const meta = await previewMaterial(asset)
    const url = meta.preview_url || ''
    if (url) cacheMaterialPreview(key, url)
  } catch {
    /* silent: big preview falls back to icon, dialog surfaces errors */
  } finally {
    const next = new Set(materialPreviewLoadingIds.value)
    next.delete(key)
    materialPreviewLoadingIds.value = next
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
}

function closeContextMenu() {
  contextMenu.value = null
}

function fileFromContext() {
  return contextMenu.value?.kind === 'file' ? contextMenu.value.file : null
}

function orderFromContext() {
  return contextMenu.value?.kind === 'order' ? contextMenu.value.order : null
}

function directoryFromContext() {
  return contextMenu.value?.kind === 'directory' ? contextMenu.value.dir : null
}

function syncEditForm(file: DriveFileRow | null) {
  itemEditForm.value = {
    order_no: file?.order_no ?? '',
    difficulty_class: file?.difficulty_class || currentDirRow.value?.difficulty_class || difficultyOptions.value[0] || '',
    page_count: file?.page_count || 1,
  }
}

watch(selectedFile, syncEditForm)
watch(activeMode, (mode) => {
  if (mode === 'operational' && materialItems.value.length === 0 && !suppressMaterialAutoload.value && !materialLoading.value) {
    void loadMaterials()
  }
})

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
  if (hasQueryValue(route.query.q)) {
    searchQuery.value = route.query.q
    searchScope.value = normalizeScope(route.query.scope)
    await runUnifiedSearch()
  }
  if (activeMode.value === 'operational') await loadMaterials(hasQueryValue(route.query.q) ? route.query.q : '')
  window.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
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
          <h2>{{ canManageDrive ? '全站素材网盘' : '我的素材网盘' }}</h2>
        </div>
      </div>
      <form class="aw-drive__search" @submit.prevent="runUnifiedSearch">
        <Search :size="16" aria-hidden="true" />
        <select v-model="searchScope" aria-label="搜索范围">
          <option value="all">全部</option>
          <option value="operational">运营素材</option>
          <option value="files">交稿文件</option>
          <option value="orders">订单·计件</option>
        </select>
        <input
          v-model="searchQuery"
          type="search"
          placeholder="搜索运营素材、订单号、文件名"
          @keyup.enter="runUnifiedSearch"
        />
        <button v-if="searchActive" class="aw-drive__search-clear" type="button" aria-label="清除搜索" @click="clearSearch">
          <X :size="14" aria-hidden="true" />
        </button>
      </form>
    </header>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="actionError" class="aw-inline-alert aw-inline-alert--error">{{ actionError }}</p>

    <section v-if="searchActive" class="aw-drive-search-results">
      <div class="aw-drive-search-results__head">
        <span>统一检索「{{ searchQuery }}」</span>
        <span class="aw-drive-search-results__count">{{ searchLoading ? '搜索中…' : `共 ${searchTotal} 条` }}</span>
      </div>
      <p v-if="!searchLoading && searchResults.length === 0" class="aw-drive-empty">没有匹配内容</p>
      <ul v-else class="aw-drive-hit-list">
        <li v-for="hit in searchResults" :key="`${hit.source}-${hit.id}`" class="aw-drive-hit">
          <button
            class="aw-drive-hit__thumb"
            type="button"
            :aria-label="`定位 ${hit.title || hit.primary_code || '搜索结果'}`"
            @click="locateSearchRow(hit)"
          >
            <ImageDown v-if="hit.scope === 'operational'" :size="24" aria-hidden="true" />
            <FileDown v-else :size="24" aria-hidden="true" />
          </button>
          <div class="aw-drive-hit__body">
            <strong>{{ hit.title || hit.primary_code }}</strong>
            <span class="aw-drive-hit__path">
              {{ hit.source_label || hit.source }} <ChevronRight :size="12" /> {{ hit.primary_code || hit.order_no || '—' }}
            </span>
            <small>{{ hit.order_no || hit.business_month || '—' }} · {{ statusText(hit.status) }}</small>
          </div>
          <div class="aw-drive-hit__actions">
            <button class="aw-secondary-button" type="button" @click="locateSearchRow(hit)">
              <FolderOpen :size="14" aria-hidden="true" />
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
            <Folder :size="15" aria-hidden="true" />
            <span>上传目录</span>
            <button v-if="canManageDrive" class="aw-drive-mini-button" type="button" aria-label="新建上传目录" @click="creatingDirectory = true">
              <Plus :size="14" aria-hidden="true" />
            </button>
          </p>
          <form v-if="creatingDirectory" class="aw-drive-inline-form" @submit.prevent="createDirectory">
            <input v-model.trim="newDirectoryName" placeholder="目录名称" aria-label="目录名称" />
            <select v-model="newDirectoryDifficulty" aria-label="难度">
              <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
            </select>
            <input v-model.trim="newDirectoryFileTypes" placeholder="格式限制，留空=全部" aria-label="允许上传格式" />
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
                <input v-model.trim="directoryEditForm.allowed_file_types" placeholder="格式限制，留空=全部" aria-label="编辑允许上传格式" />
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
                @dblclick="startDirectoryEdit(dir)"
                @click="openDirectory(dir)"
                @contextmenu.prevent.stop="openContextMenu($event, { kind: 'directory', dir })"
              >
                <Folder :size="16" aria-hidden="true" />
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
            <FolderOpen :size="16" aria-hidden="true" />
            <span class="aw-drive-side__name">全部运营素材</span>
          </button>
        </section>
      </nav>

      <main class="aw-drive-main">
        <div class="aw-drive-main__bar">
          <nav class="aw-drive__breadcrumb" aria-label="路径">
            <template v-if="activeMode === 'directories'">
              <button class="aw-drive__crumb" type="button" :class="{ 'is-active': !selectedDir }" @click="goDrivesHome">全部目录</button>
              <template v-if="selectedDir">
                <ChevronRight :size="14" aria-hidden="true" />
                <button class="aw-drive__crumb" type="button" :class="{ 'is-active': selectedOrder == null }" @click="backToDir">{{ selectedDir.name }}</button>
              </template>
              <template v-if="selectedDir && selectedOrder != null">
                <ChevronRight :size="14" aria-hidden="true" />
                <span class="aw-drive__crumb" :class="{ 'is-active': true }">{{ orderLabel(selectedOrder) }}</span>
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
            <template v-if="activeMode === 'directories'">
              <button v-if="selectedOrder != null" class="aw-secondary-button" type="button" @click="selectAllFilesInOrder">全选</button>
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
              <form class="aw-drive__search aw-drive__search--inline" @submit.prevent="loadMaterials()">
                <Search :size="16" aria-hidden="true" />
                <input v-model="materialQuery" type="search" placeholder="按文件名 / SKU / 外部路径过滤" />
                <button v-if="materialQuery" class="aw-drive__search-clear" type="button" aria-label="清除" @click="materialQuery = ''; loadMaterials()">
                  <X :size="14" aria-hidden="true" />
                </button>
              </form>
              <button class="aw-primary-button" type="button" @click="loadMaterials()">
                <Search :size="16" aria-hidden="true" />
                搜索
              </button>
            </template>
          </div>
        </div>

        <div class="aw-drive-main__content">
          <template v-if="activeMode === 'directories'">
            <template v-if="!selectedDir">
              <p v-if="dirLoading" class="aw-drive-empty">加载中…</p>
              <p v-else-if="directories.length === 0" class="aw-drive-empty">暂无上传目录，点击左侧「+」创建</p>
              <div v-else class="aw-drive-tiles">
                <button
                  v-for="dir in directories"
                  :key="dirKey(dir)"
                  class="aw-drive-tile"
                  :class="{ 'is-disabled': dir.enabled === false }"
                  type="button"
                  @click="openDirectory(dir)"
                  @dblclick="startDirectoryEdit(dir)"
                  @dragover.prevent
                  @drop.prevent="dropOnDirectory($event, dir)"
                  @contextmenu.prevent.stop="openContextMenu($event, { kind: 'directory', dir })"
                >
                  <Folder class="aw-drive-tile__icon" aria-hidden="true" />
                  <strong class="aw-drive-tile__name" :title="dir.name">{{ dir.name }}</strong>
                  <small class="aw-drive-tile__meta">{{ dir.file_count }} 个文件</small>
                </button>
              </div>
            </template>

            <template v-else-if="selectedOrder == null">
              <p v-if="ordersLoading" class="aw-drive-empty">加载中…</p>
              <p v-else-if="orders.length === 0" class="aw-drive-empty">该目录暂无订单</p>
              <div v-else class="aw-drive-tiles">
                <article
                  v-for="order in orders"
                  :key="order.order_no || '__empty__'"
                  class="aw-drive-tile aw-drive-tile--with-action"
                  @dragover.prevent
                  @drop.prevent="dropOnOrder($event, order)"
                  @contextmenu.prevent.stop="openContextMenu($event, { kind: 'order', order })"
                >
                  <button class="aw-drive-tile__button" type="button" @click="selectOrder(order.order_no)">
                    <FolderOpen class="aw-drive-tile__icon" aria-hidden="true" />
                    <strong class="aw-drive-tile__name" :title="orderLabel(order.order_no)">{{ orderLabel(order.order_no) }}</strong>
                    <small class="aw-drive-tile__meta">{{ order.file_count }} 个文件</small>
                  </button>
                  <button
                    v-if="canMaintainItems"
                    class="aw-drive-tile__quick"
                    type="button"
                    :aria-label="`编辑 ${orderLabel(order.order_no)}`"
                    title="编辑订单文件"
                    @click.stop="editOrderFolder(order)"
                  >
                    <Pencil :size="14" aria-hidden="true" />
                  </button>
                </article>
              </div>
            </template>

            <template v-else>
              <p v-if="filesLoading" class="aw-drive-empty">加载中…</p>
              <div v-else-if="files.length === 0" class="aw-drive-drop" @dragover.prevent @drop.prevent="dropOnCurrentOrder">
                <Upload :size="26" aria-hidden="true" />
                <span>该订单暂无文件，可拖拽上传到这里</span>
              </div>
              <template v-else>
                <div class="aw-drive-files aw-drive-files--roomy" @dragover.prevent @drop.prevent="dropOnCurrentOrder">
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
                        :aria-label="`选择 ${file.original_filename}`"
                        @change="toggleFile(file, ($event.target as HTMLInputElement).checked)"
                      />
                    </label>
                    <button class="aw-drive-file-card__button" type="button" @click="selectFile(file)" @dblclick="openFilePreview(file)">
                      <span class="aw-drive-file-card__media">
                        <DriveThumb :file-id="file.id" :filename="file.original_filename" :mime-type="file.mime_type" :preview-status="file.preview_status" />
                      </span>
                      <span class="aw-drive-file-card__name">{{ file.original_filename }}</span>
                    </button>
                  </article>
                </div>
                <div class="aw-drive-pager">
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
                    v-for="node in materialDirectoryNodes"
                    :key="node.path || '__material_root__'"
                    class="aw-material-folder-node"
                    :class="{
                      'is-active': selectedMaterialFolderPath === node.path,
                      'is-expanded': isMaterialFolderExpanded(node.path),
                      'has-children': materialFolderHasChildren(node.path),
                    }"
                    type="button"
                    :aria-expanded="materialFolderHasChildren(node.path) ? isMaterialFolderExpanded(node.path) : undefined"
                    :style="{ paddingLeft: `${8 + node.depth * 14}px` }"
                    @click="toggleMaterialFolderNode(node.path)"
                  >
                    <ChevronRight
                      v-if="materialFolderHasChildren(node.path)"
                      :size="13"
                      class="aw-material-folder-node__chevron"
                      aria-hidden="true"
                    />
                    <span v-else class="aw-material-folder-node__spacer" aria-hidden="true" />
                    <FolderOpen v-if="selectedMaterialFolderPath === node.path" :size="15" aria-hidden="true" />
                    <Folder v-else :size="15" aria-hidden="true" />
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

                <div v-if="visibleMaterialFolders.length" class="aw-material-folder-grid">
                  <button
                    v-for="folder in visibleMaterialFolders"
                    :key="folder.path"
                    class="aw-material-folder-card"
                    type="button"
                    @click="openMaterialFolder(folder.path)"
                  >
                    <FolderOpen :size="24" aria-hidden="true" />
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
                    :class="{ 'is-active': activeMaterial && materialAssetKey(activeMaterial) === materialAssetKey(asset) }"
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
                        <MaterialListThumb :asset="asset" :cached-url="materialPreviewUrls[materialAssetKey(asset)]" @loaded="cacheMaterialPreview" />
                      </span>
                      <span class="aw-material-row__body">
                        <strong :title="titleOf(asset)">{{ materialFolderFileName(asset) }}</strong>
                        <small>{{ materialCodeOf(asset) }} · {{ materialTypeLabel(asset) }}</small>
                      </span>
                      <span class="aw-chip aw-chip--subtle aw-material-row__source">{{ sourceLabelOf(asset) }}</span>
                    </button>
                  </div>
                </div>

                <section v-if="canManageDrive" class="aw-client-materials-panel" aria-label="客户端素材管理">
                  <div class="aw-client-materials-panel__head">
                    <div>
                      <strong>客户端素材管理</strong>
                      <span>{{ enabledClientMaterialCount }} 个上架中 · {{ disabledClientMaterialCount }} 个已停用</span>
                    </div>
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
                  </div>
                  <div v-if="visibleClientMaterials.length" class="aw-client-materials-list">
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
              </section>
            </div>
          </template>
        </div>
      </main>

      <aside v-if="detailOpen" class="aw-drive-drawer" aria-label="详情">
        <template v-if="activeMode === 'directories' && selectedFile">
          <div class="aw-drive-drawer__head">
            <p class="aw-eyebrow">文件详情</p>
            <button class="aw-drive-mini-button" type="button" aria-label="关闭" @click="closeDetail">
              <X :size="14" aria-hidden="true" />
            </button>
          </div>
          <button class="aw-drive__detail-preview" type="button" @click="openFilePreview(selectedFile)">
            <DriveThumb :file-id="selectedFile.id" :filename="selectedFile.original_filename" :mime-type="selectedFile.mime_type" :preview-status="selectedFile.preview_status" />
            <span class="aw-drive__detail-hint">点击预览</span>
          </button>
          <h3 class="aw-drive__detail-name">{{ selectedFile.original_filename }}</h3>
          <dl class="aw-material-detail__list">
            <div><dt>订单号</dt><dd>{{ orderLabel(selectedFile.order_no) }}</dd></div>
            <div><dt>目录</dt><dd>{{ selectedFile.upload_directory_name }}</dd></div>
            <div><dt>格式</dt><dd>{{ fileFormatLabel(selectedFile) }}</dd></div>
            <div><dt>上传时间</dt><dd>{{ formatDateTime(selectedFile.created_at) }}</dd></div>
            <div><dt>业务月</dt><dd>{{ selectedFile.business_month || '—' }}</dd></div>
            <div><dt>难度</dt><dd>{{ selectedFile.difficulty_class || '—' }}</dd></div>
            <div><dt>QC</dt><dd>{{ statusText(selectedFile.qc_status) }}</dd></div>
            <div><dt>计价</dt><dd>{{ statusText(selectedFile.pricing_status) }}</dd></div>
            <div><dt>大小</dt><dd>{{ formatSize(selectedFile.file_size) }}</dd></div>
          </dl>
          <div class="aw-drive__detail-actions">
            <button class="aw-primary-button" type="button" @click="downloadFile(selectedFile)">
              <Download :size="16" aria-hidden="true" />
              下载
            </button>
            <button class="aw-secondary-button" type="button" @click="downloadSelectedFiles">{{ selectedFileActionLabel }}打包</button>
          </div>
          <div v-if="canMaintainItems" class="aw-drive-maintenance">
            <p class="aw-eyebrow">内联维护</p>
            <label class="aw-field">
              <span>订单号</span>
              <input v-model.trim="itemEditForm.order_no" />
            </label>
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
              <button class="aw-secondary-button" type="button" @click="saveSelectedItemEdit">保存订单</button>
              <button class="aw-secondary-button" type="button" @click="setSelectedQC('checked')">
                <CheckCircle2 :size="15" aria-hidden="true" />
                QC 通过
              </button>
              <button class="aw-secondary-button" type="button" @click="setSelectedQC('needs_fix')">需修</button>
              <button class="aw-secondary-button" type="button" @click="repriceSelectedItem">重计价</button>
            </div>
            <template v-if="canManageDrive">
              <label class="aw-field">
                <span>移动到目录</span>
                <select v-model.number="moveTargetDirectoryId">
                  <option :value="0">选择目录</option>
                  <option v-for="dir in directoryOptions" :key="dir.id" :value="dir.id">{{ dir.name }}</option>
                </select>
              </label>
              <div class="aw-inline-actions">
                <button class="aw-secondary-button" type="button" :disabled="!moveTargetDirectoryId" @click="moveSelectedFiles">移动文件</button>
              </div>
              <label class="aw-field">
                <span>删除原因</span>
                <input v-model.trim="deleteReason" placeholder="删除必须填写原因" />
              </label>
              <button class="aw-secondary-button" type="button" :disabled="!deleteReason.trim()" @click="deleteSelectedFiles">
                <Trash2 :size="15" aria-hidden="true" />
                删除文件
              </button>
            </template>
          </div>
        </template>

        <template v-else-if="activeMode === 'operational' && activeMaterial">
          <div class="aw-drive-drawer__head">
            <p class="aw-eyebrow">素材详情</p>
            <button class="aw-drive-mini-button" type="button" aria-label="关闭" @click="closeDetail">
              <X :size="14" aria-hidden="true" />
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
            <div><dt>路径</dt><dd>{{ activeMaterial.origin_path || '—' }}</dd></div>
            <div><dt>客户端</dt><dd>{{ activeClientMaterial ? (activeClientMaterial.enabled ? '已上架' : '已停用') : '未上架' }}</dd></div>
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
                  <Plus :size="14" aria-hidden="true" />
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
      <template v-else-if="contextMenu.kind === 'order'">
        <button type="button" @click="orderFromContext() && openUploadForOrder(orderFromContext()!)">
          <Upload :size="14" aria-hidden="true" />
          上传到订单
        </button>
        <button v-if="canMaintainItems" type="button" @click="orderFromContext() && editOrderFolder(orderFromContext()!)">
          <Pencil :size="14" aria-hidden="true" />
          编辑订单文件
        </button>
        <button type="button" @click="orderFromContext() && selectAllFilesForOrder(orderFromContext()!)">全选订单文件</button>
      </template>
      <template v-else>
        <button type="button" @click="fileFromContext() && openFilePreview(fileFromContext()!)">预览</button>
        <button type="button" @click="fileFromContext() && downloadFile(fileFromContext()!)">下载</button>
        <button type="button" @click="fileFromContext() && selectFile(fileFromContext()!, true)">加入选择</button>
      </template>
    </div>

    <DriveUploadDialog
      :key="uploadDialogKey"
      :open="uploadOpen"
      :directory-id="selectedDir && !selectedDir.unassigned ? selectedDir.id ?? undefined : undefined"
      :directory-name="selectedDir?.name ?? ''"
      :difficulty-class="currentDirRow?.difficulty_class"
      :default-order-no="uploadDefaultOrderNo"
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
      @close="closePreview"
      @download="previewDownload && previewDownload()"
    />
  </section>
</template>
