<template>
  <div class="task-list-view">
    <!-- 顶栏 + 搜索/过滤：统一卡片容器 -->
    <div class="header-card">
      <div class="page-header">
        <h2 class="page-title">任务中心</h2>
        <div class="page-header-actions">
          <BaseButton
            size="sm"
            variant="secondary"
            :loading="refreshingList"
            @click="refreshList(true)"
          >
            {{ refreshingList ? '刷新中...' : '刷新列表' }}
          </BaseButton>
          <BaseButton v-if="can('task.create')" variant="primary" @click="goCreate">
            创建任务
          </BaseButton>
          <BaseButton
            v-if="can('task.create')"
            variant="secondary"
            @click="goExcelAssistCreate"
          >
            Excel 辅助创建
          </BaseButton>
        </div>
      </div>
      <div class="task-category-switch" aria-label="任务分类">
        <BaseButton
          v-for="option in taskCategoryOptions"
          :key="option.value"
          size="sm"
          :variant="filters.taskCategory === option.value ? 'secondary' : 'ghost'"
          :class="{ 'task-category-active': filters.taskCategory === option.value }"
          @click="setTaskCategory(option.value)"
        >
          {{ option.label }}
        </BaseButton>
      </div>
      <div class="task-tabs" aria-label="任务中心列表范围">
        <BaseButton
          v-for="tab in taskTabs"
          :key="tab.value"
          size="sm"
          :variant="activeTab === tab.value ? 'secondary' : 'ghost'"
          :class="{ 'task-tab-active': activeTab === tab.value }"
          @click="setTaskTab(tab.value)"
        >
          {{ tab.label }}
        </BaseButton>
      </div>
      <div class="toolbar">
        <BaseInput
          v-model="searchKeyword"
          placeholder="搜索任务号、SKU、产品名称"
          class="search-input w-72"
          @input="debouncedSearch"
        />
        <BaseButton
          size="sm"
          variant="secondary"
          class="advanced-filter-toggle"
          :class="{
            'advanced-filter-toggle--active': advancedFilterOpen || activeAdvancedFilterCount > 0,
          }"
          @click="advancedFilterOpen = !advancedFilterOpen"
        >
          {{
            advancedFilterOpen
              ? '收起筛选'
              : activeAdvancedFilterCount > 0
                ? `筛选 ${activeAdvancedFilterCount}`
                : '高级筛选'
          }}
        </BaseButton>
      </div>
      <div v-show="advancedFilterOpen" class="filter-bar-wrap">
        <TaskFilterBar v-model:filters="filters" @update:filters="page = 1" />
      </div>
      <p v-if="tabStatusScopeHint" class="tab-status-scope-hint" role="status">
        {{ tabStatusScopeHint }}
      </p>
    </div>

    <!-- 批量操作条（有选中时滑出） -->
    <Transition name="batch-bar-slide">
      <div v-if="selectedIds.size > 0" class="batch-action-bar">
        <span class="batch-count">已选 {{ selectedIds.size }} 条</span>
        <!-- v0.6 对齐：2026-03-18 批量提醒/指派 -->
        <BaseButton
          v-if="canBatchAssign"
          size="sm"
          variant="secondary"
          :disabled="batchReminding"
          @click="batchRemind"
        >
          {{ batchReminding ? '提醒中...' : '批量提醒' }}
        </BaseButton>
        <BaseButton
          v-if="canBatchAssign"
          size="sm"
          variant="secondary"
          @click="showBatchAssign = true"
        >
          批量指派
        </BaseButton>
        <BaseButton
          v-if="can('warehouse.receive')"
          size="sm"
          variant="secondary"
          :disabled="batchReceiving"
          @click="batchReceive"
        >
          {{ batchReceiving ? '接收中...' : '批量接收' }}
        </BaseButton>
        <BaseButton size="sm" variant="ghost" @click="selectedIds.clear()">
          取消选中
        </BaseButton>
      </div>
    </Transition>
    <p v-if="listActionError" class="list-action-error">{{ listActionError }}</p>

    <!-- 主内容区（三态统一包裹） -->
    <AsyncStateWrapper
      :loading="tasksStore.loading"
      :error="tasksStore.loadError"
      :empty="filteredList.length === 0"
      empty-title="暂无任务"
      :empty-description="emptyDescription"
      :skeleton-rows="6"
      @retry="refreshList(true)"
    >
      <template #empty-action>
        <BaseButton
          v-if="can('task.create')"
          variant="primary"
          size="sm"
          @click="goCreate"
        >
          创建任务
        </BaseButton>
      </template>

      <!-- 统一卡片视图 -->
      <div class="task-cards">
        <div
          v-for="task in pagedList"
          :key="task.id"
          class="task-card"
          :class="{
            'task-card--selected': selectedIds.has(task.id),
            'task-card--overdue': isOverdue(task),
            'task-card--claimable': canClaimTask(task),
          }"
          @click="goDetail(task)"
        >
          <div class="card-row card-row-top">
            <label class="card-check" @click.stop>
              <input
                type="checkbox"
                class="checkbox"
                :checked="selectedIds.has(task.id)"
                @change="toggleSelect(task.id)"
              />
            </label>
            <div class="card-tags">
              <span class="task-category-pill" :class="taskCategoryClass(task)">
                {{ taskCategoryLabel(task) }}
              </span>
              <WorkflowLaneTag
                v-if="shouldShowWorkflowLaneTagOnCard(task)"
                :lane="task.workflowLane"
              />
              <TaskTypeBadge :type="task.businessType ?? task.taskType" />
            </div>
          </div>
          <div class="card-no-row">
            <span class="card-no">{{ task.taskNo }}</span>
          </div>
          <div
            class="card-product"
            :title="task.productName?.trim() ? task.productName : undefined"
          >
            {{ task.productName }}
          </div>
          <div class="card-status-row flex flex-wrap items-center gap-1 mt-1">
            <TaskMainStatusBadge
              v-if="task.mainStatus"
              :status="task.mainStatus"
              :label-override="getTaskCenterCardStatusLabel(task) ?? undefined"
            />
            <TaskStatusTag v-else :status="task.status" />
            <FilingStatusBadge
              v-if="task.filing_status || isRetouchTask(task)"
              :status="task.filing_status"
              :task-type="task.businessType ?? task.taskType"
              :missing-fields-summary="task.missing_fields_summary_cn"
              :error-message="task.filing_error_message"
            />
          </div>
          <div class="card-meta-block">
            <div class="card-meta-line card-meta-line--ownership">
              <span class="card-meta-key">归属</span>
              <span class="card-meta-value" :title="ownershipPrimary(task)">{{ ownershipPrimary(task) }}</span>
            </div>
            <div class="card-meta-line card-meta-line--sku">
              <span class="card-meta-key">SKU</span>
              <span class="card-meta-value" :title="displaySku(task)">{{ displaySku(task) }}</span>
            </div>
            <div class="card-meta-line card-meta-line--creator">
              <span class="card-meta-key">创建</span>
              <span class="card-meta-value" :title="taskCreatorDisplayName(task)">{{
                taskCreatorDisplayName(task)
              }}</span>
            </div>
            <div
              v-if="shouldShowDesignerMetaOnTaskCenterCard(task)"
              class="card-meta-line card-meta-line--design"
            >
              <span class="card-meta-key">设计</span>
              <span class="card-meta-value" :title="taskDesignerDisplayName(task)">{{
                taskDesignerDisplayName(task)
              }}</span>
            </div>
          </div>
          <div class="card-row card-row-bottom">
            <span class="card-updated">更新于 {{ formatDate(task.updatedAt) }}</span>
            <span :class="isOverdue(task) ? 'card-due card-due-overdue' : 'card-due'">
              截止 {{ task.dueAt ? formatDate(task.dueAt) : '-' }}
            </span>
            <BaseButton
              v-if="canClaimTask(task)"
              size="sm"
              variant="secondary"
              :loading="claimingTaskId === task.id"
              :disabled="Boolean(claimingTaskId)"
              @click.stop="claimTask(task)"
            >
              {{ taskCenterClaimButtonLabel(task, claimingTaskId === task.id) }}
            </BaseButton>
          </div>
        </div>
      </div>
    </AsyncStateWrapper>

    <!-- 分页页脚：翻页按钮，每页固定展示，不追加 -->
    <div v-if="filteredList.length > 0" class="footer-card">
      <span class="text-xs text-slate-500">共 {{ tasksStore.listTotal }} 条</span>
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-1 text-xs text-slate-500">
          每页
          <BaseSelect
            v-model="pageSize"
            class="task-page-size-select"
            :options="pageSizeOptions"
          />
          条
        </label>
        <div class="pagination flex items-center gap-2">
          <BaseButton
            type="button"
            variant="ghost"
            size="sm"
            class="pager-btn"
            :disabled="page <= 1 || refreshingList"
            @click="goToPage(page - 1)"
          >
            上一页
          </BaseButton>
          <span class="pager-info text-xs text-slate-500">
            第 {{ page }} / {{ totalPages }} 页，已显示 {{ visibleCount }} / {{ tasksStore.listTotal }}
          </span>
          <label class="page-jump text-xs text-slate-500">
            跳至
            <BaseInput
              v-model="jumpPage"
              type="number"
              class="page-jump-input"
              :disabled="refreshingList"
              @keyup.enter="jumpToPage"
            />
            页
          </label>
          <BaseButton
            type="button"
            variant="ghost"
            size="sm"
            class="pager-btn"
            :disabled="refreshingList"
            @click="jumpToPage"
          >
            跳转
          </BaseButton>
          <BaseButton
            type="button"
            variant="ghost"
            size="sm"
            class="pager-btn"
            :disabled="page >= totalPages || refreshingList"
            @click="goToPage(page + 1)"
          >
            下一页
          </BaseButton>
        </div>
      </div>
    </div>

    <TaskCreateModal
      v-if="can('task.create')"
      v-model="showCreateModal"
      :initial-draft-id="queryString(route.query.draft_id)"
      @created="handleTaskCreated"
    />

    <!-- v0.6 批量指派弹窗 -->
    <DesignerSelectDialog
      v-model="showBatchAssign"
      :designers="batchDesignerOptions"
      :loading="batchDesignersLoading"
      @confirm="onBatchAssignConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount, onMounted, reactive, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTasksStore } from '@/stores/tasks'
import { usePermissionsStore } from '@/stores/permissions'
import type { Task, LegacyTaskStatus } from '@/domain/types/task'
import { isDoneStatus, shouldShowDesignerMetaOnTaskCenterCard } from '@/domain/task-actions'
import { usePermission } from '@/composables/usePermission'
import type { TaskListFilters } from '@/components/task/TaskFilterBar.vue'
import TaskFilterBar from '@/components/task/TaskFilterBar.vue'
import TaskStatusTag from '@/components/task/TaskStatusTag.vue'
import TaskTypeBadge from '@/components/task/TaskTypeBadge.vue'
import WorkflowLaneTag from '@/components/task/WorkflowLaneTag.vue'
import TaskMainStatusBadge from '@/components/business/TaskMainStatusBadge.vue'
import FilingStatusBadge from '@/components/business/FilingStatusBadge.vue'
import AsyncStateWrapper from '@/components/base/AsyncStateWrapper.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'
import TaskCreateModal from '@/components/task/TaskCreateModal.vue'
import DesignerSelectDialog from '@/components/task/DesignerSelectDialog.vue'
import { tasksApi } from '@/services/api/tasksApi'
import type { TaskListParams } from '@/services/apiTypes'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import {
  formatTaskDueAtDisplay,
  isOverdueByTimestamp as checkOverdue,
} from '@/utils/date'
import { getTaskOwnershipDisplay } from '@/domain/task-ownership'
import { formatTaskActionDenyMessage } from '@/domain/task-action-deny'
import { taskCreatorDisplayName, taskDesignerDisplayName } from '@/domain/task-actors'
import { getTaskCenterCardStatusLabel } from '@/domain/task-center-card-status'
import {
  canClaimTaskFromCenter,
  isCustomizationModuleClaimTask,
  taskCenterClaimButtonLabel,
  userCanActAsCustomizationClaimActor,
  userIsPureDesignerForCustomizationClaim,
} from '@/domain/task-center-claim'
import { PermissionEnum } from '@/types'

const router = useRouter()
const route = useRoute()
const tasksStore = useTasksStore()
const permissionsStore = usePermissionsStore()
const { can, canAccessAction } = usePermission()

// ── 状态记忆：从路由 query 或 sessionStorage 恢复 ──────────────────────────
const STORAGE_KEY = 'task-list-state'
type TaskListTab = 'all' | 'pool' | 'mine' | 'archived' | 'terminated'

const taskTabs: Array<{ label: string; value: TaskListTab }> = [
  { label: '全任务', value: 'all' },
  { label: '未指派任务', value: 'pool' },
  { label: '我的任务', value: 'mine' },
  { label: '已归档', value: 'archived' },
  { label: '已终止', value: 'terminated' },
]

const emptyDescription = computed(() => {
  if (activeTab.value === 'terminated') return '当前暂无已终止任务'
  if (activeTab.value !== 'pool') return '当前筛选条件下没有任务'
  return '当前暂无可接单任务'
})

const taskCategoryOptions = [
  { label: '全部', value: '' },
  { label: '常规任务', value: 'normal' },
  { label: '定制任务', value: 'customization' },
]

const pageSizeOptions: BaseSelectOption[] = [
  { value: 20, label: '20' },
  { value: 50, label: '50' },
  { value: 100, label: '100' },
]

function queryString(value: unknown): string {
  return Array.isArray(value) ? String(value[0] ?? '') : String(value ?? '')
}

function parseStatusQuery(value: unknown): LegacyTaskStatus[] {
  const raw = queryString(value).trim()
  if (!raw) return []
  return raw
    .split(',')
    .map((token) => token.trim())
    .filter(Boolean) as LegacyTaskStatus[]
}

function parseTaskTab(value: unknown): TaskListTab {
  const raw = queryString(value)
  return raw === 'pool' || raw === 'mine' || raw === 'archived' || raw === 'terminated'
    ? raw
    : 'all'
}

function restoreState() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as Record<string, unknown>
  } catch {
    // ignore
  }
  return {}
}

function saveState() {
  try {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        searchKeyword: searchKeyword.value,
        activeTab: activeTab.value,
        filters: filters.value,
        page: page.value,
        pageSize: pageSize.value,
        sortKey: sortKey.value,
        sortOrder: sortOrder.value,
      }),
    )
  } catch {
    // ignore
  }
}

const saved = restoreState()

const searchKeyword = ref((route.query.q as string) || (saved.searchKeyword as string) || '')
const activeTab = ref<TaskListTab>(parseTaskTab(route.query.tab ?? saved.activeTab))
const sortKey = ref<'taskNo' | 'updatedAt' | 'dueAt'>((saved.sortKey as 'taskNo' | 'updatedAt' | 'dueAt') ?? 'updatedAt')
const sortOrder = ref<'asc' | 'desc'>((saved.sortOrder as 'asc' | 'desc') ?? 'desc')
const page = ref((saved.page as number) ?? 1)
const pageSize = ref((saved.pageSize as number) ?? 20)
const showCreateModal = ref(false)
const showBatchAssign = ref(false)
const batchReminding = ref(false)
const batchReceiving = ref(false)
const refreshingList = ref(false)
const advancedFilterOpen = ref(false)
const claimingTaskId = ref<string | null>(null)
const jumpPage = ref<number | string>(page.value)
const {
  designers: batchDesignerOptions,
  loading: batchDesignersLoading,
  loadDesigners: loadBatchDesigners,
} = useDesignerOptions({
  includeEmpty: false,
  autoLoad: false,
  requiredActions: ['task.assign', 'task.assign.team', 'task.assign.department'],
})
const listActionError = ref('')
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

const canBatchAssign = computed(
  () =>
    can('task.assign') ||
    canAccessAction('task.assign.department') ||
    canAccessAction('task.assign.team'),
)

/** 方案 B：搜索防抖，300ms 后触发服务端检索 */
function debouncedSearch() {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  page.value = 1
  saveState()
  searchDebounceTimer = setTimeout(() => {
    searchDebounceTimer = null
    refreshList(true)
  }, 300)
}

const selectedIds = reactive(new Set<string>())

/** 后端 batch 接口要求 task_ids 为 JSON 数字数组 ([]int64)，不能为字符串 */
function selectedIdsAsNumericOrError(): number[] | null {
  const ids = Array.from(selectedIds)
  const nums = ids.map((id) => parseInt(id, 10))
  if (nums.some((n) => Number.isNaN(n))) {
    listActionError.value = '选中任务 ID 无效，请刷新后重试'
    return null
  }
  return nums
}

const STATUS_FILTER_EXPANSION: Partial<Record<LegacyTaskStatus, LegacyTaskStatus[]>> = {
  InProgress: ['InProgress', 'Assigned'],
  PendingAuditA: ['PendingAuditA', 'PendingAuditB'],
  RejectedByAuditA: ['RejectedByAuditA', 'RejectedByAuditB'],
  Outsourcing: [
    'Outsourcing',
    'PendingOutsourceReview',
    'PendingCustomizationReview',
    'PendingCustomizationProduction',
    'PendingEffectRevision',
    'PendingProductionTransfer',
  ],
  Completed: ['Completed', 'PendingClose'],
}

function expandStatusFilter(statuses: LegacyTaskStatus[]): LegacyTaskStatus[] {
  const out = new Set<LegacyTaskStatus>()
  for (const status of statuses) {
    const expanded = STATUS_FILTER_EXPANSION[status]
    if (expanded?.length) {
      for (const item of expanded) out.add(item)
      continue
    }
    out.add(status)
  }
  return Array.from(out)
}

const defaultTaskFilters: TaskListFilters = {
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
}

/**
 * “已归档/已结束”tab 默认展示所有终态任务：
 * - Archived: 严格归档
 * - Completed / PendingClose: 已完成或待结单
 * - Cancelled: 已取消
 */
const ARCHIVED_TAB_DEFAULT_STATUSES: LegacyTaskStatus[] = [
  'PendingClose',
  'Completed',
  'Archived',
  'Cancelled',
]
const savedFiltersRaw = saved.filters && typeof saved.filters === 'object' ? { ...saved.filters } : {}
delete (savedFiltersRaw as { productSource?: unknown }).productSource
const filters = ref<TaskListFilters>({
  ...defaultTaskFilters,
  ...(savedFiltersRaw as Partial<TaskListFilters>),
})

const CUSTOMIZATION_REVIEWER_ROLES = [
  'CustomizationReviewer',
  'customization_reviewer',
  'customizationreviewer',
] as const
const NORMAL_AUDIT_ROLES = ['Audit_A', 'Audit_B', 'audit_a', 'audit_b', 'auditor'] as const
const TASK_LIST_SCOPE_QUERY_KEYS = [
  'tab',
  'task_category',
  'status',
  'q',
  'task_type',
  'priority',
  'creator_id',
  'owner_department',
  'owner_org_team',
  'warehouse_status',
  'date_from',
  'date_to',
  'overdue',
] as const

function queryHasTaskListScope(query: Record<string, unknown>): boolean {
  return TASK_LIST_SCOPE_QUERY_KEYS.some((key) => {
    if (!(key in query) || query[key] == null) return false
    return queryString(query[key]).trim() !== ''
  })
}

function applyAuditRoleDefaultScope() {
  if (queryHasTaskListScope(route.query as Record<string, unknown>)) return
  const canReviewCustomization = permissionsStore.hasAnyRole(CUSTOMIZATION_REVIEWER_ROLES)
  const canReviewNormal = permissionsStore.hasAnyRole(NORMAL_AUDIT_ROLES)
  if (!canReviewCustomization && !canReviewNormal) return

  activeTab.value = 'all'
  if (canReviewCustomization && !canReviewNormal) {
    filters.value = {
      ...filters.value,
      taskCategory: 'customization',
      status: ['PendingCustomizationReview', 'PendingEffectReview'],
    }
    return
  }
  if (canReviewNormal && !canReviewCustomization) {
    filters.value = {
      ...filters.value,
      taskCategory: 'normal',
      status: ['PendingAuditA', 'PendingAuditB'],
    }
    return
  }
  filters.value = {
    ...filters.value,
    taskCategory: '',
    status: ['PendingAuditA', 'PendingAuditB', 'PendingCustomizationReview', 'PendingEffectReview'],
  }
}

applyAuditRoleDefaultScope()

const activeAdvancedFilterCount = computed(() => {
  const f = filters.value
  let count = 0
  if (f.taskCategory) count += 1
  if (f.status.length > 0) count += 1
  if (f.taskType) count += 1
  if (f.priority) count += 1
  if (f.ownerDepartment) count += 1
  if (f.ownerOrgTeam) count += 1
  if (f.creatorId) count += 1
  if (f.assigneeId) count += 1
  if (f.warehouseStatus) count += 1
  if (f.dateFrom || f.dateTo) count += 1
  if (f.overdueOnly) count += 1
  return count
})

if (typeof route.query.owner_department === 'string') {
  filters.value.ownerDepartment = route.query.owner_department
}
if (typeof route.query.owner_org_team === 'string') {
  filters.value.ownerOrgTeam = route.query.owner_org_team
}
if (typeof route.query.task_category === 'string') {
  filters.value.taskCategory = route.query.task_category
}
if (typeof route.query.warehouse_status === 'string') {
  filters.value.warehouseStatus = route.query.warehouse_status
}
if (typeof route.query.task_type === 'string') {
  filters.value.taskType = route.query.task_type
}
if (route.query.status != null) {
  filters.value.status = parseStatusQuery(route.query.status)
}
if (typeof route.query.priority === 'string') {
  filters.value.priority = route.query.priority
}
if (typeof route.query.creator_id === 'string') {
  filters.value.creatorId = route.query.creator_id
}
if (typeof route.query.date_from === 'string') {
  filters.value.dateFrom = route.query.date_from
}
if (typeof route.query.date_to === 'string') {
  filters.value.dateTo = route.query.date_to
}
if (typeof route.query.sort === 'string') {
  const raw = route.query.sort
  const direction = raw.startsWith('-') ? 'desc' : 'asc'
  const field = raw.startsWith('-') ? raw.slice(1) : raw
  const map: Record<string, typeof sortKey.value> = {
    task_no: 'taskNo',
    updated_at: 'updatedAt',
    due_at: 'dueAt',
  }
  if (map[field]) {
    sortKey.value = map[field]
    sortOrder.value = direction
  }
}

function setTaskTab(tab: TaskListTab) {
  if (activeTab.value === tab) return
  if (filters.value.status.length > 0) {
    filters.value = { ...filters.value, status: [] }
  }
  activeTab.value = tab
}

const tabStatusScopeHint = computed(() => {
  if (!filters.value.status.length) return ''
  if (activeTab.value === 'pool') {
    return '当前处于未指派任务页签，手动选择任务状态可能会替换页签默认范围。'
  }
  if (activeTab.value === 'archived' || activeTab.value === 'terminated') {
    return '当前页签默认限定特定状态范围，手动选择任务状态后将按所选状态筛选。'
  }
  return ''
})

function queryHasNonEmptyParam(query: Record<string, unknown>, key: string): boolean {
  if (!(key in query) || query[key] == null) return false
  return queryString(query[key]).trim() !== ''
}

function parseOverdueQuery(query: Record<string, unknown>): boolean {
  if (!queryHasNonEmptyParam(query, 'overdue')) return false
  const raw = queryString(query.overdue).trim().toLowerCase()
  return raw === 'true' || raw === '1'
}

function setTaskCategory(category: string) {
  if (filters.value.taskCategory === category) return
  filters.value = { ...filters.value, taskCategory: category }
}

/** 方案 B：构建服务端分页 + 搜索参数 */
function buildListParams(opt?: { page?: number; append?: boolean }): TaskListParams {
  const kw = searchKeyword.value.trim()
  const f = filters.value
  const params: TaskListParams = {
    page: opt?.page ?? page.value,
    page_size: pageSize.value,
  }
  if (kw) params.keyword = kw
  if (activeTab.value === 'mine') params.filter = 'mine'
  if (activeTab.value === 'pool') {
    if (f.taskCategory === 'customization') {
      params.designer_empty = true
    } else {
      params.status = 'PendingAssign'
    }
  }
  if (activeTab.value === 'archived' && !f.status.length) {
    params.status = ARCHIVED_TAB_DEFAULT_STATUSES.join(',')
  }
  if (activeTab.value === 'terminated') {
    const terminatedStatus = f.status.length ? expandStatusFilter(f.status).join(',') : 'Cancelled'
    params.status = terminatedStatus
  } else if (f.status.length) {
    params.status = expandStatusFilter(f.status).join(',')
  }
  if (f.taskType) {
    const map: Record<string, string> = {
      ORIGINAL_PRODUCT_DEV: 'original_product_development',
      NEW_PRODUCT_DEV: 'new_product_development',
      PURCHASE_TASK: 'purchase_task',
      RETOUCH_TASK: 'retouch_task',
      CUSTOMER_CUSTOMIZATION: 'customer_customization',
      REGULAR_CUSTOMIZATION: 'regular_customization',
    }
    params.task_type = map[f.taskType] ?? f.taskType
  }
  if (f.priority) params.priority = f.priority
  if (f.dateFrom) params.date_from = f.dateFrom
  if (f.dateTo) params.date_to = f.dateTo
  if (f.taskCategory === 'normal' || f.taskCategory === 'customization') {
    params.workflow_lane = f.taskCategory
  }
  const sortMap: Record<typeof sortKey.value, string> = {
    taskNo: 'task_no',
    updatedAt: 'updated_at',
    dueAt: 'due_at',
  }
  params.sort = `${sortOrder.value === 'desc' ? '-' : ''}${sortMap[sortKey.value]}`
  if (f.creatorId) params.creator_id = f.creatorId
  if (f.assigneeId) params.designer_id = f.assigneeId
  if (f.ownerDepartment) params.owner_department = f.ownerDepartment
  if (f.ownerOrgTeam) params.owner_org_team = f.ownerOrgTeam
  if (f.overdueOnly) params.overdue = true
  return params
}

function ownershipPrimary(task: Task): string {
  return getTaskOwnershipDisplay(task).primary
}

/** 方案 B：服务端分页 + 搜索，列表由后端过滤；仅对 warehouseStatus 做前端兜底（后端未统一支持） */
const filteredList = computed(() => {
  let list = tasksStore.list
  const f = filters.value
  if (f.creatorId) {
    const creatorId = String(f.creatorId).trim()
    list = list.filter((t) => String(t.creatorId ?? '').trim() === creatorId)
  }
  if (f.warehouseStatus) {
    list = list.filter(
      (t) => t.warehouseSubStatus === f.warehouseStatus || t.warehouseReceiveStatus === f.warehouseStatus,
    )
  }
  return list
})

/** 翻页模式：每页固定展示，不追加；total 来自 listTotal */
const totalPages = computed(() => Math.max(1, Math.ceil(tasksStore.listTotal / pageSize.value)))
const visibleCount = computed(() => {
  const loadedThroughCurrentPage = (page.value - 1) * pageSize.value + filteredList.value.length
  return Math.min(tasksStore.listTotal, loadedThroughCurrentPage)
})

const pagedList = computed(() => filteredList.value)

// ── 批量选择 ────────────────────────────────────────────────────────────────
function toggleSelect(id: string) {
  if (selectedIds.has(id)) selectedIds.delete(id)
  else selectedIds.add(id)
}

async function batchRemind() {
  if (selectedIds.size === 0) return
  const taskIds = selectedIdsAsNumericOrError()
  if (!taskIds) return
  batchReminding.value = true
  listActionError.value = ''
  try {
    await tasksApi.batchRemind({ task_ids: taskIds })
    selectedIds.clear()
    await refreshList(true)
  } catch (error) {
    listActionError.value = error instanceof Error ? error.message : '批量提醒失败，请稍后重试'
  } finally {
    batchReminding.value = false
  }
}

async function onBatchAssignConfirm(payload: { assigneeId: string; assigneeName: string }) {
  if (selectedIds.size === 0) return
  const taskIds = selectedIdsAsNumericOrError()
  if (!taskIds) return
  listActionError.value = ''
  try {
    await tasksApi.batchAssign({
      task_ids: taskIds,
      designer_id: parseInt(payload.assigneeId, 10),
      designer_name: payload.assigneeName,
    })
    showBatchAssign.value = false
    selectedIds.clear()
    await refreshList(true)
  } catch (error) {
    listActionError.value = formatTaskActionDenyMessage(error, '批量指派失败，请稍后重试')
  }
}

async function batchReceive() {
  const ids = Array.from(selectedIds)
  if (ids.length === 0 || batchReceiving.value) return
  batchReceiving.value = true
  listActionError.value = ''
  // v4.2 修复：老板要求 + 批量接收此前是清空选中无任何后端动作，改为逐条调用真实仓库接收接口
  const results = await Promise.allSettled(ids.map((id) => tasksStore.receiveInWarehouse(id)))
  const failures = results.filter((result): result is PromiseRejectedResult => result.status === 'rejected')
  const successCount = results.length - failures.length
  if (successCount > 0) {
    selectedIds.clear()
    await refreshList(true)
  }
  if (failures.length > 0) {
    const firstMessage = failures[0]?.reason instanceof Error
      ? failures[0].reason.message
      : '部分任务接收失败，请检查任务状态与 SKU'
    listActionError.value = successCount > 0
      ? `已接收 ${successCount} 条，另有 ${failures.length} 条失败：${firstMessage}`
      : firstMessage
  }
  batchReceiving.value = false
}

// ── 工具函数 ────────────────────────────────────────────────────────────────
function isOverdue(task: Task): boolean {
  return checkOverdue(task.dueAt, isDoneStatus(task))
}

function taskCategoryLabel(task: Task): string {
  return task.customizationRequired === true || task.workflowLane === 'customization'
    ? '定制任务'
    : '常规任务'
}

/**
 * 分类胶囊已展示「常规任务 / 定制任务」时，不再叠加同语义的 WorkflowLaneTag（常规 / 定制），避免顶部拥挤。
 * 若业务标记为定制但 lane 仍为 normal 等不一致情形，仍保留短标签以免丢失信息。
 */
function shouldShowWorkflowLaneTagOnCard(task: Task): boolean {
  const lane = String(task.workflowLane ?? '').trim().toLowerCase()
  if (lane !== 'normal' && lane !== 'customization') return false

  const categoryIsCustomization =
    task.customizationRequired === true || task.workflowLane === 'customization'

  if (categoryIsCustomization && lane === 'customization') return false
  if (!categoryIsCustomization && lane === 'normal') return false
  return true
}

function taskCategoryClass(task: Task): string {
  return task.customizationRequired === true || task.workflowLane === 'customization'
    ? 'task-category-pill-custom'
    : 'task-category-pill-normal'
}

function isRetouchTask(task: Task): boolean {
  const type = task.businessType ?? task.taskType
  return type === 'RETOUCH_TASK'
}

function displaySku(task: Task): string {
  return task.primarySkuCode ?? task.sku ?? '-'
}

/** 池中「接单」仅面向设计侧；单靠 design:work 会漏掉只下发 task.asset_upload / task.design_submit 的普通设计师 */
function userCanClaimFromDesignerPool(): boolean {
  if (can(PermissionEnum.DESIGN_WORK)) return true
  if (can(PermissionEnum.DESIGN_UPLOAD)) return true
  if (can(PermissionEnum.DESIGN_SUBMIT)) return true
  if (canAccessAction('task.asset_upload') || canAccessAction('task.design_submit')) return true
  return permissionsStore.hasAnyRole(['Designer', 'CustomizationOperator'])
}

function userCanClaimCustomizationFromPool(): boolean {
  const hasRole = (roles: readonly string[]) => permissionsStore.hasAnyRole(roles)
  if (userIsPureDesignerForCustomizationClaim(hasRole)) return false
  return userCanActAsCustomizationClaimActor(hasRole, permissionsStore.isCustomizationOperator)
}

function taskCenterClaimGate(): {
  canActAsCustomizationClaimActor: boolean
  canClaimFromDesignerPool: boolean
  activeTabIsPool: boolean
} {
  return {
    canActAsCustomizationClaimActor: userCanClaimCustomizationFromPool(),
    canClaimFromDesignerPool: userCanClaimFromDesignerPool(),
    activeTabIsPool: activeTab.value === 'pool',
  }
}

function canClaimTask(task: Task): boolean {
  return canClaimTaskFromCenter(task, taskCenterClaimGate())
}

async function claimTask(task: Task) {
  if (!canClaimTask(task) || claimingTaskId.value) return
  claimingTaskId.value = task.id
  listActionError.value = ''
  try {
    if (isCustomizationModuleClaimTask(task)) {
      await tasksStore.claimCustomizationModule(task.id)
      await refreshList(true)
      return
    }
    const me = permissionsStore.currentUser
    if (!me) return
    const currentUserId = Number.parseInt(String(me.id ?? ''), 10)
    if (Number.isNaN(currentUserId)) {
      listActionError.value = '当前账号信息异常，无法接单，请重新登录后重试'
      return
    }
    await tasksApi.assign(task.id, {
      designer_id: currentUserId,
      designer_name: me.name,
    })
    await router.push(`/tasks/${task.id}`)
  } catch (error) {
    listActionError.value = formatTaskActionDenyMessage(
      error,
      '任务已被他人接单，请刷新列表后重试',
    )
    await refreshList(true)
  } finally {
    claimingTaskId.value = null
  }
}

function formatDate(iso: string): string {
  return formatTaskDueAtDisplay(iso)
}

function goCreate() {
  if (!can('task.create')) return
  saveState()
  if (route.name !== 'TaskCreate') {
    void router.push({ name: 'TaskCreate' })
  } else {
    showCreateModal.value = true
  }
}

function goExcelAssistCreate() {
  if (!can('task.create')) return
  saveState()
  void router.push({ name: 'TaskExcelAssistCreate' })
}

function goDetail(task: Task) {
  saveState()
  void router.push(`/tasks/${task.id}`)
}

/** 翻页：请求指定页并替换列表（不追加） */
async function goToPage(p: number) {
  const maxP = Math.max(1, totalPages.value)
  const targetPage = Math.min(maxP, Math.max(1, p))
  if (targetPage === page.value && tasksStore.list.length > 0) return
  page.value = targetPage
  saveState()
  if (refreshingList.value) return
  refreshingList.value = true
  listActionError.value = ''
  try {
    await tasksStore.loadTaskListForView(buildListParams({ page: targetPage }))
  } catch (error) {
    listActionError.value = error instanceof Error ? error.message : '加载失败'
  } finally {
    refreshingList.value = false
  }
}

function jumpToPage() {
  const targetPage = Number(jumpPage.value)
  if (!Number.isFinite(targetPage)) {
    jumpPage.value = page.value
    return
  }
  void goToPage(Math.trunc(targetPage))
}

/** 方案 B：服务端分页 + 搜索，统一走 loadTaskListForView */
async function refreshList(_force?: boolean) {
  if (refreshingList.value) return
  refreshingList.value = true
  listActionError.value = ''
  try {
    page.value = 1
    saveState()
    await tasksStore.loadTaskListForView(buildListParams({ page: 1 }))
  } catch (error) {
    listActionError.value = error instanceof Error ? error.message : '刷新任务列表失败'
  } finally {
    refreshingList.value = false
  }
}

async function handleTaskCreated(taskId: string) {
  // v4.2 修复：老板要求 + 创建成功后强刷全局任务列表，避免排序、统计和徽标状态滞后
  await tasksStore.loadTaskById(taskId)
  await refreshList(true)
}

onMounted(async () => {
  await refreshList(true)
})

onBeforeUnmount(() => {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
})

// 根据路由控制创建任务弹窗（支持侧边栏 /tasks/create 与 ?create=1 深链接）
watch(
  () => [route.name, route.query.create, route.meta.openCreateModal],
  () => {
    const shouldOpen =
      (route.meta.openCreateModal === true || route.query.create === '1') && can('task.create')
    showCreateModal.value = shouldOpen
    if (shouldOpen) {
      page.value = 1
    }
  },
  { immediate: true },
)

/** 关闭弹窗时同步路由，否则仍停留在 /tasks/create 或 ?create=1，与“已关闭”状态不一致；浏览器返回能走是因为路由变了。 */
watch(showCreateModal, (open) => {
  if (open) return
  if (route.name === 'TaskCreate') {
    const q = { ...route.query } as Record<string, string | string[] | undefined>
    delete q.create
    delete q.draft_id
    void router.replace({ name: 'TaskList', query: q })
  } else if (route.name === 'TaskList' && queryString(route.query.create) === '1') {
    const q = { ...route.query } as Record<string, string | string[] | undefined>
    delete q.create
    delete q.draft_id
    void router.replace({ path: route.path, query: q })
  }
})

watch(showBatchAssign, (open) => {
  if (open && batchDesignerOptions.value.length === 0) loadBatchDesigners()
})

watch(
  pageSize,
  () => {
    page.value = 1
    saveState()
    refreshList(true)
  },
)

watch([searchKeyword, activeTab, sortKey, sortOrder, page], saveState)
watch(page, (value) => {
  jumpPage.value = value
})
watch([sortKey, sortOrder], () => {
  refreshList(true)
})

watch(activeTab, () => {
  page.value = 1
  saveState()
  refreshList(true)
})

watch(
  filters,
  () => {
    page.value = 1
    saveState()
    refreshList(true)
  },
  { deep: true },
)

watch(
  () => [filters.value, searchKeyword.value, activeTab.value, sortKey.value, sortOrder.value] as const,
  () => {
    const q = { ...route.query } as Record<string, string | string[] | undefined>
    const kw = searchKeyword.value.trim()
    if (kw) q.q = kw
    else delete q.q
    if (activeTab.value !== 'all') q.tab = activeTab.value
    else delete q.tab
    if (filters.value.status.length) q.status = filters.value.status.join(',')
    else delete q.status
    if (filters.value.ownerDepartment) q.owner_department = filters.value.ownerDepartment
    else delete q.owner_department
    if (filters.value.ownerOrgTeam) q.owner_org_team = filters.value.ownerOrgTeam
    else delete q.owner_org_team
    if (filters.value.taskCategory) q.task_category = filters.value.taskCategory
    else delete q.task_category
    if (filters.value.warehouseStatus) q.warehouse_status = filters.value.warehouseStatus
    else delete q.warehouse_status
    if (filters.value.taskType) q.task_type = filters.value.taskType
    else delete q.task_type
    if (filters.value.creatorId) q.creator_id = filters.value.creatorId
    else delete q.creator_id
    if (filters.value.priority) q.priority = filters.value.priority
    else delete q.priority
    if (filters.value.dateFrom) q.date_from = filters.value.dateFrom
    else delete q.date_from
    if (filters.value.dateTo) q.date_to = filters.value.dateTo
    else delete q.date_to
    const sortMap: Record<typeof sortKey.value, string> = {
      taskNo: 'task_no',
      updatedAt: 'updated_at',
      dueAt: 'due_at',
    }
    q.sort = `${sortOrder.value === 'desc' ? '-' : ''}${sortMap[sortKey.value]}`
    router.replace({ path: route.path, query: q })
  },
  { deep: true },
)

watch(
  () => route.query,
  (query) => {
    const nextTab = parseTaskTab(query.tab)
    const nextFilters = {
      ...filters.value,
      status: parseStatusQuery(query.status),
      ownerDepartment: queryString(query.owner_department),
      ownerOrgTeam: queryString(query.owner_org_team),
      taskCategory: queryString(query.task_category),
      warehouseStatus: queryString(query.warehouse_status),
      taskType: queryString(query.task_type),
      creatorId: queryString(query.creator_id),
      assigneeId: queryHasNonEmptyParam(query, 'designer_id')
        ? queryString(query.designer_id)
        : '',
      priority: queryString(query.priority),
      dateFrom: queryString(query.date_from),
      dateTo: queryString(query.date_to),
      overdueOnly: parseOverdueQuery(query),
    }
    const nextKeyword = queryString(query.q)
    let changed = false
    if (activeTab.value !== nextTab) {
      activeTab.value = nextTab
      changed = true
    }
    if (searchKeyword.value !== nextKeyword) {
      searchKeyword.value = nextKeyword
      changed = true
    }
    if (typeof query.sort === 'string') {
      const direction = query.sort.startsWith('-') ? 'desc' : 'asc'
      const field = query.sort.startsWith('-') ? query.sort.slice(1) : query.sort
      const sortMap: Record<string, typeof sortKey.value> = {
        task_no: 'taskNo',
        updated_at: 'updatedAt',
        due_at: 'dueAt',
      }
      if (sortMap[field] && (sortKey.value !== sortMap[field] || sortOrder.value !== direction)) {
        sortKey.value = sortMap[field]
        sortOrder.value = direction
        changed = true
      }
    }
    if (JSON.stringify(filters.value) !== JSON.stringify(nextFilters)) {
      filters.value = nextFilters
      changed = true
    }
    if (changed) page.value = 1
  },
)

watch(totalPages, (value) => {
  if (page.value > value) page.value = value
})
</script>

<style scoped>
.task-list-view {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-height: 100dvh;
  padding-bottom: 2rem;
}
.header-card {
  background: rgba(255, 255, 255, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.6);
  border-radius: 1rem;
  box-shadow: 0 4px 24px -4px rgba(28, 25, 23, 0.06);
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.filter-bar-wrap {
  border-top: 1px solid rgb(231 229 228);
  padding-top: 0.5rem;
  margin-top: 0.25rem;
}
.tab-status-scope-hint {
  margin: 0;
  font-size: 0.6875rem;
  line-height: 1.4;
  color: rgb(100 116 139);
}
.task-tabs,
.task-category-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-wrap: wrap;
}
.task-category-switch {
  border-top: 1px solid rgb(231 229 228);
  padding-top: 0.75rem;
}
.task-tab-active,
.task-category-active {
  font-weight: 700;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.page-header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.page-title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 800;
  font-family: Manrope, sans-serif;
  color: rgb(28 25 23);
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 1rem;
}
.search-input {
  flex-shrink: 0;
}
/* 批量操作条 */
.batch-action-bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.75rem;
  background: rgba(255, 255, 255, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.6);
  border-radius: 1rem;
  font-size: 0.8125rem;
  box-shadow: 0 4px 24px -4px rgba(28, 25, 23, 0.06);
}
.batch-count {
  font-weight: 500;
  color: rgb(51 65 85);
  flex: 1;
}
.list-action-error {
  margin: 0;
  padding: 0.625rem 0.875rem;
  border-radius: 0.75rem;
  background: rgb(254 242 242);
  border: 1px solid rgb(254 202 202);
  color: rgb(185 28 28);
  font-size: 0.8125rem;
}
.batch-bar-slide-enter-active,
.batch-bar-slide-leave-active {
  transition: opacity 0.2s, transform 0.2s;
}
.batch-bar-slide-enter-from,
.batch-bar-slide-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.checkbox {
  width: 0.875rem;
  height: 0.875rem;
  border-radius: 0.25rem;
  cursor: pointer;
  accent-color: rgb(15 23 42);
}

/* 统一卡片视图：列宽约 300px 起，大屏约 320px，同屏约 4～5 列、卡片更宽便于阅读 */
.task-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
}
@media (min-width: 1600px) {
  .task-cards {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}
.task-card {
  padding: 0.875rem;
  background: #fff;
  border: 1px solid rgba(255, 255, 255, 0.6);
  border-radius: 0.875rem;
  box-shadow: 0 4px 24px -4px rgba(28, 25, 23, 0.06);
  cursor: pointer;
  transition: all 0.2s;
}
.task-card:hover {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(255, 255, 255, 0.8);
  transform: translateY(-2px);
  box-shadow: 0 12px 48px -12px rgba(28, 25, 23, 0.08);
}
.card-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}
.card-row-top {
  align-items: center;
  flex-wrap: nowrap;
  margin-bottom: 0.35rem;
}
.card-tags {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.25rem;
}
.card-row-bottom {
  margin-top: 0.5rem;
  margin-bottom: 0;
  justify-content: flex-start;
  flex-wrap: wrap;
}
.card-check {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  padding-top: 0.05rem;
}
.card-no-row {
  margin-bottom: 0.4rem;
  padding-bottom: 0.4rem;
  border-bottom: 1px solid rgb(241 245 249);
}
.card-no {
  display: block;
  font-weight: 600;
  font-size: 0.75rem;
  color: rgb(51 65 85);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  letter-spacing: 0.01em;
}
.card-product {
  min-width: 0;
  font-size: 0.875rem;
  color: rgb(15 23 42);
  font-weight: 500;
  line-height: 1.35;
  /* 固定两行高度；超出两行省略，悬停 title 见全文 */
  min-height: calc(1.35em * 2);
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.card-meta-block {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  margin-top: 0.5rem;
}
.card-meta-line {
  display: grid;
  grid-template-columns: 2.25rem minmax(0, 1fr);
  column-gap: 0.5rem;
  align-items: baseline;
  font-size: 0.75rem;
  line-height: 1.4;
}
.card-meta-key {
  text-align: right;
  flex-shrink: 0;
  font-weight: 600;
}
/* 仅字色区分字段，无底色；标签略饱和、正文统一为 slate 保证可读 */
.card-meta-line--ownership .card-meta-key {
  color: rgb(67 56 202);
}
.card-meta-line--sku .card-meta-key {
  color: rgb(180 83 9);
}
.card-meta-line--creator .card-meta-key {
  color: rgb(4 120 87);
}
.card-meta-line--design .card-meta-key {
  color: rgb(91 33 182);
}
.card-meta-value {
  min-width: 0;
  color: rgb(71 85 105);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.card-updated {
  font-size: 0.6875rem;
  color: rgb(148 163 184);
}
.card-due {
  font-size: 0.6875rem;
  color: rgb(100 116 139);
}
.card-due-overdue {
  color: rgb(220 38 38);
  font-weight: 600;
}
.task-category-pill {
  border-radius: 9999px;
  padding: 0.1rem 0.4rem;
  font-size: 0.625rem;
  font-weight: 600;
  white-space: nowrap;
}
.task-category-pill-normal {
  color: rgb(30 64 175);
  background: rgb(239 246 255);
  border: 1px solid rgb(191 219 254);
}
.task-category-pill-custom {
  color: rgb(109 40 217);
  background: rgb(245 243 255);
  border: 1px solid rgb(221 214 254);
}

/* 分页 */
.footer-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background: #fff;
  border: 1px solid rgba(255, 255, 255, 0.6);
  border-radius: 1rem;
  box-shadow: 0 4px 24px -4px rgba(28, 25, 23, 0.06);
}
.pagination {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.infinite-sentinel {
  width: 100%;
  height: 1px;
}
.pager-btn {
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  border-radius: 9999px;
  border: 1px solid rgb(226 232 240);
  background: white;
  color: rgb(51 65 85);
  cursor: pointer;
}
.pager-btn:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}
.pager-info {
  font-size: 0.75rem;
  color: rgb(100 116 139);
}
.task-page-size-select {
  min-width: 4.75rem;
}
.page-jump {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
.page-jump-input {
  width: 5rem;
}

/* Light admin task list skin. Style-only. */
.task-list-view {
  color: #374151;
  background: transparent;
}

.header-card,
.batch-action-bar,
.task-card,
.footer-card {
  border: 1px solid #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06) !important;
}

.header-card {
  position: relative;
  overflow: hidden;
}

.header-card::before {
  display: none;
}

.page-title {
  position: relative;
  color: #111827 !important;
  font-size: clamp(1.8rem, 2.6vw, 3rem);
  font-weight: 900;
}

.filter-bar-wrap,
.task-category-switch,
.card-no-row {
  border-color: #e5e7eb !important;
}

.task-tabs,
.task-category-switch,
.toolbar,
.filter-bar-wrap,
.page-header {
  position: relative;
}

.task-tab-active,
.task-category-active {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #ffffff !important;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2);
}

.task-card {
  overflow: hidden;
}

.task-card:hover {
  border-color: #d1d5db !important;
  background: #ffffff !important;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08) !important;
}

.card-no,
.card-product,
.batch-count {
  color: #111827 !important;
}

.card-meta-value,
.card-updated,
.card-due,
.pager-info,
.footer-card,
.page-jump {
  color: #6b7280 !important;
}

.card-meta-key {
  color: #6b7280 !important;
}

.task-category-pill-normal,
.task-category-pill-custom {
  background: #f3f4f6 !important;
  border-color: #e5e7eb !important;
  color: #374151 !important;
}

.checkbox {
  accent-color: #2563eb;
}

.list-action-error {
  background: #fef2f2;
  border-color: #fecaca;
  color: #b91c1c;
}

/* Task center tokens: light admin system. */
.task-list-view {
  --tc-page: #f5f6f8;
  --tc-panel: #ffffff;
  --tc-panel-strong: #ffffff;
  --tc-card: #ffffff;
  --tc-card-soft: #f9fafb;
  --tc-border: #e5e7eb;
  --tc-border-strong: #d1d5db;
  --tc-text: #111827;
  --tc-muted: #6b7280;
  --tc-faint: #9ca3af;
  --tc-cyan: #2563eb;
  --tc-blue: #2563eb;
  --tc-green: #15803d;
  --tc-amber: #b45309;
  --tc-pink: #db2777;
  background: #f5f6f8;
}

.header-card,
.batch-action-bar,
.footer-card {
  border-color: var(--tc-border) !important;
  background: #ffffff !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06) !important;
}

.header-card {
  border-radius: 1.1rem;
  padding: 1.1rem;
}

.page-title {
  font-size: clamp(1.75rem, 2.4vw, 2.85rem);
  line-height: 1.05;
}

.toolbar {
  align-items: stretch;
}

.search-input {
  width: min(32rem, 100%) !important;
}

.task-category-switch,
.task-tabs {
  gap: 0.4rem;
}

.task-category-switch :deep(button),
.task-tabs :deep(button),
.toolbar :deep(button),
.batch-action-bar :deep(button),
.footer-card :deep(button) {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
}

.task-tab-active,
.task-category-active {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #ffffff !important;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2) !important;
}

.filter-bar-wrap {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
}

.filter-bar-wrap :deep(.filter-bar) {
  gap: 0.65rem;
}

.filter-bar-wrap :deep(.field-label) {
  color: var(--tc-muted) !important;
  font-weight: 750;
}

.task-cards {
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 0.9rem;
}

@media (min-width: 1600px) {
  .task-cards {
    grid-template-columns: repeat(auto-fill, minmax(390px, 1fr));
  }
}

.task-card {
  position: relative;
  display: flex;
  min-height: 16.2rem;
  flex-direction: column;
  gap: 0.48rem;
  overflow: hidden;
  border-color: var(--tc-border) !important;
  border-radius: 0.95rem;
  padding: 0.95rem;
  background: #ffffff !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06) !important;
}

.task-card::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: #2563eb;
  opacity: 1;
}

.task-card:hover {
  border-color: #d1d5db !important;
  background: #ffffff !important;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08) !important;
}

.task-card--selected {
  border-color: #93c5fd !important;
  background: #eff6ff !important;
  box-shadow: 0 0 0 1px #bfdbfe, 0 4px 12px rgba(37, 99, 235, 0.12) !important;
}

.task-card--selected::before {
  width: 4px;
  background: #2563eb;
  opacity: 1;
}

.task-card--overdue {
  border-color: #fcd34d !important;
}

.task-card--overdue::before {
  background: #f59e0b;
  opacity: 1;
}

.card-row-top {
  position: relative;
  z-index: 1;
  min-height: 1.7rem;
  margin-bottom: 0.2rem;
}

.card-check {
  border-radius: 0.42rem;
}

.checkbox {
  width: 0.9rem;
  height: 0.9rem;
  border-radius: 0.28rem;
  accent-color: var(--tc-cyan);
}

.card-tags {
  gap: 0.35rem;
}

.card-tags :deep(*) {
  max-width: 100%;
}

.task-category-pill,
.card-tags :deep(.task-type-badge),
.card-tags :deep(.workflow-lane-tag) {
  border: 1px solid #e5e7eb !important;
  background: #f3f4f6 !important;
  color: #374151 !important;
  font-weight: 850 !important;
}

.task-category-pill-normal {
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.task-category-pill-custom {
  background: #fffbeb !important;
  color: #b45309 !important;
}

.card-no-row {
  position: relative;
  z-index: 1;
  margin-bottom: 0.2rem;
  border-color: rgba(210, 229, 255, 0.13) !important;
}

.card-no {
  color: #6b7280 !important;
  font-family: var(--yb-font-data);
  font-size: 0.78rem;
  font-weight: 850;
  letter-spacing: 0;
}

.card-product {
  position: relative;
  z-index: 1;
  min-height: calc(1.38em * 2);
  color: #111827 !important;
  font-size: 0.96rem;
  font-weight: 850;
  line-height: 1.38;
}

.card-meta-block {
  position: relative;
  z-index: 1;
  margin-top: 0.2rem;
  gap: 0.25rem;
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  padding: 0.55rem 0.62rem;
  background: #f9fafb;
}

.card-meta-line {
  grid-template-columns: 2.6rem minmax(0, 1fr);
  column-gap: 0.55rem;
  font-size: 0.77rem;
}

.card-meta-key {
  color: #6b7280 !important;
  font-weight: 850;
}

.card-meta-value {
  color: #374151 !important;
  font-weight: 650;
}

.card-row-bottom {
  position: relative;
  z-index: 1;
  margin-top: auto;
  gap: 0.55rem 0.75rem;
  border-top: 1px solid rgba(210, 229, 255, 0.1);
  padding-top: 0.55rem;
}

.card-updated,
.card-due {
  color: var(--tc-muted) !important;
  font-size: 0.72rem;
  font-weight: 650;
}

.card-due-overdue {
  color: #b45309 !important;
  font-weight: 850;
}

.card-row-bottom :deep(button) {
  min-height: 1.9rem;
  border-color: #bfdbfe !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.footer-card {
  border-radius: 1rem;
}

@media (max-width: 760px) {
  .task-cards {
    grid-template-columns: minmax(0, 1fr);
  }

  .toolbar,
  .page-header,
  .footer-card {
    align-items: stretch;
    flex-direction: column;
  }

  .search-input {
    width: 100% !important;
  }
}

/* Task center correction: readable density and stable wide-screen columns. */
.task-list-view {
  gap: 1rem;
  padding-inline: clamp(0.75rem, 1vw, 1.25rem);
}

.header-card {
  gap: 0.82rem !important;
  border-radius: 1rem !important;
  padding: 1rem !important;
}

.header-card::before {
  opacity: 0.48;
}

.page-header {
  min-height: 3.1rem;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 0.7rem;
}

.page-title {
  font-size: clamp(1.75rem, 1.8vw, 2.25rem) !important;
  line-height: 1.04 !important;
}

.page-header-actions :deep(button) {
  min-height: 2.15rem;
}

.task-category-switch {
  padding-top: 0.55rem !important;
}

.task-tabs {
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 0.55rem;
}

.task-category-switch :deep(button),
.task-tabs :deep(button) {
  min-height: 1.85rem;
  padding-inline: 0.72rem;
  font-size: 0.75rem;
}

.toolbar {
  gap: 0.7rem;
}

.search-input {
  width: min(27rem, 100%) !important;
}

.filter-bar-wrap {
  margin-top: 0 !important;
  padding-top: 0.75rem !important;
}

.filter-bar-wrap :deep(.filter-bar) {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.25rem, 1fr));
  gap: 0.68rem;
  align-items: end;
}

.filter-bar-wrap :deep(.filter-field) {
  min-width: 0;
  width: auto;
}

.filter-bar-wrap :deep(.filter-field-wide) {
  min-width: 0;
}

.filter-bar-wrap :deep(.filter-bar-trailing) {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.55rem;
}

.task-cards {
  grid-template-columns: minmax(0, 1fr) !important;
  gap: 1rem !important;
}

@media (min-width: 900px) {
  .task-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
  }
}

@media (min-width: 1280px) {
  .task-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr)) !important;
  }
}

@media (min-width: 1680px) {
  .task-cards {
    grid-template-columns: repeat(4, minmax(0, 1fr)) !important;
  }
}

.task-card {
  min-height: 14.8rem;
  gap: 0.52rem;
  border-radius: 1rem;
  padding: 1rem;
  background: #ffffff !important;
}

.task-card--claimable:not(.task-card--overdue)::before {
  background: #2563eb;
}

.card-row-top {
  min-height: 1.55rem;
  margin-bottom: 0.05rem;
}

.card-no-row {
  margin-bottom: 0.12rem;
  padding-bottom: 0.5rem;
}

.card-no {
  font-size: 0.76rem;
  color: #6b7280 !important;
}

.card-product {
  min-height: calc(1.42em * 2);
  font-size: 1rem;
  line-height: 1.42;
}

.card-meta-block {
  margin-top: 0.12rem;
  border-color: #e5e7eb;
  padding: 0.58rem 0.66rem;
  background: #f9fafb;
}

.card-meta-line {
  grid-template-columns: 2.45rem minmax(0, 1fr);
  font-size: 0.78rem;
}

.card-row-bottom {
  min-height: 2rem;
  padding-top: 0.58rem;
}

.card-row-bottom :deep(button) {
  min-height: 1.85rem;
  padding-inline: 0.72rem;
}

/* Task center defect pass: badges, filters, card scale, and icon/button language. */
.toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.advanced-filter-toggle {
  min-width: 6.25rem;
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
}

.advanced-filter-toggle--active {
  border-color: #bfdbfe !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.filter-bar-wrap {
  border-top: 1px solid #e5e7eb !important;
  padding-top: 0.72rem !important;
  background: #ffffff !important;
}

.filter-bar-wrap :deep(.filter-bar) {
  grid-template-columns: repeat(auto-fit, minmax(8.75rem, 1fr));
  gap: 0.58rem;
}

.filter-bar-wrap :deep(.field-label) {
  font-size: 0.68rem;
  letter-spacing: 0.02em;
}

.filter-bar-wrap :deep(.filter-overdue) {
  color: #cbd5e1;
}

.filter-bar-wrap :deep(.filter-overdue-input) {
  accent-color: var(--tc-cyan);
}

.task-cards {
  gap: 0.78rem !important;
}

.task-card {
  min-height: 12.9rem;
  gap: 0.4rem;
  border-color: #e5e7eb !important;
  border-radius: 0.86rem;
  padding: 0.82rem;
  background: #ffffff !important;
}

.task-card:hover {
  border-color: #d1d5db !important;
}

.task-card--selected {
  border-color: #93c5fd !important;
  background: #eff6ff !important;
}

.card-tags {
  align-items: center;
  gap: 0.28rem;
}

.task-category-pill,
.card-tags :deep(.task-type-badge),
.card-tags :deep(.lane-tag) {
  display: inline-flex !important;
  height: 1.25rem;
  align-items: center;
  border-color: #e5e7eb !important;
  border-radius: 999px;
  padding: 0 0.45rem !important;
  background: #f3f4f6 !important;
  color: #374151 !important;
  font-size: 0.66rem !important;
  font-weight: 750 !important;
  line-height: 1 !important;
}

.task-category-pill-normal,
.task-category-pill-custom,
.card-tags :deep(.badge-new),
.card-tags :deep(.badge-original),
.card-tags :deep(.badge-purchase),
.card-tags :deep(.badge-retouch),
.card-tags :deep(.is-normal),
.card-tags :deep(.is-customization) {
  background: #f3f4f6 !important;
  color: #374151 !important;
}

.task-category-pill-custom,
.card-tags :deep(.badge-new),
.card-tags :deep(.is-customization) {
  border-color: #bbf7d0 !important;
  color: #15803d !important;
}

.card-status-row {
  gap: 0.32rem !important;
  margin-top: 0 !important;
}

.card-status-row :deep(.inline-flex),
.card-status-row :deep(.filing-status-badge) {
  min-height: 1.28rem;
  border-color: #e5e7eb !important;
  border-radius: 999px;
  padding: 0.1rem 0.48rem !important;
  background: #f3f4f6 !important;
  color: #374151 !important;
  font-size: 0.68rem !important;
  font-weight: 780 !important;
  line-height: 1 !important;
}

.card-status-row :deep(.bg-emerald-100),
.card-status-row :deep(.text-emerald-800),
.card-status-row :deep(.border-emerald-200) {
  border-color: #bbf7d0 !important;
  background: #ecfdf5 !important;
  color: #15803d !important;
}

.card-status-row :deep(.bg-blue-100),
.card-status-row :deep(.text-blue-800),
.card-status-row :deep(.border-blue-200) {
  border-color: #bfdbfe !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.card-status-row :deep(.bg-amber-100),
.card-status-row :deep(.text-amber-800),
.card-status-row :deep(.border-amber-200) {
  border-color: #fde68a !important;
  background: #fffbeb !important;
  color: #b45309 !important;
}

.card-status-row :deep(.bg-red-100),
.card-status-row :deep(.text-red-800),
.card-status-row :deep(.border-red-200) {
  border-color: #fecaca !important;
  background: #fef2f2 !important;
  color: #b91c1c !important;
}

.card-no-row {
  padding-bottom: 0.4rem;
}

.card-no {
  font-size: 0.72rem;
}

.card-product {
  min-height: calc(1.36em * 2);
  font-size: 0.93rem;
  line-height: 1.36;
}

.card-meta-block {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.28rem 0.58rem;
  border-color: #e5e7eb;
  border-radius: 0.66rem;
  padding: 0.48rem 0.56rem;
  background: #f9fafb;
}

.card-meta-line {
  min-width: 0;
  grid-template-columns: 2.15rem minmax(0, 1fr);
  column-gap: 0.38rem;
  font-size: 0.7rem;
}

.card-row-bottom {
  min-height: 1.55rem;
  gap: 0.35rem 0.55rem;
  padding-top: 0.42rem;
}

.card-updated,
.card-due {
  font-size: 0.68rem;
}

.page-header-actions :deep(button),
.toolbar :deep(button),
.card-row-bottom :deep(button),
.footer-card :deep(button) {
  border-radius: 0.62rem !important;
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: none !important;
}

.page-header-actions :deep(button:hover),
.toolbar :deep(button:hover),
.card-row-bottom :deep(button:hover),
.footer-card :deep(button:hover) {
  border-color: #d1d5db !important;
  background: #f3f4f6 !important;
  color: #111827 !important;
}

.page-header-actions :deep(button svg),
.toolbar :deep(button svg),
.footer-card :deep(button svg) {
  width: 1rem;
  height: 1rem;
  color: #6b7280;
  stroke-width: 2;
}

/* Narrow viewport repair: prevent the left rail from eating content. */
.task-list-view,
.header-card,
.task-card,
.card-product,
.card-no-row,
.card-meta-block,
.card-row-bottom {
  min-width: 0;
}

.task-list-view {
  overflow-x: hidden;
}

.card-no,
.card-meta-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-product {
  overflow-wrap: anywhere;
  word-break: break-word;
}

@media (max-width: 720px) {
  .task-list-view {
    gap: 0.75rem;
    padding-inline: 0.35rem;
  }

  .header-card {
    border-radius: 0.82rem !important;
    padding: 0.72rem !important;
  }

  .page-header {
    gap: 0.65rem;
    align-items: flex-start;
  }

  .page-title {
    font-size: clamp(1.75rem, 10vw, 2.35rem) !important;
  }

  .page-header-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .task-category-switch,
  .task-tabs {
    display: flex;
    width: 100%;
    overflow-x: auto;
    flex-wrap: nowrap;
    scrollbar-width: none;
  }

  .task-category-switch::-webkit-scrollbar,
  .task-tabs::-webkit-scrollbar {
    display: none;
  }

  .toolbar {
    align-items: stretch;
  }

  .search-input {
    width: 100% !important;
    flex: 1 1 100%;
  }

  .advanced-filter-toggle {
    width: 100%;
  }

  .task-cards {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  .task-card {
    min-height: auto;
    padding: 0.72rem;
  }

  .card-row-top {
    align-items: flex-start;
    gap: 0.45rem;
  }

  .card-tags {
    min-width: 0;
    flex: 1;
    flex-wrap: wrap;
  }

  .card-product {
    display: -webkit-box;
    min-height: auto;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .card-meta-block {
    grid-template-columns: minmax(0, 1fr);
  }

  .card-meta-line {
    grid-template-columns: 2.25rem minmax(0, 1fr);
  }

  .card-row-bottom {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
  }
}

/* Edge and radius repair: keep a consistent gutter so rounded glass corners render cleanly. */
.task-list-view {
  padding-inline: clamp(0.45rem, 0.7vw, 0.75rem) !important;
}

.header-card,
.task-card,
.batch-action-bar,
.footer-card {
  isolation: isolate;
  background-clip: padding-box !important;
  contain: paint;
  transform: translateZ(0);
}

.header-card::before,
.task-card::before {
  border-radius: inherit;
}

@media (max-width: 720px) {
  .task-list-view {
    padding-inline: 0.35rem !important;
  }
}

/* Final task-center layout stabilization under the persistent sidebar. */
.task-list-view {
  padding-inline: 0 !important;
}

.task-cards {
  grid-template-columns: minmax(0, 1fr) !important;
}

@media (min-width: 1180px) {
  .task-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
  }
}

@media (min-width: 1520px) {
  .task-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr)) !important;
  }
}

@media (min-width: 1900px) {
  .task-cards {
    grid-template-columns: repeat(4, minmax(0, 1fr)) !important;
  }
}

.header-card,
.task-card,
.batch-action-bar,
.footer-card {
  overflow: hidden;
  clip-path: inset(0 round 1rem);
}

.task-card {
  clip-path: inset(0 round 0.86rem);
}

.batch-action-bar,
.footer-card {
  clip-path: inset(0 round 1rem);
}

/* Legacy dark-corner patches neutralized for light shell. */
.header-card,
.batch-action-bar,
.footer-card {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
}

.header-card::before {
  display: none !important;
}

.task-card {
  background: #ffffff !important;
}

.task-card:hover {
  background: #ffffff !important;
}

/* Phase 2: align task center with light shell — final overrides win over legacy dark patches. */
.task-list-view {
  margin: 0 !important;
  min-height: auto;
  padding: 0 !important;
  background: transparent !important;
}

.header-card,
.batch-action-bar,
.footer-card,
.task-card {
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

.header-card::before {
  display: none !important;
}

.page-header {
  border-bottom-color: #e5e7eb !important;
}

.filter-bar-wrap {
  background: #ffffff !important;
}

.advanced-filter-toggle {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
}

.advanced-filter-toggle--active {
  border-color: #bfdbfe !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.task-card,
.task-card:hover,
.task-card--selected {
  background: #ffffff !important;
}

.card-no {
  color: #6b7280 !important;
}

.card-product {
  color: #111827 !important;
}

.card-meta-block {
  background: #f9fafb !important;
  border-color: #e5e7eb !important;
}

.card-status-row :deep(.inline-flex),
.card-status-row :deep(.filing-status-badge),
.card-status-row :deep(.bg-emerald-100),
.card-status-row :deep(.bg-blue-100),
.card-status-row :deep(.bg-amber-100),
.card-status-row :deep(.bg-red-100) {
  border-color: #e5e7eb !important;
  background: #f3f4f6 !important;
  color: #374151 !important;
}

.card-status-row :deep(.text-emerald-800) {
  color: #15803d !important;
}

.card-status-row :deep(.text-blue-800) {
  color: #1d4ed8 !important;
}

.card-status-row :deep(.text-amber-800) {
  color: #b45309 !important;
}

.card-status-row :deep(.text-red-800) {
  color: #b91c1c !important;
}

.page-header-actions :deep(button),
.toolbar :deep(button),
.card-row-bottom :deep(button),
.footer-card :deep(button) {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: none !important;
}

.page-header-actions :deep(button:hover),
.toolbar :deep(button:hover),
.card-row-bottom :deep(button:hover),
.footer-card :deep(button:hover) {
  border-color: #d1d5db !important;
  background: #f3f4f6 !important;
  color: #111827 !important;
}

.page-header-actions :deep(button svg),
.toolbar :deep(button svg),
.footer-card :deep(button svg) {
  color: #6b7280;
}

/* Task center filter tabs: explicit default / hover / active (beats generic :deep(button) resets). */
.task-category-switch :deep(button:not(.task-category-active)),
.task-tabs :deep(button:not(.task-tab-active)) {
  border: 1px solid #dbe3ef !important;
  background: #ffffff !important;
  color: #334155 !important;
  font-weight: 500 !important;
  box-shadow: none !important;
}

.task-category-switch :deep(button:not(.task-category-active):hover),
.task-tabs :deep(button:not(.task-tab-active):hover) {
  border-color: #93c5fd !important;
  background: #f8fafc !important;
  color: #1e293b !important;
  box-shadow: none !important;
}

.task-category-switch :deep(button.task-category-active),
.task-tabs :deep(button.task-tab-active) {
  border: 1px solid #2563eb !important;
  background: #2563eb !important;
  color: #ffffff !important;
  font-weight: 700 !important;
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.16) !important;
}

.task-category-switch :deep(button.task-category-active:hover),
.task-tabs :deep(button.task-tab-active:hover) {
  border-color: #1d4ed8 !important;
  background: #1d4ed8 !important;
  color: #ffffff !important;
  box-shadow: 0 4px 12px rgba(29, 78, 216, 0.22) !important;
}
</style>
