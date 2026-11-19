<template>
  <div class="design-board-page" :class="{ 'oa-design-board': oaEnabled }">
    <header class="design-board-header">
      <div>
        <h1>{{ departmentTitle }}</h1>
        <p>人员产能、任务结构、设计文件量、效率与质量风险的部门级视图。</p>
      </div>
      <DesktopSlot target="status">
        <span v-if="dashboard" class="freshness" role="status" :title="departmentTitle + ' · ' + formatDateTime(dashboard.generated_at)">{{ formatTime(dashboard.generated_at) }} 已更新</span>
      </DesktopSlot>
      <DesktopSlot target="actions">
      <div class="header-actions">
        <button class="back-link" type="button" @click="returnToTaskBoard"><ArrowLeft :size="16" />返回主页</button>
        <BaseButton variant="ghost" size="sm" :disabled="loading" @click="load"><RefreshCw :size="15" />{{ loading ? '刷新中…' : '刷新' }}</BaseButton>
        <BaseButton variant="primary" size="sm" :disabled="!dashboard" @click="exportCsv"><Download :size="15" />导出明细</BaseButton>
      </div>
      </DesktopSlot>
    </header>

    <section class="filter-panel" aria-label="看板筛选条件">
      <div class="filter-top">
      <div class="preset-row">
        <strong v-if="oaEnabled" class="design-scope">{{ departmentTitle }}</strong>
        <span v-else>快捷周期</span>
        <button v-for="preset in presets" :key="preset.days" type="button" :class="{ active: selectedPreset === preset.days }" @click="applyPreset(preset.days)">{{ preset.label }}</button>
      </div>
      <div v-if="oaEnabled" class="design-filter-actions">
        <button v-if="dashboard?.summary.overdue_task_count || overdueOnly" type="button" class="design-risk-filter" :aria-pressed="overdueOnly" @click="toggleOverdue">已逾期 {{ dashboard?.summary.overdue_task_count ?? 0 }}</button>
      <button type="button" class="filter-expand" :aria-expanded="showFilters" aria-controls="design-board-filter-grid" @click="showFilters = !showFilters">{{ showFilters ? '收起筛选' : '展开筛选' }}</button>
      </div>
      </div>
      <div v-show="!oaEnabled || showFilters" id="design-board-filter-grid" class="filter-grid">
        <label>开始日期<input v-model="filters.start_date" type="date" /></label>
        <label>结束日期<input v-model="filters.end_date" type="date" /></label>
        <label>日期口径<select v-model="filters.date_basis"><option value="created">任务创建</option><option value="completed">任务完成</option><option value="deadline">截止日期</option></select></label>
        <label>统计粒度<select v-model="filters.granularity"><option value="day">按日</option><option value="week">按周</option><option value="month">按月</option></select></label>
        <label>设计人员<select v-model="filters.designer_id"><option value="">全部人员</option><option v-for="member in memberOptions" :key="member.user_id" :value="String(member.user_id)">{{ member.display_name }}{{ member.status !== 'active' ? '（停用）' : '' }}</option></select></label>
        <label>团队<select v-model="filters.team_id"><option value="">全部团队</option><option v-for="team in teamOptions" :key="team.id" :value="team.id">{{ team.label }}</option></select></label>
        <label>任务类型<select v-model="filters.task_type"><option value="">全部类型</option><option v-for="item in taskTypeOptions" :key="item.key" :value="item.key">{{ item.label }}</option></select></label>
        <label>任务状态<select v-model="filters.status"><option value="">全部状态</option><option v-for="item in statusOptions" :key="item.key" :value="item.key">{{ item.label }}</option></select></label>
        <label>优先级<select v-model="filters.priority"><option value="">全部优先级</option><option v-for="item in priorityOptions" :key="item.key" :value="item.key">{{ item.label }}</option></select></label>
        <label>业务类型<select v-model="filters.business_lane"><option value="">全部业务</option><option value="normal">常规</option><option value="customization">定制</option></select></label>
      </div>
      <div v-show="!oaEnabled || showFilters" class="filter-actions"><span>所有指标、图表和明细共用当前筛选范围</span><BaseButton variant="ghost" size="sm" @click="resetFilters">重置</BaseButton><BaseButton variant="primary" size="sm" :disabled="loading" @click="applyDesignFilters">应用筛选</BaseButton></div>
    </section>

    <div v-if="error" class="state-card error" role="alert">{{ error }}<BaseButton variant="ghost" size="sm" @click="load">重试</BaseButton></div>
    <div v-else-if="loading && !dashboard" class="state-card">正在汇总设计部数据…</div>
    <template v-else-if="dashboard">
      <section class="kpi-grid" aria-label="部门核心指标">
        <article v-for="card in kpiCards" :key="card.label" class="kpi-card" :class="oaEnabled ? (card.tone === 'red' && Number(card.value) > 0 ? 'oa-kpi-risk' : undefined) : card.tone">
          <span>{{ card.label }}</span><strong>{{ card.value }}</strong><small>{{ card.hint }}</small>
        </article>
      </section>

      <section v-if="!dashboard.summary.task_count" class="state-card design-empty" role="status">
        <h2>当前筛选范围内暂无设计任务</h2>
        <p>可以调整团队、设计人员或日期范围。</p>
        <BaseButton v-if="oaEnabled && !showFilters" size="sm" variant="secondary" @click="showFilters = true">展开筛选</BaseButton>
      </section>
      <template v-else>
      <section class="analysis-grid analysis-grid--wide">
        <article class="panel trend-panel"><div class="panel-head"><div><h2>产出趋势</h2><p>所选日期口径下的任务、完成、设计文件与SKU变化</p></div><span>{{ filters.granularity === 'day' ? '按日' : filters.granularity === 'week' ? '按周' : '按月' }}</span></div><DesignDepartmentTrendChart :points="dashboard.trend" /></article>
        <article class="panel"><div class="panel-head"><div><h2>人员工作量</h2><p>按任务量排序，可点击人员继续筛选</p></div><select v-model="peopleSort"><option value="task_count">任务量</option><option value="design_file_count">设计文件</option><option value="completed_task_count">完成量</option><option value="average_turnaround_hours">平均时长</option></select></div>
          <div class="people-bars"><button v-for="person in sortedPeople.slice(0, 12)" :key="person.user_id" type="button" @click="selectPerson(person.user_id)"><span>{{ person.display_name }}</span><i><b :style="{ width: `${personBarWidth(person)}%` }" /></i><strong>{{ personMetricValue(person) }}</strong></button></div>
        </article>
      </section>

      <section class="analysis-grid">
        <article class="panel"><div class="panel-head"><div><h2>任务种类占比</h2><p>任务量与对应设计文件量</p></div></div><DesignDepartmentBreakdownChart title="任务种类占比" :items="dashboard.task_types" /></article>
        <article class="panel"><div class="panel-head"><div><h2>任务状态分布</h2><p>识别积压、完成和异常任务</p></div></div><DesignDepartmentBreakdownChart title="任务状态分布" :items="dashboard.statuses" /></article>
        <article class="panel"><div class="panel-head"><div><h2>优先级结构</h2><p>当前范围内任务紧急程度</p></div></div><DesignDepartmentBreakdownChart title="优先级结构" :items="dashboard.priorities" /></article>
        <article class="panel composition-panel"><div class="panel-head"><div><h2>图量与资源结构</h2><p>源文件、成品文件、SKU和资源单元</p></div></div><div class="composition-list"><div v-for="item in dashboard.file_types" :key="item.key"><span>{{ item.label }}</span><strong>{{ item.task_count }}</strong></div></div><div class="quality-grid"><div><span>一次通过率</span><b>{{ percent(dashboard.summary.first_pass_rate) }}</b></div><div><span>按期完成率</span><b>{{ percent(dashboard.summary.on_time_rate) }}</b></div><div><span>中位处理时长</span><b>{{ hours(dashboard.summary.median_turnaround_hours) }}</b></div><div><span>P90处理时长</span><b>{{ hours(dashboard.summary.p90_turnaround_hours) }}</b></div></div></article>
      </section>

      </template>

      <section class="panel table-panel">
        <div class="panel-head"><div><h2>人员绩效明细</h2><p>共 {{ dashboard.people.length }} 人；停用账号保留历史数据</p></div><input v-model.trim="peopleKeyword" type="search" placeholder="搜索人员" /></div>
        <div class="design-table-wrap"><table><thead><tr><th>人员</th><th>任务</th><th>完成</th><th>进行中</th><th>逾期</th><th>设计文件</th><th>SKU</th><th>完成率</th><th>一次通过</th><th>按期完成</th><th>平均时长</th><th>工作量占比</th></tr></thead><tbody><tr v-for="person in filteredPeople" :key="person.user_id"><td><button type="button" class="person-link" @click="selectPerson(person.user_id)">{{ person.display_name }}</button><small>{{ person.team_name || '未分组' }}</small></td><td>{{ person.task_count }}</td><td>{{ person.completed_task_count }}</td><td>{{ person.active_task_count }}</td><td :class="{ danger: person.overdue_task_count > 0 }">{{ person.overdue_task_count }}</td><td>{{ person.design_file_count }}</td><td>{{ person.sku_count }}</td><td>{{ percent(person.completion_rate) }}</td><td>{{ percent(person.first_pass_rate) }}</td><td>{{ percent(person.on_time_rate) }}</td><td>{{ hours(person.average_turnaround_hours) }}</td><td>{{ percent(person.workload_share) }}</td></tr></tbody></table></div>
      </section>

      <section id="design-task-exceptions" ref="taskSection" class="panel table-panel">
        <div class="panel-head"><div><h2>任务明细与异常定位</h2><p>{{ overdueOnly ? '最近 200 条任务中的逾期明细' : '最多展示当前筛选范围内最近 200 条任务' }}</p></div><input v-model.trim="taskKeyword" type="search" placeholder="任务号、产品或人员" /></div>
        <div class="design-table-wrap"><table><thead><tr><th>任务号</th><th>设计人员</th><th>类型</th><th>状态</th><th>优先级</th><th>源文件</th><th>成品</th><th>SKU</th><th>打回</th><th>处理时长</th><th>截止时间</th></tr></thead><tbody><tr v-for="task in filteredTasks" :key="task.task_id"><td><button type="button" class="task-link" @click="openTask(task.task_id)">{{ task.task_no }}</button><small>{{ task.product_name }}</small></td><td>{{ task.designer_name }}</td><td>{{ taskTypeLabel(task.task_type) }}</td><td><span class="status-pill" :class="{ overdue: task.overdue }">{{ taskStatusLabel(task.task_status) }}</span></td><td>{{ priorityLabel(task.priority) }}</td><td>{{ task.source_file_count }}</td><td>{{ task.final_file_count }}</td><td>{{ task.sku_count }}</td><td :class="{ danger: task.audit_reject_count > 0 }">{{ task.audit_reject_count }}</td><td>{{ task.turnaround_hours == null ? '—' : hours(task.turnaround_hours) }}</td><td>{{ task.deadline_at ? formatDateTime(task.deadline_at) : '—' }}</td></tr></tbody></table></div>
      </section>

      <details class="definitions"><summary>指标口径说明</summary><dl><template v-for="(definition, key) in dashboard.definitions" :key="key"><dt>{{ definitionLabel(key) }}</dt><dd>{{ definition }}</dd></template></dl></details>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Download, RefreshCw } from 'lucide-vue-next'
import { oaEnabled } from '@/oa/access'
import DesktopSlot from '@/oa/components/DesktopSlot.vue'
import { goBackTo } from '@/oa/returnNavigation'
import { taskDetailLocation, taskReturnSource } from '@/oa/taskNavigation'
import BaseButton from '@/components/base/BaseButton.vue'
import DesignDepartmentTrendChart from '@/components/dashboard/DesignDepartmentTrendChart.vue'
import DesignDepartmentBreakdownChart from '@/components/dashboard/DesignDepartmentBreakdownChart.vue'
import { tasksApi } from '@/services/api/tasksApi'
import type { DesignDepartmentDashboard, DesignDepartmentDashboardBreakdown, DesignDepartmentPersonMetric, DesignDepartmentTaskRow } from '@/types/dashboard'

const router = useRouter()
function returnToTaskBoard() {
  const source = taskReturnSource(router.currentRoute?.value.query.taskBoardFrom)
  const previous = taskReturnSource(window.history.state?.back)
  const target = [source, previous].find((entry) => entry?.split(/[?#]/, 1)[0] === '/')
  void goBackTo(router, target || '/?workspace=tasks')
}
async function openTask(id: number) {
  await rememberDesignFilters(appliedFilters)
  void router.push(taskDetailLocation(id, router.currentRoute?.value.fullPath || '/design-dashboard'))
}
const entryQuery = router.currentRoute?.value.query ?? {}
const dashboard = ref<DesignDepartmentDashboard | null>(null)
const showFilters = ref(entryQuery.design_filters === '1')
const departmentTitle = computed(() => {
  const department = dashboard.value?.department_name
  if (oaEnabled && dashboard.value && (!dashboard.value.department_id || !department || department === '未分配')) return '跨部门设计产能看板（全部可见范围）'
  return `${department || '设计部'}数据看板`
})
const loading = ref(false)
const error = ref('')
const peopleKeyword = ref(entryText(entryQuery.design_people_search))
const taskKeyword = ref(entryText(entryQuery.design_task_search))
const overdueOnly = ref(entryQuery.design_overdue === '1')
const taskSection = ref<HTMLElement>()
function toggleOverdue() {
  overdueOnly.value = !overdueOnly.value
  if (overdueOnly.value) taskSection.value?.scrollIntoView({ block: 'start' })
}
const taskTypeOptions = ref<DesignDepartmentDashboardBreakdown[]>([])
const statusOptions = ref<DesignDepartmentDashboardBreakdown[]>([])
const priorityOptions = ref<DesignDepartmentDashboardBreakdown[]>([])
const teamOptions = ref<Array<{ id: string; label: string }>>([])
const peopleSort = ref<'task_count' | 'design_file_count' | 'completed_task_count' | 'average_turnaround_hours'>('task_count')
const presets = [{ days: 7, label: '近7天' }, { days: 30, label: '近30天' }, { days: 90, label: '近90天' }]
const selectedPreset = ref([7, 30, 90].includes(Number(entryQuery.design_period)) ? Number(entryQuery.design_period) : 30)
let controller: AbortController | null = null

function entryText(value: unknown) { return typeof value === 'string' ? value : '' }
function entryDate(value: unknown, fallback: string) {
  const text = entryText(value)
  return /^\d{4}-\d{2}-\d{2}$/.test(text) ? text : fallback
}

function beijingDate(offsetDays = 0) {
  const now = new Date(Date.now() + offsetDays * 86400000)
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(now)
}
const filters = reactive<{
  start_date: string
  end_date: string
  date_basis: 'created' | 'completed' | 'deadline'
  granularity: 'day' | 'week' | 'month'
  designer_id: string
  team_id: string
  task_type: string
  status: string
  priority: string
  business_lane: string
}>({
  start_date: entryDate(entryQuery.design_start, beijingDate(-29)),
  end_date: entryDate(entryQuery.design_end, beijingDate()),
  date_basis: entryQuery.design_basis === 'completed' || entryQuery.design_basis === 'deadline' ? entryQuery.design_basis : 'created',
  granularity: entryQuery.design_granularity === 'week' || entryQuery.design_granularity === 'month' ? entryQuery.design_granularity : 'day',
  designer_id: entryText(entryQuery.design_person),
  team_id: entryText(entryQuery.design_team),
  task_type: entryText(entryQuery.design_type),
  status: entryText(entryQuery.design_status),
  priority: entryText(entryQuery.design_priority),
  business_lane: entryText(entryQuery.design_lane),
})
let appliedFilters = { ...filters }
const memberOptions = computed(() => dashboard.value?.options.members ?? [])

async function load() {
  controller?.abort(); const request = new AbortController(); controller = request; loading.value = true; error.value = ''
  const requested = { ...filters }
  try {
    const response = await tasksApi.designDepartmentDashboard({ start_date: requested.start_date, end_date: requested.end_date, date_basis: requested.date_basis, granularity: requested.granularity, designer_ids: requested.designer_id || undefined, team_ids: requested.team_id || undefined, task_types: requested.task_type || undefined, statuses: requested.status || undefined, priorities: requested.priority || undefined, business_lanes: requested.business_lane || undefined }, request.signal)
    if (request.signal.aborted || controller !== request) return
    dashboard.value = response.data.data
    appliedFilters = requested
    taskTypeOptions.value = mergeBreakdownOptions(taskTypeOptions.value, optionBreakdowns(dashboard.value.options.task_types, dashboard.value.task_types, taskTypeLabel))
    statusOptions.value = mergeBreakdownOptions(statusOptions.value, optionBreakdowns(dashboard.value.options.statuses, dashboard.value.statuses, taskStatusLabel))
    priorityOptions.value = mergeBreakdownOptions(priorityOptions.value, optionBreakdowns(dashboard.value.options.priorities, dashboard.value.priorities, priorityLabel))
    teamOptions.value = mergeSimpleOptions(teamOptions.value, dashboard.value.options.teams)
  } catch (cause) {
    if (request.signal.aborted || controller !== request) return
    error.value = cause instanceof Error ? cause.message : '设计部看板加载失败，请稍后重试。'
  } finally { if (controller === request) loading.value = false }
}
async function rememberDesignFilters(savedFilters = filters) {
  const current = router.currentRoute?.value
  if (!oaEnabled || current?.name !== 'DesignDepartmentDashboard') return
  const query = {
    ...current.query,
    design_period: String(selectedPreset.value),
    design_start: savedFilters.start_date,
    design_end: savedFilters.end_date,
    design_basis: savedFilters.date_basis,
    design_granularity: savedFilters.granularity,
    design_person: savedFilters.designer_id || undefined,
    design_team: savedFilters.team_id || undefined,
    design_type: savedFilters.task_type || undefined,
    design_status: savedFilters.status || undefined,
    design_priority: savedFilters.priority || undefined,
    design_lane: savedFilters.business_lane || undefined,
    design_filters: showFilters.value ? '1' : undefined,
    design_people_search: peopleKeyword.value || undefined,
    design_task_search: taskKeyword.value || undefined,
    design_overdue: overdueOnly.value ? '1' : undefined,
  }
  await router.replace({ query })
}
function applyDesignFilters() {
  void rememberDesignFilters()
  void load()
}
function applyPreset(days: number) {
  selectedPreset.value = days
  filters.end_date = beijingDate()
  filters.start_date = beijingDate(-(days - 1))
  applyDesignFilters()
}
function resetFilters() { selectedPreset.value = 30; Object.assign(filters, { start_date: beijingDate(-29), end_date: beijingDate(), date_basis: 'created', granularity: 'day', designer_id: '', team_id: '', task_type: '', status: '', priority: '', business_lane: '' }); applyDesignFilters() }
function selectPerson(id: number) { filters.designer_id = String(id); applyDesignFilters() }
function mergeBreakdownOptions(current: DesignDepartmentDashboardBreakdown[], next: DesignDepartmentDashboardBreakdown[]) { const map = new Map(current.map((item) => [item.key, item])); for (const item of next) map.set(item.key, item); return Array.from(map.values()) }
function mergeSimpleOptions(current: Array<{ id: string; label: string }>, next: Array<{ id: string; label: string }>) { const map = new Map(current.map((item) => [item.id, item])); for (const item of next) map.set(item.id, item); return Array.from(map.values()) }
function optionBreakdowns(keys: string[], populated: DesignDepartmentDashboardBreakdown[], label: (key: string) => string) { const map = new Map(populated.map((item) => [item.key, item])); return keys.map((key) => map.get(key) ?? { key, label: label(key), task_count: 0, design_file_count: 0, sku_count: 0, share: 0 }) }

const kpiCards = computed(() => {
  const value = dashboard.value?.summary
  if (!value) return []
  return [
    { label: '任务量', value: value.task_count, hint: `完成 ${value.completed_task_count} · 进行中 ${value.active_task_count}`, tone: 'blue' },
    { label: '设计文件量', value: value.design_file_count, hint: `源文件 ${value.source_file_count} · 成品 ${value.final_file_count}`, tone: 'violet' },
    { label: 'SKU / 资源单元', value: `${value.sku_count} / ${value.resource_unit_count}`, hint: `平均每任务 ${value.average_files_per_task} 个设计文件`, tone: 'cyan' },
    { label: '参与人员', value: `${value.contributing_member_count} / ${value.member_count}`, hint: '有任务人员 / 在职部门成员', tone: 'green' },
    { label: '完成率', value: percent(value.completion_rate), hint: '完成任务 / 筛选任务', tone: 'green' },
    { label: '一次通过率', value: percent(value.first_pass_rate), hint: `打回任务 ${value.rejected_task_count}`, tone: 'amber' },
    { label: '平均处理时长', value: hours(value.average_turnaround_hours), hint: `样本 ${value.turnaround_sample_count} 条`, tone: 'blue' },
    { label: '风险任务', value: value.overdue_task_count + value.due_soon_task_count, hint: `逾期 ${value.overdue_task_count} · 72小时内到期 ${value.due_soon_task_count}`, tone: 'red' },
  ]
})
const sortedPeople = computed(() => [...(dashboard.value?.people ?? [])].sort((a, b) => Number(b[peopleSort.value]) - Number(a[peopleSort.value])))
const filteredPeople = computed(() => { const q = peopleKeyword.value.toLowerCase(); return sortedPeople.value.filter((item) => !q || `${item.display_name} ${item.username} ${item.team_name}`.toLowerCase().includes(q)) })
const filteredTasks = computed(() => { const q = taskKeyword.value.toLowerCase(); return (dashboard.value?.recent_tasks ?? []).filter((item) => (!overdueOnly.value || item.overdue) && (!q || `${item.task_no} ${item.product_name} ${item.designer_name}`.toLowerCase().includes(q))) })
function personMetricValue(person: DesignDepartmentPersonMetric) { const value = person[peopleSort.value]; return peopleSort.value === 'average_turnaround_hours' ? hours(Number(value)) : String(value) }
function personBarWidth(person: DesignDepartmentPersonMetric) { const max = Math.max(...sortedPeople.value.map((item) => Number(item[peopleSort.value])), 1); return Math.max(2, Number(person[peopleSort.value]) / max * 100) }
function percent(value: number) { return `${Number(value || 0).toFixed(1)}%` }
function hours(value: number) { return `${Number(value || 0).toFixed(1)} 小时` }
function formatTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
function formatDateTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
function taskTypeLabel(value: string) { return ({ new_product_development: '新品开发', original_product_development: '原品开发', retouch_task: '修图任务', sku_planning: 'SKU规划' } as Record<string, string>)[value] ?? value }
function taskStatusLabel(value: string) { return ({ Completed: '已完成', InProgress: '处理中', PendingAudit: '待审核', Cancelled: '已取消', Blocked: '已阻塞', PendingAssign: '待指派', Assigned: '已指派' } as Record<string, string>)[value] ?? value }
function priorityLabel(value: string) { return ({ critical: '紧急', high: '高', drawing: '画图优先', normal: '普通', low: '低' } as Record<string, string>)[value] ?? value }
function definitionLabel(key: string) { return ({ task_count: '任务量', design_file_count: '设计文件量', first_pass_rate: '一次通过率', turnaround: '处理时长', on_time_rate: '按期完成率' } as Record<string, string>)[key] ?? key }
function exportCsv() { if (!dashboard.value) return; const headers = ['任务号','产品','设计人员','类型','状态','优先级','源文件','成品文件','SKU','打回次数','处理时长']; const rows = filteredTasks.value.map((item: DesignDepartmentTaskRow) => [item.task_no,item.product_name,item.designer_name,taskTypeLabel(item.task_type),taskStatusLabel(item.task_status),priorityLabel(item.priority),item.source_file_count,item.final_file_count,item.sku_count,item.audit_reject_count,item.turnaround_hours ?? '']); const csv = '\ufeff' + [headers,...rows].map((row) => row.map((cell) => `"${String(cell).replace(/"/g,'""')}"`).join(',')).join('\n'); const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' })); const link = document.createElement('a'); link.href=url; link.download=`设计部看板-${filters.start_date}-${filters.end_date}.csv`; link.click(); setTimeout(() => URL.revokeObjectURL(url),1000) }
onMounted(load)
onBeforeUnmount(() => controller?.abort())
</script>

<style scoped>
.design-board-page{min-height:100%;padding:24px;background:rgb(var(--yb-bg-page));color:rgb(var(--yb-text-strong))}.design-board-header{display:flex;justify-content:space-between;gap:24px;align-items:flex-start;margin-bottom:18px}.design-board-header h1{font-size:30px;margin:8px 0 3px}.design-board-header p,.panel-head p{margin:0;color:rgb(var(--yb-text-muted));font-size:13px}.back-link,.person-link,.task-link{border:0;background:none;color:rgb(var(--yb-brand));padding:0;display:inline-flex;align-items:center;gap:5px;cursor:pointer}.header-actions{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.freshness{font-size:12px;color:rgb(var(--yb-text-label))}.filter-panel,.panel,.kpi-card,.state-card,.definitions{background:rgb(var(--yb-surface));border:1px solid rgb(var(--yb-border-slate));border-radius:14px;box-shadow:0 4px 18px rgb(var(--yb-shadow)/.04)}.filter-panel{padding:16px;margin-bottom:16px}.preset-row{display:flex;align-items:center;gap:7px;margin-bottom:13px;font-size:13px;color:rgb(var(--yb-text-muted-strong))}.preset-row button{border:1px solid rgb(var(--yb-border-blue-soft));background:rgb(var(--yb-surface-subtle));border-radius:20px;padding:5px 11px;cursor:pointer}.preset-row button.active{background:rgb(var(--yb-surface-blue-control));border-color:rgb(var(--yb-brand-border-strong));color:rgb(var(--yb-brand-link))}.filter-grid{display:grid;grid-template-columns:repeat(5,minmax(140px,1fr));gap:10px}.filter-grid label{font-size:12px;color:rgb(var(--yb-text-muted-strong));display:flex;flex-direction:column;gap:5px}.filter-grid input,.filter-grid select,.panel-head input,.panel-head select{height:36px;border:1px solid rgb(var(--yb-border-form));border-radius:8px;background:rgb(var(--yb-surface));padding:0 9px;color:rgb(var(--yb-text-body-strong))}.filter-actions{display:flex;align-items:center;justify-content:flex-end;gap:8px;margin-top:12px}.filter-actions>span{margin-right:auto;color:rgb(var(--yb-text-form-muted));font-size:12px}.state-card{padding:24px;text-align:center}.state-card.error{color:rgb(var(--yb-danger-action))}.kpi-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-bottom:16px}.kpi-card{padding:16px;min-height:118px;display:flex;flex-direction:column;border-top:3px solid rgb(var(--yb-status-pending))}.kpi-card span{font-size:13px;color:rgb(var(--yb-text-form-secondary))}.kpi-card strong{font-size:28px;margin:10px 0 5px}.kpi-card small{color:rgb(var(--yb-text-form-muted-alt))}.kpi-card.green{border-top-color:rgb(var(--yb-success-emerald-bright))}.kpi-card.violet{border-top-color:rgb(var(--yb-purple))}.kpi-card.cyan{border-top-color:rgb(var(--yb-teal-accent))}.kpi-card.amber{border-top-color:rgb(var(--yb-warning-accent))}.kpi-card.red{border-top-color:rgb(var(--yb-red))}.analysis-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin-bottom:16px}.analysis-grid--wide{grid-template-columns:1.6fr 1fr}.panel{padding:17px;min-width:0}.panel-head{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;margin-bottom:12px}.panel-head h2{font-size:16px;margin:0 0 3px}.panel-head>span{font-size:12px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-strong));border-radius:20px;padding:4px 9px}.people-bars{display:flex;flex-direction:column;gap:8px;max-height:320px;overflow:auto}.people-bars button{display:grid;grid-template-columns:85px 1fr 58px;gap:8px;align-items:center;border:0;background:none;padding:4px;cursor:pointer;text-align:left}.people-bars i{height:9px;background:rgb(var(--yb-surface-slate));border-radius:5px;overflow:hidden}.people-bars b{display:block;height:100%;background:linear-gradient(90deg,rgb(var(--yb-brand-bright)),rgb(var(--yb-purple)));border-radius:5px}.people-bars strong{text-align:right;font-size:12px}.composition-list{display:grid;grid-template-columns:repeat(2,1fr);gap:10px}.composition-list>div,.quality-grid>div{padding:12px;border-radius:10px;background:rgb(var(--yb-surface-subtle));display:flex;justify-content:space-between}.composition-list strong{font-size:20px}.quality-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:10px;margin-top:12px}.quality-grid>div{flex-direction:column;gap:7px}.quality-grid span{font-size:12px;color:rgb(var(--yb-text-form-secondary))}.quality-grid b{font-size:18px}.table-panel{margin-bottom:16px}.design-table-wrap{overflow:auto;max-height:520px}.table-panel table{width:100%;border-collapse:collapse;min-width:1050px;font-size:12px}.table-panel th{position:sticky;top:0;background:rgb(var(--yb-surface-raised));color:rgb(var(--yb-text-soft));text-align:left;padding:10px 8px;z-index:1}.table-panel td{padding:10px 8px;border-top:1px solid rgb(var(--yb-border-quiet))}.table-panel td small{display:block;max-width:230px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:rgb(var(--yb-text-form-muted-alt));margin-top:3px}.danger{color:rgb(var(--yb-danger));font-weight:700}.status-pill{display:inline-block;padding:3px 7px;border-radius:10px;background:rgb(var(--yb-surface-slate))}.status-pill.overdue{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger))}.definitions{padding:13px 16px;margin-bottom:24px}.definitions summary{cursor:pointer;font-weight:600}.definitions dl{display:grid;grid-template-columns:120px 1fr;gap:8px 14px}.definitions dt{font-weight:600}.definitions dd{margin:0;color:rgb(var(--yb-text-muted-strong))}.error button{margin-left:12px}@media(max-width:1250px){.filter-grid{grid-template-columns:repeat(4,1fr)}.kpi-grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:800px){.design-board-page{padding:14px}.design-board-header{flex-direction:column}.filter-grid{grid-template-columns:repeat(2,1fr)}.analysis-grid,.analysis-grid--wide{grid-template-columns:1fr}.kpi-grid{grid-template-columns:1fr 1fr}.header-actions{width:100%}}@media(max-width:480px){.filter-grid,.kpi-grid{grid-template-columns:1fr}.design-board-header h1{font-size:24px}}
.oa-design-board .design-board-header h1 { font-size: 18px; font-weight: 600; }
.oa-design-board .panel-head h2 { font-size: 13px; font-weight: 500; }
.oa-design-board .panel-head p, .oa-design-board .kpi-card small { font-size: 12px; }
.oa-design-board td, .oa-design-board th { font-size: 13px; }
.oa-design-board th, .oa-design-board strong, .oa-design-board b { font-weight: 500; }
.oa-design-board .back-link { font-size: 14px; }
.oa-design-board .filter-panel { padding: 5px 12px; }
.oa-design-board .filter-top { min-height: 36px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; }
.oa-design-board .preset-row { margin: 0; }
.oa-design-board .filter-expand { min-height: 32px; font-size: 13px; padding: 4px 8px; border: 1px solid rgb(var(--yb-border)); border-radius: 6px; }
.oa-design-board .kpi-card { color: rgb(var(--yb-text)); background: rgb(var(--yb-surface)); border-color: rgb(var(--yb-border)); }
.oa-design-board .kpi-card strong { font-weight: 500; color: rgb(var(--yb-text-strong)); }
.oa-design-board .kpi-card.oa-kpi-risk strong { color: rgb(var(--yb-danger)); }

.design-filter-actions { display: flex; align-items: center; gap: 6px; }
.design-risk-filter { border: 1px solid rgb(var(--yb-warning-accent) / .35); border-radius: 999px; background: rgb(var(--yb-warning-accent) / .09); color: rgb(var(--yb-warning-text)); padding: 5px 9px; font-size: 12px; white-space: nowrap; }
.design-risk-filter:hover, .design-risk-filter[aria-pressed=true] { background: rgb(var(--yb-warning-accent) / .2); }
@media (min-width: 1280px) {
  .oa-design-board { width: 100%; max-width: none; margin: 0; padding: 0; background: transparent; }
  .oa-design-board .design-board-header { display: contents; }
  .oa-design-board .design-board-header > div:first-child { display: none; }
  .header-actions { flex-wrap: nowrap; gap: 8px; white-space: nowrap; }
  .header-actions :deep(button) { min-height: 28px; padding: 4px 8px; font-size: 12px; }
  .header-actions :deep(button > span) { display: inline-flex; align-items: center; gap: 4px; }
  .freshness { white-space: nowrap; }
  .oa-design-board .filter-panel { border-radius: 6px; margin-bottom: 10px; box-shadow: none; }
  .design-scope { margin-inline-end: 8px; }
  .oa-design-board .kpi-grid { gap: 0; border: 1px solid rgb(var(--yb-border)); border-radius: 6px; overflow: hidden; margin-bottom: 12px; }
  .oa-design-board .kpi-card { min-height: 72px; gap: 2px; border: 0; border-radius: 0; box-shadow: none; padding: 6px 12px; }
  .oa-design-board .kpi-card + .kpi-card { border-inline-start: 1px solid rgb(var(--yb-border)); }
  .oa-design-board .kpi-card:nth-child(n+5) { border-block-start: 1px solid rgb(var(--yb-border)); }
  .oa-design-board .kpi-card strong { font-size: 20px; line-height: 1.15; margin: 0; font-variant-numeric: tabular-nums; }
  .oa-design-board .analysis-grid { gap: 12px; margin-bottom: 12px; }
  .oa-design-board .panel { padding: 12px; border-radius: 6px; }
  .oa-design-board :deep(.design-trend-chart) { height: 176px; min-height: 0; }
  .oa-design-board .people-bars { max-height: none; overflow: visible; }
  .oa-design-board .design-table-wrap { max-height: none; overflow: visible; }
  .oa-design-board .table-panel table { min-width: 0; }
  .oa-design-board .table-panel th { position: static; }
  .oa-design-board .table-panel td { height: 40px; padding: 4px 8px; }
  .oa-design-board .table-panel tr:hover td { background: rgb(var(--yb-surface-hover)); }
  .oa-design-board .table-panel th:nth-child(n+2), .oa-design-board .table-panel td:nth-child(n+2) { font-variant-numeric: tabular-nums; }
  .oa-design-board .table-panel:not(#design-task-exceptions) :is(th, td):nth-child(n+2),
  .oa-design-board #design-task-exceptions :is(th, td):nth-child(n+6) { text-align: right; white-space: nowrap; }
}
@media (min-width: 2300px) {
  .oa-design-board .kpi-grid { grid-template-columns: repeat(8, minmax(0, 1fr)); }
  .oa-design-board .kpi-card:nth-child(n+5) { border-block-start: 0; }
}
.design-empty { margin-bottom: 12px; }
.design-empty h2 { margin: 0 0 4px; font-size: 13px; font-weight: 500; }
.design-empty p { margin: 0 0 10px; color: rgb(var(--yb-text-muted)); font-size: 13px; }
:global(html.oa-live-ui #app .content .oa-design-board :is(.panel-head,.design-empty) h2) { font-size: 13px; font-weight: 500; }
:global(html.oa-live-ui #app .content .oa-design-board :is(strong,b,th)) { font-weight: 500; }
:global(html.oa-live-ui #app .content .oa-design-board :is(button,input:not([type=checkbox],[type=file]))) { border-radius: 6px; }
.oa-design-board :deep(button.inline-flex > span:last-child) { display: contents; }
:global(html.oa-live-ui #app .oa-bar-page-actions .header-actions button) { border-radius: 6px; }
:global(html.oa-live-ui #app .oa-bar-page-actions .header-actions button > span) { display: contents; }
:global(html.oa-live-ui #app .oa-bar-page-actions .header-actions button svg) { width: 16px; height: 16px; }
</style>
