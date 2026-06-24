<template>
  <BaseModal
    :model-value="modelValue"
    title="编辑子项商品资料"
    :show-confirm="false"
    cancel-text="关闭"
    panel-class="max-w-[min(760px,94vw)]"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <div class="sku-edit-body">
      <p v-if="errorText" class="sku-edit-error">{{ errorText }}</p>
      <div class="sku-edit-grid">
        <BaseInput
          v-model="form.productName"
          label="产品名称"
          placeholder="请输入产品名称"
          :maxlength="ERP_PRODUCT_NAME_MAX_LENGTH"
          :hint="erpProductNameHint(form.productName)"
          :error="erpProductNameError(form.productName)"
        />
        <IIdSelector
          v-model="productIIdModel"
          label="款式编码"
          placeholder="搜索或选择款式编码"
        />
        <BaseTextarea
          v-model="form.specText"
          class="sku-edit-span-2"
          label="规格 / 工艺补充"
          :rows="2"
          placeholder="除标准尺寸外的规格、工艺等补充说明"
        />
        <TaskSpecStructuredInput
          v-model="form.sizeText"
          class="sku-edit-span-2"
          label="尺寸"
          placeholder="请选择宽高或面积并填写数字"
          hint="与创建任务、单任务编辑保持一致；修改后会参与成本规则重新匹配。"
        />
        <BaseInput
          v-model="form.quantity"
          type="number"
          label="数量"
          placeholder="可选"
        />
        <BaseTextarea
          v-model="form.designRequirement"
          class="sku-edit-span-2"
          label="设计要求"
          :rows="3"
          placeholder="请输入设计要求"
        />
      </div>

      <div class="sku-edit-refs">
        <p class="sku-edit-section-title">行级参考图</p>
        <ReferenceUploadPanel
          v-model="skuReferenceRefs"
          :task-id="taskId"
          :target-sku-code="skuItem?.skuCode"
          owner-module-key="basic_info"
          upload-policy="append_only"
          compact
        />
        <p v-if="refsChanged" class="sku-edit-ref-hint">参考图变更后，保存时会同步当前子项资料。</p>
      </div>

      <div class="sku-edit-cost">
        <div class="sku-edit-cost-head">
          <p class="sku-edit-section-title">子项成本</p>
          <span class="sku-edit-cost-current">{{ currentCostText }}</span>
        </div>
        <div class="sku-edit-grid">
          <BaseInput
            v-model="form.costPrice"
            type="number"
            label="成本单价"
            placeholder="例如 10.725"
            hint="保存后仅同步当前 SKU 的 ERP 成本"
          />
          <BaseInput
            v-model="form.costReason"
            label="覆盖原因"
            placeholder="例如 运营手动修正成本"
          />
        </div>
      </div>

      <label class="sku-edit-check">
        <input v-model="form.triggerFiling" type="checkbox" />
        <span>保存后触发 ERP 同步评估</span>
      </label>
      <BaseInput v-model="form.remark" label="备注" placeholder="可选" />
    </div>

    <template #footer>
      <footer class="sku-edit-footer">
        <BaseButton variant="secondary" size="sm" :disabled="saving" @click="$emit('update:modelValue', false)">
          取消
        </BaseButton>
        <BaseButton variant="primary" size="sm" :loading="saving" :disabled="saving" @click="submit">
          保存
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import TaskSpecStructuredInput from '@/components/task/TaskSpecStructuredInput.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import type { TaskSkuItem } from '@/domain/types/task'
import {
  ERP_PRODUCT_NAME_MAX_LENGTH,
  erpProductNameError,
  erpProductNameHint,
  erpProductNameLimitMessage,
  isErpProductNameTooLong,
} from '@/domain/erp-product-name'
import { tasksApi } from '@/services/api/tasksApi'

const props = defineProps<{
  modelValue: boolean
  taskId: string
  skuItem: TaskSkuItem | null
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  saved: []
}>()

const saving = ref(false)
const errorText = ref('')
const form = ref({
  productName: '',
  productIId: '',
  designRequirement: '',
  specText: '',
  sizeText: '',
  quantity: '',
  costPrice: '',
  costReason: '',
  triggerFiling: false,
  remark: '',
})
const initialCostDraft = ref('')
const initialSpecDraft = ref('')
const initialSizeTextDraft = ref('')
const skuReferenceRefs = ref<(Record<string, unknown> | string)[]>([])
const initialReferenceRefsDraft = ref('[]')

const productIIdModel = computed({
  get: () => form.value.productIId ?? '',
  set: (value: string | undefined) => {
    form.value.productIId = String(value ?? '').trim()
  },
})

const refsChanged = computed(() => referenceRefsDraftKey(skuReferenceRefs.value) !== initialReferenceRefsDraft.value)
const specChanged = computed(() => specDraftKey() !== initialSpecDraft.value)
const sizeTextChanged = computed(() => String(form.value.sizeText ?? '').trim() !== initialSizeTextDraft.value)

const currentCostText = computed(() => {
  const cost = numericCost(props.skuItem?.costPrice)
  const estimated = numericCost(props.skuItem?.estimatedCost)
  const amount = cost ?? estimated
  if (amount == null) return props.skuItem?.requiresManualReview ? '当前待补成本' : '当前无成本'
  const source = props.skuItem?.manualCostOverride ? '手动' : cost != null ? '当前' : '预估'
  return `${source} ¥${amount.toFixed(3)}`
})

watch(
  () => [props.modelValue, props.skuItem] as const,
  () => {
    if (!props.modelValue || !props.skuItem) return
    form.value = {
      productName: String(props.skuItem.productNameSnapshot ?? ''),
      productIId: String(props.skuItem.productIId ?? ''),
      designRequirement: String(props.skuItem.designRequirement ?? ''),
      specText: String(props.skuItem.specText ?? ''),
      sizeText: String(props.skuItem.sizeText ?? ''),
      quantity: numberDraftValue(props.skuItem.quantity, 0),
      costPrice: costDraftValue(props.skuItem.costPrice ?? props.skuItem.estimatedCost),
      costReason: String(props.skuItem.manualCostOverrideReason ?? '运营手动维护子项成本'),
      triggerFiling: false,
      remark: '',
    }
    initialCostDraft.value = form.value.costPrice
    initialSpecDraft.value = specDraftKey()
    initialSizeTextDraft.value = String(form.value.sizeText ?? '').trim()
    skuReferenceRefs.value = [...(props.skuItem.referenceFileRefs ?? [])] as (Record<string, unknown> | string)[]
    initialReferenceRefsDraft.value = referenceRefsDraftKey(skuReferenceRefs.value)
    errorText.value = ''
  },
  { immediate: true },
)

function numericCost(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined
  return value
}

function costDraftValue(value: unknown): string {
  const amount = numericCost(value)
  return amount == null ? '' : amount.toFixed(3).replace(/\.?0+$/, '')
}

function numberDraftValue(value: unknown, precision = 4): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return ''
  return value.toFixed(precision).replace(/\.?0+$/, '')
}

function normalizedCostDraft(value: string): string {
  const trimmed = String(value ?? '').trim()
  if (!trimmed) return ''
  const amount = Number(trimmed)
  if (!Number.isFinite(amount)) return trimmed
  return String(amount)
}

function referenceRefsDraftKey(refs: (Record<string, unknown> | string)[]): string {
  return JSON.stringify(refs ?? [])
}

function specDraftKey(): string {
  return JSON.stringify({
    specText: String(form.value.specText ?? '').trim(),
    sizeText: String(form.value.sizeText ?? '').trim(),
    quantity: String(form.value.quantity ?? '').trim(),
  })
}

function optionalPositiveIntegerDraft(value: string | number, label: string): number | undefined {
  const text = String(value ?? '').trim()
  if (!text) return 0
  const amount = Number(text)
  if (!Number.isFinite(amount) || amount < 0) {
    throw new Error(`请输入有效${label}`)
  }
  if (!Number.isInteger(amount)) {
    throw new Error(`${label}必须是整数`)
  }
  return amount
}

async function submit() {
  if (!props.skuItem) return
  const skuItemID = props.skuItem.id
  if (isErpProductNameTooLong(form.value.productName)) {
    errorText.value = erpProductNameLimitMessage('产品名称')
    return
  }
  const costDraft = String(form.value.costPrice ?? '').trim()
  const shouldPatchCost = normalizedCostDraft(costDraft) !== normalizedCostDraft(initialCostDraft.value)
  if (shouldPatchCost) {
    const cost = Number(costDraft)
    if (!Number.isFinite(cost) || cost < 0) {
      errorText.value = '请输入有效成本'
      return
    }
    if (typeof skuItemID !== 'number') {
      errorText.value = '缺少 SKU 子项 ID，无法维护成本'
      return
    }
  }
  let quantity: number | undefined
  try {
    quantity = optionalPositiveIntegerDraft(form.value.quantity, '数量')
  } catch (err) {
    errorText.value = err instanceof Error ? err.message : '请输入有效数量'
    return
  }
  saving.value = true
  errorText.value = ''
  const payload: Record<string, unknown> = {
    product_name: form.value.productName.trim() || null,
    product_i_id: form.value.productIId.trim() || null,
    design_requirement: form.value.designRequirement.trim() || null,
    spec_text: form.value.specText.trim(),
    size_text: form.value.sizeText.trim(),
    quantity,
    reference_file_refs: skuReferenceRefs.value,
    trigger_filing: form.value.triggerFiling === true || refsChanged.value || specChanged.value || undefined,
    remark: form.value.remark.trim() || undefined,
  }
  if (sizeTextChanged.value) {
    payload.width = 0
    payload.height = 0
    payload.area = 0
  }
  try {
    if (typeof skuItemID !== 'number') {
      throw new Error('缺少子项 ID，无法维护子项商品资料')
    }
    await tasksApi.patchSkuItem(props.taskId, skuItemID, payload)

    if (shouldPatchCost) {
      await tasksApi.patchSkuItemCostInfo(props.taskId, skuItemID as number, {
        cost_price: Number(costDraft),
        manual_cost_override: true,
        manual_cost_override_reason: form.value.costReason.trim() || '运营手动维护子项成本',
        remark: form.value.remark.trim() || `维护子项成本 ${props.skuItem.skuCode ?? ''}`.trim(),
      })
    }
  } catch (err) {
    errorText.value = err instanceof Error ? err.message : '保存失败'
    return
  } finally {
    saving.value = false
  }
  emit('saved')
  emit('update:modelValue', false)
}
</script>

<style scoped>
.sku-edit-body {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.sku-edit-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.sku-edit-span-2 {
  grid-column: 1 / -1;
}

.sku-edit-section-title {
  margin: 0 0 0.35rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: #475467;
}

.sku-edit-refs {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  border: 1px solid #eaecf0;
  border-radius: 0.75rem;
  background: #fff;
  padding: 0.75rem;
}

.sku-edit-ref-hint {
  margin: 0;
  color: #047857;
  font-size: 0.75rem;
}

.sku-edit-cost {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  border: 1px solid #eaecf0;
  border-radius: 0.75rem;
  background: #f9fafb;
  padding: 0.75rem;
}

.sku-edit-cost-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.sku-edit-cost-current {
  color: #344054;
  font-size: 0.75rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.sku-edit-check {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
  color: #475467;
}

.sku-edit-error {
  margin: 0;
  padding: 0.5rem 0.6rem;
  border: 1px solid #fecaca;
  border-radius: 0.5rem;
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.8125rem;
}

.sku-edit-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 680px) {
  .sku-edit-grid {
    grid-template-columns: 1fr;
  }

  .sku-edit-cost-head {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.2rem;
  }
}
</style>
