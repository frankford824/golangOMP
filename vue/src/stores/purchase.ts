import { defineStore } from 'pinia'
import type { PurchaseInfo, PurchaseStatus } from '@/types'
import { useTasksStore } from './tasks'

export const usePurchaseStore = defineStore('purchase', () => {
  const tasksStore = useTasksStore()

  function ensurePurchaseInfo(taskId: string): PurchaseInfo | null {
    const task = tasksStore.getById(taskId)
    if (!task || task.taskType !== 'PURCHASE_TASK') return null
    const current: PurchaseInfo = task.purchaseInfo ?? {
      status: 'PendingPurchase',
    }
    return current
  }

  function updatePurchaseInfo(taskId: string, patch: Partial<PurchaseInfo>, action_id?: string) {
    const base = ensurePurchaseInfo(taskId)
    if (!base) return
    const next: PurchaseInfo = {
      ...base,
      ...patch,
    }
    tasksStore.updateTask(
      taskId,
      {
        purchaseInfo: next,
      },
      action_id,
    )
  }

  function updatePurchasePrice(
    taskId: string,
    price: { amount: number; currency: string },
    action_id?: string,
  ) {
    updatePurchaseInfo(
      taskId,
      {
        purchasePrice: price,
      },
      action_id,
    )
  }

  function updateSupplier(
    taskId: string,
    payload: { supplierId?: string; supplierName?: string },
    action_id?: string,
  ) {
    updatePurchaseInfo(
      taskId,
      {
        supplierId: payload.supplierId,
        supplierName: payload.supplierName,
      },
      action_id,
    )
  }

  function setPurchaseStatus(taskId: string, status: PurchaseStatus, action_id?: string) {
    const base = ensurePurchaseInfo(taskId)
    if (!base) return
    const next: PurchaseInfo = {
      ...base,
      status,
    }
    tasksStore.updateTask(
      taskId,
      {
        purchaseInfo: next,
      },
      action_id,
    )
    // 采购完成后，采购任务可直接进入仓库节点参与结单判断
    if (status === 'Purchased') {
      tasksStore.markPendingWarehouse(taskId)
    }
  }

  return {
    updatePurchaseInfo,
    updatePurchasePrice,
    updateSupplier,
    setPurchaseStatus,
  }
})

