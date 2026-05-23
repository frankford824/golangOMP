/**
 * V1 后端 API 数据结构类型定义
 * 对应文档：frontend/V1_API_*.md 与 docs/api/openapi.yaml。
 *
 * 此文件仅定义与后端交互的原始 DTO 类型，不包含前端业务逻辑类型。
 * 前端业务类型（Task、PermissionUser 等）保留在 src/domain/types/ 和 src/types/。
 */

// ─── 认证与用户 ───────────────────────────────────────────────────────────────

/** 后端 frontend_access 结构，控制前端菜单/页面/操作的显隐 */
export interface FrontendAccess {
  /** 侧边栏/顶部菜单项 key 列表 */
  menus?: string[]
  /** 可访问的路由/页面 key 列表 */
  pages?: string[]
  /** 可执行的操作/按钮 key 列表 */
  actions?: string[]
  /** 数据范围等组织归属信息由后端 scope 侧负责，非菜单显隐来源 */
  scopes?: string[]
  /** 当前主数据/业务模块 key */
  modules?: string[]
  /** 兼容别名：页面 keys */
  page_keys?: string[]
  /** 兼容别名：菜单 keys */
  menu_keys?: string[]
  /** 兼容别名：模块 keys */
  module_keys?: string[]
  /** 兼容别名：动作 keys */
  permission_flags?: string[]
  /** 兼容别名：scope keys */
  access_scopes?: string[]
  /** 运行时数据范围标识 */
  view_all?: boolean
  /** 运行时可管理部门（兼容不同后端命名） */
  managed_departments?: string[]
  /** 运行时可管理团队（兼容不同后端命名） */
  managed_teams?: string[]
  /** 运行时部门编码列表 */
  department_codes?: string[]
  /** 运行时团队编码列表 */
  team_codes?: string[]
  /** 运行时角色列表（可用于审计展示） */
  roles?: string[]
  /** 运行时当前部门 */
  department?: string
  /** 运行时当前团队 */
  team?: string
  /** 是否超级管理员 */
  is_super_admin: boolean
  /** 是否部门管理员 */
  is_department_admin: boolean
  /**
   * 可选：小组长（设计组长调度等）。未返回时前端无法区分组长与普通成员，仅靠 `design:work` 不够。
   */
  is_group_leader?: boolean
}

/** 后端用户对象 */
export interface BackendUser {
  id: string
  username: string
  display_name: string
  department: string
  team: string
  roles: string[]
  frontend_access: FrontendAccess
  mobile?: string
  email?: string
}

/** POST /v1/auth/login 响应 */
export interface LoginResponse {
  data?: {
    user?: BackendUser
    session?: {
      session_id?: string
      token?: string
      token_type?: string
      expires_at?: string
    }
  }
  user?: BackendUser
  token?: string
  session?: {
    session_id?: string
    token?: string
    token_type?: string
    expires_at?: string
  }
  frontend_access?: FrontendAccess
}

/** POST /v1/auth/register 请求体 */
export interface RegisterPayload {
  username?: string
  account?: string
  password: string
  display_name?: string
  name?: string
  department: string
  team?: string
  group?: string
  mobile: string
  phone?: string
  email?: string
  /** 管理员密钥，可选；若当前接口支持则传此字段注册为部门管理员 */
  admin_key?: string
  secret_key?: string
}

/** PUT /v1/auth/password 请求体 */
export interface ChangePasswordPayload {
  old_password: string
  new_password: string
}

// ─── 任务 ─────────────────────────────────────────────────────────────────────

/**
 * 后端任务摘要（列表场景 GET /v1/tasks 的 item）
 * 选品：item.product_selection.erp_product（product_id、sku_id、sku_code、product_name、name 等）
 * 与 product_selection.source_match_type（与 erp_product 平级，不在 erp_product 内）。
 */
export interface BackendTaskSummary {
  id: string
  task_no: string
  workflow: Record<string, unknown>
  product_selection?: Record<string, unknown>
  procurement_summary?: Record<string, unknown>
  [key: string]: unknown
}

export type TaskListItem = BackendTaskSummary

/** Step 87：建档状态接口响应 GET /v1/tasks/:id/filing-status */
export interface FilingStatusResponse {
  filing_status?: string
  filing_error_message?: string
  missing_fields?: string[]
  missing_fields_summary_cn?: string
  last_filed_at?: string | null
  erp_sync_required?: boolean
  filing_trigger_source?: string
  last_filing_attempt_at?: string | null
  erp_sync_version?: number
}

/** GET /v1/tasks 请求参数 */
export interface TaskListParams {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
  task_type?: string
  workflow_lane?: 'normal' | 'customization' | string
  assignee_id?: string
  /** v0.9：按设计师筛选列表（与 designer_* 读模型一致） */
  designer_id?: string
  /** 定制泳道「未指派美工」：仅 designer_id 为空（勿与 status=PendingAssign 混用） */
  designer_empty?: boolean
  group_id?: string
  department?: string
  /** 规范归属：部门筛选 */
  owner_department?: string
  /** 规范归属：组织树团队筛选 */
  owner_org_team?: string
  /** 按任务创建人用户 id 筛选（OpenAPI：creator_id） */
  creator_id?: string | number
  [key: string]: unknown
}

/** 用户任务草稿 */
export interface TaskDraft {
  id: string
  task_type: string
  payload: Record<string, unknown>
  created_at?: string
  updated_at?: string
  expires_at?: string
  created_by?: string
}

/** V1 模块读模型摘要（用于按钮级 allowed_actions 渲染） */
export interface ModuleSummary {
  module_key: string
  state?: string
  scope?: {
    in_scope?: boolean
    deny_code?: string
  }
  /** OpenAPI returns a flat string array; older mocks used `{ actions: [...] }`. */
  allowed_actions?: string[] | { actions?: string[] } | null
  [key: string]: unknown
}

/**
 * GET /v1/tasks/{id}/detail 信封：`data` 内除 `task` 外常含子表与模块信息。
 * 前端业务读模型以 `tasksStore.loadTaskById` 合并 `task` + `task_detail` 后的 `Task` 为准；
 * 本类型描述**原始 HTTP 体**形态，供类型提示与排查；细字段以后端契约为准。
 */
export interface TaskDetailResponse {
  task?: BackendTaskSummary & Record<string, unknown>
  /** 任务扩展子表（需求、分类、remark 等；`reference_file_refs_json` 仅 legacy 回退） */
  task_detail?: Record<string, unknown>
  modules?: ModuleSummary[]
  events?: unknown[]
  timeline?: unknown[]
  comments?: unknown[]
  /** 详情顶层参考图对象数组（canonical）；`task_detail.reference_file_refs_json` 仅 legacy fallback */
  reference_file_refs?: unknown[] | null
  [key: string]: unknown
}

export interface CancelRequest {
  reason: string
  force?: boolean
}

/** POST /v1/tasks/{id}/assign 请求体（后端 designer_id 为 int64） */
export interface AssignTaskPayload {
  designer_id: number | null
  designer_name?: string
  remark?: string
}

/** POST /v1/tasks/{id}/audit/approve 请求体 */
export interface AuditApprovePayload {
  comment?: string
}

/** POST /v1/tasks/{id}/audit/reject 请求体（后端要求 Stage、Comment 必填） */
export interface AuditRejectPayload {
  stage: string
  comment: string
}

/**
 * PATCH /v1/tasks/{id}/business-info 请求体（字段按业务语义分流）：
 * - 中文/展示值/i_id：使用 `category`
 * - 后端内部类目码（前缀 KT_ 或 OUT_）：使用 `category_code`
 * - 建议二者互斥提交
 */
export interface BusinessInfoPatch {
  [key: string]: unknown
}

/** POST /v1/tasks/{id}/submit-design 单项资产提交事实（基于已 complete 的 upload session） */
export interface SubmitDesignAssetItem {
  upload_session_id: string
  /** 可选：便于后端日志/校验分支识别 */
  asset_kind?: 'source' | 'delivery'
  /** batch delivery 必填，且需与 upload-session 创建时 target_sku_code 一致 */
  target_sku_code?: string
}

/** POST /v1/tasks/{id}/submit-design 请求体 */
export interface SubmitDesignPayload {
  assets: SubmitDesignAssetItem[]
  remark?: string
}

// ─── ERP ──────────────────────────────────────────────────────────────────────

/** GET /v1/erp/products 请求参数 */
export interface ErpProductsParams {
  keyword?: string
  sku_code?: string
  category?: string
  page?: number
  page_size?: number
}

// ─── 日志 ─────────────────────────────────────────────────────────────────────

/** GET /v1/operation-logs 单条记录（聚合：任务事件 / 导出事件 / 集成调用） */
export type OperationLogSource = 'task_event' | 'export_event' | 'integration_call'

export interface OperationLogEntry {
  source: OperationLogSource
  log_id: string
  reference_type: string
  reference_id: string
  event_type: string
  summary: string
  actor_id: number | null
  actor_username?: string
  actor_type: string
  status?: string
  payload?: Record<string, unknown> | unknown
  created_at: string
}

/** GET /v1/permission-logs 单条（与 docs/openapi.yaml PermissionLog 对齐） */
export interface PermissionLog {
  id: number | string
  actor_id?: number | null
  actor_username?: string
  actor_source?: string
  action_type?: string
  target_user_id?: number | null
  target_username?: string
  method?: string
  route_path?: string
  granted?: boolean
  reason?: string
  created_at: string
}

/** 服务器日志（v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节） */
export interface ServerLog {
  id: number
  level: string
  msg: string
  /** 后端可能返回对象或可 JSON 解析的字符串 */
  details?: Record<string, unknown> | string
  created_at: string
}

// ─── 资产与访问策略（live v0.4）──────────────────────────────────────────────────

/** 资产访问策略（后端返回，前端按此展示下载方式） */
export interface AssetAccessPolicy {
  lan_url?: string
  tailscale_url?: string
  public_url?: string
  access_hint?: string
  source_file_requires_private_network?: boolean
}

/** canonical 下载模式：当前主链优先 `direct`，`proxy` 仅兼容 fallback。 */
export type AssetDownloadMode = 'direct' | 'proxy' | 'public' | 'private_network'

/** 资产版本（含访问策略）
 * v0.6 对齐：必须仅根据 preview_available 与 download_mode 决策 UI */
export interface BackendAssetVersion {
  id: string
  file_role: string
  version?: number
  file_name?: string
  created_at?: string
  /** 规范业务文件访问入口（列表嵌套版本时由后端下发） */
  download_url?: string
  lan_url?: string
  tailscale_url?: string
  public_url?: string
  access_hint?: string
  source_file_requires_private_network?: boolean
  /** v0.6 对齐：K 节 Source/PSD 为 false，reference/delivery 为 true */
  preview_available?: boolean
  /** canonical 下载方式；旧 `public/private_network` 仅作兼容读取 */
  download_mode?: AssetDownloadMode
  [key: string]: unknown
}

/** 后端资产项 */
export interface BackendAsset {
  id: string
  task_id?: string
  file_role: string
  previous_asset_id?: string | number | null
  current_asset_id?: string | number | null
  replacement_actor_id?: string | number | null
  workflow_lane?: 'normal' | 'customization' | string
  source_department?: string | null
  versions?: BackendAssetVersion[]
  approved_version?: number
  warehouse_ready_version?: number
  [key: string]: unknown
}

// ─── 定制单（OpenAPI OutsourceOrder，GET /v1/outsource-orders，字段名兼容 outsource_*）────────

export type BackendOutsourceOrderStatus =
  | 'created'
  | 'packaged'
  | 'sent'
  | 'in_production'
  | 'returned'
  | 'reviewing'
  | 'approved'
  | 'rejected'
  | 'closed'

/** 与后端 JSON 对齐；部分字段可能由后端扩展（任务号、SKU 等） */
export interface BackendOutsourceOrderRaw {
  id: number
  outsource_no: string
  task_id: number
  vendor_name: string
  outsource_type: string
  delivery_requirement?: string
  settlement_note?: string
  status: BackendOutsourceOrderStatus | string
  returned_at?: string | null
  created_at: string
  updated_at?: string
  /** 若后端在列表项中嵌套任务摘要 */
  task_no?: string
  sku?: string
  product_name?: string
  [key: string]: unknown
}

export interface OutsourceOrderListQuery {
  task_id?: number
  status?: string
  vendor?: string
  page?: number
  page_size?: number
}

export interface CustomizationJobRaw {
  id: number | string
  task_id?: number | string
  source_asset_id?: number | string | null
  previous_asset_id?: number | string | null
  current_asset_id?: number | string | null
  customization_level_code?: string
  customization_level_name?: string
  review_reference_unit_price?: number | null
  review_reference_weight_factor?: number | null
  unit_price?: number | null
  weight_factor?: number | null
  note?: string
  customization_review_decision?: 'approved' | 'return_to_designer' | 'reviewer_fixed' | string
  decision_type?: 'final' | 'effect_preview' | string
  assigned_operator_id?: number | string | null
  last_operator_id?: number | string | null
  replacement_actor_id?: number | string | null
  replacement_actor_name?: string | null
  replacement_actor_username?: string | null
  /**
   * pricing identity（定价身份），不是权限角色。
   * 后端可能返回 `employment_type`，也可能返回历史字段 `pricing_worker_type`。
   */
  employment_type?: 'full_time' | 'part_time' | string | null
  pricing_worker_type?: 'full_time' | 'part_time' | string | null
  workflow_lane?: 'normal' | 'customization' | string
  source_department?: string | null
  status?: string
  warehouse_reject_reason?: string | null
  warehouse_reject_category?: string | null
  created_at?: string
  updated_at?: string
  [key: string]: unknown
}

// ─── 通用分页响应 ─────────────────────────────────────────────────────────────

export interface PagedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

// ─── 通用错误响应 ─────────────────────────────────────────────────────────────

export interface ApiError {
  code?: string
  message?: string
  detail?: string | Record<string, string[]>
  details?: ActionErrorDetails | Record<string, unknown>
  trace_id?: string
}

/** task action 403 等场景细粒度拒绝信息 */
export interface ActionErrorDetails {
  deny_code?: string
  deny_reason?: string
  matched_rule?: string
  scope_source?: string
  action?: string
  actor_id?: number
  actor_roles?: string[]
  task_id?: number
  owner_department?: string
  owner_org_team?: string
  [key: string]: unknown
}
