import { shallowRef } from 'vue'
import { getOASessionId } from '@/services/oaSession'
import type { AxiosError, AxiosResponse } from 'axios'
import { parseApiErrorPayload, resolveApiUserMessage } from '@/utils/api-message-zh'

type RequestError = { message: string; traceId?: string; oaSessionId?: string | null }

export const oaRequestError = shallowRef<RequestError>()

export function showOaRequestError(error: RequestError) {
  if (!error.traceId || (error.oaSessionId !== undefined && error.oaSessionId !== getOASessionId())) return
  const notice = { message: error.message, traceId: error.traceId }
  oaRequestError.value = notice
  return notice
}

export function isOaApiUrl(url?: string): boolean {
  return url?.startsWith('/v1/') === true || url?.startsWith('/oa-api/') === true
}

export function oaResponseTraceId(response?: AxiosResponse<unknown>) {
  const body = response?.data as { trace_id?: unknown; error?: { trace_id?: unknown } } | undefined
  const candidates = [response?.headers?.['x-trace-id'], body?.error?.trace_id, body?.trace_id]
  return candidates.find((id): id is string => typeof id === 'string' &&
    /^(?:[0-9a-f]{32}|[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12})$/i.test(id))
}

export function oaResponseError(error: AxiosError) {
  const parsed = parseApiErrorPayload(error)
  const body = error.response?.data as { error?: unknown } | undefined
  const traceId = oaResponseTraceId(error.response)
  const status = error.response?.status
  const extension = error.config?.url?.startsWith('/oa-api/')
  const message = extension && typeof body?.error === 'string' ? body.error : parsed.message
  const localized = message && /[\u4e00-\u9fff]/.test(message) && message.length <= 300

  if (!status) {
    return {
      traceId,
      message: error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT'
        ? '请求超时，暂时无法确认操作结果。请刷新页面核对后再试。'
        : '无法连接服务。请检查网络，恢复后刷新页面核对操作结果。',
    }
  }
  if (status === 413) return { traceId, message: '上传内容超过大小限制，请缩小文件后重试。' }
  if (status === 504) return { traceId, message: '服务响应超时。请刷新页面核对操作结果后再试。' }
  // OA extension messages are authored for operators, including its legacy error string.
  if (extension && localized) return { traceId, message }
  if (status >= 500) return { traceId, message: '服务暂时不可用，请稍后重试。' }
  if (parsed.code === 'INVOICE_ALREADY_CLAIMED') {
    return { traceId, message: '发票号码已被其他单据占用，请核对明细。' }
  }
  return { traceId, message: resolveApiUserMessage(error) }
}
