import { describe, expect, it } from 'vitest'

import { resolveApiUserMessage } from '@/utils/api-message-zh'

describe('resolveApiUserMessage', () => {
  it('prefers backend conflict message over generic CONFLICT copy', () => {
    const message = resolveApiUserMessage({
      status: 409,
      responseData: {
        error: {
          code: 'CONFLICT',
          message: '检测到同一次提交编号已用于另一份任务内容，请刷新页面后重新提交。',
        },
      },
    })

    expect(message).toBe('检测到同一次提交编号已用于另一份任务内容，请刷新页面后重新提交。')
  })

  it('keeps semantic auth code copy ahead of backend noise', () => {
    const message = resolveApiUserMessage({
      status: 401,
      responseData: {
        error: {
          code: 'UNAUTHORIZED',
          message: 'invalid credentials',
        },
      },
    })

    expect(message).toBe('账号或密码不正确，请检查后重试')
  })
})
