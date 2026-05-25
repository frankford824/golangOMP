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

        <p class="hint-text text-xs text-slate-500 mb-3">
          导出使用<strong>任务中心</strong>当前 Tab、关键词与高级筛选条件。筛选结果共
          <strong>{{ tasksStore.listTotal }}</strong> 条，当前页已加载
          <strong>{{ currentPageTasks.length }}</strong> 条。
        </p>
        <p
          v-if="exportFeedback"
          class="text-xs mb-2"
          :class="exportFeedbackIsError ? 'text-red-600' : 'text-emerald-700'"
        >
          {{ exportFeedback }}
        </p>
        <div v-if="!hasTaskCenterContext" class="mt-2">
          <BaseEmptyState
            title="请先在任务中心加载列表"
            description="打开任务中心并应用筛选后，再回到导出中心导出全部筛选结果。"
          />
        </div>
        <div v-else class="mt-2 flex flex-wrap items-center gap-3">
          <BaseButton
            size="sm"
            variant="secondary"
            :loading="exporting && !exportingAll"
            :disabled="exportBusy || !currentPageTasks.length"
            @click="onExportCurrentPage"
          >
            导出当前页
          </BaseButton>
          <BaseButton
            size="sm"
            variant="primary"
            :loading="exportingAll"
            :disabled="exportBusy || tasksStore.listTotal === 0"
            @click="onExportAllFiltered"
          >
            导出全部筛选结果
          </BaseButton>
          <p class="hint-text text-xs text-slate-500">
            当前页 {{ currentPageTasks.length }} 条；全部筛选结果最多同步导出
            {{ TASK_EXPORT_MAX_TOTAL }} 条。
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
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import { usePermission } from '@/composables/usePermission'
import { TASK_EXPORT_MAX_TOTAL } from '@/constants/task-export'
import {
  formatDateOnlyBeijing,
  formatDateTimeBeijing,
  getBeijingDateCompactString,
  nowISO,
} from '@/utils/date'
import {
  fetchAllFilteredTasks,
  TaskExportAllError,
} from '@/utils/export-all-filtered-tasks'
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
const exportingAll = ref(false)
const exportFeedback = ref('')
const exportFeedbackIsError = ref(false)

const exportBusy = computed(() => exporting.value || exportingAll.value)
const currentPageTasks = computed(() => tasksStore.list)
const hasTaskCenterContext = computed(() => tasksStore.lastListQueryParams != null)

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

function setExportFeedback(message: string, isError = false) {
  exportFeedback.value = message
  exportFeedbackIsError.value = isError
}

function runTaskCsvExport(tasks: Task[], historyLabel: string) {
  const csv = buildTaskCsv(tasks)
  const dateStr = getBeijingDateCompactString()
  const blob = downloadCsv(csv, `tasks_export_${dateStr}.csv`)
  const sizeLabel = `${(blob.size / 1024).toFixed(1)} KB`
  pushHistory(historyLabel, tasks.length, sizeLabel)
}

function onExportCurrentPage() {
  if (!currentPageTasks.value.length) return
  setExportFeedback('')
  try {
    exporting.value = true
    runTaskCsvExport(currentPageTasks.value, '任务列表导出（当前页）')
    setExportFeedback(`已导出当前页 ${currentPageTasks.value.length} 条任务。`)
  } catch (e) {
    setExportFeedback('导出失败，请稍后重试', true)
    // eslint-disable-next-line no-console
    console.error(e)
  } finally {
    exporting.value = false
  }
}

async function onExportAllFiltered() {
  if (exportBusy.value) return
  setExportFeedback('正在导出全部筛选结果...', false)
  exportingAll.value = true
  try {
    const { items, total } = await fetchAllFilteredTasks(
      tasksStore.lastListQueryParams,
      (params) => tasksStore.loadTaskListSnapshot(params),
    )
    runTaskCsvExport(items, '任务列表导出（全部筛选）')
    setExportFeedback(`已导出全部筛选结果，共 ${total} 条任务。`)
  } catch (e) {
    const message =
      e instanceof TaskExportAllError
        ? e.message
        : e instanceof Error
          ? e.message
          : '导出失败，请稍后重试'
    setExportFeedback(message, true)
    // eslint-disable-next-line no-console
    console.error(e)
  } finally {
    exportingAll.value = false
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
  background: #2563eb;
  color: #ffffff;
}
.content-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  padding: 1rem;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
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
.simple-table th {
  background: #f3f4f6;
  color: #374151;
  font-weight: 600;
}
.simple-table th,
.simple-table td {
  border: 1px solid #e5e7eb;
  padding: 0.25rem 0.5rem;
  text-align: left;
  color: #111827;
}
.simple-table tbody tr:hover td {
  background: #f9fafb;
}
</style>
