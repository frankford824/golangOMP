import { describe, expect, it } from 'vitest'
import { AxiosError } from 'axios'
import {
  parseApiErrorPayload,
  resolveApiUserMessage,
} from '@/utils/api-message-zh'
import { inferMockFamily } from '@/mocks'

function axiosError(status: number, data: unknown): AxiosError {
  return new AxiosError('failed', String(status), undefined, undefined, {
    status,
    statusText: 'ERROR',
    headers: {},
    config: {} as never,
    data,
  })
}

describe('P0 API error contract', () => {
  it('extracts code, deny_code and trace id from V1 envelopes', () => {
    const err = axiosError(409, {
      error: {
        code: 'CONFLICT',
        deny_code: 'task_already_claimed',
        message: 'state changed',
        trace_id: 'trace-1',
      },
    })

    expect(parseApiErrorPayload(err)).toMatchObject({
      status: 409,
      code: 'CONFLICT',
      denyCode: 'task_already_claimed',
      traceId: 'trace-1',
    })
    expect(resolveApiUserMessage(err)).toBe('任务已被接单，无法作废')
  })

  it('keeps validation messages user-facing for 422 responses', () => {
    const err = axiosError(422, {
      error: {
        code: 'VALIDATION_ERROR',
        message: 'password must include letters and numbers',
      },
    })

    expect(resolveApiUserMessage(err)).toBe('密码必须包含字母和数字')
  })
})

describe('P0 mock family routing', () => {
  it('maps V1 canonical paths to mock families', () => {
    expect(inferMockFamily('/v1/auth/login')).toBe('auth')
    expect(inferMockFamily('/v1/me/org')).toBe('me')
    expect(inferMockFamily('/v1/me/notifications')).toBe('notifications')
    expect(inferMockFamily('/v1/tasks/123/asset-center/upload-sessions')).toBe('task-assets')
    expect(inferMockFamily('/v1/tasks/batch-create/parse-excel')).toBe('batch')
    expect(inferMockFamily('/v1/reports/l1/cards')).toBe('reports')
    expect(inferMockFamily('/v1/search')).toBeUndefined()
  })
})
