<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ArrowDownRight, ArrowRight, ArrowUpRight, BarChart3, CalendarDays, ChevronLeft, ChevronRight, ClipboardList, Copy, FileText, Package, Search, Wallet } from 'lucide-vue-next'
import OrderDateField from '../components/OrderDateField.vue'
import OperationNav from '../components/OperationNav.vue'
import PageHead from '../components/PageHead.vue'
import FreshnessStatus from '../components/FreshnessStatus.vue'
import { getToken, TOKEN_STORAGE_KEY, type HttpAppError } from '@/services/http'
import { api, dateInChina, errorText, money, send } from '../api'
import { useOAAccess } from '../access'
import { usePermissionsStore } from '@/stores/permissions'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const permissions = usePermissionsStore()
const route = useRoute()

type Mode = 'overview' | 'products' | 'detail' | 'ranking' | 'personal'
const props = defineProps<{ mode: Mode; productId?: string }>()
type DataRow = {
  day?: string; hour?: number | string; shop_id?: string; platform?: string; shop_item_id?: string; sku?: string; product_name?: string
  quantity?: string; order_count?: number; payable_amount?: string; item_pay_total?: string
  cost_verified_total?: string | null; item_pay_verified_total?: string | null; verified_gross?: string | null; verified_coverage?: string | null
  key?: string; tasks?: { id?: number; task_id?: number; task_no?: string; title?: string }[]
  feedback?: { id: number; note?: string; live_date?: string; task_id?: number }[]
  shop_name?: string; growth_status?: string; growth_rate?: number
}
type TaskLink = { id: number; task_no: string; status: string }
type Feedback = { id: number; shop_id: string; note: string; adjusted_on: string; task_id?: number }
type OrderResult = { rows: DataRow[]; freshness?: { source_committed_at?: string; lag_seconds?: number } | null; stale?: boolean; has_more?: boolean; current_cost?: string | null; cost_context_available?: boolean; tasks?: TaskLink[]; feedback?: Feedback[]; forecast?: string | null; month_actual?: string | null }

const today = dateInChina()
const earliest = '2026-06-01'
const periods = computed(() => [
  { label: '今天', daysBack: 0 },
  { label: '近 7 日', daysBack: 6 },
  { label: '近 30 日', daysBack: 29 },
  ...(props.mode === 'personal' ? [{ label: '本月', daysBack: Number(today.slice(-2)) - 1 }] : []),
])
const appliedPeriod = ref<{ from: string; to: string }>()
function rangeStart(daysBack: number) {
  const date = new Date(today + 'T12:00:00Z')
  date.setUTCDate(date.getUTCDate() - daysBack)
  return [date.toISOString().slice(0, 10), earliest].sort()[1]
}
function routeDate(key: 'from' | 'to') {
  const value = route.query[key]
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value) || value < earliest || value > today) return ''
  const date = new Date(value + 'T12:00:00Z')
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value ? value : ''
}
const incomingFrom = routeDate('from')
const incomingTo = routeDate('to')
const useIncomingPeriod = Boolean(incomingFrom && incomingTo && incomingFrom <= incomingTo)
const from = ref(useIncomingPeriod ? incomingFrom : today)
const to = ref(useIncomingPeriod ? incomingTo : today)
const shop = ref(props.mode !== 'personal' && typeof route.query.shop_id === 'string' ? route.query.shop_id : '')
const navigationQuery = ref<Record<string, string>>({})
function recordScope(query: URLSearchParams) {
  const start = query.get('from') || today
  const end = query.get('to') || today
  const shopID = query.get('shop_id')
  appliedPeriod.value = { from: start, to: end }
  navigationQuery.value = {
    from: start,
    to: end,
    ...(shopID ? { shop_id: shopID } : {}),
    ...(query.get('include_brush') === '1' ? { include_brush: '1' } : {}),
  }
}
function productLink(sku: string, shopId?: string) {
  return {
    path: '/operations/products/' + encodeURIComponent(sku),
    query: shopId ? { ...navigationQuery.value, shop_id: shopId } : navigationQuery.value,
  }
}
const selectedSku = ref(props.productId ?? '')
const group = ref('shop')
const appliedGroup = ref('shop')
let appliedQuery = ''
const includeBrush = ref(route.query.include_brush === '1')
const loadedDetail = ref<{ shop: string; sku: string; from: string; to: string }>()
const shopOptions = ref<DataRow[]>([])
const shopError = ref('')
const productsError = ref('')
const history = ref<Record<string, unknown>[]>([])
const historyError = ref('')
const historyOpen = ref(false)
const historyBusy = ref(false)
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
const hourlyTrend = ref<OrderResult>()
const weekTrend = ref<OrderResult>()
const weekTrendUnavailable = ref(false)
const rankingSummary = ref<OrderResult>()
const overviewDay = computed(() => props.mode === 'overview' && appliedPeriod.value?.from === appliedPeriod.value?.to ? appliedPeriod.value?.to : undefined)
function earlierDay(day: string, days: number) {
  const date = new Date(day + 'T12:00:00Z')
  date.setUTCDate(date.getUTCDate() - days)
  return date.toISOString().slice(0, 10)
}
const trendWeekStart = computed(() => overviewDay.value ? [earlierDay(overviewDay.value, 6), earliest].sort()[1] : '')
const trendPrevious = computed(() => overviewDay.value ? earlierDay(overviewDay.value, 1) : '')
const busy = ref(false)
const error = ref('')
const latest = ref(0)
const locallyStale = ref(false)
let appliedOverview: { query: string; offset: number; account: string; token: string } | undefined
let refreshTimer: number | undefined
const mayView = computed(() => can('orders.view'))
const title = computed(() => ({ overview: '经营总览', products: '商品表现', detail: '商品详情', ranking: '经营排行', personal: '我的业绩' }[props.mode]))

function queryParams(options: Record<string, string> = {}) {
  const query = new URLSearchParams({ from: from.value, to: to.value, ...options })
  if (includeBrush.value && options.group !== 'growth') query.set('include_brush', '1')
  if (shop.value.trim()) query.set('shop_id', shop.value.trim())
  return query.toString()
}
function clearOverview() {
  data.value = undefined
  summary.value = undefined
  locallyStale.value = false
  hourlyTrend.value = undefined
  weekTrend.value = undefined
  weekTrendUnavailable.value = false
}

async function loadOverview(refresh = false, pageChange = false) {
  const account = currentProfile.value?.id
  const token = getToken()
  if (appliedOverview && (appliedOverview.account !== account || appliedOverview.token !== token)) resetOverviewSession()
  if (!mayView.value || !account || !token) return
  const scope = refresh ? appliedOverview : {
    account, token, offset: offset.value,
    query: pageChange && appliedOverview ? appliedOverview.query : queryParams(),
  }
  if (!scope || scope.account !== account || scope.token !== token) return
  const sequence = ++latest.value
  const sameScope = appliedOverview?.query === scope.query && appliedOverview.account === account && appliedOverview.token === token
  const sameProductPage = sameScope && appliedOverview?.offset === scope.offset
  if (!sameScope) clearOverview()
  else if (!sameProductPage) data.value = undefined
  appliedOverview = scope
  recordScope(new URLSearchParams(scope.query))
  busy.value = true
  productsError.value = ''
  error.value = ''
  try {
    async function readOrders(endpoint: string, request: URLSearchParams) {
      try {
        return await api<OrderResult>('/oa-api/orders/' + endpoint + '?' + request)
      } catch (cause) {
        const failure = cause as HttpAppError
        const raw = failure.responseData as { error?: string } | undefined
        if (sequence === latest.value && (failure.status === 401 || failure.status === 403 || raw?.error === '订单数据源拒绝访问')) {
          clearOverview()
          appliedOverview = undefined
          error.value = errorText(failure)
        }
        throw cause
      }
    }
    async function readTrend(query: string) {
      if (!overviewDay.value) return
      const request = new URLSearchParams(query)
      if (trendPrevious.value >= earliest) {
        request.set('from', trendPrevious.value)
        try {
          const hourly = await readOrders('hourly', request)
          if (hourly.rows?.some(row => row?.day && row.hour != null)) return { hourly }
        } catch (cause) {
          if (!appliedOverview || sequence !== latest.value) throw cause
        }
      }
      request.set('from', trendWeekStart.value)
      return { weekly: await readOrders('summary', request) }
    }
    const [head, tail, trend] = await Promise.allSettled([
      readOrders('summary', new URLSearchParams(scope.query)),
      readOrders('products', new URLSearchParams(scope.query)),
      readTrend(scope.query),
    ])
    if (sequence !== latest.value) return
    if (!appliedOverview) return
    if (scope.account !== currentProfile.value?.id || scope.token !== getToken() || !mayView.value) {
      clearOverview()
      appliedOverview = undefined
      return
    }
    locallyStale.value = sameScope && (
      (head.status === 'rejected' && !!summary.value) || (sameProductPage && tail.status === 'rejected' && !!data.value)
    )
    if (trend.status === 'fulfilled') {
      hourlyTrend.value = trend.value?.hourly
      weekTrend.value = trend.value?.weekly
    }
    weekTrendUnavailable.value = trend.status === 'rejected'
    if (head.status === 'fulfilled') summary.value = head.value
    else if (!summary.value) error.value = errorText(head.reason)
    if (tail.status === 'fulfilled') data.value = tail.value
    else if (!data.value) productsError.value = errorText(tail.reason)
  } finally {
    if (sequence === latest.value) busy.value = false
  }
}

async function load(resetPage = true) {
  if (resetPage) offset.value = 0
  if (props.mode === 'overview') return loadOverview(false, !resetPage)
  if (!mayView.value) return
  if (props.mode === 'detail' && !selectedSku.value.trim()) return
  const sequence = ++latest.value
  const detailFilters = !resetPage && loadedDetail.value ? loadedDetail.value : { shop: shop.value, sku: selectedSku.value.trim(), from: from.value, to: to.value }
  busy.value = true
  loadedDetail.value = undefined
  historyOpen.value = false
  comparison.value = undefined
  comparisonError.value = ''
  productsError.value = ''
  data.value = undefined
  summary.value = undefined
  rankingSummary.value = undefined
  error.value = ''
  try {
    const endpoint = { products: 'products', ranking: 'rankings', detail: 'sku', personal: 'personal' }[props.mode]
    const options: Record<string, string> = {}
    if (props.mode === 'ranking') options.group = group.value
    if (props.mode === 'products' && route.query.scope !== 'overview') options.mine = '1'
    if (offset.value) options.offset = String(offset.value)
    if (props.mode === 'detail') options.sku = selectedSku.value.trim()
    if (resetPage || !appliedQuery) {
      appliedGroup.value = group.value
      appliedQuery = queryParams(options)
      recordScope(new URLSearchParams(appliedQuery))
    }
    const applied = new URLSearchParams(appliedQuery)
    applied.set('offset', String(offset.value))
    const summaryQuery = new URLSearchParams(applied)
    summaryQuery.delete('offset')
    summaryQuery.delete('group')
    const dimensions = props.mode === 'ranking' && (
      ['shop', 'growth'].includes(appliedGroup.value) ||
      (appliedGroup.value === 'platform' && !summaryQuery.has('include_brush'))
    ) ? api<OrderResult>('/oa-api/orders/summary?' + summaryQuery).catch(() => undefined) : undefined
    const [result, supplemental] = await Promise.all([api<OrderResult>(`/oa-api/orders/${endpoint}?${applied}`), dimensions])
    if (sequence === latest.value) {
      data.value = result
      rankingSummary.value = supplemental?.stale ? undefined : supplemental
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

const summaryRows = computed(() => summary.value?.rows?.filter(Boolean) ?? [])
const rows = computed(() => data.value?.rows?.filter(Boolean) ?? [])
const displayRows = computed(() => props.mode === 'overview' ? rows.value.slice(0, 10) : rows.value)
const linkedTask = computed(() => {
  const filters = loadedDetail.value
  if (!filters || !rows.value.length || data.value?.stale) return ''
  const shop_id = filters.shop || (new Set(rows.value.map(row => row.shop_id)).size === 1 ? rows.value[0]?.shop_id : '')
  if (!shop_id) return ''
  const query = new URLSearchParams({ intent: 'retouch', oa_shop: shop_id, oa_sku: filters.sku, oa_from: filters.from, oa_to: filters.to })
  return '/tasks/create?' + query
})
const freshness = computed(() => {
  const available = [summary.value?.freshness, data.value?.freshness].filter(value => value != null)
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
const leadingShops = computed(() => {
  const stores = new Map<string, DataRow[]>()
  for (const row of summaryRows.value) {
    if (!row.shop_id) continue
    const items = stores.get(row.shop_id) ?? []
    items.push(row)
    stores.set(row.shop_id, items)
  }
  const totals = [...stores].flatMap(([id, items]) => {
    const amount = sum(items, 'payable_amount')
    if (amount === null) return []
    return [{ id, amount }]
  })
  const precision = Math.max(0, ...totals.map(item => item.amount.split('.')[1]?.length ?? 0))
  const sorted = totals.map(item => {
    const [whole, fraction = ''] = item.amount.replace('-', '').split('.')
    const value = BigInt(whole + fraction.padEnd(precision, '0')) * (item.amount.startsWith('-') ? -1n : 1n)
    return { ...item, value }
  })
  return sorted.sort((left, right) => left.value === right.value ? left.id.localeCompare(right.id) : left.value > right.value ? -1 : 1).slice(0, 5)
})
const chartRows = computed(() => props.mode === 'personal' ? rows.value : overviewDay.value ? (weekTrend.value?.rows?.filter(Boolean) ?? []) : summaryRows.value)
const chartKey = computed<MoneyKey>(() => props.mode === 'personal' ? 'item_pay_total' : 'payable_amount')
const chartDays = computed(() => {
  const byDay = new Map<string, DataRow[]>()
  for (const row of chartRows.value) {
    if (!row.day) continue
    const items = byDay.get(row.day) ?? []
    items.push(row)
    byDay.set(row.day, items)
  }
  return [...byDay].sort(([left], [right]) => left.localeCompare(right)).flatMap(([day, items]) => {
    const amount = sum(items, chartKey.value)
    if (amount === null || !Number.isFinite(Number(amount))) return []
    return [{ day, amount, value: Number(amount) }]
  })
})
const chartHeading = computed(() => props.mode === 'personal' ? '实际商品销售额趋势' : hourlyTrend.value ? '分时订单应付' : overviewDay.value ? '近 7 日订单应付' : '订单应付趋势')
const chartPeriod = computed(() => hourlyTrend.value ? overviewDay.value + ' 对比 ' + trendPrevious.value : overviewDay.value ? trendWeekStart.value + ' 至 ' + overviewDay.value : periodLabel.value)
const plot = { left: 100, right: 702, top: 12, bottom: props.mode === 'overview' ? 92 : 121 }
const chartHeight = props.mode === 'overview' ? 120 : 149
const hourlySeries = computed(() => {
  const entries = (hourlyTrend.value?.rows ?? []).flatMap(row => {
    const hour = Number(row?.hour)
    const value = Number(row?.payable_amount)
    if (!row?.day || row.hour == null || !Number.isInteger(hour) || hour < 0 || hour > 23 ||
      row.payable_amount == null || !Number.isFinite(value)) return []
    return [{ day: row.day, hour, amount: row.payable_amount, value }]
  })
  const isToday = overviewDay.value === today
  const currentHour = isToday ? Number(new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Shanghai', hour: '2-digit', hourCycle: 'h23' }).format(new Date())) : 23
  const current = entries.filter(point => point.day === overviewDay.value && point.hour <= currentHour).sort((a, b) => a.hour - b.hour)
  const lastHour = isToday ? current.at(-1)?.hour ?? currentHour : 23
  const previous = entries.filter(point => point.day === trendPrevious.value && point.hour <= lastHour).sort((a, b) => a.hour - b.hour)
  return [{ label: '所选日', points: current }, { label: '前一日同期', points: previous }]
})
const axisValues = computed(() => hourlyTrend.value ? hourlySeries.value.flatMap(series => series.points.map(point => point.value)) : chartDays.value.map(day => day.value))
const domain = computed(() => ({
  min: Math.min(0, ...axisValues.value),
  max: Math.max(0, ...axisValues.value),
}))
const plotY = (value: number) => plot.bottom - (value - domain.value.min) / (domain.value.max - domain.value.min || 1) * (plot.bottom - plot.top)
const hourX = (hour: number) => plot.left + hour / 24 * (plot.right - plot.left)
const hourLabel = (hour: number) => String(hour).padStart(2, '0') + ':00'
const hourlyLines = computed(() => hourlySeries.value.map(series => ({
  label: series.label,
  points: series.points.map(point => ({ ...point, x: hourX(point.hour), y: plotY(point.value) })),
})))
const ticks = computed(() => (domain.value.min === domain.value.max ? [0] : [0, 1, 2, 3].map(index =>
  domain.value.min + (domain.value.max - domain.value.min) * index / 3)))
const axisNumber = new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 })
const points = computed(() => {
  const start = Date.parse(overviewDay.value ? trendWeekStart.value : appliedPeriod.value?.from ?? '')
  const end = Date.parse(appliedPeriod.value?.to ?? '')
  return chartDays.value.map(day => ({
    ...day,
    x: end > start ? plot.left + (Date.parse(day.day) - start) / (end - start) * (plot.right - plot.left) : (plot.left + plot.right) / 2,
    y: plotY(day.value),
  }))
})
const activeDay = ref('')
const activePoint = computed(() => points.value.find(point => point.day === activeDay.value) ?? points.value.at(-1))
const line = computed(() => points.value.map(point => point.x + ',' + point.y).join(' '))
const area = computed(() => {
  const first = points.value[0]
  const last = points.value.at(-1)
  return first && last ? first.x + ',' + plotY(0) + ' ' + line.value + ' ' + last.x + ',' + plotY(0) : ''
})
function showDateLabel(index: number) {
  const last = points.value.length - 1
  return index === 0 || index === last || (index % Math.ceil(last / 6) === 0 && last - index >= Math.ceil(last / 6))
}
function setActivePoint(event: PointerEvent) {
  const chart = event.currentTarget as SVGSVGElement
  const transform = chart.getScreenCTM()
  if (!transform || !points.value.length) return
  const coordinate = new DOMPoint(event.clientX, event.clientY).matrixTransform(transform.inverse())
  activeDay.value = points.value.reduce((nearest, point) =>
    Math.abs(point.x - coordinate.x) < Math.abs(nearest.x - coordinate.x) ? point : nearest).day
}
function range(daysBack: number) {
  from.value = rangeStart(daysBack)
  to.value = today
  if (props.mode !== 'detail' || selectedSku.value.trim()) void load()
}

const rankingUsesPayable = computed(() => ['shop', 'platform', 'growth'].includes(appliedGroup.value))
const rankingValue = (row: DataRow) => rankingUsesPayable.value ? row.payable_amount : row.item_pay_total
const rankingMetrics = computed(() => {
  const groups = new Map<string, DataRow[]>()
  for (const row of rankingSummary.value?.rows ?? []) {
    const key = appliedGroup.value === 'platform' ? row?.platform : row?.shop_id
    if (!key) continue
    const entries = groups.get(key) ?? []
    entries.push(row)
    groups.set(key, entries)
  }
  const total = Number(sum(rankingSummary.value?.rows?.filter(Boolean) ?? [], 'payable_amount'))
  return new Map([...groups].map(([key, entries]) => {
    const payable = Number(sum(entries, 'payable_amount'))
    const gross = sum(entries, 'verified_gross')
    const verifiedSales = Number(sum(entries, 'item_pay_verified_total'))
    const share = total > 0 && payable >= 0 && payable <= total ? payable / total : null
    return [key, {
      count: entries.every(row => row.order_count != null) ? entries.reduce((value, row) => value + Number(row.order_count), 0) : null,
      gross,
      grossRate: gross != null && verifiedSales > 0 ? Number(gross) / verifiedSales : null,
      share,
    }]
  }))
})
const showRankingMetrics = computed(() => ['shop', 'platform', 'growth'].includes(appliedGroup.value))
const percent = (value?: number | null) => value == null ? '—' : (value * 100).toFixed(1) + '%'
const rankingGroups: Record<string, string> = { shop: '店铺', growth: '店铺', person: '人员', platform: '平台', product: '商品' }
const periodLabel = computed(() => {
  const value = appliedPeriod.value
  return value ? value.from === value.to ? value.from : value.from + ' 至 ' + value.to : ''
})
const monthStart = today.slice(0, 7) + '-01'
const fullCurrentMonth = computed(() => !!appliedPeriod.value && appliedPeriod.value.from <= monthStart && appliedPeriod.value.to === today)
const canReview = computed(() => permissions.hasMenu('task_list') || permissions.hasMenu('cost_rules') || can('finance.apply'))

async function loadShops() {
  try { shopOptions.value = (await api<OrderResult>('/oa-api/orders/shops')).rows || [] }
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
  const sequence = latest.value
  try {
    const query = new URLSearchParams({ shop_id: event.shop_id, sku: filters.sku, adjusted_on: event.adjusted_on, days: '7' })
    const result = await api<OrderResult>('/oa-api/orders/compare?' + query)
    if (sequence === latest.value) comparison.value = result.rows
  } catch (cause) { if (sequence === latest.value) comparisonError.value = errorText(cause) }
  finally { comparisonBusy.value = false }
}
async function loadHistory() {
  const filters = loadedDetail.value
  if (!filters) return
  history.value = []
  historyError.value = ''
  historyOpen.value = true
  historyBusy.value = true
  const sequence = latest.value
  try {
    const query = new URLSearchParams({ from: filters.from, to: filters.to, sku: filters.sku })
    if (filters.shop) query.set('shop_id', filters.shop)
    const result = await api<{ rows: Record<string, unknown>[] }>('/oa-api/orders/cost-history?' + query)
    if (sequence === latest.value) history.value = result.rows
  } catch (cause) { if (sequence === latest.value) historyError.value = errorText(cause) }
  finally { historyBusy.value = false }
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
  offset.value = 0
  busy.value = false
  clearOverview()
}
function onSessionStorage(event: StorageEvent) {
  if (event.key === TOKEN_STORAGE_KEY || event.key === null) resetOverviewSession()
}
watch(() => props.productId, id => {
  if (props.mode !== 'detail') return
  selectedSku.value = id ?? ''
  void load()
})
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
  if (!useIncomingPeriod && props.mode === 'personal') {
    from.value = monthStart
  } else if (!useIncomingPeriod && props.mode === 'detail') {
    from.value = rangeStart(6)
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
<div :class="['oa-page commerce-workspace', { 'commerce-overview': mode === 'overview' }]">
  <PageHead :title="title" subtitle="按北京时间下单日">
    <FreshnessStatus v-if="mayView" :freshness="freshness" :stale="cached" />
    <RouterLink v-if="mode === 'overview' && permissions.hasMenu('dashboard')" class="oa-button" :to="{ path: '/', query: { workspace: 'tasks' } }"><BarChart3 :size="15" />任务运营看板</RouterLink>
    <RouterLink v-if="mode === 'detail'" class="oa-button" :to="{ path: '/operations/products', query: navigationQuery }"><ChevronLeft :size="15" />返回商品</RouterLink>
    <RouterLink v-if="mode === 'detail' && linkedTask && permissions.hasAction('task.create')" class="oa-button oa-button-primary" :to="linkedTask">发起优化任务 <ArrowUpRight :size="14" /></RouterLink>
  </PageHead>
  <OperationNav v-if="mayView" class="commerce-nav" :query="navigationQuery" />
  <p v-if="!mayView" class="oa-error" role="alert">当前账号没有经营查询权限。</p>
  <template v-else>
    <form class="oa-panel commerce-filter" aria-label="经营筛选" @submit.prevent="load()">
      <div class="oa-actions oa-filter workspace-filters">
        <div class="commerce-period" aria-label="快捷日期">
          <CalendarDays :size="15" aria-hidden="true" />
          <button v-for="period in periods" :key="period.label" type="button" :disabled="busy"
            :aria-pressed="from === rangeStart(period.daysBack) && to === today" @click="range(period.daysBack)">{{ period.label }}</button>
        </div>
      <label v-if="mode === 'ranking'">排名维度 <select v-model="group"><option value="shop">店铺</option><option value="growth">店铺增长</option><option v-if="currentProfile?.super_admin" value="person">人员</option><option value="platform">平台</option><option value="product">商品</option></select></label>
      <label v-if="mode === 'detail'">SKU <input v-model="selectedSku" required /></label>
      <label v-if="mode !== 'personal'">店铺 <select v-if="shopOptions.length" v-model="shop" aria-label="店铺"><option value="">全部可见店铺</option><option v-if="shop && !shopOptions.some(item => item.shop_id === shop)" :value="shop">{{ shop }}</option><option v-for="item in shopOptions" :key="item.shop_id" :value="item.shop_id">{{ item.shop_name || item.shop_id }}</option></select><input v-else v-model="shop" aria-label="店铺" placeholder="全部可见店铺" /></label>
      <label>日期 <OrderDateField v-model="from" label="开始日期" :max="to" :min="earliest" /></label>
      <label>至 <OrderDateField v-model="to" label="结束日期" :min="from" :max="today" /></label>
      <label v-if="mode !== 'ranking' || group !== 'growth'" class="oa-check-inline"><input v-model="includeBrush" type="checkbox" />包含刷单</label>
      <button class="oa-button-primary" :disabled="busy"><Search :size="15" />{{ busy ? '查询中…' : '查询' }}</button>
      </div>
    </form>
    <p v-if="shopError" class="oa-watermark">{{ shopError }}</p>
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button @click="load()">重试</button></p>
    <p v-else-if="busy && !summary && !data" class="oa-panel oa-empty" role="status">正在读取经营数据…</p>
    <section v-if="mode === 'overview' && summary" class="oa-kpis commerce-metrics" aria-label="经营指标">
      <div v-for="(metric, index) in kpis" :key="metric.name" :class="['oa-kpi', { 'commerce-metric-primary': index === 0 }]">
        <span>{{ metric.name }}</span><b>{{ metric.value }}</b>
        <small v-if="metric.foot" :class="['commerce-coverage', { 'commerce-coverage-low': parseFloat(metric.foot) < 90 }]">{{ parseFloat(metric.foot) < 90 ? '仅覆盖' : '覆盖' }} {{ metric.foot }}</small>
      </div>
    </section>
    <section v-if="mode === 'personal' && data && fullCurrentMonth" class="oa-kpis commerce-metrics commerce-personal" aria-label="本月业绩">
      <div class="oa-kpi commerce-metric-primary"><span>本月实际</span><b>{{ money(data.month_actual) }}</b></div>
      <div class="oa-kpi"><span>月末预估</span><b>{{ money(data.forecast) }}</b><small v-if="data.forecast == null">完整日不足或数据不全</small></div>
    </section>
    <div v-if="(mode === 'overview' && summary) || (mode === 'personal' && data)" :class="['commerce-insights', { 'commerce-insights-links': mode === 'overview' }]">
      <section class="oa-panel commerce-chart-panel">
        <div class="commerce-panel-heading">
          <div><h2>{{ chartHeading }}</h2><p class="oa-muted">{{ chartPeriod }}</p></div>
          <p v-if="hourlyTrend" class="commerce-hourly-legend"><span>所选日</span><span>前一日同期</span></p>
          <div v-else-if="activePoint && (overviewDay || points.length > 1)" class="commerce-chart-value">
            <time :datetime="activePoint.day">{{ activePoint.day }}</time>
            <strong>{{ money(activePoint.amount) }}</strong>
          </div>
        </div>
        <div v-if="hourlyTrend" class="commerce-chart-scroll">
          <svg class="commerce-chart" :viewBox="`0 0 720 ${chartHeight}`" role="group" aria-label="分时订单应付，按小时显示已有数据">
            <g v-for="tick in ticks" :key="tick">
              <line :x1="plot.left" :x2="plot.right" :y1="plotY(tick)" :y2="plotY(tick)" stroke="var(--oa-line)" />
              <text :x="plot.left - 12" :y="plotY(tick) + 4" text-anchor="end">{{ axisNumber.format(tick) }} 元</text>
            </g>
            <text v-for="hour in [0, 4, 8, 12, 16, 20, 24]" :key="hour" :x="hourX(hour)" :y="chartHeight - 7" text-anchor="middle">{{ hourLabel(hour) }}</text>
            <g v-for="(series, index) in hourlyLines" :key="series.label" :class="{ 'commerce-hourly-baseline': index === 1 }">
              <g v-for="(point, i) in series.points" :key="point.hour">
                <line v-if="i > 0 && series.points[i - 1].hour + 1 === point.hour" class="commerce-hourly-segment"
                  :x1="series.points[i - 1].x" :y1="series.points[i - 1].y" :x2="point.x" :y2="point.y" />
                <circle class="commerce-chart-dot" :cx="point.x" :cy="point.y" r="3" tabindex="0" :aria-label="point.day + ' ' + hourLabel(point.hour) + ' ' + money(point.amount)">
                  <title>{{ point.day }} {{ hourLabel(point.hour) }} {{ money(point.amount) }}</title>
                </circle>
              </g>
            </g>
          </svg>
          <p v-if="hourlyTrend.stale || weekTrendUnavailable" class="oa-muted">分时趋势显示上次查询成功的快照。</p>
        </div>
        <div v-else-if="overviewDay && points.length" class="commerce-chart-scroll">
          <svg class="commerce-chart commerce-week-chart" viewBox="0 0 720 98" role="group" :aria-label="chartHeading" @pointermove="setActivePoint">
            <line :x1="plot.left" :x2="plot.right" :y1="plotY(0) * .7" :y2="plotY(0) * .7" stroke="var(--oa-line)" />
            <g v-for="point in points" :key="point.day">
              <rect :x="point.x - 15" :y="Math.min(point.y, plotY(0)) * .7" width="30" :height="Math.max(1, Math.abs(point.y - plotY(0)) * .7)" rx="3"
                :class="{ 'commerce-today-bar': point.day === overviewDay }" tabindex="0" :aria-label="point.day + ' ' + money(point.amount)" @focus="activeDay = point.day">
                <title>{{ point.day }} {{ money(point.amount) }}</title>
              </rect>
              <text :x="point.x" y="91" text-anchor="middle">{{ point.day.slice(5) }}</text>
            </g>
          </svg>
          <p v-if="weekTrendUnavailable || weekTrend?.stale" class="oa-muted">近 7 日趋势显示上次查询成功的快照。</p>
        </div>
        <div v-else-if="points.length > 1" class="commerce-chart-scroll">
          <svg class="commerce-chart" :viewBox="`0 0 720 ${chartHeight}`" role="group" :aria-label="chartHeading" @pointermove="setActivePoint">
            <defs><linearGradient id="commerce-trend-fill" x1="0" y1="0" x2="0" y2="1"><stop stop-color="var(--oa-accent)" stop-opacity=".18" /><stop offset="1" stop-color="var(--oa-accent)" stop-opacity=".02" /></linearGradient></defs>
            <g v-for="tick in ticks" :key="tick">
              <line :x1="plot.left" :x2="plot.right" :y1="plotY(tick)" :y2="plotY(tick)" stroke="var(--oa-line)" />
              <text :x="plot.left - 12" :y="plotY(tick) + 4" text-anchor="end">{{ axisNumber.format(tick) }} 元</text>
            </g>
            <line v-if="activePoint" :x1="activePoint.x" :x2="activePoint.x" :y1="plot.top" :y2="plot.bottom" class="commerce-chart-guide" />
            <polygon v-if="points.length > 1" :points="area" fill="url(#commerce-trend-fill)" />
            <polyline v-if="points.length > 1" :points="line" fill="none" stroke="var(--oa-accent)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
            <g v-for="(point, index) in points" :key="point.day">
              <circle :cx="point.x" :cy="point.y" :r="point.day === activePoint?.day ? 5 : 3" class="commerce-chart-dot" tabindex="0" :aria-label="point.day + ' ' + money(point.amount)" @focus="activeDay = point.day">
                <title>{{ point.day }} {{ money(point.amount) }}</title>
              </circle>
              <text v-if="showDateLabel(index)" :x="point.x" :y="chartHeight - 7" text-anchor="middle">{{ point.day.slice(5) }}</text>
            </g>
          </svg>
        </div>
        <div v-else-if="activePoint" class="commerce-single-day">
          <span><time :datetime="activePoint.day">{{ activePoint.day }}</time> · {{ mode === 'personal' ? '实际商品销售额' : '订单应付' }}</span>
          <strong>{{ money(activePoint.amount) }}</strong>
          <button type="button" :disabled="busy" @click="range(6)">查看近 7 日趋势 <ArrowRight :size="15" /></button>
        </div>
        <p v-else class="oa-empty">{{ weekTrendUnavailable ? '近 7 日趋势暂不可用' : '所选范围暂无趋势数据' }}</p>
      </section>
      <section v-if="mode === 'overview'" class="oa-panel commerce-shop-rank">
        <div class="commerce-panel-heading"><div><h2>店铺表现</h2><p class="oa-muted">按订单应付排名 · 前 5</p></div></div>
        <ol v-if="leadingShops.length">
          <li v-for="(item, index) in leadingShops" :key="item.id">
            <RouterLink :to="{ path: '/operations/orders/detail', query: { ...navigationQuery, shop_id: item.id } }">
              <span class="commerce-rank">{{ index + 1 }}</span><span class="commerce-shop-name" :title="shopName(item.id)">{{ shopName(item.id) }}</span>
              <strong>{{ money(item.amount) }}</strong><ArrowUpRight :size="14" />
            </RouterLink>
          </li>
        </ol>
        <p v-else class="oa-empty">所选范围暂无店铺数据</p>
      </section>
    </div>
    <p v-if="productsError" class="oa-error" role="alert">商品数据暂不可用。{{ productsError }} <button type="button" @click="load(false)">重试商品数据</button></p>
    <section v-if="data" class="oa-panel commerce-results">
      <div class="commerce-panel-heading">
        <div><h2>{{ mode === 'ranking' ? rankingGroups[appliedGroup] + (appliedGroup === 'growth' ? '增长' : '') + '排名' : mode === 'personal' ? '实际商品销售额' : mode === 'detail' ? 'SKU 成交与成本' : mode === 'overview' ? '热销商品 Top 10' : '商品表现' }}</h2><p class="oa-muted">{{ periodLabel }}</p></div>
        <RouterLink v-if="mode === 'overview'" class="workspace-row-action" :to="{ path: '/operations/products', query: { ...navigationQuery, scope: 'overview' } }">查看全部商品表现 <ArrowRight :size="14" /></RouterLink>
        <span v-if="copyStatus" class="oa-muted" role="status">{{ copyStatus }}</span>
        <span v-else-if="mode !== 'overview' && rows.length" class="oa-muted">{{ rows.length }} 条{{ data.has_more ? ' · 还有更多' : '' }}</span>
      </div>
      <p v-if="!rows.length" class="oa-empty">所选范围暂无数据</p>
      <div v-else class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" :aria-label="title + '数据表'">
        <table v-if="mode === 'ranking' || mode === 'personal'" :class="['oa-table', { 'commerce-ranking-table': mode === 'ranking' }]">
          <thead><tr><th v-if="mode === 'ranking'" class="oa-number">名次</th><th :class="{ 'oa-number': mode === 'personal' }">{{ mode === 'ranking' ? '归属' : '下单日' }}</th><th class="oa-number">{{ mode === 'ranking' && ['shop','platform','growth'].includes(appliedGroup) ? '订单应付' : '商品销售额' }}</th><template v-if="mode === 'ranking' && showRankingMetrics"><th class="oa-number">应付占比</th><th class="oa-number">有效订单数</th><th class="oa-number">核实毛利</th><th class="oa-number" title="核实毛利 ÷ 有核实成本的商品销售额">核实毛利率</th></template><th v-if="mode === 'ranking' && appliedGroup === 'growth'" class="oa-number">较同长度基期</th></tr></thead>
          <tbody>
            <tr v-for="(row, index) in rows" :key="row.key ? (row.shop_id || '') + ':' + row.key : row.day || index">
              <td v-if="mode === 'ranking'" class="oa-number"><span :class="['commerce-rank', { 'commerce-rank-leading': offset + index < 3 }]">{{ offset + index + 1 }}</span></td>
              <td :class="{ 'oa-number': mode === 'personal' }">
                <RouterLink v-if="mode === 'ranking' && ['shop', 'growth'].includes(appliedGroup) && row.key"
                  class="workspace-row-action commerce-rank-link"
                  :to="{ path: '/operations/orders/detail', query: { ...navigationQuery, shop_id: row.key } }">
                  {{ shopName(row.key) }} <ArrowUpRight :size="14" />
                </RouterLink>
                <span v-else>{{ row.key || row.day || '—' }}</span>
              </td>
              <td class="oa-number"><strong>{{ money(mode === 'ranking' ? rankingValue(row) : row.item_pay_total) }}</strong></td>
              <template v-if="mode === 'ranking' && showRankingMetrics">
                <td class="oa-number">
                  <div v-if="row.key && rankingMetrics.get(row.key)?.share != null" class="commerce-rank-amount">
                    <span class="commerce-rank-track" aria-hidden="true"><span :style="{ inlineSize: percent(rankingMetrics.get(row.key)?.share) }" /></span>
                    {{ percent(rankingMetrics.get(row.key)?.share) }}
                  </div>
                  <span v-else>—</span>
                </td>
                <td class="oa-number">{{ row.key ? (rankingMetrics.get(row.key)?.count?.toLocaleString('zh-CN') ?? '—') : '—' }}</td>
                <td class="oa-number">{{ money(row.key ? rankingMetrics.get(row.key)?.gross : null) }}</td>
                <td class="oa-number">{{ percent(row.key ? rankingMetrics.get(row.key)?.grossRate : null) }}</td>
              </template>
              <td v-if="mode === 'ranking' && appliedGroup === 'growth'" class="oa-number">
                <span v-if="row.growth_rate != null && row.growth_status !== 'zero_baseline'" :class="['commerce-growth', Number(row.growth_rate) > 0 ? 'commerce-growth-positive' : Number(row.growth_rate) < 0 ? 'commerce-growth-negative' : '']">
                  <ArrowUpRight v-if="Number(row.growth_rate) > 0" :size="14" /><ArrowDownRight v-else-if="Number(row.growth_rate) < 0" :size="14" />{{ (Number(row.growth_rate) * 100).toFixed(1) }}%
                </span>
                <span v-else>{{ row.growth_status === 'zero_baseline' ? '基期为零' : '—' }}</span>
              </td>
            </tr>
          </tbody>
        </table>
        <table v-else class="oa-table">
          <thead><tr><th>商品 / SKU</th><th>店铺</th><th class="oa-number">销量</th><th class="oa-number">商品销售额</th><th class="oa-number">成交核实成本</th><th class="oa-number">核实毛利</th><th class="oa-number">核实覆盖率</th><th v-if="mode === 'detail'" class="oa-number">当前维护单价</th></tr></thead>
          <tbody><tr v-for="(row, index) in displayRows" :key="`${row.shop_id}-${row.shop_item_id}-${row.sku}-${index}`">
            <td>
              <RouterLink v-if="mode !== 'detail' && row.sku" class="commerce-product-name" :to="productLink(row.sku, row.shop_id)" :title="row.product_name || row.shop_item_id || row.sku">{{ row.product_name || row.shop_item_id || row.sku }}</RouterLink>
              <b v-else class="oa-ellipsis" :title="row.product_name || row.shop_item_id || row.sku">{{ row.product_name || row.shop_item_id || row.sku || '—' }}</b>
              <small v-if="row.sku" class="commerce-sku">{{ row.sku }} <button type="button" class="workspace-copy" :aria-label="'复制 SKU ' + row.sku" title="复制 SKU" @click="copySKU(row.sku)"><Copy :size="14" /></button></small>
            </td>
            <td>{{ shopName(row.shop_id) }}</td><td class="oa-number">{{ row.quantity ?? '—' }}</td><td class="oa-number">{{ money(row.item_pay_total) }}</td>
            <td class="oa-number">{{ money(row.cost_verified_total) }}</td><td class="oa-number">{{ money(row.verified_gross) }}</td>
            <td class="oa-number"><span v-if="row.verified_coverage != null" :class="['oa-pill', { 'oa-pill-warn': Number(row.verified_coverage) < .9 }]">{{ Number(row.verified_coverage) < .9 ? '仅覆盖 ' : '' }}{{ (Number(row.verified_coverage) * 100).toFixed(1) }}%</span><span v-else>—</span></td>
            <td v-if="mode === 'detail'" class="oa-number">{{ money(data.current_cost) }}</td>
          </tr></tbody>
        </table>
      </div>
      <div v-if="mode !== 'overview' && (data.has_more || offset)" class="oa-actions workspace-pager" aria-label="列表分页"><span>第 {{ Math.floor(offset / 100) + 1 }} 页</span><button :disabled="!offset || busy" @click="offset = Math.max(0, offset - 100); load(false)"><ChevronLeft :size="15" />上一页</button><button :disabled="!data.has_more || busy" @click="offset += 100; load(false)">下一页<ChevronRight :size="15" /></button></div>
    </section>
    <section v-if="mode === 'overview' && summary && canReview" class="oa-panel commerce-shortcuts">
        <div class="commerce-panel-heading"><h2>办理与复盘</h2></div>
        <RouterLink v-if="permissions.hasMenu('task_list')" to="/tasks"><ClipboardList :size="17" /><span>任务运营与设计交付</span><ArrowUpRight :size="15" /></RouterLink>
        <RouterLink v-if="can('pricing.view')" :to="{ path: '/operations/pricing', query: { sku: productId } }"><Package :size="17" /><span>SKU 核价</span><ArrowUpRight :size="15" /></RouterLink>
        <RouterLink v-if="permissions.hasMenu('cost_rules')" to="/cost-rules"><Package :size="17" /><span>SKU 成本维护</span><ArrowUpRight :size="15" /></RouterLink>
        <RouterLink v-if="can('finance.apply')" to="/fees/mine"><Wallet :size="17" /><span>我的费用申请</span><ArrowUpRight :size="15" /></RouterLink>
    </section>
    <template v-if="mode === 'detail' && data">
      <div class="commerce-insights commerce-detail-layout">
        <section class="oa-panel oa-link-list">
          <h2>关联任务</h2>
          <RouterLink v-for="task in data.tasks || []" :key="task.id" :to="'/tasks/' + task.id">{{ task.task_no || task.id }} <ArrowUpRight :size="15" /></RouterLink>
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
        <div class="commerce-panel-heading">
          <h2>历史成交成本</h2>
          <button :disabled="busy || historyBusy || !rows.length" @click="historyOpen && !historyError ? historyOpen = false : loadHistory()"><FileText :size="15" />{{ historyBusy ? '读取中…' : historyError ? '重新读取' : historyOpen ? '收起成本记录' : '展开成本记录' }}</button>
        </div>
        <p v-if="historyError" class="oa-error">{{ historyError }}</p>
        <p v-else-if="historyBusy" class="oa-empty" role="status">正在读取成本记录…</p>
        <div v-else-if="historyOpen && history.length" class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" :aria-label="title + '数据表'"><table class="oa-table"><thead><tr><th class="oa-number">下单日</th><th>店铺</th><th>订单子行</th><th class="oa-number">记录成本</th><th>成本档位</th><th>可信性</th></tr></thead><tbody><tr v-for="(row, index) in history" :key="index"><td>{{ row.order_time || '—' }}</td><td>{{ shopName(String(row.shop_id || '')) }}</td><td>{{ row.sub_order_id || '—' }}</td><td>{{ money(String(row.cost_amount ?? '')) }}</td><td>{{ row.cost_amount == null ? '未知' : costTierLabels[String(row.cost_tier)] || '未分类' }}</td><td><span v-if="implausibleCost(row.cost_implausible)" class="oa-pill oa-pill-warn">不可信 · 已排除</span><span v-else-if="row.cost_tier === 'verified' && row.cost_amount != null">计入核实毛利</span><span v-else>未计入核实毛利</span></td></tr></tbody></table></div>
        <p v-else-if="historyOpen" class="oa-empty">当前范围暂无历史成本记录</p>
      </section>
    </template>
  </template>
</div>
</template>

<style scoped src="./commerce-workspace.css" />
