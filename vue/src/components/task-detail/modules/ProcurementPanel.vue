<template>
  <ModuleSection :module="module" title="采购" eyebrow="procurement">
    <template #default="{ readonly }">
      <div class="mt-3 space-y-3 text-sm text-[var(--v1-text-secondary)]">
        <p>采购记录用于仓库交接门禁，提交交仓前请先保存并完成采购流程。</p>
        <dl class="grid grid-cols-2 gap-2 rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3">
          <div>
            <dt>采购状态</dt>
            <dd class="font-medium text-[var(--v1-text-primary)]">{{ procurementStatusLabel }}</dd>
          </div>
          <div>
            <dt>采购价</dt>
            <dd class="font-medium text-[var(--v1-text-primary)]">{{ procurementPriceLabel }}</dd>
          </div>
          <div>
            <dt>采购数量</dt>
            <dd class="font-medium text-[var(--v1-text-primary)]">{{ quantityLabel }}</dd>
          </div>
          <div>
            <dt>供应商</dt>
            <dd class="font-medium text-[var(--v1-text-primary)]">{{ supplierLabel }}</dd>
          </div>
        </dl>

        <div class="grid grid-cols-1 gap-2 md:grid-cols-2">
          <label class="field-item">
            <span>采购价</span>
            <input
              v-model.number="form.procurementPrice"
              type="number"
              min="0"
              step="0.01"
              :disabled="readonly || saving"
              placeholder="请输入采购价"
            />
          </label>
          <label class="field-item">
            <span>采购数量</span>
            <input
              v-model.number="form.quantity"
              type="number"
              min="1"
              step="1"
              :disabled="readonly || saving"
              placeholder="请输入采购数量"
            />
          </label>
          <label class="field-item md:col-span-2">
            <span>供应商名称</span>
            <input
              v-model="form.supplierName"
              type="text"
              :disabled="readonly || saving"
              placeholder="请输入供应商名称"
            />
          </label>
        </div>

        <p v-if="errorMessage" class="text-red-600">{{ errorMessage }}</p>
        <p v-else-if="successMessage" class="text-emerald-600">{{ successMessage }}</p>

        <div class="flex items-center justify-end gap-2">
          <button
            type="button"
            class="rounded-md border border-[var(--v1-border)] px-3 py-1.5 text-sm text-[var(--v1-text-primary)] disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="readonly || saving"
            @click="saveProcurement"
          >
            保存采购信息
          </button>
          <button
            type="button"
            class="rounded-md bg-[var(--v1-text-primary)] px-3 py-1.5 text-sm text-white disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="readonly || saving || isCompleted"
            @click="completeProcurement"
          >
            {{ isCompleted ? '采购已完成' : '完成采购' }}
          </button>
        </div>
      </div>
    </template>
  </ModuleSection>
</template>

<script setup lang="ts">
import { computed, inject, reactive, ref, watch } from 'vue'
import type { ComputedRef } from 'vue'
import type { ModuleSummary } from '@/services/apiTypes'
import type { Task } from '@/domain/types/task'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { useTasksStore } from '@/stores/tasks'
import ModuleSection from '@/components/task-detail/ModuleSection.vue'

defineProps<{ module?: ModuleSummary }>()

const tasksStore = useTasksStore()
const injectedTask = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
const task = computed(() => injectedTask?.value ?? null)

const form = reactive({
  procurementPrice: undefined as number | undefined,
  quantity: undefined as number | undefined,
  supplierName: '',
})
const saving = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const isCompleted = computed(() => String(task.value?.procurementApiStatus ?? '').toLowerCase() === 'completed')
const procurementStatusLabel = computed(() => task.value?.procurementApiStatus ?? 'draft')
const procurementPriceLabel = computed(() => {
  const amount = task.value?.purchaseInfo?.purchasePrice?.amount
  return typeof amount === 'number' && Number.isFinite(amount) ? `¥${amount}` : '-'
})
const quantityLabel = computed(() => {
  const quantity = task.value?.purchaseInfo?.quantity
  return typeof quantity === 'number' && Number.isFinite(quantity) ? String(quantity) : '-'
})
const supplierLabel = computed(() => {
  const name = String(task.value?.purchaseInfo?.supplierName ?? '').trim()
  return name || '-'
})

watch(
  task,
  (current) => {
    form.procurementPrice = current?.purchaseInfo?.purchasePrice?.amount
    form.quantity = current?.purchaseInfo?.quantity
    form.supplierName = String(current?.purchaseInfo?.supplierName ?? '')
  },
  { immediate: true },
)

function validateProcurementForm(): string | null {
  if (form.procurementPrice == null || !Number.isFinite(form.procurementPrice)) return '请填写采购价'
  if (form.quantity == null || !Number.isFinite(form.quantity) || form.quantity <= 0) return '请填写采购数量'
  return null
}

async function saveProcurement() {
  const current = task.value
  if (!current) return
  const invalid = validateProcurementForm()
  if (invalid) {
    errorMessage.value = invalid
    successMessage.value = ''
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    await tasksStore.saveProcurementRecord(current.id, {
      status: (current.procurementApiStatus as 'draft' | 'prepared' | 'in_progress' | 'completed') ?? 'draft',
      procurement_price: form.procurementPrice,
      quantity: form.quantity,
      supplier_name: form.supplierName.trim(),
      purchase_remark: String(current.note ?? '').trim() || undefined,
    })
    successMessage.value = '采购信息已保存'
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '保存采购信息失败'
    successMessage.value = ''
  } finally {
    saving.value = false
  }
}

async function completeProcurement() {
  const current = task.value
  if (!current) return
  const invalid = validateProcurementForm()
  if (invalid) {
    errorMessage.value = invalid
    successMessage.value = ''
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    await tasksStore.bootstrapProcurement(current.id, {
      procurement_price: form.procurementPrice as number,
      quantity: form.quantity as number,
      supplier_name: form.supplierName.trim(),
      purchase_remark: String(current.note ?? '').trim() || undefined,
    })
    successMessage.value = '采购流程已完成'
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '采购流程推进失败'
    successMessage.value = ''
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.field-item {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.field-item > span {
  font-size: 0.75rem;
  color: var(--v1-text-secondary);
}

.field-item > input {
  border: 1px solid var(--v1-border);
  border-radius: 0.5rem;
  padding: 0.5rem 0.625rem;
  background: #fff;
  color: var(--v1-text-primary);
}
</style>
