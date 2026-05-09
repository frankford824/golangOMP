type ErrorLike = {
  response?: {
    data?: {
      error?: {
        code?: string
        message?: string
        details?: Record<string, unknown>
      }
    }
  }
  message?: string
}

export function extractTaskActionDenyCode(error: unknown): string | undefined {
  const e = error as ErrorLike
  const details = e?.response?.data?.error?.details as Record<string, unknown> | undefined
  const denyCode = details?.deny_code
  if (typeof denyCode === 'string' && denyCode.trim() !== '') return denyCode.trim()
  const denyReason = details?.deny_reason
  if (typeof denyReason === 'string' && denyReason.trim() !== '') return denyReason.trim()
  return undefined
}

const DENY_TEXT: Record<string, string> = {
  invalid_designer_id: '指派人不合法',
  reassign_target_out_of_managed_department: '只能改派给您管辖部门内的成员',
  task_reassign_requires_manager_scope: '仅管理员可改派',
  INVALID_STATE_TRANSITION: '当前任务状态不允许该操作',
  task_out_of_team_scope: '你无权操作其他团队任务',
  task_out_of_department_scope: '你无权操作其他部门任务',
  task_not_reassignable: '当前状态不允许重新分配',
  task_status_not_actionable: '当前状态不允许该操作',
  warehouse_stage_mismatch: '当前任务不在可执行仓库动作的阶段',
  task_not_closable: '当前任务尚不满足结单条件',
  missing_required_role: '你缺少执行该操作所需角色',
  audit_stage_mismatch: '当前任务不在对应审核阶段',
  customization_stage_mismatch: '该任务已被审核处理过，正在刷新列表…',
}

/**
 * 判断本次 403/409 是否为「定制审核阶段已漂移」——即该任务已被另一位审核员
 * 或并发请求推进，当前快照已失效。调用方应走「静默成功 + 刷新列表」分支，
 * 避免给用户抛出原始网络错误。
 */
export function isCustomizationStageMismatch(error: unknown): boolean {
  return extractTaskActionDenyCode(error) === 'customization_stage_mismatch'
}

/** 查表获取 deny_code 对应的中文提示（用于静默成功路径回填 banner 文案）。 */
export function denyCodeText(code: string): string | undefined {
  return DENY_TEXT[code]
}

export function formatTaskActionDenyMessage(
  error: unknown,
  fallback = '当前无权限执行该操作',
): string {
  const code = extractTaskActionDenyCode(error)
  if (code && DENY_TEXT[code]) return DENY_TEXT[code]
  const msg = (error as { message?: unknown })?.message
  if (typeof msg === 'string' && msg.trim() !== '') return msg
  return fallback
}

