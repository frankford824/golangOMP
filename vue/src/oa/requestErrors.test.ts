// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { AxiosError, AxiosHeaders, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'
import RequestErrorNotice from './components/RequestErrorNotice.vue'
import { oaRequestError, oaResponseError, showOaRequestError } from './requestErrors'

const id = '3000a3f9ac8e426fbb507a025468cc84'

function requestError(status: number | undefined, data: unknown, url: string, headers: Record<string, string> = {}) {
  const config: InternalAxiosRequestConfig = { url, headers: new AxiosHeaders() }
  const response: AxiosResponse | undefined = status
    ? { status, data, headers: new AxiosHeaders(headers), statusText: '', config }
    : undefined
  return new AxiosError('', response ? undefined : 'ECONNABORTED', config, undefined, response)
}

afterEach(() => {
  oaRequestError.value = undefined
  vi.unstubAllGlobals()
})

describe('OA request errors', () => {
  it('preserves the legacy message and business code behavior while preferring the gateway trace', () => {
    const error = requestError(409, {
      code: 'INVOICE_ALREADY_CLAIMED',
      error: '发票号码已被其他单据占用，请核对明细',
      trace_id: 'faca6014-6cba-42f1-8b7f-5f7abbcbd150',
    }, '/oa-api/finance/documents', { 'x-trace-id': id })
    expect(oaResponseError(error)).toEqual({
      message: '发票号码已被其他单据占用，请核对明细', traceId: id,
    })
  })

  it('uses the standard OA business error and the core denial details', () => {
    const oa = requestError(403, {
      error: { code: 'FORBIDDEN', message: '当前账号无人员排行权限', trace_id: id },
    }, '/oa-api/orders/rankings')
    const core = requestError(403, {
      error: { code: 'PERMISSION_DENIED', message: 'forbidden', details: { deny_code: 'task_out_of_scope' }, trace_id: id },
    }, '/v1/tasks/1')
    expect(oaResponseError(oa)).toEqual({ message: '当前账号无人员排行权限', traceId: id })
    expect(oaResponseError(core)).toEqual({ message: '当前任务不在你的可见范围内', traceId: id })
  })

  it('shows Chinese for a gateway HTML response and skips trace on transport failures', () => {
    expect(oaResponseError(requestError(504, '<html>any content</html>', '/v1/tasks', { 'x-trace-id': id })))
      .toEqual({ message: '服务响应超时。请刷新页面核对操作结果后再试。', traceId: id })
    const transport = oaResponseError(requestError(undefined, undefined, '/v1/tasks'))
    expect(transport.message).toContain('暂时无法确认操作结果')
    showOaRequestError(transport)
    expect(oaRequestError.value).toBeUndefined()
  })

  it('copies the trace received from the API', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    showOaRequestError({ message: '服务暂时不可用', traceId: id })
    const notice = mount(RequestErrorNotice)
    expect(notice.get('input').element.value).toBe(id)
    await notice.get('button').trigger('click')
    expect(writeText).toHaveBeenCalledWith(id)
    expect(notice.text()).toContain('已复制追踪号')
    notice.unmount()
  })
})
