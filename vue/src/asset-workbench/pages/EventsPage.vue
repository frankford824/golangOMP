<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchEventRow } from '@aw/shared/api/assetWorkbenchApi'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

const rows = ref<AssetWorkbenchEventRow[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const filters = ref({
  event_type: '',
  entity_type: '',
})
const eventGridRows = computed(() => rows.value as unknown as Record<string, unknown>[])
const eventGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'event_type', label: '事件', width: 180 },
  { key: 'entity_type', label: '对象', width: 148 },
  { key: 'entity_id', label: '对象 ID', width: 100, align: 'right' },
  { key: 'actor_user_id', label: '操作者', width: 100, align: 'right' },
  { key: 'reason', label: '原因', width: 220 },
  { key: 'created_at', label: '时间', width: 190 },
])

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
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">操作留痕</p>
        <h2>操作日志</h2>
      </div>
      <button class="aw-secondary-button" type="button" @click="loadEvents">刷新</button>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <input v-model="filters.event_type" placeholder="事件类型" />
        <input v-model="filters.entity_type" placeholder="对象类型" />
        <button type="button" @click="loadEvents">筛选</button>
      </div>
      <p v-if="loading" class="aw-copy">正在加载操作日志</p>
      <p v-else-if="error" class="aw-copy">{{ error }}</p>
      <WorkbenchDataGrid
        v-else-if="rows.length"
        :columns="eventGridColumns"
        :rows="eventGridRows"
        row-key="id"
        storage-key="events"
        group-by="event_type"
      >
        <template #cell="{ column, value }">
          <span v-if="column.key === 'actor_user_id'">{{ value || 'system' }}</span>
          <span v-else-if="column.key === 'entity_id'">{{ value || '—' }}</span>
          <strong v-else-if="column.key === 'created_at'">{{ value }}</strong>
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
