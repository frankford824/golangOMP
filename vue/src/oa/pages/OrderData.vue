<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import FreshnessStatus from '../components/FreshnessStatus.vue'
import { api, dateInChina, errorText, money } from '../api'
import { useOAAccess } from '../access'
import '../styles.css'

const { currentProfile } = useOAAccess()

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
const today = dateInChina()
const from = ref(props.mode === 'ledger' ? today.slice(0, 7) + '-01' : today)
const to = ref(today)
const shop = ref('')
const platform = ref('')
const offset = ref(0)
let appliedQuery = ''
const data = ref<{ rows: Row[]; has_more?: boolean; stale?: boolean; freshness?: { source_committed_at?: string; lag_seconds?: number } }>()
const shopNames = ref<{ shop_id: string; shop_name: string }[]>([])
const busy = ref(false)
const error = ref('')
const allowed = computed(() => props.mode !== 'ledger' || !!currentProfile.value?.super_admin)
const copyStatus = ref('')
async function copyOrder(id: string) {
  try { await navigator.clipboard.writeText(id); copyStatus.value = '已复制订单号' }
  catch { copyStatus.value = '无法复制订单号，请手动复制' }
}
const title = computed(() => ({ detail: '订单明细', refunds: '售后退款', ledger: '经营台账' }[props.mode]))

async function load(next = 0, useApplied = false) {
  if (!allowed.value || busy.value) return
  busy.value = true
  data.value = undefined
  error.value = ''
  try {
    if (!useApplied || !appliedQuery) appliedQuery = new URLSearchParams({ from: from.value, to: to.value, shop_id: shop.value, platform: platform.value }).toString()
    const query = new URLSearchParams(appliedQuery)
    query.set('offset', String(next))
    data.value = await api('/oa-api/orders/' + props.mode + '?' + query)
    offset.value = next
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
function display(column: Column, row: Row) {
  const value = row[column.key]
  if (value == null || value === '') return '—'
  if (column.key === 'shop_id') return shopNames.value.find(item => item.shop_id === value)?.shop_name || value
  if (column.date && typeof value === 'string' && /^\d{4}-\d{2}-\d{2}/.test(value)) {
    const instant = /(?:Z|[+-]\d{2}:?\d{2})$/.test(value) ? new Date(value) : undefined
    if (instant && !Number.isNaN(instant.getTime())) return new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(instant)
    return value.replace('T', ' ').slice(0, 16)
  }
  if (column.money) return money(String(value))
  if (column.key.startsWith('is_')) return value === true || value === 1 || value === '1' ? '是' : '否'
  return value
}
onMounted(() => {
  void load()
  if (allowed.value) void api<{ rows: { shop_id: string; shop_name: string }[] }>('/oa-api/orders/shops').then(response => { shopNames.value = response.rows || [] }).catch(() => undefined)
})
</script>

<template>
<div class="oa-page commerce-workspace">
  <PageHead :title="title" :subtitle="mode === 'refunds' ? '按确认售后的申请日期' : mode === 'detail' ? '按下单时间与订单号倒序' : '按店铺、核算月份与核算状态'">
    <RouterLink class="oa-button" to="/operations/rankings">返回经营分析</RouterLink>
  </PageHead>
  <p v-if="!allowed" class="oa-error" role="alert">当前账号没有经营台账权限。</p>
  <template v-else>
    <form class="oa-actions oa-filter workspace-filters" @submit.prevent="load()">
      <label>店铺<select v-if="shopNames.length" v-model="shop" aria-label="店铺"><option value="">全部可见店铺</option><option v-for="item in shopNames" :value="item.shop_id" :key="item.shop_id">{{ item.shop_name || item.shop_id }}</option></select><input v-else v-model="shop" aria-label="店铺" placeholder="全部可见店铺" /></label>
      <details v-if="mode === 'detail'" class="workspace-more"><summary>更多筛选{{ platform ? ' · 1' : '' }}</summary><label>平台<input v-model.trim="platform" placeholder="平台编码" /></label></details>
      <label>开始日期<input v-model="from" type="date" :max="to" min="2026-06-01" required /></label>
      <label>结束日期<input v-model="to" type="date" :min="from" :max="today" required /></label>
      <button class="oa-button-primary" :disabled="busy">查询</button>
    </form>
    <FreshnessStatus :freshness="data?.freshness" :stale="data?.stale" />
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button @click="load()">重试</button></p>
    <p v-else-if="busy" class="oa-panel oa-empty" role="status">正在读取{{ title }}…</p>
    <section v-else-if="data" class="oa-panel"><p v-if="copyStatus" class="oa-muted" role="status">{{ copyStatus }}</p>
      <p v-if="!data.rows?.length" class="oa-empty">所选范围暂无{{ title }}数据</p>
      <div v-else class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" :aria-label="title + '数据表'">
        <table class="oa-table"><thead><tr><th v-for="column in columns[mode]" :key="column.key" :class="{ 'oa-number': column.money || column.date }">{{ column.label }}</th></tr></thead><tbody>
          <tr v-for="(row, index) in data.rows" :key="index"><td v-for="column in columns[mode]" :key="column.key" :class="{ 'oa-number': column.money || column.date }"><time v-if="column.date && row[column.key]" :datetime="String(row[column.key])">{{ display(column, row) }}</time><span v-else-if="['shop_id', 'name'].includes(column.key)" class="oa-ellipsis" :title="String(display(column, row))">{{ display(column, row) }}</span><template v-else>{{ display(column, row) }}<button v-if="column.key === 'online_order_no' && row[column.key]" type="button" class="workspace-copy" aria-label="复制平台订单号" @click="copyOrder(String(row[column.key]))">复制</button></template></td></tr>
        </tbody></table>
      </div>
      <div v-if="offset || data.has_more" class="oa-actions workspace-pager" aria-label="列表分页"><span>第 {{ Math.floor(offset / 100) + 1 }} 页</span><button :disabled="!offset" @click="load(Math.max(0, offset - 100), true)">上一页</button><button :disabled="!data.has_more" @click="load(offset + 100, true)">下一页</button></div>
    </section>
  </template>
</div>
</template>

<style scoped src="./order-workspace.css" />
