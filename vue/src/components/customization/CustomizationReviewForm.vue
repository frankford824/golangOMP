/**
 * 定制审核表单（共享弹窗）。
 *
 * 供两处入口共用：
 *   1. 任务详情 `AuditOutsourceBlock` 的「提交定制审核」按钮（mode=initial）。
 *   2. 审核工作台 `CustomizationReviewActionPanel` 的定制初审 / 二次效果审核入口。
 *
 * 组件职责：UI 门禁 + payload 组装；
 * 绝对禁止：推导 task_status、伪造成功。上传修改源文件仅走 canonical
 * upload-session，调用方接 `submit` 事件自行决定审核 endpoint：
 *   - `initial` → POST /v1/tasks/:id/customization/review
 *   - `effect`  → POST /v1/customization-jobs/:jobId/effect-review
 *
 * v7.0 规范一致：payload 字段保留既有契约（source_asset_id /
 * current_asset_id / customization_review_decision / customization_level_* / customization_price /
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
        <label class="field">
          <span class="field-label">审核决策</span>
          <select v-model="form.decision" class="reason-textarea">
            <option value="approved">通过</option>
            <option value="return_to_designer">打回美工处理</option>
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
      <div v-if="showSourceUpload" class="source-upload-box">
        <div class="source-upload-head">
          <span class="field-label">上传修改源文件</span>
          <input
            type="file"
            class="source-upload-input"
            :disabled="uploading || loading || disabled"
            @change="onSourceFileChange"
          />
        </div>
        <p v-if="uploading" class="source-upload-status">
          上传中 {{ uploadProgress }}%
        </p>
        <p v-else-if="uploadedSourceAssetId" class="source-upload-status">
          已上传：{{ uploadedFileName }}，资产 ID：<span class="asset-id">{{ uploadedSourceAssetId }}</span>
        </p>
        <p v-else class="source-upload-status">
          可选上传；上传成功后本次提交会绑定最新源文件。
        </p>
        <p v-if="uploadError" class="modal-error">{{ uploadError }}</p>
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
          :disabled="loading || uploading || disabled"
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
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'

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
  reviewer_id?: number | string
  source_asset_id?: number | string | null
  current_asset_id?: number | string | null
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
    taskId?: string | number | null
    loading?: boolean
    error?: string
    canUploadSource?: boolean
    targetSkuCode?: string | null
    disabled?: boolean
  }>(),
  {
    mode: 'initial',
    taskId: null,
    loading: false,
    error: '',
    canUploadSource: false,
    targetSkuCode: null,
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [boolean]
  submit: [CustomizationReviewPayload]
  cancel: []
}>()

const form = reactive({
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
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadedSourceAssetId = ref<number | string | null>(null)
const uploadedFileName = ref('')
const uploadError = ref('')
let uploadAbortController: AbortController | null = null

const dialogTitle = computed(() =>
  props.mode === 'effect' ? '提交效果审核（定制二次审核）' : '提交定制审核',
)

const hintText = computed(() =>
  props.mode === 'effect'
    ? '本次为效果二次审核，审核结果仅记录意见，不会创建新的定制任务。'
    : '打回美工处理后任务回到定制生产阶段，由美工继续改稿；通过后将进入仓库接收。',
)

const showSourceUpload = computed(() => props.canUploadSource && props.taskId != null)

function resetForm() {
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
  resetUploadState()
}

/** 打开弹窗时回填默认值；关闭弹窗时清表，避免下次复用残留数据。 */
watch(
  () => props.modelValue,
  (open) => {
    if (open) resetForm()
  },
  { immediate: true },
)

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

function resetUploadState() {
  uploadAbortController?.abort()
  uploadAbortController = null
  uploading.value = false
  uploadProgress.value = 0
  uploadedSourceAssetId.value = null
  uploadedFileName.value = ''
  uploadError.value = ''
}

function extractUploadedAssetId(uploaded: unknown): number | string | null {
  const roots = [
    uploaded,
    (uploaded as { asset?: unknown })?.asset,
    (uploaded as { version?: unknown })?.version,
    (uploaded as { session?: unknown })?.session,
  ]
  for (const root of roots) {
    if (!root || typeof root !== 'object') continue
    const r = root as Record<string, unknown>
    const raw = r.asset_id ?? r.assetId ?? r.id
    if (typeof raw === 'number' && Number.isFinite(raw)) return raw
    if (typeof raw === 'string' && raw.trim()) return raw.trim()
  }
  return null
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error && err.message.trim()) return err.message
  return fallback
}

async function onSourceFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (input) input.value = ''
  if (!file || !showSourceUpload.value || uploading.value || props.loading || props.disabled) return
  uploading.value = true
  uploadProgress.value = 0
  uploadError.value = ''
  try {
    const taskId = String(props.taskId ?? '').trim()
    if (!taskId) throw new Error('缺少任务 ID，无法上传源文件')
    uploadAbortController = new AbortController()
    const uploaded = await uploadTaskFileViaAssetSession(
      taskId,
      file,
      {
        asset_kind: 'source',
        owner_module_key: 'customization',
        target_sku_code: props.targetSkuCode?.trim() || undefined,
        remark: `定制审核源文件：${file.name}`,
      },
      {
        signal: uploadAbortController.signal,
        onProgress: (progress) => {
          uploadProgress.value = Math.max(0, Math.min(100, Math.round(progress.percent ?? 0)))
        },
      },
    )
    const assetId = extractUploadedAssetId(uploaded)
    if (assetId == null) throw new Error('上传成功但未返回资产 ID')
    uploadedSourceAssetId.value = assetId
    uploadedFileName.value = file.name
    uploadProgress.value = 100
  } catch (err) {
    uploadedSourceAssetId.value = null
    uploadedFileName.value = ''
    uploadError.value = errorMessage(err, '源文件上传失败')
  } finally {
    uploading.value = false
    uploadAbortController = null
  }
}

function onSubmit() {
  if (props.loading || uploading.value || props.disabled) return
  localError.value = ''
  const payload: CustomizationReviewPayload = {
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
  if (uploadedSourceAssetId.value != null) {
    if (props.mode === 'effect') {
      payload.current_asset_id = uploadedSourceAssetId.value
    } else {
      payload.source_asset_id = uploadedSourceAssetId.value
    }
  }
  emit('submit', payload)
}

function onCancel() {
  uploadAbortController?.abort()
  uploadAbortController = null
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
  color: rgb(var(--yb-brand-deep));
  background: rgb(var(--yb-brand-soft));
  border: 1px solid rgb(var(--yb-brand-border));
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
  color: rgb(var(--yb-text-soft));
  font-weight: 500;
}
.reason-textarea {
  width: 100%;
  min-height: 2.25rem;
  padding: 0.35rem 0.5rem;
  font-size: 0.875rem;
  border: 1px solid rgb(var(--yb-text-disabled));
  border-radius: 6px;
  background: white;
  box-sizing: border-box;
}
.reason-textarea:focus {
  outline: none;
  border-color: rgb(var(--yb-brand-accent));
  box-shadow: 0 0 0 2px rgb(var(--yb-brand-accent) / 0.25);
}
.source-upload-box {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 6px;
  background: rgb(var(--yb-surface-soft));
}
.source-upload-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.source-upload-input {
  max-width: 100%;
  font-size: 0.8125rem;
}
.source-upload-status {
  margin: 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  line-height: 1.45;
}
.asset-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
}
.modal-error {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.8125rem;
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
  border-radius: 6px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
