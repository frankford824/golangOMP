<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, Boxes, Library, RefreshCw, Search } from 'lucide-vue-next'

import { assetWorkbenchApi, type OverviewSearchRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { chipClass } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

type OverviewGridRow = OverviewSearchRow & {
  source_id: string
  source_label: string
  created_label: string
  amount_label: string
  page_count_label: string
}

const router = useRouter()
const keyword = ref('')
const creator = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const page = ref(1)
const pageSize = ref(30)

const overviewRequest = usePageRequest(
  (signal) =>
    assetWorkbenchApi.overviewSearch({
      q: keyword.value.trim() || undefined,
      creator: creator.value.trim() || undefined,
      date_from: dateFrom.value || undefined,
      date_to: dateTo.value || undefined,
      page: page.value,
      page_size: pageSize.value,
    }, signal),
  { items: [], total: 0, page: 1, size: pageSize.value },
  '总盘查询失败',
)
const loading = overviewRequest.loading
const error = overviewRequest.error
const overviewData = computed(() => overviewRequest.data.value ?? { items: [], total: 0, page: 1, size: pageSize.value })
const rows = computed(() => overviewData.value.items)
const total = computed(() => overviewData.value.total)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const gridRows = computed<OverviewGridRow[]>(
  () =>
    rows.value.map((row) => ({
      ...row,
      source_id: `${row.source}:${row.id}`,
      source_label: sourceMeta(row.source).label,
      created_label: formatDateTime(row.created_at),
      amount_label: row.amount ? formatMoney(row.amount) : '—',
      page_count_label: row.page_count ? formatInt(row.page_count) : '—',
    })),
)
const dataRows = computed(() => gridRows.value as unknown as Record<string, unknown>[])
const columns = [
  { key: 'source_label', label: '类型', width: 112 },
  { key: 'primary_code', label: '编码 / 批次', width: 170 },
  { key: 'order_no', label: '订单号', width: 150 },
  { key: 'title', label: '名称', width: 260 },
  { key: 'secondary_code', label: '款式 / 类型', width: 150 },
  { key: 'creator_name', label: '创建人', width: 132 },
  { key: 'created_label', label: '创建时间', width: 176 },
  { key: 'business_month', label: '业务月', width: 104 },
  { key: 'page_count_label', label: '页数', width: 88, align: 'right' as const },
  { key: 'amount_label', label: '金额', width: 112, align: 'right' as const },
  { key: 'status', label: '状态', width: 132 },
  { key: 'actions', label: '动作', width: 104, align: 'center' as const },
]

function sourceMeta(source: string) {
  if (source === 'system_asset') return { label: '素材', tone: 'info' as const, icon: Library }
  if (source === 'submission') return { label: '提交', tone: 'warn' as const, icon: Boxes }
  if (source === 'piecework_item') return { label: '计件', tone: 'success' as const, icon: Boxes }
  return { label: source || '未知', tone: 'neutral' as const, icon: Boxes }
}

function gridRowAsOverview(row: Record<string, unknown>): OverviewGridRow {
  return row as unknown as OverviewGridRow
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

async function loadOverview(resetPage = false) {
  if (resetPage) page.value = 1
  await overviewRequest.run()
}

async function goPage(nextPage: number) {
  page.value = Math.min(totalPages.value, Math.max(1, nextPage))
  await loadOverview(false)
}

function openRow(row: OverviewSearchRow) {
  if (row.route_path) {
    void router.push(row.route_path)
  }
}

onMounted(() => {
  void loadOverview()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">总盘查询</p>
        <h2>素材、成品与计件统一检索</h2>
        <p>按编码、订单号、创建人和日期快速定位资产工作台记录。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadOverview(false)">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
      </div>
    </div>

    <div class="aw-panel">
      <div class="aw-form-grid">
        <label>
          编码 / 订单号
          <input v-model="keyword" placeholder="素材编码、订单号、文件名" @keyup.enter="loadOverview(true)" />
        </label>
        <label>
          创建人
          <input v-model="creator" placeholder="姓名或账号" @keyup.enter="loadOverview(true)" />
        </label>
        <label>
          开始日期
          <input v-model="dateFrom" type="date" />
        </label>
        <label>
          结束日期
          <input v-model="dateTo" type="date" />
        </label>
      </div>
      <div class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="loading" @click="loadOverview(true)">
          <Search :size="16" aria-hidden="true" />
          查询
        </button>
      </div>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ formatInt(total) }} 条结果</span>
        <span>第 {{ formatInt(page) }} / {{ formatInt(totalPages) }} 页</span>
        <button type="button" :disabled="page <= 1 || loading" @click="goPage(page - 1)">上一页</button>
        <button type="button" :disabled="page >= totalPages || loading" @click="goPage(page + 1)">下一页</button>
      </div>
      <AsyncBoundary :loading="loading" :error="error" loading-label="正在查询总盘" @retry="loadOverview(false)">
        <WorkbenchDataGrid
          v-if="rows.length"
          :columns="columns"
          :rows="dataRows"
          row-key="source_id"
          storage-key="overview-search"
          :height="560"
          :row-clickable="true"
          @row-click="openRow(gridRowAsOverview($event))"
        >
          <template #cell="{ row, column, value }">
            <span v-if="column.key === 'source_label'" :class="chipClass(sourceMeta(gridRowAsOverview(row).source).tone)">
              <component :is="sourceMeta(gridRowAsOverview(row).source).icon" :size="14" aria-hidden="true" />
              {{ value }}
            </span>
            <span v-else-if="column.key === 'amount_label'" class="aw-cell-money">{{ value }}</span>
            <span v-else-if="column.key === 'page_count_label'" class="aw-cell-num">{{ value }}</span>
            <button v-else-if="column.key === 'actions'" class="aw-icon-button" type="button" @click.stop="openRow(gridRowAsOverview(row))">
              <ArrowRight :size="16" aria-hidden="true" />
            </button>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>没有匹配记录</h3>
          <p>调整编码、订单号、创建人或日期后再查。</p>
        </div>
      </AsyncBoundary>
    </div>
  </section>
</template>
