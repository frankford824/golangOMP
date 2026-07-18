import http from '@/services/http'

export type ResourceMode = 'single' | 'set'

export interface ResourceFile {
  revision_item_id?: number
  task_asset_id: number
  file_name: string
  mime_type?: string
  file_size?: number | null
  download_url?: string
  preview_url?: string
}

export interface ResourceRevisionItem {
  id: number
  revision_id: number
  task_asset_id: number
  sort_order: number
  item_name?: string
  file?: ResourceFile
}

export interface ResourceRevision {
  id: number
  group_id: number
  revision_no: number
  status: 'draft' | 'submitted' | 'finalized' | 'rejected' | 'superseded'
  mode: ResourceMode
  source_stage: 'design' | 'audit' | 'retouch' | 'migration' | 'reopen'
  source_file?: ResourceFile | null
  items: ResourceRevisionItem[]
  references: Array<Record<string, unknown>>
}

export interface ResourceGroup {
  id: number
  task_id: number
  scope_kind: 'task' | 'sku' | 'retouch_requirement'
  task_sku_item_id?: number | null
  retouch_requirement_id?: number | null
  lock_version: number
  migration_incomplete: boolean
  migration_issue?: string
  task_no?: string
  sku_code?: string
  business_lane?: string
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
  task_no?: string
  sku_code?: string
  resource_role: 'reference' | 'source' | 'final'
  file_name: string
  mime_type?: string
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
    q?: string
    format_category?: string
    business_lane?: string
    page?: number
    page_size?: number
  } = {}): Promise<ResourceGroupListResult> {
    return unwrap(await http.get('/v1/resource-groups', { params }))
  },
  async get(id: number): Promise<ResourceGroup> {
    return unwrap(await http.get(`/v1/resource-groups/${id}`))
  },
  async batchDownload(groupIds: number[]): Promise<{ items: Array<ResourceFile & { group_id: number; sort_order: number }> }> {
    return unwrap(await http.post('/v1/resource-groups/batch-download', { group_ids: groupIds }))
  },
  async submitDesign(taskId: number, bundle: ResourceBundle, groups: ResourceGroupSubmission[]): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/submit-design`, {
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: crypto.randomUUID(),
      groups,
    }))
  },
  async auditDecision(taskId: number, bundle: ResourceBundle, decision: 'approve' | 'return_to_design', reason: string, groups: ResourceGroupSubmission[] = []): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/audit/decision`, {
      decision,
      reason,
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: crypto.randomUUID(),
      groups,
    }))
  },
  async reopen(taskId: number, bundle: ResourceBundle, target: 'design' | 'audit' | 'retouch', reason: string): Promise<ResourceBundle> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/reopen`, {
      target,
      reason,
      expected_workflow_revision: bundle.workflow_revision,
      idempotency_key: crypto.randomUUID(),
    }))
  },
}
