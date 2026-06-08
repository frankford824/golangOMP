<template>
  <BaseModal
    :model-value="modelValue"
    :title="isBatchTask ? '编辑母任务信息' : '编辑任务信息'"
    :show-confirm="false"
    cancel-text="关闭"
    panel-class="max-w-[min(1060px,96vw)] !max-h-[94vh] task-info-edit-modal-panel"
    @update:model-value="onClose"
  >
    <div v-if="loadError" class="edit-error-banner">
      {{ loadError }}
    </div>
    <div v-if="submitError" class="edit-error-banner">
      {{ submitError }}
    </div>
    <div v-if="loading" class="edit-loading-text">加载可编辑数据…</div>
    <div v-else class="edit-workspace">
      <section v-if="!isBatchTask" class="form-card">
        <p class="section-eyebrow">商品信息</p>
        <div class="form-grid">
          <BaseInput
            v-model="form.product_name"
            label="产品名称"
            placeholder="与创建侧一致"
            :maxlength="ERP_PRODUCT_NAME_MAX_LENGTH"
            :hint="erpProductNameHint(form.product_name)"
          />
          <IIdSelector
            v-if="showField.i_id"
            v-model="iIdModel"
            label="款式编码"
            placeholder="搜索或选择款式编码"
          />
          <BaseInput v-if="showField.material" v-model="form.material" label="材质" placeholder="可选" />
          <BaseTextarea
            v-if="showField.spec_text"
            v-model="form.spec_text"
            label="规格说明"
            :rows="2"
            placeholder="规格、工艺等"
          />
          <BaseTextarea
            v-if="showField.size_text"
            v-model="form.size_text"
            label="尺寸"
            :rows="2"
            placeholder="尺寸描述"
          />
          <BaseInput
            v-if="showField.reference_link"
            v-model="form.reference_link"
            label="参考链接"
            placeholder="可选"
            class="sm:col-span-2"
          />
        </div>
        <label v-if="showField.trigger_filing" class="field-checkbox-label mt-3">
          <input v-model="form.trigger_filing" type="checkbox" class="native-checkbox" />
          <span>保存商品信息时强制触发一次 ERP 建档同步</span>
        </label>
      </section>

      <section class="form-card">
        <p class="section-eyebrow">任务基础</p>
        <div class="form-grid">
          <BaseTextarea
            v-if="showField.design_requirement"
            v-model="form.design_requirement"
            class="sm:col-span-2"
            label="设计 / 修改需求"
            :rows="3"
          />
          <BaseTextarea v-model="form.note" class="sm:col-span-2" label="运营备注" :rows="2" />
          <div>
            <label class="field-label">截止时间</label>
            <input v-model="form.due_at" type="date" class="native-input" />
          </div>
          <div>
            <label class="field-label">优先级</label>
            <select v-model="form.priority" class="native-input">
              <option value="low">低</option>
              <option value="normal">普通</option>
              <option value="high">高</option>
              <option value="critical">加急</option>
            </select>
          </div>
        </div>
      </section>

      <section class="form-card">
        <p class="section-eyebrow">成本</p>
        <div class="form-grid">
          <BaseInput
            v-model.number="form.cost_price"
            type="number"
            min="0"
            step="0.001"
            label="成本单价（CNY）"
            placeholder="请输入成本单价，可不填"
          />
          <label class="field-checkbox-label self-end pb-1">
            <input v-model="form.manual_cost_override" type="checkbox" class="native-checkbox" />
            <span>手动指定成本</span>
          </label>
          <BaseInput
            v-model="form.manual_cost_override_reason"
            class="sm:col-span-2"
            label="覆盖原因"
            placeholder="如：仓库维护成本价、ERP 同步前修正"
          />
        </div>
        <p class="section-hint section-hint--after-grid">
          系统预估成本仅作参考；保存成本单价后将按人工维护成本处理，并请求同步 ERP。
        </p>
      </section>

      <section v-if="isPurchaseTask" class="form-card">
        <p class="section-eyebrow">采购</p>
        <p class="section-hint">仅采购任务会提交到 PATCH procurement；请与 OpenAPI 状态枚举保持一致。</p>
        <div class="form-grid">
          <div>
            <label class="field-label">采购状态（API）</label>
            <select v-model="form.procurement_status" class="native-input">
              <option value="draft">draft</option>
              <option value="prepared">prepared</option>
              <option value="in_progress">in_progress</option>
              <option value="completed">completed</option>
            </select>
          </div>
          <BaseInput
            v-model.number="form.procurement_price"
            type="number"
            min="0"
            step="0.001"
            label="采购单价"
            placeholder="可选"
          />
          <BaseInput
            v-model.number="form.procurement_quantity"
            type="number"
            min="0"
            step="1"
            label="采购数量"
            placeholder="可选"
          />
          <BaseInput v-model="form.supplier_name" label="供应商" placeholder="可选" />
          <BaseTextarea v-model="form.purchase_remark" class="sm:col-span-2" label="采购备注" :rows="2" />
          <div>
            <label class="field-label">预计到货日</label>
            <input v-model="form.expected_delivery_date" type="date" class="native-input" />
          </div>
        </div>
      </section>

      <section class="form-card">
        <p class="section-eyebrow">保存说明</p>
        <BaseTextarea
          v-model="form.save_remark"
          :rows="2"
          label="备注（如有特殊情况，可填写备注说明）"
        />
      </section>
    </div>

    <template #footer>
      <footer class="edit-modal-footer">
        <BaseButton variant="secondary" size="sm" :disabled="saving || loading" @click="onClose(false)">
          取消
        </BaseButton>
        <BaseButton variant="primary" size="sm" :loading="saving" :disabled="saving || loading" @click="submit">
          保存全部变更
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Task } from '@/domain/types/task'
import { tasksApi } from '@/services/api/tasksApi'
import type { BusinessInfoPatch } from '@/services/apiTypes'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import { normalizePriorityForApi } from '@/domain/task-priority'
import {
  ERP_PRODUCT_NAME_MAX_LENGTH,
  erpProductNameHint,
  erpProductNameLimitMessage,
  isErpProductNameTooLong,
} from '@/domain/erp-product-name'
import { taskBeijingDateKey, toBeijingEndOfDayISO } from '@/utils/date'

const props = defineProps<{
  modelValue: boolean
  task: Task
  productIndex?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  saved: []
}>()

const isBatchTask = computed(() => props.task.isBatchTask === true)
const isPurchaseTask = computed(() => isPurchaseTaskType(props.task))
const taskKind = computed(() => props.task.businessType ?? props.task.taskType)
const showField = computed(() => {
  const k = taskKind.value
  return {
    i_id: k === 'NEW_PRODUCT_DEV' || k === 'PURCHASE_TASK',
    category_code: false,
    material: false,
    spec_text: k !== 'RETOUCH_TASK',
    size_text: k !== 'RETOUCH_TASK',
    reference_link: k === 'NEW_PRODUCT_DEV',
    trigger_filing: k !== 'RETOUCH_TASK',
    design_requirement: k !== 'PURCHASE_TASK',
  }
})

type EditForm = {
  product_name: string
  i_id: string
  category_code: string
  spec_text: string
  size_text: string
  material: string
  reference_link: string
  trigger_filing: boolean
  design_requirement: string
  note: string
  due_at: string
  priority: string
  cost_price: number | undefined
  manual_cost_override: boolean
  manual_cost_override_reason: string
  procurement_status: string
  procurement_price: number | undefined
  procurement_quantity: number | undefined
  supplier_name: string
  purchase_remark: string
  expected_delivery_date: string
  save_remark: string
}

function emptyForm(): EditForm {
  return {
    product_name: '',
    i_id: '',
    category_code: '',
    spec_text: '',
    size_text: '',
    material: '',
    reference_link: '',
    trigger_filing: false,
    design_requirement: '',
    note: '',
    due_at: '',
    priority: 'normal',
    cost_price: undefined,
    manual_cost_override: false,
    manual_cost_override_reason: '',
    procurement_status: 'draft',
    procurement_price: undefined,
    procurement_quantity: undefined,
    supplier_name: '',
    purchase_remark: '',
    expected_delivery_date: '',
    save_remark: '',
  }
}

const loading = ref(false)
const loadError = ref('')
const submitError = ref('')
const saving = ref(false)
const form = ref<EditForm>(emptyForm())
const baseline = ref<EditForm>(emptyForm())
const iIdModel = computed({
  get: () => form.value.i_id ?? '',
  set: (value: string | undefined) => {
    form.value.i_id = String(value ?? '').trim()
  },
})

function extractEnvelopeData(res: unknown): Record<string, unknown> | null {
  if (res == null || typeof res !== 'object') return null
  const r = res as Record<string, unknown>
  const data = r.data
  if (data && typeof data === 'object') return data as Record<string, unknown>
  return null
}

function activeSku(t: Task) {
  const idx = Math.max(0, props.productIndex ?? 0)
  return t.skuItems?.[idx] ?? null
}

function procurementApiStatusFromTask(t: Task): string {
  if (t.procurementApiStatus?.trim()) return t.procurementApiStatus.trim()
  const st = t.purchaseInfo?.status
  if (st === 'Purchased') return 'completed'
  return 'draft'
}

function isPurchaseTaskType(t: Task): boolean {
  return t.businessType === 'PURCHASE_TASK' || t.taskType === 'PURCHASE_TASK'
}

function hydrateFromTaskAndPanels(
  t: Task,
  productPanel: Record<string, unknown> | null,
  costPanel: Record<string, unknown> | null,
) {
  const sku = activeSku(t)
  const next = emptyForm()
  next.product_name = String(
    productPanel?.product_name ??
      productPanel?.product_name_snapshot ??
      sku?.productNameSnapshot ??
      t.productName ??
      '',
  ).trim()
  next.i_id = String(productPanel?.i_id ?? productPanel?.product_i_id ?? t.erpIId ?? '').trim()
  next.category_code = String(
    productPanel?.category_code ?? t.newProductCategoryCode ?? t.erpCategoryCode ?? t.category ?? '',
  ).trim()
  next.spec_text = String(productPanel?.spec_text ?? t.specText ?? '').trim()
  next.size_text = String(productPanel?.size_text ?? t.sizeText ?? '').trim()
  next.material = String(productPanel?.material ?? t.newProductMaterial ?? '').trim()
  next.reference_link = String(productPanel?.reference_link ?? t.productReferenceUrl ?? '').trim()

  next.design_requirement = String(
    isBatchTask.value ? t.designRequirement ?? '' : sku?.designRequirement ?? t.designRequirement ?? '',
  ).trim()
  next.note = String(t.note ?? '').trim()
  next.due_at = taskBeijingDateKey(t.dueAt)
  next.priority = String(t.priority ?? 'normal')

  const costFromPanel = costPanel?.cost_price
  const costNum =
    typeof costFromPanel === 'number' && Number.isFinite(costFromPanel)
      ? costFromPanel
      : t.costPrice?.amount ?? t.newProductCostUnitPrice
  next.cost_price = typeof costNum === 'number' && Number.isFinite(costNum) ? costNum : undefined
  next.manual_cost_override = Boolean(costPanel?.manual_cost_override ?? false)
  next.manual_cost_override_reason = String(costPanel?.manual_cost_override_reason ?? '').trim()

  next.procurement_status = procurementApiStatusFromTask(t)
  const pq = t.purchaseInfo?.quantity
  next.procurement_quantity = typeof pq === 'number' && Number.isFinite(pq) ? pq : undefined
  const pp = t.purchaseInfo?.purchasePrice?.amount
  next.procurement_price = typeof pp === 'number' && Number.isFinite(pp) ? pp : undefined
  next.supplier_name = String(t.purchaseInfo?.supplierName ?? '').trim()
  next.purchase_remark = String(t.purchaseInfo?.note ?? '').trim()
  next.expected_delivery_date = taskBeijingDateKey(t.purchaseInfo?.expectedArrivalAt ?? null)

  return next
}

async function loadPanels() {
  const t = props.task
  loadError.value = ''
  loading.value = true
  try {
    const [pi, ci] = await Promise.all([
      tasksApi.getProductInfo(t.id).catch(() => null),
      tasksApi.getCostInfo(t.id).catch(() => null),
    ])
    const productPanel = pi ? extractEnvelopeData((pi as { data?: unknown }).data) : null
    const costPanel = ci ? extractEnvelopeData((ci as { data?: unknown }).data) : null
    const filled = hydrateFromTaskAndPanels(t, productPanel, costPanel)
    baseline.value = { ...filled, trigger_filing: false, save_remark: '' }
    form.value = { ...filled, trigger_filing: false, save_remark: '' }
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
    const filled = hydrateFromTaskAndPanels(t, null, null)
    baseline.value = { ...filled, trigger_filing: false, save_remark: '' }
    form.value = { ...filled, trigger_filing: false, save_remark: '' }
  } finally {
    loading.value = false
  }
}

function onClose(v: boolean) {
  emit('update:modelValue', v)
}

function normStr(s: string) {
  return String(s ?? '').trim()
}

function optionalRemark() {
  const r = normStr(form.value.save_remark)
  return r || undefined
}

function isOriginalProductTask(): boolean {
  const k = taskKind.value
  return k === 'ORIGINAL_PRODUCT_DEV'
}

function buildProductPatch(b: EditForm, c: EditForm): Record<string, unknown> | null {
  const patch: Record<string, unknown> = {}
  const remark = optionalRemark()

  // note 走 product-info（OpenAPI 确认 product-info PATCH 接受 note）
  if (normStr(c.note) !== normStr(b.note)) {
    patch.note = normStr(c.note)
  }

  // design_requirement / change_request 同样走 product-info（与 note 共享 task_details 底表）
  if (showField.value.design_requirement && normStr(c.design_requirement) !== normStr(b.design_requirement)) {
    const fieldKey = isOriginalProductTask() ? 'change_request' : 'design_requirement'
    patch[fieldKey] = normStr(c.design_requirement)
  }

  if (!isBatchTask.value) {
    if (normStr(c.product_name) !== normStr(b.product_name)) {
      patch.product_name = normStr(c.product_name) || null
    }
    if (showField.value.i_id && normStr(c.i_id) !== normStr(b.i_id)) {
      patch.i_id = normStr(c.i_id) || null
    }
    if (showField.value.category_code && normStr(c.category_code) !== normStr(b.category_code)) {
      patch.category_code = normStr(c.category_code) || null
    }
    if (showField.value.spec_text && normStr(c.spec_text) !== normStr(b.spec_text)) {
      patch.spec_text = normStr(c.spec_text) || null
    }
    if (showField.value.size_text && normStr(c.size_text) !== normStr(b.size_text)) {
      patch.size_text = normStr(c.size_text) || null
    }
    if (showField.value.material && normStr(c.material) !== normStr(b.material)) {
      patch.material = normStr(c.material) || null
    }
    if (showField.value.reference_link && normStr(c.reference_link) !== normStr(b.reference_link)) {
      patch.reference_link = normStr(c.reference_link) || null
    }
    if (showField.value.trigger_filing && c.trigger_filing) {
      patch.trigger_filing = true
    }
  }

  const touched = Object.keys(patch).length > 0
  if (touched && remark) patch.remark = remark
  return touched ? patch : null
}

function buildBusinessPatch(b: EditForm, c: EditForm): Record<string, unknown> | null {
  const patch: Record<string, unknown> = {}
  const remark = optionalRemark()
  // design_requirement / note 已移至 buildProductPatch → product-info
  const dueIso = c.due_at ? toBeijingEndOfDayISO(c.due_at) : null
  const baseDue = b.due_at ? toBeijingEndOfDayISO(b.due_at) : null
  if (dueIso !== baseDue) {
    patch.deadline_at = dueIso
  }
  const pr = normalizePriorityForApi(c.priority)
  const prB = normalizePriorityForApi(b.priority)
  if (pr !== prB) {
    patch.priority = pr
  }
  const touched = Object.keys(patch).length > 0
  if (touched && remark) patch.remark = remark
  return touched ? patch : null
}

function buildCostPatch(b: EditForm, c: EditForm): Record<string, unknown> | null {
  const patch: Record<string, unknown> = {}
  const remark = optionalRemark()
  const bc = b.cost_price
  const cc = c.cost_price
  const reason = normStr(c.manual_cost_override_reason)
  if (cc !== bc) {
    patch.cost_price = cc == null || Number.isNaN(cc) ? null : cc
    patch.manual_cost_override = true
    patch.manual_cost_override_reason = reason || '仓库/运营手动维护成本'
    patch.trigger_filing = true
  } else if (Boolean(c.manual_cost_override) !== Boolean(b.manual_cost_override)) {
    patch.manual_cost_override = c.manual_cost_override
  }
  if (reason !== normStr(b.manual_cost_override_reason) && !('manual_cost_override_reason' in patch)) {
    patch.manual_cost_override_reason = reason || null
  }
  const touched = Object.keys(patch).length > 0
  if (touched && remark) patch.remark = remark
  return touched ? patch : null
}

function buildProcurementPatch(b: EditForm, c: EditForm): Record<string, unknown> | null {
  const patch: Record<string, unknown> = {}
  const remark = optionalRemark()
  if (normStr(c.procurement_status) !== normStr(b.procurement_status)) {
    patch.status = normStr(c.procurement_status) || 'draft'
  }
  const bp = b.procurement_price
  const cp = c.procurement_price
  if (cp !== bp) {
    if (typeof cp === 'number' && Number.isFinite(cp)) {
      patch.procurement_price = cp
    }
  }
  const bq = b.procurement_quantity
  const cq = c.procurement_quantity
  if (cq !== bq) {
    if (typeof cq === 'number' && Number.isFinite(cq)) {
      patch.quantity = Math.trunc(cq)
    }
  }
  if (normStr(c.supplier_name) !== normStr(b.supplier_name)) {
    patch.supplier_name = normStr(c.supplier_name) || null
  }
  if (normStr(c.purchase_remark) !== normStr(b.purchase_remark)) {
    patch.purchase_remark = normStr(c.purchase_remark) || null
  }
  const bd = normStr(b.expected_delivery_date)
  const cd = normStr(c.expected_delivery_date)
  if (cd !== bd) {
    patch.expected_delivery_at = cd ? toBeijingEndOfDayISO(cd) : null
  }
  if (Object.keys(patch).length === 0) return null
  if (remark) patch.remark = remark
  return patch
}

async function submit() {
  const t = props.task
  submitError.value = ''
  saving.value = true
  const b = baseline.value
  const c = form.value
  const errors: string[] = []
  if (!isBatchTask.value && isErpProductNameTooLong(c.product_name)) {
    submitError.value = erpProductNameLimitMessage('产品名称')
    saving.value = false
    return
  }
  try {
    const productPatch = buildProductPatch(b, c)
    const businessPatch = buildBusinessPatch(b, c)
    const costPatch = buildCostPatch(b, c)
    const procurementPatch =
      isPurchaseTaskType(t) ? buildProcurementPatch(b, c) : null

    if (!productPatch && !businessPatch && !costPatch && !procurementPatch) {
      submitError.value = '没有检测到变更'
      return
    }

    if (productPatch) {
      try {
        await tasksApi.patchProductInfo(t.id, productPatch)
      } catch (e) {
        errors.push(`商品信息：${e instanceof Error ? e.message : '保存失败'}`)
      }
    }
    if (businessPatch) {
      try {
        await tasksApi.patchBusinessInfo(t.id, businessPatch as BusinessInfoPatch)
      } catch (e) {
        errors.push(`业务信息：${e instanceof Error ? e.message : '保存失败'}`)
      }
    }
    if (costPatch) {
      try {
        await tasksApi.patchCostInfo(t.id, costPatch)
      } catch (e) {
        errors.push(`成本：${e instanceof Error ? e.message : '保存失败'}`)
      }
    }
    if (procurementPatch) {
      try {
        await tasksApi.patchTaskProcurement(t.id, procurementPatch)
      } catch (e) {
        errors.push(`采购：${e instanceof Error ? e.message : '保存失败'}`)
      }
    }

    if (errors.length) {
      submitError.value = errors.join('；')
      return
    }

    emit('saved')
    emit('update:modelValue', false)
  } finally {
    saving.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      submitError.value = ''
      void loadPanels()
    }
  },
)
</script>

<style scoped>
.edit-workspace {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding-bottom: 0.35rem;
}

.form-card {
  border: 1px solid #e5e7eb;
  border-radius: 0.875rem;
  background: #ffffff;
  padding: 0.875rem 1rem;
  color: #111827;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
}

.section-eyebrow {
  margin: 0 0 0.65rem;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #6b7280;
}

.section-hint {
  margin: 0.35rem 0 0.65rem;
  font-size: 0.75rem;
  color: #6b7280;
  line-height: 1.45;
}

.section-hint--after-grid {
  margin-top: 0.65rem;
  margin-bottom: 0;
}

.form-grid {
  display: grid;
  gap: 0.65rem 0.875rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

@media (max-width: 720px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}

.field-label {
  display: block;
  margin-bottom: 0.35rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #374151;
  letter-spacing: 0.01em;
}

.field-checkbox-label {
  display: flex;
  cursor: pointer;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
  color: #374151;
  line-height: 1.4;
}

.native-input {
  width: 100%;
  min-height: 2.75rem;
  border: 1px solid #d1d5db;
  border-radius: 0.75rem;
  background: #ffffff;
  color: #111827;
  font-size: 0.875rem;
  padding: 0.45rem 0.65rem;
  outline: none;
  color-scheme: light;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.native-input::placeholder {
  color: #9ca3af;
}

.native-input:focus {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

.native-input:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  background: #f9fafb;
}

select.native-input {
  cursor: pointer;
  appearance: none;
  background-image: linear-gradient(45deg, transparent 50%, #6b7280 50%),
    linear-gradient(135deg, #6b7280 50%, transparent 50%);
  background-position:
    calc(100% - 1.1rem) calc(50% + 0.12rem),
    calc(100% - 0.75rem) calc(50% + 0.12rem);
  background-size:
    0.35rem 0.35rem,
    0.35rem 0.35rem;
  background-repeat: no-repeat;
  padding-right: 2rem;
}

input.native-input[type='date']::-webkit-calendar-picker-indicator {
  cursor: pointer;
  opacity: 0.75;
}

.native-checkbox {
  width: 1rem;
  height: 1rem;
  flex-shrink: 0;
  margin: 0;
  border-radius: 0.25rem;
  accent-color: #2563eb;
  cursor: pointer;
}

.edit-error-banner {
  margin-bottom: 0.65rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid #fecaca;
  border-radius: 0.5rem;
  background: #fef2f2;
  font-size: 0.875rem;
  color: #b91c1c;
  line-height: 1.45;
}

.edit-loading-text {
  padding: 2rem 0;
  text-align: center;
  font-size: 0.875rem;
  color: #6b7280;
}

.edit-modal-footer {
  display: flex;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
  padding: 1rem 1.25rem;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
}

/* BaseInput / BaseTextarea / IIdSelector（BaseSelect）局部浅色覆写 */
.edit-workspace :deep(label.text-sm.font-medium.text-slate-600),
.edit-workspace :deep(label.field-label) {
  color: #374151 !important;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.form-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}

.form-card :deep(input),
.form-card :deep(textarea) {
  border-color: #d1d5db !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: none !important;
}

.form-card :deep(input) {
  min-height: 2.75rem;
  border-radius: 0.75rem !important;
}

.form-card :deep(textarea) {
  border-radius: 0.75rem !important;
  resize: vertical;
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: #9ca3af !important;
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12) !important;
}

.form-card :deep(input:disabled),
.form-card :deep(textarea:disabled) {
  cursor: not-allowed;
  opacity: 0.55;
  background: #f9fafb !important;
}

.form-card :deep(.relative > div) {
  min-height: 2.75rem;
  border-color: #d1d5db !important;
  border-radius: 0.75rem !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: none !important;
}

.form-card :deep(.relative > div:focus-within) {
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12) !important;
}

.form-card :deep(.relative button) {
  color: #111827 !important;
}

.form-card :deep(.relative .text-slate-500) {
  color: #6b7280 !important;
}

.form-card :deep(.text-xs.text-slate-400) {
  color: #6b7280 !important;
}

/* 浅色弹窗外壳（对齐 BaseModal / 阶段一全局壳） */
:global(.task-info-edit-modal-panel) {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: 0 10px 40px rgba(15, 23, 42, 0.12) !important;
}

:global(.task-info-edit-modal-panel > header) {
  border-bottom: 1px solid #e5e7eb;
  background: #ffffff;
}

:global(.task-info-edit-modal-panel > header h2) {
  color: #111827 !important;
}

:global(.task-info-edit-modal-panel > header button) {
  color: #6b7280 !important;
}

:global(.task-info-edit-modal-panel > header button:hover) {
  color: #111827 !important;
}

:global(.task-info-edit-modal-panel > div.flex-1) {
  color: #111827 !important;
}
</style>
