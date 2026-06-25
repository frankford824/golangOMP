<template>
  <ModuleSection :module="module" title="云仓" eyebrow="warehouse">
    <template #default="{ readonly }">
      <div class="mt-3 space-y-3 text-sm text-[var(--v1-text-secondary)]">
        <p v-if="!isPurchaseTask">云仓动作按 allowed_actions 和模块状态控制。</p>
        <template v-else>
          <p>
            采购任务交仓以后端 `workflow.can_prepare_warehouse` 为准；若后端返回缺失字段，请先在采购模块补齐。
          </p>
          <div class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3">
            <p>采购状态：{{ procurementStatus }}</p>
            <p>可交仓：{{ canPrepareWarehouse ? '是' : '否' }}</p>
            <p v-if="blockingMessage" class="mt-1 text-[rgb(var(--yb-danger))]">{{ blockingMessage }}</p>
          </div>
          <div class="flex justify-end">
            <button
              type="button"
              class="rounded-md bg-[var(--v1-text-primary)] px-3 py-1.5 text-sm text-[rgb(var(--yb-text-inverse))] disabled:cursor-not-allowed disabled:opacity-50"
              :disabled="readonly || preparing || !canPrepareWarehouse"
              @click="prepareWarehouse"
            >
              {{ preparing ? '提交中...' : '交仓准备' }}
            </button>
          </div>
        </template>
      </div>
    </template>
  </ModuleSection>
</template>

<script setup lang="ts">
import { computed, inject, ref } from 'vue'
import type { ComputedRef } from 'vue'
import type { ModuleSummary } from '@/services/apiTypes'
import type { Task } from '@/domain/types/task'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { useTasksStore } from '@/stores/tasks'
import { tasksApi } from '@/services/api/tasksApi'
import { warehouseBlockingReasonLine } from '@/utils/warehouse-blocking'
import ModuleSection from '@/components/task-detail/ModuleSection.vue'

defineProps<{ module?: ModuleSummary }>()

const tasksStore = useTasksStore()
const injectedTask = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
const task = computed(() => injectedTask?.value ?? null)
const preparing = ref(false)
const blockingMessage = ref('')

const isPurchaseTask = computed(
  () =>
    !!task.value &&
    (task.value.businessType === 'PURCHASE_TASK' || task.value.taskType === 'PURCHASE_TASK'),
)
const procurementStatus = computed(() => task.value?.procurementApiStatus ?? '-')
const canPrepareWarehouse = computed(
  () =>
    task.value?.canPrepareWarehouse === true ||
    task.value?.warehousePrepareReady === true ||
    task.value?.procurementApiStatus === 'completed',
)

function parsePrepareError(err: unknown): string | null {
  const details =
    (err as any)?.response?.data?.error?.details ??
    (err as any)?.data?.error?.details ??
    (err as any)?.response?.data?.details
  const missingSummary = details?.missing_fields_summary_cn
  if (typeof missingSummary === 'string' && missingSummary.trim()) return missingSummary.trim()
  const missingFields = details?.missing_fields
  if (Array.isArray(missingFields) && missingFields.length > 0) {
    return `缺少字段：${missingFields.map((item: unknown) => String(item)).join('、')}`
  }
  const reasons = details?.warehouse_blocking_reasons
  if (Array.isArray(reasons) && reasons.length > 0) {
    return reasons
      .map((reason: any) => warehouseBlockingReasonLine(String(reason?.code ?? ''), String(reason?.message ?? '')))
      .filter((line: string) => line.trim().length > 0)
      .join('；')
  }
  return null
}

async function prepareWarehouse() {
  if (!task.value) return
  preparing.value = true
  blockingMessage.value = ''
  try {
    await tasksApi.warehousePrepare(task.value.id)
    await tasksStore.loadTaskById(task.value.id)
  } catch (error) {
    blockingMessage.value = parsePrepareError(error) ?? (error instanceof Error ? error.message : '交仓准备失败')
  } finally {
    preparing.value = false
  }
}
</script>
