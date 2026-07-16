/**
 * GET /v1/search — shapes from docs/openapi.yaml (SearchResultGroup + envelope).
 */

import { TASK_STATUS_LABELS } from '@/domain/enums/task-status'
import type { LegacyTaskStatus } from '@/domain/types/task'

export type V1GlobalSearchScope = 'all' | 'tasks' | 'assets' | 'products' | 'users'

export interface V1GlobalSearchTaskHit {
  id: number
  task_no: string
  title?: string | null
  task_status?: string | null
  priority?: string | null
  task_type?: string | null
  sku_code?: string | null
  primary_sku_code?: string | null
  i_id?: string | null
  owner_department?: string | null
  owner_team?: string | null
  owner_org_team?: string | null
  creator_id?: number | null
  creator_name?: string | null
  designer_id?: number | null
  designer_name?: string | null
  created_at?: string | null
  deadline_at?: string | null
  highlight?: string | null
}

export interface V1GlobalSearchAssetHit {
  asset_id: number
  resource_group_id?: number | null
  finalized_revision_id?: number | null
  task_no?: string | null
  sku_code?: string | null
  mode?: 'single' | 'set' | string | null
  final_item_count?: number | null
  resource_id?: string | null
  file_name: string
  source_module_key?: string | null
  task_id?: number | null
  flow_review_status?: string | null
  usable_state?: string | null
  usable_label?: string | null
  source_type?: 'system' | 'external' | string | null
  source_label?: string | null
  external_kind?: string | null
  external_mount_path?: string | null
  external_driver?: string | null
}

export interface V1GlobalSearchProductHit {
  erp_code: string
  product_name: string
  i_id?: string | null
  category?: string | null
}

export interface V1GlobalSearchUserHit {
  user_id: number
  username: string
  department_name?: string | null
}

export interface V1SearchResultGroup {
  tasks?: V1GlobalSearchTaskHit[]
  assets?: V1GlobalSearchAssetHit[]
  products?: V1GlobalSearchProductHit[]
  users?: V1GlobalSearchUserHit[]
}

export interface V1GlobalSearchResponse {
  query: string
  results: V1SearchResultGroup
}

/** One row in the global search overlay lists (navigation + display). */
export type GlobalSearchOverlayHit = {
  id: string
  type: 'task' | 'asset' | 'product' | 'user'
  title: string
  subtitle: string
  /** 任务状态中文（来自 API task_status，未知则回退原字符串） */
  statusLabel?: string
  badgeLabel?: string
}

export type GlobalSearchOverlayBundle = {
  tasks: GlobalSearchOverlayHit[]
  assets: GlobalSearchOverlayHit[]
  products: GlobalSearchOverlayHit[]
  users: GlobalSearchOverlayHit[]
}

export function emptyGlobalSearchBundle(): GlobalSearchOverlayBundle {
  return { tasks: [], assets: [], products: [], users: [] }
}

function taskStatusSearchLabel(raw: string | null | undefined): string | undefined {
  if (raw == null || raw === '') return undefined
  return TASK_STATUS_LABELS[raw as LegacyTaskStatus] ?? raw
}

function taskHitToOverlay(row: V1GlobalSearchTaskHit): GlobalSearchOverlayHit {
  return {
    id: String(row.id),
    type: 'task',
    title: row.task_no,
    subtitle: row.title ?? '',
    statusLabel: taskStatusSearchLabel(row.task_status),
  }
}

function assetHitToOverlay(row: V1GlobalSearchAssetHit): GlobalSearchOverlayHit {
  if (row.source_type === 'task_resource_group') {
    const groupID = Number(row.resource_group_id || 0)
    const modeLabel = row.mode === 'set' ? '套装' : '单图'
    const itemCount = Math.max(0, Number(row.final_item_count || 0))
    const identity = [row.task_no, row.sku_code].filter((item) => typeof item === 'string' && item.trim()).join(' · ')
    return {
      id: String(groupID),
      type: 'asset',
      title: identity || '任务资源',
      subtitle: modeLabel + ' · ' + itemCount + ' 张最终成品',
      badgeLabel: modeLabel,
    }
  }
  const usableLabel =
    typeof row.usable_label === 'string' && row.usable_label.trim()
      ? row.usable_label.trim()
      : ''
  const sourceLabel =
    typeof row.source_label === 'string' && row.source_label.trim()
      ? row.source_label.trim()
      : row.source_type === 'external' || row.source_type === 'external_asset'
      ? '外部资源'
      : '系统资源'
  const sub = [sourceLabel, row.external_mount_path].filter(Boolean).join(' · ')
  const externalID = String(row.resource_id || row.asset_id)
  return {
    id: externalID.startsWith('ext-') ? externalID : 'ext-' + externalID,
    type: 'asset',
    title: row.file_name,
    subtitle: sub,
    badgeLabel: usableLabel || sourceLabel,
  }
}

function productHitToOverlay(row: V1GlobalSearchProductHit): GlobalSearchOverlayHit {
  const sub = [row.erp_code, row.category].filter((x) => x != null && String(x).trim() !== '').join(' · ')
  return {
    id: row.erp_code,
    type: 'product',
    title: row.product_name,
    subtitle: sub,
  }
}

function userHitToOverlay(row: V1GlobalSearchUserHit): GlobalSearchOverlayHit {
  return {
    id: String(row.user_id),
    type: 'user',
    title: row.username,
    subtitle: row.department_name ?? '',
  }
}

export function mapV1SearchResultsToOverlayBundle(group: V1SearchResultGroup): GlobalSearchOverlayBundle {
  return {
    tasks: (group.tasks ?? []).map(taskHitToOverlay),
    assets: (group.assets ?? []).map(assetHitToOverlay),
    products: (group.products ?? []).map(productHitToOverlay),
    users: (group.users ?? []).map(userHitToOverlay),
  }
}

export function parseV1GlobalSearchResponse(data: unknown): V1GlobalSearchResponse | null {
  if (!data || typeof data !== 'object') return null
  const o = data as Record<string, unknown>
  if (typeof o.query !== 'string' || !o.results || typeof o.results !== 'object') return null
  return o as unknown as V1GlobalSearchResponse
}
