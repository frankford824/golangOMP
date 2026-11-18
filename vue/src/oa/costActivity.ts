import { ref } from 'vue'
import { getToken } from '@/services/http'
import { api } from './api'

const todoCount = ref(0)
let generation = 0
let pending: Promise<void> | undefined
let session: string | null = null

function reset() {
  generation++
  pending = undefined
  session = null
  todoCount.value = 0
}

function refresh(): Promise<void> {
  const token = getToken()
  if (token !== session) { reset(); session = token }
  if (!token) return Promise.resolve()
  if (pending) return pending
  const current = generation
  pending = api<{ pending_count: number; reply_count: number }>('/oa-api/orders/cost-requests?summary=1')
    .then(result => {
      if (generation === current && getToken() === token) todoCount.value = (result.pending_count ?? 0) + (result.reply_count ?? 0)
    }).catch(() => {
      // The inbox offers retry. Keep the last count for this session during a transient outage.
    }).finally(() => {
      if (generation === current) pending = undefined
    })
  return pending
}
export function invalidateCostActivity() { reset(); void refresh() }
export function useCostActivity() { return { get todoCount() { return todoCount.value }, reset, refresh, invalidate: invalidateCostActivity } }
