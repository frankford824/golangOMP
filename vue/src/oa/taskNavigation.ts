import type { RouteLocationRaw } from 'vue-router'
import { safeReturnPath } from './returnNavigation'

const taskEntryPaths = ['/', '/tasks', '/design-dashboard', '/tasks/create', '/me/task-drafts', '/me/notifications', '/users']
const composeEntryPaths = ['/tasks', '/me/task-drafts']
const operationsEntry = /^\/operations\/(?:orders\/detail|products(?:\/[^/]+)?)$/
const assetDetailEntry = /^\/asset-center\/(?:\d+|ext-[^/]+)$/

export function taskReturnSource(value: unknown): string | undefined {
  return safeReturnPath(value, (path) => taskEntryPaths.includes(path) || operationsEntry.test(path))
}

export function composeReturnSource(value: unknown): string | undefined {
  return safeReturnPath(value, (path) => composeEntryPaths.includes(path) || operationsEntry.test(path))
}

export function assetTaskReturnSource(value: unknown): string | undefined {
  return safeReturnPath(value, (path) => assetDetailEntry.test(path))
}

export function taskDetailLocation(id: string | number, source: string): RouteLocationRaw {
  return { name: 'TaskDetail', params: { id }, query: { taskFrom: taskReturnSource(source) } }
}

export function taskReturnLabel(source: string): string {
  const path = source.split(/[?#]/, 1)[0]
  if (path === '/design-dashboard') return '返回设计部门看板'
  if (path === '/tasks/create') return '返回创建任务'
  if (path === '/me/task-drafts') return '返回任务草稿'
  if (path === '/me/notifications') return '返回通知中心'
  if (path === '/users') return new URL(source, 'https://local.invalid').searchParams.has('user_id') ? '返回人员详情' : '返回人员管理'
  if (path === '/') return '返回任务看板'
  if (assetDetailEntry.test(path)) return '返回资产详情'
  if (operationsEntry.test(path)) return '返回经营数据'
  return '返回任务中心'
}
