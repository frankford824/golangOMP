<template>
  <div class="warehouse-status-block">
    <div class="status-row">
      <span class="status-label">仓库状态</span>
      <span v-if="task.warehouseSubStatus" class="status-badge" :class="statusBadgeClass">
        {{ getWarehouseSubStatusLabel(task.warehouseSubStatus) }}
      </span>
      <span v-else class="text-[rgb(var(--yb-text-faint))] text-sm">-</span>
    </div>

    <!-- 操作按钮 -->
    <div class="action-row">
      <BaseButton
        v-if="can('warehouse.receive') && canReceive"
        size="sm"
        variant="primary"
        :loading="receiveLoading"
        :disabled="receiveLoading"
        @click="confirmReceive"
      >
        确认接收
      </BaseButton>
      <BaseButton
        v-if="can('warehouse.return') && canReturn"
        size="sm"
        variant="danger"
        disabled
        :title="unsupportedWarehouseActionHint"
        @click="openReturnDialog"
      >
        退回
      </BaseButton>
    </div>
    <p v-if="can('warehouse.return') && canReturn" class="action-hint">
      {{ unsupportedWarehouseActionHint }}
    </p>
    <p v-if="actionError" class="action-hint action-error">{{ actionError }}</p>

  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Task } from '@/domain/types/task'
import { getWarehouseSubStatusLabel } from '@/domain/enums/task-status'
import {
  getTaskActionAvailability,
  shouldHideWarehouseReceiveActions,
} from '@/domain/task-action-availability'
import { formatTaskActionDenyMessage } from '@/domain/task-action-deny'
import { usePermission } from '@/composables/usePermission'
import { useSubmitGuard } from '@/composables/useSubmitGuard'
import { useTasksStore } from '@/stores/tasks'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{ task: Task }>()
const { can } = usePermission()
const tasksStore = useTasksStore()

const { submitting: receiveLoading, guard: receiveGuard } = useSubmitGuard()
const actionError = ref('')
// v4.2 修复：老板要求 + 仓库退回缺少真实接口时不再允许本地改状态
const unsupportedWarehouseActionHint = '当前系统暂不支持仓库退回操作，请先在任务详情中查看状态。'

const canReceive = computed(() => {
  if (shouldHideWarehouseReceiveActions(props.task)) return false
  return (
    getTaskActionAvailability(props.task).canShowWarehouseActions &&
    (props.task.warehouseSubStatus === 'PENDING_RECEIVE' ||
      props.task.warehouseReceiveStatus === 'pending')
  )
})
const canReturn = computed(() => {
  if (shouldHideWarehouseReceiveActions(props.task)) return false
  return (
    getTaskActionAvailability(props.task).canShowWarehouseActions &&
    (props.task.warehouseSubStatus === 'RECEIVED' ||
      props.task.warehouseReceiveStatus === 'received')
  )
})

const statusBadgeClass = computed(() => {
  const s = props.task.warehouseSubStatus
  if (s === 'RECEIVED' || s === 'DONE') return 'badge-success'
  if (s === 'RETURNED') return 'badge-danger'
  if (s === 'PENDING_RECEIVE' || s === 'PACKING') return 'badge-warning'
  return 'badge-default'
})

async function confirmReceive() {
  actionError.value = ''
  await receiveGuard(async () => {
    try {
      await tasksStore.receiveInWarehouse(props.task.id)
      await tasksStore.forceRefreshList()
    } catch (e) {
      actionError.value = formatTaskActionDenyMessage(e, '仓库接收失败')
    }
  })
}

function openReturnDialog() {
  // v4.2 修复：老板要求 + 仅展示接口未接入提示，不再打开本地退回表单
}
</script>

<style scoped>
.warehouse-status-block { padding: 0.5rem 0; }
.status-row { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.625rem; }
.status-label { font-size: 0.75rem; color: rgb(var(--yb-text-muted-strong)); }
.status-badge { font-size: 0.75rem; font-weight: 500; padding: 0.15rem 0.5rem; border-radius: 4px; }
.badge-success { background: rgb(var(--yb-success-emerald) / 0.15); color: rgb(var(--yb-success-emerald)); }
.badge-warning { background: rgb(var(--yb-warning-accent) / 0.15); color: rgb(var(--yb-warning)); }
.badge-danger { background: rgb(var(--yb-danger) / 0.1); color: rgb(var(--yb-danger)); }
.badge-default { background: rgb(var(--yb-surface-slate)); color: rgb(var(--yb-text-soft)); }
.action-row { display: flex; gap: 0.5rem; margin-bottom: 0.5rem; }
.action-hint { margin: 0 0 0.5rem; font-size: 0.75rem; color: rgb(var(--yb-text-placeholder)); }
.action-error { color: rgb(var(--yb-danger-text)); }
</style>
