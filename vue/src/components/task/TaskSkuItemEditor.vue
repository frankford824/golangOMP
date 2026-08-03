<template>
  <div class="sku-editor">
    <article v-for="item in items" :key="itemKey(item)" class="sku-editor-row">
      <header>
        <div>
          <strong>{{ displayText(item.sku_code) || `子项 ${displayText(item.sequence_no)}` }}</strong>
          <span>{{ displayText(item.product_name_snapshot) || '未填写产品名称' }}</span>
        </div>
        <span class="edit-state">{{ canEdit ? '可编辑' : '只读' }}</span>
      </header>

      <form v-if="drafts[itemKey(item)]" @submit.prevent="save(item)">
        <label>产品名称<input v-model.trim="drafts[itemKey(item)].productName" :disabled="!canEdit || isSaving(item)" /></label>
        <label>ERP 商品编码<input v-model.trim="drafts[itemKey(item)].productIID" :disabled="!canEdit || isSaving(item)" /></label>
        <label>规格<input v-model.trim="drafts[itemKey(item)].specText" :disabled="!canEdit || isSaving(item)" placeholder="例如：500ml" /></label>
        <label>尺寸<input v-model.trim="drafts[itemKey(item)].sizeText" :disabled="!canEdit || isSaving(item)" placeholder="例如：20×30cm" /></label>
        <label>宽<input v-model.trim="drafts[itemKey(item)].width" :disabled="!canEdit || isSaving(item)" inputmode="decimal" /></label>
        <label>高<input v-model.trim="drafts[itemKey(item)].height" :disabled="!canEdit || isSaving(item)" inputmode="decimal" /></label>
        <label>面积<input v-model.trim="drafts[itemKey(item)].area" :disabled="!canEdit || isSaving(item)" inputmode="decimal" /></label>
        <label>数量<input v-model.trim="drafts[itemKey(item)].quantity" :disabled="!canEdit || isSaving(item)" inputmode="numeric" /></label>
        <label class="wide">运营修改要求<textarea v-model.trim="drafts[itemKey(item)].designRequirement" :disabled="!canEdit || isSaving(item)" rows="2" /></label>
        <label>当前/人工成本<input v-model.trim="drafts[itemKey(item)].costPrice" :disabled="!canEdit || isSaving(item)" inputmode="decimal" placeholder="留空则不改成本" /></label>
        <label>成本调整原因<input v-model.trim="drafts[itemKey(item)].costReason" :disabled="!canEdit || isSaving(item)" :required="Boolean(drafts[itemKey(item)].costPrice)" placeholder="人工改成本时必填" /></label>
        <div class="row-actions">
          <p v-if="messages[itemKey(item)]" :class="{ error: failures[itemKey(item)] }" role="status">{{ messages[itemKey(item)] }}</p>
          <button v-if="canEdit" type="submit" :disabled="isSaving(item)">{{ isSaving(item) ? '保存中…' : '保存该 SKU' }}</button>
        </div>
      </form>
    </article>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { tasksApi } from '@/services/api/tasksApi'

interface SKUItem extends Record<string, unknown> {
  id?: number | string
  sequence_no?: number | string
  sku_code?: string
  product_name_snapshot?: string
  product_i_id?: string
  design_requirement?: string
  cost_price?: number | string | null
  manual_cost_override_reason?: string
  variant_json?: Record<string, unknown> | string | null
}

interface SKUDraft {
  productName: string
  productIID: string
  specText: string
  sizeText: string
  width: string
  height: string
  area: string
  quantity: string
  designRequirement: string
  costPrice: string
  costReason: string
}

const props = defineProps<{ taskId: number; items: SKUItem[]; canEdit: boolean }>()
const emit = defineEmits<{ saved: [] }>()
const drafts = ref<Record<string, SKUDraft>>({})
const saving = ref<Record<string, boolean>>({})
const messages = ref<Record<string, string>>({})
const failures = ref<Record<string, boolean>>({})

function displayText(value: unknown) {
  return value == null ? '' : String(value)
}

function itemKey(item: SKUItem) {
  return String(item.id || item.sku_code || item.sequence_no || '')
}

function variant(item: SKUItem): Record<string, unknown> {
  if (item.variant_json && typeof item.variant_json === 'object') return item.variant_json
  if (typeof item.variant_json !== 'string' || !item.variant_json.trim()) return {}
  try {
    const parsed = JSON.parse(item.variant_json) as unknown
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : {}
  } catch {
    return {}
  }
}

function sourceValue(item: SKUItem, key: string) {
  const direct = item[key]
  return direct == null || direct === '' ? variant(item)[key] : direct
}

function createDraft(item: SKUItem): SKUDraft {
  return {
    productName: displayText(item.product_name_snapshot),
    productIID: displayText(item.product_i_id),
    specText: displayText(sourceValue(item, 'spec_text')),
    sizeText: displayText(sourceValue(item, 'size_text')),
    width: displayText(sourceValue(item, 'width')),
    height: displayText(sourceValue(item, 'height')),
    area: displayText(sourceValue(item, 'area')),
    quantity: displayText(item.quantity),
    designRequirement: displayText(item.design_requirement),
    costPrice: displayText(item.cost_price),
    costReason: displayText(item.manual_cost_override_reason),
  }
}

function optionalNumber(value: string): number | undefined {
  if (!value.trim()) return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function optionalInteger(value: string): number | undefined {
  const parsed = optionalNumber(value)
  return parsed != null && Number.isInteger(parsed) ? parsed : undefined
}

function isSaving(item: SKUItem) {
  return Boolean(saving.value[itemKey(item)])
}

watch(
  () => props.items,
  (items) => {
    drafts.value = Object.fromEntries(items.map((item) => [itemKey(item), createDraft(item)]))
    messages.value = {}
    failures.value = {}
  },
  { immediate: true },
)

async function save(item: SKUItem) {
  const key = itemKey(item)
  const skuItemId = Number(item.id)
  const draft = drafts.value[key]
  if (!props.canEdit || !draft || !Number.isInteger(skuItemId) || skuItemId <= 0 || saving.value[key]) return

  const cost = optionalNumber(draft.costPrice)
  if (draft.costPrice && cost == null) {
    failures.value[key] = true
    messages.value[key] = '成本必须是有效数字。'
    return
  }
  if (draft.quantity && optionalInteger(draft.quantity) == null) {
    failures.value[key] = true
    messages.value[key] = '数量必须是整数。'
    return
  }
  if (cost != null && !draft.costReason.trim()) {
    failures.value[key] = true
    messages.value[key] = '人工调整成本时必须填写原因。'
    return
  }

  saving.value[key] = true
  failures.value[key] = false
  messages.value[key] = ''
  try {
    await tasksApi.patchSkuItem(String(props.taskId), skuItemId, {
      product_name: draft.productName,
      product_i_id: draft.productIID,
      spec_text: draft.specText,
      size_text: draft.sizeText,
      width: optionalNumber(draft.width),
      height: optionalNumber(draft.height),
      area: optionalNumber(draft.area),
      quantity: optionalInteger(draft.quantity),
      design_requirement: draft.designRequirement,
      remark: '任务详情逐 SKU 信息维护',
    })
    if (cost != null) {
      await tasksApi.patchSkuItemCostInfo(String(props.taskId), skuItemId, {
        cost_price: cost,
        manual_cost_override: true,
        manual_cost_override_reason: draft.costReason,
        remark: '任务详情逐 SKU 成本维护',
      })
    }
    messages.value[key] = '已保存。'
    emit('saved')
  } catch (cause) {
    failures.value[key] = true
    messages.value[key] = cause instanceof Error ? cause.message : '保存失败，请稍后重试。'
  } finally {
    saving.value[key] = false
  }
}
</script>

<style scoped>
.sku-editor{display:grid;gap:12px;margin-top:10px}.sku-editor-row{overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:13px;background:rgb(var(--yb-surface))}.sku-editor-row>header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 14px;border-bottom:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft))}.sku-editor-row header div{min-width:0;display:grid;gap:3px}.sku-editor-row header strong{color:rgb(var(--yb-brand));font:800 12px var(--yb-font-data)}.sku-editor-row header span:not(.edit-state){overflow:hidden;color:rgb(var(--yb-text-muted));font-size:12px;text-overflow:ellipsis;white-space:nowrap}.edit-state{flex:0 0 auto;padding:3px 8px;border-radius:999px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-size:10px;font-weight:750}.sku-editor-row form{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;padding:14px}.sku-editor-row label{min-width:0;display:grid;gap:5px;color:rgb(var(--yb-text-muted));font-size:11px;font-weight:700}.sku-editor-row label.wide{grid-column:span 2}.sku-editor-row input,.sku-editor-row textarea{width:100%;min-height:36px;padding:8px 9px;border:1px solid rgb(var(--yb-border));border-radius:9px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:500 12px var(--yb-font-sans);box-sizing:border-box}.sku-editor-row textarea{resize:vertical}.sku-editor-row input:disabled,.sku-editor-row textarea:disabled{background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text-muted))}.row-actions{grid-column:1/-1;display:flex;align-items:center;justify-content:flex-end;gap:12px}.row-actions p{margin:0;color:rgb(var(--yb-success-strong));font-size:11px}.row-actions p.error{color:rgb(var(--yb-danger-text))}.row-actions button{min-height:36px;padding:0 13px;border:0;border-radius:9px;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-size:12px;font-weight:750;cursor:pointer}.row-actions button:disabled{opacity:.55;cursor:wait}@media(max-width:900px){.sku-editor-row form{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:560px){.sku-editor-row form{grid-template-columns:1fr}.sku-editor-row label.wide{grid-column:auto}}
</style>
