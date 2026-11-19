import { onBeforeUnmount, shallowRef } from 'vue'

export type FeeConfirmation = {
  title: string; message?: string; facts?: [string, string][]; confirmText: string
  danger?: boolean; reasonLabel?: string
}
export type ConfirmationResult = { reason: string } | null

export function useFeeConfirm() {
  const state = shallowRef<FeeConfirmation>()
  let resolve: ((result: ConfirmationResult) => void) | undefined
  function finish(result: ConfirmationResult) {
    const pending = resolve
    resolve = undefined
    state.value = undefined
    pending?.(result)
  }
  function ask(options: FeeConfirmation): Promise<ConfirmationResult> {
    if (resolve) return Promise.resolve(null)
    state.value = options
    return new Promise<ConfirmationResult>((done) => { resolve = done })
  }
  onBeforeUnmount(() => finish(null))
  return { state, ask, finish }
}
