<template>
  <div class="audit-history-panel">
    <h4 class="block-title">审核记录</h4>
    <div v-if="!records.length" class="empty">暂无审核记录</div>
    <ul v-else class="record-list">
      <li v-for="r in records" :key="r.id" class="record-item">
        <span class="record-action">{{ actionLabel(r.action) }}</span>
        <span class="record-meta">{{ r.auditorName }} · {{ formatDate(r.createdAt) }}</span>
        <p v-if="r.comment" class="record-comment">{{ r.comment }}</p>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import type { AuditRecord } from '@/types'
import { formatDateBeijing } from '@/utils/date'
import { getAuditActionLabel } from '@/domain/mappers/audit-action'

defineProps<{ records: AuditRecord[] }>()

function actionLabel(action: AuditRecord['action']) {
  return getAuditActionLabel(action)
}

function formatDate(iso: string) {
  return formatDateBeijing(iso)
}
</script>

<style scoped>
.audit-history-panel {
  padding: 1rem;
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 8px;
}
.block-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}
.empty {
  font-size: 0.875rem;
  color: rgb(var(--yb-text-placeholder));
}
.record-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.record-item {
  padding: 0.5rem 0;
  border-bottom: 1px solid rgb(var(--yb-surface-slate));
  font-size: 0.8125rem;
}
.record-item:last-child {
  border-bottom: none;
}
.record-action {
  font-weight: 500;
  color: rgb(var(--yb-text-navy));
  margin-right: 0.5rem;
}
.record-meta {
  color: rgb(var(--yb-text-muted-strong));
}
.record-comment {
  margin: 0.25rem 0 0;
  color: rgb(var(--yb-text-soft));
}
</style>
