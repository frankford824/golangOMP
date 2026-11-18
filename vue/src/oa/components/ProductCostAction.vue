<script setup lang="ts">
import { nextTick, ref, watch, computed } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import { usePermissionsStore } from '@/stores/permissions'
import { useOAAccess } from '../access'
import { errorText, send } from '../api'
import { invalidateCostActivity } from '../costActivity'
export type CostTarget = {sku:string;shop:string;shopName:string;from:string;to:string;includeBrush:boolean}
const { can, currentProfile } = useOAAccess()
const permissions=usePermissionsStore()
const mayMaintain=computed(()=>permissions.hasMenu('cost_rules') && can('cost.request.handle'))
defineProps<{ split?: boolean; detailLink?: RouteLocationRaw }>()
const emit = defineEmits<{ dismiss: []; closed: []; saved: [message: string] }>()
const heading = ref<HTMLElement>()
const subject=ref<CostTarget>()
const reason=ref(''), message=ref(''), error=ref(''), busy=ref(false)
let key=''
function open(target: CostTarget, focus = true) {
  if (busy.value) return
  subject.value = target
  reason.value = ''
  message.value = ''
  error.value = ''
  key = crypto.randomUUID()
  if (focus) void nextTick(() => heading.value?.focus())
}
function dismiss() {
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
defineExpose({ open, close, dismiss, pending: busy })
</script>
<template>
  <aside v-if="subject" class="sku-inspector" :class="{ 'sku-inspector-split': split }" aria-labelledby="sku-review-title">
    <header>
      <h2 id="sku-review-title" ref="heading" tabindex="-1">核对 SKU 成本</h2>
      <button type="button" :disabled="busy" aria-label="关闭成本核对" @click="close">关闭</button>
    </header>
    <p><strong>{{ subject.sku }}</strong> · {{ subject.shopName }}</p>
    <p class="sku-hint">经营报表使用成交时的成本。编码当前成本由成本中心维护。</p>
    <div class="sku-actions">
      <RouterLink v-if="detailLink" class="oa-button" :to="detailLink" target="_blank" rel="noopener">打开完整详情 ↗</RouterLink>
      <RouterLink v-if="mayMaintain" class="oa-button" :to="{ path: '/cost-rules', query: { sku: subject.sku } }"
        target="_blank" rel="noopener">维护当前编码成本 ↗</RouterLink>
      <RouterLink v-if="can('pricing.view')" class="oa-button" :to="{ path: '/operations/pricing', query: { sku: subject.sku } }"
        target="_blank" rel="noopener">查看试算依据 ↗</RouterLink>
    </div>
    <form v-if="!message" @submit.prevent="submit">
      <label>核价原因
        <textarea v-model.trim="reason" maxlength="300" required
          placeholder="例如：已更换材质，请核对这个编码的维护成本" @keydown.enter.exact.prevent="submit" />
      </label>
      <p v-if="error" role="alert" class="oa-error">{{ error }}</p>
      <button type="submit" class="oa-button-primary" :disabled="busy || !reason">{{ busy ? '正在发送…' : split ? '提交核价请求并下一条' : '推送核价' }}</button>
    </form>
    <div v-else role="status">
      <p>{{ message }}</p>
      <RouterLink class="oa-button" :to="{ path: '/me/notifications', query: { section: 'cost', costQueue: 'mine' } }" @click="close">查看我的请求</RouterLink>
    </div>
  </aside>
</template>

<style scoped>
.sku-inspector {
  position: fixed; z-index: 12; inset: 44px 0 0 auto; width: min(400px, 100vw);
  min-width: 0; min-height: 0; overflow: auto; padding: 16px;
  background: var(--oa-surface); color: var(--oa-ink); border: 1px solid var(--oa-line);
  box-shadow: -8px 0 24px rgb(var(--yb-shadow) / .11);
}
.sku-inspector header, .sku-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.sku-inspector header { justify-content: space-between; margin-bottom: 12px; }
.sku-inspector p { margin: 8px 0; overflow-wrap: anywhere; }
.sku-hint { font-size: 12px; color: var(--oa-secondary); }
.sku-inspector form, .sku-inspector label { display: grid; gap: 8px; margin-top: 8px; }
.sku-inspector textarea { min-height: 72px; resize: vertical; }
.sku-inspector form button { justify-self: end; }
@media (min-width: 1920px) {
  .sku-inspector-split { position: static; width: auto; border-radius: 8px; box-shadow: none; }
}
</style>
