import http from '@/services/http'

export interface ProductManagementPagination {
  page: number
  page_size: number
  total: number
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

export interface ProductCostLegacyFallbackTrendItem {
  date: string
  total_records: number
  unbound_iid_count: number
  legacy_fallback_ratio: number
}

export interface ProductCostDashboardResponse {
  total_count: number
  total_records?: number
  groups: ProductCostIssueCount[]
  tags: ProductCostIssueCount[]
  legacy_fallback_ratio?: number
  unbound_iid_count?: number
  legacy_fallback_enabled?: boolean
  legacy_fallback_mode?: 'warn' | 'disabled' | string
  legacy_fallback_warning?: string
  legacy_fallback_trend?: ProductCostLegacyFallbackTrendItem[]
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
  suggested_rule_groups: string[]
  suggested_group_count: number
  mapping_confidence: 'unique' | 'conflict' | 'unmatched'
  suggested_display_name?: string
  match_count?: number
  average_cost_price?: number | null
  example_sku_code?: string
  example_task_no?: string
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

export interface CostRuleSummary {
  id?: number
  rule_id?: number
  rule_name?: string
  category_code?: string
}

function normalizeCostRuleGroupName(ruleName: string): string {
  let label = ruleName.trim()
  if (!label) return ''
  label = label.replace(/(?:基础单价|小面积保底|小面积附加|开槽拼接加价|开槽加价|特殊工艺加价|工艺加价|保底|附加|加价)$/u, '')
  return label.trim()
}

function isReadableCostRuleGroupName(label: string): boolean {
  const text = label.trim()
  if (!text) return false
  const normalized = text.toLowerCase()
  if (['board', 'cloth', 'material', 'paper'].includes(normalized)) return false
  return true
}

function costRuleGroupNameScore(label: string): number {
  const text = label.trim()
  if (!isReadableCostRuleGroupName(text)) return 0
  let score = 1
  if (/[\u4e00-\u9fa5]/u.test(text)) score += 4
  if (/(KT|喷绘|写真|覆膜|布|板|纸|灯片|背胶)/iu.test(text)) score += 2
  return score
}

function costRuleGroupDisplayName(rule: Record<string, unknown>, ruleGroup: string): string {
  const candidates = [
    normalizeCostRuleGroupName(String(rule.rule_name ?? '')),
    String(rule.display_name ?? '').trim(),
    normalizeCostRuleGroupName(String(rule.remark ?? '')),
    String(rule.product_family ?? '').trim(),
    ruleGroup,
  ]
  return candidates.find(isReadableCostRuleGroupName) ?? ruleGroup
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
  sku_codes?: string[]
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
  category_id?: number | null
  category_code?: string
  rule_group?: string
  width?: number | null
  height?: number | null
  area?: number | null
  quantity?: number | null
  process?: string
  notes?: string
  erp_i_id?: string
  product_i_id?: string
}

export interface CostRulePreviewResponse {
  matched_rule?: {
    rule_id?: number
    rule_name?: string
    rule_version?: number
    rule_type?: string
    priority?: number
    source?: string
    governance_status?: string
  } | null
  matched_rule_id?: number | null
  matched_rule_version?: number | null
  estimated_cost?: number | null
  rule_source?: string
  governance_status?: string
  requires_manual_review?: boolean
  explanation?: string
  match_mode?: 'binding_erp_i_id' | 'binding_product_i_id' | 'legacy_alias' | 'no_match' | string
  erp_i_id?: string
  product_i_id?: string
  normalized_i_id?: string
  rule_group?: string
  legacy_alias_fallback?: boolean
  applied_rules?: Array<{ rule_id?: number; rule_name?: string; rule_version?: number; rule_type?: string; priority?: number; source?: string; governance_status?: string }>
}

export const costManagementApi = {
  async getCostDashboard(params?: Record<string, unknown>, signal?: AbortSignal): Promise<ProductCostDashboardResponse> {
    const { data } = await http.get<{ data?: ProductCostDashboardResponse } | ProductCostDashboardResponse>(
      '/v1/cost-management/dashboard',
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
      const displayName = costRuleGroupDisplayName(rule, ruleGroup)
      if (existing) {
        existing.active_rule_count = (existing.active_rule_count ?? 0) + 1
        if (costRuleGroupNameScore(displayName) > costRuleGroupNameScore(existing.display_name)) {
          existing.display_name = displayName
        }
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

  async getCostRule(id: number | string, signal?: AbortSignal): Promise<CostRuleSummary> {
    const { data } = await http.get<{ data?: CostRuleSummary } | CostRuleSummary>(
      `/v1/cost-rules/${encodeURIComponent(String(id))}`,
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRuleSummary
  },

  async createCostRecalculationRun(
    payload: CreateCostRecalculationRunRequest,
    signal?: AbortSignal,
  ): Promise<CostRecalculationRun> {
    const { data } = await http.post<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      '/v1/cost-management/recalculation-runs',
      payload,
      { signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRecalculationRun
  },

  async listCostRecalculationRuns(
    params?: { page?: number; page_size?: number; status?: string },
    signal?: AbortSignal,
  ): Promise<CostRecalculationRunListResponse> {
    const { data } = await http.get<CostRecalculationRunListResponse>('/v1/cost-management/recalculation-runs', { params, signal })
    return data
  },

  async getCostRecalculationRun(
    id: number | string,
    params?: { page?: number; page_size?: number },
    signal?: AbortSignal,
  ): Promise<CostRecalculationRun> {
    const { data } = await http.get<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      `/v1/cost-management/recalculation-runs/${encodeURIComponent(String(id))}`,
      { params, signal },
    )
    return ('data' in data && data.data ? data.data : data) as CostRecalculationRun
  },

  async applyCostRecalculationRun(id: number | string, signal?: AbortSignal): Promise<ApplyCostRecalculationRunResponse> {
    const { data } = await http.post<ApplyCostRecalculationRunResponse>(
      `/v1/cost-management/recalculation-runs/${encodeURIComponent(String(id))}/apply`,
      {},
      { signal },
    )
    return data
  },

  async syncCostRecalculationRunERP(id: number | string, signal?: AbortSignal): Promise<SyncCostRecalculationRunERPResponse> {
    const { data } = await http.post<SyncCostRecalculationRunERPResponse>(
      `/v1/cost-management/recalculation-runs/${encodeURIComponent(String(id))}/sync-erp`,
      {},
      { signal },
    )
    return data
  },

  async cancelCostRecalculationRun(id: number | string, signal?: AbortSignal): Promise<CostRecalculationRun> {
    const { data } = await http.post<{ data?: CostRecalculationRun } | CostRecalculationRun>(
      `/v1/cost-management/recalculation-runs/${encodeURIComponent(String(id))}/cancel`,
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
