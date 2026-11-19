<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import http from '@/services/http'
import { usePermission } from '@/composables/usePermission'
import { errorText } from '../api'
import { useOAAccess } from '../access'
import { oaReturnLabel } from '@/oa/pageTitle'
import OAPageReturn from '../components/OAPageReturn.vue'
import { orderReturnQuery, orderReturnSource, orderReturnTarget, orderVisit } from '../orderFilterMemory'
import OperationNav from '../components/OperationNav.vue'
import PageHead from '../components/PageHead.vue'
import '../styles.css'

type Rule = { rule_id: number; rule_name: string; rule_version: number }
type Basis = { erp_i_id?: string; product_i_id?: string; width?: number; height?: number; area?: number; quantity?: number; process?: string; notes?: string; input?: { area_mode?: string } }
type Calculation = { area_m2: number; billable_quantity: number; missing?: string[]; lines?: { name: string; quantity: number; unit: string; amount: number }[] }
type Row = {
  sku_code: string; product_name?: string; sku_item_id?: number; current_cost: number | null; suggested_cost: number | null;
  rule?: Rule; applied_rules?: Rule[]; calculation?: Calculation; input?: Basis; match_mode?: string; state: 'ready' | 'protected' | 'needs_review' | 'not_found' | 'ambiguous' | 'unavailable' | 'incomplete';
  reason: string;
}

const route = useRoute()
const router = useRouter()
const returnTarget = computed(() => orderReturnTarget(route))
const returnSource = computed(() => orderReturnSource(route))
type Visit = { codes: string; queried: boolean; row?: string; closed: boolean; scroll: number }
let visit = orderVisit<Visit>('pricing')
const previous = visit.read()
const resultRegion = ref<HTMLElement>()
let restoring = Boolean(previous)
const queried = ref(previous?.queried ?? false)
function saveVisit() {
  if (!restoring) visit.save({ codes: codes.value, queried: queried.value, row: selected.value?.sku_code, closed, scroll: resultRegion.value?.scrollTop ?? 0 })
}
function costLink(sku?: string) {
  const query = Object.fromEntries(Object.entries(route.query).filter(([key, value]) =>
    ['returnTo', 'returnFallback', 'sku'].includes(key) && typeof value === 'string')) as Record<string, string>
  if (codes.value && codes.value.length <= 500) query.sku = codes.value
  return { name: 'CostRules', query: { ...(sku ? { sku, focus: 'cost-detail' } : {}),
    ...orderReturnQuery(router, route.path, query, route.fullPath) } }
}
const { can } = useOAAccess()
const { canAccessMenu } = usePermission()
const codes = ref(previous?.codes ?? (typeof route.query.sku === 'string' ? route.query.sku : ''))
const parsed = computed(() => [...new Set(codes.value.split(/[\r\n,，、;；]+/).map(code => code.trim().toUpperCase()).filter(Boolean))])
const rows = ref<Row[]>([])
const selected = ref<Row>()
const inspectorHeading = ref<HTMLElement>()
let viewport: MediaQueryList | undefined
let closed = previous?.closed ?? false
watch([codes, selected, queried], saveVisit, { flush: 'post' })
function select(row: Row, focus = true) {
  selected.value = row
  if (focus) void nextTick(() => inspectorHeading.value?.focus())
}
function openDefault() {
  if (!restoring && viewport?.matches && !closed && !selected.value && rows.value[0]) select(rows.value[0], false)
}
function close() {
  const row = selected.value
  closed = true
  selected.value = undefined
  if (row) void nextTick(() => rowButtons.get(row.sku_code)?.focus())
}
const rowButtons = new Map<string, HTMLButtonElement>()
function bindRowButton(sku: string, element: unknown) {
  if (element instanceof HTMLButtonElement) rowButtons.set(sku, element)
  else rowButtons.delete(sku)
}
function rowKey(event: KeyboardEvent) {
  if (!selected.value || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return
  if (event.key === 'Escape') {
    close()
    return
  }
  if ((event.target as HTMLElement)?.closest('input, textarea, select') || !['ArrowUp', 'ArrowDown'].includes(event.key)) return
  event.preventDefault()
  const target = rows.value[rows.value.indexOf(selected.value) + (event.key === 'ArrowUp' ? -1 : 1)]
  if (target) {
    rowButtons.get(target.sku_code)?.scrollIntoView({ block: 'nearest' })
    select(target)
  }
}
watch(rows, () => {
  if (!rows.value.includes(selected.value!)) selected.value = undefined
  openDefault()
})
onMounted(() => {
  viewport = window.matchMedia?.('(min-width: 2300px)')
  viewport?.addEventListener('change', openDefault)
  void restorePreview(previous)
})
async function restorePreview(saved?: Visit) {
  const activeVisit = visit
  if (saved?.queried) {
    await preview()
    await nextTick()
    if (activeVisit !== visit || route.path !== '/operations/pricing') return
    const row = rows.value.find(item => item.sku_code === saved.row)
    if (row) select(row, false)
    if (resultRegion.value) resultRegion.value.scrollTop = saved.scroll
  }
  restoring = false
}
onUnmounted(() => viewport?.removeEventListener('change', openDefault))
const busy = ref(false)
const error = ref('')
const canOpenMaintenance = computed(() => canAccessMenu('cost_rules'))
const labels: Record<Row['state'], string> = {
  ready: '已有试算建议', protected: '当前成本受保护', needs_review: '缺少计价依据',
  not_found: '找不到编码', ambiguous: '编码冲突', unavailable: '暂不可用', incomplete: '待缩小范围',
}
const amount = new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', minimumFractionDigits: 2, maximumFractionDigits: 4 })
const price = (value: number | null | undefined) => value == null ? '—' : amount.format(value)
const validSku = (code: string) => [...code].length <= 128 && !/\p{Cc}/u.test(code)
let requestVersion = 0

function changedInput() {
  requestVersion++
  queried.value = false
  rows.value = []
  busy.value = false
  error.value = ''
}

watch(() => route.fullPath, async () => {
  if (route.path !== '/operations/pricing') return
  visit = orderVisit<Visit>('pricing')
  const saved = visit.read()
  restoring = Boolean(saved)
  closed = saved?.closed ?? false
  codes.value = saved?.codes ?? (typeof route.query.sku === 'string' ? route.query.sku : '')
  changedInput()
  await restorePreview(saved)
})

async function preview() {
  if (busy.value) return
  error.value = ''
  if (!parsed.value.length || parsed.value.length > 30 || parsed.value.some(code => !validSku(code))) {
    error.value = '请输入 1 到 30 个有效的商品编码。'
    return
  }
  const version = ++requestVersion
  busy.value = true
  rows.value = []
  try {
    const response = await http.post<{ data: { rows: Row[] } }>('/oa-api/pricing/preview', { sku_codes: parsed.value }, { timeout: 60_000 })
    if (version === requestVersion) {
      rows.value = response.data.data.rows
      queried.value = true
    }
  } catch (e) {
    if (version === requestVersion) {
      const message = (e as { responseData?: { error?: unknown } })?.responseData?.error
      error.value = typeof message === 'string' ? message : errorText(e)
    }
  } finally {
    if (version === requestVersion) busy.value = false
  }
}
</script>

<template>
  <main class="oa-page oa-pricing">
    <OAPageReturn v-if="returnTarget" :to="returnTarget" :source="returnSource">{{ oaReturnLabel(returnTarget) }}</OAPageReturn>
    <PageHead title="SKU 核价" subtitle="按编码查看成本中心的当前成本、计价依据和规则试算结果。">
      <RouterLink v-if="canOpenMaintenance" class="oa-button" :to="costLink()">编码成本维护</RouterLink>
    </PageHead>
    <OperationNav v-if="can('orders.view')" />
    <div class="pricing-body" @keydown="rowKey" role="region" aria-label="SKU 核价工作区">
    <section class="pricing-panel yb-page-surface" aria-labelledby="price-request-title">
      <h2 id="price-request-title">输入商品编码</h2>
      <form class="code-form" @submit.prevent="preview">
        <label for="pricing-codes" class="oa-muted">支持换行或逗号分隔，每批最多 30 个不同编码。</label>
        <textarea id="pricing-codes" v-model="codes" rows="3" spellcheck="false" placeholder="每行一个编码，例如：HSC20454" :disabled="busy" @input="changedInput" />
        <div class="pricing-actions">
          <button type="submit" class="oa-button-primary" :disabled="busy">{{ busy ? '核对中…' : '查询成本与建议' }}</button>
          <span class="oa-muted">{{ parsed.length }} / 30 个编码</span>
        </div>
      </form>
    </section>

    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <div v-if="rows.length" class="pricing-workspace" :class="{ 'has-inspector': selected }">
    <section class="pricing-results pricing-panel yb-page-surface" aria-labelledby="price-results-title">
      <div class="pricing-results-head">
        <h2 id="price-results-title">核价结果</h2>
        <p class="oa-muted">共 {{ rows.length }} 个编码。规则试算供核对。维护正式成本请打开对应的编码成本记录。</p>
      </div>
      <div ref="resultRegion" class="oa-table-wrap" tabindex="0" @scroll="saveVisit" role="region" aria-label="逐项核价结果">
        <table class="oa-table pricing-table">
          <thead><tr><th scope="col">编码 / 商品</th><th scope="col" class="oa-number">当前成本</th><th scope="col" class="oa-number">试算建议</th><th scope="col">状态与依据</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.sku_code" :class="{ 'is-selected': selected === row }"
              @click="!($event.target as HTMLElement).closest('a,button') && select(row)">
              <td>
                <RouterLink v-if="canOpenMaintenance && row.sku_item_id"
                  :to="costLink(row.sku_code)" custom v-slot="{ href }">
                  <a :href="href" target="_blank" rel="noopener" @click.exact.prevent="select(row)">{{ row.sku_code }}</a>
                </RouterLink>
                <strong v-else>{{ row.sku_code }}</strong>
                <span v-if="row.product_name" class="oa-muted pricing-detail" :title="row.product_name">{{ row.product_name }}</span>
              </td>
              <td class="oa-number">{{ row.sku_item_id && row.current_cost == null ? '未记录' : price(row.current_cost) }}</td>
              <td class="oa-number">{{ price(row.suggested_cost) }}</td>
              <td class="pricing-state">
                <button :ref="(element) => bindRowButton(row.sku_code, element)"
                  type="button" class="oa-pill" :class="{ 'oa-pill-warn': row.state !== 'ready' }"
                  :aria-label="row.sku_code + ' · ' + labels[row.state] + ' · 查看依据'" @click="select(row)">{{ labels[row.state] }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <aside v-if="selected" class="pricing-inspector yb-page-surface" aria-labelledby="pricing-inspector-title">
      <header>
        <h2 id="pricing-inspector-title" tabindex="-1" ref="inspectorHeading">{{ selected.sku_code }}</h2>
        <button type="button" aria-label="关闭计价依据" @click="close">关闭</button>
      </header>
      <p v-if="selected.product_name" class="oa-muted">{{ selected.product_name }}</p>
      <p>{{ labels[selected.state] }}</p>
      <p v-if="selected.reason">{{ selected.reason }}</p>
      <RouterLink v-if="canOpenMaintenance && selected.sku_item_id" class="oa-button"
        :to="costLink(selected.sku_code)" target="_blank" rel="noopener">打开完整成本记录 ↗</RouterLink>
      <dl class="pricing-inspector-totals">
        <div><dt>当前成本</dt><dd>{{ selected.sku_item_id && selected.current_cost == null ? '未记录' : price(selected.current_cost) }}</dd></div>
        <div><dt>试算建议</dt><dd>{{ price(selected.suggested_cost) }}</dd></div>
      </dl>
      <section v-if="selected.rule || selected.input || selected.calculation">
        <h3>计价依据</h3>
        <dl>
          <div v-if="selected.rule"><dt>规则</dt><dd>{{ selected.rule.rule_name }}（#{{ selected.rule.rule_id }}，v{{ selected.rule.rule_version }}）</dd></div>
          <div v-if="selected.applied_rules && selected.applied_rules.length > 1"><dt>参与计价</dt><dd>{{ selected.applied_rules.map(rule => rule.rule_name + ' v' + rule.rule_version).join('、') }}</dd></div>
          <div v-if="selected.match_mode"><dt>绑定来源</dt><dd>{{ selected.match_mode === 'binding_erp_i_id' ? 'ERP 款式编码' : selected.match_mode === 'binding_product_i_id' ? '商品款式编码' : '未命中明确款式绑定' }}</dd></div>
          <div v-if="selected.input?.erp_i_id"><dt>ERP 款式</dt><dd>{{ selected.input.erp_i_id }}</dd></div>
          <div v-if="selected.input?.product_i_id"><dt>商品款式</dt><dd>{{ selected.input.product_i_id }}</dd></div>
          <div v-if="selected.input?.width != null"><dt>宽</dt><dd>{{ selected.input.width }} m</dd></div>
          <div v-if="selected.input?.height != null"><dt>高</dt><dd>{{ selected.input.height }} m</dd></div>
          <div v-if="selected.input?.area != null"><dt>面积</dt><dd>{{ selected.input.area }} m²</dd></div>
          <div v-if="selected.input?.quantity != null"><dt>数量</dt><dd>{{ selected.input.quantity }}</dd></div>
          <div v-if="selected.input?.process"><dt>工艺</dt><dd>{{ selected.input.process }}</dd></div>
          <div v-if="selected.input?.notes"><dt>规格</dt><dd>{{ selected.input.notes }}</dd></div>
          <div v-if="selected.input?.input?.area_mode"><dt>面积方式</dt><dd>{{ selected.input.input.area_mode === 'faces' ? '多面用料' : selected.input.input.area_mode === 'total' ? '整套总面积' : '按尺寸' }}</dd></div>
          <div v-if="selected.calculation?.billable_quantity"><dt>有效计费量</dt><dd>{{ selected.calculation.billable_quantity }}</dd></div>
          <div v-for="(line, index) in selected.calculation?.lines" :key="index"><dt>{{ line.name }}</dt><dd>{{ line.quantity }} {{ line.unit }} · {{ price(line.amount) }}</dd></div>
          <div v-if="selected.calculation?.missing?.length"><dt>待补输入</dt><dd>{{ selected.calculation.missing.join('、') }}</dd></div>
        </dl>
      </section>
    </aside>
    </div>
    </div>
  </main>
</template>

<style scoped>
.oa-pricing { display: grid; gap: 12px; min-width: 0; }
.oa-pricing :deep(.oa-page-head) { margin-bottom: 0; }
.pricing-body { display: grid; align-content: start; gap: 12px; min-inline-size: 0; }
@media (min-width: 1024px) {
  .oa-pricing { display: flex; flex-direction: column; block-size: 100%; min-block-size: 0; }
  .oa-pricing > :not(.pricing-body) { flex: none; }
  .pricing-body { min-block-size: 0; overflow: auto; scrollbar-gutter: stable; }
}
.pricing-panel { border-radius: 8px; padding: 12px 20px; min-width: 0; }
.code-form { display: grid; gap: 8px; max-width: 640px; margin-top: 8px; }
.code-form textarea { width: 100%; resize: vertical; }
.pricing-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.pricing-results-head { margin-bottom: 12px; }
.pricing-table { min-width: 650px; }
.pricing-table td { height: 50px; vertical-align: middle; }
.pricing-detail { display: block; margin-top: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pricing-state { width: 168px; white-space: normal; }
.pricing-state p { margin-top: 4px; overflow-wrap: anywhere; }
.pricing-inspector dl { margin: 0; }
.pricing-inspector dl div { display: grid; grid-template-columns: 7em 1fr; gap: 6px; padding-block: 2px; }
.pricing-inspector dt { color: var(--oa-muted); }
.pricing-inspector dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }

.pricing-workspace { min-width: 0; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr); }
.pricing-table { table-layout: fixed; }
.pricing-table th.oa-number { width: 120px; }
.pricing-table :is(td, th).oa-number { text-align: end; font-family: var(--yb-font-mono); font-variant-numeric: tabular-nums; }
.pricing-table tbody tr { cursor: pointer; }
.pricing-table tbody tr:hover > td { background: rgb(var(--yb-surface-hover)); }
.pricing-table tbody tr.is-selected > td { background: rgb(var(--yb-brand-soft)); }
.pricing-inspector {
  position: fixed; z-index: 12; inset: 44px 0 0 auto; width: min(400px,100vw);
  min-width: 0; min-height: 0; overflow: auto; padding: 16px 24px;
  background: var(--oa-surface); box-shadow: -8px 0 24px rgb(var(--yb-shadow) / .11);
}
.pricing-inspector header { display: flex; justify-content: space-between; align-items: start; gap: 8px; }
.pricing-inspector h2, .pricing-inspector p { overflow-wrap: anywhere; margin-block: 6px; }
.pricing-inspector .pricing-inspector-totals { margin-block: 12px; }
.pricing-inspector h3 { margin-block: 12px 4px; }
@media (min-width: 1280px) {
  .pricing-body { display: flex; flex-direction: column; flex: 1; overflow: hidden; scrollbar-gutter: auto; }
  .pricing-body > :not(.pricing-workspace) { flex: none; }
  .pricing-workspace { flex: 1; grid-template-rows: minmax(0, 1fr); gap: 12px; }
  .pricing-results { display: flex; flex-direction: column; min-height: 0; padding: 0; overflow: hidden; }
  .pricing-results-head { margin: 0; padding: 9px 12px; }
  .pricing-results > .oa-table-wrap { flex: 1; min-height: 0; overflow: auto; }
  .pricing-table th { position: sticky; inset-block-start: 0; z-index: 2; }
}
@media (min-width: 1920px) {
  .pricing-workspace.has-inspector { grid-template-columns: minmax(0, 1fr) 420px; }
  .pricing-inspector { position: static; width: auto; box-shadow: none; border-radius: 8px; }
}
@media (min-width: 2300px) {
  .pricing-workspace.has-inspector { grid-template-columns: minmax(0, 1fr) 480px; }
}
</style>
