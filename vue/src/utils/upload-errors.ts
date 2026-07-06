import axios, { type AxiosError } from 'axios'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

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
  // 若已包含相同问题编号（极少场景：错误经多次拼接），避免重复追加。
  if (base.includes(traceId)) return base
  return `${base}（问题编号：${traceId}）`
}

function uploadPhaseLabel(phase: UploadFailurePhase): string {
  if (phase === 'create_session') return '创建上传入口失败'
  if (phase === 'part_upload') return '文件上传失败'
  if (phase === 'main_complete') return '确认上传结果失败'
  if (phase === 'submit_design') return '提交审核失败'
  if (phase === 'reference_upload') return '参考图上传失败'
  if (phase === 'abort') return '取消上传失败'
  return '上传失败'
}

export function formatSubmitDesignFailureMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const status = err.response?.status
    const data = err.response?.data
    const messages = collectBackendMessages(data)
    const joined = messages.join(' | ').toLowerCase()
    const hit = SUBMIT_DESIGN_ERROR_HINTS.find((item) => joined.includes(item.needle))
    if (hit) return hit.message
    const userMessage = resolveApiUserMessage(err, { fallback: '' })
    if (userMessage) return `提交审核失败：${userMessage}`
    if (messages.length > 0) {
      return '提交审核失败，请检查填写内容后重试'
    }
    if (status != null) return '提交审核失败，请稍后重试'
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
  const transportHint = options?.transportLabel?.trim() ? '，请重新上传' : ''
  const phaseLabel = uploadPhaseLabel(phase)

  if (axios.isAxiosError(err)) {
    const ax = err as AxiosError
    const status = ax.response?.status
    const data = ax.response?.data
    const traceId = pickBackendTraceId(data)
    const byCode = messageForUploadErrorCode(pickBackendErrorCode(data))
    if (byCode) return appendTraceId(byCode, traceId)

    const backendCode = pickBackendErrorCode(data)
    const denyCode = pickBackendDetailString(data, 'deny_code')
    const action = pickBackendDetailString(data, 'action')
    if (
      backendCode === 'PERMISSION_DENIED' &&
      denyCode === 'task_status_not_actionable' &&
      (action === 'asset_upload_session_complete' || action === 'asset_upload_session_cancel')
    ) {
      return appendTraceId(
        '任务状态已更新，当前不能继续上传，请刷新任务后重试',
        traceId,
      )
    }

    const userMessage = resolveApiUserMessage(err, { fallback: '' })

    if (status != null) {
      if (userMessage) {
        return appendTraceId(`${phaseLabel}${partHint}${transportHint}：${userMessage}`, traceId)
      }
      return appendTraceId(
        `${phaseLabel}${partHint}，请稍后重试`,
        traceId,
      )
    }

    if (ax.code === 'ECONNABORTED' || ax.message?.toLowerCase().includes('timeout')) {
      return appendTraceId(
        `上传超时${partHint}，请检查网络后重试`,
        traceId,
      )
    }

    if (
      ax.code === 'ERR_NETWORK' ||
      ax.message === 'Network Error' ||
      (typeof ax.message === 'string' && ax.message.toLowerCase().includes('network'))
    ) {
      return appendTraceId(
        `上传服务暂时无法连接${partHint}，请检查网络或稍后重试`,
        traceId,
      )
    }
  }

  if (err instanceof Error && err.message) {
    const message = err.message.trim()
    if (message) return `${phaseLabel}${partHint}：${message}`
  }

  return `${phaseLabel}${partHint}，请稍后重试`
}
