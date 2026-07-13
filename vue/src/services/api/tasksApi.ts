/**
 * 任务相关 API
 * 对应文档：v0.4 API 使用说明.md § 3. 任务接口
 *
 * 注意：此文件已替换原 mock 实现，所有操作直接调用后端真实接口。
 * 前端 Task 类型仍保留在 src/domain/types/task.ts，
 * 与后端响应的映射在 src/domain/mappers/task-mappers.ts 中进行。
 */

import http from '@/services/http'
import type {
  TaskListParams,
  AssignTaskPayload,
  BusinessInfoPatch,
  SubmitDesignPayload,
} from '@/services/apiTypes'
import type { TaskOperationalOverview } from '@/types/dashboard'

export interface TaskReferenceBatchDownloadItem {
  key: string
  filename: string
  file_size: number
  mime_type?: string
  download_url: string
  expires_at?: string | null
  source_kind: 'formalized_asset' | 'legacy_ref' | string
  asset_id?: number | null
  ref_id?: string | null
}

export interface TaskReferenceBatchDownloadFailure {
  key?: string
  source_kind?: 'formalized_asset' | 'legacy_ref' | string
  asset_id?: number | null
  ref_id?: string | null
  filename?: string
  reason: string
}

export interface TaskReferenceBatchDownloadManifest {
  items: TaskReferenceBatchDownloadItem[]
  failures?: TaskReferenceBatchDownloadFailure[]
  success_count: number
  failure_count: number
  total_size: number
  expires_at?: string | null
}

export interface TaskReferenceBatchDownloadResponse {
  data?: TaskReferenceBatchDownloadManifest
}

export interface AuditHandoverCandidateFilters {
  keyword?: string
  status?: 'PendingAuditA' | 'PendingAuditB' | ''
  owner_org_team?: string
  page?: number
  page_size?: number
}

export interface AuditHandoverCandidateItem {
  task_id: number
  task_no: string
  sku_code?: string
  primary_sku_code?: string
  product_name?: string
  task_status: 'PendingAuditA' | 'PendingAuditB' | string
  owner_org_team?: string
  current_handler_id?: number | null
  current_handler_name?: string
  updated_at: string
}

export interface AuditHandoverCandidateListData {
  items: AuditHandoverCandidateItem[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
  eligible_count: number
  selected_limit: number
}

export interface AuditHandoverCandidateListResponse {
  data?: AuditHandoverCandidateListData
}

export interface BatchAuditHandoverPayload {
  mode: 'explicit' | 'all_matching'
  task_ids?: number[]
  filters?: AuditHandoverCandidateFilters
  to_auditor_id: number
  reason: string
  current_judgement?: string
  risk_remark?: string
}

export interface BatchAuditHandoverResultItem {
  task_id: number
  task_no?: string
  status: 'success' | 'failed'
  message?: string
  handover_id?: number | null
}

export interface BatchAuditHandoverData {
  success_count: number
  failure_count: number
  results: BatchAuditHandoverResultItem[]
}

export interface BatchAuditHandoverResponse {
  data?: BatchAuditHandoverData
}

export interface TaskAiSummaryPerson {
  role: string
  name: string
  id?: string
  note?: string
}

export interface TaskAiSummaryBlocker {
  title: string
  owner?: string
  reason?: string
}

export interface TaskAiSummaryAction {
  role: string
  action: string
  timing?: string
}

export interface TaskAiSummaryTimelineItem {
  time?: string
  stage: string
  actor?: string
  summary: string
}

export interface TaskAiSummaryStuckPoint {
  level: 'high' | 'medium' | 'low' | string
  title: string
  reason: string
  owner?: string
  next_action?: string
}

export interface TaskAiSummarySkuAssetCost {
  sku: string
  asset_status?: string
  erp_status?: string
  cost_status?: string
  note?: string
}

export interface TaskAiSummaryResponse {
  decision?: string
  impact?: string
  primary_blocker?: TaskAiSummaryBlocker | null
  actions?: TaskAiSummaryAction[]
  evidence?: string[]
  headline?: string
  current_status?: string
  people?: TaskAiSummaryPerson[]
  timeline?: TaskAiSummaryTimelineItem[]
  stuck_points?: TaskAiSummaryStuckPoint[]
  sku_asset_erp_cost?: TaskAiSummarySkuAssetCost[]
  next_actions?: string[]
  confidence?: 'high' | 'medium' | 'low' | string
  raw_text?: string
  generated_at?: string
  model?: string
  provider?: string
}

export interface TaskFilterActorOption {
  id: number
  name: string
  username?: string
  display_name?: string
  department?: string
  team?: string
  task_count?: number
  last_used_at?: string | null
}

export interface TaskFilterOptionsResponse {
  creators?: TaskFilterActorOption[]
  designers?: TaskFilterActorOption[]
}

// ─── 任务列表 / 详情 ──────────────────────────────────────────────────────────

export const tasksApi = {
  /**
   * 获取任务列表
   * GET /v1/tasks
   * 权限：已登录用户
   */
  list: (params?: TaskListParams, signal?: AbortSignal) =>
    http.get('/v1/tasks', { params, signal }),

  /**
   * 获取任务中心筛选候选项
   * GET /v1/tasks/filter-options
   * 权限：已登录且可看任务中心的用户
   */
  filterOptions: (signal?: AbortSignal) =>
    http.get<{ data?: TaskFilterOptionsResponse }>('/v1/tasks/filter-options', { signal }),

  /**
   * 任务池列表（按模块领取）
   * GET /v1/tasks/pool
   */
  pool: (params?: TaskListParams, signal?: AbortSignal) =>
    http.get('/v1/tasks/pool', { params, signal }),

  /**
   * 获取任务看板摘要（今日待办等汇总）
   * GET /v1/task-board/summary
   * 权限：已登录用户
   */
  boardSummary: (signal?: AbortSignal) =>
    http.get('/v1/task-board/summary', { signal }),

  /**
   * 获取运营主页权威全量统计快照。
   * GET /v1/task-board/overview
   */
  operationalOverview: (signal?: AbortSignal) =>
    http.get<{ data: TaskOperationalOverview }>('/v1/task-board/overview', { signal }),

  /**
   * 获取任务看板各队列明细
   * GET /v1/task-board/queues
   * 权限：已登录用户
   */
  boardQueues: (signal?: AbortSignal) =>
    http.get('/v1/task-board/queues', { signal }),

  /**
   * 获取单个任务详情（主读模型）
   * GET /v1/tasks/{id}
   * 权限：已登录用户（可查看该任务）
   *
   * 契约摘要：
   * - 原品选品：读模型 `product_selection.erp_product`（sku_code、product_id、product_name/name 等）及
   *   平级 `product_selection.source_match_type`（列表项与详情 data 路径一致）。
   * - 参考图：任务顶层稳定返回 `reference_file_refs`（对象数组，可来自 canonical asset session 或 pre-task fallback；
   *   不再使用 `reference_images` 直传）。
   * - 设计图：`design_assets`（按资产根分组）与 `asset_versions`（扁平版本列表）均可能返回；
   *   upload-session complete 后即持久化可见，不依赖 submit-design 才出现在详情。
   * 详情展示以本接口为准。
   */
  getById: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}`, { signal }),

  /**
   * 获取任务完整详情（含关联信息）
   * GET /v1/tasks/{id}/detail
   * 权限：已登录用户（可查看该任务）
   *
   * 响应常为信封结构（`task` + `task_detail` + 模块等）；业务展示用读模型以
   * `tasksStore.loadTaskById` 合并后的 `Task` 为准，类型见 `TaskDetailResponse`。
   */
  getDetail: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/detail`, { signal }),

  generateAiSummary: (id: string, signal?: AbortSignal) =>
    http.post<{ data?: TaskAiSummaryResponse }>(
      `/v1/tasks/${encodeURIComponent(id)}/ai-summary`,
      {},
      { signal },
    ),

  batchDownloadTaskReferences: (id: string, signal?: AbortSignal) =>
    http.post<TaskReferenceBatchDownloadResponse>(
      `/v1/tasks/${encodeURIComponent(id)}/reference-assets/batch-download`,
      {},
      { signal },
    ),

  /**
   * 任务业务事件流（审核替换、指派等）
   * GET /v1/tasks/{id}/events
   */
  listTaskEvents: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${encodeURIComponent(id)}/events`, { signal }),

  // ─── 任务创建 ──────────────────────────────────────────────────────────────

  /**
   * 创建新任务
   * POST /v1/tasks
   * 权限：运营、管理员
   */
  create: (payload: Record<string, unknown>, signal?: AbortSignal, idempotencyKey?: string) =>
    http.post('/v1/tasks', payload, {
      signal,
      headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
    }),

  /**
   * 可选预展示：根据当前表单信息，向后端请求一组“将要使用”的 SKU 码。
   * 后端统一负责去重/并发分配；前端不再自行拼序号。
   */
  prepareProductCodes: (payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post('/v1/tasks/prepare-product-codes', payload, { signal }),

  // ─── 任务指派 ──────────────────────────────────────────────────────────────

  /**
   * 指派 / 更换设计师（首次指派与「重新指派」共用；重新指派仅在前端判定为「尚未产生设计版本」等条件时展示）
   * POST /v1/tasks/{id}/assign
   * 权限：由后端 RBAC 决定
   */
  assign: (id: string, payload: AssignTaskPayload, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/assign`, payload, { signal }),

  /**
   * 作废/关闭任务。force=false 为普通作废；force=true 为管理员关闭。
   * POST /v1/tasks/{id}/cancel
   */
  cancel: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${encodeURIComponent(id)}/cancel`, payload, { signal }),

  /**
   * v0.6 对齐：2026-03-18 批量提醒
   * POST /v1/tasks/batch/remind
   */
  /** task_ids 须为 JSON 数字数组，与后端 []int64 对齐 */
  batchRemind: (payload: { task_ids: number[] }, signal?: AbortSignal) =>
    http.post('/v1/tasks/batch/remind', payload, { signal }),

  /**
   * v0.6 对齐：2026-03-18 批量指派，命中批量 handler 返回 batch items
   * POST /v1/tasks/batch/assign
   */
  batchAssign: (
    payload: { task_ids: number[]; designer_id: number; designer_name?: string },
    signal?: AbortSignal,
  ) =>
    http.post('/v1/tasks/batch/assign', payload, { signal }),

  /**
   * 模块级接单（CAS 由后端判断）
   * POST /v1/tasks/{id}/modules/{module_key}/claim
   */
  claimModule: (
    id: string,
    moduleKey: string,
    payload: Record<string, unknown> = {},
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/modules/${encodeURIComponent(moduleKey)}/claim`,
      payload,
      { signal },
    ),

  triggerModuleAction: (
    id: string,
    moduleKey: string,
    action: string,
    payload: Record<string, unknown> = {},
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/modules/${encodeURIComponent(moduleKey)}/actions/${encodeURIComponent(action)}`,
      payload,
      { signal },
    ),

  /** P 图模块提交：POST /v1/tasks/{id}/modules/retouch/actions/submit */
  submitRetouchModule: (id: string, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${encodeURIComponent(id)}/modules/retouch/actions/submit`, {}, { signal }),

  reassignModule: (
    id: string,
    moduleKey: string,
    payload: Record<string, unknown>,
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/modules/${encodeURIComponent(moduleKey)}/reassign`,
      payload,
      { signal },
    ),

  poolReassignModule: (
    id: string,
    moduleKey: string,
    payload: Record<string, unknown>,
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/modules/${encodeURIComponent(moduleKey)}/pool-reassign`,
      payload,
      { signal },
    ),

  getCostOverrides: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${encodeURIComponent(id)}/cost-overrides`, { signal }),

  reviewCostOverride: (
    id: string,
    eventId: string,
    payload: Record<string, unknown>,
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/cost-overrides/${encodeURIComponent(eventId)}/review`,
      payload,
      { signal },
    ),

  financeMarkCostOverride: (
    id: string,
    eventId: string,
    payload: Record<string, unknown>,
    signal?: AbortSignal,
  ) =>
    http.post(
      `/v1/tasks/${encodeURIComponent(id)}/cost-overrides/${encodeURIComponent(eventId)}/finance-mark`,
      payload,
      { signal },
    ),

  // ─── 设计提交 ──────────────────────────────────────────────────────────────

  /**
   * 设计提交审核（业务流转动作）
   * POST /v1/tasks/{id}/submit-design
   * 权限：被指派的设计师
   *
   * 仅承担提交/审核状态机推进；不承担「在任务详情中 materialize 版本列表」的职责。
   * 设计资产与版本以 GET /v1/tasks/{id} 返回的 `design_assets` / `asset_versions`（及归一化结果）为准。
   */
  submitDesign: (id: string, payload: SubmitDesignPayload, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/submit-design`, payload, { signal }),

  /**
   * 定制审核入口
   * POST /v1/tasks/{id}/customization/review
   */
  submitCustomizationReview: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/customization/review`, payload, { signal }),

  // ─── 审核流程 ──────────────────────────────────────────────────────────────

  /**
   * 审核员领取审核任务（兼容入口；审核通过/驳回不再要求先领取）
   * POST /v1/tasks/{id}/audit/claim
   * 权限：审核员
   * 后端要求 stage 必填，与 openapi 一致：A=常规审核 / B=常规审核交接复核 / outsource_review=历史外协复核
   */
  auditClaim: (id: string, payload: { stage: string }, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/audit/claim`, payload, { signal }),

  /**
   * 审核通过
   * POST /v1/tasks/{id}/audit/approve
   * 权限：审核员（不要求先领取）
   * 后端要求 stage、next_status 必填（openapi required）
   */
  auditApprove: (
    id: string,
    payload: { stage: string; next_status: string; comment?: string },
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/${id}/audit/approve`, payload, { signal }),

  /**
   * 审核驳回
   * POST /v1/tasks/{id}/audit/reject
   * 权限：审核员（不要求先领取）
   * 后端要求 stage（问题分类）、comment（审核说明）必填
   */
  auditReject: (id: string, payload: { stage: string; comment: string }, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/audit/reject`, payload, { signal }),

  /**
   * 审核交班
   * POST /v1/tasks/{id}/audit/handover
   */
  auditHandover: (
    id: string,
    payload: {
      to_auditor_id: number
      reason: string
      current_judgement?: string
      risk_remark?: string
    },
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/${id}/audit/handover`, payload, { signal }),

  /**
   * 统一审核交班候选列表
   * GET /v1/tasks/audit/handover-candidates
   */
  listAuditHandoverCandidates: (params?: AuditHandoverCandidateFilters, signal?: AbortSignal) =>
    http.get<AuditHandoverCandidateListResponse>('/v1/tasks/audit/handover-candidates', {
      params,
      signal,
    }),

  /**
   * 统一审核交班批量提交
   * POST /v1/tasks/audit/handover-batch
   */
  batchAuditHandover: (payload: BatchAuditHandoverPayload, signal?: AbortSignal) =>
    http.post<BatchAuditHandoverResponse>('/v1/tasks/audit/handover-batch', payload, { signal }),

  /**
   * 任务交班记录列表
   * GET /v1/tasks/{id}/audit/handovers
   */
  listAuditHandovers: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/audit/handovers`, { signal }),

  /**
   * 接手交班
   * POST /v1/tasks/{id}/audit/takeover
   */
  auditTakeover: (id: string, payload: { handover_id: number }, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/audit/takeover`, payload, { signal }),

  /**
   * 审核转交
   * POST /v1/tasks/{id}/audit/transfer
   */
  auditTransfer: (
    id: string,
    payload: { to_auditor_id: number; stage: string; from_auditor_id?: number; comment?: string; reason?: string },
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/${id}/audit/transfer`, payload, { signal }),

  /**
   * @deprecated 新逻辑请改用 /v1/customization-jobs*。
   * 任务下创建定制单（任务需处于 PendingOutsource，API 路径仍为 outsource）
   * POST /v1/tasks/{id}/outsource
   */
  createTaskOutsource: (
    id: string,
    payload: {
      operator_id: number
      vendor_name: string
      outsource_type: string
      delivery_requirement?: string
      settlement_note?: string
    },
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/${id}/outsource`, payload, { signal }),

  // ─── 仓库流程 ──────────────────────────────────────────────────────────────

  /**
   * 仓库准备（将任务推进到待仓库接收队列）
   * POST /v1/tasks/{id}/warehouse/prepare
   */
  warehousePrepare: (id: string, payload: Record<string, unknown> = {}, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/warehouse/prepare`, payload, { signal }),

  /**
   * 仓库接收（标记已收到实物）
   * POST /v1/tasks/{id}/warehouse/receive
   * 权限：仓库员
   */
  warehouseReceive: (id: string, payload: Record<string, unknown> = {}, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/warehouse/receive`, payload, { signal }),

  /**
   * 仓库驳回
   * POST /v1/tasks/{id}/warehouse/reject
   */
  warehouseReject: (
    id: string,
    payload: {
      reject_reason?: string
      reject_category?: string
      remark?: string
      receiver_id?: number
    },
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/${id}/warehouse/reject`, payload, { signal }),

  /**
   * 仓库完成（确认入库完成）
   * POST /v1/tasks/{id}/warehouse/complete
   * 权限：仓库员
   */
  warehouseComplete: (id: string, payload: Record<string, unknown> = {}, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/warehouse/complete`, payload, { signal }),

  /**
   * 正式关单（仓库完成后的 PendingClose → Closed/Completed）
   * POST /v1/tasks/{id}/close
   * 要求任务处于 PendingClose；readiness 失败时 409 + cannot_close_reasons
   */
  closeTask: (id: string, payload: Record<string, unknown> = {}, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/close`, payload, { signal }),

  // ─── 业务信息 ──────────────────────────────────────────────────────────────

  /**
   * 获取任务业务信息（商品编码、成本等）
   * GET /v1/tasks/{id}/business-info
   * 权限：已登录用户（有相关权限）
   */
  getBusinessInfo: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/business-info`, { signal }),

  /**
   * 更新任务业务信息
   * PATCH /v1/tasks/{id}/business-info
   * 权限：运营、管理员
   */
  patchBusinessInfo: (id: string, patch: BusinessInfoPatch, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/business-info`, patch, { signal }),

  // ─── v0.6 对齐：FRONTEND_ALIGNMENT_v0.5(1).md K 节 per-task 商品/成本接口 ───

  /**
   * GET /v1/tasks/{id}/product-info
   * 权限：Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/Admin
   */
  getProductInfo: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/product-info`, { signal }),

  /**
   * PATCH /v1/tasks/{id}/product-info
   * 权限：Ops/Warehouse/Admin；设计/审核/外协仅可读
   * 局部编辑仅提交变更字段
   */
  patchProductInfo: (id: string, patch: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/product-info`, patch, { signal }),

  /**
   * GET /v1/tasks/{id}/cost-info
   * 权限：Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/Admin
   */
  getCostInfo: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/cost-info`, { signal }),

  /**
   * PATCH /v1/tasks/{id}/cost-info
   * 权限：Ops/Warehouse/Admin；设计/审核/外协仅可读
   * 局部编辑仅提交变更字段
   */
  patchCostInfo: (id: string, patch: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/cost-info`, patch, { signal }),

  /**
   * PATCH /v1/tasks/{id}/sku-items/{sku_item_id}/cost-info
   * 批量母任务子项成本维护；保存后后端按该子项成本重新请求 ERP 同步。
   */
  patchSkuItemCostInfo: (id: string, skuItemId: number | string, patch: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/sku-items/${skuItemId}/cost-info`, patch, { signal }),

  /**
   * POST /v1/tasks/{id}/cost-quote/preview
   * 权限：Ops/Warehouse/Admin；设计/审核/外协不可调用
   */
  costQuotePreview: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/cost-quote/preview`, payload, { signal }),

  // ─── 采购任务 procurement 记录（OpenAPI: PATCH + advance）──────────────────

  /**
   * 创建/更新采购任务 procurement 草稿
   * PATCH /v1/tasks/{id}/procurement
   */
  patchTaskProcurement: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/procurement`, payload, { signal }),

  /**
   * 采购生命周期推进：prepare | start | complete | reopen
   * POST /v1/tasks/{id}/procurement/advance
   */
  advanceTaskProcurement: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/procurement/advance`, payload, { signal }),

  /**
   * 批量 SKU 子项编辑（后端待补充正式契约）
   * PATCH /v1/tasks/{id}/sku-items
   */
  patchSkuItems: (
    id: string,
    payload: { items: Array<Record<string, unknown>>; trigger_filing?: boolean },
    signal?: AbortSignal,
  ) => http.patch(`/v1/tasks/${id}/sku-items`, payload, { signal }),

  /**
   * 单个 SKU 子项编辑（后端待补充正式契约）
   * PATCH /v1/tasks/{id}/sku-items/{sku_item_id}
   */
  patchSkuItem: (id: string, skuItemId: number, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/tasks/${id}/sku-items/${skuItemId}`, payload, { signal }),

  // ─── Step 87：建档 / ERP 同步 ───────────────────────────────────────────────

  /**
   * 获取任务建档状态
   * GET /v1/tasks/:id/filing-status
   */
  getFilingStatus: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/tasks/${id}/filing-status`, { signal }),

  /**
   * 重试建档同步（失败态时手动触发）
   * POST /v1/tasks/:id/filing/retry
   */
  retryFiling: (id: string, signal?: AbortSignal) =>
    http.post(`/v1/tasks/${id}/filing/retry`, {}, { signal }),
}

// ─── 向后兼容：保留旧的函数式导出（供 Pinia store 直接调用）──────────────────
// 这些函数在后续重构 store 时可逐步移除

/** @deprecated 请使用 tasksApi.list */
export async function fetchTaskList(params?: TaskListParams) {
  const res = await tasksApi.list(params)
  return res.data
}

/** @deprecated 请使用 tasksApi.getById */
export async function fetchTaskById(id: string) {
  const res = await tasksApi.getById(id)
  return res.data
}

/** @deprecated 请使用 tasksApi.create */
export async function createTask(payload: Record<string, unknown>, idempotencyKey?: string) {
  const res = await tasksApi.create(payload, undefined, idempotencyKey)
  return res.data
}

/** @deprecated 请使用 tasksApi.patchBusinessInfo */
export async function updateTask(id: string, patch: BusinessInfoPatch) {
  const res = await tasksApi.patchBusinessInfo(id, patch)
  return res.data
}
