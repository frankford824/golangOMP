<template>
  <details class="cost-explanation-panel" :open="open">
    <summary>
      <div>
        <span class="eyebrow">只读规则验证</span>
        <strong>{{ title }}</strong>
        <small>{{ summaryText }}</small>
      </div>
      <span class="summary-action">展开试算</span>
    </summary>

    <div class="panel-body">
      <div class="current-evidence">
        <div><span>当前成本</span><strong>{{ money(seed.currentCost) }}</strong></div>
        <div><span>当前规则</span><strong>{{ seed.currentRuleName || '尚未记录' }}</strong></div>
        <div><span>规则版本</span><strong>{{ seed.currentRuleVersion ? `第 ${seed.currentRuleVersion} 版` : '—' }}</strong></div>
        <div><span>当前结论</span><strong>{{ seed.requiresManualReview ? '需要人工复核' : '系统已记录' }}</strong></div>
      </div>

      <form class="preview-form" @submit.prevent="preview">
        <label>ERP 商品编码<input v-model.trim="draft.productIID" placeholder="用于精确匹配规则绑定" /></label>
        <label>规则分组 / 类目编码<input v-model.trim="draft.categoryCode" placeholder="未绑定 i_id 时用于兜底匹配" /></label>
        <label>宽<input v-model.trim="draft.width" inputmode="decimal" /></label>
        <label>高<input v-model.trim="draft.height" inputmode="decimal" /></label>
        <label>面积<input v-model.trim="draft.area" inputmode="decimal" /></label>
        <label>数量<input v-model.trim="draft.quantity" inputmode="numeric" /></label>
        <label>工艺<input v-model.trim="draft.process" /></label>
        <label class="wide">补充说明<input v-model.trim="draft.notes" placeholder="例如材质、特殊工艺或尺寸来源" /></label>
        <div class="form-actions">
          <p>这里只做只读试算，不会修改任务、资产、成本规则或 ERP 数据。</p>
          <button type="submit" :disabled="previewing">{{ previewing ? '试算中…' : '重新试算并解释' }}</button>
        </div>
      </form>

      <section v-if="result" class="result-card" aria-live="polite">
        <header>
          <div><span>试算结果</span><strong>{{ money(result.estimated_cost) }}</strong></div>
          <em :class="{ warning: result.requires_manual_review }">{{ result.requires_manual_review ? '需人工复核' : '规则可自动计算' }}</em>
        </header>
        <dl>
          <div><dt>匹配路径</dt><dd>{{ matchModeLabel }}</dd></div>
          <div><dt>规则版本</dt><dd>{{ result.matched_rule_version ? `第 ${result.matched_rule_version} 版` : '未匹配' }}</dd></div>
          <div><dt>规则来源</dt><dd>{{ result.rule_source || '—' }}</dd></div>
          <div><dt>规则分组</dt><dd>{{ result.rule_group || draft.categoryCode || '—' }}</dd></div>
        </dl>
        <p class="explanation">{{ result.explanation || '服务未返回额外解释。' }}</p>
        <p class="diagnosis">{{ diagnosis }}</p>

        <div class="feedback">
          <strong>这个结果符合业务预期吗？</strong>
          <textarea v-model.trim="feedbackNote" rows="2" placeholder="有疑问时请说明预期成本、输入来源或疑似错误类目" />
          <div>
            <button type="button" :disabled="feedbackBusy" @click="submitFeedback('expected')">符合预期</button>
            <button type="button" class="secondary" :disabled="feedbackBusy || !feedbackNote" @click="submitFeedback('needs_review')">结果有疑问</button>
            <span v-if="feedbackMessage" :class="{ error: feedbackFailed }" role="status">{{ feedbackMessage }}</span>
          </div>
        </div>
      </section>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  costManagementApi,
  type CostRulePreviewResponse,
} from '@/services/api/costManagementApi'
import { workflowTelemetryApi } from '@/services/api/workflowTelemetryApi'

interface CostPreviewSeed {
  categoryCode?: string
  productIID?: string
  erpIID?: string
  width?: number | string | null
  height?: number | string | null
  area?: number | string | null
  quantity?: number | string | null
  process?: string
  notes?: string
  currentCost?: number | null
  currentRuleName?: string
  currentRuleVersion?: number | null
  requiresManualReview?: boolean
}

const props = withDefaults(defineProps<{
  title: string
  seed: CostPreviewSeed
  taskId?: number
  taskSkuItemId?: number
  assetId?: number
  resourceId?: string
  skuCode?: string
  open?: boolean
}>(), { open: false })

const draft = reactive({
  categoryCode: '',
  productIID: '',
  erpIID: '',
  width: '',
  height: '',
  area: '',
  quantity: '',
  process: '',
  notes: '',
})
const result = ref<CostRulePreviewResponse | null>(null)
const previewing = ref(false)
const feedbackBusy = ref(false)
const feedbackNote = ref('')
const feedbackMessage = ref('')
const feedbackFailed = ref(false)
const error = ref('')

watch(
  () => props.seed,
  (seed) => {
    draft.categoryCode = text(seed.categoryCode)
    draft.productIID = text(seed.productIID)
    draft.erpIID = text(seed.erpIID)
    draft.width = text(seed.width)
    draft.height = text(seed.height)
    draft.area = text(seed.area)
    draft.quantity = text(seed.quantity)
    draft.process = text(seed.process)
    draft.notes = text(seed.notes)
    result.value = null
    error.value = ''
  },
  { immediate: true, deep: true },
)

const summaryText = computed(() => {
  if (result.value) return diagnosis.value
  if (props.seed.currentRuleName) return `当前按「${props.seed.currentRuleName}」记录，可展开复算核对`
  return '当前没有可解释的规则快照，可展开核对输入与规则绑定'
})

const matchModeLabel = computed(() => ({
  binding_erp_i_id: 'ERP i_id 精确绑定',
  binding_product_i_id: '商品 i_id 精确绑定',
  legacy_alias: '旧类目文本兜底',
  no_match: '未匹配',
}[result.value?.match_mode || ''] || result.value?.match_mode || '服务未说明'))

const diagnosis = computed(() => {
  const previewResult = result.value
  if (!previewResult) return ''
  if (!previewResult.matched_rule_id) return '未匹配成本规则：优先核对 ERP 商品编码是否绑定规则，再核对规则分组或类目编码。'
  if (previewResult.requires_manual_review) return '规则已经匹配，但当前输入或规则类型要求人工复核；请核对尺寸、面积、数量与特殊工艺。'
  if (typeof props.seed.currentCost === 'number' && typeof previewResult.estimated_cost === 'number') {
    const delta = previewResult.estimated_cost - props.seed.currentCost
    if (Math.abs(delta) > 0.01) return `试算比当前成本${delta > 0 ? '高' : '低'} ${money(Math.abs(delta))}；请核对当前成本快照和本次输入是否来自同一版本。`
  }
  return '试算规则已匹配，结果与当前记录未发现明显冲突。'
})

function text(value: unknown) {
  return value == null ? '' : String(value)
}

function optionalNumber(value: string) {
  if (!value.trim()) return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function optionalInteger(value: string) {
  const parsed = optionalNumber(value)
  return parsed != null && Number.isInteger(parsed) ? parsed : undefined
}

function money(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? `¥ ${value.toFixed(2)}` : '待计算'
}

async function preview() {
  error.value = ''
  feedbackMessage.value = ''
  if (!draft.productIID && !draft.erpIID && !draft.categoryCode) {
    error.value = props.seed.currentRuleName
      ? '当前规则快照缺少可复算的 ERP 商品编码或规则分组，请先补充后再试算。'
      : '请先填写 ERP 商品编码或规则分组 / 类目编码。'
    return
  }
  const quantity = optionalInteger(draft.quantity)
  if (draft.quantity && quantity == null) {
    error.value = '数量必须是整数。'
    return
  }
  previewing.value = true
  try {
    result.value = await costManagementApi.previewCostRule({
      category_code: draft.categoryCode || undefined,
      product_i_id: draft.productIID || undefined,
      erp_i_id: draft.erpIID || undefined,
      width: optionalNumber(draft.width),
      height: optionalNumber(draft.height),
      area: optionalNumber(draft.area),
      quantity,
      process: draft.process || undefined,
      notes: draft.notes || undefined,
    })
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '成本试算失败，请稍后重试。'
  } finally {
    previewing.value = false
  }
}

async function submitFeedback(outcome: 'expected' | 'needs_review') {
  if (!result.value || feedbackBusy.value || (outcome === 'needs_review' && !feedbackNote.value)) return
  feedbackBusy.value = true
  feedbackFailed.value = false
  feedbackMessage.value = ''
  try {
    await workflowTelemetryApi.recordEvent({
      event_type: 'user_action',
      action: 'cost.preview.feedback',
      page_url: window.location.pathname,
      page_name: props.taskId ? '任务成本解释' : '资产成本解释',
      component_id: 'cost-explanation-panel',
      task_id: props.taskId,
      task_sku_item_id: props.taskSkuItemId,
      asset_id: props.assetId,
      sku_code: props.skuCode,
      resource_type: 'cost_preview',
      resource_id: props.resourceId,
      outcome,
      payload: {
        feedback_note: feedbackNote.value,
        matched_rule_id: result.value.matched_rule_id,
        matched_rule_version: result.value.matched_rule_version,
        estimated_cost: result.value.estimated_cost,
        rule_source: result.value.rule_source,
        match_mode: result.value.match_mode,
        requires_manual_review: result.value.requires_manual_review,
        input: { ...draft },
      },
    })
    feedbackMessage.value = '反馈已记录，规则与输入仍保持不变。'
  } catch (cause) {
    feedbackFailed.value = true
    feedbackMessage.value = cause instanceof Error ? cause.message : '反馈记录失败，请稍后重试。'
  } finally {
    feedbackBusy.value = false
  }
}
</script>

<style scoped>
.cost-explanation-panel{overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:14px;background:rgb(var(--yb-surface))}.cost-explanation-panel>summary{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px 16px;cursor:pointer;list-style:none}.cost-explanation-panel>summary::-webkit-details-marker{display:none}.cost-explanation-panel>summary div{min-width:0;display:grid;gap:3px}.cost-explanation-panel>summary strong{font-size:14px}.cost-explanation-panel>summary small{overflow:hidden;color:rgb(var(--yb-text-muted));font-size:11px;text-overflow:ellipsis;white-space:nowrap}.eyebrow{color:rgb(var(--yb-brand));font-size:10px;font-weight:850;letter-spacing:.08em}.summary-action{flex:0 0 auto;color:rgb(var(--yb-brand));font-size:11px;font-weight:750}.panel-body{display:grid;gap:14px;padding:0 16px 16px;border-top:1px solid rgb(var(--yb-border))}.current-evidence{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));margin-top:14px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface-soft));overflow:hidden}.current-evidence div{display:grid;gap:4px;padding:10px}.current-evidence div+div{border-left:1px solid rgb(var(--yb-border))}.current-evidence span,.current-evidence strong{font-size:11px}.current-evidence span{color:rgb(var(--yb-text-muted))}.preview-form{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.preview-form label{display:grid;gap:5px;color:rgb(var(--yb-text-muted));font-size:11px;font-weight:700}.preview-form label.wide{grid-column:span 2}.preview-form input,.feedback textarea{width:100%;box-sizing:border-box;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:500 12px var(--yb-font-sans)}.preview-form input{min-height:35px;padding:7px 9px}.form-actions{grid-column:1/-1;display:flex;align-items:center;justify-content:space-between;gap:12px}.form-actions p{margin:0;color:rgb(var(--yb-text-muted));font-size:10px}.form-actions button,.feedback button{min-height:34px;padding:0 12px;border:1px solid rgb(var(--yb-brand));border-radius:8px;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-size:11px;font-weight:750;cursor:pointer}.form-actions button:disabled,.feedback button:disabled{opacity:.5;cursor:not-allowed}.result-card{display:grid;gap:10px;padding:13px;border:1px solid rgb(var(--yb-brand-border));border-radius:11px;background:rgb(var(--yb-brand-soft))}.result-card>header{display:flex;align-items:center;justify-content:space-between}.result-card>header div{display:grid;gap:2px}.result-card>header span{color:rgb(var(--yb-text-muted));font-size:10px}.result-card>header strong{color:rgb(var(--yb-brand));font-size:18px}.result-card em{padding:4px 7px;border-radius:999px;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text));font-size:10px;font-style:normal;font-weight:750}.result-card em.warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.result-card dl{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin:0}.result-card dl div{display:grid;gap:2px}.result-card dt{color:rgb(var(--yb-text-muted));font-size:10px}.result-card dd{margin:0;font-size:11px;font-weight:700}.explanation,.diagnosis{margin:0;line-height:1.5;font-size:11px}.diagnosis{padding:8px;border-radius:8px;background:rgb(var(--yb-surface));font-weight:650}.feedback{display:grid;gap:8px;padding-top:10px;border-top:1px solid rgb(var(--yb-brand-border))}.feedback>strong{font-size:11px}.feedback textarea{padding:8px;resize:vertical}.feedback>div{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.feedback button.secondary{background:rgb(var(--yb-surface));color:rgb(var(--yb-brand))}.feedback span{color:rgb(var(--yb-success-text));font-size:10px}.feedback span.error,.error-message{color:rgb(var(--yb-danger-text))}.error-message{margin:0;font-size:11px}@media(max-width:850px){.current-evidence,.preview-form,.result-card dl{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:520px){.current-evidence,.preview-form,.result-card dl{grid-template-columns:1fr}.current-evidence div+div{border-left:0;border-top:1px solid rgb(var(--yb-border))}.preview-form label.wide{grid-column:auto}.form-actions{align-items:stretch;flex-direction:column}.form-actions button{width:100%}}
</style>
