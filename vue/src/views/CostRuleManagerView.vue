<template>
  <main class="cost-manager-page">
    <header class="yb-page-surface yb-page-header-row">
      <div class="yb-page-heading-copy">
        <h1 class="yb-page-title">成本规则</h1>
        <p class="yb-page-subtitle">选择方案，维护价格与款式，查看关联 SKU。</p>
      </div>
      <div class="header-actions">
        <button class="secondary" :disabled="loading" @click="loadAll">{{ loading ? '刷新中…' : '刷新数据' }}</button>
        <button class="primary" @click="openRuleEditor()">新增方案</button>
      </div>
    </header>

    <div v-if="error" class="message error" role="alert">{{ error }}<button @click="loadAll">重试</button></div>
    <div v-if="notice" class="message notice" role="status">{{ notice }}</div>

    <nav class="workspace-tabs" aria-label="成本工作区"><button v-for="tab in ['方案与绑定','绑定检查','更新记录']" :key="tab" :class="{active:workspaceTab===tab}" @click="workspaceTab=tab">{{tab}}</button></nav>
    <section v-show="workspaceTab==='方案与绑定'" class="cost-layout">
      <aside class="rule-groups" aria-label="成本规则分组">
        <label class="group-search">搜索材质 / 款式编码<input v-model.trim="groupSearch" placeholder="输入编码或名称" /></label>
        <label class="group-search">显示范围<select v-model="groupScope"><option value="bound">已绑定款式的方案</option><option value="all">全部方案（含未绑定）</option></select></label>
        <header><span>计价方案</span><b>{{ filteredGroups.length }}</b></header>
        <button
          v-for="group in filteredGroups"
          :key="group.code"
          :class="{ active: selectedGroupCode === group.code }"
          @click="selectedGroupCode = group.code"
        >
          <span><strong>{{ group.name }}</strong><small>{{ activeBindingCount(group.code) }} 个款式</small></span>
        </button>
        <div v-if="!loading && !filteredGroups.length" class="empty-small">没有匹配的方案。可切换“全部方案”查看未绑定项。</div>
      </aside>

      <section class="rule-workspace">
        <header class="workspace-heading">
          <div>
            <h2>{{ selectedGroup?.name || '成本计算规则' }}</h2>
            <p>{{ selectedGroup ? `${selectedBindings.length} 个款式已绑定此方案` : '从左侧选择方案查看。' }}</p>
          </div>
          <button v-if="selectedGroup && !selectedModel" class="secondary" @click="openRuleEditor(undefined, selectedGroup.code)">添加收费项</button>
        </header>

        <section v-if="selectedModel || modelEditing" class="model-workspace">
          <button class="secondary" @click="startModel()">编辑方案价格</button>
          <template v-if="modelEditing">
            <label>方案名称<input v-model.trim="modelName" /></label>
            <CostModelEditor v-model="modelJSON" />
            <p>保存后供明确绑定的款式使用；历史金额须通过更新记录单独重算。</p>
            <button class="primary" :disabled="savingModel" @click="saveModel">{{ savingModel?'保存中…':'保存方案' }}</button>
          </template>
        </section>
        <div v-if="selectedGroup" class="rule-list">
          <article v-for="rule in currentRules" :key="rule.rule_id" class="rule-card">
            <div class="rule-icon" aria-hidden="true">{{ ruleTypeIcon(rule.rule_type) }}</div>
            <div class="rule-copy">
              <div><strong>{{ rule.rule_name || '未命名规则' }}</strong><span>{{ ruleStateLabel(rule) }}</span></div>
              <p>{{ ruleSummary(rule) }}</p>
            </div>
            <button class="text-button" @click="rule.rule_type==='cost_model'?startModel(rule):openRuleEditor(rule)">编辑</button>
          </article>
          <div v-if="!currentRules.length" class="empty-large">此方案暂无启用的价格。</div>
          <details v-if="historicalRules.length"><summary>已停用或历史价格（{{historicalRules.length}}）</summary><p v-for="rule in historicalRules" :key="rule.rule_id">{{rule.rule_name}}：{{ruleSummary(rule)}}</p></details>
        </div>

        <section v-if="selectedGroup" class="binding-panel">
          <header>
            <div><h3>绑定款式</h3><p>绑定后，新计价使用本方案；已有金额不会自动改变。</p></div>
            <button class="secondary" @click="bindingDialogOpen = true">绑定新款式</button>
          </header>
          <label class="binding-search">查找已绑定款式<input v-model.trim="bindingSearch" placeholder="款式编码或名称" /></label>
          <div class="binding-chips">
            <span v-for="binding in filteredBindings" :key="binding.id"><b>{{ binding.i_id_raw || binding.normalized_i_id }}</b><small>{{ binding.display_name || '已绑定' }}</small><button class="text-button" :disabled="savingBinding" @click="unbind(binding)">{{unbindID===binding.id?'确认解除':'解除绑定'}}</button></span>
            <span v-if="!selectedBindings.length" class="empty-inline">还没有款式绑定到此组。</span>
          </div>
        </section>
      </section>

      <CostBoundSKUList :group="selectedGroupCode" :revision="dataRevision" />
      <aside v-if="calculatorOpen" id="cost-calculator" class="calculator floating-calculator" aria-label="成本试算器" @keydown.esc="calculatorOpen=false">
        <header><h2>成本试算</h2><button class="text-button" @click="calculatorOpen=false">收起试算</button><span>只试算，不保存或修改成本。</span></header>
        <label>试算方案<select v-model="calculator.rule_group" :disabled="modelEditing"><option value="">请选择</option><option v-for="group in ruleGroups" :key="group.code" :value="group.code">{{ group.name }}</option></select></label>
        <small v-if="modelEditing">正在试算中间栏尚未保存的价格。</small>
        <CostInputFields v-if="calculatorUsesModel || modelEditing" v-model="modelInput" />
        <template v-else>
        <div class="field-pair"><label>宽（米）<input v-model.number="calculator.width" type="number" min="0" step="0.01" /></label><label>高（米）<input v-model.number="calculator.height" type="number" min="0" step="0.01" /></label></div>
        <div class="field-pair"><label>面积（㎡）<input v-model.number="calculator.area" type="number" min="0" step="0.001" /></label><label>数量<input v-model.number="calculator.quantity" type="number" min="1" step="1" /></label></div>
        <label>特殊工艺<input v-model.trim="calculator.process" placeholder="例如：覆膜、开槽" /></label>
        </template>
        <button class="primary calculate-button" :disabled="previewing || !calculator.rule_group" @click="previewCost">{{ previewing ? '计算中…' : '开始试算' }}</button>
        <div class="preview-result" :class="{ warning: preview?.requires_manual_review }">
          <span>预计成本</span>
          <strong>{{ previewCostLabel }}</strong>
          <p>{{ previewExplanation }}</p>
          <div v-for="(line,i) in preview?.calculation?.lines || []" :key="i">{{line.name}}：{{line.quantity}} {{line.unit}} × {{line.unit_price}} × {{line.multiplier}} = {{line.amount}}</div>
        </div>
      </aside>
    </section>

    <section v-show="workspaceTab==='绑定检查'" class="mapping-diagnostics" aria-labelledby="mapping-diagnostics-title">
      <header>
        <div>
          <p class="eyebrow">款式编码与计价规则</p>
          <h2 id="mapping-diagnostics-title">检查款式绑定</h2>
          <p>检查哪些款式还没有选定计价方案。历史价格仅供参考，绑定前请核对。</p>
        </div>
        <button class="secondary" @click="bindingDialogOpen = true">处理未绑定款式</button>
      </header>
      <div class="diagnostic-grid">
        <article><span>已明确绑定</span><strong>{{ bindings.filter((item) => item.is_active).length }}</strong><small>款式编码直接命中规则组</small></article>
        <article><span>当前加载的唯一建议</span><strong>{{ candidateStats.unique }}</strong><small>可核对后绑定</small></article>
        <article class="warning"><span>需要选择方案</span><strong>{{ candidateStats.conflict }}</strong><small>曾使用过不同价格，请核对后选择</small></article>
        <article class="danger"><span>当前加载的无证据项</span><strong>{{ candidateStats.unmatched }}</strong><small>需补充明确规则</small></article>
      </div>
    </section>

    <section v-show="workspaceTab==='更新记录'" class="operations-panel" aria-labelledby="cost-operations-title">
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
      <div class="exact-run-builder">
        <label><span>指定 SKU 生成影响预览</span><textarea v-model="explicitSKUText" rows="2" placeholder="输入 SKU，支持空格、逗号或换行分隔" /></label>
        <button class="secondary" :disabled="creatingExplicitRun || !explicitSKUCodes.length" @click="createExplicitRun">{{ creatingExplicitRun ? '生成中…' : `预览 ${explicitSKUCodes.length || ''} 个 SKU` }}</button>
      </div>
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
            <button v-if="canCancelRun(run.status)" class="secondary" :disabled="runActionBusy === run.id" @click="cancelRun(run.id)">取消预览</button>
            <button v-if="canApplyRun(run.status)" class="primary" :disabled="runActionBusy === run.id" @click="applyRun(run.id)">确认更新</button>
            <button v-if="canSyncRun(run.status)" class="primary" :disabled="runActionBusy === run.id" @click="syncRunERP(run.id)">同步到 ERP</button>
          </div>
        </article>
      </div>
    </section>

    <button class="calculator-toggle" :aria-expanded="calculatorOpen" aria-controls="cost-calculator" @click="calculatorOpen=!calculatorOpen">{{calculatorOpen?'收起':'试算'}}</button>
    <Teleport to="body">
      <div v-if="ruleDialogOpen" class="modal-layer" @keydown.esc="closeRuleEditor">
        <button class="modal-mask" aria-label="关闭规则编辑" @click="closeRuleEditor" />
        <form class="modal-card" @submit.prevent="saveRule">
          <header><div><p class="eyebrow">{{ ruleDraft.rule_id ? '调整现有规则' : '新增计算规则' }}</p><h2>{{ ruleDraft.rule_id ? '编辑规则' : '新建规则' }}</h2></div><button type="button" class="close" @click="closeRuleEditor">×</button></header>
          <div class="form-grid">
            <label class="span-2">规则名称<input v-model.trim="ruleDraft.rule_name" required placeholder="例如：KT 板面积计价" /></label>
            <label>适用材质 / 方案<select v-model="ruleDraft.category_code" required :disabled="!!ruleDraft.rule_id"><option value="">请选择</option><option v-for="group in availableCategories" :key="group.code" :value="group.code">{{group.name}}</option></select></label>
            <label v-if="ruleDraft.rule_type!=='cost_model'">计算方式<select v-model="ruleDraft.rule_type"><option v-for="item in ruleTypes" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
            <CostModelEditor v-if="ruleDraft.rule_type==='cost_model'" v-model="newModelJSON" hide-material class="span-2" />
            <template v-if="ruleDraft.rule_type==='fixed_unit_price'"><label>每平方米价格（元）<input v-model.number="ruleDraft.base_price" required type="number" min="0" step="0.01" /></label><label>价格调整倍数<input v-model.number="ruleDraft.tax_multiplier" type="number" min="0.0001" step="0.01" placeholder="不调整填 1" /></label></template>
            <label v-if="ruleDraft.rule_type==='minimum_billable_area'">不足多少平方米按此面积收费<input v-model.number="ruleDraft.min_area" required type="number" min="0" step="0.001" /></label>
            <template v-if="ruleDraft.rule_type==='area_threshold_surcharge'"><label>面积小于（平方米）<input v-model.number="ruleDraft.area_threshold" required type="number" min="0" step="0.001" /></label><label>每平方米另加（元）<input v-model.number="ruleDraft.surcharge_amount" required type="number" min="0" step="0.01" /></label></template>
            <template v-if="ruleDraft.rule_type==='special_process_surcharge'"><label>收费工艺名称<input v-model.trim="ruleDraft.special_process_keyword" required placeholder="例如：开槽" /></label><label>该工艺另加（元）<input v-model.number="ruleDraft.special_process_price" required type="number" min="0" step="0.01" /></label></template>
            <template v-if="ruleDraft.rule_type==='size_based_formula'">
              <template v-if="formulaKind==='print'"><label>单面价格（元/张）<input v-model.number="printDraft.single" required type="number" min="0" step="0.001" /></label><label>双面价格（元/张）<input v-model.number="printDraft.double" required type="number" min="0" step="0.001" /></label></template>
              <template v-else-if="formulaKind==='keyword'"><label>适用产品关键词<input v-model.trim="keywordDraft.keyword" required /></label><label>每平方米价格（元）<input v-model.number="keywordDraft.price" required type="number" min="0" step="0.001" /></label></template>
              <p v-else class="span-2">{{ruleDescription(ruleDraft)}}。此特殊计价方式暂不支持修改价格；可维护名称与启用状态，或另建明确单价的方案。</p>
            </template>
            <label class="switch-row"><input v-model="ruleDraft.is_active" type="checkbox" /> 当前启用</label>
            <label v-if="ruleDraft.source?.endsWith('_sample')" class="switch-row span-2"><input v-model="confirmedPrice" type="checkbox" /> 已核对价格，允许用于新 SKU 自动计价</label>
            <label class="span-2">维护说明<textarea v-model.trim="ruleDraft.governance_note" rows="3" placeholder="说明本次调整原因，方便后续追溯。" /></label>
          </div>
          <footer><button type="button" class="secondary" @click="closeRuleEditor">取消</button><button class="primary" :disabled="savingRule">{{ savingRule ? '保存中…' : '保存价格' }}</button></footer>
        </form>
      </div>

      <div v-if="bindingDialogOpen" class="modal-layer" @keydown.esc="bindingDialogOpen = false">
        <button class="modal-mask" aria-label="关闭款式绑定" @click="bindingDialogOpen = false" />
        <section class="modal-card binding-dialog" role="dialog" aria-modal="true" aria-labelledby="binding-title">
          <header><div><p class="eyebrow">款式与规则</p><h2 id="binding-title">绑定到“{{ selectedGroup?.name }}”</h2></div><button class="close" @click="bindingDialogOpen = false">×</button></header>
          <label class="dialog-search">搜索款式编码<input v-model.trim="candidateKeyword" placeholder="输入款式编码或名称" @keyup.enter="loadCandidates" /><button class="secondary" @click="loadCandidates">搜索</button></label>
          <div class="candidate-list">
            <article v-for="style in erpStyles" :key="`erp-${style.i_id}`"><span><strong>{{style.i_id}}</strong><small>{{style.label||style.category_name||'ERP 款式'}} · {{boundStyleName(style.i_id)}}</small></span><button class="primary" :disabled="savingBinding || bindingForStyle(style.i_id)?.rule_group===selectedGroupCode" @click="bindStyle(style.i_id)">{{pendingStyle===style.i_id?'确认改绑到本方案':bindingForStyle(style.i_id)?'改绑':'绑定'}}</button></article>
            <article v-for="candidate in candidates" :key="candidate.normalized_i_id">
              <span>
                <strong>{{ candidate.i_id_raw || candidate.display_i_id || candidate.erp_i_id || candidate.product_i_id || candidate.normalized_i_id }}</strong>
                <small>{{ candidateEvidence(candidate) }}</small>
                <small>{{ candidate.match_count || 0 }} 个 SKU · 示例 {{ candidate.example_sku_code || '无' }}<template v-if="typeof candidate.average_cost_price === 'number'"> · 平均系统成本 ¥{{ candidate.average_cost_price.toFixed(2) }}</template></small>
              </span>
              <button class="primary" :disabled="savingBinding" @click="bindCandidate(candidate)">{{ candidateConfidence(candidate) === 'conflict' ? '确认绑定此组' : '绑定' }}</button>
            </article>
            <div v-if="!candidates.length && !erpStyles.length" class="empty-large">没有找到待绑定款式。</div>
          </div>
        </section>
      </div>

      <div v-if="runDialogOpen" class="modal-layer" @keydown.esc="runDialogOpen = false">
        <button class="modal-mask" aria-label="关闭成本影响明细" @click="runDialogOpen = false" />
        <section class="modal-card run-dialog" role="dialog" aria-modal="true" aria-labelledby="run-dialog-title">
          <header><div><p class="eyebrow">应用前核对</p><h2 id="run-dialog-title">{{ selectedRun?.run_no || '成本影响明细' }}</h2></div><button class="close" @click="runDialogOpen = false">×</button></header>
          <div class="run-detail-list">
            <article v-for="item in selectedRun?.items || []" :key="item.id">
              <div><strong>{{ item.sku_code || '未命名 SKU' }}</strong><small>{{ item.task_no || '未关联任务号' }}</small><small v-if="item.conflict_reason || item.skip_reason" class="warning">{{ item.conflict_reason || item.skip_reason }}</small></div>
              <span>{{ costChangeLabel(item.old_cost_price, item.new_cost_price) }}</span>
              <em>{{ runItemStatusLabel(item.status) }}</em>
            </article>
            <div v-if="!(selectedRun?.items?.length)" class="empty-large">当前预览没有可更新的 SKU。</div>
          </div>
          <footer>
            <button class="secondary" @click="runDialogOpen = false">关闭</button>
            <button v-if="selectedRun && canCancelRun(selectedRun.status)" class="secondary" :disabled="runActionBusy === selectedRun.id" @click="cancelRun(selectedRun.id)">取消本次预览</button>
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
import CostModelEditor from '@/components/cost/CostModelEditor.vue'
import CostInputFields from '@/components/cost/CostInputFields.vue'
import CostBoundSKUList from '@/components/cost/CostBoundSKUList.vue'
import '@/components/cost/cost-manager-workspace.css'
import {ruleDescription,printPrices,keywordPrice} from '@/domain/cost-rule-presentation'
import {emptyCostModel,emptyCostInput} from '@/domain/cost-model'
import { categoriesApi } from '@/services/api/categoriesApi'
import { erpApi } from '@/services/api/erpApi'
import {
  costManagementApi,
  type CostRecalculationRun,
  type CostRuleBinding,
  type CostRulePreviewResponse,
  type ProductCostDashboardResponse,
  type UnboundCostRuleCandidate,
} from '@/services/api/costManagementApi'

interface CostRuleRow {
  source?: string
  governance_status?: string
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
  formula_expression?: string
  supersedes_rule_id?: number | null
  priority?: number
  is_active?: boolean
  governance_note?: string
}

interface RuleDraft extends Partial<CostRuleRow> { rule_name: string; category_code: string; rule_type: string; is_active: boolean; priority: number }

const ruleTypes = [
  { value: 'cost_model', label: '按用料和工艺收费' },
  { value: 'fixed_unit_price', label: '按平方米收费' },
  { value: 'area_threshold_surcharge', label: '小面积另加费用' },
  { value: 'minimum_billable_area', label: '设置最低收费面积' },
  { value: 'size_based_formula', label: '单双面 / 特定规格价格' },
  { value: 'special_process_surcharge', label: '特殊工艺加价' },
  { value: 'manual_quote', label: '人工报价' },
]
const rules = ref<CostRuleRow[]>([])
const bindings = ref<CostRuleBinding[]>([])
const candidates = ref<UnboundCostRuleCandidate[]>([])
const erpStyles=ref<Array<{i_id:string;label?:string;category_name?:string}>>([]),pendingStyle=ref(''),unbindID=ref<number|null>(null)
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
const creatingExplicitRun = ref(false)
const explicitSKUText = ref('')
const candidateKeyword = ref('')
const preview = ref<CostRulePreviewResponse | null>(null)
const ruleDraft = reactive<RuleDraft>(emptyRuleDraft())
const calculator = reactive({ rule_group: '', width: 1, height: 1, area: null as number | null, quantity: 1, process: '' })
const workspaceTab=ref('方案与绑定')
const groupSearch=ref('')
const groupScope=ref('bound'),bindingSearch=ref(''),calculatorOpen=ref(false),dataRevision=ref(0)
const newModelJSON=ref(JSON.stringify(emptyCostModel())),confirmedPrice=ref(false)
const materialOptions=ref<Array<{code:string;name:string}>>([])
const availableCategories=computed(()=>[...new Map([...ruleGroups.value.map(g=>({code:g.code,name:g.name})),...materialOptions.value].map(g=>[g.code,g])).values()])
const formulaKind=ref('print'),printDraft=reactive({single:0,double:0}),keywordDraft=reactive({keyword:'',price:0})
const modelEditing=ref(false),savingModel=ref(false),modelName=ref(''),modelJSON=ref(JSON.stringify(emptyCostModel()))
const modelInput=ref(emptyCostInput())
const modelRuleID=ref<number|undefined>()
const isEffectiveModel=(r:CostRuleRow)=>r.rule_type==='cost_model'&&r.is_active&&(!r.governance_status||r.governance_status==='effective')
const selectedModel=computed(()=>selectedGroup.value?.rules.filter(isEffectiveModel).sort((a,b)=>(b.rule_version||1)-(a.rule_version||1))[0])
const calculatorUsesModel=computed(()=>rules.value.some(r=>r.category_code===calculator.rule_group&&isEffectiveModel(r)))
function activeBindingCount(code:string){return bindings.value.filter(b=>b.rule_group===code&&b.is_active).length}
function effectiveRules(items:CostRuleRow[]){const active=items.filter(r=>r.is_active!==false&&(!r.governance_status||r.governance_status==='effective'));const replaced=new Set(active.map(r=>r.supersedes_rule_id).filter(Boolean));return active.filter(r=>!replaced.has(r.rule_id))}
const filteredGroups=computed(()=>ruleGroups.value.filter(g=>(groupScope.value==='all'||(activeBindingCount(g.code)>0&&effectiveRules(g.rules).length>0))&&(!groupSearch.value||`${g.name} ${g.code} ${bindings.value.filter(b=>b.is_active&&b.rule_group===g.code).map(b=>b.i_id_raw).join(' ')}`.toLowerCase().includes(groupSearch.value.toLowerCase()))))
const currentRules=computed(()=>effectiveRules(selectedGroup.value?.rules||[]))
const historicalRules=computed(()=>selectedGroup.value?.rules.filter(r=>!currentRules.value.includes(r))||[])
const filteredBindings=computed(()=>selectedBindings.value.filter(b=>`${b.i_id_raw} ${b.display_name}`.toLowerCase().includes(bindingSearch.value.toLowerCase())))
watch(selectedGroupCode,()=>{modelEditing.value=false;preview.value=null})
watch([modelJSON,modelInput,calculator],()=>{preview.value=null},{deep:true})
function ruleStateLabel(rule:CostRuleRow){if(!rule.is_active)return '已停用';if(rule.governance_status==='expired'||rule.governance_status==='scheduled')return '暂未使用';if(rule.source?.endsWith('_sample'))return '价格待核定';return '使用中'}
function startModel(rule?:CostRuleRow){
  const existing=rule||selectedModel.value;modelRuleID.value=existing?.rule_id;modelName.value=existing?.rule_name||`${selectedGroup.value?.name||''}计价方案`
  const m=emptyCostModel();m.material=selectedGroup.value?.name||''
  const old=selectedGroup.value?.rules||[];const base=old.find(r=>r.rule_type==='fixed_unit_price');const minimum=old.find(r=>r.rule_type==='minimum_billable_area');const small=old.find(r=>r.rule_type==='area_threshold_surcharge')
  m.unit_price=base?.base_price||0;m.multiplier=base?.tax_multiplier||1;m.minimum=minimum?.min_area||0;m.small_area_threshold=small?.area_threshold||0;m.small_area_surcharge=small?.surcharge_amount||0
  if(!base)m.basis='manual'
  modelJSON.value=existing?.formula_expression||JSON.stringify(m);modelEditing.value=true
}
async function saveModel(){
  if(!selectedGroup.value)return;savingModel.value=true;error.value=''
  try {const payload={rule_name:modelName.value,category_code:selectedGroup.value.code,rule_type:'cost_model',formula_expression:modelJSON.value,is_active:true,priority:1,source:'admin_manual',governance_note:'通过统一方案编辑器保存；历史成本不自动覆盖'}
    if(modelRuleID.value)await categoriesApi.updateCostRule(modelRuleID.value,payload);else await categoriesApi.createCostRule(payload)
    await loadAll();modelEditing.value=false;notice.value='计价方案已保存，请在右侧试算。历史成本保持原值。'
  }catch(cause){error.value=cause instanceof Error?cause.message:'方案保存失败'}finally{savingModel.value=false}
}

const ruleGroups = computed(() => {
  const grouped = new Map<string, CostRuleRow[]>()
  for (const rule of rules.value) {
    const code = String(rule.category_code || 'UNASSIGNED').trim()
    grouped.set(code, [...(grouped.get(code) || []), rule])
  }
  return Array.from(grouped, ([code, groupRules]) => ({ code, name: groupDisplayName(groupRules, code), rules: groupRules.sort((a, b) => (a.priority || 0) - (b.priority || 0)) }))
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})
const selectedGroup = computed(() => ruleGroups.value.find((group) => group.code === selectedGroupCode.value) || ruleGroups.value[0] || null)
const selectedBindings = computed(() => bindings.value.filter((binding) => binding.rule_group === selectedGroup.value?.code && binding.is_active))
watch(filteredGroups,groups=>{if(!groups.some(g=>g.code===selectedGroupCode.value))selectedGroupCode.value=groups[0]?.code||''})
const candidateStats = computed(() => candidates.value.reduce((result, candidate) => {
  result[candidateConfidence(candidate)] += 1
  return result
}, { unique: 0, conflict: 0, unmatched: 0 }))
const previewCostLabel = computed(() => typeof preview.value?.estimated_cost === 'number' ? `¥ ${preview.value.estimated_cost.toFixed(2)}` : preview.value?.requires_manual_review ? '需要人工报价' : '—')
const previewExplanation = computed(() => preview.value?.explanation || (preview.value ? '计算完成。' : '填写尺寸后试算，不会修改任何任务或 ERP 数据。'))
const erpMismatchCount = computed(() => costDashboard.value.tags?.find((item) => item.code === 'erp_mismatch')?.count || 0)
const explicitSKUCodes = computed(() => [...new Set(explicitSKUText.value.split(/[\s,，;；]+/u).map((item) => item.trim()).filter(Boolean))])

watch(selectedGroup, (group) => {
  if (!group) return
  calculator.rule_group = group.code
  preview.value = null
}, { immediate: true })

function emptyRuleDraft(group = ''): RuleDraft { return { rule_name: '', category_code: group, rule_type: 'fixed_unit_price', is_active: true, priority: 100 } }
function replaceDraft(next: RuleDraft) { for (const key of Object.keys(ruleDraft)) delete (ruleDraft as Record<string, unknown>)[key]; Object.assign(ruleDraft, next) }
function groupDisplayName(groupRules: CostRuleRow[], code: string) {
  const family = groupRules.map((rule) => String(rule.product_family || '').trim().toLowerCase()).find(Boolean) || ''
  const familyLabel = ({ board: '板材制作', cloth: '布艺制作', photo: '照片与印刷', paper: '纸张印刷' } as Record<string, string>)[family]
  const ruleLabel = groupRules.map((rule) => String(rule.rule_name || '').replace(/(?:基础单价|最低计价面积|面积加价|小面积附加|工艺加价)$/u, '').trim()).find(Boolean)
  return ruleLabel || familyLabel || code
}
function ruleTypeIcon(type: string) { return ({ fixed_unit_price: '¥', area_threshold_surcharge: '㎡', minimum_billable_area: '▣', size_based_formula: '×', special_process_surcharge: '+', manual_quote: '人' } as Record<string, string>)[type] || '•' }
function ruleSummary(rule: CostRuleRow) {return ruleDescription(rule)}

async function loadAll() {
  loading.value = true; error.value = ''; notice.value = ''
  try {
    const [ruleResponse, bindingResponse, runResponse, dashboardResponse, candidateResponse] = await Promise.all([
      loadRulePages(),
      loadBindingPages(),
      costManagementApi.listCostRecalculationRuns({ page: 1, page_size: 20 }),
      costManagementApi.getCostDashboard(),
      costManagementApi.listUnboundCostRuleCandidates({ page: 1, page_size: 100 }),
    ])
    const body = ruleResponse.data as { data?: CostRuleRow[] }
    rules.value = body.data || []
    bindings.value = bindingResponse.data || []
    runs.value = runResponse.data || []
    costDashboard.value = dashboardResponse
    candidates.value = candidateResponse.data || []
    dataRevision.value++
    if (!selectedGroupCode.value || !filteredGroups.value.some((group) => group.code === selectedGroupCode.value)) selectedGroupCode.value = filteredGroups.value[0]?.code || ''
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成本规则加载失败。' }
  finally { loading.value = false }
}

function openRuleEditor(rule?: CostRuleRow, group = '') {
 if(!rule)void categoriesApi.list({page:1,page_size:100,is_active:true}).then(response=>{materialOptions.value=(response.data.data||[]).filter(c=>c.category_code).map(c=>({code:c.category_code!,name:c.display_name||c.category_name||c.category_code!}))}).catch(()=>{})
 replaceDraft(rule ? { ...rule, is_active: rule.is_active !== false, priority: rule.priority ?? 100 } : {...emptyRuleDraft(group||selectedGroupCode.value),rule_type:group?'fixed_unit_price':'cost_model'})
 newModelJSON.value=rule?.rule_type==='cost_model' ? (rule.formula_expression||JSON.stringify(emptyCostModel())) : JSON.stringify(emptyCostModel());confirmedPrice.value=false
 const print=printPrices(rule?.formula_expression),keyword=keywordPrice(rule?.formula_expression)
 formulaKind.value=print?'print':keyword?'keyword':rule?.formula_expression?'other':'print'
 Object.assign(printDraft,print||{single:0,double:0});Object.assign(keywordDraft,keyword||{keyword:'',price:0})
 ruleDialogOpen.value = true
}
function closeRuleEditor() { if (!savingRule.value) ruleDialogOpen.value = false }
async function saveRule() {
  savingRule.value = true; error.value = ''; notice.value = ''
  try {
    if(!ruleDraft.rule_id&&ruleDraft.rule_type==='cost_model'&&rules.value.some(r=>r.category_code===ruleDraft.category_code&&isEffectiveModel(r)))throw new Error('此材质已有计价方案，请关闭窗口后直接编辑现有方案，避免重复收费。')
    if(ruleDraft.rule_type==='cost_model'&&ruleDraft.source?.endsWith('_sample')&&!confirmedPrice.value)throw new Error('请先核对价格并勾选确认，再改为按用料和工艺收费。')
    if(ruleDraft.rule_type==='cost_model'){const model=JSON.parse(newModelJSON.value);model.material=availableCategories.value.find(g=>g.code===ruleDraft.category_code)?.name||model.material;ruleDraft.formula_expression=JSON.stringify(model)}
    if(ruleDraft.rule_type==='size_based_formula'&&formulaKind.value==='print')ruleDraft.formula_expression=`print_side:single=${printDraft.single},double=${printDraft.double}`
    if(ruleDraft.rule_type==='size_based_formula'&&formulaKind.value==='keyword')ruleDraft.formula_expression=`keyword_area_unit_price:${keywordDraft.keyword}=${keywordDraft.price}`
    if(confirmedPrice.value)ruleDraft.source='admin_manual'
    const payload = Object.fromEntries(Object.entries(ruleDraft).filter(([, value]) => value !== '' && value != null))
    if (ruleDraft.rule_id) await categoriesApi.updateCostRule(ruleDraft.rule_id, payload)
    else await categoriesApi.createCostRule(payload)
    const group = ruleDraft.category_code
    groupScope.value='all'
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
  try {
    const [old,erp]=await Promise.all([costManagementApi.listUnboundCostRuleCandidates({ keyword: candidateKeyword.value, page: 1, page_size: 100 }),erpApi.getIids({q:candidateKeyword.value,page:1,page_size:100})])
    candidates.value=old.data||[];erpStyles.value=erp.data?.data||[];pendingStyle.value=''
  }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '待绑定款式加载失败。' }
}
function bindingForStyle(iid:string){const normalized=iid.normalize('NFKC').replace(/\s/g,'').toUpperCase();return bindings.value.find(b=>b.is_active&&b.normalized_i_id===normalized)}
function boundStyleName(iid:string){const binding=bindingForStyle(iid);return binding?`已绑定：${ruleGroups.value.find(g=>g.code===binding.rule_group)?.name||binding.display_name}`:'尚未绑定'}
async function bindStyle(iid:string){
 const existing=bindingForStyle(iid);if(existing&&pendingStyle.value!==iid){pendingStyle.value=iid;return}
 if(!selectedGroup.value)return;savingBinding.value=true
 try{if(existing)await costManagementApi.updateCostRuleBinding(existing.id,{rule_group:selectedGroupCode.value});else await costManagementApi.createCostRuleBinding({i_id_raw:iid,rule_group:selectedGroupCode.value,display_name:iid,is_active:true,source:'manual'})
 await loadAll();pendingStyle.value='';notice.value='款式绑定已保存，右侧 SKU 列表已刷新。'}catch(e){error.value=e instanceof Error?e.message:'绑定失败'}finally{savingBinding.value=false}
}
async function unbind(binding:CostRuleBinding){if(unbindID.value!==binding.id){unbindID.value=binding.id;return}savingBinding.value=true;try{await costManagementApi.updateCostRuleBinding(binding.id,{is_active:false});groupScope.value='all';await loadAll();notice.value='已解除绑定，历史成本未修改。';unbindID.value=null}catch(e){error.value=e instanceof Error?e.message:'解除失败'}finally{savingBinding.value=false}}
async function loadRulePages(){let page=1;const rows:CostRuleRow[]=[];while(true){const response=await categoriesApi.listCostRules({page,page_size:100});const body=response.data as {data?:CostRuleRow[];pagination?:{total:number}};const batch=body.data||[];rows.push(...batch);if(batch.length<100||rows.length>=(body.pagination?.total??Infinity))break;page++}return {data:{data:rows}}}
async function loadBindingPages(){let page=1;const rows:CostRuleBinding[]=[];while(true){const response=await costManagementApi.listCostRuleBindings({page,page_size:100});const batch=response.data||[];rows.push(...batch);if(batch.length<100||rows.length>=(response.pagination?.total??Infinity))break;page++}return {data:rows}}
function candidateEvidence(candidate: UnboundCostRuleCandidate) {
  if (candidateConfidence(candidate) === 'unique') {const code=candidate.suggested_rule_group||candidate.suggested_rule_groups?.[0];return `历史使用方案：${ruleGroups.value.find(g=>g.code===code)?.name||'待核对'}`}
  if (candidateConfidence(candidate) === 'conflict') return '历史使用过多个方案，请核对后选择'
  return '尚无历史价格参考，请按实际材质选择方案'
}
function candidateConfidence(candidate: UnboundCostRuleCandidate): 'unique' | 'conflict' | 'unmatched' {
  if (candidate.mapping_confidence) return candidate.mapping_confidence
  if ((candidate.suggested_rule_groups?.length || 0) > 1) return 'conflict'
  if (candidate.suggested_rule_group || candidate.suggested_rule_groups?.length === 1) return 'unique'
  return 'unmatched'
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
  try { preview.value = await costManagementApi.previewCostRule({ rule_group: calculator.rule_group, model:modelEditing.value?JSON.parse(modelJSON.value):undefined, input:calculatorUsesModel.value||modelEditing.value?modelInput.value:undefined, width: calculator.width, height: calculator.height, area: calculator.area, quantity: calculator.quantity, process: calculator.process }) }
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
function canCancelRun(status: string) { return status === 'previewed' }
function formatDateTime(value?: string) { if (!value) return '时间待确认'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '时间待确认' : date.toLocaleString('zh-CN', { hour12: false }) }
function costChangeLabel(oldValue?: number | null, nextValue?: number | null) { const oldLabel = typeof oldValue === 'number' ? `¥${oldValue.toFixed(2)}` : '未设置'; const nextLabel = typeof nextValue === 'number' ? `¥${nextValue.toFixed(2)}` : '需人工确认'; return `${oldLabel} → ${nextLabel}` }
async function openRun(id: number) {
  error.value = ''
  try { selectedRun.value = await costManagementApi.getCostRecalculationRun(id, { page: 1, page_size: 200 }); runDialogOpen.value = true }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '成本影响明细加载失败。' }
}
async function createExplicitRun() {
  if (!explicitSKUCodes.value.length || creatingExplicitRun.value) return
  creatingExplicitRun.value = true; error.value = ''; notice.value = ''
  try {
    const run = await costManagementApi.createCostRecalculationRun({ mode: 'explicit', sku_codes: explicitSKUCodes.value, reason: '指定 SKU 成本修复预览' })
    explicitSKUText.value = ''
    await loadAll()
    await openRun(run.id)
    notice.value = `已生成 ${run.run_no || `#${run.id}`} 影响预览，确认前不会修改任务或 ERP 成本。`
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '指定 SKU 影响预览生成失败。' }
  finally { creatingExplicitRun.value = false }
}
async function applyRun(id: number) {
  runActionBusy.value = id; error.value = ''
  try { const result = await costManagementApi.applyCostRecalculationRun(id); selectedRun.value = result.run; await loadAll(); notice.value = '已更新确认范围内的任务与 SKU 成本。ERP 尚未同步，需要单独确认。' }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '成本更新失败，请核对冲突后重试。' }
  finally { runActionBusy.value = null }
}
async function cancelRun(id: number) {
  runActionBusy.value = id; error.value = ''
  try {
    const run = await costManagementApi.cancelCostRecalculationRun(id)
    if (selectedRun.value?.id === id) selectedRun.value = { ...selectedRun.value, ...run }
    await loadAll()
    notice.value = '本次成本影响预览已取消，不会再阻塞同一 SKU 的新预览。'
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成本影响预览取消失败，请稍后重试。' }
  finally { runActionBusy.value = null }
}
async function syncRunERP(id: number) {
  runActionBusy.value = id; error.value = ''
  try { const result = await costManagementApi.syncCostRecalculationRunERP(id); selectedRun.value = result.run; await loadAll(); notice.value = '成本已进入 ERP 同步队列，可在更新记录中继续查看结果。' }
  catch (cause) { error.value = cause instanceof Error ? cause.message : 'ERP 成本同步失败，请稍后重试。' }
  finally { runActionBusy.value = null }
}

onMounted(loadAll)
</script>

<style scoped>
.cost-manager-page{display:grid;gap:1rem}.eyebrow{margin:0;color:rgb(var(--yb-brand));font-size:.7rem;font-weight:900;letter-spacing:.11em}.yb-page-title{margin:.25rem 0 0}.yb-page-subtitle{max-width:52rem}.header-actions{display:flex;gap:.6rem}.primary,.secondary,.text-button,.close{min-height:2.5rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;padding:0 .9rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.primary{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-weight:800}.text-button{min-height:2rem;border:0;color:rgb(var(--yb-brand));font-weight:800}.cost-layout{display:grid;grid-template-columns:15rem minmax(0,1fr) 20rem;gap:1rem;align-items:start}.rule-groups,.rule-workspace,.calculator,.operations-panel{border:1px solid rgb(var(--yb-border));border-radius:1rem;background:rgb(var(--yb-surface))}.rule-groups{overflow:hidden}.rule-groups>header{display:flex;justify-content:space-between;padding:.9rem 1rem;border-bottom:1px solid rgb(var(--yb-border));font-size:.75rem;color:rgb(var(--yb-text-muted))}.rule-groups>button{width:100%;display:flex;align-items:center;justify-content:space-between;gap:.5rem;border:0;border-bottom:1px solid rgb(var(--yb-border));padding:.8rem 1rem;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer}.rule-groups>button.active{background:rgb(var(--yb-brand-soft));box-shadow:inset .2rem 0 rgb(var(--yb-brand))}.rule-groups>button span{min-width:0;display:grid;gap:.2rem}.rule-groups small{overflow:hidden;color:rgb(var(--yb-text-muted));font-size:.65rem;text-overflow:ellipsis}.rule-workspace{min-width:0;padding:1rem;display:grid;gap:1rem}.workspace-heading,.binding-panel>header,.operations-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem}.workspace-heading h2,.calculator h2,.binding-panel h3,.operations-heading h2{margin:.2rem 0}.workspace-heading p,.binding-panel p,.calculator header span,.operations-heading p{margin:.25rem 0 0;color:rgb(var(--yb-text-muted));font-size:.78rem}.rule-list{display:grid;gap:.6rem}.rule-card{display:grid;grid-template-columns:2.3rem minmax(0,1fr) auto;align-items:center;gap:.8rem;padding:.8rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;background:rgb(var(--yb-surface-soft))}.rule-card.inactive{opacity:.62}.rule-icon{width:2.3rem;height:2.3rem;display:grid;place-items:center;border-radius:.65rem;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-weight:900}.rule-copy{min-width:0}.rule-copy>div{display:flex;align-items:center;gap:.55rem}.rule-copy span{border-radius:999px;padding:.18rem .4rem;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text));font-size:.62rem}.rule-copy p{margin:.25rem 0;color:rgb(var(--yb-text));font-size:.76rem}.rule-copy small{color:rgb(var(--yb-text-muted));font-size:.65rem}.binding-panel{display:grid;gap:.8rem;padding-top:1rem;border-top:1px solid rgb(var(--yb-border))}.binding-chips{display:flex;flex-wrap:wrap;gap:.5rem}.binding-chips>span{display:grid;gap:.15rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.5rem .65rem;background:rgb(var(--yb-surface-soft))}.binding-chips small{color:rgb(var(--yb-text-muted));font-size:.62rem}.calculator{position:sticky;top:1rem;padding:1rem;display:grid;gap:.8rem}.calculator label,.form-grid label{display:grid;gap:.35rem;color:rgb(var(--yb-text-muted));font-size:.72rem}.calculator input,.calculator select,.form-grid input,.form-grid select,.form-grid textarea,.dialog-search input{width:100%;min-height:2.5rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.55rem .7rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:inherit}.field-pair{display:grid;grid-template-columns:1fr 1fr;gap:.6rem}.calculate-button{width:100%}.preview-result{display:grid;gap:.25rem;border-radius:.8rem;padding:.8rem;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.preview-result.warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.preview-result span{font-size:.68rem}.preview-result strong{font-size:1.4rem}.preview-result p{margin:0;font-size:.7rem;line-height:1.5}.operations-panel{display:grid;gap:1rem;padding:1rem}.cost-health{display:flex;gap:.6rem}.cost-health span,.run-summary span{display:grid;gap:.12rem;border-radius:.65rem;padding:.48rem .65rem;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text-muted));font-size:.64rem}.cost-health b,.run-summary b{color:rgb(var(--yb-text));font-size:.86rem}.cost-health .warning,.run-summary .warning{color:rgb(var(--yb-warning-text))}.run-list{display:grid;gap:.6rem}.run-card{display:grid;grid-template-columns:minmax(13rem,.8fr) minmax(18rem,1fr) auto;align-items:center;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;padding:.75rem}.run-identity{display:grid;grid-template-columns:auto 1fr;align-items:center;gap:.3rem .55rem}.run-identity small{grid-column:2;color:rgb(var(--yb-text-muted));font-size:.66rem}.run-status{border-radius:999px;padding:.25rem .45rem;background:rgb(var(--yb-surface-soft));font-size:.62rem;font-style:normal}.run-status.success{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.run-status.warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.run-status.danger{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.run-summary,.run-actions{display:flex;gap:.45rem}.run-actions{justify-content:flex-end}.run-dialog{width:min(56rem,100%)}.run-detail-list{min-height:0;display:grid;align-content:start;gap:.45rem;padding:1rem 1.2rem;overflow:auto}.run-detail-list article{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.65rem}.run-detail-list article>div{display:grid;gap:.2rem}.run-detail-list small{color:rgb(var(--yb-text-muted))}.run-detail-list em{font-size:.68rem;font-style:normal;color:rgb(var(--yb-text-muted))}.message{display:flex;align-items:center;justify-content:space-between;gap:1rem;border-radius:.75rem;padding:.7rem 1rem}.message.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.message.notice{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.message button{border:0;background:transparent;color:inherit;cursor:pointer;text-decoration:underline}.empty-small,.empty-large{padding:1.2rem;color:rgb(var(--yb-text-muted));text-align:center}.empty-inline{color:rgb(var(--yb-text-muted))}.modal-layer{position:fixed;inset:0;z-index:100;display:grid;place-items:center;padding:1rem}.modal-mask{position:absolute;inset:0;border:0;background:rgb(var(--yb-overlay-night)/.48)}.modal-card{position:relative;width:min(45rem,100%);max-height:min(52rem,92vh);display:grid;grid-template-rows:auto minmax(0,1fr) auto;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:1rem;background:rgb(var(--yb-surface));box-shadow:0 1.5rem 4rem rgb(var(--yb-shadow)/.25)}.modal-card>header,.modal-card>footer{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:1rem 1.2rem;border-bottom:1px solid rgb(var(--yb-border))}.modal-card>header h2{margin:.2rem 0 0}.modal-card>footer{justify-content:flex-end;border-top:1px solid rgb(var(--yb-border));border-bottom:0}.close{width:2.5rem;padding:0;font-size:1.3rem}.form-grid{min-height:0;display:grid;grid-template-columns:1fr 1fr;gap:.8rem;padding:1.2rem;overflow:auto}.span-2{grid-column:1/-1}.switch-row{grid-template-columns:auto 1fr;align-items:center}.switch-row input{width:auto;min-height:auto}.binding-dialog{grid-template-rows:auto auto minmax(0,1fr)}.dialog-search{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.6rem;padding:1rem 1.2rem}.candidate-list{min-height:0;display:grid;align-content:start;gap:.5rem;padding:0 1.2rem 1.2rem;overflow:auto}.candidate-list article{display:flex;align-items:center;justify-content:space-between;gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;padding:.7rem}.candidate-list article>span{display:grid;gap:.2rem}.candidate-list small{color:rgb(var(--yb-text-muted))}@media(max-width:1120px){.cost-layout{grid-template-columns:13rem minmax(0,1fr)}.calculator{position:static;grid-column:1/-1;grid-template-columns:repeat(2,minmax(0,1fr))}.calculator>header,.preview-result,.calculate-button{grid-column:1/-1}.run-card{grid-template-columns:1fr}.run-actions{justify-content:flex-start}}@media(max-width:760px){.cost-layout{grid-template-columns:1fr}.rule-groups{display:flex;overflow:auto}.rule-groups>header{display:none}.rule-groups>button{min-width:10rem;border-right:1px solid rgb(var(--yb-border));border-bottom:0}.calculator{grid-column:auto;grid-template-columns:1fr}.calculator>*{grid-column:auto}.header-actions,.cost-health,.run-summary,.run-actions{width:100%;flex-wrap:wrap}.header-actions button{flex:1}.form-grid{grid-template-columns:1fr}.span-2{grid-column:auto}.field-pair{grid-template-columns:1fr 1fr}.operations-heading{display:grid}.run-detail-list article{grid-template-columns:1fr}.run-detail-list article span,.run-detail-list article em{justify-self:start}}
.cost-manager-page{width:min(100%,96rem);min-width:0;margin-inline:auto;overflow-x:hidden}
.cost-layout{grid-template-columns:minmax(14rem,16rem) minmax(30rem,1fr) minmax(18rem,20rem)}
.rule-groups{position:sticky;top:1rem;max-height:min(46rem,calc(100vh - 9rem));overflow:auto}
.rule-workspace{align-self:start}
.calculator{position:sticky;top:1rem;grid-column:auto;grid-template-columns:1fr;align-items:stretch}
.calculator>header{align-self:start}.calculator .calculate-button{align-self:stretch}.calculator .preview-result{min-height:4.2rem}
.calculator input,.calculator select,.form-grid input,.form-grid select,.form-grid textarea,.dialog-search input{box-sizing:border-box;min-width:0}
.operations-panel,.run-card,.run-summary{min-width:0}
.run-card{grid-template-columns:minmax(11rem,.75fr) minmax(0,1fr) auto}
.run-summary{flex-wrap:wrap}
@media(max-width:1320px){.cost-layout{grid-template-columns:minmax(13rem,15rem) minmax(0,1fr)}.calculator{position:static;grid-column:2;grid-template-columns:repeat(2,minmax(0,1fr))}.calculator>header,.preview-result,.calculate-button{grid-column:1/-1}.run-card{grid-template-columns:1fr}.run-actions{justify-content:flex-start}}
@media(max-width:900px){.cost-layout{grid-template-columns:1fr}.rule-groups{position:static;display:flex;max-height:none;overflow:auto}.rule-groups>header{display:none}.rule-groups>button{min-width:11rem;border-right:1px solid rgb(var(--yb-border));border-bottom:0}.calculator{grid-column:auto}}
@media(max-width:620px){.calculator{grid-template-columns:1fr}.calculator>header,.preview-result,.calculate-button{grid-column:auto}.workspace-heading,.binding-panel>header{display:grid}.field-pair{grid-template-columns:1fr}.cost-health,.run-summary,.run-actions{width:100%;flex-wrap:wrap}}
.mapping-diagnostics{display:grid;gap:1rem;padding:1rem;border:1px solid rgb(var(--yb-border));border-radius:1rem;background:rgb(var(--yb-surface))}
.exact-run-builder{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:.7rem;padding:.8rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;background:rgb(var(--yb-surface-soft))}.exact-run-builder label{display:grid;gap:.35rem;color:rgb(var(--yb-text-muted));font-size:.72rem}.exact-run-builder textarea{box-sizing:border-box;width:100%;min-height:3.6rem;resize:vertical;border:1px solid rgb(var(--yb-border));border-radius:.65rem;padding:.55rem .7rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:inherit}@media(max-width:620px){.exact-run-builder{grid-template-columns:1fr}.exact-run-builder button{width:100%}}
.mapping-diagnostics>header{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem}
.mapping-diagnostics h2{margin:.2rem 0}
.mapping-diagnostics p{margin:.25rem 0 0;color:rgb(var(--yb-text-muted));font-size:.78rem}
.diagnostic-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:.65rem}
.diagnostic-grid article{display:grid;gap:.25rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem;padding:.8rem;background:rgb(var(--yb-surface-soft))}
.diagnostic-grid span,.diagnostic-grid small{color:rgb(var(--yb-text-muted));font-size:.68rem}
.diagnostic-grid strong{font-size:1.35rem}
.diagnostic-grid .warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}
.diagnostic-grid .danger{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}
@media(max-width:900px){.diagnostic-grid{grid-template-columns:1fr 1fr}}
@media(max-width:620px){.mapping-diagnostics>header{display:grid}.diagnostic-grid{grid-template-columns:1fr}}
</style>
<style scoped>
.workspace-tabs{display:flex;gap:8px}.workspace-tabs button{padding:8px 16px;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.workspace-tabs .active{background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.group-search{display:grid;gap:6px;padding:12px;font-size:12px}.group-search input,.model-workspace>label input{min-width:0;width:100%;box-sizing:border-box;padding:8px;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.model-workspace{display:grid;gap:12px}.model-workspace>label{display:grid;gap:6px}.model-workspace p{font-size:12px;color:rgb(var(--yb-text-muted))}.cost-layout{grid-template-columns:minmax(190px,.7fr) minmax(340px,1.4fr) minmax(300px,1fr);height:calc(100dvh - 225px);min-height:460px;align-items:stretch;gap:12px}.rule-groups,.rule-workspace,.calculator{position:static;grid-column:auto;max-height:100%;overflow:auto;min-height:0;display:block}.rule-groups>.group-search,.rule-workspace>.workspace-heading,.calculator>header{position:sticky;top:0;z-index:2;background:rgb(var(--yb-surface));padding-block:10px}.calculator{display:flex;flex-direction:column;gap:12px}.rule-workspace>*{margin-bottom:16px}.rule-copy>div{flex-wrap:wrap}.calculator .preview-result{flex-shrink:0}.rule-groups>button{min-width:0}@media(max-width:1050px){.cost-layout{grid-template-columns:180px minmax(300px,1fr);height:auto}.calculator{grid-column:2}.rule-groups{grid-row:span 2;max-height:calc(100dvh - 180px);position:sticky;top:0}.rule-workspace{max-height:65vh}}@media(max-width:650px){.cost-layout{display:flex;flex-direction:column;min-height:0}.rule-groups{position:static;max-height:180px;display:block}.rule-workspace,.calculator{max-height:none}.workspace-tabs{flex-wrap:wrap}}
</style>
