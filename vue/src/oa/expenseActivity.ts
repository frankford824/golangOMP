import { ref } from 'vue'
import { getToken } from '@/services/http'
import { api } from './api'

export const expenseActivityActions = ['finance.apply', 'finance.approve', 'finance.pay'] as const
export const expenseActivityURL = '/oa-api/finance/notifications'

export type ExpenseActivityItem = {
  id: number
  unread: boolean
  created_at: string
  payload: { document_id: string; state: string; title: string; kind: 'approve' | 'revise' | 'pay' | 'result' | 'cc' }
}

export type ExpenseActivityPage = {
  todos: ExpenseActivityItem[]
  items: ExpenseActivityItem[]
  approve_count?: number
  pay_count?: number
  todo_count: number
  result_count: number
  unread_result_count: number
}

const approveCount = ref<number | null>(null)
const payCount = ref<number | null>(null)
const todoCount = ref<number | null>(null)
const unreadResultCount = ref(0)
const revision = ref(0)
let token: string | null = null
let request = 0
let pending = false

function reset() {
  request++
  pending = false
  token = null
  approveCount.value = null
  payCount.value = null
  todoCount.value = null
  unreadResultCount.value = 0
}

async function refresh(force = false) {
  const requestedToken = getToken()
  if (requestedToken !== token) reset()
  token = requestedToken
  if (!token || (pending && !force)) return
  const current = ++request
  pending = true
  try {
    const result = await api<ExpenseActivityPage>(expenseActivityURL + '?summary=1')
    if (current === request && requestedToken === getToken()) {
      approveCount.value = result.approve_count ?? null
      payCount.value = result.pay_count ?? null
      todoCount.value = result.todo_count ?? null
      unreadResultCount.value = result.unread_result_count ?? 0
    }
  } catch {
    if (current === request && requestedToken === getToken()) {
      approveCount.value = null
      payCount.value = null
      todoCount.value = null
      unreadResultCount.value = 0
    }
  } finally {
    if (current === request) pending = false
  }
}

function invalidate() {
  revision.value++
  void refresh(true)
}

export function useExpenseActivity() {
  return {
    get approveCount() { return approveCount.value },
    get payCount() { return payCount.value },
    get todoCount() { return todoCount.value },
    get unreadResultCount() { return unreadResultCount.value },
    get revision() { return revision.value },
    refresh, invalidate, reset,
  }
}
