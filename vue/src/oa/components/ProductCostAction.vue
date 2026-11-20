<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import { usePermission } from '@/composables/usePermission'
import { getToken } from '@/services/http'
import { costMaintenanceApi, maintenanceStatuses, type MaintenanceRow } from '@/services/api/costMaintenanceApi'
import { costCoverageDefinition } from '../costCoverage'
import { useOAAccess } from '../access'
import { errorText, money, send } from '../api'
import { invalidateCostActivity } from '../costActivity'
export type CostTarget = {
  sku: string; shop: string; shopName: string; from: string; to: string; includeBrush: boolean
  productName: string; quantity?: string; sales?: string; coverage?: string; missingCostLines?: string; today: boolean
}
const { can, currentProfile } = useOAAccess()
const { can: hasNativePermission } = usePermission()
const mayReadCost = computed(() => hasNativePermission('catalog.view'))
const mayMaintain = computed(() => can('cost.request.handle'))
const props = defineProps<{ split?: boolean; detailLink?: RouteLocationRaw; pricingLink?: RouteLocationRaw }>()
const emit = defineEmits<{ dismiss: []; closed: []; saved: [message: string] }>()
const heading = ref<HTMLElement>()
const panel = ref<HTMLElement>()
const wide = ref(false)
const overlay = computed(() => !props.split || !wide.value)
let media: MediaQueryList | undefined
function syncViewport() {
  wide.value = media?.matches ?? false
}
onMounted(() => {
  media = window.matchMedia?.('(min-width: 1920px)')
  syncViewport()
  media?.addEventListener('change', syncViewport)
})
onUnmounted(() => {
  media?.removeEventListener('change', syncViewport)
  readController?.abort()
})
watch(panel, element => {
  if (element instanceof HTMLDialogElement && !element.open) {
    element.showModal()
    heading.value?.focus()
  }
}, { flush: 'post' })
function backdropClick(event: MouseEvent) {
  if (!overlay.value || event.target !== panel.value || !panel.value) return
  const { left, right, top, bottom } = panel.value.getBoundingClientRect()
  if (event.clientX < left || event.clientX > right || event.clientY < top || event.clientY > bottom) close()
}
const subject=ref<CostTarget>()
const reason=ref(''), message=ref(''), error=ref(''), busy=ref(false)
const currentCost = ref<MaintenanceRow>()
const loadingCost = ref(false)
const costReadError = ref('')
const staleCost = ref(false)
const editingPrice = ref(false)
const price = ref<number | string>('')
const priceReason = ref('')
const maintenanceMessage = ref('')
const canMaintainRow = computed(() => mayReadCost.value && mayMaintain.value && (currentCost.value?.revision ?? 0) >= 1)
const priceValue = computed(() => Number(price.value))
const priceChanged = computed(() => price.value !== '' && Number.isFinite(priceValue.value) &&
  priceValue.value >= 0 && priceValue.value <= 1e9 &&
  (currentCost.value?.cost_price == null ||
    Math.round(priceValue.value * 10_000) !== Math.round(currentCost.value.cost_price * 10_000)))
let readController: AbortController | undefined
let key = ''
const costAmount = (value?: number | null) => value == null ? '—' : '¥' + value.toString()

async function loadCurrentCost() {
  readController?.abort()
  currentCost.value = undefined
  costReadError.value = ''
  staleCost.value = false
  editingPrice.value = false
  error.value = ''
  const sku = subject.value?.sku
  loadingCost.value = Boolean(sku && mayReadCost.value)
  if (!sku || !mayReadCost.value) return
  const token = getToken()
  const controller = new AbortController()
  readController = controller
  try {
    let cursor = 0
    let match: MaintenanceRow | undefined
    // Keyword search also matches names. Only use the exact canonical SKU.
    for (let page = 0; page < 20; page++) {
      const result = await costMaintenanceApi.list({
        keyword: sku, review_state: 'all', page_size: 100, ...(cursor ? { before_id: cursor } : {}),
      }, controller.signal)
      if (controller.signal.aborted || token !== getToken()) return
      for (const row of result.data) {
        if (row.sku_code !== sku) continue
        if (match && match.id !== row.id) {
          costReadError.value = '成本中心有多条同码维护记录，请发起核价复核。'
          return
        }
        match = row
      }
      if (result.next_before_id === 0) {
        currentCost.value = match
        return
      }
      if (result.next_before_id < 0 || (cursor && result.next_before_id >= cursor)) break
      cursor = result.next_before_id
    }
    costReadError.value = '暂时无法定位编码成本，请刷新后重试。'
  } catch (cause) {
    if (!controller.signal.aborted && token === getToken()) costReadError.value = errorText(cause)
  } finally {
    if (!controller.signal.aborted && token === getToken()) loadingCost.value = false
  }
}

function editPrice() {
  if (busy.value || staleCost.value || !canMaintainRow.value || !currentCost.value) return
  price.value = String(currentCost.value.cost_price ?? '')
  priceReason.value = ''
  error.value = ''
  maintenanceMessage.value = ''
  editingPrice.value = true
}

async function maintain(action: 'confirm' | 'modify') {
  const row = currentCost.value
  const target = subject.value
  if (!row || !target || !canMaintainRow.value || busy.value || staleCost.value) return
  if (action === 'confirm' ? row.cost_price == null : !priceChanged.value || !priceReason.value.trim()) return
  const token = getToken()
  busy.value = true
  error.value = ''
  maintenanceMessage.value = ''
  try {
    const updated = await costMaintenanceApi.maintain(row.id, {
      action,
      token: row.token,
      revision: row.revision,
      erp_cost: row.erp_cost,
      ...(action === 'modify' ? { cost_price: priceValue.value, reason_code: 'cost_incorrect', reason_detail: priceReason.value.trim() } : {}),
    })
    invalidateCostActivity()
    if (target !== subject.value || token !== getToken()) return
    currentCost.value = updated
    editingPrice.value = false
    maintenanceMessage.value = '系统成本已' + (action === 'confirm' ? '确认' : '保存') + '，ERP 同步状态以回读结果为准。'
  } catch (cause) {
    if (target !== subject.value || token !== getToken()) return
    error.value = errorText(cause)
    staleCost.value = (cause as { status?: number } | null)?.status === 409
  } finally {
    busy.value = false
  }
}

function open(target: CostTarget, focus = true) {
  if (busy.value) return
  subject.value = target
  reason.value = Number(target.missingCostLines) > 0 ? `本组有 ${target.missingCostLines} 条订单行缺少成本，请核对该 SKU。` : ''
  message.value = ''
  error.value = ''
  maintenanceMessage.value = ''
  key = crypto.randomUUID()
  void loadCurrentCost()
  if (focus) void nextTick(() => heading.value?.focus())
}
function dismiss() {
  readController?.abort()
  subject.value = undefined
  emit('dismiss')
}
function close() {
  if (busy.value) return
  emit('closed')
  dismiss()
}
async function submit() {
 const row=subject.value
 if(!row || !reason.value.trim() || busy.value || message.value)return
 busy.value=true;error.value=''
 try {
  const ret=await send<{existing:boolean}>('/oa-api/orders/cost-requests',{
    sku:row.sku,shop_id:row.shop,from:row.from,to:row.to,include_brush:row.includeBrush,reason:reason.value.trim(),request_key:key,
  })
  if(row!==subject.value)return
  message.value=ret.existing?'已有这项核价请求，可在通知中心查看进展。':'已提交给成本维护人员，处理结果会发到你的通知中心。'
  invalidateCostActivity()
  busy.value = false
  emit('saved', message.value)
 }catch(cause){if(row===subject.value)error.value=errorText(cause)}
 finally{busy.value=false}
}
watch(() => currentProfile.value?.id, dismiss)
watch(mayReadCost, () => {
  if (subject.value) void loadCurrentCost()
})
defineExpose({ open, close, dismiss, pending: busy })
</script>
<template>
  <component :is="overlay ? 'dialog' : 'aside'" v-if="subject" ref="panel"
    class="sku-inspector" :class="{ 'sku-inspector-split': !overlay }" aria-labelledby="sku-review-title"
    @cancel.prevent="close" @click="backdropClick">
    <header>
      <h2 id="sku-review-title" ref="heading" tabindex="-1">核对 SKU 成本</h2>
      <button type="button" :disabled="busy" aria-label="关闭成本核对" @click="close">关闭</button>
    </header>
    <section class="sku-summary" aria-label="商品经营摘要">
      <h3>{{ subject.productName }}</h3>
      <p>{{ subject.sku }} · {{ subject.shopName }}</p>
      <p v-if="!subject.today">{{ subject.from === subject.to ? subject.from : subject.from + ' 至 ' + subject.to }}</p>
      <dl>
        <div><dt>{{ subject.today ? '今日销量' : '所选期间销量' }}</dt><dd>{{ subject.quantity ?? '—' }}</dd></div>
        <div><dt>{{ subject.today ? '今日销售额' : '所选期间销售额' }}</dt><dd>{{ money(subject.sales) }}</dd></div>
        <div><dt>成交成本核实覆盖率</dt><dd>{{ subject.coverage ?? '—' }}</dd></div>
      </dl>
    </section>
    <section class="sku-summary sku-current-cost" aria-label="当前编码成本">
      <h3>当前编码成本</h3>
      <p v-if="loadingCost" role="status">正在读取当前成本…</p>
      <p v-else-if="!mayReadCost">当前账号无编码成本查看权限。</p>
      <template v-else-if="costReadError">
        <p role="alert" class="oa-error">{{ costReadError }}</p>
        <button type="button" @click="loadCurrentCost">重新读取成本</button>
      </template>
      <template v-else>
        <dl>
          <div><dt>系统单位成本</dt><dd>{{ costAmount(currentCost?.cost_price) }}</dd></div>
          <div><dt>ERP 最近观测单位成本</dt><dd>{{ costAmount(currentCost?.erp_cost) }}</dd></div>
        </dl>
        <p v-if="currentCost">{{ maintenanceStatuses[currentCost.status] ?? currentCost.status }}</p>
        <p v-else>成本中心没有与此订单 SKU 完全一致的维护记录。</p>
      </template>
    </section>
    <p v-if="subject.missingCostLines != null" class="sku-hint">{{ Number(subject.missingCostLines) > 0 ? '本组有 ' + subject.missingCostLines + ' 条订单行缺少成本。' : '本组订单行均有成本金额。' }}</p>
    <p class="sku-hint">{{ costCoverageDefinition }} 当前编码成本用于日常维护。</p>
    <div class="sku-actions">
      <RouterLink v-if="detailLink" class="oa-button" :to="detailLink">查看商品详情</RouterLink>
      <RouterLink v-if="can('pricing.view')" class="oa-button" :to="pricingLink || { path: '/operations/pricing', query: { sku: subject.sku, exact_sku: '1' } }">查看试算依据</RouterLink>
    </div>
    <template v-if="canMaintainRow">
      <p v-if="maintenanceMessage" role="status">{{ maintenanceMessage }}</p>
      <p v-if="error" role="alert" class="oa-error">{{ error }}</p>
      <button v-if="staleCost" type="button" :disabled="busy" @click="loadCurrentCost">刷新当前成本</button>
      <form v-else-if="editingPrice" @submit.prevent="maintain('modify')">
        <label>新系统单位成本
          <input v-model="price" type="number" min="0" max="1000000000" step="0.0001" required :disabled="busy" />
        </label>
        <label>修改原因
          <textarea v-model.trim="priceReason" maxlength="200" required :disabled="busy" />
        </label>
        <div class="sku-actions">
          <button type="button" :disabled="busy" @click="editingPrice = false; error = ''">取消修改</button>
          <button type="submit" class="oa-button-primary" :disabled="busy || !priceChanged || !priceReason">{{ busy ? '保存中…' : '保存成本' }}</button>
        </div>
      </form>
      <div v-else class="sku-actions sku-maintenance-actions">
        <button type="button" :disabled="busy" @click="editPrice">修改成本</button>
        <button v-if="currentCost?.cost_price != null && (!currentCost.reviewed || currentCost.status === 'conflict')"
          type="button" class="oa-button-primary" :disabled="busy" @click="maintain('confirm')">确认当前系统成本</button>
      </div>
    </template>
    <form v-if="!loadingCost && !message && !editingPrice && (!canMaintainRow || Number(subject.missingCostLines) > 0 || currentCost?.cost_price == null)" @submit.prevent="submit">
      <label>核价原因
        <textarea v-model.trim="reason" maxlength="300" required
          placeholder="例如：已更换材质，请核对这个编码的维护成本" @keydown.enter.exact.prevent="submit" />
      </label>
      <p v-if="error" role="alert" class="oa-error">{{ error }}</p>
      <button type="submit" class="oa-button-primary" :disabled="busy || !reason">{{ busy ? '正在发送…' : split ? '提交核价请求并下一条' : '推送核价' }}</button>
    </form>
    <div v-else-if="message" role="status">
      <p>{{ message }}</p>
      <RouterLink class="oa-button" :to="{ path: '/me/notifications', query: { section: 'cost', costQueue: 'mine' } }" @click="close">查看我的请求</RouterLink>
    </div>
  </component>
</template>

<style scoped>
.sku-inspector {
  position: fixed; z-index: var(--oa-inspector-layer, 140); inset: 0 0 0 auto; width: min(420px, 100vw);
  min-width: 0; min-height: 0; max-width: none; max-height: none; height: 100dvh;
  margin: 0; overflow: auto; padding: 16px 24px;
  background: var(--oa-surface); color: var(--oa-ink); border: 0; border-inline-start: 1px solid var(--oa-line);
  box-shadow: -8px 0 24px rgb(var(--yb-shadow) / .11);
}
.sku-inspector::backdrop { background: var(--oa-inspector-scrim); }
.sku-inspector .sku-summary { padding: 12px; border-radius: 6px; background: rgb(var(--yb-surface-muted)); }
.sku-current-cost, .sku-maintenance-actions { margin-top: 12px; }
.sku-summary h3 { overflow-wrap: anywhere; }
.sku-inspector .sku-summary p { margin: 4px 0; color: var(--oa-secondary); font-size: 12px; }
.sku-summary dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 8px 0 0; }
.sku-summary dd { margin: 0; font: 600 14px/1.5 var(--yb-font-mono); overflow-wrap: anywhere; }
.sku-summary dt { color: var(--oa-secondary); font-size: 12px; }
.sku-inspector header, .sku-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.sku-inspector header { justify-content: space-between; margin-bottom: 12px; }
.sku-inspector p { margin: 8px 0; overflow-wrap: anywhere; }
.sku-hint { font-size: 12px; color: var(--oa-secondary); }
.sku-inspector form, .sku-inspector label { display: grid; gap: 8px; margin-top: 8px; }
.sku-inspector textarea { min-height: 72px; resize: vertical; }
.sku-inspector form button { justify-self: end; }
@media (min-width: 1920px) {
  .sku-inspector-split { position: static; width: auto; height: auto; border: 1px solid var(--oa-line); border-radius: 8px; box-shadow: none; }
}
</style>
