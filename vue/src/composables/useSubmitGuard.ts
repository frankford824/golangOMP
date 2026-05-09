import { ref } from 'vue'

/**
 * 提交动作的重入防抖原语（best-effort，不是 token bucket）。
 *
 * 使用场景：任何用户触发的 POST/PATCH/DELETE 提交按钮。将处理函数包在
 * `guard(async () => { ... })` 里即可：
 *   1. 函数执行中时 `submitting.value === true`；
 *   2. 同一 guard 实例二次触发将被静默丢弃（返回 `undefined`）；
 *   3. 无论 fulfilled 或 rejected，`submitting.value` 都会在 finally 复位。
 *
 * 组件应将 `submitting` 绑到触发按钮的 `:disabled` 和 `:loading`，做到
 * 「UI 门禁 + 代码门禁」双保险；即使 UI 层被绕过（键盘 enter、上层 dialog
 * 再触发一次），`guard` 仍会挡掉重复请求。
 *
 * 与 `task-actions` 的 action_id（UUID v4）职责区分：
 *   - `action_id`：用于幂等重试，让后端识别同一次业务动作的多次请求；
 *   - `useSubmitGuard`：用于抑制重复点击，避免制造幂等场景。
 * 两者互补、不互斥。
 */
export function useSubmitGuard() {
  const submitting = ref(false)

  async function guard<T>(fn: () => Promise<T>): Promise<T | undefined> {
    if (submitting.value) return undefined
    submitting.value = true
    try {
      return await fn()
    } finally {
      submitting.value = false
    }
  }

  return { submitting, guard }
}

export type SubmitGuard = ReturnType<typeof useSubmitGuard>
