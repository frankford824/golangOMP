<template>
  <div class="task-list-view">
    <!-- 顶栏 + 搜索/过滤：统一卡片容器 -->
    <div class="header-card">
      <div class="page-header">
        <h2 class="page-title task-list-page-title">任务中心</h2>
        <div class="page-header-actions">
          <BaseButton
            size="sm"
            variant="secondary"
            class="task-list-action-button"
            :loading="refreshingList"
            @click="refreshList(true)"
          >
            {{ refreshingList ? '刷新中...' : '刷新列表' }}
          </BaseButton>
          <BaseButton
            v-if="can('task.create')"
            variant="primary"
            class="task-list-action-button task-list-action-button--primary"
            @click="goCreate"
          >
            创建任务
          </BaseButton>
          <BaseButton
            v-if="can('task.create')"
            variant="secondary"
            class="task-list-action-button"
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
          placeholder="搜索任务号、SKU、任务名、子项名称或设计要求"
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
    <p v-if="listActionSuccess" class="list-action-success">{{ listActionSuccess }}</p>

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
        <TaskCard
          v-for="task in pagedList"
          :key="task.id"
          :task="task"
          :selected="selectedIds.has(task.id)"
          :overdue="isOverdue(task)"
          :claimable="canClaimTask(task)"
          :customization="isCustomizationTask(task)"
          :title="taskCardTitle(task)"
          :sku="displaySku(task)"
          :ownership="ownershipPrimary(task)"
          :creator="taskCreatorDisplayName(task)"
          :designer="taskDesignerDisplayName(task)"
          :show-designer="shouldShowDesignerMetaOnTaskCenterCard(task)"
          :category-label="taskCategoryLabel(task)"
          :category-kind="isCustomizationTask(task) ? 'custom' : 'normal'"
          :show-lane-tag="shouldShowWorkflowLaneTagOnCard(task)"
          :status-label-override="getTaskCenterCardStatusLabel(task) ?? undefined"
          :is-retouch="isRetouchTask(task)"
          :is-batch="isBatchCard(task)"
          :batch-count="batchItemCount(task)"
          :batch-preview="taskCardBatchPreview(task)"
          :has-more-batch="batchItemCount(task) > 2"
          :claiming="claimingTaskId === task.id"
          :claim-disabled="Boolean(claimingTaskId)"
          :claim-label="taskCenterClaimButtonLabel(task, claimingTaskId === task.id)"
          :can-copy-no="canCopyTaskField(task.taskNo)"
          :can-copy-title="canCopyTaskField(taskCardTitle(task))"
          :can-copy-sku="canCopyTaskField(displaySku(task))"
          :updated-text="formatDate(task.updatedAt)"
          :due-text="task.dueAt ? formatDate(task.dueAt) : '-'"
          @pointerdown="onTaskCardPointerDown"
          @pointermove="onTaskCardPointerMove"
          @click="onTaskCardClick($event, task)"
          @toggle-select="toggleSelect(task.id)"
          @copy="copyTaskCardText"
          @claim="claimTask(task)"
          @open-batch="openBatchItemsModal(task)"
        />
      </div>
    </AsyncStateWrapper>

    <!-- 分页页脚：翻页按钮，每页固定展示，不追加 -->
    <div v-if="filteredList.length > 0" class="footer-card">
      <span class="text-xs text-[rgb(var(--yb-text-muted))]">共 {{ tasksStore.listTotal }} 条</span>
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-1 text-xs text-[rgb(var(--yb-text-muted))]">
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
          <span class="pager-info text-xs text-[rgb(var(--yb-text-muted))]">
            第 {{ page }} / {{ totalPages }} 页，已显示 {{ visibleCount }} / {{ tasksStore.listTotal }}
          </span>
          <label class="page-jump text-xs text-[rgb(var(--yb-text-muted))]">
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

    <BaseModal
      v-model="batchItemsModalOpen"
      title="批量任务明细"
      :show-confirm="false"
      cancel-text="关闭"
      panel-class="max-w-[min(780px,94vw)]"
    >
      <div v-if="batchItemsModalTask" class="batch-items-modal">
        <div class="batch-items-modal-header">
          <p class="batch-items-modal-title">{{ taskCardTitle(batchItemsModalTask) }}</p>
          <span class="batch-count-pill">共 {{ batchItemCount(batchItemsModalTask) }} 项</span>
        </div>
        <div class="batch-items-modal-list">
          <div
            v-for="(item, index) in batchModalItems"
            :key="batchItemKey(item, index)"
            class="batch-items-modal-row"
          >
            <span class="batch-modal-index">{{ item.sequenceNo ?? index + 1 }}</span>
            <div class="batch-modal-main">
              <p class="batch-modal-name">{{ batchItemDisplayName(item) }}</p>
              <p v-if="batchItemSubText(item)" class="batch-modal-sub">{{ batchItemSubText(item) }}</p>
            </div>
            <span class="batch-modal-sku">{{ item.skuCode || '-' }}</span>
          </div>
        </div>
      </div>
    </BaseModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount, onMounted, reactive, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTasksStore } from '@/stores/tasks'
import { usePermissionsStore } from '@/stores/permissions'
import type { Task, TaskSkuItem, LegacyTaskStatus } from '@/domain/types/task'
import { isDoneStatus, shouldShowDesignerMetaOnTaskCenterCard } from '@/domain/task-actions'
import { usePermission } from '@/composables/usePermission'
import type { TaskListFilters } from '@/components/task/TaskFilterBar.vue'
import TaskFilterBar from '@/components/task/TaskFilterBar.vue'
import TaskCard from '@/components/task/TaskCard.vue'
import AsyncStateWrapper from '@/components/base/AsyncStateWrapper.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseModal from '@/components/base/BaseModal.vue'
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
import { expandTaskListStatusFilter } from '@/domain/task-list-status-filter'
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
const batchItemsModalTask = ref<Task | null>(null)
const batchItemsModalOpen = computed({
  get: () => batchItemsModalTask.value != null,
  set: (open: boolean) => {
    if (!open) batchItemsModalTask.value = null
  },
})
const batchModalItems = computed(() => batchSkuItems(batchItemsModalTask.value))
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
const listActionSuccess = ref('')
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
let listActionSuccessTimer: ReturnType<typeof setTimeout> | null = null
let listActionSeq = 0
const taskCardPointerState = {
  x: 0,
  y: 0,
  moved: false,
}

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
    listActionError.value = '选择的任务无效，请刷新后重试'
    return null
  }
  return nums
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
    const terminatedStatus = f.status.length ? expandTaskListStatusFilter(f.status).join(',') : 'Cancelled'
    params.status = terminatedStatus
  } else if (f.status.length) {
    params.status = expandTaskListStatusFilter(f.status).join(',')
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

function isCustomizationTask(task: Task): boolean {
  const lane = String(task.workflowLane ?? '').trim().toLowerCase()
  return task.customizationRequired === true || lane === 'customization'
}

function taskCategoryLabel(task: Task): string {
  return isCustomizationTask(task) ? '定制任务' : '常规任务'
}

/**
 * 分类胶囊已展示「常规任务 / 定制任务」时，不再叠加同语义的 WorkflowLaneTag（常规 / 定制），避免顶部拥挤。
 * 若业务标记为定制但 lane 仍为 normal 等不一致情形，仍保留短标签以免丢失信息。
 */
function shouldShowWorkflowLaneTagOnCard(task: Task): boolean {
  const lane = String(task.workflowLane ?? '').trim().toLowerCase()
  if (lane !== 'normal' && lane !== 'customization') return false

  const categoryIsCustomization = isCustomizationTask(task)

  if (categoryIsCustomization && lane === 'customization') return false
  if (!categoryIsCustomization && lane === 'normal') return false
  return true
}

function isRetouchTask(task: Task): boolean {
  const type = task.businessType ?? task.taskType
  return type === 'RETOUCH_TASK'
}

function normalizeCardText(value: unknown): string {
  return String(value ?? '').trim()
}

function batchSkuItems(task: Task | null | undefined): TaskSkuItem[] {
  return Array.isArray(task?.skuItems) ? task.skuItems.filter(Boolean) : []
}

function batchItemCount(task: Task): number {
  const apiCount =
    typeof task.batchItemCount === 'number' && Number.isFinite(task.batchItemCount)
      ? task.batchItemCount
      : 0
  return Math.max(apiCount, batchSkuItems(task).length, task.isBatchTask === true ? 1 : 0)
}

function isBatchCard(task: Task): boolean {
  return task.isBatchTask === true || batchItemCount(task) > 1
}

function batchItemDisplayName(item: TaskSkuItem | null | undefined): string {
  if (!item) return ''
  return (
    normalizeCardText(item.productNameSnapshot) ||
    normalizeCardText(item.productShortName) ||
    normalizeCardText(item.designRequirement) ||
    normalizeCardText(item.skuCode)
  )
}

function batchItemSubText(item: TaskSkuItem | null | undefined): string {
  if (!item) return ''
  const design = normalizeCardText(item.designRequirement)
  const shortName = normalizeCardText(item.productShortName)
  const main = batchItemDisplayName(item)
  if (design && design !== main) return design
  if (shortName && shortName !== main) return shortName
  return ''
}

function batchItemSummary(item: TaskSkuItem): string {
  const name = batchItemDisplayName(item) || `子项 ${item.sequenceNo ?? ''}`.trim()
  const sub = batchItemSubText(item)
  return sub ? `${name} · ${sub}` : name
}

function batchItemKey(item: TaskSkuItem, index: number): string {
  return `${item.id ?? item.skuCode ?? item.sequenceNo ?? index}-${index}`
}

/** 卡片批量预览:固定展示前 2 项(其余进「查看全部」弹窗),预映射成展示结构 */
function taskCardBatchPreview(task: Task) {
  return batchSkuItems(task)
    .slice(0, 2)
    .map((item, index) => ({
      key: batchItemKey(item, index),
      seq: item.sequenceNo ?? index + 1,
      summary: batchItemSummary(item),
    }))
}

function taskCardTitle(task: Task): string {
  const taskName = normalizeCardText(task.productName)
  if (!isBatchCard(task)) return taskName || task.taskNo
  return taskName || batchItemDisplayName(batchSkuItems(task)[0]) || task.taskNo
}

function filingStatusOfItem(item: TaskSkuItem): string {
  return normalizeCardText(item.erp_sync_status || item.filing_status).toLowerCase()
}

function batchSkuSummary(task: Task): string {
  const items = batchSkuItems(task)
  const count = batchItemCount(task)
  const filed = items.filter((item) => filingStatusOfItem(item) === 'filed').length
  const failed = items.filter((item) => filingStatusOfItem(item) === 'filing_failed').length
  if (filed > 0 || failed > 0) {
    return `${count}个SKU · 已同步${filed} · 失败${failed}`
  }
  const firstSku = items.map((item) => normalizeCardText(item.skuCode)).find(Boolean)
  return firstSku ? `${count}个SKU · ${firstSku}等` : `${count}个SKU`
}

function displaySku(task: Task): string {
  if (isBatchCard(task)) return batchSkuSummary(task)
  return task.primarySkuCode ?? task.sku ?? '-'
}

function openBatchItemsModal(task: Task) {
  batchItemsModalTask.value = task
}

function canCopyTaskField(value: string | null | undefined): boolean {
  const text = String(value ?? '').trim()
  return text !== '' && text !== '-' && text !== '—'
}

async function writeClipboardText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  textarea.style.top = '0'
  document.body.appendChild(textarea)
  textarea.select()
  try {
    const ok = document.execCommand('copy')
    if (!ok) throw new Error('copy command failed')
  } finally {
    document.body.removeChild(textarea)
  }
}

function flashListActionSuccess(message: string) {
  if (listActionSuccessTimer) {
    clearTimeout(listActionSuccessTimer)
    listActionSuccessTimer = null
  }
  listActionSuccess.value = message
  listActionSuccessTimer = setTimeout(() => {
    listActionSuccess.value = ''
    listActionSuccessTimer = null
  }, 1800)
}

async function copyTaskCardText(value: string | null | undefined, label: string) {
  const text = String(value ?? '').trim()
  if (!canCopyTaskField(text)) return
  listActionError.value = ''
  try {
    await writeClipboardText(text)
    flashListActionSuccess(`已复制${label}`)
  } catch {
    listActionError.value = `复制${label}失败，请手动选择复制`
  }
}

function onTaskCardPointerDown(event: PointerEvent) {
  taskCardPointerState.x = event.clientX
  taskCardPointerState.y = event.clientY
  taskCardPointerState.moved = false
}

function onTaskCardPointerMove(event: PointerEvent) {
  if (taskCardPointerState.moved) return
  const dx = Math.abs(event.clientX - taskCardPointerState.x)
  const dy = Math.abs(event.clientY - taskCardPointerState.y)
  taskCardPointerState.moved = dx > 4 || dy > 4
}

function hasActiveTextSelection(): boolean {
  const selected = window.getSelection?.()?.toString().trim()
  return Boolean(selected)
}

function shouldIgnoreTaskCardClick(event: MouseEvent): boolean {
  if (taskCardPointerState.moved || hasActiveTextSelection()) return true
  const target = event.target instanceof HTMLElement ? event.target : null
  return Boolean(
    target?.closest(
      'button,a,input,label,select,textarea,[role="button"],[contenteditable="true"],[data-card-no-nav="true"]',
    ),
  )
}

function onTaskCardClick(event: MouseEvent, task: Task) {
  if (shouldIgnoreTaskCardClick(event)) return
  goDetail(task)
}

/** 池中「接单」仅面向设计侧；单靠 design.work 会漏掉只下发 task.asset_upload / task.design_submit 的普通设计师 */
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
  const seq = ++listActionSeq
  refreshingList.value = true
  listActionError.value = ''
  try {
    await tasksStore.loadTaskListForView(buildListParams({ page: targetPage }))
  } catch (error) {
    if (seq === listActionSeq) {
      listActionError.value = error instanceof Error ? error.message : '加载失败'
    }
  } finally {
    if (seq === listActionSeq) {
      refreshingList.value = false
    }
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
  const seq = ++listActionSeq
  refreshingList.value = true
  listActionError.value = ''
  try {
    page.value = 1
    saveState()
    await tasksStore.loadTaskListForView(buildListParams({ page: 1 }))
  } catch (error) {
    if (seq === listActionSeq) {
      listActionError.value = error instanceof Error ? error.message : '刷新任务列表失败'
    }
  } finally {
    if (seq === listActionSeq) {
      refreshingList.value = false
    }
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
  listActionSeq += 1
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  if (listActionSuccessTimer) clearTimeout(listActionSuccessTimer)
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
  background: rgb(var(--yb-surface) / 0.98);
  border: 1px solid rgb(var(--yb-surface) / 0.6);
  border-radius: 1rem;
  box-shadow: 0 4px 24px -4px rgb(var(--yb-shadow-stone) / 0.06);
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.filter-bar-wrap {
  border-top: 1px solid rgb(var(--yb-legacy-border));
  padding-top: 0.5rem;
  margin-top: 0.25rem;
}
.tab-status-scope-hint {
  margin: 0;
  font-size: 0.6875rem;
  line-height: 1.4;
  color: var(--tc-muted);
}
.task-tabs,
.task-category-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-wrap: wrap;
}
.task-category-switch {
  border-top: 1px solid rgb(var(--yb-legacy-border));
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
  font-family: var(--yb-font-display);
  color: rgb(var(--yb-legacy-text));
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
  background: rgb(var(--yb-surface) / 0.98);
  border: 1px solid rgb(var(--yb-surface) / 0.6);
  border-radius: 1rem;
  font-size: 0.8125rem;
  box-shadow: 0 4px 24px -4px rgb(var(--yb-shadow-stone) / 0.06);
}
.batch-count {
  font-weight: 500;
  color: var(--tc-slate);
  flex: 1;
}
.list-action-error {
  margin: 0;
  padding: 0.625rem 0.875rem;
  border-radius: 0.75rem;
  background: var(--tc-red-soft);
  border: 1px solid var(--tc-red-border);
  color: var(--tc-red);
  font-size: 0.8125rem;
}
.list-action-success {
  margin: 0;
  padding: 0.625rem 0.875rem;
  border-radius: 0.75rem;
  border: 1px solid var(--tc-green-border);
  background: var(--tc-green-ui-soft);
  color: var(--tc-green-deep);
  font-size: 0.8125rem;
  font-weight: 700;
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
/* 分页 */
.footer-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background: var(--tc-panel);
  border: 1px solid rgb(var(--yb-surface) / 0.6);
  border-radius: 1rem;
  box-shadow: 0 4px 24px -4px rgb(var(--yb-shadow-stone) / 0.06);
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
  border: 1px solid var(--tc-border-slate);
  background: var(--tc-panel);
  color: var(--tc-slate);
  cursor: pointer;
}
.pager-btn:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}
.pager-info {
  font-size: 0.75rem;
  color: var(--tc-muted);
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
  color: rgb(var(--yb-text-body));
  background: transparent;
}

.header-card,
.batch-action-bar,
.footer-card {
  border: 1px solid var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
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
  color: var(--tc-text);
  font-size: clamp(1.8rem, 2.6vw, 3rem);
  font-weight: 900;
}

.filter-bar-wrap,
.task-category-switch {
  border-color: var(--tc-border);
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
  background: var(--tc-panel);
  border-color: var(--tc-border);
  color: var(--tc-body);
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.pager-info,
.footer-card,
.page-jump {
  color: var(--tc-muted);
}

.list-action-error {
  background: var(--tc-red-soft);
  border-color: var(--tc-red-border);
  color: var(--tc-red);
}

/* Task center tokens: light admin system. */
.task-list-view {
  --tc-page: rgb(var(--yb-bg-page));
  --tc-panel: rgb(var(--yb-surface));
  --tc-panel-strong: rgb(var(--yb-surface));
  --tc-panel-subtle: rgb(var(--yb-surface-subtle));
  --tc-panel-muted: rgb(var(--yb-surface-muted));
  --tc-card: rgb(var(--yb-surface));
  --tc-card-soft: rgb(var(--yb-surface-soft));
  --tc-border: rgb(var(--yb-border));
  --tc-border-strong: rgb(var(--yb-border-strong));
  --tc-border-subtle: rgb(var(--yb-border-subtle));
  --tc-border-slate: rgb(var(--yb-border-slate));
  --tc-text: rgb(var(--yb-text));
  --tc-text-deep: rgb(var(--yb-text-deep));
  --tc-body: rgb(var(--yb-text-body));
  --tc-slate: rgb(var(--yb-text-slate));
  --tc-muted: rgb(var(--yb-text-muted));
  --tc-soft: rgb(var(--yb-text-soft));
  --tc-faint: rgb(var(--yb-text-faint));
  --tc-disabled: rgb(var(--yb-text-disabled));
  --tc-blue: rgb(var(--yb-brand));
  --tc-blue-strong: rgb(var(--yb-brand-strong));
  --tc-blue-subtle: rgb(var(--yb-brand-subtle));
  --tc-blue-soft: rgb(var(--yb-brand-soft));
  --tc-blue-wash: rgb(var(--yb-brand-wash));
  --tc-blue-border: rgb(var(--yb-brand-border));
  --tc-blue-border-strong: rgb(var(--yb-brand-border-strong));
  --tc-green: rgb(var(--yb-success-strong));
  --tc-green-deep: rgb(var(--yb-success-deep));
  --tc-teal: rgb(var(--yb-success-teal));
  --tc-green-ui-soft: rgb(var(--yb-success-ui-soft));
  --tc-green-soft: rgb(var(--yb-success-soft));
  --tc-green-border: rgb(var(--yb-success-border));
  --tc-amber: rgb(var(--yb-warning-text));
  --tc-amber-accent: rgb(var(--yb-warning-accent));
  --tc-amber-soft: rgb(var(--yb-warning-soft));
  --tc-amber-border: rgb(var(--yb-warning-border));
  --tc-amber-border-soft: rgb(var(--yb-warning-border-soft));
  --tc-red: rgb(var(--yb-danger-text));
  --tc-red-soft: rgb(var(--yb-danger-soft));
  --tc-red-border: rgb(var(--yb-danger-border));
  --tc-pink: rgb(var(--yb-magenta));
  --tc-purple: rgb(var(--yb-purple-text));
  --tc-purple-soft: rgb(var(--yb-purple-soft));
  --tc-purple-border: rgb(var(--yb-purple-border));
  --tc-indigo: rgb(var(--yb-indigo-text));
  --tc-indigo-strong: rgb(var(--yb-indigo-strong));
  --tc-indigo-soft: rgb(var(--yb-indigo-soft));
  background: var(--tc-page);
}

.header-card,
.batch-action-bar,
.footer-card {
  border-color: var(--tc-border);
  background: var(--tc-panel);
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
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
  width: min(32rem, 100%);
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
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
}

.task-tab-active,
.task-category-active {
  background: var(--tc-panel);
  border-color: var(--tc-border);
  color: var(--tc-body);
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.filter-bar-wrap {
  border-color: var(--tc-border);
  background: var(--tc-panel);
}

.filter-bar-wrap :deep(.filter-bar) {
  gap: 0.65rem;
}

.filter-bar-wrap :deep(.field-label) {
  color: var(--tc-muted);
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
    width: 100%;
  }
}

/* Task center correction: readable density and stable wide-screen columns. */
.task-list-view {
  gap: 1rem;
  padding-inline: clamp(0.75rem, 1vw, 1.25rem);
}

.header-card {
  gap: 0.82rem;
  border-radius: 1rem;
  padding: 1rem;
}

.header-card::before {
  opacity: 0.48;
}

.page-header {
  min-height: 3.1rem;
  border-bottom: 1px solid var(--tc-border);
  padding-bottom: 0.7rem;
}

.page-title {
  font-size: clamp(1.75rem, 1.8vw, 2.25rem);
  line-height: 1.04;
}

.page-header-actions :deep(button) {
  min-height: 2.15rem;
}

.task-category-switch {
  padding-top: 0.55rem;
}

.task-tabs {
  border-bottom: 1px solid var(--tc-border);
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
  width: min(27rem, 100%);
}

.filter-bar-wrap {
  margin-top: 0;
  padding-top: 0.75rem;
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
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
}

@media (min-width: 900px) {
  .task-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1280px) {
  .task-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (min-width: 1680px) {
  .task-cards {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

/* Task center defect pass: badges, filters, card scale, and icon/button language. */
.toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.advanced-filter-toggle {
  min-width: 6.25rem;
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
}

.advanced-filter-toggle--active {
  border-color: var(--tc-blue-border);
  background: var(--tc-blue-soft);
  color: var(--tc-blue-strong);
}

.filter-bar-wrap {
  border-top: 1px solid var(--tc-border);
  padding-top: 0.72rem;
  background: var(--tc-panel);
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
  color: var(--tc-disabled);
}

.filter-bar-wrap :deep(.filter-overdue-input) {
  accent-color: var(--tc-blue);
}

.task-cards {
  gap: 0.78rem;
}

.page-header-actions :deep(button),
.toolbar :deep(button),
.footer-card :deep(button) {
  border-radius: 0.62rem;
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
  box-shadow: none;
}

.page-header-actions :deep(button:hover),
.toolbar :deep(button:hover),
.footer-card :deep(button:hover) {
  border-color: var(--tc-border-strong);
  background: var(--tc-panel-muted);
  color: var(--tc-text);
}

.page-header-actions :deep(button svg),
.toolbar :deep(button svg),
.footer-card :deep(button svg) {
  width: 1rem;
  height: 1rem;
  color: var(--tc-muted);
  stroke-width: 2;
}

/* Narrow viewport repair: prevent the left rail from eating content. */
.task-list-view,
.header-card {
  min-width: 0;
}

.task-list-view {
  overflow-x: hidden;
}

@media (max-width: 720px) {
  .task-list-view {
    gap: 0.75rem;
    padding-inline: 0.35rem;
  }

  .header-card {
    border-radius: 0.82rem;
    padding: 0.72rem;
  }

  .page-header {
    gap: 0.65rem;
    align-items: flex-start;
  }

  .page-title {
    font-size: clamp(1.75rem, 10vw, 2.35rem);
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
    width: 100%;
    flex: 1 1 100%;
  }

  .advanced-filter-toggle {
    width: 100%;
  }

  .task-cards {
    grid-template-columns: minmax(0, 1fr);
  }
}

/* Edge and radius repair: keep a consistent gutter so rounded glass corners render cleanly. */
.task-list-view {
  padding-inline: clamp(0.45rem, 0.7vw, 0.75rem);
}

.header-card,
.batch-action-bar,
.footer-card {
  isolation: isolate;
  background-clip: padding-box;
  contain: paint;
  transform: translateZ(0);
}

.header-card::before {
  border-radius: inherit;
}

@media (max-width: 720px) {
  .task-list-view {
    padding-inline: 0.35rem;
  }
}

/* Final task-center layout stabilization under the persistent sidebar. */
.task-list-view {
  padding-inline: 0;
}

.task-cards {
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
}

@media (min-width: 900px) {
  .task-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1280px) {
  .task-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (min-width: 1680px) {
  .task-cards {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (min-width: 2200px) {
  .task-cards {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }
}

.header-card,
.batch-action-bar,
.footer-card {
  overflow: hidden;
  clip-path: inset(0 round 1rem);
}

.batch-action-bar,
.footer-card {
  clip-path: inset(0 round 1rem);
}

/* Legacy dark-corner patches neutralized for light shell. */
.header-card,
.batch-action-bar,
.footer-card {
  border-color: var(--tc-border);
  background: var(--tc-panel);
}

.header-card::before {
  display: none;
}

/* Phase 2: align task center with light shell — final overrides win over legacy dark patches. */
.task-list-view {
  margin: 0;
  min-height: auto;
  padding: 0;
  background: transparent;
}

.header-card,
.batch-action-bar,
.footer-card {
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.header-card::before {
  display: none;
}

.page-header {
  border-bottom-color: var(--tc-border);
}

.filter-bar-wrap {
  background: var(--tc-panel);
}

.advanced-filter-toggle {
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
}

.advanced-filter-toggle--active {
  border-color: var(--tc-blue-border);
  background: var(--tc-blue-soft);
  color: var(--tc-blue-strong);
}

.page-header-actions :deep(button),
.toolbar :deep(button),
.footer-card :deep(button) {
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
  box-shadow: none;
}

.page-header-actions :deep(button:hover),
.toolbar :deep(button:hover),
.footer-card :deep(button:hover) {
  border-color: var(--tc-border-strong);
  background: var(--tc-panel-muted);
  color: var(--tc-text);
}

.page-header-actions :deep(button svg),
.toolbar :deep(button svg),
.footer-card :deep(button svg) {
  color: var(--tc-muted);
}

/* Task center filter tabs: explicit default / hover / active (beats generic :deep(button) resets). */
.task-category-switch :deep(button:not(.task-category-active)),
.task-tabs :deep(button:not(.task-tab-active)) {
  border: 1px solid var(--tc-border-subtle);
  background: var(--tc-panel);
  color: var(--tc-slate);
  font-weight: 500;
  box-shadow: none;
}

.task-category-switch :deep(button:not(.task-category-active):hover),
.task-tabs :deep(button:not(.task-tab-active):hover) {
  border-color: var(--tc-blue-border-strong);
  background: var(--tc-panel-subtle);
  color: var(--tc-text-deep);
  box-shadow: none;
}

.task-category-switch :deep(button.task-category-active),
.task-tabs :deep(button.task-tab-active) {
  border: 1px solid var(--tc-blue);
  background: var(--tc-blue);
  color: rgb(var(--yb-text-inverse));
  font-weight: 700;
  box-shadow: 0 4px 10px rgb(var(--yb-brand) / 0.16);
}

.task-category-switch :deep(button.task-category-active:hover),
.task-tabs :deep(button.task-tab-active:hover) {
  border-color: var(--tc-blue-strong);
  background: var(--tc-blue-strong);
  color: rgb(var(--yb-text-inverse));
  box-shadow: 0 4px 12px rgb(var(--yb-brand-strong) / 0.22);
}

:global(#app .task-list-action-button),
:global(#app .advanced-filter-toggle) {
  border-radius: 0.62rem;
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
}

:global(#app .task-list-action-button--primary) {
  box-shadow: 0 1px 2px rgb(var(--yb-brand) / 0.2);
}

:global(#app .advanced-filter-toggle) {
  box-shadow: none;
}

:global(#app .task-list-action-button:hover),
:global(#app .advanced-filter-toggle:hover) {
  border-color: var(--tc-border-strong);
  background: var(--tc-panel-muted);
  color: var(--tc-text);
}

:global(#app .advanced-filter-toggle.advanced-filter-toggle--active) {
  border-color: var(--tc-blue-border);
  background: var(--tc-blue-soft);
  color: var(--tc-blue-strong);
}

:global(#app .task-category-switch button:not(.task-category-active)),
:global(#app .task-tabs button:not(.task-tab-active)) {
  border: 1px solid var(--tc-border-subtle);
  background: var(--tc-panel);
  color: var(--tc-slate);
  font-weight: 500;
  box-shadow: none;
}

:global(#app .task-category-switch button:not(.task-category-active):hover),
:global(#app .task-tabs button:not(.task-tab-active):hover) {
  border-color: var(--tc-blue-border-strong);
  background: var(--tc-panel-subtle);
  color: var(--tc-text-deep);
  box-shadow: none;
}

:global(#app .task-list-page-title) {
  font-size: clamp(1.75rem, 1.8vw, 2.25rem);
  line-height: 1.04;
}

:global(#app .task-category-active),
:global(#app .task-tab-active) {
  border: 1px solid var(--tc-blue);
  background: var(--tc-blue);
  color: rgb(var(--yb-text-inverse));
  font-weight: 700;
  box-shadow: 0 4px 10px rgb(var(--yb-brand) / 0.16);
}

:global(#app .task-category-active:hover),
:global(#app .task-tab-active:hover) {
  border-color: var(--tc-blue-strong);
  background: var(--tc-blue-strong);
  color: rgb(var(--yb-text-inverse));
  box-shadow: 0 4px 12px rgb(var(--yb-brand-strong) / 0.22);
}

:global(#app .pager-btn) {
  border-radius: 0.62rem;
  border-color: var(--tc-border);
  background: var(--tc-panel);
  color: var(--tc-body);
  box-shadow: none;
}

:global(#app .pager-btn:hover) {
  border-color: var(--tc-border-strong);
  background: var(--tc-panel-muted);
  color: var(--tc-text);
}

@media (max-width: 720px) {
  :global(#app .task-list-page-title) {
    font-size: clamp(1.75rem, 10vw, 2.35rem);
  }
}

.batch-count-pill {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  min-height: 1.25rem;
  border: 1px solid var(--tc-blue-border);
  border-radius: 999px;
  padding: 0.12rem 0.45rem;
  background: var(--tc-blue-soft);
  color: var(--tc-blue-strong);
  font-size: 0.68rem;
  font-weight: 800;
  line-height: 1;
  white-space: nowrap;
}

.batch-modal-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--tc-indigo-soft);
  color: var(--tc-indigo-strong);
  font-family: var(--yb-font-data);
  font-size: 0.65rem;
  font-weight: 800;
}

.batch-items-modal {
  display: grid;
  gap: 0.85rem;
  padding-bottom: 0.75rem;
}

.batch-items-modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  border-bottom: 1px solid var(--tc-border);
  padding-bottom: 0.75rem;
}

.batch-items-modal-title {
  min-width: 0;
  margin: 0;
  color: var(--tc-text);
  font-size: 0.95rem;
  font-weight: 800;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.batch-items-modal-list {
  display: grid;
  gap: 0.45rem;
}

.batch-items-modal-row {
  display: grid;
  grid-template-columns: 1.8rem minmax(0, 1fr) minmax(5rem, auto);
  align-items: center;
  gap: 0.65rem;
  border: 1px solid var(--tc-border);
  border-radius: 0.7rem;
  padding: 0.62rem 0.7rem;
  background: var(--tc-panel);
}

.batch-modal-index {
  width: 1.45rem;
  height: 1.45rem;
}

.batch-modal-main {
  min-width: 0;
}

.batch-modal-name,
.batch-modal-sub {
  margin: 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.batch-modal-name {
  color: var(--tc-text);
  font-size: 0.82rem;
  font-weight: 800;
}

.batch-modal-sub {
  margin-top: 0.15rem;
  color: rgb(var(--yb-text-secondary));
  font-size: 0.72rem;
}

.batch-modal-sku {
  justify-self: end;
  min-width: 0;
  max-width: 10rem;
  overflow: hidden;
  color: var(--tc-soft);
  font-family: var(--yb-font-data);
  font-size: 0.72rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .batch-items-modal-header {
    display: grid;
  }

  .batch-items-modal-row {
    grid-template-columns: 1.8rem minmax(0, 1fr);
  }

  .batch-modal-sku {
    grid-column: 2;
    justify-self: start;
    max-width: 100%;
  }
}
</style>
