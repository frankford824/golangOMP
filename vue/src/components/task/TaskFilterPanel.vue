<template>
  <div class="filter-panel">
    <NForm label-placement="top" size="medium" :show-feedback="false" class="filter-form">
      <div class="filter-grid">
        <NFormItem label="关键词">
          <NInput
            :value="draftKeyword"
            clearable
            placeholder="任务号、SKU、任务名…"
            @update:value="draftKeyword = $event || ''"
          />
        </NFormItem>
        <NFormItem label="任务分组">
          <NSelect
            :value="draft.taskCategory || null"
            clearable
            placeholder="全部分组"
            :options="taskCategoryOptions"
            @update:value="patch({ taskCategory: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="任务状态" class="span-2">
          <NSelect
            :value="draft.status"
            multiple
            clearable
            placeholder="全部状态"
            :options="statusOptions"
            @update:value="patch({ status: ($event || []) as TaskStatus[] })"
          />
        </NFormItem>
        <NFormItem label="任务类型">
          <NSelect
            :value="draft.taskType || null"
            clearable
            placeholder="全部类型"
            :options="taskTypeOptions"
            @update:value="patch({ taskType: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="优先级">
          <NSelect
            :value="draft.priority || null"
            clearable
            placeholder="全部"
            :options="priorityOptions"
            @update:value="patch({ priority: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="归属部门">
          <NSelect
            :value="draft.ownerDepartment || null"
            clearable
            filterable
            placeholder="全部"
            :options="departmentSelectOptions"
            @update:value="onOwnerDepartmentChange(String($event || ''))"
          />
        </NFormItem>
        <NFormItem label="归属团队">
          <NSelect
            :value="draft.ownerOrgTeam || null"
            clearable
            filterable
            placeholder="全部"
            :options="teamSelectOptions"
            @update:value="patch({ ownerOrgTeam: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="创建人">
          <NSelect
            :value="draft.creatorId || null"
            clearable
            filterable
            placeholder="创建人"
            :options="creatorSelectOptions"
            @update:value="patch({ creatorId: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="设计师">
          <NSelect
            :value="draft.assigneeId || null"
            clearable
            filterable
            placeholder="设计师"
            :options="assigneeSelectOptions"
            @update:value="patch({ assigneeId: String($event || '') })"
          />
        </NFormItem>
        <NFormItem label="创建起始">
          <NDatePicker
            :value="dateFromMs"
            type="date"
            clearable
            class="w-full"
            @update:value="onDateFrom"
          />
        </NFormItem>
        <NFormItem label="创建结束">
          <NDatePicker
            :value="dateToMs"
            type="date"
            clearable
            class="w-full"
            @update:value="onDateTo"
          />
        </NFormItem>
        <NFormItem label="其他">
          <NCheckbox :checked="draft.overdueOnly" @update:checked="patch({ overdueOnly: !!$event })">
            仅逾期
          </NCheckbox>
        </NFormItem>
      </div>
      <div class="filter-actions">
        <NButton @click="reset">重置</NButton>
        <NButton type="primary" @click="apply">应用筛选</NButton>
      </div>
    </NForm>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { NButton, NCheckbox, NDatePicker, NForm, NFormItem, NInput, NSelect } from 'naive-ui'
import type { ActiveTaskStatus as TaskStatus } from '@/domain/types/task'
import { useTaskFilterOptions } from '@/composables/useTaskFilterOptions'
import { useOrgOwnershipFilterOptions } from '@/composables/useOrgOwnershipFilterOptions'

export interface TaskListFilters {
  status: TaskStatus[]
  taskCategory: string
  taskType: string
  priority: string
  creatorId: string
  assigneeId: string
  dateFrom: string
  dateTo: string
  overdueOnly: boolean
  ownerDepartment: string
  ownerOrgTeam: string
}

const props = withDefaults(
  defineProps<{ filters: TaskListFilters; keyword?: string }>(),
  {
    filters: () => ({
      status: [],
      taskCategory: '',
      taskType: '',
      priority: '',
      creatorId: '',
      assigneeId: '',
      dateFrom: '',
      dateTo: '',
      overdueOnly: false,
      ownerDepartment: '',
      ownerOrgTeam: '',
    }),
    keyword: '',
  },
)

const emit = defineEmits<{
  apply: [TaskListFilters, string]
  reset: [TaskListFilters, string]
}>()

const emptyFilters = (): TaskListFilters => ({
  status: [], taskCategory: '', taskType: '', priority: '', creatorId: '', assigneeId: '',
  dateFrom: '', dateTo: '', overdueOnly: false, ownerDepartment: '', ownerOrgTeam: '',
})
const cloneFilters = (filters: TaskListFilters): TaskListFilters => ({ ...filters, status: [...filters.status] })
const draft = reactive<TaskListFilters>(cloneFilters(props.filters))
const draftKeyword = ref(props.keyword)

watch(
  () => props.filters,
  (value) => Object.assign(draft, cloneFilters(value)),
  { deep: true },
)
watch(() => props.keyword, (value) => { draftKeyword.value = value })

function patch(partial: Partial<TaskListFilters>) {
  Object.assign(draft, partial)
}

function onOwnerDepartmentChange(v: string) {
  patch({ ownerDepartment: v, ownerOrgTeam: '' })
}

function parseDateMs(value: string): number | null {
  if (!value) return null
  const ms = Date.parse(value.includes('T') ? value : `${value}T00:00:00`)
  return Number.isFinite(ms) ? ms : null
}

function formatDate(ms: number | null): string {
  if (ms == null) return ''
  const d = new Date(ms)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const dateFromMs = computed(() => parseDateMs(draft.dateFrom))
const dateToMs = computed(() => parseDateMs(draft.dateTo))
function onDateFrom(value: number | null) { patch({ dateFrom: formatDate(value) }) }
function onDateTo(value: number | null) { patch({ dateTo: formatDate(value) }) }

const statusOptions = [
  { value: 'Draft', label: '草稿' },
  { value: 'PendingAssign', label: '待指派' },
  { value: 'InProgress', label: '进行中' },
  { value: 'PendingAudit', label: '待审核' },
  { value: 'Completed', label: '已完成' },
  { value: 'Archived', label: '已归档' },
  { value: 'Blocked', label: '阻塞' },
  { value: 'Cancelled', label: '已取消' },
]
const taskCategoryOptions = [
  { label: '常规任务', value: 'normal' },
  { label: '定制任务', value: 'customization' },
]
const taskTypeOptions = [
  { label: '原有产品开发', value: 'ORIGINAL_PRODUCT_DEV' },
  { label: '新品开发', value: 'NEW_PRODUCT_DEV' },
  { label: '策划 SKU', value: 'SKU_PLANNING' },
  { label: 'P 图任务', value: 'RETOUCH_TASK' },
  { label: '客户定制', value: 'CUSTOMER_CUSTOMIZATION' },
  { label: '常规定制', value: 'REGULAR_CUSTOMIZATION' },
]
const priorityOptions = [
  { label: '低', value: 'low' },
  { label: '普通', value: 'normal' },
  { label: '高', value: 'high' },
  { label: '加急', value: 'critical' },
]

const { departmentOptions, teamOptions } = useOrgOwnershipFilterOptions(() => draft.ownerDepartment ?? '')
const { creatorOptions, assigneeOptions } = useTaskFilterOptions(true, '全部')
const departmentSelectOptions = computed(() => departmentOptions.value.map((item) => ({ label: item.label, value: item.value })))
const teamSelectOptions = computed(() => teamOptions.value.map((item) => ({ label: item.label, value: item.value })))
const creatorSelectOptions = computed(() => creatorOptions.value.filter((item) => item.value).map((item) => ({ label: item.label, value: item.value })))
const assigneeSelectOptions = computed(() => assigneeOptions.value.filter((item) => item.value).map((item) => ({ label: item.label, value: item.value })))

function apply() { emit('apply', cloneFilters(draft), draftKeyword.value.trim()) }
function reset() {
  const next = emptyFilters()
  Object.assign(draft, next)
  draftKeyword.value = ''
  emit('reset', next, '')
}
</script>

<style scoped>
.filter-panel {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 14px;
  background: rgb(var(--yb-surface-soft));
  padding: 16px 18px;
}
.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(12rem, 1fr));
  gap: 8px 16px;
}
.span-2 { grid-column: span 2; }
.filter-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}
.w-full { width: 100%; }
@media (max-width: 1100px) {
  .filter-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .span-2 { grid-column: span 2; }
}
@media (max-width: 700px) {
  .filter-grid { grid-template-columns: 1fr; }
  .span-2 { grid-column: auto; }
}
</style>
