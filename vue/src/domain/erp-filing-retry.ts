import type { Task, TaskSkuItem } from '@/domain/types/task'
import { RoleEnum } from '@/types'

const FILING_FAILED = 'filing_failed'

function isFilingFailedStatus(status: string | undefined | null): boolean {
  return String(status ?? '').trim() === FILING_FAILED
}

function skuItemNeedsErpFilingRetry(item: TaskSkuItem): boolean {
  if (isFilingFailedStatus(item.filing_status)) return true
  if (isFilingFailedStatus(item.erp_sync_status)) return true
  if (item.erp_sync_required === true) return true
  return false
}

/** 任务是否处于可手动重试 ERP 建档/同步的状态（读模型字段，不推导业务规则）。 */
export function taskNeedsErpFilingRetry(task: Task | null | undefined): boolean {
  if (!task) return false
  if (isFilingFailedStatus(task.filing_status)) return true
  if (task.erp_sync_required === true) return true
  const items = task.skuItems
  if (Array.isArray(items) && items.length > 0) {
    return items.some(skuItemNeedsErpFilingRetry)
  }
  return false
}

/** Ops / Admin 系角色可见手动重试入口（与 filing/retry 路由角色族对齐的子集）。 */
export function canUserRetryErpFiling(hasAnyRole: (roles: readonly string[]) => boolean): boolean {
  return hasAnyRole([
    RoleEnum.OPS,
    'admin',
    RoleEnum.SUPER_ADMIN,
    RoleEnum.HR_ADMIN,
    RoleEnum.DEPT_ADMIN,
    'role_admin',
  ])
}
