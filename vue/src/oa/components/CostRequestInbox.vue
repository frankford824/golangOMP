<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getToken } from '@/services/http'
import { usePermissionsStore } from '@/stores/permissions'
import { useOAAccess } from '../access'
import { api, errorText, send } from '../api'
import { invalidateCostActivity } from '../costActivity'
import { feeDateTime } from '../feeDisplay'
import OAEmpty from './OAEmpty.vue'

defineProps<{compact?: boolean}>()
defineEmits<{navigate:[]}>()
const { can, currentProfile } = useOAAccess()
const permissions = usePermissionsStore()
const reviewer = computed(() => can('cost.request.handle'))
const route=useRoute()
type Queue='mine'|'pending'|'replies'
const queue = ref<Queue>(route.query.costQueue==='mine'?'mine':reviewer.value?'pending':'mine')
const unread=ref(0)
const activePending=ref(0)
type Item = {id:number;created_at:string;unread:boolean;state:'pending'|'handled';shop_id:string;sku:string;sender:string;reason:string;reply?:string;handled_by?:string;handled_at?:string}
type Page = {items:Item[];has_more:boolean;pending_count:number;reply_count:number}
const rows=ref<Item[]>([])
const busy=ref(false), acting=ref(false), error=ref(''), message=ref('')
const hasMore=ref(false), history=ref<number[]>([0])
const selected=ref<number>(), reply=ref('')
let sequence=0
let chooseLanding=route.query.costQueue!=='mine'
async function load(before=0, change?:'next'|'prev', background=false){
 const seq=++sequence
 const token=getToken()
 if(!background) busy.value=true
 error.value=''
 try{
  const params=new URLSearchParams({queue:queue.value})
  if(before) params.set('before',String(before))
  const result=await api<Page>('/oa-api/orders/cost-requests?'+params)
  if(seq!==sequence || token!==getToken())return
  if (chooseLanding) {
    chooseLanding=false
    if(result.reply_count && !result.pending_count && queue.value!=='replies') {
      queue.value='replies'
      await load()
      return
    }
  }
  rows.value=result.items||[]; hasMore.value=!!result.has_more; unread.value=result.reply_count; activePending.value=result.pending_count
  if(change==='next')history.value.push(before)
  else if(change==='prev')history.value.pop()
  else if(!before)history.value=[0]
 }catch(cause){if(seq===sequence && token===getToken()){error.value=errorText(cause);rows.value=[];hasMore.value=false}}
 finally{if(seq===sequence)busy.value=false}
}
function switchQueue(next:Queue){
 if(acting.value)return
 chooseLanding=false
 queue.value=next;rows.value=[];history.value=[0];selected.value=undefined;reply.value='';message.value=''
 void load()
}
async function respond(id:number){
 if(acting.value||!reply.value.trim())return
 const token=getToken()
 acting.value=true;error.value=''
 try{
  await send('/oa-api/orders/cost-requests/'+id+'/resolve',{reply:reply.value.trim()})
  if(token!==getToken())return
  message.value='核对结果已发给申请人';selected.value=undefined;reply.value=''
  invalidateCostActivity();await load()
 }catch(cause){if(token===getToken())error.value=errorText(cause)}finally{if(token===getToken())acting.value=false}
}
async function read(item:Item){
 if(acting.value)return
 const token=getToken()
 acting.value=true
 try {
  await send('/oa-api/orders/cost-requests/'+item.id+'/read',{})
  if(token!==getToken())return
  item.unread=false
  invalidateCostActivity()
  await load(history.value.at(-1))
 } catch(cause) {
  if(token===getToken())error.value=errorText(cause)
 } finally { if(token===getToken())acting.value=false }

}
// Keep an open queue current without interrupting a reply the user is typing.
let timer:number|undefined
function visibleRefresh(){
 if(document.visibilityState==='visible' && !busy.value && !acting.value && !selected.value && history.value.length===1)void load(0,undefined,true)
}
onMounted(()=>{void load();window.addEventListener('focus',visibleRefresh);timer=window.setInterval(visibleRefresh,30000)})
onUnmounted(()=>{sequence++;window.removeEventListener('focus',visibleRefresh);window.clearInterval(timer)})
watch(()=>[currentProfile.value?.id,reviewer.value],()=>{
 ++sequence
 acting.value=false
 busy.value=false
 error.value=''
 unread.value=0
 activePending.value=0
 switchQueue(reviewer.value?'pending':'mine')
}, { flush:'sync' })
</script>
<template>
 <section class="cost-requests" :class="{'cost-requests-compact':compact}" aria-label="成本核对">
  <header><h2>成本核对</h2><button type="button" :disabled="busy || acting" @click="load(history.at(-1))">刷新</button></header>
  <div class="cost-queue" aria-label="核价消息范围"><button v-if="reviewer" type="button" :disabled="acting" :aria-pressed="queue==='pending'" @click="switchQueue('pending')">待核对 {{ activePending }}</button><button type="button" :disabled="acting" :aria-pressed="queue==='mine'" @click="switchQueue('mine')">我发出的</button><button type="button" :disabled="acting" :aria-pressed="queue==='replies'" @click="switchQueue('replies')">未读回复 {{ unread }}</button></div>
  <p v-if="error" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
  <p v-if="busy" role="status">正在读取核价请求…</p>
  <OAEmpty v-else-if="!rows.length && !error" compact kind="inbox" :title="queue==='pending'?'暂无待核对的 SKU':queue==='replies'?'暂无未读回复':'暂无已发出的请求'" />
  <article v-for="row in rows" :key="row.id">
   <header><strong>{{ row.sku }}</strong><span class="cost-status">{{ row.state==='pending'?'待核对':row.unread?'有新回复':'已回复' }}</span></header>
   <p class="cost-meta">{{ row.sender }} · 店铺 {{ row.shop_id }} · {{ feeDateTime(row.created_at) }}</p>
   <p>{{ row.reason }}</p>
   <p v-if="row.reply"><strong>{{ row.handled_by }} 回复</strong>　{{ row.reply }}</p>
   <div v-if="queue==='pending'" class="cost-actions">
    <RouterLink v-if="permissions.hasMenu('cost_rules')" :to="{path:'/cost-rules',query:{sku:row.sku}}" @click="$emit('navigate')">查看编码成本</RouterLink>
    <RouterLink v-if="can('pricing.view')" :to="{path:'/operations/pricing',query:{sku:row.sku}}" @click="$emit('navigate')">查看试算依据</RouterLink>
    <button type="button" @click="selected=row.id;reply=''" :disabled="acting">回复核对结果</button>
   </div>
   <form v-if="queue==='pending' && selected===row.id" @submit.prevent="respond(row.id)">
    <label>核对结果<textarea v-model.trim="reply" maxlength="300" required placeholder="说明核对依据、是否已在成本中心处理" /></label>
    <div class="cost-actions"><button type="submit" :disabled="acting || !reply">发送结果</button><button type="button" :disabled="acting" @click="selected=undefined">取消</button></div>
   </form>
   <button v-if="row.unread" :disabled="acting" type="button" @click="read(row)">标记已读</button>
  </article>
  <footer v-if="hasMore || history.length>1"><button :disabled="busy || acting || history.length<2" @click="load(history.at(-2),'prev')">上一页</button><button :disabled="busy || acting || !hasMore" @click="load(rows.at(-1)?.id,'next')">下一页</button></footer>
 </section>
</template>
<style scoped>
.cost-requests { background:var(--oa-surface);border:1px solid var(--oa-line);border-radius:8px;padding:16px;min-width:0; }
.cost-requests header,.cost-requests footer,.cost-actions,.cost-queue { display:flex;align-items:center;gap:8px;flex-wrap:wrap; }
.cost-requests header { justify-content:space-between; }
.cost-requests h2,.cost-requests p { margin:0; }
.cost-requests article { padding:12px 0; border-bottom:1px solid var(--oa-line);overflow-wrap:anywhere;display:grid;gap:4px; }
.cost-requests button,.cost-requests a { border:1px solid var(--oa-line);border-radius:6px;padding:4px 8px;min-height:32px;color:var(--oa-accent); }
.cost-requests button:disabled { opacity:.55; }
.cost-queue { margin:8px 0; }
.cost-queue [aria-pressed=true],.cost-status { background:rgb(var(--yb-brand-soft)); }
.cost-meta,.cost-status { color:var(--oa-secondary);font-size:12px; }.cost-status{padding:2px 8px;border-radius:6px;}
.cost-requests form label {display:grid;gap:4px;}.cost-requests textarea{border:1px solid var(--oa-line);padding:8px;border-radius:6px;resize:vertical;}
.cost-requests-compact { border:0;padding:8px; }
</style>
