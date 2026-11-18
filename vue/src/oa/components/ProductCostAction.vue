<script setup lang="ts">
import { nextTick, ref, watch, computed } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'
import { useOAAccess } from '../access'
import { errorText, send } from '../api'
import { invalidateCostActivity } from '../costActivity'
export type CostTarget = {sku:string;shop:string;shopName:string;from:string;to:string;includeBrush:boolean}
const { can, currentProfile } = useOAAccess()
const permissions=usePermissionsStore()
const mayMaintain=computed(()=>permissions.hasMenu('cost_rules') && can('cost.request.handle'))
const dialog=ref<HTMLDialogElement>()
const subject=ref<CostTarget>()
const reason=ref(''), message=ref(''), error=ref(''), busy=ref(false)
let key=''
async function open(target:CostTarget) {
 if (busy.value) return
 subject.value=target
 reason.value='';message.value='';error.value='';key=crypto.randomUUID()
 await nextTick()
 dialog.value?.showModal()
}
function close() {
 if(!busy.value)dialog.value?.close()
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
 }catch(cause){if(row===subject.value)error.value=errorText(cause)}
 finally{busy.value=false}
}
watch(()=>currentProfile.value?.id,()=>{subject.value=undefined;dialog.value?.close()})
defineExpose({open})
</script>
<template>
 <dialog ref="dialog" class="sku-review-dialog" aria-labelledby="sku-review-title" @cancel="busy && $event.preventDefault()">
  <template v-if="subject">
   <header><h2 id="sku-review-title">核对 SKU 成本</h2><button type="button" :disabled="busy" aria-label="关闭成本核对" @click="close">关闭</button></header>
   <p><strong>{{ subject.sku }}</strong> · {{ subject.shopName }}</p>
   <p class="sku-hint">经营报表使用成交时的成本。编码当前成本由成本中心维护。</p>
   <div v-if="!message" class="sku-actions">
    <RouterLink v-if="mayMaintain" class="oa-button" :to="{path:'/cost-rules',query:{sku:subject.sku}}" @click="close">维护当前编码成本</RouterLink>
    <RouterLink v-if="can('pricing.view')" class="oa-button" :to="{path:'/operations/pricing',query:{sku:subject.sku}}" @click="close">查看试算依据</RouterLink>
   </div>
   <form v-if="!message" @submit.prevent="submit">
    <label>核价原因<textarea v-model.trim="reason" maxlength="300" required placeholder="例如：已更换材质，请核对这个编码的维护成本" /></label>
    <p v-if="error" role="alert" class="oa-error">{{ error }}</p>
    <button type="submit" class="oa-button-primary" :disabled="busy || !reason">{{ busy?'正在发送…':'推送核价' }}</button>
   </form>
   <div v-else role="status"><p>{{ message }}</p><RouterLink class="oa-button" :to="{path:'/me/notifications',query:{section:'cost',costQueue:'mine'}}" @click="close">查看我的请求</RouterLink></div>
  </template>
 </dialog>
</template>
<style scoped>
.sku-review-dialog {width:min(440px,calc(100% - 24px)); margin:auto; border:1px solid var(--oa-line); background:var(--oa-surface);border-radius:12px;padding:20px;color:var(--oa-ink);}
.sku-review-dialog::backdrop {background:var(--yb-overlay-night);}
.sku-review-dialog header,.sku-actions {display:flex;align-items:center;gap:8px;flex-wrap:wrap;}
.sku-review-dialog header {justify-content:space-between;margin-bottom:12px;}
.sku-review-dialog p {margin:8px 0;overflow-wrap:anywhere;}
.sku-hint{font-size:12px;color:var(--oa-secondary);}
.sku-review-dialog form,.sku-review-dialog label{display:grid;gap:8px;margin-top:8px;}
.sku-review-dialog textarea{min-height:72px;resize:vertical;}
.sku-review-dialog form button{justify-self:end;}
</style>
