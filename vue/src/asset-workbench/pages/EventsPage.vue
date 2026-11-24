<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchEventRow, type PersonnelSuggestion } from '@aw/shared/api/assetWorkbenchApi'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, entityTypeMeta, eventReasonText, eventTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import PersonnelAutocomplete from '@aw/shared/ui/PersonnelAutocomplete.vue'

const filters = ref({
  event_type: 'all',
  entity_type: 'all',
  entity_id: '',
  actor_user_id: '',
})
const selectedActor = ref<PersonnelSuggestion | null>(null)
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/settings/events')
type EventGridRow = AssetWorkbenchEventRow & { event_label: string; entity_label: string; actor_label: string; reason_label: string; created_at_label: string }

const eventFilterOptions = [
  { value: 'all', label: '全部事件' },
  { value: 'profile.pii_viewed', label: '查看完整资料' },
  { value: 'profile.upserted', label: '更新人员资料' },
  { value: 'settlement.visibility_changed', label: '调整客户端结算展示' },
  { value: 'submission.created', label: '提交作品' },
  { value: 'system_asset.downloaded', label: '下载素材' },
  { value: 'system_asset.batch_downloaded', label: '批量下载素材' },
  { value: 'file.moved', label: '移动文件' },
  { value: 'file.deleted', label: '删除文件' },
  { value: 'file.renamed', label: '重命名文件' },
  { value: 'item.repriced', label: '重新计价' },
  { value: 'item.qc_updated', label: '更新质检' },
  { value: 'settlement.confirmed', label: '确认结算' },
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
const pageSize = 50
const page = ref(1)
const appliedFilters = ref({ ...filters.value })
const visibleDate = ref('')
const eventDayFormatter = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' })
const eventsRequest = usePageRequest(
  (signal) =>
    assetWorkbenchApi.listEvents({
      page: page.value,
      page_size: pageSize,
      event_type: appliedFilters.value.event_type === 'all' ? undefined : appliedFilters.value.event_type,
      entity_type: appliedFilters.value.entity_type === 'all' ? undefined : appliedFilters.value.entity_type,
      entity_id: appliedFilters.value.entity_id || undefined,
      actor_user_id: appliedFilters.value.actor_user_id || undefined,
    }, signal),
  { items: [], total: 0 },
  '操作日志加载失败',
)
const rows = computed(() => (eventsRequest.data.value?.items ?? []).filter((row) => {
  if (!visibleDate.value) return true
  const date = new Date(row.created_at)
  return !Number.isNaN(date.getTime()) && eventDayFormatter.format(date) === visibleDate.value
}))
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const total = computed(() => eventsRequest.data.value?.total ?? 0)
const loading = eventsRequest.loading
const error = eventsRequest.error

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
  await eventsRequest.run()
}

function updateActor(value: string) {
  filters.value.actor_user_id = value
  selectedActor.value = null
}

function selectActor(person: PersonnelSuggestion) {
  filters.value.actor_user_id = person.name
  selectedActor.value = person
}

function applyFilters() {
  const text = filters.value.actor_user_id.trim()
  const id = selectedActor.value ? String(selectedActor.value.user_id) : text
  if (id && !/^[1-9]\d*$/.test(id)) {
    error.value = '请从候选中选择操作者，或输入账号编号。'
    return
  }
  appliedFilters.value = { ...filters.value, actor_user_id: id }
  page.value = 1
  void loadEvents()
}

function changePage(delta: number) {
  page.value = Math.min(pageCount.value, Math.max(1, page.value + delta))
  void loadEvents()
}

onMounted(() => {
  void loadEvents()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">设置</p>
        <h2>{{ pageLabel }}</h2>
        <p>{{ pageSubtitle }}。上传、质检、结算、资料维护等关键操作都会留下事件记录。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="loadEvents">刷新</button>
      </div>
    </div>

    <div class="aw-data-surface">
      <form class="aw-grid-toolbar" @submit.prevent="applyFilters">
        <select v-model="filters.event_type" aria-label="事件类型">
          <option v-for="option in eventFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="filters.entity_type" aria-label="对象类型">
          <option v-for="option in entityFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <PersonnelAutocomplete scope="audit_actor" label="操作者" placeholder="操作者姓名或编号"
          hide-label :model-value="filters.actor_user_id" :selected="selectedActor !== null"
          @update:model-value="updateActor" @selected="selectActor" />
        <input v-model.trim="filters.entity_id" aria-label="对象记录编号" placeholder="对象记录编号" inputmode="numeric" pattern="[1-9][0-9]*" />
        <button type="submit" :disabled="loading">筛选</button>
        <span>{{ formatInt(total) }} 条匹配日志</span>
      </form>
      <div class="aw-grid-toolbar">
        <label>筛选本页日期（北京时间） <input v-model="visibleDate" aria-label="筛选本页日期" type="date" /></label>
        <span v-if="visibleDate">本页 {{ rows.length }} 条符合日期</span>
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载操作日志"
        @retry="loadEvents"
      >
        <WorkbenchDataGrid
          v-if="rows.length"
          :columns="eventGridColumns"
          :rows="eventGridRows"
          row-key="id"
          storage-key="events"
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
      </AsyncBoundary>
      <nav v-if="pageCount > 1" class="aw-drive-pager" aria-label="操作记录分页">
        <button class="aw-grid-button" type="button" :disabled="loading || page <= 1" @click="changePage(-1)">上一页</button>
        <span>第 {{ page }} / {{ pageCount }} 页</span>
        <button class="aw-grid-button" type="button" :disabled="loading || page >= pageCount" @click="changePage(1)">下一页</button>
      </nav>
    </div>
  </section>
</template>
