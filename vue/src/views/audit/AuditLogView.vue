<template>
  <div class="audit-log-view min-h-[100dvh]">
    <div class="page-header">
      <h2 class="page-title">审计日志</h2>
    </div>
    <div v-if="!canView" class="mt-6">
      <BaseEmptyState title="无查看权限" description="当前角色无审计日志查看权限，请联系管理员开通。" />
    </div>
    <template v-else>
      <section class="content-card rounded-lg border border-gray-200 shadow-sm p-6">
        <h3 class="section-title">筛选条件</h3>
        <div
          class="grid grid-cols-1 sm:grid-cols-2 lg:flex lg:items-center lg:gap-x-4 lg:flex-wrap gap-4"
        >
          <BaseInput
            v-model="filters.taskNo"
            label="任务号"
            placeholder="按任务号筛选"
          />
          <BaseInput
            v-model="filters.auditor"
            label="操作人"
            placeholder="按操作人筛选"
          />
          <BaseSelect
            v-model="filters.action"
            label="动作类型"
            placeholder="全部动作"
            :options="actionTypeOptions"
          />
          <BaseDatePicker
            v-model="filters.start"
            label="开始日期"
          />
          <BaseDatePicker
            v-model="filters.end"
            label="结束日期"
          />
        </div>
      </section>

      <section class="content-card mt-4 rounded-lg border border-gray-200 shadow-sm p-6">
        <h3 class="section-title">日志列表</h3>
        <div v-if="loading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState
          v-else-if="loadError"
          :title="loadError"
          @retry="reload"
        />
        <BaseEmptyState
          v-else-if="!pagedRecords.length"
          title="暂无日志"
          description="根据当前筛选条件未找到日志记录。"
        />
        <template v-else>
          <table class="simple-table mt-2">
            <thead>
              <tr>
                <th>时间</th>
                <th>操作人</th>
                <th>任务号</th>
                <th>动作</th>
                <th>原因</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in pagedRecords" :key="r.id">
                <td>{{ formatAt(r.createdAt) }}</td>
                <td>{{ r.auditorName }}</td>
                <td>{{ taskNoOf(r.taskId) }}</td>
                <td>{{ actionLabel(r.action) }}</td>
                <td>{{ r.comment ?? '-' }}</td>
              </tr>
            </tbody>
          </table>
          <div class="pager">
            <button
              type="button"
              class="pager-btn"
              :disabled="page === 1"
              @click="page--"
            >
              上一页
            </button>
            <span class="pager-info text-xs text-slate-500">
              第 {{ page }} 页 / 共 {{ totalPages }} 页（{{ filteredRecords.length }} 条）
            </span>
            <button
              type="button"
              class="pager-btn"
              :disabled="page === totalPages"
              @click="page++"
            >
              下一页
            </button>
          </div>
        </template>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useTasksStore } from '@/stores/tasks'
import { useAuditsStore } from '@/stores/audits'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseDatePicker from '@/components/base/BaseDatePicker.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import { usePermission } from '@/composables/usePermission'
import {
  endOfBeijingDayMs,
  formatDateTimeBeijing,
  startOfBeijingDayMs,
  taskInstantMs,
} from '@/utils/date'
import { getAuditActionLabel, AUDIT_ACTION_LABELS } from '@/domain/mappers/audit-action'
import type { AuditRecord } from '@/types'

const tasksStore = useTasksStore()
const auditsStore = useAuditsStore()
const { can } = usePermission()

const canView = computed(() => can('audit.view'))

const actionTypeOptions = [
  { value: '', label: '全部动作' },
  ...(Object.entries(AUDIT_ACTION_LABELS) as [AuditRecord['action'], string][]).map(([value, label]) => ({
    value,
    label,
  })),
]

const loading = computed(() => auditsStore.loading)
const loadError = computed(() => auditsStore.loadError)

const filters = ref({
  taskNo: '',
  auditor: '',
  action: '',
  start: '',
  end: '',
})

const allRecords = computed(() => auditsStore.records)

const filteredRecords = computed(() => {
  let list = allRecords.value
  const f = filters.value
  if (f.taskNo.trim()) {
    const kw = f.taskNo.trim()
    list = list.filter((r) => taskNoOf(r.taskId).includes(kw))
  }
  if (f.auditor.trim()) {
    list = list.filter((r) => r.auditorName.includes(f.auditor.trim()))
  }
  if (f.action) {
    list = list.filter((r) => r.action === f.action)
  }
  if (f.start) {
    const start = startOfBeijingDayMs(f.start)
    list = list.filter((r) => taskInstantMs(r.createdAt) >= start)
  }
  if (f.end) {
    const end = endOfBeijingDayMs(f.end)
    list = list.filter((r) => taskInstantMs(r.createdAt) <= end)
  }
  return list.slice().sort((a, b) => b.createdAt.localeCompare(a.createdAt))
})

const pageSize = 20
const page = ref(1)

const totalPages = computed(() =>
  filteredRecords.value.length ? Math.ceil(filteredRecords.value.length / pageSize) : 1,
)

const pagedRecords = computed(() => {
  const start = (page.value - 1) * pageSize
  return filteredRecords.value.slice(start, start + pageSize)
})

function reload() {
  loadAuditLogs()
}

function formatAt(iso: string) {
  return formatDateTimeBeijing(iso)
}

function taskNoOf(taskId: string) {
  const task = tasksStore.getById(taskId)
  return task?.taskNo ?? taskId
}

function actionLabel(action: AuditRecord['action']) {
  return getAuditActionLabel(action)
}

async function loadAuditLogs() {
  await auditsStore.loadAuditLogs({
    taskNo: filters.value.taskNo.trim() || undefined,
    auditor: filters.value.auditor.trim() || undefined,
    action: filters.value.action || undefined,
    start: filters.value.start || undefined,
    end: filters.value.end || undefined,
  })
}

onMounted(() => {
  loadAuditLogs()
})
</script>

<style scoped>
.audit-log-view {
  padding: 0;
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
.content-card {
  background: #fff;
}
.section-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
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
.pager {
  margin-top: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.pager-btn {
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  border-radius: 9999px;
  border: 1px solid #cbd5f5;
  background: #fff;
}
.pager-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
