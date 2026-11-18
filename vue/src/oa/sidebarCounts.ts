import { onMounted, onUnmounted, ref, watch, type ComputedRef } from 'vue'
import { tasksApi } from '@/services/api/tasksApi'
import { costMaintenanceApi } from '@/services/api/costMaintenanceApi'

export function useSidebarCounts(access: ComputedRef<{ account: number | string | undefined; tasks: boolean; costs: boolean }>) {
  const audits = ref(0)
  const conflicts = ref<number | string>(0)
  let abort: AbortController | undefined
  let lastRead = 0

  async function refresh() {
    const scope = access.value
    if (!scope.account || (!scope.tasks && !scope.costs) || document.hidden) return
    if (Date.now() - lastRead < 60_000) return
    lastRead = Date.now()
    const controller = new AbortController()
    abort?.abort()
    abort = controller
    await Promise.allSettled([
      scope.tasks && tasksApi.list({ operational_bucket: 'pending_audit', page: 1, page_size: 1 }, controller.signal).then(({ data }) => {
        const total = data?.pagination?.total
        if (!controller.signal.aborted && Number.isSafeInteger(total) && total >= 0) audits.value = total
      }),
      scope.costs && costMaintenanceApi.list({ status: 'conflict', review_state: 'pending', page_size: 10 }, controller.signal).then(result => {
        if (controller.signal.aborted || !Array.isArray(result.data)) return
        // This cursor endpoint returns no total. Never present the first page as a company-wide count.
        conflicts.value = result.next_before_id > 0 && result.data.length ? result.data.length + '+' : result.data.length
      }),
    ])
  }
  watch(access, () => {
    abort?.abort()
    audits.value = 0
    conflicts.value = 0
    lastRead = 0
    void refresh()
  }, { immediate: true })
  let timer: number | undefined
  onMounted(() => {
    window.addEventListener('focus', refresh)
    document.addEventListener('visibilitychange', refresh)
    timer = window.setInterval(refresh, 60_000)
  })
  onUnmounted(() => {
    abort?.abort()
    window.clearInterval(timer)
    window.removeEventListener('focus', refresh)
    document.removeEventListener('visibilitychange', refresh)
  })
  return { audits, conflicts }
}
