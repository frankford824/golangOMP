<template>
  <div class="filter-bar">
    <div class="filter-field">
      <span class="field-label">任务分组</span>
      <BaseSelect
        :model-value="filters.taskCategory"
        clearable
        placeholder="全部分组"
        :options="taskCategoryOptions"
        @update:model-value="patchFilters({ taskCategory: String($event) })"
      />
    </div>
    <div class="filter-field filter-field--status">
      <span class="field-label">任务状态</span>
      <TaskStatusMultiSelect
        :model-value="filters.status"
        :options="statusOptions"
        @update:model-value="patchFilters({ status: $event })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">任务类型</span>
      <BaseSelect
        :model-value="filters.taskType"
        clearable
        placeholder="任务类型"
        :options="taskTypeOptions"
        @update:model-value="patchFilters({ taskType: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">优先级</span>
      <BaseSelect
        :model-value="filters.priority"
        clearable
        placeholder="全部"
        :options="priorityOptions"
        @update:model-value="patchFilters({ priority: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">归属部门</span>
      <BaseSelect
        :model-value="filters.ownerDepartment"
        clearable
        placeholder="全部"
        :options="departmentOptions"
        @update:model-value="onOwnerDepartmentChange(String($event))"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">归属团队</span>
      <BaseSelect
        :model-value="filters.ownerOrgTeam"
        clearable
        placeholder="全部"
        :options="teamOptionsFiltered"
        @update:model-value="patchFilters({ ownerOrgTeam: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">创建人</span>
      <BaseSelect
        :model-value="filters.creatorId"
        clearable
        filterable
        filter-placeholder="输入姓名筛选"
        placeholder="创建人"
        :options="creatorOptions"
        @update:model-value="patchFilters({ creatorId: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">设计师</span>
      <BaseSelect
        :model-value="filters.assigneeId"
        clearable
        filterable
        filter-placeholder="输入姓名筛选"
        placeholder="设计师"
        :options="assigneeOptions"
        @update:model-value="patchFilters({ assigneeId: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">仓库接收</span>
      <BaseSelect
        :model-value="filters.warehouseStatus"
        clearable
        placeholder="仓库接收"
        :options="warehouseStatusOptions"
        @update:model-value="patchFilters({ warehouseStatus: String($event) })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">起始日期</span>
      <BaseDatePicker
        :model-value="filters.dateFrom"
        @update:model-value="patchFilters({ dateFrom: $event })"
      />
    </div>
    <div class="filter-field">
      <span class="field-label">结束日期</span>
      <BaseDatePicker
        :model-value="filters.dateTo"
        @update:model-value="patchFilters({ dateTo: $event })"
      />
    </div>
    <div class="filter-bar-trailing">
      <label class="filter-overdue">
        <input
          :checked="filters.overdueOnly"
          type="checkbox"
          class="filter-overdue-input"
          @change="onOverdueChange"
        />
        <span>仅逾期</span>
      </label>
      <BaseButton size="sm" variant="secondary" @click="reset">清空全部筛选</BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { LegacyTaskStatus as TaskStatus } from '@/domain/types/task'
import BaseDatePicker from '@/components/base/BaseDatePicker.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import TaskStatusMultiSelect from '@/components/task/TaskStatusMultiSelect.vue'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { useTaskCreatorOptions } from '@/composables/useTaskCreatorOptions'
import { useOrgOwnershipFilterOptions } from '@/composables/useOrgOwnershipFilterOptions'

export interface TaskListFilters {
  status: TaskStatus[]
  taskCategory: string
  taskType: string
  priority: string
  /** 创建人用户 id，对应 GET /v1/tasks `creator_id` */
  creatorId: string
  assigneeId: string
  warehouseStatus: string
  dateFrom: string
  dateTo: string
  overdueOnly: boolean
  /** 规范归属：部门筛选 */
  ownerDepartment: string
  /** 规范归属：组织树团队筛选 */
  ownerOrgTeam: string
}

const props = withDefaults(
  defineProps<{ filters: TaskListFilters }>(),
  {
    filters: () => ({
      status: [],
      taskCategory: '',
      taskType: '',
      priority: '',
      creatorId: '',
      assigneeId: '',
      warehouseStatus: '',
      dateFrom: '',
      dateTo: '',
      overdueOnly: false,
      ownerDepartment: '',
      ownerOrgTeam: '',
    }),
  },
)

const emit = defineEmits<{ 'update:filters': [TaskListFilters] }>()

function patchFilters(partial: Partial<TaskListFilters>) {
  emit('update:filters', { ...props.filters, ...partial })
}

const { departmentOptions, teamOptions: teamOptionsFiltered } =
  useOrgOwnershipFilterOptions(() => props.filters.ownerDepartment ?? '')

function onOwnerDepartmentChange(v: string) {
  patchFilters({ ownerDepartment: v, ownerOrgTeam: '' })
}

function onOverdueChange(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  patchFilters({ overdueOnly: checked })
}

const statusOptions: { value: TaskStatus; label: string }[] = [
  { value: 'Draft', label: '草稿' },
  { value: 'PendingAssign', label: '待指派' },
  { value: 'InProgress', label: '进行中' },
  { value: 'PendingAuditA', label: '待审核' },
  { value: 'RejectedByAuditA', label: '审核打回' },
  { value: 'PendingOutsource', label: '待定制' },
  { value: 'Outsourcing', label: '定制中' },
  { value: 'PendingWarehouseReceive', label: '待仓库接收' },
  { value: 'Completed', label: '已完成' },
  { value: 'Archived', label: '已归档' },
  { value: 'Blocked', label: '阻塞' },
  { value: 'Cancelled', label: '已取消' },
]

const taskCategoryOptions: BaseSelectOption[] = [
  { label: '常规任务', value: 'normal' },
  { label: '定制任务', value: 'customization' },
]

const taskTypeOptions: BaseSelectOption[] = [
  { label: '原有产品开发', value: 'ORIGINAL_PRODUCT_DEV' },
  { label: '新品开发', value: 'NEW_PRODUCT_DEV' },
  { label: '采购任务', value: 'PURCHASE_TASK' },
  { label: 'P 图任务', value: 'RETOUCH_TASK' },
  { label: '客户定制', value: 'CUSTOMER_CUSTOMIZATION' },
  { label: '常规定制', value: 'REGULAR_CUSTOMIZATION' },
]

const priorityOptions: BaseSelectOption[] = [
  { label: '低', value: 'low' },
  { label: '普通', value: 'normal' },
  { label: '高', value: 'high' },
  { label: '加急', value: 'critical' },
]

const { creatorOptions } = useTaskCreatorOptions(true, '全部')
const { assigneeOptions } = useDesignerOptions(true, '全部')

const warehouseStatusOptions: BaseSelectOption[] = [
  { label: '待接收', value: 'pending' },
  { label: '已接收', value: 'received' },
  { label: '已退回', value: 'returned' },
  { label: '已归档', value: 'archived' },
]

function reset() {
  emit('update:filters', {
    status: [],
    taskCategory: '',
    taskType: '',
    priority: '',
    creatorId: '',
    assigneeId: '',
    warehouseStatus: '',
    dateFrom: '',
    dateTo: '',
    overdueOnly: false,
    ownerDepartment: '',
    ownerOrgTeam: '',
  })
}
</script>

<style scoped>
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}
.filter-field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  width: 10rem;
}
.filter-field--status {
  width: 12.5rem;
}
.field-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(100 116 139);
}
.field-hint-error {
  margin: 0;
  font-size: 0.6875rem;
  color: rgb(220 38 38);
}
/* 与筛选项底缘对齐的尾区：复选+文案与按钮同一水平中线对齐 */
.filter-bar-trailing {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
  min-height: 2rem;
}
.filter-overdue {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1.25;
  color: rgb(100 116 139);
  cursor: pointer;
  user-select: none;
}
.filter-overdue-input {
  width: 0.875rem;
  height: 0.875rem;
  margin: 0;
  flex-shrink: 0;
  border-radius: 0.2rem;
  border: 1px solid rgb(203 213 225);
  accent-color: rgb(15 23 42);
  cursor: pointer;
  vertical-align: middle;
}
</style>
