import { ref } from 'vue'
import { getToken } from '@/services/http'
import { api } from './api'

export const expenseActivityActions = ['finance.apply', 'finance.approve', 'finance.pay'] as const
export const expenseActivityURL = '/oa-api/finance/notifications'

export type ExpenseActivityItem = {
  id: number
  created_at: string
  payload: { document_id: string; state: string; title: string; kind: 'approve' | 'revise' | 'pay' | 'result' }
}

export type ExpenseActivityPage = {
  todos: ExpenseActivityItem[]
  items: ExpenseActivityItem[]
  todo_count: number
  result_count: number
}

const todoCount = ref<number | null>(null)
const revision = ref(0)
let token: string | null = null
let request = 0
let pending = false

function reset() {
  request++
  pending = false
  token = null
  todoCount.value = null
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
    if (current === request && requestedToken === getToken()) todoCount.value = result.todo_count ?? null
  } catch {
    if (current === request && requestedToken === getToken()) todoCount.value = null
  } finally {
    if (current === request) pending = false
  }
}

function invalidate() {
  revision.value++
  void refresh(true)
}

export function useExpenseActivity() {
  return { get todoCount() { return todoCount.value }, get revision() { return revision.value }, refresh, invalidate, reset }
}
