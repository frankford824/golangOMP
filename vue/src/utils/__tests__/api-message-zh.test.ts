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

  it('maps asset-workbench price overlap conflicts to business copy', () => {
    const message = resolveApiUserMessage({
      status: 409,
      responseData: {
        error: {
          code: 'CONFLICT',
          message: 'Price effective range overlaps an existing rule.',
        },
      },
    })

    expect(message).toBe('这条单价的生效时间与已有单价重叠，请调整生效日期或使用「替代」发布新版本')
  })

  it('does not leak unmapped English backend messages to users', () => {
    const message = resolveApiUserMessage({
      status: 409,
      responseData: {
        error: {
          code: 'CONFLICT',
          message: 'Unknown internal conflict from service.',
        },
      },
    })

    expect(message).toBe('与已有数据冲突，请更换后重试')
  })

  it('maps deny_code from error.details before generic permission copy', () => {
    const message = resolveApiUserMessage({
      status: 403,
      responseData: {
        error: {
          code: 'PERMISSION_DENIED',
          message: 'role is not assignable by current actor',
          details: {
            deny_code: 'role_not_assignable',
          },
        },
      },
    })

    expect(message).toBe('当前账号不能分配所选角色，请调整后重试')
  })

  it('maps customization upload deny codes to business copy', () => {
    const message = resolveApiUserMessage({
      status: 400,
      responseData: {
        error: {
          code: 'INVALID_REQUEST',
          message: 'customization review uploads only support source assets',
          details: {
            deny_code: 'customization_review_asset_type_not_allowed',
          },
        },
      },
    })

    expect(message).toBe('定制审核阶段只能上传修改后的源文件')
  })
})
