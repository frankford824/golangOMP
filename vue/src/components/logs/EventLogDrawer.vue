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
  background:
    radial-gradient(circle at 18% 5%, rgba(255, 45, 141, 0.16), transparent 28rem),
    radial-gradient(circle at 86% 0%, rgba(100, 210, 255, 0.12), transparent 30rem),
    rgba(0, 0, 0, 0.82);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
}
.drawer-panel {
  position: relative;
  width: 400px;
  max-width: 100%;
  border-left: 1px solid rgba(160, 176, 204, 0.24);
  background:
    radial-gradient(circle at 0% 0%, rgba(100, 210, 255, 0.08), transparent 18rem),
    linear-gradient(145deg, rgba(17, 24, 39, 0.99), rgba(7, 12, 20, 0.995));
  box-shadow: -28px 0 70px -42px rgba(0, 0, 0, 0.95);
  color: #dce7f7;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
  border-bottom: 1px solid rgba(160, 176, 204, 0.18);
  flex-shrink: 0;
}
.drawer-title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 600;
  color: #f8fbff;
}
.drawer-close {
  padding: 0.25rem;
  font-size: 1.25rem;
  line-height: 1;
  color: #b8c4d8;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(160, 176, 204, 0.18);
  border-radius: 0.65rem;
  cursor: pointer;
}
.drawer-close:hover {
  color: #ffffff;
  border-color: rgba(100, 210, 255, 0.42);
  background: rgba(34, 48, 71, 0.92);
}
.drawer-loading,
.drawer-empty,
.drawer-error {
  padding: 1rem;
  font-size: 0.875rem;
  color: #b8c4d8;
}
.drawer-error {
  color: #ffb4ad;
  background: rgba(255, 69, 58, 0.12);
}
.event-list {
  list-style: none;
  margin: 0;
  padding: 1rem;
  overflow-y: auto;
}
.event-item {
  padding: 0.5rem 0;
  border-bottom: 1px solid rgba(160, 176, 204, 0.14);
  font-size: 0.8125rem;
}
.event-time {
  display: block;
  color: #8fa0b8;
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
  color: #f8fbff;
}
.event-summary {
  margin: 0;
  line-height: 1.45;
  color: #dce7f7;
  font-size: 0.8125rem;
}
.event-text {
  color: #dce7f7;
}
.event-replacement-grid {
  margin: 0.4rem 0 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.25rem 0.75rem;
}
.event-replacement-grid dt {
  font-size: 0.6875rem;
  color: #8fa0b8;
}
.event-replacement-grid dd {
  margin: 0;
  font-size: 0.75rem;
  color: #f8fbff;
}
.trace-link {
  color: #64d2ff;
  text-decoration: none;
}
.trace-link:hover {
  text-decoration: underline;
}
</style>
