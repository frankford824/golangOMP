import { describe, expect, it } from 'vitest'

import { mapRawBackendMessageToZh, resolveApiUserMessage } from '@/utils/api-message-zh'

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

  it('maps network failures to anti-repeat action copy', () => {
    const message = resolveApiUserMessage(new Error('Network Error'))

    expect(message).toBe('连接中断，本次操作没有完成。请恢复网络后重试，不要重复点击提交。')
  })

  it('maps asset-workbench settlement and priced-work mutation messages', () => {
    expect(mapRawBackendMessageToZh('Submission item cannot be changed after settlement batch attachment.')).toBe(
      '当前作品已进入待确认的结算批次，暂时不能移动或删除。请先取消该批次后再操作。',
    )
    expect(mapRawBackendMessageToZh('All files in one priced work must be selected together.')).toBe(
      '该作品由文件夹内的多个文件组成，请勾选整个文件夹作品后再移动或删除。',
    )
    expect(mapRawBackendMessageToZh('Payee payout profile is incomplete; confirm is blocked.')).toBe(
      '无法确认批次：批次内有人员尚未补全姓名、身份证或支付宝信息。请先在人员资料中补齐后重试。',
    )
    expect(mapRawBackendMessageToZh('Supplement permission can only be opened for the current natural month.')).toBe(
      '补录权限只能开放到当前自然月，请读取当前月份后重试。',
    )
    expect(mapRawBackendMessageToZh('Settlement supplements must be recorded in the current natural month.')).toBe(
      '补录工资只能计入当前自然月，请刷新后重新提交。',
    )
  })
})
