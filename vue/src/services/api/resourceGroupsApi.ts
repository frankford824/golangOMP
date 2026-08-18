import http from '@/services/http'
import { generateActionId } from '@/utils/uuid'

export type ResourceMode = 'single' | 'set'

export interface ResourceFile {
  revision_item_id?: number
  task_asset_id: number
  file_name: string
  mime_type?: string
  file_size?: number | null
  availability?: 'available' | 'historical_unavailable'
  unavailable_reason?: 'legacy_original_object_missing'
  download_url?: string
  preview_url?: string
  download_expires_at?: string | null
}

export interface ResourceRevisionItem {
  id: number
  revision_id: number
  task_asset_id: number
  sort_order: number
  item_name?: string
  file?: ResourceFile
}

export interface ResourceReference {
  id?: number
  revision_id?: number
  reference_file_ref_id?: number
  formal_task_asset_id?: number | null
  sort_order?: number
  ref_id?: string
  file_name?: string
  scope?: string
  mime_type?: string
  file_size?: number | null
  availability?: 'available' | 'historical_unavailable'
  unavailable_reason?: 'legacy_original_object_missing'
  download_url?: string
  preview_url?: string
}

export interface ResourceRevision {
  id: number
  group_id: number
  revision_no: number
  status: 'draft' | 'submitted' | 'finalized' | 'rejected' | 'superseded'
  mode: ResourceMode
  source_task_asset_id?: number | null
  source_stage: 'design' | 'audit' | 'retouch' | 'migration' | 'reopen'
  created_by: number
  created_by_name?: string
  reason?: string
  legacy_migration: boolean
  evidence_summary?: ResourceRevisionEvidence | null
  source_file?: ResourceFile | null
  items: ResourceRevisionItem[]
  references: ResourceReference[]
  submitted_at?: string | null
  finalized_at?: string | null
  created_at: string
}

export interface ResourceRevisionEvidence {
  schema_version: 'migration_v2'
  manifest_sha256: string
  confidence: 'confirmed_auto' | 'proposed_review' | 'hard_blocked'
  confirmed_by: number
  confirmed_at: string
  evidence_event_count: number
  evidence_event_ids: string[]
  evidence_event_ids_complete: boolean
  upload_session_ids: string[]
  upload_sessions_known: boolean
  business_reason?: string
  business_reason_sha256?: string
}

export interface ResourceSKUProfile {
  id: number
  task_id: number
  task_sku_item_id?: number | null
  sku_code: string
  product_i_id?: string
  erp_i_id?: string
  category_name?: string
  product_family?: string
  product_name?: string
  creator_name?: string
  task_no?: string
  combo_sku_codes?: string[]
  cost_price?: number | null
  cost_trace?: {
    rule_name?: string
    rule_source?: string
    matched_rule_version?: number | null
    requires_manual_review?: boolean
    manual_cost_override?: boolean
    manual_cost_override_reason?: string
    input_snapshot?: Record<string, unknown>
    calculation_snapshot?: Record<string, unknown>
  } | null
  spec_text?: string
  size_text?: string
  area_trace?: {
    width_m?: number | null
    height_m?: number | null
    quantity?: number | null
    area_m2?: number | null
    formula?: string
    source_label?: string
    warning?: string
  } | null
  erp_sync_status?: string
  base_sync_status?: string
  image_sync_status?: string
  last_erp_synced_at?: string | null
  last_erp_checked_at?: string | null
}

export interface ProductCostReconciliation {
  product_management_record_id: number
  sku_code: string
  system_cost_price?: number | null
  erp_cost_price?: number | null
  cost_delta?: number | null
  status: 'matched' | 'mismatched' | 'system_missing' | 'erp_missing' | 'unavailable'
  checked_at: string
  message: string
  system_trace?: ResourceSKUProfile['cost_trace']
  erp_i_id?: string
  erp_product_name?: string
}

export interface ResourceGroup {
  id: number
  task_id: number
  scope_kind: 'task' | 'sku' | 'retouch_requirement'
  task_sku_item_id?: number | null
  retouch_requirement_id?: number | null
  working_revision_id?: number | null
  finalized_revision_id?: number | null
  lock_version: number
  migration_incomplete: boolean
  migration_issue?: string
  task_no?: string
  sku_code?: string
  product_name?: string
  creator_id?: number
  creator_name?: string
  business_lane?: string
  sku_profile?: ResourceSKUProfile | null
  working_revision?: ResourceRevision | null
  finalized_revision?: ResourceRevision | null
}

export interface ResourceBundle {
  task_id: number
  workflow_revision: number
  groups: ResourceGroup[]
}

export interface FlatResourceItem {
  group_id: number
  task_id: number
  revision_id: number
  resource_item_id: number
  task_asset_id?: number
  sort_order: number
  task_no?: string
  task_type: string
  sku_code?: string
  resource_role: 'reference' | 'source' | 'final'
  file_name: string
  mime_type?: string
  resource_owner_id?: number
  resource_owner_name?: string
  resource_created_at: string
  preview_url?: string
  download_url?: string
}

export interface ResourceGroupListResult {
  items: ResourceGroup[]
  flat_items?: FlatResourceItem[]
  view_mode?: 'group' | 'flat'
  page: number
  page_size: number
  total: number
}

export interface ResourceRevisionListResult {
  items: ResourceRevision[]
  working_revision_id?: number | null
  finalized_revision_id?: number | null
  page: number
  page_size: number
  total: number
}

export interface ResourceGroupDownloadItem {
  group_id: number
  revision_id: number
  revision_item_id: number
  task_id: number
  sku_code?: string
  sort_order: number
  filename: string
  mime_type?: string
  file_size?: number | null
  download_url: string
}

export interface ResourceDownloadInfo {
  download_mode: string
  download_url?: string | null
  access_hint?: string | null
  preview_available?: boolean
  filename: string
  file_size: number
  mime_type?: string
  expires_at?: string | null
  items?: ResourceDownloadInfo[]
}

export interface ResourceGroupSubmission {
  group_id: number
  expected_group_lock_version: number
  mode: ResourceMode
  source_task_asset_id?: number
  final_task_asset_ids: number[]
  reference_file_ref_ids?: number[]
}

const unwrap = <T>(response: { data?: { data?: T } | T }): T => {
  const body = response.data as { data?: T } | T
  return body && typeof body === 'object' && 'data' in body ? (body.data as T) : (body as T)
}

export const resourceGroupsApi = {
  async taskBundle(taskId: number): Promise<ResourceBundle> {
    return unwrap(await http.get(`/v1/tasks/${taskId}/resource-bundle`))
  },
  async list(params: {
    task_id?: number
    sku_code?: string
    task_no?: string
    creator_id?: number | string
    resource_role?: 'reference' | 'source' | 'final' | ''
    format_category?: 'image' | 'design' | 'pdf' | 'video' | 'archive'
    business_lane?: 'normal' | 'customization'
    q?: string
    file_format?: string
    created_from?: string
    created_to?: string
    resource_owner_id?: number | string
    task_type?: string
    page?: number
    page_size?: number
  } = {}): Promise<ResourceGroupListResult> {
    return unwrap(await http.get('/v1/resource-groups', { params }))
  },
  async get(id: number): Promise<ResourceGroup> {
    return unwrap(await http.get(`/v1/resource-groups/${id}`))
  },
  async costReconciliation(id: number): Promise<ProductCostReconciliation> {
    return unwrap(await http.get(`/v1/resource-groups/${id}/cost-reconciliation`))
  },
  async revisions(id: number, params: { page?: number; page_size?: number } = {}): Promise<ResourceRevisionListResult> {
    return unwrap(await http.get(`/v1/resource-groups/${id}/revisions`, { params }))
  },
  async batchDownload(groupIds: number[]): Promise<{ items: ResourceGroupDownloadItem[] }> {
    return unwrap(await http.post('/v1/resource-groups/batch-download', { group_ids: groupIds }))
  },
  async previewTaskAsset(taskAssetId: number, signal?: AbortSignal): Promise<ResourceDownloadInfo> {
    return unwrap(await http.get(`/v1/task-assets/${taskAssetId}/preview`, { signal }))
  },
  async downloadTaskAsset(taskAssetId: number, signal?: AbortSignal): Promise<ResourceDownloadInfo> {
    return unwrap(await http.get(`/v1/task-assets/${taskAssetId}/download`, { signal }))
  },
  async submitDesign(taskId: number, bundle: ResourceBundle, groups: ResourceGroupSubmission[]): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/submit-design`, {
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: generateActionId(),
      groups,
    }))
  },
  async auditDecision(taskId: number, bundle: ResourceBundle, decision: 'approve' | 'return_to_design', reason: string, groups: ResourceGroupSubmission[] = []): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/audit/decision`, {
      decision,
      reason,
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: generateActionId(),
      groups,
    }))
  },
  async reopen(taskId: number, bundle: ResourceBundle, target: 'design' | 'audit' | 'retouch', reason: string): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/reopen`, {
      target,
      reason,
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: generateActionId(),
    }))
  },
}
