import { shallowRef } from 'vue'

export class OASessionRecoveryError extends Error {
  readonly traceId: string | undefined
  readonly blocked: boolean

  constructor(readonly cause: unknown) {
    const blocked = (cause as { status?: number } | null)?.status === 403
    super(blocked ? '当前账号无法访问 OA。' : '暂时无法确认登录状态，请重试。')
    this.blocked = blocked
    const trace = (cause as { traceId?: unknown } | null)?.traceId
    this.traceId = typeof trace === 'string' ? trace : undefined
  }
}

export const oaSessionRecovery = shallowRef<{ target: string; error: OASessionRecoveryError }>()

export function showOASessionRecovery(target: string, error: OASessionRecoveryError): void {
  oaSessionRecovery.value = { target, error }
}
