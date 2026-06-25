<template>
  <div class="task-table w-full min-w-0">
    <div class="task-table__head" role="row">
      <span class="col-task">任务</span>
      <span class="col-owner">负责人</span>
      <span class="col-status">状态</span>
      <span class="col-due">截止</span>
    </div>
    <div v-if="!rows.length" class="task-table__empty" role="status">暂无任务数据</div>
    <div
      v-for="(row, i) in rows"
      v-else
      :key="row.id"
      class="task-table__row"
      :class="{ 'task-table__row--alt': i % 2 === 0 }"
      role="row"
    >
      <span class="col-task" :title="row.title">{{ row.title }}</span>
      <span class="col-owner">{{ row.owner }}</span>
      <span
        class="col-status"
        :class="`task-table__status--${row.statusTone}`"
      >{{ row.statusLabel }}</span>
      <span class="col-due">{{ row.due }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '@/domain/types/task'
import { getTaskStatusLabel } from '@/domain/enums/task-status'
import { isDoneStatus, isInAuditQueue, isInCustomizationFlow } from '@/domain/task-actions'
import { taskBeijingDateKey } from '@/utils/date'

const props = defineProps<{
  tasks: Task[]
}>()

const rows = computed(() => {
  return props.tasks.map((t) => {
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
</script>

<style scoped>
.task-table {
  display: flex;
  flex-direction: column;
  gap: 0;
  border-radius: 0.5rem;
  overflow: hidden;
  font-size: 0.8125rem;
  min-width: 32rem;
}
@media (min-width: 640px) {
  .task-table {
    min-width: 0;
  }
}
.task-table__head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 6.5rem 5rem 5.5rem;
  align-items: center;
  column-gap: 0.5rem;
  padding: 0.75rem;
  background: rgb(var(--yb-surface-slate));
  font-weight: 600;
  color: rgb(var(--yb-text-slate));
}
.task-table__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 6.5rem 5rem 5.5rem;
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
.task-table__empty {
  padding: 1.5rem 0.75rem;
  text-align: center;
  color: rgb(var(--yb-text-placeholder));
  background: rgb(var(--yb-surface-canvas));
}
</style>
