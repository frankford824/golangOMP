<template>
  <BaseModal
    :model-value="modelValue"
    :title="isBatchTask ? '编辑母任务信息' : '编辑任务信息'"
    :show-confirm="false"
    cancel-text="关闭"
    panel-class="max-w-[min(1060px,96vw)] task-info-edit-modal-panel"
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
            :error="erpProductNameError(form.product_name)"
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
            label="规格 / 工艺补充"
            :rows="2"
            placeholder="除标准尺寸外的规格、工艺等补充说明"
          />
          <TaskSpecStructuredInput
            v-if="showField.size_text"
            v-model="form.size_text"
            class="sm:col-span-2"
            label="尺寸"
            placeholder="请选择宽高或面积并填写数字"
            hint="与创建任务保持一致；修改后会参与成本规则重新匹配。"
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

      <section v-else class="form-card">
        <p class="section-eyebrow">母任务信息</p>
        <div class="form-grid">
          <BaseInput
            v-model="form.product_name"
            label="任务名称"
            placeholder="例如：升学宴海报批量任务"
            hint="用于识别这一批任务，不会覆盖每个 SKU 的产品名称"
            class="sm:col-span-2"
          />
        </div>
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
            <div class="due-at-input-row">
              <input v-model="form.due_at" type="date" class="native-input" aria-label="任务截止日期" />
              <select v-model="form.due_at_hour" class="native-input due-hour-select">
                <option v-for="opt in dueHourOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </div>
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
            <input v-model="form.expected_delivery_date" type="date" class="native-input" aria-label="预计到货日" />
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
import TaskSpecStructuredInput from '@/components/task/TaskSpecStructuredInput.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import { normalizePriorityForApi } from '@/domain/task-priority'
import {
  ERP_PRODUCT_NAME_MAX_LENGTH,
  erpProductNameError,
  erpProductNameHint,
  erpProductNameLimitMessage,
  isErpProductNameTooLong,
} from '@/domain/erp-product-name'
import { taskBeijingDateKey, taskBeijingHour, toBeijingEndOfDayISO, toBeijingHourISO } from '@/utils/date'

const props = defineProps<{
  modelValue: boolean
  task: Task
  productIndex?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  saved: [Partial<Task>?]
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
  due_at_hour: string
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
    due_at_hour: '18',
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
const dueHourFallback = 18
const dueHourOptions = Array.from({ length: 24 }, (_, hour) => ({
  value: String(hour),
  label: `${String(hour).padStart(2, '0')}:00`,
}))
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
  const panelProductName = productPanel?.product_name ?? productPanel?.product_name_snapshot
  next.product_name = String(
    isBatchTask.value
      ? panelProductName ?? t.productName ?? ''
      : panelProductName ?? sku?.productNameSnapshot ?? t.productName ?? '',
  ).trim()
  next.i_id = String(productPanel?.i_id ?? productPanel?.product_i_id ?? t.erpIId ?? '').trim()
  next.category_code = String(
    productPanel?.category_code ?? t.newProductCategoryCode ?? t.erpCategoryCode ?? t.category ?? '',
  ).trim()
  const specText = String(productPanel?.spec_text ?? t.specText ?? '').trim()
  const sizeText = String(productPanel?.size_text ?? t.sizeText ?? '').trim()
  next.spec_text = specText
  next.size_text = sizeText || (isStructuredDimensionText(specText) ? specText : '')
  next.material = String(productPanel?.material ?? t.newProductMaterial ?? '').trim()
  next.reference_link = String(productPanel?.reference_link ?? t.productReferenceUrl ?? '').trim()

  next.design_requirement = String(
    isBatchTask.value ? t.designRequirement ?? '' : sku?.designRequirement ?? t.designRequirement ?? '',
  ).trim()
  next.note = String(t.note ?? '').trim()
  next.due_at = taskBeijingDateKey(t.dueAt)
  next.due_at_hour = String(taskBeijingHour(t.dueAt) ?? dueHourFallback)
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

  if (normStr(c.product_name) !== normStr(b.product_name)) {
    patch.product_name = normStr(c.product_name) || null
  }

  if (!isBatchTask.value) {
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
      if (shouldMirrorEditedSizeToSpec(b, c)) {
        patch.spec_text = normStr(c.size_text) || null
      }
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

function isStructuredDimensionText(value: string): boolean {
  const text = normStr(value)
  if (!text) return false
  return (
    /^\d+(?:\.\d+)?\s*[x×*]\s*\d+(?:\.\d+)?\s*(?:cm|m|mm|厘米|米|毫米)$/i.test(text) ||
    /^\d+(?:\.\d+)?\s*(?:平方米|平方|平米|㎡|m2|m²|平方厘米|cm2|cm²|平方毫米|mm2|mm²)$/i.test(text)
  )
}

function shouldMirrorEditedSizeToSpec(b: EditForm, c: EditForm): boolean {
  const baseSpec = normStr(b.spec_text)
  if (!showField.value.spec_text || !isStructuredDimensionText(baseSpec)) return false
  if (normStr(c.spec_text) !== baseSpec) return false
  const baseSize = normStr(b.size_text)
  return baseSize === '' || baseSize === baseSpec
}

function buildBusinessPatch(b: EditForm, c: EditForm): Record<string, unknown> | null {
  const patch: Record<string, unknown> = {}
  const remark = optionalRemark()
  // design_requirement / note 已移至 buildProductPatch → product-info
  const dueIso = c.due_at ? toBeijingHourISO(c.due_at, parseDueHour(c.due_at_hour)) : null
  const baseDue = b.due_at ? toBeijingHourISO(b.due_at, parseDueHour(b.due_at_hour)) : null
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

function buildOptimisticTaskPatch(
  productPatch: Record<string, unknown> | null,
  businessPatch: Record<string, unknown> | null,
): Partial<Task> | undefined {
  const patch: Partial<Task> = {}
  if (productPatch && 'product_name' in productPatch) {
    patch.productName = String(productPatch.product_name ?? '').trim()
  }
  if (businessPatch && 'deadline_at' in businessPatch) {
    patch.dueAt = (businessPatch.deadline_at as string | null) ?? null
  }
  if (businessPatch && 'priority' in businessPatch) {
    patch.priority = normalizePriorityForApi(String(businessPatch.priority ?? 'normal')) as Task['priority']
  }
  return Object.keys(patch).length > 0 ? patch : undefined
}

function parseDueHour(value: string): number {
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed >= 0 && parsed <= 23 ? parsed : dueHourFallback
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
  if (isBatchTask.value && !normStr(c.product_name)) {
    submitError.value = '任务名称不能为空'
    saving.value = false
    return
  }
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

    emit('saved', buildOptimisticTaskPatch(productPatch, businessPatch))
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
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.875rem;
  background: rgb(var(--yb-surface));
  padding: 0.875rem 1rem;
  color: rgb(var(--yb-text));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.section-eyebrow {
  margin: 0 0 0.65rem;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgb(var(--yb-text-muted));
}

.section-hint {
  margin: 0.35rem 0 0.65rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted));
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
  color: rgb(var(--yb-text-body));
  letter-spacing: 0.01em;
}

.field-checkbox-label {
  display: flex;
  cursor: pointer;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-body));
  line-height: 1.4;
}

.native-input {
  width: 100%;
  min-height: 2.75rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  font-size: 0.875rem;
  padding: 0.45rem 0.65rem;
  outline: none;
  color-scheme: light;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.native-input::placeholder {
  color: rgb(var(--yb-text-faint));
}

.native-input:focus {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.native-input:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  background: rgb(var(--yb-surface-soft));
}

select.native-input {
  cursor: pointer;
  appearance: none;
  background-image: linear-gradient(45deg, transparent 50%, rgb(var(--yb-text-muted)) 50%),
    linear-gradient(135deg, rgb(var(--yb-text-muted)) 50%, transparent 50%);
  background-position:
    calc(100% - 1.1rem) calc(50% + 0.12rem),
    calc(100% - 0.75rem) calc(50% + 0.12rem);
  background-size:
    0.35rem 0.35rem,
    0.35rem 0.35rem;
  background-repeat: no-repeat;
  padding-right: 2rem;
}

.due-at-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 7rem;
  gap: 0.5rem;
  align-items: center;
}

.due-hour-select {
  min-width: 0;
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
  accent-color: rgb(var(--yb-brand));
  cursor: pointer;
}

.edit-error-banner {
  margin-bottom: 0.65rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgb(var(--yb-danger-border));
  border-radius: 0.5rem;
  background: rgb(var(--yb-danger-soft));
  font-size: 0.875rem;
  color: rgb(var(--yb-danger-text));
  line-height: 1.45;
}

.edit-loading-text {
  padding: 2rem 0;
  text-align: center;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-muted));
}

.edit-modal-footer {
  display: flex;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
  padding: 1rem 1.25rem;
  border-top: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

/* BaseInput / BaseTextarea / IIdSelector（BaseSelect）局部浅色覆写 */
.edit-workspace :deep(label.field-label) {
  color: rgb(var(--yb-text-body));
  font-weight: 600;
  letter-spacing: 0.01em;
}

.form-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}

.form-card :deep(input),
.form-card :deep(textarea) {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: none;
}

.form-card :deep(input) {
  min-height: 2.75rem;
  border-radius: 0.75rem;
}

.form-card :deep(textarea) {
  border-radius: 0.75rem;
  resize: vertical;
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: rgb(var(--yb-text-faint));
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.form-card :deep(input:disabled),
.form-card :deep(textarea:disabled) {
  cursor: not-allowed;
  opacity: 0.55;
  background: rgb(var(--yb-surface-soft));
}

.form-card :deep(.relative > div) {
  min-height: 2.75rem;
  border-color: rgb(var(--yb-border-strong));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: none;
}

.form-card :deep(.relative > div:focus-within) {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.form-card :deep(.relative button) {
  color: rgb(var(--yb-text));
}

/* 浅色弹窗外壳（对齐 BaseModal / 阶段一全局壳） */
:global(.task-info-edit-modal-panel) {
  max-height: 94vh;
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: 0 10px 40px rgb(var(--yb-shadow) / 0.12);
}

:global(.task-info-edit-modal-panel > header) {
  border-bottom: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

:global(.task-info-edit-modal-panel > header h2) {
  color: rgb(var(--yb-text));
}

:global(.task-info-edit-modal-panel > header button) {
  color: rgb(var(--yb-text-muted));
}

:global(.task-info-edit-modal-panel > header button:hover) {
  color: rgb(var(--yb-text));
}

:global(.task-info-edit-modal-panel > div.flex-1) {
  color: rgb(var(--yb-text));
}
</style>
