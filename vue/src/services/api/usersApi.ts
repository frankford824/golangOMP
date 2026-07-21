import http from '@/services/http'

export type DesignersLane = 'normal' | 'customization' | 'audit' | 'all'

export interface GetDesignersOptions {
  /**
   * 工作流泳道。
   * - undefined / 'normal'：不拼查询串，保留与迭代前一致的 URL、缓存键与遥测数据。
   * - 'customization' / 'audit' / 'all'：追加 `?workflow_lane=<value>`，由后端返回对应泳道用户。
   */
  workflowLane?: DesignersLane
}

export interface UsersQuery {
  page?: number
  page_size?: number
  keyword?: string
  status?: 'active' | 'disabled'
  role?: string
  department?: string
  team?: string
}

function isAbortSignal(value: unknown): value is AbortSignal {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as AbortSignal).aborted === 'boolean' &&
    typeof (value as AbortSignal).addEventListener === 'function'
  )
}

export const usersApi = {
  /**
   * 获取设计师列表
   * GET /v1/users/designers
   * 返回显式权限模型筛选后的 id、username、display_name，用于创建、指派与审核交班。
   * 调用形态：
   *   - usersApi.getDesigners()
   *   - usersApi.getDesigners(signal)
   *   - usersApi.getDesigners({ workflowLane: 'customization' })
   *   - usersApi.getDesigners({ workflowLane }, signal)
   */
  getDesigners: (
    optsOrSignal?: GetDesignersOptions | AbortSignal,
    maybeSignal?: AbortSignal,
  ) => {
    const opts: GetDesignersOptions =
      optsOrSignal && !isAbortSignal(optsOrSignal)
        ? (optsOrSignal as GetDesignersOptions)
        : {}
    const signal: AbortSignal | undefined = isAbortSignal(optsOrSignal)
      ? (optsOrSignal as AbortSignal)
      : maybeSignal
    const lane = opts.workflowLane
    const url =
      lane && lane !== 'normal'
        ? `/v1/users/designers?workflow_lane=${encodeURIComponent(lane)}`
        : '/v1/users/designers'
    return http.get(url, { signal })
  },

  /**
   * 用户管理目录（分页列表）
   * GET /v1/users
   * 权限：管理与组织视图专用；请勿在任务中心等页面对普通 Designer 无门控调用。
   * 指派/设计师下拉请用 {@link usersApi.getDesigners}；展示任务的创建人等多用任务快照字段而非本接口。
   */
  list: (
    params: UsersQuery = {},
    signal?: AbortSignal,
  ) =>
    http.get('/v1/users', { params, signal }),

  /**
   * 获取单个用户详情
   * GET /v1/users/{id}
   * 权限：管理员，或用户本人
   */
  getById: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/users/${id}`, { signal }),

  /**
   * 更新用户（含组织归属：department / team）
   * PATCH /v1/users/{id}
   * 部分环境 OpenAPI 仅列出 display_name/status；联调时按后端实际支持字段传参。
   */
  patch: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/users/${id}`, payload, { signal }),

  update: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/users/${id}`, payload, { signal }),

  /**
   * 创建用户
   * POST /v1/users
   */
  create: (payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post('/v1/users', payload, { signal }),

  /**
   * 管理员重置用户密码
   * PUT /v1/users/{id}/password
   */
  resetPassword: (id: string, payload: { password: string }, signal?: AbortSignal) =>
    http.put(`/v1/users/${id}/password`, payload, { signal }),

  /** POST /v1/users/{id}/activate */
  activate: (id: string, signal?: AbortSignal) =>
    http.post(`/v1/users/${encodeURIComponent(id)}/activate`, {}, { signal }),

  /** POST /v1/users/{id}/deactivate */
  deactivate: (id: string, signal?: AbortSignal) =>
    http.post(`/v1/users/${encodeURIComponent(id)}/deactivate`, {}, { signal }),
}
