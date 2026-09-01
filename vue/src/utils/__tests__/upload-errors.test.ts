import { describe, expect, it } from 'vitest'

import { formatUploadFailureMessage } from '@/utils/upload-errors'

function axiosLikeError(status: number, data: unknown): unknown {
  return {
    isAxiosError: true,
    response: {
      status,
      data,
    },
    message: `Request failed with status code ${status}`,
  }
}

describe('formatUploadFailureMessage', () => {
  it('formats backend upload denial as business copy without HTTP or phase details', () => {
    const message = formatUploadFailureMessage(
      'create_session',
      axiosLikeError(400, {
        error: {
          code: 'INVALID_REQUEST',
          message: 'customization review uploads only support source assets',
          details: {
            deny_code: 'customization_review_asset_type_not_allowed',
          },
        },
      }),
    )

    expect(message).toContain('创建上传入口失败')
    expect(message).toContain('定制审核阶段只能上传修改后的源文件')
    expect(message).not.toContain('HTTP')
    expect(message).not.toContain('阶段：')
    expect(message).not.toContain('customization review uploads')
  })

  it('formats network upload failure without CORS or origin jargon', () => {
    const message = formatUploadFailureMessage('part_upload', {
      isAxiosError: true,
      code: 'ERR_NETWORK',
      message: 'Network Error',
    })

    expect(message).toContain('上传服务暂时无法连接')
    expect(message).not.toContain('CORS')
    expect(message).not.toContain('Origin')
    expect(message).not.toContain('阶段：')
  })

  it('explains an OSS 403 as an expired or rejected upload credential', () => {
    const message = formatUploadFailureMessage(
      'part_upload',
      axiosLikeError(403, {
        Code: 'AccessDenied',
        Message: 'Request has expired',
      }),
    )

    expect(message).toContain('上传凭证已过期或被存储服务拒绝')
    expect(message).toContain('减少同时上传数量')
    expect(message).not.toContain('暂无权限')
  })

  it('tells the user to refresh before retrying a failed completion', () => {
    const message = formatUploadFailureMessage(
      'main_complete',
      axiosLikeError(500, {
        error: {
          code: 'INTERNAL_ERROR',
          message: '服务暂时不可用，请稍后重试',
          trace_id: 'trace-upload-complete',
        },
      }),
    )

    expect(message).toContain('系统未能登记本次文件')
    expect(message).toContain('请先刷新任务确认现有文件')
    expect(message).not.toContain('已自动清理')
    expect(message).toContain('trace-upload-complete')
    expect(message).not.toContain('服务暂时不可用')
  })
})
