/**
 * 定制审核表单（共享弹窗）。
 *
 * 供两处入口共用：
 *   1. 任务详情 `AuditOutsourceBlock` 的「提交定制审核」按钮（mode=initial）。
 *   2. 审核工作台 `CustomizationReviewActionPanel` 的定制初审 / 二次效果审核入口。
 *
 * 组件职责：UI 门禁 + payload 组装；
 * 绝对禁止：推导 task_status、伪造成功、或在此处直接调用接口。
 * 调用方接 `submit` 事件自行决定 endpoint：
 *   - `initial` → POST /v1/tasks/:id/customization/review
 *   - `effect`  → POST /v1/customization-jobs/:jobId/effect-review
 *
 * v7.0 规范一致：payload 字段保留既有契约（reviewer_id / source_asset_id /
 * customization_review_decision / customization_level_* / customization_price /
 * customization_weight_factor / customization_note），另追加子表字段
 * `market_tier / price_tier / ref_price / ref_inventory / order_no`
 * （customization_reviews 子表空位），后端按需消费，不破坏现有契约。
 */
<template>
  <BaseModal
    :model-value="modelValue"
    :title="dialogTitle"
    :show-confirm="false"
    panel-class="max-w-2xl"
    @update:model-value="(v) => emit('update:modelValue', v)"
    @cancel="onCancel"
  >
    <div class="customization-modal">
      <p class="customization-modal-hint">
        {{ hintText }}
      </p>
      <div class="customization-modal-grid">
        <BaseInput
          v-model="form.reviewerId"
          label="审核人 ID *"
          placeholder="请输入审核人 ID"
        />
        <BaseInput
          v-if="mode === 'initial'"
          v-model="form.sourceAssetId"
          label="源资产 ID"
          placeholder="可选，留空则按后端规则处理"
        />
        <label class="field">
          <span class="field-label">审核决策</span>
          <select v-model="form.decision" class="reason-textarea">
            <option value="approved">通过</option>
            <option value="return_to_designer">退回设计</option>
            <option value="reviewer_fixed">审核人修正</option>
          </select>
        </label>
        <BaseInput
          v-model="form.levelCode"
          label="定制等级编码"
          placeholder="请输入定制等级编码"
        />
        <BaseInput
          v-model="form.levelName"
          label="定制等级名称"
          placeholder="请输入定制等级名称"
        />
        <BaseInput
          v-model="form.price"
          label="定制定价"
          placeholder="例如 12.50"
        />
        <BaseInput
          v-model="form.weightFactor"
          label="定制权重系数"
          placeholder="例如 1.20"
        />
        <BaseInput
          v-model="form.marketTier"
          label="市场层级"
          placeholder="可选，填写市场层级"
        />
        <BaseInput
          v-model="form.priceTier"
          label="价格层级"
          placeholder="可选，填写价格层级"
        />
        <BaseInput
          v-model="form.refPrice"
          label="参考单价"
          placeholder="可选，填写参考单价"
        />
        <BaseInput
          v-model="form.refInventory"
          label="参考库存"
          placeholder="可选，填写参考库存"
        />
        <BaseInput
          v-model="form.orderNo"
          label="订单编号"
          placeholder="可选，填写订单编号"
        />
      </div>
      <BaseTextarea
        v-model="form.note"
        label="审核说明"
        :rows="4"
        placeholder="补充本次定制审核说明"
      />
      <p v-if="localError" class="modal-error">{{ localError }}</p>
      <p v-if="error" class="modal-error">{{ error }}</p>
    </div>
    <template #footer>
      <footer class="modal-footer">
        <BaseButton
          variant="secondary"
          size="sm"
          :disabled="loading"
          @click="onCancel"
        >
          取消
        </BaseButton>
        <BaseButton
          size="sm"
          :loading="loading"
          :disabled="loading"
          @click="onSubmit"
        >
          确认提交
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { reactive, computed, ref, watch } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'

export type CustomizationReviewMode = 'initial' | 'effect'

/**
 * 组装后的提交 payload。字段命名与后端 openapi
 * (`customization/review` 与 `customization-jobs/:id/effect-review`)
 * 对齐，两个 endpoint 同形字段互通；`decision` 始终下发，
 * 作为前端已校验的硬约束体现在类型里（非 optional）。
 *
 * 追加字段 `market_tier / price_tier / ref_price / ref_inventory / order_no`
 * 对应 customization_reviews 子表的空位，后端按需消费、不破坏既有契约。
 */
export interface CustomizationReviewPayload {
  reviewer_id: number | string
  source_asset_id?: number | string | null
  customization_review_decision: string
  customization_level_code?: string
  customization_level_name?: string
  customization_price?: number | null
  customization_weight_factor?: number | null
  customization_note?: string
  market_tier?: string
  price_tier?: string
  ref_price?: number | null
  ref_inventory?: number | null
  order_no?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    mode?: CustomizationReviewMode
    defaultReviewerId?: string | number | null
    loading?: boolean
    error?: string
  }>(),
  {
    mode: 'initial',
    defaultReviewerId: null,
    loading: false,
    error: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [boolean]
  submit: [CustomizationReviewPayload]
  cancel: []
}>()

const form = reactive({
  reviewerId: '',
  sourceAssetId: '',
  decision: 'approved',
  levelCode: '',
  levelName: '',
  price: '',
  weightFactor: '',
  note: '',
  marketTier: '',
  priceTier: '',
  refPrice: '',
  refInventory: '',
  orderNo: '',
})

const localError = ref('')

const dialogTitle = computed(() =>
  props.mode === 'effect' ? '提交效果审核（定制二次审核）' : '提交定制审核',
)

const hintText = computed(() =>
  props.mode === 'effect'
    ? '本次为效果二次审核，审核结果仅记录意见，不会创建新的定制任务。'
    : '「退回设计师」属于成功分支，不会创建新的定制任务。',
)

function resetForm() {
  form.reviewerId = props.defaultReviewerId != null ? String(props.defaultReviewerId) : ''
  form.sourceAssetId = ''
  form.decision = 'approved'
  form.levelCode = ''
  form.levelName = ''
  form.price = ''
  form.weightFactor = ''
  form.note = ''
  form.marketTier = ''
  form.priceTier = ''
  form.refPrice = ''
  form.refInventory = ''
  form.orderNo = ''
  localError.value = ''
}

/** 打开弹窗时回填默认值；关闭弹窗时清表，避免下次复用残留数据。 */
watch(
  () => props.modelValue,
  (open) => {
    if (open) resetForm()
  },
  { immediate: true },
)

function toApiId(value: string): number | string | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  return /^\d+$/.test(trimmed) ? Number(trimmed) : trimmed
}

function toNullableApiId(value: string): number | string | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  return /^\d+$/.test(trimmed) ? Number(trimmed) : trimmed
}

function toNullableNumber(value: string): number | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}

function nonEmpty(value: string): string | undefined {
  const t = value.trim()
  return t ? t : undefined
}

function onSubmit() {
  if (props.loading) return
  localError.value = ''
  const reviewerId = toApiId(form.reviewerId)
  if (reviewerId == null) {
    localError.value = '请填写审核人 ID'
    return
  }
  const payload: CustomizationReviewPayload = {
    reviewer_id: reviewerId,
    customization_review_decision: form.decision,
    customization_level_code: nonEmpty(form.levelCode),
    customization_level_name: nonEmpty(form.levelName),
    customization_price: toNullableNumber(form.price),
    customization_weight_factor: toNullableNumber(form.weightFactor),
    customization_note: nonEmpty(form.note),
    market_tier: nonEmpty(form.marketTier),
    price_tier: nonEmpty(form.priceTier),
    ref_price: toNullableNumber(form.refPrice),
    ref_inventory: toNullableNumber(form.refInventory),
    order_no: nonEmpty(form.orderNo),
  }
  if (props.mode === 'initial') {
    payload.source_asset_id = toNullableApiId(form.sourceAssetId)
  }
  emit('submit', payload)
}

function onCancel() {
  emit('update:modelValue', false)
  emit('cancel')
}
</script>

<style scoped>
.customization-modal {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.customization-modal-hint {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
  color: #1e40af;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 6px;
  line-height: 1.45;
}
.customization-modal-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem 0.75rem;
}
@media (max-width: 640px) {
  .customization-modal-grid {
    grid-template-columns: 1fr;
  }
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}
.field-label {
  font-size: 0.75rem;
  color: #475569;
  font-weight: 500;
}
.reason-textarea {
  width: 100%;
  min-height: 2.25rem;
  padding: 0.35rem 0.5rem;
  font-size: 0.875rem;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: white;
  box-sizing: border-box;
}
.reason-textarea:focus {
  outline: none;
  border-color: #60a5fa;
  box-shadow: 0 0 0 2px rgb(96 165 250 / 0.25);
}
.modal-error {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.8125rem;
  color: #b91c1c;
  background: #fef2f2;
  border-radius: 6px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
