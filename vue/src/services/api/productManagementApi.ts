import http from '@/services/http'

export type ProductImageSource =
  | 'manual'
  | 'erp_product_image'
  | 'delivery'
  | 'derived_preview'
  | 'task_reference'
  | 'auto_on_close'
  | 'missing'

export type ProductSyncStatus =
  | 'pending_sync'
  | 'queued'
  | 'syncing'
  | 'synced'
  | 'failed'
  | 'cooling_down'
  | 'waiting_image'

export interface ProductManagementCostTrace {
  rule_name?: string
  rule_source?: string
  matched_rule_version?: number
  prefill_source?: string
  requires_manual_review?: boolean
  manual_cost_override?: boolean
  manual_cost_override_reason?: string
  input_snapshot?: Record<string, unknown>
  calculation_snapshot?: Record<string, unknown>
  snapshot_at?: string
}

export interface ProductManagementAreaTrace {
  width_m?: number
  height_m?: number
  quantity?: number
  area_m2?: number
  formula?: string
  source?: string
  source_label?: string
  confidence?: string
  warning?: string
}

export interface ProductManagementRecord {
  id: number
  record_key: string
  task_id: number
  task_sku_item_id?: number
  task_no: string
  task_type?: string
  source_mode?: string
  sku_code: string
  product_i_id: string
  erp_i_id?: string
  category_name?: string
  product_family?: string
  product_name: string
  cost_price?: number | null
  cost_trace?: ProductManagementCostTrace | null
  spec_text?: string
  size_text?: string
  area_trace?: ProductManagementAreaTrace | null
  creator_id: number
  creator_name: string
  task_created_at: string
  image_source: ProductImageSource
  image_source_label: string
  image_selection_mode: 'auto' | 'manual'
  image_asset_id?: number
  image_asset_version_id?: number
  image_preview_url?: string
  image_filename?: string
  image_mime_type?: string
  image_missing_reason?: string
  image_sync_source?: ProductImageSource
  erp_sync_status: ProductSyncStatus
  base_sync_status?: ProductSyncStatus
  image_sync_status?: ProductSyncStatus
  last_erp_synced_at?: string
  last_erp_checked_at?: string
  last_base_synced_at?: string
  last_image_synced_at?: string
  sync_cooldown_until?: string
  last_sync_error?: string
  base_sync_error?: string
  image_sync_error?: string
  image_required?: boolean
  can_maintain_image: boolean
  can_cross_task_select: boolean
  can_sync_erp: boolean
  can_force_override: boolean
  created_at: string
  updated_at: string
}

export interface ProductImageCandidate {
  asset_id: number
  asset_version_id: number
  task_id: number
  task_no: string
  sku_code?: string
  source: ProductImageSource
  source_label: string
  preview_url?: string
  file_name: string
  mime_type?: string
  created_at: string
}

export interface ProductManagementListParams {
  keyword?: string
  display_scope?: 'combo' | 'single' | 'all'
  image_source?: ProductImageSource | ''
  sync_status?: ProductSyncStatus | ''
  base_sync_status?: ProductSyncStatus | ''
  image_sync_status?: ProductSyncStatus | ''
  cost_status?: 'missing' | 'ready' | ''
  issue_scope?: 'attention' | 'all'
  creator_id?: number
  page?: number
  page_size?: number
}

export interface ProductManagementPagination {
  page: number
  page_size: number
  total: number
}

export interface ProductManagementListResponse {
  data: ProductManagementRecord[]
  pagination: ProductManagementPagination
}

export interface ProductManagementComboChild {
  record: ProductManagementRecord
  quantity: number
}

export interface ProductManagementComboGroup {
  group_key: string
  group_type: 'combo' | 'single'
  combo_sku_code?: string
  combo_name?: string
  combo_short_name?: string
  erp_i_id?: string
  entity_sku_id?: string
  pic_url?: string
  brand?: string
  vc_name?: string
  properties_value?: string
  enabled?: boolean | null
  cost_price?: number | null
  sale_price?: number | null
  weight?: number | null
  sku_qty?: number | null
  erp_created_at?: string
  modified_at?: string
  last_synced_at?: string
  children: ProductManagementComboChild[]
}

export interface ProductManagementComboSyncSummary {
  id: number
  window_begin: string
  window_end: string
  page_index: number
  page_size: number
  status: string
  last_success_at?: string
  next_retry_at?: string
  last_error?: string
  processed_items: number
}

export interface ProductManagementComboTreeResponse extends ProductManagementListResponse {
  groups: ProductManagementComboGroup[]
  combo_sync_summary?: ProductManagementComboSyncSummary
}

export type ProductCostIssueGroupCode = 'cannot_calculate' | 'possibly_wrong' | 'looks_abnormal'

export type ProductCostIssueTagCode =
  | 'cost_missing'
  | 'manual_quote'
  | 'erp_mismatch'
  | 'rule_version_outdated'
  | 'unbound_iid'
  | 'area_spec_abnormal'

export interface ProductCostIssueCount {
  code: ProductCostIssueGroupCode | ProductCostIssueTagCode | string
  label: string
  count: number
}

export interface ProductCostDashboardResponse {
  total_count: number
  groups: ProductCostIssueCount[]
  tags: ProductCostIssueCount[]
  legacy_fallback_ratio?: number
  unbound_iid_count?: number
  generated_at?: string
}

export interface CostRuleBinding {
  id: number
  i_id_raw: string
  normalized_i_id: string
  rule_group: string
  display_name: string
  source?: string
  is_active: boolean
  created_by?: number
  updated_by?: number
  created_at?: string
  updated_at?: string
}

export interface CostRuleBindingListParams {
  rule_group?: string
  keyword?: string
  i_id?: string
  is_active?: boolean
  page?: number
  page_size?: number
}

export interface CostRuleBindingListResponse {
  data: CostRuleBinding[]
  pagination?: ProductManagementPagination
}

export interface UpsertCostRuleBindingPayload {
  i_id_raw?: string
  normalized_i_id?: string
  rule_group?: string
  display_name?: string
  source?: string
  is_active?: boolean
}

export interface UnboundCostRuleCandidate {
  i_id_raw?: string
  display_i_id?: string
  normalized_i_id: string
  product_i_id?: string
  erp_i_id?: string
  suggested_rule_group?: string
  suggested_display_name?: string
  match_count?: number
  sku_count?: number
  task_count?: number
  last_seen_at?: string
}

export interface UnboundCostRuleCandidateListResponse {
  data: UnboundCostRuleCandidate[]
  pagination?: ProductManagementPagination
}

export interface CostRuleGroupOption {
  rule_group: string
  display_name: string
  active_rule_count?: number
}

export type CostRecalculationRunMode = 'single' | 'explicit' | 'all_matching'

export type CostRecalculationRunStatus =
  | 'previewing'
  | 'previewed'
  | 'preview_failed'
  | 'applying'
  | 'applied'
  | 'partially_applied'
  | 'apply_failed'
  | 'erp_syncing'
  | 'erp_synced'
  | 'partially_erp_synced'
  | 'cancelled'

export type CostRecalculationRunItemStatus =
  | 'previewed'
  | 'applied'
  | 'skipped'
  | 'conflict'
  | 'failed'
  | 'erp_queued'
  | 'erp_synced'
  | 'erp_failed'

export interface CreateCostRecalculationRunRequest {
  mode: CostRecalculationRunMode
  record_ids?: number[]
  product_management_record_id?: number
  filters?: Record<string, unknown>
  issue_group?: ProductCostIssueGroupCode | ''
  issue_tag?: ProductCostIssueTagCode | ''
  reason?: string
}

export interface CostRecalculationRunSummary {
  total_count?: number
  previewed_count?: number
  applied_count?: number
  skipped_count?: number
  conflict_count?: number
  failed_count?: number
  erp_syncable_count?: number
  erp_synced_count?: number
  erp_failed_count?: number
  task_count?: number
  confirmation_text?: string
}

export interface CostRecalculationRunItem {
  id: number
  run_id: number
  product_management_record_id: number
  task_id?: number
  task_no?: string
  sku_code?: string
  erp_i_id?: string
  product_i_id?: string
  normalized_i_id?: string
  old_cost_price?: number | null
  new_cost_price?: number | null
  cost_delta?: number | null
  old_rule_id?: number | null
  new_rule_id?: number | null
  new_rule_version?: number | null
  match_mode?: string
  status: CostRecalculationRunItemStatus | string
  skip_reason?: string
  conflict_reason?: string
  preview_snapshot_json?: Record<string, unknown>
  apply_snapshot_json?: Record<string, unknown>
}

export interface CostRecalculationRun {
  id: number
  run_no?: string
  status: CostRecalculationRunStatus | string
  mode: CostRecalculationRunMode | string
  filters?: Record<string, unknown>
  summary?: CostRecalculationRunSummary
  items?: CostRecalculationRunItem[]
  pagination?: ProductManagementPagination
  created_by?: number
  created_at?: string
  previewed_at?: string
  applied_at?: string
  erp_synced_at?: string
  cancelled_at?: string
}

export interface CostRecalculationRunListResponse {
  data: CostRecalculationRun[]
  pagination?: ProductManagementPagination
}

export interface ApplyCostRecalculationRunResponse {
  run: CostRecalculationRun
  summary?: CostRecalculationRunSummary
  results?: CostRecalculationRunItem[]
}

export interface SyncCostRecalculationRunERPResponse {
  run: CostRecalculationRun
  summary?: CostRecalculationRunSummary
  results?: CostRecalculationRunItem[]
}

export interface CostRulePreviewRequest {
  category_code?: string
  rule_group?: string
  width?: number | null
  height?: number | null
  area?: number | null
  quantity?: number | null
  process?: string
  notes?: string
}

export interface CostRulePreviewResponse {
  matched_rule_id?: number | null
  matched_rule_version?: number | null
  estimated_cost?: number | null
  rule_source?: string
  governance_status?: string
  requires_manual_review?: boolean
  explanation?: string
  applied_rules?: Array<{ rule_id?: number; rule_name?: string; rule_version?: number; rule_type?: string }>
}

export const productManagementApi = {
  async list(params: ProductManagementListParams, signal?: AbortSignal): Promise<ProductManagementListResponse> {
    const { data } = await http.get<ProductManagementListResponse>('/v1/product-management', { params, signal })
    return data
  },

  async listComboTree(params: ProductManagementListParams, signal?: AbortSignal): Promise<ProductManagementComboTreeResponse> {
    const { data } = await http.get<ProductManagementComboTreeResponse>('/v1/product-management/combo-tree', { params, signal })
    return data
  },

  async listByTask(taskId: number): Promise<ProductManagementRecord[]> {
    const { data } = await http.get<{ data: ProductManagementRecord[] }>(`/v1/tasks/${taskId}/product-management`)
    return data.data ?? []
  },

  async listImageCandidates(recordId: number): Promise<ProductImageCandidate[]> {
    const { data } = await http.get<{ data: ProductImageCandidate[] }>(
      `/v1/product-management/${recordId}/image-candidates`,
    )
    return data.data ?? []
  },

  async reparseImage(recordId: number): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/reparse-image`,
    )
    return data.data
  },

  async setManualImage(recordId: number, assetId: number): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/image`,
      { asset_id: assetId },
    )
    return data.data
  },

  async requestSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/sync-request`,
      { force },
    )
    return data.data
  },

  async requestBaseSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/base-sync-request`,
      { force },
    )
    return data.data
  },

  async requestImageSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/image-sync-request`,
      { force },
    )
    return data.data
  },

  async getCostDashboard(params?: Record<string, unknown>, signal?: AbortSignal): Promise<ProductCostDashboardResponse> {
    const { data } = await http.get<{ data?: ProductCostDashboardResponse } | ProductCostDashboardResponse>(
      '/v1/product-management/cost-dashboard',
      { params, signal },
    )
    return ('data' in data && data.data ? data.data : data) as ProductCostDashboardResponse
  },

  async listCostRuleBindings(
    params?: CostRuleBindingListParams,
    signal?: AbortSignal,
  ): Promise<CostRuleBindingListResponse> {
    const { data } = await http.get<CostRuleBindingListResponse>('/v1/cost-rule-bindings', { params, signal })
    return data
  },

  async createCostRuleBinding(payload: UpsertCostRuleBindingPayload, signal?: AbortSignal): Promise<CostRuleBinding> {
    const { data } = await http.post<{ data?: CostRuleBinding } | CostRuleBinding>('/v1/cost-rule-bindings', payload, { signal })
    return ('data' in data && data.data ? data.data : data) as CostRuleBinding
  },

  async updateCostRuleBinding(
    id: number | string,
    payload: UpsertCostRuleBindingPayload,
    signal?: AbortSignal,
  ): Promise<CostRuleBinding> {
    const { data } = await http.patch<{ data?: CostRuleBinding } | CostRuleBinding>(
      `/v1/cost-rule-bindings/${encodeURIComponent(String(id))}`,
      payload,
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRuleBinding
  },

  async listUnboundCostRuleCandidates(
    params?: { keyword?: string; page?: number; page_size?: number },
    signal?: AbortSignal,
  ): Promise<UnboundCostRuleCandidateListResponse> {
    const { data } = await http.get<UnboundCostRuleCandidateListResponse>('/v1/cost-rule-bindings/unbound-candidates', { params, signal })
    return data
  },

  async listCostRuleGroups(signal?: AbortSignal): Promise<CostRuleGroupOption[]> {
    const { data } = await http.get<{ data?: Array<Record<string, unknown>> }>('/v1/cost-rules', {
      params: { is_active: true, page: 1, page_size: 500 },
      signal,
    })
    const groups = new Map<string, CostRuleGroupOption>()
    for (const rule of data.data ?? []) {
      const ruleGroup = String(rule.category_code ?? '').trim()
      if (!ruleGroup) continue
      const existing = groups.get(ruleGroup)
      const ruleName = String(rule.rule_name ?? '').trim()
      const displayName = String(rule.product_family ?? rule.display_name ?? ruleName ?? ruleGroup).trim() || ruleGroup
      if (existing) {
        existing.active_rule_count = (existing.active_rule_count ?? 0) + 1
      } else {
        groups.set(ruleGroup, {
          rule_group: ruleGroup,
          display_name: displayName,
          active_rule_count: 1,
        })
      }
    }
    return Array.from(groups.values()).sort((a, b) => a.display_name.localeCompare(b.display_name, 'zh-CN'))
  },

  async createCostRecalculationRun(
    payload: CreateCostRecalculationRunRequest,
    signal?: AbortSignal,
  ): Promise<CostRecalculationRun> {
    const { data } = await http.post<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      '/v1/product-management/cost-recalculation-runs',
      payload,
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRecalculationRun
  },

  async listCostRecalculationRuns(
    params?: { page?: number; page_size?: number; status?: string },
    signal?: AbortSignal,
  ): Promise<CostRecalculationRunListResponse> {
    const { data } = await http.get<CostRecalculationRunListResponse>('/v1/product-management/cost-recalculation-runs', { params, signal })
    return data
  },

  async getCostRecalculationRun(
    id: number | string,
    params?: { page?: number; page_size?: number },
    signal?: AbortSignal,
  ): Promise<CostRecalculationRun> {
    const { data } = await http.get<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      `/v1/product-management/cost-recalculation-runs/${encodeURIComponent(String(id))}`,
      { params, signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRecalculationRun
  },

  async applyCostRecalculationRun(id: number | string, signal?: AbortSignal): Promise<ApplyCostRecalculationRunResponse> {
    const { data } = await http.post<ApplyCostRecalculationRunResponse>(
      `/v1/product-management/cost-recalculation-runs/${encodeURIComponent(String(id))}/apply`,
      {},
      { signal },
    )
    return data
  },

  async syncCostRecalculationRunERP(id: number | string, signal?: AbortSignal): Promise<SyncCostRecalculationRunERPResponse> {
    const { data } = await http.post<SyncCostRecalculationRunERPResponse>(
      `/v1/product-management/cost-recalculation-runs/${encodeURIComponent(String(id))}/sync-erp`,
      {},
      { signal },
    )
    return data
  },

  async cancelCostRecalculationRun(id: number | string, signal?: AbortSignal): Promise<CostRecalculationRun> {
    const { data } = await http.post<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      `/v1/product-management/cost-recalculation-runs/${encodeURIComponent(String(id))}/cancel`,
      {},
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRecalculationRun
  },

  async previewCostRule(payload: CostRulePreviewRequest, signal?: AbortSignal): Promise<CostRulePreviewResponse> {
    const { data } = await http.post<{ data?: CostRulePreviewResponse } | CostRulePreviewResponse>(
      '/v1/cost-rules/preview',
      {
        ...payload,
        category_code: payload.category_code ?? payload.rule_group,
      },
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRulePreviewResponse
  },
}
