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
})
