<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import http from '@/services/http'
import { usePermission } from '@/composables/usePermission'
import { errorText } from '../api'
import { useOAAccess } from '../access'
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
const { can } = useOAAccess()
const { canAccessMenu } = usePermission()
const codes = ref(typeof route.query.sku === 'string' ? route.query.sku : '')
const parsed = computed(() => [...new Set(codes.value.split(/[\r\n,，、;；]+/).map(code => code.trim().toUpperCase()).filter(Boolean))])
const rows = ref<Row[]>([])
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
  rows.value = []
  busy.value = false
  error.value = ''
}

watch(() => route.query.sku, (sku) => {
  codes.value = typeof sku === 'string' ? sku : ''
  changedInput()
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
    if (version === requestVersion) rows.value = response.data.data.rows
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
    <PageHead title="SKU 核价" subtitle="按编码查看成本中心的当前成本、计价依据和规则试算结果。">
      <RouterLink v-if="can('orders.view')" class="oa-button" to="/operations/products">商品表现</RouterLink>
      <RouterLink v-if="canOpenMaintenance" class="oa-button" :to="{ name: 'CostRules' }">编码成本维护</RouterLink>
    </PageHead>
    <OperationNav v-if="can('orders.view')" />
    <div class="pricing-body" tabindex="0" role="region" aria-label="SKU 核价工作区">
    <section class="pricing-panel yb-page-surface" aria-labelledby="price-request-title">
      <h2 id="price-request-title">输入商品编码</h2>
      <form class="code-form" @submit.prevent="preview">
        <label for="pricing-codes" class="oa-muted">支持换行或逗号分隔，每批最多 30 个不同编码。</label>
        <textarea id="pricing-codes" v-model="codes" rows="3" spellcheck="false" :disabled="busy" @input="changedInput" />
        <div class="pricing-actions">
          <button type="submit" class="oa-button-primary" :disabled="busy">{{ busy ? '核对中…' : '查询成本与建议' }}</button>
          <span class="oa-muted">{{ parsed.length }} / 30 个编码</span>
        </div>
      </form>
    </section>

    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <section v-if="rows.length" class="pricing-panel yb-page-surface" aria-labelledby="price-results-title">
      <div class="pricing-results-head">
        <h2 id="price-results-title">核价结果</h2>
        <p class="oa-muted">共 {{ rows.length }} 个编码。规则试算供核对。维护正式成本请打开对应的编码成本记录。</p>
      </div>
      <div class="oa-table-wrap" tabindex="0" role="region" aria-label="逐项核价结果">
        <table class="oa-table pricing-table">
          <thead><tr><th scope="col">编码 / 商品</th><th scope="col" class="oa-number">当前成本</th><th scope="col" class="oa-number">试算建议</th><th scope="col">状态与依据</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.sku_code">
              <td>
                <strong>{{ row.sku_code }}</strong>
                <span v-if="row.product_name" class="oa-muted pricing-detail">{{ row.product_name }}</span>
                <RouterLink
                  v-if="canOpenMaintenance && row.sku_item_id"
                  :to="{ name: 'CostRules', query: { sku: row.sku_code } }"
                  :aria-label="'查看编码成本 ' + row.sku_code"
                >查看编码成本</RouterLink>
              </td>
              <td class="oa-number">
                {{ row.sku_item_id && row.current_cost == null ? '未记录' : price(row.current_cost) }}
                <span v-if="row.sku_item_id" class="oa-muted pricing-detail">成本中心记录</span>
              </td>
              <td class="oa-number">{{ price(row.suggested_cost) }}</td>
              <td class="pricing-state">
                <span class="oa-pill" :class="{ 'oa-pill-warn': row.state !== 'ready' }">{{ labels[row.state] }}</span>
                <p v-if="row.reason">{{ row.reason }}</p>
                <details v-if="row.rule || row.input || row.calculation">
                  <summary>查看计价依据</summary>
                  <dl>
                    <div v-if="row.rule"><dt>规则</dt><dd>{{ row.rule.rule_name }}（#{{ row.rule.rule_id }}，v{{ row.rule.rule_version }}）</dd></div>
                    <div v-if="row.applied_rules && row.applied_rules.length > 1"><dt>参与计价</dt><dd>{{ row.applied_rules.map(rule => rule.rule_name + ' v' + rule.rule_version).join('、') }}</dd></div>
                    <div v-if="row.match_mode"><dt>绑定来源</dt><dd>{{ row.match_mode === 'binding_erp_i_id' ? 'ERP 款式编码' : row.match_mode === 'binding_product_i_id' ? '商品款式编码' : '未命中明确款式绑定' }}</dd></div>
                    <div v-if="row.input?.erp_i_id"><dt>ERP 款式</dt><dd>{{ row.input.erp_i_id }}</dd></div>
                    <div v-if="row.input?.product_i_id"><dt>商品款式</dt><dd>{{ row.input.product_i_id }}</dd></div>
                    <div v-if="row.input?.width != null"><dt>宽</dt><dd>{{ row.input.width }} m</dd></div>
                    <div v-if="row.input?.height != null"><dt>高</dt><dd>{{ row.input.height }} m</dd></div>
                    <div v-if="row.input?.area != null"><dt>面积</dt><dd>{{ row.input.area }} m²</dd></div>
                    <div v-if="row.input?.quantity != null"><dt>数量</dt><dd>{{ row.input.quantity }}</dd></div>
                    <div v-if="row.input?.process"><dt>工艺</dt><dd>{{ row.input.process }}</dd></div>
                    <div v-if="row.input?.notes"><dt>规格</dt><dd>{{ row.input.notes }}</dd></div>
                    <div v-if="row.input?.input?.area_mode"><dt>面积方式</dt><dd>{{ row.input.input.area_mode === 'faces' ? '多面用料' : row.input.input.area_mode === 'total' ? '整套总面积' : '按尺寸' }}</dd></div>
                    <div v-if="row.calculation?.billable_quantity"><dt>有效计费量</dt><dd>{{ row.calculation.billable_quantity }}</dd></div>
                    <div v-for="(line, index) in row.calculation?.lines" :key="index"><dt>{{ line.name }}</dt><dd>{{ line.quantity }} {{ line.unit }} · {{ price(line.amount) }}</dd></div>
                    <div v-if="row.calculation?.missing?.length"><dt>待补输入</dt><dd>{{ row.calculation.missing.join('、') }}</dd></div>
                  </dl>
                </details>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
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
.pricing-panel { border-radius: 8px; padding: 16px; min-width: 0; }
.code-form { display: grid; gap: 8px; max-width: 640px; margin-top: 8px; }
.code-form textarea { width: 100%; resize: vertical; }
.pricing-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.pricing-results-head { margin-bottom: 12px; }
.pricing-table { min-width: 650px; }
.pricing-table td { vertical-align: top; }
.pricing-detail { display: block; margin-top: 4px; }
.pricing-state { width: 44%; white-space: normal; }
.pricing-state p { margin-top: 4px; overflow-wrap: anywhere; }
.pricing-state dl { margin: 0; }
.pricing-state dl div { display: grid; grid-template-columns: 7em 1fr; gap: 6px; padding-block: 2px; }
.pricing-state dt { color: var(--oa-muted); }
.pricing-state dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
</style>
