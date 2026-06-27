<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchEventRow } from '@aw/shared/api/assetWorkbenchApi'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, eventTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

const rows = ref<AssetWorkbenchEventRow[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const filters = ref({
  event_type: '',
  entity_type: '',
})
type EventGridRow = AssetWorkbenchEventRow & { event_label: string; actor_label: string; created_at_label: string }

const eventGridRowsWithLabels = computed<EventGridRow[]>(() =>
  rows.value.map((row) => ({
    ...row,
    event_label: eventTypeMeta(row.event_type).label,
    actor_label: row.actor_user_id ? `#${row.actor_user_id}` : 'system',
    created_at_label: formatEventTime(row.created_at),
  })),
)
const eventGridRows = computed(() => eventGridRowsWithLabels.value as unknown as Record<string, unknown>[])
const eventGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'event_label', label: '事件', width: 180 },
  { key: 'entity_type', label: '对象', width: 148 },
  { key: 'entity_id', label: '对象 ID', width: 100, align: 'right' },
  { key: 'actor_label', label: '操作者', width: 100, align: 'right' },
  { key: 'reason', label: '原因', width: 220 },
  { key: 'created_at_label', label: '时间', width: 190 },
])

function formatEventTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value || '—'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function gridRowAsEvent(row: Record<string, unknown>): EventGridRow {
  return row as unknown as EventGridRow
}

async function loadEvents() {
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.listEvents({
      page: 1,
      page_size: 50,
      event_type: filters.value.event_type || undefined,
      entity_type: filters.value.entity_type || undefined,
    })
    rows.value = result.items
    total.value = result.total
  } catch (err) {
    error.value = err instanceof Error ? err.message : '操作日志加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadEvents()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">操作留痕</p>
        <h2>操作日志</h2>
        <p>上传、质检、结算、档案维护等关键操作都会留下事件记录。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="loadEvents">刷新</button>
      </div>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <input v-model="filters.event_type" placeholder="事件类型" />
        <input v-model="filters.entity_type" placeholder="对象类型" />
        <button type="button" @click="loadEvents">筛选</button>
        <span>{{ formatInt(total) }} 条日志</span>
      </div>
      <p v-if="loading" class="aw-copy">正在加载操作日志</p>
      <p v-else-if="error" class="aw-copy">{{ error }}</p>
      <WorkbenchDataGrid
        v-else-if="rows.length"
        :columns="eventGridColumns"
        :rows="eventGridRows"
        row-key="id"
        storage-key="events"
        group-by="event_label"
      >
        <template #cell="{ row, column, value }">
          <span
            v-if="column.key === 'event_label'"
            :class="chipClass(eventTypeMeta(gridRowAsEvent(row).event_type).tone)"
          >
            {{ value }}
          </span>
          <span v-else-if="column.key === 'entity_id'" class="aw-cell-num">{{ value || '—' }}</span>
          <span v-else-if="column.key === 'actor_label'" class="aw-cell-num">{{ value }}</span>
          <span v-else-if="column.key === 'created_at_label'" class="aw-cell-num">{{ value }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-else class="aw-empty-state">
        <h3>暂无操作日志</h3>
        <p>上传、质检、结算、档案维护等关键操作会在这里留下记录。当前筛选返回 {{ total }} 条。</p>
      </div>
    </div>
  </section>
</template>
