/**
 * 统一将接口错误体中的 message / code / detail 转为面向用户的中文说明。
 * 优先使用后端返回字段，再通过映射表将常见英文文案译为中文；已是中文则原样展示。
 */

import axios from 'axios'

/** 后端 error.code（大写）→ 中文 */
export const API_ERROR_CODE_ZH: Record<string, string> = {
  UNAUTHORIZED: '账号或密码不正确，请检查后重试',
  UNAUTHENTICATED: '登录已过期，请重新登录',
  FORBIDDEN: '暂无权限执行该操作',
  PERMISSION_DENIED: '暂无权限执行该操作',
  NOT_FOUND: '请求的资源不存在',
  BAD_REQUEST: '请求参数有误，请检查填写内容',
  INVALID_ARGUMENT: '提交的信息不符合要求',
  VALIDATION_ERROR: '提交的信息不符合要求',
  CONFLICT: '与已有数据冲突，请更换后重试',
  ALREADY_EXISTS: '该记录已存在',
  INTERNAL_ERROR: '服务暂时不可用，请稍后重试',
  INTERNAL_SERVER_ERROR: '服务暂时不可用，请稍后重试',
  UNAVAILABLE: '服务暂时不可用，请稍后重试',
  DEADLINE_EXCEEDED: '请求超时，请稍后重试',
  RESOURCE_EXHAUSTED: '系统繁忙，请稍后再试',
  ABORTED: '操作被中断，请重试',
  INVALID_REQUEST: '请求参数有误，请检查填写内容',
}

/** 后端 deny_code → 前端降级/提示文案。 */
export const API_DENY_CODE_ZH: Record<string, string> = {
  task_create_field_denied_by_scope: '当前组织范围无权填写该字段',
  task_out_of_scope: '当前任务不在你的可见范围内',
  task_out_of_stage_scope: '当前流程阶段不在你的可操作范围内',
  task_not_assigned_to_actor: '该任务未分配给你处理',
  task_status_not_actionable: '当前任务状态不允许执行该操作',
  task_not_reassignable: '当前任务不可改派',
  module_action_role_denied: '当前角色无权执行该模块操作',
  department_scope_only: '仅部门范围内可操作',
  team_scope_only: '仅组内范围可操作',
  org_admin_scope_only: '仅组织管理员可操作',
  user_update_field_denied_by_scope: '当前组织范围无权修改该用户字段',
  role_assignment_denied_by_scope: '当前组织范围无权调整该角色',
  management_access_required: '需要管理权限',
  reports_super_admin_only: '报表仅超级管理员可见',
  asset_version_race_retry: '资产版本发生并发更新，请刷新后重试',
  audit_log_access_denied: '无权查看该审计日志',
  workflow_lane_unsupported: '当前工作流通道不支持该操作',
  old_password_mismatch: '旧密码不正确',
  password_confirmation_required: '请确认新密码',
  password_confirmation_mismatch: '两次输入的新密码不一致',
  module_not_instantiated: '该模块尚未实例化',
  module_out_of_scope: '该模块不在你的可操作范围内',
  module_state_mismatch: '模块状态已变化，请刷新后重试',
  module_claim_conflict: '该任务已被他人领取',
  module_blueprint_missing_team: '池组配置缺失，请联系管理员',
  task_already_claimed: '任务已被接单，无法作废',
}

/**
 * 后端 error.message 常见英文（小写键）→ 中文。
 * 与登录页、http 400 拦截器共用，避免多处漂移。
 */
export const API_ERROR_MESSAGE_ZH: Record<string, string> = {
  'invalid account or password': '账号或密码不正确，请检查后重试',
  'invalid credentials': '账号或密码不正确，请检查后重试',
  unauthorized: '未通过身份验证，请重新登录',
  forbidden: '暂无权限执行该操作',
  'username already exists': '用户名已被使用',
  'user already exists': '用户已存在',
  'password must be at least 8 characters': '密码至少 8 个字符',
  'password must include letters and numbers': '密码必须包含字母和数字',
  'mobile format is invalid': '手机号格式不正确',
  'invalid mobile': '手机号格式不正确',
  'invalid email': '邮箱格式不正确',
  'invalid department': '部门无效，请重新选择',
  'invalid team': '组无效，请重新选择',
  'admin key is invalid': '管理员密钥不正确',
  'invalid admin key': '管理员密钥不正确',
  'registration is disabled': '当前已关闭自助注册',
  'too many requests': '请求过于频繁，请稍后再试',
  'rate limit exceeded': '请求过于频繁，请稍后再试',
  'request failed with status code 400': '请求参数有误',
  'request failed with status code 403': '暂无权限执行该操作',
  'request failed with status code 404': '请求的资源不存在',
  'request failed with status code 409': '与已有数据冲突，请更换后重试',
  'request failed with status code 500': '服务暂时不可用，请稍后重试',
  'erp product name length validation failed': '产品名称将同步为 ERP 简称，最多可填写 40 个字，请精简后再提交',
  'erp product name exceeds length limit': '产品名称将同步为 ERP 简称，最多可填写 40 个字，请精简后再提交',
  /** GET /v1/assets/{id}/preview 等：409 + INVALID_STATE_TRANSITION 常见英文文案 */
  'asset preview is not available': '资产预览不可用',
  'asset preview is not available.': '资产预览不可用',
  'price effective range overlaps an existing rule.': '该价目规则的生效时间与已有规则重叠，请调整生效日期或使用「替代」发布新版本',
  'deduction effective range overlaps an existing rule.': '该扣减规则的生效时间与已有规则重叠，请调整生效日期或使用「替代」发布新版本',
  'welfare effective range overlaps an existing rule.': '该福利规则的生效时间与已有规则重叠，请调整生效日期或使用「替代」发布新版本',
  'promo effective range overlaps an existing rule.': '该大促规则的生效时间与已有规则重叠，请调整生效日期或使用「替代」发布新版本',
  'effective_from is required.': '请选择生效开始日期',
  'effective_to must be after effective_from.': '生效结束日期必须晚于生效开始日期',
  'worker_type, job_grade and difficulty_class are required.': '请选择用工类型、等级和计件分类',
  'unit_price must be non-negative.': '单价不能小于 0',
  'deduction_amount must be non-negative.': '扣减金额不能小于 0',
  'job_grade is not valid for worker_type.': '当前等级不适用于所选用工类型',
  'network error': '网络异常，请检查连接后重试',
  timeout: '请求超时，请稍后重试',
  'network timeout': '请求超时，请稍后重试',
}

type NestedApiError = {
  code?: string
  deny_code?: string
  message?: string
  trace_id?: string
  detail?: string | Record<string, unknown>
  details?: unknown
}

type UnwrappedBody = {
  error?: NestedApiError
  code?: string
  deny_code?: string
  message?: string
  detail?: string
}

function unwrapResponseBody(data: unknown): UnwrappedBody | undefined {
  if (data == null) return undefined
  if (Array.isArray(data)) return data.length ? unwrapResponseBody(data[0]) : undefined
  if (typeof data !== 'object') return undefined
  const o = data as Record<string, unknown>
  if ('data' in o && o.data != null && typeof o.data === 'object') {
    return unwrapResponseBody(o.data)
  }
  return o as UnwrappedBody
}

export interface ParsedApiError {
  status?: number
  code?: string
  denyCode?: string
  message?: string
  detail?: string
  traceId?: string
}

function pickBodyAndStatus(err: unknown): { status?: number; body?: UnwrappedBody } {
  if (axios.isAxiosError(err)) {
    return {
      status: err.response?.status,
      body: unwrapResponseBody(err.response?.data),
    }
  }
  const he = err as { responseData?: unknown; status?: number }
  if (he.responseData !== undefined) {
    return { status: he.status, body: unwrapResponseBody(he.responseData) }
  }
  return {}
}

/** 从任意错误对象解析后端信封（兼容 HttpAppError、AxiosError） */
export function parseApiErrorPayload(err: unknown): ParsedApiError {
  const { status, body } = pickBodyAndStatus(err)
  const root = body as Record<string, unknown> | undefined
  const apiErr = root?.error as NestedApiError | undefined

  const rootCode = typeof root?.code === 'string' ? root.code : ''
  const code = (typeof apiErr?.code === 'string' ? apiErr.code : rootCode).trim().toUpperCase()
  const denyCode = (
    typeof apiErr?.deny_code === 'string'
      ? apiErr.deny_code
      : typeof root?.deny_code === 'string'
        ? root.deny_code
        : ''
  ).trim()

  let message = (typeof apiErr?.message === 'string' ? apiErr.message : '').trim()
  if (!message && root && typeof root.message === 'string') {
    message = root.message.trim()
  }

  let detail = ''
  if (typeof apiErr?.detail === 'string') detail = apiErr.detail.trim()
  else if (root && typeof root.detail === 'string') detail = root.detail.trim()

  const traceId = (typeof apiErr?.trace_id === 'string' ? apiErr.trace_id : '').trim()

  return { status, code, denyCode, message, detail, traceId }
}

export function mapDenyCodeToZh(denyCode: string | undefined): string {
  if (!denyCode) return ''
  return API_DENY_CODE_ZH[denyCode] ?? ''
}

function statusFallbackZh(status: number | undefined): string {
  if (status === 401) return '账号或密码不正确，请检查后重试'
  if (status === 403) return '暂无权限，如需开通请联系管理员'
  if (status === 404) return '请求的服务不存在或已变更'
  if (status === 408) return '请求超时，请稍后重试'
  if (status === 409) return '与已有数据冲突，请更换后重试'
  if (status === 429) return '请求过于频繁，请稍后再试'
  if (status != null && status >= 500) return '服务暂时不可用，请稍后重试'
  return ''
}

/**
 * 将后端返回的单条 message/detail 文案转为中文（已是中文则不变）。
 * 供 http 400 拦截器等仅需处理「原始字符串」的场景。
 */
export function mapRawBackendMessageToZh(raw: string): string {
  const t = String(raw ?? '').trim()
  if (!t) return '请求参数有误'
  if (/[\u4e00-\u9fff]/.test(t)) return t
  const lower = t.toLowerCase()
  return API_ERROR_MESSAGE_ZH[lower] ?? API_ERROR_MESSAGE_ZH[t] ?? t
}

function hasChineseText(raw: string): boolean {
  return /[\u4e00-\u9fff]/.test(raw)
}

function mapBackendMessageForUser(raw: string): string {
  const t = String(raw ?? '').trim()
  if (!t) return ''
  const mapped = mapRawBackendMessageToZh(t)
  if (!mapped) return ''
  if (mapped === t && !hasChineseText(t)) return ''
  return mapped
}

function mapDetailToZh(detail: string): string {
  const t = detail.trim()
  if (!t) return ''
  if (hasChineseText(t)) return t
  const lower = t.toLowerCase()
  return API_ERROR_MESSAGE_ZH[lower] ?? API_ERROR_MESSAGE_ZH[t] ?? ''
}

export interface ResolveApiUserMessageOptions {
  /** 无法解析时的兜底文案 */
  fallback?: string
  /** 是否在文案末尾附加「追踪号：xxx」（默认 false） */
  includeTrace?: boolean
}

/**
 * 最泛化的校验类 code 集合：这类 code 只代表「请求参数有误」这一宽泛语义，
 * 真正的原因（如「用户已存在」「密码不合规」）藏在后端 message 里。
 * 命中这些 code 时，若后端同时下发了具体 message，必须让 message 抢位，
 * 否则用户只会看到兜底文案（见后端交接文档 · 问题 1）。
 */
const GENERIC_VALIDATION_CODES = new Set<string>([
  'INVALID_REQUEST',
  'BAD_REQUEST',
  'INVALID_ARGUMENT',
  'VALIDATION_ERROR',
])

const MESSAGE_FIRST_CODES = new Set<string>([
  ...GENERIC_VALIDATION_CODES,
  'CONFLICT',
])

/** 明显是无信息量的 message 噪声，遇到就跳过，回落到 code 映射 */
function isNoiseMessage(raw: string): boolean {
  const t = raw.trim().toLowerCase()
  if (!t) return true
  if (t.startsWith('request failed with status code')) return true
  if (t === 'bad request' || t === 'invalid request') return true
  return false
}

/**
 * 统一错误展示文案：
 * 1. 明确 deny_code 优先，避免业务态错误退化成英文技术口径；
 * 2. 具体 message（非噪声）优先；
 * 3. 泛化 code（INVALID_REQUEST 等）命中后才在缺失 message 时作兜底；
 * 3. 语义清晰的 code（UNAUTHORIZED / FORBIDDEN / NOT_FOUND 等）仍优先于英文原文 message，
 *    避免出现「invalid credentials」这类漏网；
 * 4. 最后按 detail → HTTP status → Error.message 逐级兜底。
 */
export function resolveApiUserMessage(
  err: unknown,
  options?: ResolveApiUserMessageOptions,
): string {
  const parsed = parseApiErrorPayload(err)
  const fallback = options?.fallback ?? '操作失败，请稍后重试'

  const codeZh = parsed.code ? API_ERROR_CODE_ZH[parsed.code] : undefined
  const denyZh = mapDenyCodeToZh(parsed.denyCode)
  const shouldPreferBackendMessage = !!parsed.code && MESSAGE_FIRST_CODES.has(parsed.code)
  const hasRealMessage = !!parsed.message && !isNoiseMessage(parsed.message)

  let main = ''

  if (denyZh) {
    main = denyZh
  }

  if (!main && shouldPreferBackendMessage && hasRealMessage) {
    main = mapBackendMessageForUser(parsed.message!)
  }

  if (!main && codeZh) {
    main = codeZh
  }

  if (!main && hasRealMessage) {
    main = mapBackendMessageForUser(parsed.message!)
  }

  if (!main && parsed.detail) {
    const d = mapDetailToZh(parsed.detail)
    if (d) main = d
  }

  if (!main) {
    main = statusFallbackZh(parsed.status)
  }

  if (!main && err instanceof Error) {
    const m = err.message?.trim() ?? ''
    if (m && !m.startsWith('Request failed with status code')) {
      main = mapBackendMessageForUser(m)
    }
  }

  if (!main) {
    main = fallback
  }

  if (options?.includeTrace && parsed.traceId) {
    return `${main}（追踪号：${parsed.traceId}）`
  }
  return main
}
