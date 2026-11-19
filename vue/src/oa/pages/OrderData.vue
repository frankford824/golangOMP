<script setup lang="ts">
import { orderFilters, rememberOrderFilters } from '../orderFilterMemory'
import { computed, nextTick, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Copy, Search, X } from 'lucide-vue-next'
import { useRoute } from 'vue-router'
import { orderState } from '../ordersDisplay'
import OrderDateField from '../components/OrderDateField.vue'
import OperationNav from '../components/OperationNav.vue'
import type { HttpAppError } from '@/services/http'
import OAEmpty from '../components/OAEmpty.vue'
import PageHead from '../components/PageHead.vue'
import FreshnessStatus from '../components/FreshnessStatus.vue'
import { api, dateInChina, errorText, money } from '../api'
import { useOAAccess } from '../access'
import '../styles.css'

const { currentProfile, can } = useOAAccess()
const route = useRoute()

const props = defineProps<{ mode: 'detail' | 'refunds' | 'ledger' }>()
type Cell = string | boolean | number | null
type Row = Record<string, Cell>
type Column = { key: string; label: string; money?: boolean; date?: boolean }
const columns: Record<typeof props.mode, Column[]> = {
  detail: [
    { key: 'online_order_no', label: '平台订单号' }, { key: 'shop_id', label: '店铺' },
    { key: 'platform', label: '平台' }, { key: 'order_time', date: true, label: '下单时间' },
    { key: 'status', label: '状态' }, { key: 'payable_amount', label: '订单应付', money: true },
    { key: 'collected_amount', label: '实收金额', money: true }, { key: 'is_valid', label: '有效订单' }, { key: 'is_brush', label: '刷单标记' },
  ],
  refunds: [
    { key: 'day', date: true, label: '退款日' }, { key: 'shop_id', label: '店铺' },
    { key: 'refund_amount', label: '退款总额', money: true }, { key: 'valid_refund_amount', label: '有效退款', money: true },
    { key: 'unmatched_refund_amount', label: '未匹配退款', money: true }, { key: 'outlier_refund_amount', label: '异常退款', money: true },
  ],
  ledger: [
    { key: 'shop_id', label: '店铺' }, { key: 'period', date: true, label: '月份' }, { key: 'state', label: '核算状态' }, { key: 'name', label: '科目' },
    { key: 'value', label: '金额', money: true }, { key: 'computed_at', date: true, label: '核算时间' },
  ],
}
const detailColumns: Record<typeof props.mode, Column[]> = {
  detail: [...columns.detail, { key: 'order_id', label: '订单 ID' }],
  refunds: columns.refunds,
  ledger: [...columns.ledger, { key: 'line_id', label: '科目 ID' }],
}
const today = dateInChina()
const pageSize = 100
const firstDay = '2026-06-01'
const from = ref(today)
const to = ref(today)
const shop = ref('')
const platform = ref('')
const orderNo = ref('')
const includeBrush = ref(true)
const shopError = ref('')
const offset = ref(0)
const applied = ref('')
type OrderData = { rows: Row[]; has_more?: boolean; scope_state?: 'unassigned'; stale?: boolean; freshness?: { source_committed_at?: string; lag_seconds?: number } }
const data = ref<OrderData>()
const shopNames = ref<{ shop_id: string; shop_name: string }[]>([])
const busy = ref(false)
const error = ref('')
const failedRequest = ref<{ offset: number; query: string }>()
const scope = computed(() => currentProfile.value?.id)
const allowed = computed(() => scope.value != null && can('orders.view') && (props.mode !== 'ledger' || !!currentProfile.value?.super_admin))
const copyStatus = ref('')
const selected = ref<Row>()
const dialog = ref<HTMLDialogElement>()
const resultHeader = ref<HTMLElement>()
const visibleRows = computed(() => {
  const rows = data.value?.rows ?? []
  return props.mode === 'refunds' ? rows.slice(offset.value, offset.value + pageSize) : rows
})
const hasNext = computed(() => props.mode === 'refunds'
  ? offset.value + pageSize < (data.value?.rows?.length ?? 0)
  : !!data.value?.has_more)
const page = computed(() => Math.floor(offset.value / pageSize) + 1)
const title = computed(() => ({ detail: '订单明细', refunds: '售后退款', ledger: '经营台账' }[props.mode]))
const subtitle = computed(() => ({
  detail: '按下单时间倒序',
  refunds: '按确认售后的申请日期汇总',
  ledger: '按店铺、核算月份与核算状态展示',
}[props.mode]))
const detailTitle = computed(() => ({ detail: '订单详情', refunds: '退款汇总详情', ledger: '台账科目详情' }[props.mode]))
const mobileColumns: Record<typeof props.mode, string[]> = {
  detail: ['online_order_no', 'collected_amount'],
  refunds: ['day', 'shop_id', 'refund_amount'],
  ledger: ['period', 'name', 'value'],
}
let requestId = 0
let scopeId = 0

function filterQuery() {
  const params = new URLSearchParams({ from: from.value, to: to.value, shop_id: shop.value, platform: platform.value })
  if (props.mode === 'detail') params.set('include_brush', includeBrush.value ? '1' : '0')
  if (props.mode === 'detail' && orderNo.value) params.set('online_order_no', orderNo.value)
  return params.toString()
}

function resetFilters() {
  from.value = props.mode === 'ledger' ? today.slice(0, 7) + '-01' : today
  to.value = today
  shop.value = ''
  platform.value = ''
  orderNo.value = ''
  includeBrush.value = true
}

function restoreRouteFilters() {
  const saved = orderFilters('orders-' + props.mode, route.query)
  const { from: start, to: end, shop_id: shopId, platform: platformCode } = saved
  const validDate = (value: unknown): value is string => typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value) && value >= firstDay && value <= today && new Date(value).toJSON()?.slice(0, 10) === value
  if (validDate(start) && validDate(end) && start <= end) {
    from.value = start
    to.value = end
  }
  shop.value = typeof shopId === 'string' ? shopId : ''
  orderNo.value = props.mode === 'detail' && typeof saved.online_order_no === 'string' ? saved.online_order_no : ''
  platform.value = typeof platformCode === 'string' ? platformCode : ''
  includeBrush.value = saved.include_brush !== '0'
}

const navigationQuery = computed(() => {
  const params = new URLSearchParams(applied.value)
  const query: Record<string, string> = {}
  for (const key of ['from', 'to', 'shop_id', 'platform', 'include_brush']) {
    const value = params.get(key)
    if (value) query[key] = value
  }
  if (!query.include_brush && typeof route.query.include_brush === 'string') query.include_brush = route.query.include_brush
  return query
})

function resetAndLoad() {
  resetFilters()
  void load()
}

async function load(next = 0, filters = filterQuery()) {
  if (!allowed.value || busy.value) return
  const currentRequest = ++requestId
  const mode = props.mode
  busy.value = true
  error.value = ''
  failedRequest.value = undefined
  try {
    const query = new URLSearchParams(filters)
    query.set('offset', String(next))
    const result = await api<OrderData>('/oa-api/orders/' + mode + '?' + query)
    if (requestId !== currentRequest) return
    data.value = result
    offset.value = next
    applied.value = filters
    const active = Object.fromEntries(new URLSearchParams(filters))
    const specific: Record<string, string> = {}
    if (active.platform) specific.platform = active.platform
    if (active.online_order_no) specific.online_order_no = active.online_order_no
    rememberOrderFilters('orders-' + props.mode, active, specific)
  } catch (cause) {
    if (requestId !== currentRequest) return
    const failure = cause as HttpAppError
    if (failure.status === 401 || failure.status === 403 || filters !== applied.value) {
      closeDetails()
      data.value = undefined
      applied.value = ''
      offset.value = 0
    }
    error.value = errorText(cause)
    failedRequest.value = { offset: next, query: filters }
  } finally {
    if (requestId === currentRequest) busy.value = false
  }
}

async function changePage(next: number) {
  if (props.mode === 'refunds') offset.value = next
  else await load(next, applied.value)
  if (!error.value) resultHeader.value?.focus()
}

async function openDetails(row: Row) {
  selected.value = row
  copyStatus.value = ''
  await nextTick()
  if (selected.value && !dialog.value?.open) dialog.value?.showModal()
}

function clearDetails() {
  if (!dialog.value?.open) {
    selected.value = undefined
    copyStatus.value = ''
  }
}

function closeDetails() {
  dialog.value?.close()
  clearDetails()
}

async function copyOrder(id: string) {
  try { await navigator.clipboard.writeText(id); copyStatus.value = '已复制订单号' }
  catch { copyStatus.value = '无法复制订单号，请手动复制' }
}

const appliedRange = computed(() => {
  const params = new URLSearchParams(applied.value)
  const start = params.get('from')
  const end = params.get('to')
  const shopId = params.get('shop_id')
  return [
    start === end ? start : [start, end].filter(Boolean).join(' 至 '),
    shopId && (shopNames.value.find(item => item.shop_id === shopId)?.shop_name || shopId),
    params.get('platform'), params.get('online_order_no'),
  ].filter(Boolean).join(' · ')
})
function display(column: Column, row: Row) {
  const value = row[column.key]
  if (value == null || value === '') return '—'
  if (column.key === 'shop_id') return shopNames.value.find(item => item.shop_id === value)?.shop_name || value
  if (column.date && typeof value === 'string' && /^\d{4}-\d{2}-\d{2}/.test(value)) {
    const instant = /(?:Z|[+-]\d{2}:?\d{2})$/.test(value) ? new Date(value) : undefined
    if (instant && !Number.isNaN(instant.getTime())) return new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(instant)
    return value.replace('T', ' ').slice(0, 16)
  }
  if (column.key === 'status' || column.key === 'state') return orderState(column.key, value)?.label ?? '—'
  if (column.money) return money(String(value))
  if (column.key.startsWith('is_')) return value === true || value === 1 || value === '1' ? '是' : '否'
  return value
}
function stateClass(column: Column, row: Row) {
  const tone = orderState(column.key, row[column.key])?.tone
  return tone ? ['oa-pill', 'order-state-' + tone] : undefined
}
watch([scope, allowed], async () => {
  const currentScope = ++scopeId
  shopNames.value = []
  shopError.value = ''
  if (!allowed.value) return
  try {
    const result = await api<{ rows: { shop_id: string; shop_name: string }[] }>('/oa-api/orders/shops')
    if (scopeId === currentScope) shopNames.value = result.rows || []
  } catch {
    if (scopeId === currentScope) shopError.value = '店铺列表暂不可用，可手动输入店铺 ID。'
  }
}, { immediate: true, flush: 'sync' })

watch([() => props.mode, () => route.query, scope, allowed], () => {
  ++requestId
  closeDetails()
  busy.value = false
  data.value = undefined
  error.value = ''
  failedRequest.value = undefined
  applied.value = ''
  offset.value = 0
  resetFilters()
  restoreRouteFilters()
  void load()
}, { immediate: true })
</script>

<template>
  <div class="oa-page order-data">
    <PageHead :title="title" :subtitle="subtitle" />
    <OperationNav :query="navigationQuery" />

    <div class="order-data-body" role="region" :aria-label="title + '工作区'">
    <p v-if="!allowed" class="oa-error" role="alert">当前账号没有{{ title }}权限。</p>
    <template v-else>
      <form class="order-data-filters" @submit.prevent="load()">
        <label>
          店铺
          <select v-if="shopNames.length" v-model="shop">
            <option value="">全部可见店铺</option>
            <option v-if="shop && !shopNames.some(item => item.shop_id === shop)" :value="shop">{{ shop }}</option>
            <option v-for="item in shopNames" :value="item.shop_id" :key="item.shop_id">{{ item.shop_name || item.shop_id }}</option>
          </select>
          <input v-else v-model.trim="shop" placeholder="全部可见店铺" />
        </label>
        <label v-if="mode === 'detail'">平台订单号<input v-model.trim="orderNo" maxlength="128" placeholder="精确查单" /></label>
        <label>平台<input v-model.trim="platform" placeholder="默认平台范围" /></label>
        <label>日期<OrderDateField v-model="from" label="开始日期" :max="to" :min="firstDay" /></label>
        <label>至<OrderDateField v-model="to" label="结束日期" :min="from" :max="today" /></label>
        <label v-if="mode === 'detail'" class="order-data-check"><input v-model="includeBrush" type="checkbox" />包含刷单</label>
        <div class="order-data-filter-actions">
          <button class="oa-button-primary" type="submit" :disabled="busy"><Search :size="16" aria-hidden="true" />查询</button>
          <button type="button" :disabled="busy" @click="resetAndLoad">重置</button>
        </div>
      </form>

      <p v-if="shopError" class="oa-watermark">{{ shopError }}</p>
      <p v-if="error" class="oa-error order-data-error" role="alert">
        <span>{{ error }}</span>
        <button v-if="failedRequest" type="button" @click="load(failedRequest.offset, failedRequest.query)">重试</button>
      </p>
      <p v-if="busy" class="order-data-loading oa-muted" role="status">正在读取{{ title }}…</p>
      <section v-if="data" class="order-data-results" :aria-busy="busy" aria-labelledby="order-data-results">
        <header class="order-data-results-head">
          <h2 id="order-data-results" ref="resultHeader" tabindex="-1">查询结果</h2>
          <p class="oa-muted">{{ appliedRange }}</p>
          <FreshnessStatus :freshness="data.freshness" :stale="data.stale" />
          <p v-if="copyStatus && !selected" class="oa-muted" role="status">{{ copyStatus }}</p>
        </header>
        <OAEmpty v-if="data.scope_state === 'unassigned'" title="暂无经营归属" description="当前账号没有分配可访问店铺。" kind="list" />
        <OAEmpty v-else-if="!visibleRows.length" :title="'所选范围暂无' + title + '数据'" kind="search" />
        <div v-else class="oa-table-wrap" tabindex="0" role="region" :aria-label="title + '数据表'">
          <table class="oa-table order-data-table" :class="`order-data-table-${mode}`">
            <thead>
              <tr>
                <th v-for="column in columns[mode]" :key="column.key" scope="col" :data-column="column.key" :class="{ 'oa-number': column.money, 'order-data-wide': !mobileColumns[mode].includes(column.key) }">{{ column.label }}</th>
                <th class="oa-action-col" scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, index) in visibleRows" :key="index" :class="{ 'is-selected': selected === row }"
                @click="!($event.target as HTMLElement).closest('a,button') && openDetails(row)">
                <td v-for="column in columns[mode]" :key="column.key" :data-column="column.key" :class="{ 'oa-number': column.money, 'order-data-wide': !mobileColumns[mode].includes(column.key) }">
                  <time v-if="column.date && row[column.key]" :datetime="String(row[column.key])">{{ display(column, row) }}</time>
                  <span v-else-if="column.key === 'status' || column.key === 'state'" :class="stateClass(column, row)" :title="row[column.key] == null ? undefined : String(row[column.key])">{{ display(column, row) }}</span>
                  <span v-else :class="{ 'oa-ellipsis': ['online_order_no', 'shop_id', 'name'].includes(column.key), 'order-data-order-no': column.key === 'online_order_no' }" :title="String(display(column, row))">{{ display(column, row) }}</span>
                  <button v-if="column.key === 'online_order_no' && row[column.key]" class="order-data-copy order-data-inline-copy" type="button" aria-label="复制平台订单号" @click="copyOrder(String(row.online_order_no))"><Copy :size="14" aria-hidden="true" /></button>
                </td>
                <td class="oa-action-col"><button class="order-data-view" type="button" aria-haspopup="dialog" @click="openDetails(row)">详情</button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="offset || visibleRows.length || hasNext" class="order-data-pager" aria-label="列表分页">
          <span class="oa-muted">第 {{ page }} 页<span v-if="visibleRows.length"> · 本页 {{ visibleRows.length }} 条</span></span>
          <div>
            <button type="button" :disabled="busy || !offset" @click="changePage(Math.max(0, offset - pageSize))"><ChevronLeft :size="16" aria-hidden="true" />上一页</button>
            <button type="button" :disabled="busy || !hasNext || (mode !== 'refunds' && offset >= 10000)" @click="changePage(offset + pageSize)">下一页<ChevronRight :size="16" aria-hidden="true" /></button>
          </div>
        </div>
      </section>

      <dialog ref="dialog" class="oa-dialog order-data-dialog" aria-labelledby="order-data-detail" @close="clearDetails">
        <header>
          <h2 id="order-data-detail">{{ detailTitle }}</h2>
          <button autofocus type="button" aria-label="关闭详情" @click="closeDetails"><X :size="18" aria-hidden="true" /></button>
        </header>
        <dl v-if="selected" class="order-data-fields">
          <div v-for="column in detailColumns[mode]" :key="column.key">
            <dt>{{ column.label }}</dt>
            <dd>
              <time v-if="column.date && selected[column.key]" :datetime="String(selected[column.key])">{{ display(column, selected) }}</time>
              <span v-else-if="column.key === 'status' || column.key === 'state'" :class="stateClass(column, selected)" :title="selected[column.key] == null ? undefined : String(selected[column.key])">{{ display(column, selected) }}</span>
              <template v-else>{{ display(column, selected) }}</template>
              <button v-if="column.key === 'online_order_no' && selected[column.key]" class="order-data-copy" type="button" @click="copyOrder(String(selected.online_order_no))"><Copy :size="14" aria-hidden="true" />复制</button>
            </dd>
          </div>
        </dl>
        <p v-if="copyStatus" class="oa-muted order-data-copy-status" role="status">{{ copyStatus }}</p>
      </dialog>
    </template>
    </div>
  </div>
</template>

<style scoped src="./order-data.css" />