<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchEventRow } from '@aw/shared/api/assetWorkbenchApi'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, entityTypeMeta, eventReasonText, eventTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

const rows = ref<AssetWorkbenchEventRow[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const filters = ref({
  event_type: 'all',
  entity_type: 'all',
})
type EventGridRow = AssetWorkbenchEventRow & { event_label: string; entity_label: string; actor_label: string; reason_label: string; created_at_label: string }

const eventFilterOptions = [
  { value: 'all', label: '全部事件' },
  { value: 'submission.created', label: '提交作品' },
  { value: 'system_asset.downloaded', label: '下载素材' },
  { value: 'system_asset.batch_downloaded', label: '批量下载素材' },
  { value: 'item.qc_updated', label: '更新质检' },
  { value: 'settlement.confirmed', label: '确认结算' },
  { value: 'template.assigned', label: '下发作品类型' },
  { value: 'member.identity_changed', label: '调整成员功能' },
  { value: 'account.merged', label: '合并账号' },
]
const entityFilterOptions = [
  { value: 'all', label: '全部对象' },
  { value: 'submission', label: '作品提交' },
  { value: 'submission_item', label: '作品明细' },
  { value: 'submission_file', label: '交付文件' },
  { value: 'system_asset', label: '素材库文件' },
  { value: 'settlement_batch', label: '结算批次' },
  { value: 'member', label: '成员管理' },
  { value: 'profile', label: '人员资料' },
]

const eventGridRowsWithLabels = computed<EventGridRow[]>(() =>
  rows.value.map((row) => ({
    ...row,
    event_label: eventTypeMeta(row.event_type).label,
    entity_label: entityTypeMeta(row.entity_type).label,
    actor_label: row.actor_display_name || row.actor_username || (row.actor_user_id ? `人员编号 ${row.actor_user_id}` : '系统'),
    reason_label: eventReasonText(row.reason),
    created_at_label: formatEventTime(row.created_at),
  })),
)
const eventGridRows = computed(() => eventGridRowsWithLabels.value as unknown as Record<string, unknown>[])
const eventGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'event_label', label: '事件', width: 180 },
  { key: 'entity_label', label: '对象', width: 148 },
  { key: 'entity_id', label: '记录编号', width: 100, align: 'right' },
  { key: 'actor_label', label: '操作者', width: 140 },
  { key: 'reason_label', label: '原因', width: 220 },
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
      event_type: filters.value.event_type === 'all' ? undefined : filters.value.event_type,
      entity_type: filters.value.entity_type === 'all' ? undefined : filters.value.entity_type,
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
        <p>上传、质检、结算、资料维护等关键操作都会留下事件记录。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="loadEvents">刷新</button>
      </div>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <select v-model="filters.event_type" aria-label="事件类型">
          <option v-for="option in eventFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="filters.entity_type" aria-label="对象类型">
          <option v-for="option in entityFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
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
          <span
            v-else-if="column.key === 'entity_label'"
            :class="chipClass(entityTypeMeta(gridRowAsEvent(row).entity_type).tone)"
          >
            {{ value }}
          </span>
          <span v-else-if="column.key === 'entity_id'" class="aw-cell-num">{{ value || '—' }}</span>
          <span v-else-if="column.key === 'actor_label'">{{ value }}</span>
          <span v-else-if="column.key === 'created_at_label'" class="aw-cell-num">{{ value }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-else class="aw-empty-state">
        <h3>暂无操作日志</h3>
        <p>上传、质检、结算、资料维护等关键操作会在这里留下记录。当前筛选返回 {{ total }} 条。</p>
      </div>
    </div>
  </section>
</template>
