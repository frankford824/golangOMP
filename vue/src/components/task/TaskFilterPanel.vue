<template>
  <section class="filter-panel" aria-labelledby="task-filter-title">
    <header class="filter-heading">
      <div><p>缩小任务范围</p><h2 id="task-filter-title">高级筛选</h2></div>
      <button type="button" aria-label="关闭筛选" @click="emit('close')">×</button>
    </header>
    <NForm label-placement="top" size="medium" :show-feedback="false" class="filter-form">
      <div class="filter-scroll">
        <section class="filter-section">
          <h3>任务属性</h3>
          <NFormItem label="任务状态" class="wide">
            <div class="status-options" role="group" aria-label="任务状态">
              <button v-for="option in statusOptions" :key="option.value" type="button" class="status-option" :class="{ active: draft.status.includes(option.value as TaskStatus) }" :aria-pressed="draft.status.includes(option.value as TaskStatus)" @click="toggleStatus(option.value as TaskStatus)">{{ option.label }}</button>
            </div>
          </NFormItem>
          <NFormItem label="任务分组"><NSelect :to="false" :value="draft.taskCategory || null" clearable aria-label="任务分组" :input-props="{ 'aria-label': '任务分组' }" placeholder="全部分组" :options="taskCategoryOptions" @update:value="patch({ taskCategory: String($event || '') })" /></NFormItem>
          <NFormItem label="任务类型"><NSelect :to="false" :value="draft.taskType || null" clearable aria-label="任务类型" :input-props="{ 'aria-label': '任务类型' }" placeholder="全部类型" :options="taskTypeOptions" @update:value="patch({ taskType: String($event || '') })" /></NFormItem>
          <NFormItem label="优先级"><NSelect :to="false" :value="draft.priority || null" clearable aria-label="优先级" :input-props="{ 'aria-label': '优先级' }" placeholder="全部" :options="priorityOptions" @update:value="patch({ priority: String($event || '') })" /></NFormItem>
          <NFormItem label="其他"><NCheckbox :checked="draft.overdueOnly" @update:checked="patch({ overdueOnly: !!$event })">仅看已逾期任务</NCheckbox></NFormItem>
        </section>

        <section v-if="optionsLoading || optionsLoadError || !hasLoadedOptions" class="options-status" :class="{ 'is-error': Boolean(optionsLoadError) }" role="status">
          <span v-if="optionsLoading">正在加载部门、团队和人员候选…</span>
          <template v-else-if="optionsLoadError">
            <span>{{ optionsLoadError }}，部门、团队和人员暂时选不了。</span>
            <button type="button" @click="loadFilterOptions">重新加载</button>
          </template>
          <span v-else>没有可选的部门、团队或人员，可能是你的可见范围内还没有相关任务。</span>
        </section>

        <section class="filter-section">
          <h3>组织归属</h3>
          <NFormItem label="归属部门"><NSelect :to="false" :value="draft.ownerDepartment || null" clearable filterable aria-label="归属部门" :input-props="{ 'aria-label': '归属部门' }" placeholder="全部部门" :options="departmentSelectOptions" @update:value="onOwnerDepartmentChange(String($event || ''))" /></NFormItem>
          <NFormItem label="归属团队"><NSelect :to="false" :value="draft.ownerOrgTeam || null" clearable filterable aria-label="归属团队" :input-props="{ 'aria-label': '归属团队' }" placeholder="全部团队" :options="teamSelectOptions" @update:value="patch({ ownerOrgTeam: String($event || '') })" /></NFormItem>
        </section>

        <section class="filter-section">
          <h3>相关人员</h3>
          <NFormItem label="创建人"><NSelect :to="false" :value="draft.creatorId || null" clearable filterable aria-label="创建人" :input-props="{ 'aria-label': '创建人' }" placeholder="选择创建人" :options="creatorSelectOptions" @update:value="patch({ creatorId: String($event || '') })" /></NFormItem>
          <NFormItem label="设计师 / 执行人"><NSelect :to="false" :value="draft.assigneeId || null" clearable filterable aria-label="设计师或执行人" :input-props="{ 'aria-label': '设计师或执行人' }" placeholder="选择人员" :options="assigneeSelectOptions" @update:value="patch({ assigneeId: String($event || '') })" /></NFormItem>
        </section>

        <section class="filter-section">
          <h3>创建时间</h3>
          <NFormItem label="开始日期"><NDatePicker :to="false" :value="dateFromMs" type="date" clearable class="w-full" @update:value="onDateFrom" /></NFormItem>
          <NFormItem label="结束日期"><NDatePicker :to="false" :value="dateToMs" type="date" clearable class="w-full" @update:value="onDateTo" /></NFormItem>
        </section>

        <section class="filter-section keyword-section">
          <h3>精确查找</h3>
          <NFormItem label="关键词" class="wide"><NInput :value="draftKeyword" clearable placeholder="任务号、SKU、任务名称" @update:value="draftKeyword = $event || ''" /></NFormItem>
        </section>
      </div>
      <footer class="filter-actions"><NButton @click="reset">重置</NButton><NButton type="primary" @click="apply">应用筛选</NButton></footer>
    </NForm>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { NButton, NCheckbox, NDatePicker, NForm, NFormItem, NInput, NSelect } from 'naive-ui'
import type { ActiveTaskStatus as TaskStatus } from '@/domain/types/task'
import { useTaskFilterOptions } from '@/composables/useTaskFilterOptions'

export interface TaskListFilters { status: TaskStatus[]; taskCategory: string; taskType: string; priority: string; creatorId: string; assigneeId: string; dateFrom: string; dateTo: string; overdueOnly: boolean; ownerDepartment: string; ownerOrgTeam: string }
const emptyFilters = (): TaskListFilters => ({ status: [], taskCategory: '', taskType: '', priority: '', creatorId: '', assigneeId: '', dateFrom: '', dateTo: '', overdueOnly: false, ownerDepartment: '', ownerOrgTeam: '' })
const props = withDefaults(defineProps<{ filters: TaskListFilters; keyword?: string }>(), { filters: () => ({ status: [], taskCategory: '', taskType: '', priority: '', creatorId: '', assigneeId: '', dateFrom: '', dateTo: '', overdueOnly: false, ownerDepartment: '', ownerOrgTeam: '' }), keyword: '' })
const emit = defineEmits<{ apply: [TaskListFilters, string]; reset: [TaskListFilters, string]; close: [] }>()
const cloneFilters = (filters: TaskListFilters): TaskListFilters => ({ ...filters, status: [...filters.status] })
const draft = reactive<TaskListFilters>(cloneFilters(props.filters))
const draftKeyword = ref(props.keyword)
watch(() => props.filters, (value) => Object.assign(draft, cloneFilters(value)), { deep: true })
watch(() => props.keyword, (value) => { draftKeyword.value = value })
function patch(partial: Partial<TaskListFilters>) { Object.assign(draft, partial) }
function toggleStatus(status: TaskStatus) { patch({ status: draft.status.includes(status) ? draft.status.filter((item) => item !== status) : [...draft.status, status] }) }
function onOwnerDepartmentChange(value: string) { patch({ ownerDepartment: value, ownerOrgTeam: '' }) }
function parseDateMs(value: string): number | null { if (!value) return null; const ms = Date.parse(value.includes('T') ? value : `${value}T00:00:00`); return Number.isFinite(ms) ? ms : null }
function formatDate(ms: number | null): string { if (ms == null) return ''; const date = new Date(ms); return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` }
const dateFromMs = computed(() => parseDateMs(draft.dateFrom))
const dateToMs = computed(() => parseDateMs(draft.dateTo))
function onDateFrom(value: number | null) { patch({ dateFrom: formatDate(value) }) }
function onDateTo(value: number | null) { patch({ dateTo: formatDate(value) }) }
const statusOptions = [{ value: 'Draft', label: '草稿' }, { value: 'PendingAssign', label: '待指派' }, { value: 'InProgress', label: '进行中' }, { value: 'PendingAudit', label: '待审核' }, { value: 'Completed', label: '已结单' }, { value: 'Archived', label: '已归档' }, { value: 'Blocked', label: '阻塞' }, { value: 'Cancelled', label: '已取消' }]
const taskCategoryOptions = [{ label: '常规任务', value: 'normal' }, { label: '定制任务', value: 'customization' }]
const taskTypeOptions = [{ label: '原有产品开发', value: 'ORIGINAL_PRODUCT_DEV' }, { label: '新品开发', value: 'NEW_PRODUCT_DEV' }, { label: '策划 SKU', value: 'SKU_PLANNING' }, { label: 'P 图任务', value: 'RETOUCH_TASK' }, { label: '客户定制', value: 'CUSTOMER_CUSTOMIZATION' }, { label: '常规定制', value: 'REGULAR_CUSTOMIZATION' }]
const priorityOptions = [{ label: '普通', value: 'normal' }, { label: '加急', value: 'high' }, { label: '出单画图', value: 'drawing' }]
const {
  creatorOptions,
  assigneeOptions,
  ownerDepartmentOptions,
  ownerTeamOptions,
  loadFilterOptions,
  loadError: optionsLoadError,
  loading: optionsLoading,
} = useTaskFilterOptions(true, '全部', () => draft.ownerDepartment ?? '')
const departmentSelectOptions = computed(() => ownerDepartmentOptions.value.map((item) => ({ label: item.label, value: item.value })))
const teamSelectOptions = computed(() => ownerTeamOptions.value.map((item) => ({ label: item.label, value: item.value })))
const creatorSelectOptions = computed(() => creatorOptions.value.filter((item) => item.value).map((item) => ({ label: item.label, value: item.value })))
const assigneeSelectOptions = computed(() => assigneeOptions.value.filter((item) => item.value).map((item) => ({ label: item.label, value: item.value })))
// 接口成功但返回空数组时下拉同样是空的，必须和「加载失败」分开告诉用户，否则只能看到空白。
const hasLoadedOptions = computed(() => Boolean(
  departmentSelectOptions.value.length
  || teamSelectOptions.value.length
  || creatorSelectOptions.value.length
  || assigneeSelectOptions.value.length,
))
function apply() { emit('apply', cloneFilters(draft), draftKeyword.value.trim()) }
function reset() { const next = emptyFilters(); Object.assign(draft, next); draftKeyword.value = ''; emit('reset', next, '') }
</script>

<style scoped>
.filter-panel{height:100%;display:grid;grid-template-rows:auto 1fr;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.filter-heading{display:flex;align-items:center;justify-content:space-between;padding:20px 22px;border-bottom:1px solid rgb(var(--yb-border))}.filter-heading p{margin:0;color:rgb(var(--yb-brand));font-size:11px;font-weight:850;letter-spacing:.1em}.filter-heading h2{margin:3px 0 0;font-size:22px}.filter-heading button{width:40px;height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font-size:24px;cursor:pointer}.filter-form{min-height:0;display:grid;grid-template-rows:1fr auto}.filter-scroll{min-height:0;overflow:auto}.filter-section{display:grid;grid-template-columns:1fr 1fr;gap:4px 13px;padding:18px 22px;border-bottom:1px solid rgb(var(--yb-border))}.filter-section h3{grid-column:1/-1;margin:0 0 7px;font-size:13px}.wide{grid-column:1/-1}.status-options{display:flex;flex-wrap:wrap;gap:7px}.status-option{min-height:32px;padding:0 11px;border:1px solid rgb(var(--yb-border));border-radius:999px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-muted));font-size:12px;cursor:pointer}.status-option.active{border-color:rgb(var(--yb-brand-border));background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-weight:750}.options-status{display:flex;flex-wrap:wrap;align-items:center;gap:9px;padding:12px 22px;border-bottom:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-secondary));font-size:12px;line-height:1.55}.options-status.is-error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.options-status button{min-height:30px;padding:0 11px;border:1px solid currentColor;border-radius:8px;background:transparent;color:inherit;font-size:12px;font-weight:700;cursor:pointer}
.filter-actions{position:sticky;bottom:0;display:flex;justify-content:flex-end;gap:10px;padding:15px 22px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}.w-full{width:100%}@media(max-width:520px){.filter-section{grid-template-columns:1fr}.filter-section h3,.wide{grid-column:auto}.filter-heading{padding:16px}.filter-section{padding:16px}}
</style>
