<template>
  <div class="event-stream" :class="{ 'event-stream--dashboard': variant === 'dashboard' }">
    <h3 v-if="!hideTitle" class="stream-title">{{ titleText }}</h3>
    <div v-if="loading" class="stream-loading">
      <StatusSkeleton :loading="true" :lines="5" />
    </div>
    <div v-else-if="error" class="stream-error">
      <p>{{ error }}</p>
    </div>
    <div v-else-if="!events.length" class="stream-empty">
      <p>暂无最近事件</p>
    </div>
    <ul v-else :class="variant === 'dashboard' ? 'stream-list stream-list--dashboard' : 'stream-list'">
      <li
        v-for="ev in events"
        :key="ev.id"
        class="stream-item"
        :class="{ navigable: isNavigable(ev) }"
        @click="onSelect(ev)"
      >
        <template v-if="variant === 'dashboard'">
          <span class="event-time event-time--short">{{ timeShort(ev.at) }}</span>
          <span class="event-line">
            <span class="event-title">{{ ev.title }}</span>
            <span class="event-ref" :class="{ 'event-ref--link': isNavigable(ev) }">{{ ev.refNo }}</span>
            <span class="event-actor">{{ ev.actor }}</span>
          </span>
        </template>
        <template v-else>
          <span class="event-time">{{ ev.at }}</span>
          <span class="event-title">{{ ev.title }}</span>
          <span class="event-ref" :class="{ 'event-ref--link': isNavigable(ev) }">{{ ev.refNo }}</span>
          <span class="event-actor">{{ ev.actor }}</span>
        </template>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import type { RecentEvent } from '@/types/dashboard'
import StatusSkeleton from '@/components/common/StatusSkeleton.vue'
import { useRouter } from 'vue-router'

withDefaults(
  defineProps<{
    events: RecentEvent[]
    loading?: boolean
    error?: string
    /** 不渲染内置标题，由外层看板提供 */
    hideTitle?: boolean
    titleText?: string
    /** 与 Pencil 看板「实时动态」单条时间线风格一致 */
    variant?: 'default' | 'dashboard'
  }>(),
  { hideTitle: false, titleText: '最近事件流', variant: 'default' },
)

const emit = defineEmits<{ select: [ev: RecentEvent] }>()
const router = useRouter()

function timeShort(at: string): string {
  if (!at) return ''
  const parts = at.split(/\s+/)
  if (parts.length < 2) return at
  return parts[parts.length - 1] ?? at
}

function isNavigable(ev: RecentEvent): boolean {
  return !!ev.refId
}

function onSelect(ev: RecentEvent) {
  emit('select', ev)
  if (isNavigable(ev)) {
    void router.push(`/tasks/${ev.refId}`)
  }
}
</script>

<style scoped>
.event-stream {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 1rem;
}
.stream-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.stream-loading,
.stream-error,
.stream-empty {
  padding: 0.5rem 0;
  font-size: 0.875rem;
  color: #64748b;
}
.stream-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.stream-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.625rem 0.25rem;
  border-bottom: 1px solid #f1f5f9;
  font-size: 0.8125rem;
  transition: background-color 0.15s;
  border-radius: 0.375rem;
}
.stream-item.navigable {
  cursor: pointer;
}
.stream-item:last-child {
  border-bottom: none;
}
.stream-item.navigable:hover {
  background: #f8fafc;
}
.event-time {
  flex: 0 0 8rem;
  white-space: nowrap;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}
.event-title {
  flex: 0 0 6.25rem;
  color: #0f172a;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.event-ref {
  color: #64748b;
  font-family: var(--yb-font-data);
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}
.event-ref--link {
  color: rgb(16 185 129);
}
.event-actor {
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.event-stream--dashboard {
  background: transparent;
  border: 0;
  border-radius: 0;
  padding: 0;
}
.stream-list--dashboard .stream-item {
  flex-direction: row;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.5rem 0.125rem;
  font-size: 0.8125rem;
  border-color: #e2e8f0;
}
.event-time--short {
  flex: 0 0 2.5rem;
  color: #64748b;
  font-size: 0.75rem;
}
.event-line {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.5rem;
  min-width: 0;
  color: #1e293b;
  line-height: 1.45;
}
.stream-list--dashboard .event-title {
  flex: 0 0 auto;
  font-weight: 500;
}
.stream-list--dashboard .event-ref {
  font-size: 0.75rem;
}
.stream-list--dashboard .event-actor {
  color: #94a3b8;
  font-size: 0.75rem;
}

@media (max-width: 640px) {
  .stream-item {
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
  }
  .event-time {
    flex: 0 0 auto;
  }
  .event-title {
    flex: 1 1 auto;
  }
}
</style>
