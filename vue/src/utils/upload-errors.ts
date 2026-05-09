import axios, { type AxiosError } from 'axios'

export type UploadFailurePhase =
  | 'create_session'
  | 'part_upload'
  | 'main_complete'
  | 'submit_design'
  | 'reference_upload'
  | 'abort'

export interface UploadFailureFormatOptions {
  transportLabel?: string
}

function pickBackendMessage(data: unknown): string | undefined {
  if (data == null || typeof data !== 'object') return undefined
  const o = data as Record<string, unknown>
  const nested = o.error
  if (nested && typeof nested === 'object') {
    const m = (nested as { message?: string }).message
    if (typeof m === 'string' && m.trim()) return m
  }
  for (const k of ['detail', 'message', 'msg'] as const) {
    const v = o[k]
    if (typeof v === 'string' && v.trim()) return v
  }
  return undefined
}

/** 解析后端 `error.code`（axios 响应体根或 `error` 嵌套） */
export function pickBackendErrorCode(data: unknown): string | undefined {
  if (data == null || typeof data !== 'object') return undefined
  const o = data as Record<string, unknown>
  const tryCode = (x: unknown): string | undefined => {
    if (x == null || typeof x !== 'object') return undefined
    const c = (x as { code?: unknown }).code
    return typeof c === 'string' && c.trim() ? c.trim().toUpperCase() : undefined
  }
  return tryCode(o.error) ?? tryCode(o)
}

const MSG_UPLOAD_ENV_NOT_ALLOWED =
  '当前网络环境不支持大文件上传，请连接公司内网或 VPN 后重试。参考图等小文件上传仍可使用。'

const MSG_UPLOAD_ENDPOINT_DEPRECATED =
  '设计稿上传入口已变更，请刷新页面后重试。若问题仍在，请联系管理员。'

const SUBMIT_DESIGN_ERROR_HINTS: Array<{ needle: string; message: string }> = [
  {
    needle: 'upload_session_id does not belong to task',
    message: '提交失败：存在 upload_session_id 不属于当前任务，请刷新页面后重试。',
  },
  {
    needle: 'assets[].target_sku_code is required for batch delivery submissions',
    message: '提交失败：批量任务中的 delivery 文件必须携带 target_sku_code。',
  },
  {
    needle: 'assets[].target_sku_code must belong to the task',
    message: '提交失败：target_sku_code 不属于当前任务，请检查商品归属。',
  },
  {
    needle: 'assets[].target_sku_code does not match upload session target_sku_code',
    message: '提交失败：target_sku_code 与上传会话不一致，请重新上传对应商品文件。',
  },
]

export function messageForUploadErrorCode(code: string | undefined): string | undefined {
  if (!code) return undefined
  const c = code.toUpperCase()
  if (c === 'UPLOAD_ENV_NOT_ALLOWED') return MSG_UPLOAD_ENV_NOT_ALLOWED
  if (c === 'UPLOAD_ENDPOINT_DEPRECATED') return MSG_UPLOAD_ENDPOINT_DEPRECATED
  return undefined
}

function collectBackendMessages(data: unknown): string[] {
  if (data == null || typeof data !== 'object') return []
  const out: string[] = []
  const root = data as Record<string, unknown>
  const nested = root.error
  if (nested && typeof nested === 'object') {
    const e = nested as Record<string, unknown>
    for (const k of ['detail', 'message', 'msg'] as const) {
      const v = e[k]
      if (typeof v === 'string' && v.trim()) out.push(v.trim())
    }
  }
  for (const k of ['detail', 'message', 'msg'] as const) {
    const v = root[k]
    if (typeof v === 'string' && v.trim()) out.push(v.trim())
  }
  return out
}

function pickBackendDetailString(
  data: unknown,
  key: string,
): string | undefined {
  if (data == null || typeof data !== 'object') return undefined
  const root = data as Record<string, unknown>
  const nested = root.error
  if (!nested || typeof nested !== 'object') return undefined
  const details = (nested as Record<string, unknown>).details
  if (!details || typeof details !== 'object') return undefined
  const v = (details as Record<string, unknown>)[key]
  if (typeof v === 'string' && v.trim()) return v.trim()
  return undefined
}

/**
 * 从标准错误包络中读 `error.trace_id`（后端每次 5xx/4xx 都写入 response envelope）。
 * 仅在拿到非空字符串时返回，用于在上传失败横幅上附带便于排障的 trace id。
 */
function pickBackendTraceId(data: unknown): string | undefined {
  if (data == null || typeof data !== 'object') return undefined
  const root = data as Record<string, unknown>
  const nested = root.error
  if (nested && typeof nested === 'object') {
    const v = (nested as Record<string, unknown>).trace_id
    if (typeof v === 'string' && v.trim()) return v.trim()
  }
  const topLevel = (root as Record<string, unknown>).trace_id
  if (typeof topLevel === 'string' && topLevel.trim()) return topLevel.trim()
  return undefined
}

function appendTraceId(base: string, traceId: string | undefined): string {
  if (!traceId) return base
  // 若已包含相同 trace_id（极少场景：错误经多次拼接），避免重复追加。
  if (base.includes(traceId)) return base
  return `${base} (trace_id: ${traceId})`
}

export function formatSubmitDesignFailureMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const status = err.response?.status
    const data = err.response?.data
    const messages = collectBackendMessages(data)
    const joined = messages.join(' | ').toLowerCase()
    const hit = SUBMIT_DESIGN_ERROR_HINTS.find((item) => joined.includes(item.needle))
    if (hit) return hit.message
    if (messages.length > 0) {
      return `提交审核失败${status ? `（HTTP ${status}）` : ''}：${messages[0]}`
    }
    if (status != null) return `提交审核失败（HTTP ${status}），请稍后重试`
  }
  if (err instanceof Error && err.message) return err.message
  return '提交审核失败，请稍后重试'
}

/**
 * 将上传链路错误拆成可展示文案（避免一律 “Network error”）
 */
export function formatUploadFailureMessage(
  phase: UploadFailurePhase,
  err: unknown,
  partNo?: number,
  options?: UploadFailureFormatOptions,
): string {
  const partHint = partNo != null ? `（分片 ${partNo}）` : ''
  const transportHint = options?.transportLabel?.trim()
    ? `，通道：${options.transportLabel.trim()}`
    : ''

  if (axios.isAxiosError(err)) {
    const ax = err as AxiosError
    const status = ax.response?.status
    const data = ax.response?.data
    const traceId = pickBackendTraceId(data)
    const byCode = messageForUploadErrorCode(pickBackendErrorCode(data))
    if (byCode) return appendTraceId(`${byCode}（阶段：${phase}${transportHint}）`, traceId)

    const backendCode = pickBackendErrorCode(data)
    const denyCode = pickBackendDetailString(data, 'deny_code')
    const action = pickBackendDetailString(data, 'action')
    const taskStatus = pickBackendDetailString(data, 'task_status')
    if (
      backendCode === 'PERMISSION_DENIED' &&
      denyCode === 'task_status_not_actionable' &&
      (action === 'asset_upload_session_complete' || action === 'asset_upload_session_cancel')
    ) {
      return appendTraceId(
        `上传会话处理失败：任务状态已切换为 ${taskStatus || '不可上传'}，当前阶段不再允许继续 complete/cancel（并非文件大小问题）`,
        traceId,
      )
    }

    const backend = pickBackendMessage(data)

    if (status != null) {
      if (backend) {
        const phaseLabel =
          phase === 'create_session'
            ? '创建上传会话'
            : phase === 'part_upload'
              ? '分片上传'
              : phase === 'main_complete'
                  ? '服务端确认完成'
                  : phase === 'submit_design'
                    ? '提交审核'
                  : phase === 'abort'
                    ? '取消上传会话'
                    : phase === 'reference_upload'
                      ? '参考图上传'
                      : '上传'
        return appendTraceId(
          `${phaseLabel}失败（HTTP ${status}）${partHint}${transportHint}：${backend}`,
          traceId,
        )
      }
      return appendTraceId(
        `上传失败（HTTP ${status}）${partHint}，阶段：${phase}${transportHint}`,
        traceId,
      )
    }

    if (ax.code === 'ECONNABORTED' || ax.message?.toLowerCase().includes('timeout')) {
      return appendTraceId(
        `上传超时${partHint}，阶段：${phase}${transportHint}，请检查网络后重试`,
        traceId,
      )
    }

    if (
      ax.code === 'ERR_NETWORK' ||
      ax.message === 'Network Error' ||
      (typeof ax.message === 'string' && ax.message.toLowerCase().includes('network'))
    ) {
      const origin = typeof window !== 'undefined' ? window.location.origin : ''
      return appendTraceId(
        `无法连接上传服务（网络/CORS/内网不可达）${partHint}，阶段：${phase}${transportHint}` +
          (origin ? `。当前页面 Origin：${origin}` : ''),
        traceId,
      )
    }
  }

  if (err instanceof Error && err.message) {
    return `${err.message}${partHint}（阶段：${phase}${transportHint}）`
  }

  return `上传失败${partHint}，阶段：${phase}${transportHint}`
}
