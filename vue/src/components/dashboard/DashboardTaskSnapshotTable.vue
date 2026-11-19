<template>
  <div class="task-snapshot">
    <div v-if="rows.length || selectedStatus" class="task-snapshot__filter">
      <label :for="statusFilterId">任务状态</label>
      <select :id="statusFilterId" v-model="selectedStatus">
        <option value="">全部状态</option>
        <option v-for="status in statuses" :key="status" :value="status">{{ status }}</option>
      </select>
      <span role="status">{{ visibleRows.length }} 项</span>
    </div>
    <OAEmpty
      v-if="!visibleRows.length"
      :title="selectedStatus ? '所选状态暂无近期任务' : '暂无近期任务'"
      :kind="selectedStatus ? 'search' : 'inbox'"
    >
      <BaseButton v-if="selectedStatus" variant="ghost" size="sm" @click="selectedStatus = ''">
        清除状态筛选
      </BaseButton>
    </OAEmpty>
    <div v-else class="task-snapshot__scroll" role="region" aria-label="近期任务列表" tabindex="0">
      <div class="task-table" role="table" aria-label="近期任务列表">
        <div class="task-table__head" role="row">
          <span class="col-task" role="columnheader">任务</span>
          <span class="col-owner" role="columnheader">负责人</span>
          <span class="col-status" role="columnheader">状态</span>
          <span class="col-due" role="columnheader">截止</span>
        </div>
        <div
          v-for="(row, i) in visibleRows"
          :key="row.id"
          class="task-table__row"
          :class="{ 'task-table__row--alt': i % 2 === 0 }"
          role="row"
        >
          <span class="col-task" role="cell">
            <RouterLink :to="taskDetailLocation(row.id, router.currentRoute?.value.fullPath || '/?workspace=tasks')" :title="row.title">
              {{ row.title }}
            </RouterLink>
          </span>
          <span class="col-owner" role="cell" :title="row.owner">{{ row.owner }}</span>
          <span class="col-status" role="cell" :class="`task-table__status--${row.statusTone}`">{{ row.statusLabel }}</span>
          <span class="col-due" role="cell">{{ row.due }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useRouter } from 'vue-router'
import { taskDetailLocation } from '@/oa/taskNavigation'
import BaseButton from '@/components/base/BaseButton.vue'
import OAEmpty from '@/oa/components/OAEmpty.vue'
import type { Task } from '@/domain/types/task'
import type { TaskOperationalRecentTask } from '@/types/dashboard'
import { getTaskStatusLabel } from '@/domain/enums/task-status'
import { isDoneStatus, isInAuditQueue, isInCustomizationFlow } from '@/domain/task-actions'
import { taskBeijingDateKey } from '@/utils/date'

const router = useRouter()
const props = defineProps<{
  tasks?: Task[]
  snapshots?: TaskOperationalRecentTask[]
}>()

const rows = computed(() => {
  if (props.snapshots) {
    return props.snapshots.map((task) => ({
      id: String(task.task_id),
      title: task.product_name.trim() || task.task_no,
      owner: task.owner_name || '—',
      statusLabel: getTaskStatusLabel(task.task_status as Task['status']),
      statusTone: task.task_status === 'Completed' || task.task_status === 'Archived' || task.task_status === 'Cancelled'
        ? 'success' as const
        : task.task_status.includes('Audit')
          ? 'info' as const
          : task.task_status.includes('Customization')
            ? 'warning' as const
            : 'neutral' as const,
      due: taskBeijingDateKey(task.deadline_at ?? undefined) || '—',
    }))
  }
  return (props.tasks ?? []).map((t) => {
    const statusLabel = getTaskStatusLabel(t.status)
    let statusTone: 'neutral' | 'info' | 'warning' | 'success' = 'neutral'
    if (isDoneStatus(t)) statusTone = 'success'
    else if (isInAuditQueue(t)) statusTone = 'info'
    else if (isInCustomizationFlow(t)) statusTone = 'warning'

    const title = (t.productName && t.productName.trim()) || t.taskNo
    const owner =
      t.designerName ||
      t.currentHandlerName ||
      t.requesterName ||
      '—'
    const due = taskBeijingDateKey(t.dueAt) || '—'
    return { id: t.id, title, owner, statusLabel, statusTone, due }
  })
})

const selectedStatus = ref('')
const statusFilterId = useId()
const statuses = computed(() => [...new Set([...rows.value.map((row) => row.statusLabel), selectedStatus.value])].filter(Boolean))
const visibleRows = computed(() => selectedStatus.value
  ? rows.value.filter((row) => row.statusLabel === selectedStatus.value)
  : rows.value)
</script>

<style scoped>
.task-snapshot {
  display: flex;
  flex: 1;
  min-height: 8rem;
  min-width: 0;
  flex-direction: column;
  gap: 0.5rem;
}
.task-snapshot__filter {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.8125rem;
}
.task-snapshot__filter select {
  height: 2rem;
  padding-inline: 0.5rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.375rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
}
.task-snapshot__filter span:last-child { margin-inline-start: auto; }
.task-snapshot__scroll {
  overflow: auto;
  overscroll-behavior: contain;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.65rem;
}
.col-task a { color: rgb(var(--yb-brand)); }
.col-task a:hover { text-decoration: underline; }
.task-table {
  display: flex;
  flex-direction: column;
  gap: 0;
  border-radius: 0.5rem;
  font-size: 0.8125rem;
  min-width: 27rem;
}
.task-table__head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(3rem, 0.4fr) 5.5rem 5.5rem;
  align-items: center;
  column-gap: 0.5rem;
  padding: 0.75rem;
  background: rgb(var(--yb-surface-slate));
  font-weight: 600;
  color: rgb(var(--yb-text-slate));
}
.task-table__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(3rem, 0.4fr) 5.5rem 5.5rem;
  column-gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  min-height: 2.5rem;
  align-items: center;
  border-top: 1px solid rgb(var(--yb-surface-slate));
  color: rgb(var(--yb-text-navy));
}
.task-table__row--alt {
  background: rgb(var(--yb-surface-zinc-tint));
}
.task-table__row:not(.task-table__row--alt) {
  background: rgb(var(--yb-surface));
}
.col-task {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.col-owner {
  color: rgb(var(--yb-text-slate));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.col-due {
  font-variant-numeric: tabular-nums;
  color: rgb(var(--yb-text-slate));
}
.task-table__status--info {
  color: rgb(var(--yb-brand));
  font-weight: 500;
}
.task-table__status--warning {
  color: rgb(var(--yb-warning));
  font-weight: 500;
}
.task-table__status--success {
  color: rgb(var(--yb-success-emerald));
  font-weight: 500;
}
.task-table__status--neutral {
  color: rgb(var(--yb-text-navy));
}
</style>
