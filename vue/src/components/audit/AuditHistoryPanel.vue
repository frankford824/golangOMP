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
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.block-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.empty {
  font-size: 0.875rem;
  color: #94a3b8;
}
.record-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.record-item {
  padding: 0.5rem 0;
  border-bottom: 1px solid #f1f5f9;
  font-size: 0.8125rem;
}
.record-item:last-child {
  border-bottom: none;
}
.record-action {
  font-weight: 500;
  color: #0f172a;
  margin-right: 0.5rem;
}
.record-meta {
  color: #64748b;
}
.record-comment {
  margin: 0.25rem 0 0;
  color: #475569;
}
</style>
