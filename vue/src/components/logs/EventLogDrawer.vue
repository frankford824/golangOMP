/**
 * 组件职责：事件日志抽屉，展示任务/审核/定制事件时间轴记录
 * 
 * 核心业务规则（来自 Prompt.md）：
 *   - 事件按 sequence 保序
 *   - gap 时触发 sync_status 校准
 * 
 * 主要 Store：useSyncStore、useTaskStore
 * 预留接口：GET /api/events (mock)
 * 
 * 当前状态：已接入 SequenceGapBanner
 * 维护注意 / 风险点：
 *   - 日志需支持失序自愈
 *   - 抽屉打开/关闭逻辑完整
 */
<template>
  <div
    v-if="visible"
    class="drawer-overlay"
    role="dialog"
    aria-modal="true"
  >
    <div class="drawer-backdrop" @click="handleClose" />
    <aside class="drawer-panel">
      <header class="drawer-header">
        <h2 class="drawer-title">最近事件日志</h2>
        <button type="button" class="drawer-close" aria-label="关闭" @click="handleClose">×</button>
      </header>
      <div v-if="loadError" class="drawer-error">{{ loadError }}</div>
      <template v-else>
        <div v-if="loading" class="drawer-loading">加载中...</div>
        <div v-else-if="!events.length" class="drawer-empty">暂无事件</div>
        <ul v-else class="event-list">
        <li v-for="e in events" :key="e.id" class="event-item">
          <span class="event-time">{{ e.at }}</span>
          <div class="event-item-main">
            <span class="event-type">{{ e.title }}</span>
            <p v-if="e.summary" class="event-summary">{{ e.summary }}</p>
            <span v-else class="event-text">{{ e.refNo }}</span>
          </div>
          <dl
            v-if="isReplacementEvent(e)"
            class="event-replacement-grid"
          >
            <div>
              <dt>替换前稿件</dt>
              <dd>{{ e.previous_asset_id || '—' }}</dd>
            </div>
            <div>
              <dt>当前有效稿件</dt>
              <dd>{{ e.current_asset_id || '—' }}</dd>
            </div>
            <div>
              <dt>替换人</dt>
              <dd>{{ replacementActorText(e) }}</dd>
            </div>
            <div>
              <dt>来源</dt>
              <dd>{{ laneAndDepartmentText(e.workflow_lane, e.source_department) }}</dd>
            </div>
            <div v-if="e.replacement_task_id">
              <dt>关联任务</dt>
              <dd>
                <router-link
                  :to="`/tasks/${e.replacement_task_id}`"
                  class="trace-link"
                >
                  {{ e.replacement_task_id }}
                </router-link>
              </dd>
            </div>
            <div v-if="e.replacement_note">
              <dt>备注</dt>
              <dd>{{ e.replacement_note }}</dd>
            </div>
          </dl>
        </li>
      </ul>
      </template>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RecentEvent } from '@/types/dashboard'
import { tasksApi } from '@/services/api/tasksApi'
import { extractTaskEventsList, mapTaskEventRowToRecentEvent } from '@/domain/mappers/task-events-from-api'
import { userAccountDisplay } from '@/domain/user-display'

const props = defineProps<{ modelValue: boolean; taskId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

const visible = ref(props.modelValue)
const loading = ref(false)
const loadError = ref('')
const events = ref<RecentEvent[]>([])

watch(
  () => props.modelValue,
  (v) => {
    visible.value = v
    if (v) void load()
  },
)
watch(visible, (v) => emit('update:modelValue', v))

watch(
  () => props.taskId,
  () => {
    if (visible.value) void load()
  },
)

async function load() {
  const tid = props.taskId?.trim() ?? ''
  loadError.value = ''
  events.value = []
  if (!tid || tid.startsWith('t-')) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const res = await tasksApi.listTaskEvents(tid)
    const list = extractTaskEventsList(res.data)
    events.value = list.map((row) => mapTaskEventRowToRecentEvent(row, tid))
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载事件失败'
  } finally {
    loading.value = false
  }
}

function isReplacementEvent(event: RecentEvent): boolean {
  return Boolean(event.previous_asset_id || event.current_asset_id || event.replacement_actor_id)
}

function replacementActorText(event: RecentEvent): string {
  return userAccountDisplay(event.replacement_actor_name, event.actor)
}

function laneAndDepartmentText(lane?: string, department?: string): string {
  const laneText =
    lane === 'customization' ? '定制' : lane === 'normal' ? '普通' : lane?.trim() || '—'
  const deptText = department?.trim() || '—'
  return `${laneText} / ${deptText}`
}

function handleClose() {
  visible.value = false
}
</script>

<style scoped>
.drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 7200;
  display: flex;
  justify-content: flex-end;
}
.drawer-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(15, 23, 42, 0.18);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}
.drawer-panel {
  position: relative;
  width: 400px;
  max-width: 100%;
  border-left: 1px solid #e5e7eb;
  background: #ffffff;
  box-shadow: -20px 0 40px -12px rgba(15, 23, 42, 0.12);
  color: #374151;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
  border-bottom: 1px solid #e5e7eb;
  flex-shrink: 0;
}
.drawer-title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 600;
  color: #111827;
}
.drawer-close {
  padding: 0.25rem 0.5rem;
  font-size: 1.25rem;
  line-height: 1;
  color: #6b7280;
  background: transparent;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.drawer-close:hover {
  color: #374151;
  border-color: #9ca3af;
  background: #f3f4f6;
}
.drawer-loading,
.drawer-empty,
.drawer-error {
  padding: 1rem;
  font-size: 0.875rem;
  color: #6b7280;
}
.drawer-error {
  color: #b91c1c;
  background: #fef2f2;
}
.event-list {
  list-style: none;
  margin: 0;
  padding: 1rem;
  overflow-y: auto;
}
.event-item {
  padding: 0.5rem 0;
  border-bottom: 1px solid #f3f4f6;
  font-size: 0.8125rem;
}
.event-time {
  display: block;
  color: #9ca3af;
  margin-bottom: 0.25rem;
}
.event-item-main {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.event-type {
  font-size: 0.75rem;
  font-weight: 600;
  color: #111827;
}
.event-summary {
  margin: 0;
  line-height: 1.45;
  color: #374151;
  font-size: 0.8125rem;
}
.event-text {
  color: #374151;
}
.event-replacement-grid {
  margin: 0.4rem 0 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.25rem 0.75rem;
}
.event-replacement-grid dt {
  font-size: 0.6875rem;
  color: #9ca3af;
}
.event-replacement-grid dd {
  margin: 0;
  font-size: 0.75rem;
  color: #111827;
}
.trace-link {
  color: #2563eb;
  text-decoration: none;
}
.trace-link:hover {
  text-decoration: underline;
}
</style>
