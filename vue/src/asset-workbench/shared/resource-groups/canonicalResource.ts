import {
  resourceGroupsApi,
  type FlatResourceItem,
  type ResourceDownloadInfo,
  type ResourceFile,
  type ResourceGroup,
  type ResourceReference,
  type ResourceRevision,
} from '@/services/api/resourceGroupsApi'
import { parseApiErrorPayload } from '@/utils/api-message-zh'

export type CanonicalResourceRole = '' | 'reference' | 'source' | 'final'

export interface CanonicalResourceFile {
  taskAssetId?: number
  filename: string
  mimeType?: string
  fileSize?: number | null
  previewUrl?: string
  downloadUrl?: string
}

export const canonicalResourceRoleOptions: Array<{ value: CanonicalResourceRole; label: string }> = [
  { value: '', label: '全部资源' },
  { value: 'reference', label: '参考图' },
  { value: 'source', label: '设计源文件' },
  { value: 'final', label: '最终成品' },
]

export function canonicalResourceRoleLabel(role: FlatResourceItem['resource_role']): string {
  return ({ reference: '参考图', source: '设计源文件', final: '最终成品' } as const)[role]
}

export function canonicalPreviewUnavailableMessage(role?: FlatResourceItem['resource_role']): string {
  return role === 'source'
    ? '该设计源文件暂不支持在线预览，请下载后查看。'
    : '当前资源暂不支持在线预览，请下载后查看。'
}

export function canonicalPreviewErrorMessage(
  cause: unknown,
  role?: FlatResourceItem['resource_role'],
): string {
  const parsed = parseApiErrorPayload(cause)
  const backendMessage = (parsed.message || '').trim().toLowerCase().replace(/\.$/, '')
  if (
    parsed.status === 409 &&
    parsed.code === 'INVALID_STATE_TRANSITION' &&
    (backendMessage === 'task asset preview is not available' || backendMessage === 'asset preview is not available')
  ) {
    return canonicalPreviewUnavailableMessage(role)
  }
  return cause instanceof Error ? cause.message : '预览加载失败'
}

export function currentCanonicalRevision(group: ResourceGroup): ResourceRevision | null {
  return group.finalized_revision || group.working_revision || null
}

export function canonicalGroupFinals(group: ResourceGroup) {
  return [...(currentCanonicalRevision(group)?.items || [])].sort((left, right) => left.sort_order - right.sort_order)
}

export function canonicalGroupCover(group: ResourceGroup): CanonicalResourceFile | null {
  const file = canonicalGroupFinals(group)[0]?.file
  return file ? fromResourceFile(file) : null
}

export function canonicalFileFromGroup(group: ResourceGroup, item: FlatResourceItem): CanonicalResourceFile {
  const revision = currentCanonicalRevision(group)
  if (!revision || revision.id !== item.revision_id) {
    throw new Error('资源版本已更新，请刷新列表后重试')
  }
  if (item.resource_role === 'source') {
    if (!revision.source_file) throw new Error('当前资源版本没有设计源文件')
    return fromResourceFile(revision.source_file)
  }
  if (item.resource_role === 'final') {
    const revisionItem = revision.items.find((candidate) => candidate.id === item.resource_item_id)
    if (!revisionItem?.file) throw new Error('最终成品已变化，请刷新列表后重试')
    return fromResourceFile(revisionItem.file)
  }
  const reference = revision.references.find((candidate) => candidate.id === item.resource_item_id)
  if (!reference) throw new Error('参考图已变化，请刷新列表后重试')
  return fromReference(reference)
}

export async function resolveCanonicalPreview(
  item: FlatResourceItem,
  getGroup: (groupID: number) => Promise<ResourceGroup> = resourceGroupsApi.get,
  signal?: AbortSignal,
): Promise<ResourceDownloadInfo> {
  if (item.task_asset_id) return resourceGroupsApi.previewTaskAsset(item.task_asset_id, signal)
  const group = await getGroup(item.group_id)
  const file = canonicalFileFromGroup(group, item)
  const previewUrl = file.previewUrl || file.downloadUrl
  if (!previewUrl) throw new Error(canonicalPreviewUnavailableMessage(item.resource_role))
  return asDownloadInfo(file, previewUrl, true)
}

export async function resolveCanonicalDownload(
  item: FlatResourceItem,
  getGroup: (groupID: number) => Promise<ResourceGroup> = resourceGroupsApi.get,
  signal?: AbortSignal,
): Promise<ResourceDownloadInfo> {
  if (item.task_asset_id) return resourceGroupsApi.downloadTaskAsset(item.task_asset_id, signal)
  const group = await getGroup(item.group_id)
  const file = canonicalFileFromGroup(group, item)
  if (!file.downloadUrl) throw new Error('当前账号没有该资源的下载权限，或资源已不可用')
  return asDownloadInfo(file, file.downloadUrl, false)
}

function fromResourceFile(file: ResourceFile): CanonicalResourceFile {
  return {
    taskAssetId: file.task_asset_id || undefined,
    filename: file.file_name,
    mimeType: file.mime_type,
    fileSize: file.file_size,
    previewUrl: file.preview_url,
    downloadUrl: file.download_url,
  }
}

function fromReference(reference: ResourceReference): CanonicalResourceFile {
  return {
    taskAssetId: reference.formal_task_asset_id || undefined,
    filename: reference.file_name || '参考图',
    mimeType: reference.mime_type,
    fileSize: reference.file_size,
    previewUrl: reference.preview_url,
    downloadUrl: reference.download_url,
  }
}

function asDownloadInfo(file: CanonicalResourceFile, url: string, previewAvailable: boolean): ResourceDownloadInfo {
  return {
    download_mode: 'direct',
    download_url: url,
    preview_available: previewAvailable,
    filename: file.filename,
    file_size: Number(file.fileSize || 0),
    mime_type: file.mimeType,
  }
}
