<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import http from '@/services/http'
import { errorText } from '../api'
import { useOAAccess } from '../access'
import OperationNav from '../components/OperationNav.vue'
import PageHead from '../components/PageHead.vue'

type Rule = { rule_id: number; rule_name: string; rule_version: number }
type Basis = { erp_i_id?: string; product_i_id?: string; width?: number; height?: number; area?: number; quantity?: number; process?: string; notes?: string; input?: { area_mode?: string } }
type Calculation = { area_m2: number; billable_quantity: number; missing?: string[]; lines?: { name: string; quantity: number; unit: string; amount: number }[] }
type Row = {
  sku_code: string; product_name?: string; task_id?: number; current_cost: number | null; suggested_cost: number | null;
  rule?: Rule; applied_rules?: Rule[]; calculation?: Calculation; input?: Basis; match_mode?: string; state: 'ready' | 'confirmed' | 'protected' | 'needs_review' | 'not_found' | 'ambiguous' | 'unavailable' | 'incomplete';
  reason: string; token?: string;
  review?: { batch_id: number; operator: string; reason: string; confirmed_at: string; accepted_cost: number; evidence: Row };
}
type Batch = { batch_id: number; rows: Row[] }
type ConfirmRequest = { request_id: string; reason: string; items: { sku_code: string; token: string }[] }

const route = useRoute()
const { can } = useOAAccess()
const codes = ref(typeof route.query.sku === 'string' ? route.query.sku : '')
const parsed = computed(() => [...new Set(codes.value.split(/[\r\n,，、;；]+/).map(code => code.trim()).filter(Boolean))])
const rows = ref<Row[]>([])
const checked = ref<string[]>([])
const selected = computed(() => rows.value.filter(row => row.state === 'ready' && row.token && checked.value.includes(row.sku_code)))
const ready = computed(() => rows.value.filter(row => row.state === 'ready').length)
const busy = ref(false)
const error = ref('')
const receipt = ref<Batch>()
const dialog = ref<HTMLDialogElement>()
const dialogOpen = ref(false)
const reason = ref('')
const pending = ref<ConfirmRequest>()
const stale = ref(false)
const confirmAllowed = computed(() => can('pricing.confirm'))
const labels: Record<Row['state'], string> = {
  ready: '可核价', confirmed: 'OA 已核价', protected: '已保护', needs_review: '待补依据',
  not_found: '找不到编码', ambiguous: '编码冲突', unavailable: '暂不可用', incomplete: '待缩小范围',
}
const amount = new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', minimumFractionDigits: 2, maximumFractionDigits: 4 })
const price = (value: number | null | undefined) => value == null ? '—' : amount.format(value)
const date = new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', dateStyle: 'medium', timeStyle: 'short' })
const time = (value: string) => date.format(new Date(value))
const validSku = (code: string) => [...code].length <= 128 && !/[\x00-\x1f\x7f]/.test(code)
const post = async <T,>(path: string, body: object) =>
  (await http.post<{ data: T }>('/oa-api/pricing/' + path, body, { timeout: 60_000 })).data.data

async function preview() {
  if (busy.value || pending.value) return
  error.value = ''
  if (!parsed.value.length || parsed.value.length > 30 || parsed.value.some(code => !validSku(code))) {
    error.value = '请输入 1 到 30 个有效的商品编码。'
    return
  }
  busy.value = true
  rows.value = []
  checked.value = []
  receipt.value = undefined
  stale.value = false
  try {
    rows.value = (await post<{ rows: Row[] }>('preview', { sku_codes: parsed.value })).rows
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

function changedInput() {
  rows.value = []
  checked.value = []
  receipt.value = undefined
  error.value = ''
}

function openConfirm() {
  if (!selected.value.length || !confirmAllowed.value || stale.value || busy.value) return
  error.value = ''
  reason.value = ''
  dialog.value?.showModal()
  dialogOpen.value = true
}

function closeConfirm() {
  if (!busy.value && !pending.value) dialog.value?.close()
}

async function confirm() {
  if (busy.value || !confirmAllowed.value || stale.value) return
  if (!pending.value) {
    if (!reason.value.trim() || !selected.value.length) return
    pending.value = {
      request_id: crypto.randomUUID(),
      reason: reason.value.trim(),
      items: selected.value.map(row => ({ sku_code: row.sku_code, token: row.token! })),
    }
  }
  error.value = ''
  busy.value = true
  try {
    const saved = await post<Batch>('confirm', pending.value)
    const savedRows = new Map(saved.rows.map(row => [row.sku_code, row]))
    rows.value = rows.value.map(row => savedRows.get(row.sku_code) ?? row)
    receipt.value = saved
    checked.value = []
    pending.value = undefined
    dialog.value?.close()
  } catch (e) {
    const status = (e as { response?: { status?: number } }).response?.status
    error.value = errorText(e)
    if (status != null && status >= 400 && status < 500) {
      pending.value = undefined
      if (status === 409) {
        stale.value = true
        checked.value = []
        dialog.value?.close()
      }
    }
  } finally {
    busy.value = false
  }
}
function preventCancel(event: Event) {
  if (busy.value || pending.value) event.preventDefault()
}
</script>

<template>
  <main class="oa-page oa-pricing">
    <PageHead title="SKU 核价" subtitle="按编码查看主站当前成本、绑定规则和用料依据。确认后保存 OA 核价记录。">
      <RouterLink v-if="can('orders.view')" class="oa-button" to="/operations/products">商品表现</RouterLink>
    </PageHead>
    <OperationNav v-if="can('orders.view')" />
    <section class="pricing-panel yb-page-surface" aria-labelledby="price-request-title">
      <h2 id="price-request-title">输入商品编码</h2>
      <form class="code-form" @submit.prevent="preview">
        <label for="pricing-codes" class="oa-muted">支持换行或逗号分隔，每批最多 30 个不同编码。</label>
        <textarea id="pricing-codes" v-model="codes" rows="3" spellcheck="false" :disabled="busy || Boolean(pending)" @input="changedInput" />
        <div class="pricing-actions">
          <button type="submit" class="oa-button-primary" :disabled="busy || Boolean(pending)">{{ busy && !pending ? '核对中…' : '查询建议价' }}</button>
          <span class="oa-muted">{{ parsed.length }} / 30 个编码</span>
        </div>
      </form>
    </section>

    <p v-if="error && !dialogOpen" class="pricing-error" role="alert">{{ error }}</p>
    <p v-if="receipt" class="pricing-receipt" role="status">已保存批次 #{{ receipt.batch_id }}，共 {{ receipt.rows.length }} 条核价记录。</p>

    <section v-if="rows.length" class="pricing-panel yb-page-surface" aria-labelledby="price-results-title">
      <div class="pricing-results-head">
        <div><h2 id="price-results-title">逐项复核</h2><p class="oa-muted">{{ rows.length }} 个编码，{{ ready }} 个可确认，已选 {{ selected.length }} 个。</p></div>
        <button v-if="confirmAllowed" type="button" class="oa-button-primary" :disabled="busy || stale || !selected.length" @click="openConfirm">复核并确认</button>
      </div>
      <div class="oa-table-wrap" tabindex="0" role="region" aria-label="逐项核价结果">
        <table class="oa-table pricing-table">
          <thead><tr><th v-if="confirmAllowed" scope="col">选择</th><th scope="col">编码 / 商品</th><th scope="col">主站现价</th><th scope="col">建议价</th><th scope="col">状态与依据</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.sku_code">
              <td v-if="confirmAllowed"><input v-if="row.state === 'ready'" v-model="checked" type="checkbox" :value="row.sku_code" :aria-label="'选择 ' + row.sku_code" :disabled="busy || stale"></td>
              <td><strong>{{ row.sku_code }}</strong><span v-if="row.product_name" class="oa-muted pricing-name">{{ row.product_name }}</span></td>
              <td class="oa-number">{{ price(row.current_cost) }}</td>
              <td class="oa-number">{{ price(row.suggested_cost) }}</td>
              <td class="pricing-state">
                <span class="oa-pill" :class="{ 'oa-pill-warn': !['ready', 'confirmed'].includes(row.state) }">{{ labels[row.state] }}</span>
                <p v-if="row.reason && row.state !== 'confirmed'">{{ row.reason }}</p>
                <template v-if="row.review">
                  <p>{{ row.state === 'confirmed' ? '采纳' : '上次采纳' }} {{ price(row.review.accepted_cost) }} · 批次 #{{ row.review.batch_id }}</p>
                  <p>{{ row.review.operator }} · <time :datetime="row.review.confirmed_at" title="北京时间">{{ time(row.review.confirmed_at) }}</time></p>
                  <p>{{ row.review.reason }}</p>
                </template>
                <details v-if="row.input || row.review?.evidence?.input">
                  <summary>查看计价依据</summary>
                  <template v-for="basis in [row.state === 'confirmed' || !row.input ? row.review?.evidence ?? row : row]" :key="basis.sku_code">
                    <dl>
                      <div v-if="basis !== row"><dt>核价时主站现价</dt><dd>{{ price(basis.current_cost) }}</dd></div>
                      <div v-if="basis.rule"><dt>规则</dt><dd>{{ basis.rule.rule_name }}（#{{ basis.rule.rule_id }}，v{{ basis.rule.rule_version }}）</dd></div>
                      <div v-if="basis.applied_rules && basis.applied_rules.length > 1"><dt>参与计价</dt><dd>{{ basis.applied_rules.map(rule => rule.rule_name + ' v' + rule.rule_version).join('、') }}</dd></div>
                      <div v-if="basis.match_mode"><dt>绑定来源</dt><dd>{{ basis.match_mode === 'binding_erp_i_id' ? 'ERP 款式编码' : basis.match_mode === 'binding_product_i_id' ? '商品款式编码' : '未命中' }}</dd></div>
                      <div v-if="basis.input?.erp_i_id"><dt>ERP 款式</dt><dd>{{ basis.input.erp_i_id }}</dd></div>
                      <div v-if="basis.input?.product_i_id"><dt>商品款式</dt><dd>{{ basis.input.product_i_id }}</dd></div>
                      <div v-if="basis.input?.width != null"><dt>宽</dt><dd>{{ basis.input.width }} m</dd></div>
                      <div v-if="basis.input?.height != null"><dt>高</dt><dd>{{ basis.input.height }} m</dd></div>
                      <div v-if="basis.input?.area != null"><dt>面积</dt><dd>{{ basis.input.area }} m²</dd></div>
                      <div v-if="basis.input?.quantity != null"><dt>数量</dt><dd>{{ basis.input.quantity }}</dd></div>
                      <div v-if="basis.input?.process"><dt>工艺</dt><dd>{{ basis.input.process }}</dd></div>
                      <div v-if="basis.input?.notes"><dt>规格</dt><dd>{{ basis.input.notes }}</dd></div>
                      <div v-if="basis.calculation?.billable_quantity"><dt>有效计费量</dt><dd>{{ basis.calculation.billable_quantity }}</dd></div>
                      <div v-for="(line, index) in basis.calculation?.lines" :key="index"><dt>{{ line.name }}</dt><dd>{{ line.quantity }} {{ line.unit }} · {{ price(line.amount) }}</dd></div>
                      <div v-if="basis.calculation?.missing?.length"><dt>待补输入</dt><dd>{{ basis.calculation.missing.join('、') }}</dd></div>
                      <div v-if="basis !== row && basis.reason"><dt>试算说明</dt><dd>{{ basis.reason }}</dd></div>
                    </dl>
                  </template>
                </details>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <dialog ref="dialog" class="pricing-dialog" aria-labelledby="pricing-confirm-title" @cancel="preventCancel" @close="dialogOpen = false">
      <form @submit.prevent="confirm">
        <h2 id="pricing-confirm-title">确认保存 {{ selected.length }} 条 OA 核价记录</h2>
        <p class="oa-muted">保存以下采纳价、主站现价和计价依据。主站成本、ERP 和结算仍沿用各自流程。</p>
        <ul><li v-for="row in selected" :key="row.sku_code"><span>{{ row.sku_code }}</span><strong>{{ price(row.suggested_cost) }}</strong></li></ul>
        <label for="pricing-reason">核价说明</label>
        <textarea id="pricing-reason" v-model="reason" maxlength="200" rows="2" required :disabled="busy || Boolean(pending)" />
        <p v-if="error" class="pricing-error" role="alert">{{ error }}</p>
        <div class="pricing-actions">
          <button type="button" :disabled="busy || Boolean(pending)" @click="closeConfirm">取消</button>
          <button type="submit" class="oa-button-primary" :disabled="busy || !reason.trim()">{{ busy ? '保存中…' : pending ? '重试同一批次' : '确认保存核价' }}</button>
        </div>
      </form>
    </dialog>
  </main>
</template>

<style scoped>
.oa-pricing { display: grid; gap: 12px; min-width: 0; }
.oa-pricing :deep(.oa-page-head) { margin-bottom: 0; }
.pricing-panel { border-radius: 8px; padding: 16px; min-width: 0; }
.code-form { display: grid; gap: 8px; max-width: 640px; margin-top: 8px; }
.code-form textarea, .pricing-dialog textarea { width: 100%; resize: vertical; }
.pricing-actions, .pricing-results-head { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.pricing-results-head { justify-content: space-between; margin-bottom: 12px; }
.pricing-table { min-width: 650px; }
.pricing-table td { vertical-align: top; }
.pricing-name { display: block; margin-top: 4px; }
.pricing-state { width: 44%; }
.pricing-state p { margin-top: 4px; overflow-wrap: anywhere; }
.pricing-state dl { margin: 0; }
.pricing-state dl div { display: grid; grid-template-columns: 7em 1fr; gap: 6px; padding-block: 2px; }
.pricing-state dt { color: var(--oa-muted); }
.pricing-state dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
.oa-page .pricing-error, .oa-page .pricing-receipt { color: var(--oa-ink); border: 1px solid var(--oa-line); padding: 8px 12px; border-radius: 6px; }
.pricing-panel.pricing-error { border-inline-start: 4px solid var(--oa-warning); }
.pricing-panel.pricing-receipt { border-inline-start: 4px solid var(--oa-accent); }
.pricing-dialog { max-width: min(520px, 92vw); width: 100%; max-height: 90dvh; overflow: auto; border: 1px solid var(--oa-line); border-radius: 8px; padding: 20px; color: var(--oa-ink); background: rgb(var(--yb-surface)); }
.pricing-dialog::backdrop { background: rgb(var(--yb-overlay-night) / .42); }
.pricing-dialog form { display: grid; gap: 12px; }
.pricing-dialog ul { display: grid; max-height: 240px; overflow-y: auto; margin: 0; padding: 0; list-style: none; }
.pricing-dialog li { display: flex; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--oa-line); padding: 6px 2px; }
.pricing-dialog .pricing-actions { justify-content: flex-end; }
</style>
