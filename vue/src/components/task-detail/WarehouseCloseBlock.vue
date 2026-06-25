<template>
  <section
    class="detail-block h-full flex flex-col rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-sm p-6"
    :class="{ 'warehouse-close--compact': compactLayout }"
  >
    <div class="block-header">
      <div class="flex items-center gap-2">
        <span class="block-icon">W</span>
        <h3 class="block-title">仓库与结单</h3>
      </div>
    </div>

    <!-- 采购任务：使用现有 PurchaseWarehousePanel -->
    <PurchaseWarehousePanel v-if="isPurchase" :task="task" />

    <!-- 非采购任务：WarehouseStatusBlock 占位 -->
    <WarehouseStatusBlock v-else :task="task" />

    <!-- 原品/新品：补全主档以便结单（与 workflow.cannot_close_reasons 对齐）；终态隐藏 -->
    <TaskCloseMasterDataSupplement :task="task" :compact="compactLayout" />

    <!-- 结单条件面板 -->
    <div class="close-section">
      <TaskCloseConditionPanel ref="conditionPanel" :task="task" />
      <p v-if="isTerminal" class="close-terminal-note">任务已结束，无需再结单。</p>
      <p v-if="closeError" class="close-error">{{ closeError }}</p>
      <!-- 仓库侧一键结单：底层仍按后端两段式 complete -> close 执行 -->
      <div v-if="canShowWarehouseComplete && !isTerminal" class="close-action">
        <BaseButton
          class="close-btn"
          variant="primary"
          :disabled="!canWarehouseComplete || closeLoading"
          @click="closeTask"
        >
          {{ closeLoading ? '完成中…' : '完成仓库处理' }}
        </BaseButton>
        <span v-if="!canWarehouseComplete" class="close-hint">{{ warehouseCompleteBlockedHint }}</span>
      </div>
      <div v-if="canShowCloseButton && !isTerminal" class="close-action">
        <BaseButton
          class="close-btn"
          variant="primary"
          :disabled="!canClose || closeLoading"
          @click="closeTask"
        >
          {{ closeLoading ? '提交中…' : '确认结单' }}
        </BaseButton>
        <span v-if="!canClose" class="close-hint">{{ closeBlockedHint }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, inject } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import {
  canCloseTaskForArchive,
  formatCloseArchiveError,
  isTaskCloseFlowTerminal,
} from '@/domain/task-close-eligibility'
import {
  getTaskActionAvailability,
  shouldHideWarehouseCompleteAction,
} from '@/domain/task-action-availability'
import { formatTaskActionDenyMessage } from '@/domain/task-action-deny'
import { usePermission } from '@/composables/usePermission'
import { useTasksStore } from '@/stores/tasks'
import PurchaseWarehousePanel from '@/components/purchase/PurchaseWarehousePanel.vue'
import WarehouseStatusBlock from './WarehouseStatusBlock.vue'
import TaskCloseConditionPanel from '@/components/business/TaskCloseConditionPanel.vue'
import TaskCloseMasterDataSupplement from './TaskCloseMasterDataSupplement.vue'
import BaseButton from '@/components/base/BaseButton.vue'

withDefaults(
  defineProps<{
    /** 任务详情右栏：压缩补全区与内边距 */
    compactLayout?: boolean
  }>(),
  { compactLayout: false },
)

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[WarehouseCloseBlock] 必须在 TaskDetailView 内使用')

const task = computed(() => injected.value!)
const { can } = usePermission()
const tasksStore = useTasksStore()

const isPurchase = computed(() => task.value.businessType === 'PURCHASE_TASK')
const isTerminal = computed(() => isTaskCloseFlowTerminal(task.value))
const availability = computed(() => getTaskActionAvailability(task.value))

/** 结单阶段按钮：后端 workflow 已允许结单 + Legacy 状态 = PendingClose */
const canShowCloseButton = computed(
  () => can('task.close') && availability.value.canShowClose,
)
const canClose = computed(
  () =>
    !isTerminal.value &&
    availability.value.canShowClose &&
    canCloseTaskForArchive(task.value).allowed,
)
const closeBlockedHint = computed(() => {
  if (isTerminal.value) return ''
  if (canClose.value) return ''
  const { reasons } = canCloseTaskForArchive(task.value)
  return reasons.length > 0 ? reasons.join('；') : '请满足上方结单条件'
})

/** 仓库完成入口：PendingProductionTransfer + 已接收；底层 archiveTask 会先 complete 再 close */
const canShowWarehouseComplete = computed(
  () =>
    !shouldHideWarehouseCompleteAction(task.value) &&
    availability.value.canShowWarehouseComplete &&
    can('task.close'),
)
const canWarehouseComplete = computed(
  () =>
    !isTerminal.value &&
    availability.value.canShowWarehouseComplete &&
    task.value.warehouseSubStatus === 'RECEIVED',
)
const warehouseCompleteBlockedHint = computed(() => {
  if (canWarehouseComplete.value) return ''
  if (task.value.warehouseSubStatus !== 'RECEIVED') {
    return '仓库尚未接收，无法推进到待结单'
  }
  return ''
})
const conditionPanel = ref<InstanceType<typeof TaskCloseConditionPanel> | null>(null)
const closeError = ref('')
const closeLoading = ref(false)

async function closeTask() {
  closeError.value = ''
  // 仓库侧一键结单：PendingClose → close；PendingProductionTransfer → warehouseComplete 后 close
  // archiveTask store action 自行按 status 分派
  if (!canClose.value && !canWarehouseComplete.value) return
  closeLoading.value = true
  try {
    await tasksStore.archiveTask(task.value.id)
  } catch (e) {
    closeError.value = formatTaskActionDenyMessage(e, formatCloseArchiveError(e))
  } finally {
    closeLoading.value = false
  }
}
</script>

<style scoped>
.block-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
  justify-content: space-between;
}
.block-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 0.375rem;
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-placeholder));
  font-size: 0.75rem;
  flex-shrink: 0;
}
.block-title { font-size: 0.875rem; font-weight: 600; color: rgb(var(--yb-text-deep)); margin: 0; }
.close-section { margin-top: 0.5rem; display: flex; flex-direction: column; gap: 0.75rem; }
.close-action { display: flex; align-items: flex-start; gap: 0.75rem; flex-wrap: wrap; }
.close-btn { flex-shrink: 0; white-space: nowrap; min-width: 5.5rem; }
.close-hint { font-size: 0.75rem; color: rgb(var(--yb-text-faint)); min-width: 0; flex: 1; overflow-wrap: anywhere; word-break: break-word; }
.close-error { font-size: 0.875rem; color: rgb(var(--yb-danger)); margin: 0; }
.close-terminal-note { font-size: 0.8125rem; color: rgb(var(--yb-text-muted-strong)); margin: 0; }
.warehouse-close--compact {
  padding: 0.75rem 0.85rem;
}
.warehouse-close--compact .block-header {
  margin-bottom: 0.15rem;
}
.warehouse-close--compact .close-section {
  margin-top: 0.35rem;
  gap: 0.5rem;
}
</style>
