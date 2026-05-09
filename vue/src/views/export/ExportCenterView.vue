<template>
  <div class="export-center-view min-h-[100dvh]">
    <div class="page-header">
      <h2 class="page-title">导出中心</h2>
    </div>
    <div v-if="!canExport" class="mt-6">
      <BaseEmptyState title="无导出权限" description="当前角色无任务导出权限，请联系管理员开通。" />
    </div>
    <template v-else>
      <div class="tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="tab-btn"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <section v-if="activeTab === 'tasks'" class="content-card mt-4">
        <h3 class="section-title">任务列表导出</h3>

        <!-- 穿梭框字段选择 -->
        <div class="field-selector-wrap">
          <div class="fs-label">自定义导出字段</div>
          <ExportFieldSelector
            v-model="selectedExportFields"
            :all-fields="allTaskExportFields"
          />
        </div>

        <div class="filters">
          <BaseInput
            v-model="filters.keyword"
            label="关键词"
            placeholder="任务号 / SKU / 产品名称"
          />
          <BaseInput
            v-model="filters.assignee"
            label="人物筛选"
            placeholder="创建人 / 设计师 / 当前处理人 姓名"
          />
          <BaseSelect
            v-model="filters.taskType"
            label="任务类型"
            placeholder="全部类型"
            :options="taskTypeOptions"
          />
          <BaseSelect
            v-model="filters.mainStatus"
            label="状态"
            placeholder="全部状态"
            :options="mainStatusOptions"
          />
          <BaseDatePicker
            v-model="filters.startDate"
            label="开始日期"
            :max="filters.endDate || undefined"
          />
          <BaseDatePicker
            v-model="filters.endDate"
            label="结束日期"
            :min="filters.startDate || undefined"
          />
        </div>
        <div v-if="tasksEmpty" class="mt-4">
          <BaseEmptyState title="暂无任务数据" description="根据当前筛选条件未找到可导出的任务。" />
        </div>
        <div v-else class="mt-4 flex items-center gap-3">
          <BaseButton
            size="sm"
            variant="primary"
            :loading="exporting"
            :disabled="!filteredTasks.length"
            @click="onExportTasks"
          >
            导出 CSV
          </BaseButton>
          <p class="hint-text text-xs text-slate-500">
            共 {{ filteredTasks.length }} 条任务将被导出。
          </p>
        </div>
      </section>

      <section v-else-if="activeTab === 'warehouse'" class="content-card mt-4">
        <h3 class="section-title">仓库记录导出</h3>
        <div v-if="warehouseListEmpty" class="mt-2">
          <BaseEmptyState title="暂无仓库记录" description="当前无涉及仓库接收的任务记录。" />
        </div>
        <template v-else>
          <p class="text-xs text-slate-500 mb-2">共 {{ filteredWarehouseTasks.length }} 条仓库相关任务。</p>
          <div class="mt-2 flex items-center gap-3">
            <BaseButton
              size="sm"
              variant="primary"
              :loading="exporting"
              :disabled="!filteredWarehouseTasks.length"
              @click="onExportWarehouse"
            >
              导出 CSV
            </BaseButton>
          </div>
        </template>
      </section>

      <section v-else-if="activeTab === 'outsource'" class="content-card mt-4">
        <h3 class="section-title">定制记录导出</h3>
        <div v-if="outsourceListEmpty" class="mt-2">
          <BaseEmptyState title="暂无定制记录" description="当前无定制相关审计记录，或仅占位展示。" />
        </div>
        <template v-else>
          <p class="text-xs text-slate-500 mb-2">共 {{ outsourceRecords.length }} 条定制相关记录。</p>
          <div class="mt-2 flex items-center gap-3">
            <BaseButton
              size="sm"
              variant="primary"
              :loading="exporting"
              :disabled="!outsourceRecords.length"
              @click="onExportOutsource"
            >
              导出 CSV
            </BaseButton>
          </div>
        </template>
      </section>

      <section class="content-card mt-4">
        <h3 class="section-title">导出历史</h3>
        <div v-if="exportHistoryLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState
          v-else-if="exportHistoryError"
          :title="exportHistoryError"
          @retry="reloadHistory"
        />
        <BaseEmptyState
          v-else-if="!exportHistory.length"
          title="暂无导出记录"
          description="完成首次导出后将在此展示最近记录。"
        />
        <table v-else class="simple-table mt-2">
          <thead>
            <tr>
              <th>时间</th>
              <th>导出人</th>
              <th>类型</th>
              <th>条数</th>
              <th>文件大小</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in exportHistory" :key="item.id">
              <td>{{ item.exportedAt }}</td>
              <td>{{ item.userName }}</td>
              <td>{{ item.typeLabel }}</td>
              <td>{{ item.count }}</td>
              <td>{{ item.sizeLabel }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import type { Task } from '@/domain/types/task'
import { useTasksStore } from '@/stores/tasks'
import ExportFieldSelector from '@/components/export/ExportFieldSelector.vue'
import type { ExportField } from '@/components/export/ExportFieldSelector.vue'
import { useAuditsStore } from '@/stores/audits'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseDatePicker from '@/components/base/BaseDatePicker.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import { usePermission } from '@/composables/usePermission'
import { TaskTypeEnum } from '@/domain/enums/task-type'
import { TASK_MAIN_STATUS_LABELS } from '@/domain/enums/task-status'
import type { TaskTypeEnumValue } from '@/domain/enums/task-type'
import type { TaskMainStatus } from '@/domain/types/task'
import {
  formatDateOnlyBeijing,
  formatDateTimeBeijing,
  getBeijingDateCompactString,
  nowISO,
  startOfBeijingDayMs,
  endOfBeijingDayMs,
  taskInstantMs,
} from '@/utils/date'
import {
  taskCreatorDisplayName,
  taskCurrentHandlerDisplayName,
  taskDesignerDisplayName,
} from '@/domain/task-actors'

const tasksStore = useTasksStore()
const auditsStore = useAuditsStore()
const { currentUser, can } = usePermission()

const canExport = computed(() => can('export.tasks'))

const tabs = [
  { key: 'tasks' as const, label: '任务列表导出' },
  { key: 'warehouse' as const, label: '仓库记录导出' },
  { key: 'outsource' as const, label: '定制记录导出' },
]
const activeTab = ref<typeof tabs[number]['key']>('tasks')

const exporting = ref(false)

// 穿梭框字段选择：任务导出可选字段定义
const allTaskExportFields: ExportField[] = [
  { key: 'taskNo', label: '任务号' },
  { key: 'sku', label: 'SKU' },
  { key: 'productName', label: '产品名称' },
  { key: 'taskType', label: '任务类型' },
  { key: 'mainStatus', label: '主状态' },
  { key: 'designSubStatus', label: '设计子状态' },
  { key: 'auditSubStatus', label: '审核子状态' },
  { key: 'warehouseSubStatus', label: '仓库子状态' },
  { key: 'requesterName', label: '发起人' },
  { key: 'creatorName', label: '创建人' },
  { key: 'designerName', label: '设计师' },
  { key: 'currentHandlerName', label: '当前处理人' },
  { key: 'groupName', label: '运营组' },
  { key: 'ownerDepartment', label: '归属部门' },
  { key: 'ownerOrgTeam', label: '归属团队' },
  { key: 'priority', label: '优先级' },
  { key: 'dueAt', label: '截止时间' },
  { key: 'costPrice', label: '成本价' },
  { key: 'needOutsource', label: '外协意图(need_outsource)' },
  { key: 'createdAt', label: '创建时间' },
  { key: 'updatedAt', label: '最近更新' },
]

// 默认选中基础字段
const selectedExportFields = ref<string[]>([
  'taskNo',
  'sku',
  'productName',
  'taskType',
  'mainStatus',
  'requesterName',
  'creatorName',
  'designerName',
  'currentHandlerName',
  'dueAt',
  'createdAt',
])

const taskTypeOptions = [
  { value: '', label: '全部类型' },
  { value: TaskTypeEnum.ORIGINAL_PRODUCT_DEV, label: '原品开发' },
  { value: TaskTypeEnum.NEW_PRODUCT_DEV, label: '新品开发' },
  { value: TaskTypeEnum.PURCHASE_TASK, label: '采购任务' },
]

const mainStatusOptions = [
  { value: '', label: '全部状态' },
  ...(Object.entries(TASK_MAIN_STATUS_LABELS) as [TaskMainStatus, string][]).map(([value, label]) => ({
    value,
    label,
  })),
]

const filters = ref({
  keyword: '',
  assignee: '',
  taskType: '' as TaskTypeEnumValue | '',
  mainStatus: '' as TaskMainStatus | '',
  startDate: '',
  endDate: '',
})

function getEffectiveMainStatus(t: Task): string {
  if (t.mainStatus) return t.mainStatus
  const s = t.status
  if (s === 'Draft') return 'Draft'
  if (s === 'PendingAssign' || s === 'InProgress') return 'Designing'
  if (['PendingAuditA', 'RejectedByAuditA', 'PendingAuditB', 'RejectedByAuditB'].includes(s)) return 'Auditing'
  if (['PendingOutsource', 'Outsourcing', 'PendingOutsourceReview', 'PendingCustomizationReview'].includes(s)) return 'Outsourcing'
  if (s === 'PendingWarehouseReceive') return 'Warehouse'
  if (s === 'Completed') return 'Completed'
  if (s === 'Archived') return 'Archived'
  if (s === 'Blocked') return 'Blocked'
  if (s === 'Cancelled') return 'Cancelled'
  return s
}

function normalizeTaskType(t: Task): TaskTypeEnumValue {
  return (t.taskType ?? t.businessType ?? TaskTypeEnum.ORIGINAL_PRODUCT_DEV) as TaskTypeEnumValue
}

const filteredTasks = computed<Task[]>(() => {
  let list = tasksStore.list
  const kw = filters.value.keyword.trim().toLowerCase()
  if (kw) {
    list = list.filter(
      (t) =>
        t.taskNo.toLowerCase().includes(kw) ||
        (t.sku?.toLowerCase().includes(kw)) ||
        t.productName.toLowerCase().includes(kw),
    )
  }
  const assignee = filters.value.assignee.trim()
  if (assignee) {
    list = list.filter(
      (t) =>
        taskDesignerDisplayName(t).includes(assignee) ||
        taskCreatorDisplayName(t).includes(assignee) ||
        taskCurrentHandlerDisplayName(t).includes(assignee),
    )
  }
  if (filters.value.taskType) {
    list = list.filter((t) => normalizeTaskType(t) === filters.value.taskType)
  }
  if (filters.value.mainStatus) {
    list = list.filter((t) => getEffectiveMainStatus(t) === filters.value.mainStatus)
  }
  if (filters.value.startDate) {
    const start = startOfBeijingDayMs(filters.value.startDate)
    list = list.filter((t) => taskInstantMs(t.createdAt) >= start)
  }
  if (filters.value.endDate) {
    const end = endOfBeijingDayMs(filters.value.endDate)
    list = list.filter((t) => taskInstantMs(t.createdAt) <= end)
  }
  return list
})

const tasksEmpty = computed(() => filteredTasks.value.length === 0)

const filteredWarehouseTasks = computed<Task[]>(() => {
  return tasksStore.list.filter(
    (t) =>
      t.warehouseReceiveStatus != null ||
      (t.warehouseSubStatus != null && t.warehouseSubStatus !== 'NOT_REQUIRED'),
  )
})

const warehouseListEmpty = computed(() => filteredWarehouseTasks.value.length === 0)

const outsourceRecords = computed(() => auditsStore.records)

const outsourceListEmpty = computed(() => outsourceRecords.value.length === 0)

interface ExportRecord {
  id: string
  exportedAt: string
  userName: string
  typeLabel: string
  count: number
  sizeLabel: string
}

const exportHistory = ref<ExportRecord[]>([])
const exportHistoryLoading = ref(true)
const exportHistoryError = ref('')

function loadHistoryMock() {
  exportHistoryLoading.value = false
}

function reloadHistory() {
  exportHistoryError.value = ''
  exportHistoryLoading.value = true
  setTimeout(() => {
    exportHistoryLoading.value = false
  }, 300)
}

function formatDate(iso: string) {
  return formatDateOnlyBeijing(iso)
}

function buildTaskCsv(tasks: Task[]): string {
  const header = [
    '任务号',
    'SKU',
    '产品名称',
    '状态',
    '发起人',
    '创建人',
    '设计师',
    '当前处理人',
    '归属部门',
    '归属团队',
    '创建时间',
  ]
  const rows = tasks.map((t) => [
    t.taskNo,
    t.sku ?? '',
    t.productName,
    t.status,
    t.requesterName ?? '',
    taskCreatorDisplayName(t),
    taskDesignerDisplayName(t),
    taskCurrentHandlerDisplayName(t),
    t.ownerDepartment ?? '',
    t.ownerOrgTeam ?? '',
    formatDate(t.createdAt),
  ])
  return [header, ...rows]
    .map((cols) => cols.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(','))
    .join('\n')
}

function pushHistory(typeLabel: string, count: number, sizeLabel: string) {
  exportHistory.value.unshift({
    id: `exp-${Date.now()}`,
    exportedAt: formatDateTimeBeijing(nowISO()),
    userName: currentUser.value?.name ?? '未知用户',
    typeLabel,
    count,
    sizeLabel,
  })
}

function downloadCsv(content: string, filename: string) {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', filename)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
  return blob
}

function onExportTasks() {
  if (!filteredTasks.value.length) return
  try {
    exporting.value = true
    const csv = buildTaskCsv(filteredTasks.value)
    const dateStr = getBeijingDateCompactString()
    const blob = downloadCsv(csv, `tasks_export_${dateStr}.csv`)
    const sizeLabel = `${(blob.size / 1024).toFixed(1)} KB`
    pushHistory('任务列表导出', filteredTasks.value.length, sizeLabel)
  } catch (e) {
    exportHistoryError.value = '导出失败，请稍后重试'
    // eslint-disable-next-line no-console
    console.error(e)
  } finally {
    exporting.value = false
  }
}

function onExportWarehouse() {
  if (!filteredWarehouseTasks.value.length) return
  try {
    exporting.value = true
    const csv = buildTaskCsv(filteredWarehouseTasks.value)
    const dateStr = getBeijingDateCompactString()
    const blob = downloadCsv(csv, `warehouse_export_${dateStr}.csv`)
    const sizeLabel = `${(blob.size / 1024).toFixed(1)} KB`
    pushHistory('仓库记录导出', filteredWarehouseTasks.value.length, sizeLabel)
  } catch (e) {
    exportHistoryError.value = '导出失败，请稍后重试'
    // eslint-disable-next-line no-console
    console.error(e)
  } finally {
    exporting.value = false
  }
}

const AUDIT_ACTION_LABELS: Record<string, string> = {
  sign: '提交审核',
  pass: '通过',
  reject: '打回',
  transfer: '转交',
  handover: '交班',
  takeover: '接手',
  complete: '结单',
  archive: '归档',
  return: '退回',
  warehouse_receive: '仓库接收',
}

function buildOutsourceCsv(): string {
  const header = ['时间', '操作人', '任务号', '动作', '原因']
  const rows = outsourceRecords.value.map((r) => {
    const task = tasksStore.getById(r.taskId)
    return [
      formatDate(r.createdAt),
      r.auditorName,
      task?.taskNo ?? r.taskId,
      AUDIT_ACTION_LABELS[r.action] ?? r.action,
      r.comment ?? '',
    ]
  })
  return [header, ...rows]
    .map((cols) => cols.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(','))
    .join('\n')
}

function onExportOutsource() {
  if (!outsourceRecords.value.length) return
  try {
    exporting.value = true
    const csv = buildOutsourceCsv()
    const dateStr = getBeijingDateCompactString()
    const blob = downloadCsv(csv, `outsource_export_${dateStr}.csv`)
    const sizeLabel = `${(blob.size / 1024).toFixed(1)} KB`
    pushHistory('定制记录导出', outsourceRecords.value.length, sizeLabel)
  } catch (e) {
    exportHistoryError.value = '导出失败，请稍后重试'
    // eslint-disable-next-line no-console
    console.error(e)
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  exportHistoryLoading.value = true
  setTimeout(() => {
    loadHistoryMock()
    exportHistoryLoading.value = false
  }, 300)
})
</script>

<style scoped>
.export-center-view {
  padding: 0;
}
.field-selector-wrap {
  margin-bottom: 1rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #e2e8f0;
}
.fs-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.5rem;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}
.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: #0f172a;
}
.tabs {
  display: inline-flex;
  gap: 0.25rem;
  padding: 0.125rem;
  border-radius: 9999px;
  background: #e2e8f0;
}
.tab-btn {
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  border-radius: 9999px;
  border: none;
  background: transparent;
  color: #475569;
}
.tab-btn.active {
  background: #fff;
  color: #0f172a;
}
.content-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 0.75rem;
  padding: 1rem;
}
.section-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.filters {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 1rem;
}
.simple-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.75rem;
}
.simple-table th,
.simple-table td {
  border: 1px solid #e2e8f0;
  padding: 0.25rem 0.5rem;
  text-align: left;
}
</style>
