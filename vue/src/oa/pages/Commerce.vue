<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight } from 'lucide-vue-next'
import PageHead from '../components/PageHead.vue'
import FreshnessStatus from '../components/FreshnessStatus.vue'
import { getToken, TOKEN_STORAGE_KEY, type HttpAppError } from '@/services/http'
import { api, dateInChina, errorText, money, send } from '../api'
import { useOAAccess } from '../access'
import { usePermissionsStore } from '@/stores/permissions'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const permissions = usePermissionsStore()

type Mode = 'overview' | 'products' | 'detail' | 'ranking' | 'personal'
const props = defineProps<{ mode: Mode; productId?: string }>()
type DataRow = {
  day?: string; shop_id?: string; platform?: string; shop_item_id?: string; sku?: string; product_name?: string
  quantity?: string; order_count?: number; payable_amount?: string; item_pay_total?: string
  cost_verified_total?: string | null; item_pay_verified_total?: string | null; verified_gross?: string | null; verified_coverage?: string | null
  key?: string; tasks?: { id?: number; task_id?: number; task_no?: string; title?: string }[]
  feedback?: { id: number; note?: string; live_date?: string; task_id?: number }[]
  shop_name?: string; growth_status?: string; growth_rate?: number
}
type TaskLink = { id: number; task_no: string; status: string }
type Feedback = { id: number; shop_id: string; note: string; adjusted_on: string; task_id?: number }
type OrderResult = { rows: DataRow[]; freshness?: { source_committed_at?: string; lag_seconds?: number }; stale?: boolean; has_more?: boolean; current_cost?: string | null; cost_context_available?: boolean; tasks?: TaskLink[]; feedback?: Feedback[]; forecast?: string | null; month_actual?: string | null }

const today = dateInChina()
const from = ref(today)
const to = ref(today)
const shop = ref('')
const selectedSku = ref(props.productId ?? '')
const group = ref('shop')
const overviewSection = ref<'products' | 'trend'>('products')
const appliedGroup = ref('shop')
let appliedQuery = ''
const includeBrush = ref(false)
const loadedDetail = ref<{ shop: string; sku: string; from: string; to: string }>()
const shopOptions = ref<DataRow[]>([])
const shopError = ref('')
const productsError = ref('')
const history = ref<Record<string, unknown>[]>([])
const historyError = ref('')
const historyOpen = ref(false)
const feedback = ref({ shop_id: '', adjusted_on: today, note: '', task_id: undefined as number | undefined })
const saved = ref('')
const feedbackError = ref('')
const copyStatus = ref('')
async function copySKU(sku: string) {
  try { await navigator.clipboard.writeText(sku); copyStatus.value = '已复制 SKU' }
  catch { copyStatus.value = '无法复制 SKU，请手动复制' }
}
const comparison = ref<(DataRow & { phase?: string })[]>()
const comparisonError = ref('')
const comparisonBusy = ref(false)
const offset = ref(0)
const data = ref<OrderResult>()
const summary = ref<OrderResult>()
const busy = ref(false)
const error = ref('')
const latest = ref(0)
const locallyStale = ref(false)
let appliedOverview: { query: string; account: string; token: string } | undefined
let refreshTimer: number | undefined
const mayView = computed(() => can('orders.view'))
const title = computed(() => ({ overview: '我的经营', products: '商品经营', detail: '商品详情与经营反馈', ranking: '经营分析与排名', personal: '我的业绩' }[props.mode]))

function queryParams(endpoint: string, options: Record<string, string> = {}) {
  const query = new URLSearchParams({ from: from.value, to: to.value, ...options })
  if (includeBrush.value && options.group !== 'growth' && endpoint !== 'shops') query.set('include_brush', '1')
  if (shop.value.trim()) query.set('shop_id', shop.value.trim())
  return query.toString()
}
function withQuery(endpoint: string, options: Record<string, string> = {}) {
  return api<OrderResult>(`/oa-api/orders/${endpoint}?${queryParams(endpoint, options)}`)
}

function clearOverview() {
  data.value = undefined
  summary.value = undefined
  locallyStale.value = false
}

async function loadOverview(refresh = false) {
  const account = currentProfile.value?.id
  const token = getToken()
  if (appliedOverview && (appliedOverview.account !== account || appliedOverview.token !== token)) resetOverviewSession()
  if (!mayView.value || !account || !token) return
  const scope = refresh ? appliedOverview : { account, token, query: queryParams('summary') }
  if (!scope || scope.account !== account || scope.token !== token) return
  const sequence = ++latest.value
  const sameScope = appliedOverview?.query === scope.query && appliedOverview.account === account && appliedOverview.token === token
  if (!sameScope) clearOverview()
  appliedOverview = scope
  busy.value = true
  productsError.value = ''
  error.value = ''
  try {
    const [head, tail] = await Promise.allSettled(['summary', 'products'].map(endpoint =>
      api<OrderResult>(`/oa-api/orders/${endpoint}?${scope.query}`).catch((failure: HttpAppError) => {
        const raw = failure.responseData as { error?: string } | undefined
        if (sequence === latest.value && (failure.status === 401 || failure.status === 403 || raw?.error === '订单数据源拒绝访问')) {
          clearOverview()
          appliedOverview = undefined
          error.value = errorText(failure)
        }
        throw failure
      }),
    ))
    if (sequence !== latest.value) return
    if (!appliedOverview) return
    if (scope.account !== currentProfile.value?.id || scope.token !== getToken() || !mayView.value) {
      clearOverview()
      appliedOverview = undefined
      return
    }
    locallyStale.value = sameScope && (
      (head.status === 'rejected' && !!summary.value) || (tail.status === 'rejected' && !!data.value)
    )
    if (head.status === 'fulfilled') summary.value = head.value
    else if (!summary.value) error.value = errorText(head.reason)
    if (tail.status === 'fulfilled') data.value = tail.value
    else if (!data.value) productsError.value = errorText(tail.reason)
  } finally {
    if (sequence === latest.value) busy.value = false
  }
}

async function load(resetPage = true) {
  if (props.mode === 'overview') return loadOverview()
  if (resetPage) offset.value = 0
  if (!mayView.value) return
  if (props.mode === 'detail' && !selectedSku.value.trim()) return
  const sequence = ++latest.value
  const detailFilters = !resetPage && loadedDetail.value ? loadedDetail.value : { shop: shop.value, sku: selectedSku.value.trim(), from: from.value, to: to.value }
  busy.value = true
  loadedDetail.value = undefined
  historyOpen.value = false
  productsError.value = ''
  data.value = undefined
  summary.value = undefined
  error.value = ''
  try {
    const endpoint = { products: 'products', ranking: 'rankings', detail: 'sku', personal: 'personal' }[props.mode]
    const options: Record<string, string> = {}
    if (props.mode === 'ranking') options.group = group.value
    if (props.mode === 'products') options.mine = '1'
    if (offset.value) options.offset = String(offset.value)
    if (props.mode === 'detail') options.sku = selectedSku.value.trim()
    if (resetPage || !appliedQuery) {
      appliedGroup.value = group.value
      appliedQuery = queryParams(endpoint, options)
    }
    const applied = new URLSearchParams(appliedQuery)
    applied.set('offset', String(offset.value))
    const result = await api<OrderResult>(`/oa-api/orders/${endpoint}?${applied}`)
    if (sequence === latest.value) {
      data.value = result
      if (props.mode === 'detail') {
        loadedDetail.value = detailFilters
        feedback.value.shop_id = detailFilters.shop || result.rows?.[0]?.shop_id || ''
      }
    }
  } catch (cause) { if (sequence === latest.value) error.value = errorText(cause) }
  finally { if (sequence === latest.value) busy.value = false }
}

type MoneyKey = 'payable_amount' | 'item_pay_total' | 'item_pay_verified_total' | 'verified_gross'
function sum(rows: DataRow[], key: MoneyKey): string | null {
  const values = rows.map(row => row[key]).filter((value): value is string => value != null)
  if (!values.length) return null
  const numbers = values.map(value => /^(-?)(\d+)(?:\.(\d+))?$/.exec(String(value)))
  if (numbers.some(value => !value)) return null
  const precision = Math.max(0, ...numbers.map(value => value?.[3]?.length ?? 0))
  const unit = 10n ** BigInt(precision)
  const total = numbers.reduce((result, value) => {
    if (!value) return result
    const integer = BigInt(value[2]) * unit
    const fraction = BigInt((value[3] ?? '').padEnd(precision, '0') || '0')
    return result + (integer + fraction) * (value[1] ? -1n : 1n)
  }, 0n)
  const absolute = total < 0n ? -total : total
  const fraction = precision ? '.' + (absolute % unit).toString().padStart(precision, '0') : ''
  return `${total < 0n ? '-' : ''}${absolute / unit}${fraction}`
}

const summaryRows = computed(() => summary.value?.rows ?? [])
const rows = computed(() => data.value?.rows ?? [])
const linkedTask = computed(() => {
  const filters = loadedDetail.value
  if (!filters || !rows.value.length || data.value?.stale) return ''
  const shop_id = filters.shop || (new Set(rows.value.map(row => row.shop_id)).size === 1 ? rows.value[0]?.shop_id : '')
  if (!shop_id) return ''
  const query = new URLSearchParams({ intent: 'retouch', oa_shop: shop_id, oa_sku: filters.sku, oa_from: filters.from, oa_to: filters.to })
  return '/tasks/create?' + query
})
const freshness = computed(() => {
  const available = [summary.value?.freshness, data.value?.freshness].filter(value => value !== undefined)
  if (!available.length) return undefined
  const watermarks = available.flatMap(item => item.source_committed_at ? [item.source_committed_at] : []).sort()
  const delays = available.flatMap(item => Number.isFinite(Number(item.lag_seconds)) ? [Number(item.lag_seconds)] : [])
  return { source_committed_at: watermarks[0], lag_seconds: delays.length ? Math.max(...delays) : undefined }
})
const cached = computed(() => locallyStale.value || summary.value?.stale || data.value?.stale)
const stale = computed(() => cached.value || Number(freshness.value?.lag_seconds) > 600)
const coverage = computed(() => {
  const payable = Number(sum(summaryRows.value, 'item_pay_total'))
  const verified = sum(summaryRows.value, 'item_pay_verified_total')
  return payable > 0 && verified !== null ? (100 * Number(verified) / payable).toFixed(1) + '%' : null
})
const kpis = computed(() => [
  { name: '订单销售额 · 应付', value: money(sum(summaryRows.value, 'payable_amount')) },
  { name: '有效订单', value: summaryRows.value.length ? summaryRows.value.reduce((result, row) => result + Number(row.order_count || 0), 0).toLocaleString('zh-CN') : '—' },
  { name: '商品销售额', value: money(sum(summaryRows.value, 'item_pay_total')) },
  { name: '核实毛利', value: money(sum(summaryRows.value, 'verified_gross')), foot: coverage.value },
])
const days = computed(() => [...new Set(summaryRows.value.map(row => row.day).filter((day): day is string => Boolean(day)))].sort().map(day => ({ day, value: Number(sum(summaryRows.value.filter(row => row.day === day), 'payable_amount')) })))
const max = computed(() => Math.max(1, ...days.value.map(day => day.value)))
const points = computed(() => days.value.map((day, index) => ({
  ...day, x: 45 + index / Math.max(1, days.value.length - 1) * 535,
  y: 125 - Math.max(0, day.value) / max.value * 95,
})))
const line = computed(() => points.value.map(point => `${point.x},${point.y}`).join(' '))
const area = computed(() => points.value.length ? `${points.value[0].x},125 ${line.value} ${points.value[points.value.length - 1].x},125` : '')

function range(daysBack: number) {
  const date = new Date(today + 'T12:00:00Z')
  date.setUTCDate(date.getUTCDate() - daysBack)
  from.value = date.toISOString().slice(0, 10)
  to.value = today
  void load()
}

async function loadShops() {
  try { shopOptions.value = (await withQuery('shops')).rows || [] }
  catch { shopError.value = '店铺列表暂不可用，可手动输入店铺 ID。' }
}
async function saveFeedback() {
  const form = feedback.value
  const filters = loadedDetail.value
  if (!filters) return
  busy.value = true
  feedbackError.value = ''
  saved.value = ''
  try {
    await send('/oa-api/orders/feedback', { ...form, sku: filters.sku })
    await load()
    saved.value = '上线记录已保存'
    form.note = ''
  } catch (cause) { feedbackError.value = errorText(cause) }
  finally { busy.value = false }
}
async function compare(event: Feedback) {
  const filters = loadedDetail.value
  if (!filters) return
  comparisonBusy.value = true
  comparisonError.value = ''
  comparison.value = undefined
  try {
    const query = new URLSearchParams({ shop_id: event.shop_id, sku: filters.sku, adjusted_on: event.adjusted_on, days: '7' })
    comparison.value = (await api<OrderResult>('/oa-api/orders/compare?' + query)).rows
  } catch (cause) { comparisonError.value = errorText(cause) }
  finally { comparisonBusy.value = false }
}
async function loadHistory() {
  const filters = loadedDetail.value
  if (!filters) return
  history.value = []
  historyError.value = ''
  historyOpen.value = true
  try {
    const query = new URLSearchParams({ from: filters.from, to: filters.to, sku: filters.sku })
    if (filters.shop) query.set('shop_id', filters.shop)
    history.value = (await api<{ rows: Record<string, unknown>[] }>('/oa-api/orders/cost-history?' + query)).rows
  } catch (cause) { historyError.value = errorText(cause) }
}
const costTierLabels: Record<string, string> = { verified: '核实', reference: '参考', none: '未知' }
const implausibleCost = (value: unknown) => value === true || value === 1 || value === '1'
const shopName = (id?: string) => shopOptions.value.find(item => item.shop_id === id)?.shop_name || id || '—'
function resetRefreshTimer() {
  window.clearInterval(refreshTimer)
  if (document.visibilityState !== 'visible') return
  refreshTimer = window.setInterval(() => {
    if (!busy.value && document.visibilityState === 'visible') void loadOverview(true)
  }, 30_000)
}

function resetOverviewSession() {
  latest.value++
  appliedOverview = undefined
  busy.value = false
  clearOverview()
}
function onSessionStorage(event: StorageEvent) {
  if (event.key === TOKEN_STORAGE_KEY || event.key === null) resetOverviewSession()
}
watch(() => [currentProfile.value?.id, mayView.value], () => {
  if (props.mode !== 'overview') return
  resetOverviewSession()
  void loadOverview()
})

onMounted(() => {
  if (props.mode === 'overview') {
    document.addEventListener('visibilitychange', resetRefreshTimer)
    window.addEventListener('storage', onSessionStorage)
    resetRefreshTimer()
  }
  if (mayView.value && props.mode !== 'personal') void loadShops()
  if (props.mode === 'personal') {
    from.value = today.slice(0, 7) + '-01'
  } else if (props.mode === 'detail') {
    const start = new Date(today + 'T12:00:00Z')
    start.setUTCDate(start.getUTCDate() - 6)
    from.value = start.toISOString().slice(0, 10)
  }
  void load()
})

onUnmounted(() => {
  latest.value++
  document.removeEventListener('visibilitychange', resetRefreshTimer)
  window.removeEventListener('storage', onSessionStorage)
  window.clearInterval(refreshTimer)
})
</script>

<template>
<div class="oa-page commerce-workspace">
  <PageHead :title="title" subtitle="按北京时间下单日" >
    <RouterLink v-if="mode === 'overview'" class="oa-button" to="/operations/products">我归属的商品</RouterLink>
    <RouterLink v-if="mode === 'overview'" class="oa-button" to="/operations/rankings">经营分析与排名</RouterLink>
    <RouterLink v-if="mode === 'overview' && permissions.hasMenu('dashboard')" class="oa-button" :to="{ path: '/', query: { workspace: 'tasks' } }">任务运营看板</RouterLink>
    <RouterLink v-if="mode === 'ranking'" class="oa-button" to="/operations/orders/detail">订单明细</RouterLink>
    <RouterLink v-if="mode === 'ranking'" class="oa-button" to="/operations/orders/refunds">售后退款</RouterLink>
    <RouterLink v-if="mode === 'ranking' && currentProfile?.super_admin" class="oa-button" to="/operations/orders/ledger">经营台账</RouterLink>
    <RouterLink v-if="mode === 'detail'" class="oa-button" to="/operations/products">返回商品</RouterLink>
    <RouterLink v-if="mode === 'detail' && linkedTask && permissions.hasAction('task.create')" class="oa-button oa-button-primary" :to="linkedTask">发起优化任务 <ArrowUpRight :size="14" /></RouterLink>
  </PageHead>
  <p v-if="!mayView" class="oa-error" role="alert">当前账号没有经营查询权限。</p>
  <template v-else>
    <form class="oa-actions oa-filter workspace-filters" @submit.prevent="load()">
      <label v-if="mode === 'ranking'">排名维度 <select v-model="group"><option value="shop">店铺</option><option value="growth">店铺增长</option><option v-if="currentProfile?.super_admin" value="person">人员</option><option value="platform">平台</option><option value="product">商品</option></select></label>
      <label v-if="mode === 'detail'">SKU <input v-model="selectedSku" required /></label>
      <label v-if="mode !== 'personal'">店铺 <select v-if="shopOptions.length" v-model="shop" aria-label="店铺"><option value="">全部可见店铺</option><option v-for="item in shopOptions" :key="item.shop_id" :value="item.shop_id">{{ item.shop_name || item.shop_id }}</option></select><input v-else v-model="shop" aria-label="店铺" placeholder="全部可见店铺" /></label>
      <label>开始日期 <input v-model="from" type="date" :max="to" min="2026-06-01" required /></label>
      <label>结束日期 <input v-model="to" type="date" :min="from" :max="today" required /></label>
      <details v-if="mode !== 'ranking' || group !== 'growth'" class="workspace-more"><summary>更多筛选{{ includeBrush ? ' · 含刷单' : '' }}</summary><label class="oa-check-inline"><input v-model="includeBrush" type="checkbox" />包含刷单</label></details>
      <button class="oa-button-primary" :disabled="busy">查询</button>
      <button type="button" :disabled="busy" @click="range(6)">近 7 日</button>
    </form>
    <p v-if="shopError" class="oa-watermark">{{ shopError }}</p>
    <FreshnessStatus :freshness="freshness" :stale="cached" />
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button @click="load()">重试</button></p>
    <p v-else-if="busy && !summary && !data" class="oa-panel oa-empty" role="status">正在读取经营数据…</p>
    <template v-if="mode === 'overview' && summary">
      <section class="oa-kpis" aria-label="经营指标">
        <div v-for="metric in kpis" :key="metric.name" class="oa-kpi"><span>{{ metric.name }}</span><b>{{ metric.value }}</b><small v-if="metric.foot" :class="['oa-pill', { 'oa-pill-warn': parseFloat(metric.foot) < 90 }]">{{ parseFloat(metric.foot) < 90 ? '仅覆盖' : '覆盖' }} {{ metric.foot }}</small></div>
      </section>
      <nav class="oa-actions oa-person-tabs" aria-label="经营首页内容">
        <button type="button" :aria-pressed="overviewSection === 'products'" @click="overviewSection = 'products'">商品表现</button>
        <button type="button" :aria-pressed="overviewSection === 'trend'" @click="overviewSection = 'trend'">销售趋势</button>
        <RouterLink class="oa-button" to="/me/performance">我的业绩</RouterLink>
      </nav>
      <div v-if="overviewSection === 'trend'" class="oa-split">
        <section class="oa-panel">
          <h2>订单应付趋势</h2>
          <svg v-if="points.length > 1" class="oa-chart" viewBox="0 0 600 155" role="img" aria-label="按日汇总的订单应付金额">
            <defs><linearGradient id="oa-trend-fill" x2="0" y2="1"><stop stop-color="var(--oa-accent)" stop-opacity=".4" /><stop offset="1" stop-color="var(--oa-active)" stop-opacity=".3" /></linearGradient></defs>
            <line v-for="grid in [29, 61, 93, 125]" :key="grid" x1="45" :y1="grid" x2="580" :y2="grid" stroke="var(--oa-line)" />
            <polygon :points="area" fill="url(#oa-trend-fill)" />
            <polyline :points="line" fill="none" stroke="var(--oa-accent)" stroke-width="2.2" />
            <g v-for="(point, index) in points" :key="point.day">
              <circle :cx="point.x" :cy="point.y" r="3.2" fill="var(--oa-surface)" stroke="var(--oa-accent)"><title>{{ point.day }} {{ money(point.value) }}</title></circle>
              <text v-if="index === 0 || index === points.length - 1 || points.length <= 7" :x="point.x" y="148" text-anchor="middle" fill="var(--oa-muted)" font-size="12">{{ point.day.slice(5) }}</text>
            </g>
          </svg>
          <p v-else-if="points.length" class="oa-muted">{{ points[0].day }} 订单应付 {{ kpis[0].value }}。<button type="button" :disabled="busy" @click="range(6)">查看近 7 日趋势</button></p>
          <p v-else class="oa-empty">该期间没有订单趋势数据</p>
        </section>
        <section class="oa-panel oa-link-list">
          <h2>办理与复盘</h2>
          <RouterLink v-if="permissions.hasMenu('task_list')" to="/tasks">任务运营与设计交付 <ArrowUpRight :size="15" /></RouterLink>
          <RouterLink v-if="permissions.hasMenu('cost_rules')" to="/cost-rules">SKU 成本维护 <ArrowUpRight :size="15" /></RouterLink>
          <RouterLink v-if="can('finance.apply')" to="/fees/mine">我的费用申请 <ArrowUpRight :size="15" /></RouterLink>
        </section>
      </div>
    </template>
    <p v-if="productsError" class="oa-error" role="alert">商品数据暂不可用。{{ productsError }}</p>
    <section v-if="data && (mode !== 'overview' || !summary || overviewSection === 'products')" class="oa-panel">
      <div v-if="mode === 'personal'" class="oa-actions oa-person-tabs"><span>本月实际 <strong>{{ money(data.month_actual) }}</strong></span><span>月末预估 <strong>{{ data.forecast == null ? '完整日不足或数据不全' : money(data.forecast) }}</strong></span></div>
      <p v-if="copyStatus" class="oa-muted" role="status">{{ copyStatus }}</p>
      <h2>{{ mode === 'ranking' ? '排名' : mode === 'personal' ? '实际商品销售额' : mode === 'detail' ? 'SKU 成交与成本' : '商品表现' }}</h2>
      <p v-if="!rows.length" class="oa-empty">所选范围暂无数据</p>
      <div v-else class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" :aria-label="title + '数据表'">
        <table v-if="mode === 'ranking' || mode === 'personal'" class="oa-table">
          <thead><tr><th v-if="mode === 'ranking'" class="oa-number">名次</th><th :class="{ 'oa-number': mode === 'personal' }">{{ mode === 'ranking' ? '归属' : '下单日' }}</th><th class="oa-number">{{ mode === 'ranking' && ['shop','platform','growth'].includes(appliedGroup) ? '订单应付' : '商品销售额' }}</th><th v-if="mode === 'ranking' && appliedGroup === 'growth'" class="oa-number">较同长度基期</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows" :key="row.key || row.day || index"><td v-if="mode === 'ranking'" class="oa-number">{{ offset + index + 1 }}</td><td :class="{ 'oa-number': mode === 'personal' }">{{ mode === 'ranking' && ['shop','growth'].includes(appliedGroup) ? shopName(row.key) : row.key || row.day }}</td><td class="oa-number">{{ money(mode === 'ranking' && ['shop','platform','growth'].includes(appliedGroup) ? row.payable_amount : row.item_pay_total) }}</td><td v-if="mode === 'ranking' && appliedGroup === 'growth'" class="oa-number">{{ row.growth_status === 'zero_baseline' ? '基期为零' : row.growth_rate == null ? '—' : (Number(row.growth_rate) * 100).toFixed(1) + '%' }}</td></tr></tbody>
        </table>
        <table v-else class="oa-table">
          <thead><tr><th>商品 / SKU</th><th>店铺</th><th class="oa-number">销量</th><th class="oa-number">商品销售额</th><th class="oa-number">成交核实成本</th><th class="oa-number">核实毛利</th><th class="oa-number">核实覆盖率</th><th v-if="mode === 'detail'" class="oa-number">当前维护单价</th><th v-else class="oa-action-col">操作</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows" :key="`${row.shop_id}-${row.shop_item_id}-${row.sku}-${index}`">
            <td><b class="oa-ellipsis" :title="row.product_name || row.shop_item_id || row.sku">{{ row.product_name || row.shop_item_id || row.sku }}</b><small v-if="row.sku" class="oa-muted">{{ row.sku }} <button type="button" class="workspace-copy" :aria-label="'复制 SKU ' + row.sku" @click="copySKU(row.sku)">复制</button></small></td>
            <td>{{ shopName(row.shop_id) }}</td><td class="oa-number">{{ row.quantity ?? '—' }}</td><td class="oa-number">{{ money(row.item_pay_total) }}</td>
            <td class="oa-number">{{ money(row.cost_verified_total) }}</td><td class="oa-number">{{ money(row.verified_gross) }}</td>
            <td class="oa-number"><span v-if="row.verified_coverage != null" :class="['oa-pill', { 'oa-pill-warn': Number(row.verified_coverage) < .9 }]">{{ Number(row.verified_coverage) < .9 ? '仅覆盖 ' : '' }}{{ (Number(row.verified_coverage) * 100).toFixed(1) }}%</span><span v-else>—</span></td>
            <td v-if="mode === 'detail'" class="oa-number">{{ money(data.current_cost) }}</td>
            <td v-else class="oa-action-col"><RouterLink class="workspace-row-action" v-if="row.sku" :to="'/operations/products/' + encodeURIComponent(row.sku)">查看详情</RouterLink></td>
          </tr></tbody>
        </table>
      </div>
      <div v-if="mode !== 'overview' && (data.has_more || offset)" class="oa-actions workspace-pager" aria-label="列表分页"><span>第 {{ Math.floor(offset / 100) + 1 }} 页</span><button :disabled="!offset || busy" @click="offset = Math.max(0, offset - 100); load(false)">上一页</button><button :disabled="!data.has_more || busy" @click="offset += 100; load(false)">下一页</button></div>
    </section>
    <template v-if="mode === 'detail' && data">
      <div class="oa-split">
        <section class="oa-panel oa-link-list">
          <h2>关联任务</h2>
          <RouterLink v-for="task in data.tasks || []" :key="task.id" :to="'/tasks/' + task.id">{{ task.task_no || task.id }}</RouterLink>
          <p v-if="!data.tasks?.length" class="oa-empty">暂无可查看的关联任务</p>
        </section>
        <section class="oa-panel">
          <h2>上线记录</h2>
          <dl v-if="data.feedback?.length" class="oa-kv">
            <div v-for="event in data.feedback || []" :key="event.id"><dt><time v-if="event.adjusted_on" :datetime="event.adjusted_on">{{ event.adjusted_on }}</time><span v-else>—</span></dt><dd class="workspace-note">{{ event.note || '—' }} <button :disabled="comparisonBusy" @click="compare(event)">比较前后 7 日</button></dd></div>
          </dl>
          <p v-else class="oa-empty">暂无上线记录</p>
          <p v-if="comparisonError" class="oa-error" role="alert">{{ comparisonError }}</p>
          <dl v-if="comparison" class="oa-kv"><div v-for="row in comparison" :key="row.phase"><dt>上线{{ row.phase === 'before' ? '前' : '后' }} 7 个完整日 · 商品销售额</dt><dd>{{ money(row.item_pay_total) }}</dd></div></dl>
          <p v-if="saved" class="oa-notice" role="status">{{ saved }}</p>
          <form v-if="rows.length && !stale" class="oa-form workspace-feedback" @submit.prevent="saveFeedback">
            <label><span>店铺 <span class="workspace-required" aria-hidden="true">*</span></span><select v-model="feedback.shop_id" required><option v-for="id in [...new Set(rows.map(row => row.shop_id).filter(Boolean))]" :key="id" :value="id">{{ shopName(id) }}</option></select></label>
            <label><span>实际上线日 <span class="workspace-required" aria-hidden="true">*</span></span><input v-model="feedback.adjusted_on" type="date" :max="today" min="2026-06-01" required /></label>
            <label>关联任务<select v-model="feedback.task_id"><option :value="undefined">无</option><option v-for="task in data.tasks || []" :key="task.id" :value="task.id">{{ task.task_no }}</option></select></label>
            <label><span>上线说明 <span class="workspace-required" aria-hidden="true">*</span></span><textarea v-model.trim="feedback.note" required maxlength="5000" placeholder="简述实际调整内容，便于对照销售变化" :aria-describedby="feedbackError ? 'oa-feedback-error' : undefined" /></label><p v-if="feedbackError" id="oa-feedback-error" class="oa-error" role="alert">{{ feedbackError }}</p>
            <button class="oa-button-primary" :disabled="busy">登记上线</button>
          </form>
        </section>
      </div>
      <section class="oa-panel">
        <h2>历史成交成本</h2>
        <button :disabled="busy || !rows.length" @click="loadHistory">读取成本记录</button>
        <p v-if="historyError" class="oa-error">{{ historyError }}</p>
        <div v-else-if="historyOpen && history.length" class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" :aria-label="title + '数据表'"><table class="oa-table"><thead><tr><th class="oa-number">下单日</th><th>店铺</th><th>订单子行</th><th class="oa-number">记录成本</th><th>成本档位</th><th>可信性</th></tr></thead><tbody><tr v-for="(row, index) in history" :key="index"><td>{{ row.order_time || '—' }}</td><td>{{ shopName(String(row.shop_id || '')) }}</td><td>{{ row.sub_order_id || '—' }}</td><td>{{ money(String(row.cost_amount ?? '')) }}</td><td>{{ row.cost_amount == null ? '未知' : costTierLabels[String(row.cost_tier)] || '未分类' }}</td><td><span v-if="implausibleCost(row.cost_implausible)" class="oa-pill oa-pill-warn">不可信 · 已排除</span><span v-else-if="row.cost_tier === 'verified' && row.cost_amount != null">计入核实毛利</span><span v-else>未计入核实毛利</span></td></tr></tbody></table></div>
        <p v-else-if="historyOpen" class="oa-empty">当前范围暂无历史成本记录</p>
      </section>
    </template>
  </template>
</div>
</template>

<style scoped src="./order-workspace.css" />
