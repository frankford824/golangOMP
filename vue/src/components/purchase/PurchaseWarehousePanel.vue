<template>
  <section class="info-block">
    <h4>采购与仓库</h4>
    <div v-if="!task">
      <p class="text-xs text-slate-500">任务信息加载中...</p>
    </div>
    <template v-else>
      <p class="hint-text">
        立项所需的采购信息已在创建任务时填写；金额与数量请以「成本与价格」为准，此处仅用于推进采购与仓库节点。
      </p>
      <PurchaseBusinessInfoSupplement :task="task" class="mt-3" />
      <div class="field mt-3">
        <label class="field-label">采购状态</label>
        <p class="status-text">
          {{ currentStatusLabel }}
        </p>
      </div>

      <div
        v-if="showWarehouseHandoffNotice"
        class="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-950"
      >
        <p class="font-medium text-amber-950">尚未进入仓库中心待接收队列</p>
        <p class="mt-1 leading-relaxed text-amber-900/95">
          「已采购」只表示采购记录已完成。仓库中心列表由后端在业务信息齐全后才会展示本任务；下列项未满足时，仓库节点不会触发。
        </p>
        <ul v-if="blockingLines.length" class="mt-2 list-inside list-disc space-y-0.5 text-amber-900">
          <li v-for="(line, i) in blockingLines" :key="i">{{ line }}</li>
        </ul>
        <p v-else class="mt-1 text-amber-800">
          当前无法交接仓库，请补全任务业务信息或联系后端确认规则。
        </p>
      </div>

      <p v-if="submitError" class="mt-2 text-xs text-red-600">{{ submitError }}</p>
      <div class="mt-4 flex flex-wrap gap-2">
        <BaseButton
          size="sm"
          variant="primary"
          :disabled="!canMarkPurchased || submitting"
          :loading="submitting"
          @click="openConfirm"
        >
          标记已采购并提交仓库
        </BaseButton>
        <span v-if="!canMarkPurchased && markDisabledHint" class="self-center text-xs text-slate-500">
          {{ markDisabledHint }}
        </span>
      </div>
    </template>

    <BaseModal
      v-model="showConfirm"
      title="确认标记为已采购"
    >
      <p class="mb-3 text-sm text-slate-700">
        即将把该采购任务标记为「已采购」。若分类、规格、采购价与数量等信息已齐全，后端会将任务推入仓库待接收队列；否则会停留在当前节点，仓库中心可能仍看不到本条任务。请填写原因或备注，便于后续追溯。
      </p>
      <BaseTextarea
        v-model="confirmReason"
        :rows="3"
        placeholder="请输入标记原因（必填）"
      />
      <template #footer>
        <div class="flex justify-end gap-2">
          <BaseButton size="sm" variant="ghost" :disabled="submitting" @click="showConfirm = false">
            取消
          </BaseButton>
          <BaseButton
            size="sm"
            variant="primary"
            :disabled="!confirmReason.trim()"
            :loading="submitting"
            @click="onConfirmPurchased"
          >
            确认
          </BaseButton>
        </div>
      </template>
    </BaseModal>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Task } from '@/domain/types/task'
import type { PurchaseStatus } from '@/domain/types/purchase'
import { useTasksStore } from '@/stores/tasks'
import { tasksApi } from '@/services/api/tasksApi'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import PurchaseBusinessInfoSupplement from '@/components/purchase/PurchaseBusinessInfoSupplement.vue'
import { getPurchaseStatusLabel } from '@/domain/enums/purchase-status'
import { warehouseBlockingReasonLine } from '@/utils/warehouse-blocking'

const props = defineProps<{
  task: Task
}>()

const tasksStore = useTasksStore()

const showConfirm = ref(false)
const confirmReason = ref('')
const submitting = ref(false)
const submitError = ref('')

const currentStatusLabel = computed(() => {
  const p = props.task.purchaseInfo
  const s = (p?.status ?? 'PendingPurchase') as PurchaseStatus
  return getPurchaseStatusLabel(s)
})

const canMarkPurchased = computed(() => {
  const s = (props.task.purchaseInfo?.status ?? 'PendingPurchase') as PurchaseStatus
  if (s === 'Purchased' || s === 'Cancelled' || s === 'NotRequired') return false
  return s === 'PendingPurchase' || s === 'Purchasing'
})

const markDisabledHint = computed(() => {
  const s = (props.task.purchaseInfo?.status ?? 'PendingPurchase') as PurchaseStatus
  if (s === 'Purchased') return '当前已为已采购状态'
  if (s === 'Cancelled') return '任务已取消采购'
  return ''
})

/** 后端明确返回不可准备仓库，或带有阻塞原因列表时展示说明 */
const showWarehouseHandoffNotice = computed(() => {
  const t = props.task
  if (t.canPrepareWarehouse === false) return true
  const br = t.warehouseBlockingReasons
  return Array.isArray(br) && br.length > 0
})

const blockingLines = computed(() => {
  const br = props.task.warehouseBlockingReasons
  if (!Array.isArray(br) || br.length === 0) return []
  return br.map((r) => warehouseBlockingReasonLine(r.code, r.message))
})

function openConfirm() {
  if (!canMarkPurchased.value || submitting.value) return
  submitError.value = ''
  showConfirm.value = true
}

/**
 * OpenAPI：PATCH procurement 置 status=completed；若后端仅实现 advance，则再尝试 complete 或分步推进。
 */
async function persistProcurementCompleted(taskId: string, remark: string): Promise<void> {
  try {
    await tasksApi.patchTaskProcurement(taskId, {
      status: 'completed',
      purchase_remark: remark,
    })
    return
  } catch {
    try {
      await tasksApi.advanceTaskProcurement(taskId, { action: 'complete', remark })
      return
    } catch {
      // draft → 需先 prepare / start（忽略已处于后续态的 400）
      try {
        await tasksApi.advanceTaskProcurement(taskId, { action: 'prepare' })
      } catch {
        /* noop */
      }
      try {
        await tasksApi.advanceTaskProcurement(taskId, { action: 'start' })
      } catch {
        /* noop */
      }
      await tasksApi.advanceTaskProcurement(taskId, { action: 'complete', remark })
    }
  }
}

async function onConfirmPurchased() {
  const reason = confirmReason.value.trim()
  if (!reason) return
  submitting.value = true
  submitError.value = ''
  try {
    await persistProcurementCompleted(props.task.id, reason)
    await tasksStore.loadTaskById(props.task.id)
    await tasksStore.forceRefreshList()
    showConfirm.value = false
    confirmReason.value = ''
  } catch (e) {
    submitError.value = e instanceof Error ? e.message : '提交失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.info-block {
  @apply p-4 bg-white border border-slate-200 rounded-lg;
}
.field {
  @apply space-y-1;
}
.field-label {
  @apply block text-xs font-medium text-slate-600;
}
.status-text {
  @apply text-sm text-slate-800;
}
.hint-text {
  @apply text-xs text-slate-500 leading-relaxed;
}
</style>
