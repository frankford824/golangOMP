<template>
  <main class="cost-manager-page">
    <header class="yb-page-surface yb-page-header-row">
      <div class="yb-page-heading-copy">
        <p class="eyebrow">统一成本口径</p>
        <h1 class="yb-page-title">成本规则</h1>
        <p class="yb-page-subtitle">维护款式使用哪组计价规则，并在应用前用真实尺寸试算影响；资产中心与任务详情读取同一份成本结果。</p>
      </div>
      <div class="header-actions">
        <button class="secondary" :disabled="loading" @click="loadAll">{{ loading ? '刷新中…' : '刷新数据' }}</button>
        <button class="primary" @click="openRuleEditor()">新增规则</button>
      </div>
    </header>

    <div v-if="error" class="message error" role="alert">{{ error }}<button @click="loadAll">重试</button></div>
    <div v-if="notice" class="message notice" role="status">{{ notice }}</div>

    <section class="cost-layout">
      <aside class="rule-groups" aria-label="成本规则分组">
        <header><span>当前规则组</span><b>{{ ruleGroups.length }}</b></header>
        <button
          v-for="group in ruleGroups"
          :key="group.code"
          :class="{ active: selectedGroupCode === group.code }"
          @click="selectedGroupCode = group.code"
        >
          <span><strong>{{ group.name }}</strong><small>{{ group.code }}</small></span>
          <b>{{ group.rules.length }}</b>
        </button>
        <div v-if="!loading && !ruleGroups.length" class="empty-small">尚未配置成本规则。</div>
      </aside>

      <section class="rule-workspace">
        <header class="workspace-heading">
          <div>
            <p class="eyebrow">{{ selectedGroup?.code || '请选择规则组' }}</p>
            <h2>{{ selectedGroup?.name || '成本计算规则' }}</h2>
            <p>{{ selectedGroup ? `${selectedGroup.rules.length} 条生效/历史规则共同组成这套计价方式。` : '从左侧选择一组规则查看。' }}</p>
          </div>
          <button v-if="selectedGroup" class="secondary" @click="openRuleEditor(undefined, selectedGroup.code)">在此组新增</button>
        </header>

        <div v-if="selectedGroup" class="rule-list">
          <article v-for="rule in selectedGroup.rules" :key="rule.rule_id" class="rule-card" :class="{ inactive: !rule.is_active }">
            <div class="rule-icon" aria-hidden="true">{{ ruleTypeIcon(rule.rule_type) }}</div>
            <div class="rule-copy">
              <div><strong>{{ rule.rule_name || '未命名规则' }}</strong><span>{{ rule.is_active ? '正在使用' : '已停用' }}</span></div>
              <p>{{ ruleSummary(rule) }}</p>
              <small>优先级 {{ rule.priority ?? 0 }} · 第 {{ rule.rule_version || 1 }} 版</small>
            </div>
            <button class="text-button" @click="openRuleEditor(rule)">编辑</button>
          </article>
          <div v-if="!selectedGroup.rules.length" class="empty-large">这个规则组还没有计算条目。</div>
        </div>

        <section v-if="selectedGroup" class="binding-panel">
          <header>
            <div><h3>已绑定款式</h3><p>以下款式编码会优先使用“{{ selectedGroup.name }}”。</p></div>
            <button class="secondary" @click="bindingDialogOpen = true">绑定新款式</button>
          </header>
          <div class="binding-chips">
            <span v-for="binding in selectedBindings" :key="binding.id"><b>{{ binding.i_id_raw || binding.normalized_i_id }}</b><small>{{ binding.display_name || '已绑定' }}</small></span>
            <span v-if="!selectedBindings.length" class="empty-inline">还没有款式绑定到此组。</span>
          </div>
        </section>
      </section>

      <aside class="calculator" aria-label="成本试算器">
        <header><p class="eyebrow">不会保存数据</p><h2>成本试算器</h2><span>用实际尺寸验证当前规则。</span></header>
        <label>规则组<select v-model="calculator.rule_group"><option value="">请选择</option><option v-for="group in ruleGroups" :key="group.code" :value="group.code">{{ group.name }}</option></select></label>
        <div class="field-pair"><label>宽（米）<input v-model.number="calculator.width" type="number" min="0" step="0.01" /></label><label>高（米）<input v-model.number="calculator.height" type="number" min="0" step="0.01" /></label></div>
        <div class="field-pair"><label>面积（㎡）<input v-model.number="calculator.area" type="number" min="0" step="0.001" /></label><label>数量<input v-model.number="calculator.quantity" type="number" min="1" step="1" /></label></div>
        <label>特殊工艺<input v-model.trim="calculator.process" placeholder="例如：覆膜、开槽" /></label>
        <button class="primary calculate-button" :disabled="previewing || !calculator.rule_group" @click="previewCost">{{ previewing ? '计算中…' : '开始试算' }}</button>
        <div class="preview-result" :class="{ warning: preview?.requires_manual_review }">
          <span>预计成本</span>
          <strong>{{ previewCostLabel }}</strong>
          <p>{{ previewExplanation }}</p>
        </div>
      </aside>
    </section>

    <section class="operations-panel" aria-labelledby="cost-operations-title">
      <header class="operations-heading">
        <div>
          <p class="eyebrow">变更确认与 ERP 同步</p>
          <h2 id="cost-operations-title">成本更新记录</h2>
          <p>规则调整只先生成影响预览。确认无误后再更新任务与 SKU，最后由人工明确同步到 ERP。</p>
        </div>
        <div class="cost-health" aria-label="成本数据概况">
          <span><b>{{ costDashboard.total_records ?? costDashboard.total_count ?? 0 }}</b> 个 SKU</span>
          <span :class="{ warning: erpMismatchCount > 0 }"><b>{{ erpMismatchCount }}</b> 个 ERP 差异</span>
        </div>
      </header>
      <div v-if="!runs.length" class="empty-large">尚未生成成本影响预览。</div>
      <div v-else class="run-list">
        <article v-for="run in runs" :key="run.id" class="run-card">
          <div class="run-identity">
            <span :class="['run-status', runStatusTone(run.status)]">{{ runStatusLabel(run.status) }}</span>
            <strong>{{ run.run_no || `成本更新 #${run.id}` }}</strong>
            <small>{{ formatDateTime(run.created_at) }}</small>
          </div>
          <div class="run-summary">
            <span><b>{{ run.summary?.total_count ?? 0 }}</b> 个 SKU</span>
            <span><b>{{ run.summary?.applied_count ?? 0 }}</b> 已更新</span>
            <span><b>{{ run.summary?.erp_synced_count ?? 0 }}</b> 已同步 ERP</span>
            <span v-if="run.summary?.conflict_count" class="warning"><b>{{ run.summary.conflict_count }}</b> 个冲突</span>
          </div>
          <div class="run-actions">
            <button class="secondary" @click="openRun(run.id)">查看影响</button>
            <button v-if="canApplyRun(run.status)" class="primary" :disabled="runActionBusy === run.id" @click="applyRun(run.id)">确认更新</button>
            <button v-if="canSyncRun(run.status)" class="primary" :disabled="runActionBusy === run.id" @click="syncRunERP(run.id)">同步到 ERP</button>
          </div>
        </article>
      </div>
    </section>

    <Teleport to="body">
      <div v-if="ruleDialogOpen" class="modal-layer" @keydown.esc="closeRuleEditor">
        <button class="modal-mask" aria-label="关闭规则编辑" @click="closeRuleEditor" />
        <form class="modal-card" @submit.prevent="saveRule">
          <header><div><p class="eyebrow">{{ ruleDraft.rule_id ? '调整现有规则' : '新增计算规则' }}</p><h2>{{ ruleDraft.rule_id ? '编辑规则' : '新建规则' }}</h2></div><button type="button" class="close" @click="closeRuleEditor">×</button></header>
          <div class="form-grid">
            <label class="span-2">规则名称<input v-model.trim="ruleDraft.rule_name" required placeholder="例如：KT 板面积计价" /></label>
            <label>规则组<input v-model.trim="ruleDraft.category_code" required placeholder="例如：KT_BOARD" /></label>
            <label>计算方式<select v-model="ruleDraft.rule_type"><option v-for="item in ruleTypes" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
            <label>基础单价<input v-model.number="ruleDraft.base_price" type="number" min="0" step="0.01" /></label>
            <label>含税倍率<input v-model.number="ruleDraft.tax_multiplier" type="number" min="0" step="0.01" /></label>
            <label>最低计价面积<input v-model.number="ruleDraft.min_area" type="number" min="0" step="0.001" /></label>
            <label>面积阈值<input v-model.number="ruleDraft.area_threshold" type="number" min="0" step="0.001" /></label>
            <label>附加金额<input v-model.number="ruleDraft.surcharge_amount" type="number" min="0" step="0.01" /></label>
            <label>工艺关键字<input v-model.trim="ruleDraft.special_process_keyword" placeholder="选填" /></label>
            <label>工艺加价<input v-model.number="ruleDraft.special_process_price" type="number" min="0" step="0.01" /></label>
            <label>优先级<input v-model.number="ruleDraft.priority" type="number" step="1" /></label>
            <label class="switch-row"><input v-model="ruleDraft.is_active" type="checkbox" /> 当前启用</label>
            <label class="span-2">维护说明<textarea v-model.trim="ruleDraft.governance_note" rows="3" placeholder="说明本次调整原因，方便后续追溯。" /></label>
          </div>
          <footer><button type="button" class="secondary" @click="closeRuleEditor">取消</button><button class="primary" :disabled="savingRule">{{ savingRule ? '保存中…' : '保存并生成影响预览' }}</button></footer>
        </form>
      </div>

      <div v-if="bindingDialogOpen" class="modal-layer" @keydown.esc="bindingDialogOpen = false">
        <button class="modal-mask" aria-label="关闭款式绑定" @click="bindingDialogOpen = false" />
        <section class="modal-card binding-dialog" role="dialog" aria-modal="true" aria-labelledby="binding-title">
          <header><div><p class="eyebrow">款式与规则</p><h2 id="binding-title">绑定到“{{ selectedGroup?.name }}”</h2></div><button class="close" @click="bindingDialogOpen = false">×</button></header>
          <label class="dialog-search">搜索未绑定款式<input v-model.trim="candidateKeyword" placeholder="输入款式编码或名称" @keyup.enter="loadCandidates" /><button class="secondary" @click="loadCandidates">搜索</button></label>
          <div class="candidate-list">
            <article v-for="candidate in candidates" :key="candidate.normalized_i_id">
              <span><strong>{{ candidate.i_id_raw || candidate.display_i_id || candidate.normalized_i_id }}</strong><small>{{ candidate.suggested_display_name || `${candidate.sku_count || 0} 个 SKU` }}</small></span>
              <button class="primary" :disabled="savingBinding" @click="bindCandidate(candidate)">绑定</button>
            </article>
            <div v-if="!candidates.length" class="empty-large">没有找到待绑定款式。</div>
          </div>
        </section>
      </div>

      <div v-if="runDialogOpen" class="modal-layer" @keydown.esc="runDialogOpen = false">
        <button class="modal-mask" aria-label="关闭成本影响明细" @click="runDialogOpen = false" />
        <section class="modal-card run-dialog" role="dialog" aria-modal="true" aria-labelledby="run-dialog-title">
          <header><div><p class="eyebrow">应用前核对</p><h2 id="run-dialog-title">{{ selectedRun?.run_no || '成本影响明细' }}</h2></div><button class="close" @click="runDialogOpen = false">×</button></header>
          <div class="run-detail-list">
            <article v-for="item in selectedRun?.items || []" :key="item.id">
              <div><strong>{{ item.sku_code || '未命名 SKU' }}</strong><small>{{ item.task_no || '未关联任务号' }}</small></div>
              <span>{{ costChangeLabel(item.old_cost_price, item.new_cost_price) }}</span>
              <em>{{ runItemStatusLabel(item.status) }}</em>
            </article>
            <div v-if="!(selectedRun?.items?.length)" class="empty-large">当前预览没有可更新的 SKU。</div>
          </div>
          <footer>
            <button class="secondary" @click="runDialogOpen = false">关闭</button>
            <button v-if="selectedRun && canApplyRun(selectedRun.status)" class="primary" :disabled="runActionBusy === selectedRun.id" @click="applyRun(selectedRun.id)">确认更新这些 SKU</button>
            <button v-if="selectedRun && canSyncRun(selectedRun.status)" class="primary" :disabled="runActionBusy === selectedRun.id" @click="syncRunERP(selectedRun.id)">同步已更新成本到 ERP</button>
          </footer>
        </section>
      </div>
    </Teleport>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { categoriesApi } from '@/services/api/categoriesApi'
import {
  costManagementApi,
  type CostRecalculationRun,
  type CostRuleBinding,
  type CostRulePreviewResponse,
  type ProductCostDashboardResponse,
  type UnboundCostRuleCandidate,
} from '@/services/api/costManagementApi'

interface CostRuleRow {
  rule_id: number
  rule_name: string
  rule_version?: number
  category_code: string
  product_family?: string
  rule_type: string
  base_price?: number | null
  tax_multiplier?: number | null
  min_area?: number | null
  area_threshold?: number | null
  surcharge_amount?: number | null
  special_process_keyword?: string
  special_process_price?: number | null
  priority?: number
  is_active?: boolean
  governance_note?: string
}

interface RuleDraft extends Partial<CostRuleRow> { rule_name: string; category_code: string; rule_type: string; is_active: boolean; priority: number }

const ruleTypes = [
  { value: 'fixed_unit_price', label: '固定单价' },
  { value: 'area_threshold_surcharge', label: '面积阈值加价' },
  { value: 'minimum_billable_area', label: '最低计价面积' },
  { value: 'size_based_formula', label: '尺寸公式' },
  { value: 'special_process_surcharge', label: '特殊工艺加价' },
  { value: 'manual_quote', label: '人工报价' },
]
const rules = ref<CostRuleRow[]>([])
const bindings = ref<CostRuleBinding[]>([])
const candidates = ref<UnboundCostRuleCandidate[]>([])
const runs = ref<CostRecalculationRun[]>([])
const costDashboard = ref<ProductCostDashboardResponse>({ total_count: 0, groups: [], tags: [] })
const selectedRun = ref<CostRecalculationRun | null>(null)
const selectedGroupCode = ref('')
const loading = ref(false)
const error = ref('')
const notice = ref('')
const ruleDialogOpen = ref(false)
const bindingDialogOpen = ref(false)
const runDialogOpen = ref(false)
const savingRule = ref(false)
const savingBinding = ref(false)
const previewing = ref(false)
const runActionBusy = ref<number | null>(null)
const candidateKeyword = ref('')
const preview = ref<CostRulePreviewResponse | null>(null)
const ruleDraft = reactive<RuleDraft>(emptyRuleDraft())
const calculator = reactive({ rule_group: '', width: 1, height: 1, area: null as number | null, quantity: 1, process: '' })

const ruleGroups = computed(() => {
  const grouped = new Map<string, CostRuleRow[]>()
  for (const rule of rules.value) {
    const code = String(rule.category_code || 'UNASSIGNED').trim()
    grouped.set(code, [...(grouped.get(code) || []), rule])
  }
  return Array.from(grouped, ([code, groupRules]) => ({ code, name: groupDisplayName(groupRules, code), rules: groupRules.sort((a, b) => (b.priority || 0) - (a.priority || 0)) }))
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})
const selectedGroup = computed(() => ruleGroups.value.find((group) => group.code === selectedGroupCode.value) || ruleGroups.value[0] || null)
const selectedBindings = computed(() => bindings.value.filter((binding) => binding.rule_group === selectedGroup.value?.code && binding.is_active))
const previewCostLabel = computed(() => typeof preview.value?.estimated_cost === 'number' ? `¥ ${preview.value.estimated_cost.toFixed(2)}` : preview.value?.requires_manual_review ? '需要人工报价' : '—')
const previewExplanation = computed(() => preview.value?.explanation || (preview.value ? '计算完成。' : '填写尺寸后试算，不会修改任何任务或 ERP 数据。'))
const erpMismatchCount = computed(() => costDashboard.value.tags?.find((item) => item.code === 'erp_mismatch')?.count || 0)

watch(selectedGroup, (group) => { if (group && !calculator.rule_group) calculator.rule_group = group.code }, { immediate: true })

function emptyRuleDraft(group = ''): RuleDraft { return { rule_name: '', category_code: group, rule_type: 'fixed_unit_price', is_active: true, priority: 100 } }
function replaceDraft(next: RuleDraft) { for (const key of Object.keys(ruleDraft)) delete (ruleDraft as Record<string, unknown>)[key]; Object.assign(ruleDraft, next) }
function groupDisplayName(groupRules: CostRuleRow[], code: string) {
  const family = groupRules.map((rule) => String(rule.product_family || '').trim().toLowerCase()).find(Boolean) || ''
  const familyLabel = ({ board: '板材制作', cloth: '布艺制作', photo: '照片与印刷', paper: '纸张印刷' } as Record<string, string>)[family]
  const ruleLabel = groupRules.map((rule) => String(rule.rule_name || '').replace(/(?:基础单价|最低计价面积|面积加价|小面积附加|工艺加价)$/u, '').trim()).find(Boolean)
  return ruleLabel || familyLabel || code
}
function ruleTypeLabel(type: string) { return ruleTypes.find((item) => item.value === type)?.label || '其他计价方式' }
function ruleTypeIcon(type: string) { return ({ fixed_unit_price: '¥', area_threshold_surcharge: '㎡', minimum_billable_area: '▣', size_based_formula: '×', special_process_surcharge: '+', manual_quote: '人' } as Record<string, string>)[type] || '•' }
function money(value?: number | null) { return typeof value === 'number' ? `¥${value.toFixed(2)}` : '未填写' }
function ruleSummary(rule: CostRuleRow) {
  if (rule.rule_type === 'fixed_unit_price') return `${ruleTypeLabel(rule.rule_type)} · ${money(rule.base_price)}${rule.tax_multiplier ? ` × ${rule.tax_multiplier} 含税倍率` : ''}`
  if (rule.rule_type === 'minimum_billable_area') return `${ruleTypeLabel(rule.rule_type)} · 最低 ${rule.min_area ?? '未填写'} ㎡`
  if (rule.rule_type === 'area_threshold_surcharge') return `${ruleTypeLabel(rule.rule_type)} · ${rule.area_threshold ?? '未填写'} ㎡以内加 ${money(rule.surcharge_amount)}`
  if (rule.rule_type === 'special_process_surcharge') return `${ruleTypeLabel(rule.rule_type)} · “${rule.special_process_keyword || '未填写工艺'}”加 ${money(rule.special_process_price)}`
  return ruleTypeLabel(rule.rule_type)
}

async function loadAll() {
  loading.value = true; error.value = ''; notice.value = ''
  try {
    const [ruleResponse, bindingResponse, runResponse, dashboardResponse] = await Promise.all([
      categoriesApi.listCostRules({ page: 1, page_size: 500 }),
      costManagementApi.listCostRuleBindings({ page: 1, page_size: 500 }),
      costManagementApi.listCostRecalculationRuns({ page: 1, page_size: 20 }),
      costManagementApi.getCostDashboard(),
    ])
    const body = ruleResponse.data as { data?: CostRuleRow[] }
    rules.value = body.data || []
    bindings.value = bindingResponse.data || []
    runs.value = runResponse.data || []
    costDashboard.value = dashboardResponse
    if (!selectedGroupCode.value || !ruleGroups.value.some((group) => group.code === selectedGroupCode.value)) selectedGroupCode.value = ruleGroups.value[0]?.code || ''
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成本规则加载失败。' }
  finally { loading.value = false }
}

function openRuleEditor(rule?: CostRuleRow, group = '') { replaceDraft(rule ? { ...rule, is_active: rule.is_active !== false, priority: rule.priority ?? 100 } : emptyRuleDraft(group)); ruleDialogOpen.value = true }
function closeRuleEditor() { if (!savingRule.value) ruleDialogOpen.value = false }
async function saveRule() {
  savingRule.value = true; error.value = ''; notice.value = ''
  try {
    const payload = Object.fromEntries(Object.entries(ruleDraft).filter(([, value]) => value !== '' && value != null))
    if (ruleDraft.rule_id) await categoriesApi.updateCostRule(ruleDraft.rule_id, payload)
    else await categoriesApi.createCostRule(payload)
    const group = ruleDraft.category_code
    ruleDialogOpen.value = false
    await loadAll()
    selectedGroupCode.value = group
    try {
      const run = await costManagementApi.createCostRecalculationRun({ mode: 'all_matching', filters: { rule_group: group }, reason: '成本规则维护后的影响预览' })
      notice.value = `规则已保存，并已生成影响预览${run.run_no ? `（${run.run_no}）` : ''}。系统不会在未确认时覆盖现有任务成本。`
    } catch {
      notice.value = '规则已保存；当前没有可生成影响预览的 SKU，现有成本未被修改。'
    }
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '规则保存失败。' }
  finally { savingRule.value = false }
}

async function loadCandidates() {
  try { candidates.value = (await costManagementApi.listUnboundCostRuleCandidates({ keyword: candidateKeyword.value, page: 1, page_size: 100 })).data || [] }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '待绑定款式加载失败。' }
}
async function bindCandidate(candidate: UnboundCostRuleCandidate) {
  if (!selectedGroup.value) return
  savingBinding.value = true
  try {
    await costManagementApi.createCostRuleBinding({ i_id_raw: candidate.i_id_raw || candidate.display_i_id || candidate.normalized_i_id, normalized_i_id: candidate.normalized_i_id, rule_group: selectedGroup.value.code, display_name: candidate.suggested_display_name || candidate.display_i_id || candidate.normalized_i_id, source: 'manual', is_active: true })
    await loadAll(); await loadCandidates(); notice.value = '款式已绑定，后续成本计算会优先使用这组规则。'
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '款式绑定失败。' }
  finally { savingBinding.value = false }
}
async function previewCost() {
  previewing.value = true; error.value = ''
  try { preview.value = await costManagementApi.previewCostRule({ rule_group: calculator.rule_group, width: calculator.width, height: calculator.height, area: calculator.area, quantity: calculator.quantity, process: calculator.process }) }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '成本试算失败。' }
  finally { previewing.value = false }
}

function runStatusLabel(status: string) {
  return ({ previewed: '等待确认', preview_failed: '预览失败', applying: '更新中', applied: '已更新', partially_applied: '部分更新', erp_syncing: '同步 ERP 中', erp_synced: 'ERP 已同步', partially_erp_synced: '部分已同步', erp_failed: 'ERP 同步失败', cancelled: '已取消' } as Record<string, string>)[status] || '处理中'
}
function runStatusTone(status: string) { return status.includes('failed') ? 'danger' : status.includes('partial') ? 'warning' : status.includes('synced') || status === 'applied' ? 'success' : 'neutral' }
function runItemStatusLabel(status: string) { return ({ previewed: '可更新', applied: '已更新', skipped: '已跳过', conflict: '有冲突', failed: '失败', erp_queued: '等待 ERP', erp_synced: 'ERP 已同步', erp_failed: 'ERP 失败' } as Record<string, string>)[status] || '处理中' }
function canApplyRun(status: string) { return status === 'previewed' }
function canSyncRun(status: string) { return status === 'applied' || status === 'partially_applied' }
function formatDateTime(value?: string) { if (!value) return '时间待确认'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '时间待确认' : date.toLocaleString('zh-CN', { hour12: false }) }
function costChangeLabel(oldValue?: number | null, nextValue?: number | null) { const oldLabel = typeof oldValue === 'number' ? `¥${oldValue.toFixed(2)}` : '未设置'; const nextLabel = typeof nextValue === 'number' ? `¥${nextValue.toFixed(2)}` : '需人工确认'; return `${oldLabel} → ${nextLabel}` }
async function openRun(id: number) {
  error.value = ''
  try { selectedRun.value = await costManagementApi.getCostRecalculationRun(id, { page: 1, page_size: 200 }); runDialogOpen.value = true }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '成本影响明细加载失败。' }
}
async function applyRun(id: number) {
  runActionBusy.value = id; error.value = ''
  try { const result = await costManagementApi.applyCostRecalculationRun(id); selectedRun.value = result.run; notice.value = '已更新确认范围内的任务与 SKU 成本。ERP 尚未同步，需要单独确认。'; await loadAll() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '成本更新失败，请核对冲突后重试。' }
  finally { runActionBusy.value = null }
}
async function syncRunERP(id: number) {
  runActionBusy.value = id; error.value = ''
  try { const result = await costManagementApi.syncCostRecalculationRunERP(id); selectedRun.value = result.run; notice.value = '成本已进入 ERP 同步队列，可在更新记录中继续查看结果。'; await loadAll() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : 'ERP 成本同步失败，请稍后重试。' }
  finally { runActionBusy.value = null }
}

onMounted(async () => { await loadAll(); await loadCandidates() })
</script>

<style scoped>
.cost-manager-page{display:grid;gap:1rem}.eyebrow{margin:0;color:rgb(var(--yb-brand));font-size:.7rem;font-weight:900;letter-spacing:.11em}.yb-page-title{margin:.25rem 0 0}.yb-page-subtitle{max-width:52rem}.header-actions{display:flex;gap:.6rem}.primary,.secondary,.text-button,.close{min-height:2.5rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;padding:0 .9rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.primary{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-weight:800}.text-button{min-height:2rem;border:0;color:rgb(var(--yb-brand));font-weight:800}.cost-layout{display:grid;grid-template-columns:15rem minmax(0,1fr) 20rem;gap:1rem;align-items:start}.rule-groups,.rule-workspace,.calculator,.operations-panel{border:1px solid rgb(var(--yb-border));border-radius:1rem;background:rgb(var(--yb-surface))}.rule-groups{overflow:hidden}.rule-groups>header{display:flex;justify-content:space-between;padding:.9rem 1rem;border-bottom:1px solid rgb(var(--yb-border));font-size:.75rem;color:rgb(var(--yb-text-muted))}.rule-groups>button{width:100%;display:flex;align-items:center;justify-content:space-between;gap:.5rem;border:0;border-bottom:1px solid rgb(var(--yb-border));padding:.8rem 1rem;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer}.rule-groups>button.active{background:rgb(var(--yb-brand-soft));box-shadow:inset .2rem 0 rgb(var(--yb-brand))}.rule-groups>button span{min-width:0;display:grid;gap:.2rem}.rule-groups small{overflow:hidden;color:rgb(var(--yb-text-muted));font-size:.65rem;text-overflow:ellipsis}.rule-workspace{min-width:0;padding:1rem;display:grid;gap:1rem}.workspace-heading,.binding-panel>header,.operations-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem}.workspace-heading h2,.calculator h2,.binding-panel h3,.operations-heading h2{margin:.2rem 0}.workspace-heading p,.binding-panel p,.calculator header span,.operations-heading p{margin:.25rem 0 0;color:rgb(var(--yb-text-muted));font-size:.78rem}.rule-list{display:grid;gap:.6rem}.rule-card{display:grid;grid-template-columns:2.3rem minmax(0,1fr) auto;align-items:center;gap:.8rem;padding:.8rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;background:rgb(var(--yb-surface-soft))}.rule-card.inactive{opacity:.62}.rule-icon{width:2.3rem;height:2.3rem;display:grid;place-items:center;border-radius:.65rem;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-weight:900}.rule-copy{min-width:0}.rule-copy>div{display:flex;align-items:center;gap:.55rem}.rule-copy span{border-radius:999px;padding:.18rem .4rem;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text));font-size:.62rem}.rule-copy p{margin:.25rem 0;color:rgb(var(--yb-text));font-size:.76rem}.rule-copy small{color:rgb(var(--yb-text-muted));font-size:.65rem}.binding-panel{display:grid;gap:.8rem;padding-top:1rem;border-top:1px solid rgb(var(--yb-border))}.binding-chips{display:flex;flex-wrap:wrap;gap:.5rem}.binding-chips>span{display:grid;gap:.15rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.5rem .65rem;background:rgb(var(--yb-surface-soft))}.binding-chips small{color:rgb(var(--yb-text-muted));font-size:.62rem}.calculator{position:sticky;top:1rem;padding:1rem;display:grid;gap:.8rem}.calculator label,.form-grid label{display:grid;gap:.35rem;color:rgb(var(--yb-text-muted));font-size:.72rem}.calculator input,.calculator select,.form-grid input,.form-grid select,.form-grid textarea,.dialog-search input{width:100%;min-height:2.5rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.55rem .7rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:inherit}.field-pair{display:grid;grid-template-columns:1fr 1fr;gap:.6rem}.calculate-button{width:100%}.preview-result{display:grid;gap:.25rem;border-radius:.8rem;padding:.8rem;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.preview-result.warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.preview-result span{font-size:.68rem}.preview-result strong{font-size:1.4rem}.preview-result p{margin:0;font-size:.7rem;line-height:1.5}.operations-panel{display:grid;gap:1rem;padding:1rem}.cost-health{display:flex;gap:.6rem}.cost-health span,.run-summary span{display:grid;gap:.12rem;border-radius:.65rem;padding:.48rem .65rem;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text-muted));font-size:.64rem}.cost-health b,.run-summary b{color:rgb(var(--yb-text));font-size:.86rem}.cost-health .warning,.run-summary .warning{color:rgb(var(--yb-warning-text))}.run-list{display:grid;gap:.6rem}.run-card{display:grid;grid-template-columns:minmax(13rem,.8fr) minmax(18rem,1fr) auto;align-items:center;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;padding:.75rem}.run-identity{display:grid;grid-template-columns:auto 1fr;align-items:center;gap:.3rem .55rem}.run-identity small{grid-column:2;color:rgb(var(--yb-text-muted));font-size:.66rem}.run-status{border-radius:999px;padding:.25rem .45rem;background:rgb(var(--yb-surface-soft));font-size:.62rem;font-style:normal}.run-status.success{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.run-status.warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.run-status.danger{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.run-summary,.run-actions{display:flex;gap:.45rem}.run-actions{justify-content:flex-end}.run-dialog{width:min(56rem,100%)}.run-detail-list{min-height:0;display:grid;align-content:start;gap:.45rem;padding:1rem 1.2rem;overflow:auto}.run-detail-list article{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.65rem}.run-detail-list article>div{display:grid;gap:.2rem}.run-detail-list small{color:rgb(var(--yb-text-muted))}.run-detail-list em{font-size:.68rem;font-style:normal;color:rgb(var(--yb-text-muted))}.message{display:flex;align-items:center;justify-content:space-between;gap:1rem;border-radius:.75rem;padding:.7rem 1rem}.message.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.message.notice{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.message button{border:0;background:transparent;color:inherit;cursor:pointer;text-decoration:underline}.empty-small,.empty-large{padding:1.2rem;color:rgb(var(--yb-text-muted));text-align:center}.empty-inline{color:rgb(var(--yb-text-muted))}.modal-layer{position:fixed;inset:0;z-index:100;display:grid;place-items:center;padding:1rem}.modal-mask{position:absolute;inset:0;border:0;background:rgb(var(--yb-overlay-night)/.48)}.modal-card{position:relative;width:min(45rem,100%);max-height:min(52rem,92vh);display:grid;grid-template-rows:auto minmax(0,1fr) auto;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:1rem;background:rgb(var(--yb-surface));box-shadow:0 1.5rem 4rem rgb(var(--yb-shadow)/.25)}.modal-card>header,.modal-card>footer{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:1rem 1.2rem;border-bottom:1px solid rgb(var(--yb-border))}.modal-card>header h2{margin:.2rem 0 0}.modal-card>footer{justify-content:flex-end;border-top:1px solid rgb(var(--yb-border));border-bottom:0}.close{width:2.5rem;padding:0;font-size:1.3rem}.form-grid{min-height:0;display:grid;grid-template-columns:1fr 1fr;gap:.8rem;padding:1.2rem;overflow:auto}.span-2{grid-column:1/-1}.switch-row{grid-template-columns:auto 1fr;align-items:center}.switch-row input{width:auto;min-height:auto}.binding-dialog{grid-template-rows:auto auto minmax(0,1fr)}.dialog-search{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.6rem;padding:1rem 1.2rem}.candidate-list{min-height:0;display:grid;align-content:start;gap:.5rem;padding:0 1.2rem 1.2rem;overflow:auto}.candidate-list article{display:flex;align-items:center;justify-content:space-between;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;padding:.7rem}.candidate-list article>span{display:grid;gap:.2rem}.candidate-list small{color:rgb(var(--yb-text-muted))}@media(max-width:1120px){.cost-layout{grid-template-columns:13rem minmax(0,1fr)}.calculator{position:static;grid-column:1/-1;grid-template-columns:repeat(2,minmax(0,1fr))}.calculator>header,.preview-result,.calculate-button{grid-column:1/-1}.run-card{grid-template-columns:1fr}.run-actions{justify-content:flex-start}}@media(max-width:760px){.cost-layout{grid-template-columns:1fr}.rule-groups{display:flex;overflow:auto}.rule-groups>header{display:none}.rule-groups>button{min-width:10rem;border-right:1px solid rgb(var(--yb-border));border-bottom:0}.calculator{grid-column:auto;grid-template-columns:1fr}.calculator>*{grid-column:auto}.header-actions,.cost-health,.run-summary,.run-actions{width:100%;flex-wrap:wrap}.header-actions button{flex:1}.form-grid{grid-template-columns:1fr}.span-2{grid-column:auto}.field-pair{grid-template-columns:1fr 1fr}.operations-heading{display:grid}.run-detail-list article{grid-template-columns:1fr}.run-detail-list article span,.run-detail-list article em{justify-self:start}}
.cost-manager-page{min-width:0;max-width:100%;overflow-x:hidden}
.cost-layout{grid-template-columns:minmax(13rem,16rem) minmax(0,1fr)}
.rule-groups{max-height:min(46rem,calc(100vh - 9rem));overflow:auto}
.calculator{position:static;grid-column:1/-1;grid-template-columns:minmax(12rem,1.2fr) minmax(12rem,1fr) minmax(16rem,1.3fr) minmax(12rem,1fr);align-items:end}
.calculator>header{align-self:start}.calculator .calculate-button{align-self:end}.calculator .preview-result{min-height:4.2rem}
.calculator input,.calculator select,.form-grid input,.form-grid select,.form-grid textarea,.dialog-search input{box-sizing:border-box;min-width:0}
.operations-panel,.run-card,.run-summary{min-width:0}
.run-card{grid-template-columns:minmax(11rem,.75fr) minmax(0,1fr) auto}
.run-summary{flex-wrap:wrap}
@media(max-width:1320px){.calculator{grid-template-columns:repeat(2,minmax(0,1fr))}.calculator>header,.preview-result{grid-column:1/-1}.calculate-button{grid-column:auto}.run-card{grid-template-columns:1fr}.run-actions{justify-content:flex-start}}
@media(max-width:900px){.cost-layout{grid-template-columns:1fr}.rule-groups{display:flex;max-height:none;overflow:auto}.rule-groups>header{display:none}.rule-groups>button{min-width:11rem;border-right:1px solid rgb(var(--yb-border));border-bottom:0}.calculator{grid-column:auto}}
@media(max-width:620px){.calculator{grid-template-columns:1fr}.calculator>header,.preview-result,.calculate-button{grid-column:auto}.workspace-heading,.binding-panel>header{display:grid}.field-pair{grid-template-columns:1fr}.cost-health,.run-summary,.run-actions{width:100%;flex-wrap:wrap}}
</style>
