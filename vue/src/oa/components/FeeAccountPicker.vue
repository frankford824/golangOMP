<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import { api, send, errorText } from '../api'
import { useOAAccess } from '../access'

const props = defineProps<{ type: string; manage?: boolean; ownerId?: string }>()
const emit = defineEmits<{ select:[account:{ holder:string; bank:string; account:string }] }>()
type Row = { id:string; holder:string; bank:string; last_four:string; version:number }
const { can } = useOAAccess()
const supplier = computed(() => ['payment','付款','supplier'].includes(props.type))
const scope = computed(() => supplier.value ? 'supplier' : 'personal')
const editable = computed(() => !props.ownerId && (!supplier.value || can('finance.admin') || can('finance.pay')))
const rows=ref<Row[]>([]), page=ref(1), more=ref(false), query=ref(''), term=ref('')
const open=ref(false), busy=ref(false), loading=ref(false), message=ref(''), status=ref('')
const editor=ref<null|{row?:Row;holder:string;bank:string;account:string}>(null), retiring=ref<Row>()
let seq=0
async function load(){
 const stamp=++seq
 loading.value=true; message.value=''
 const q=new URLSearchParams({scope:scope.value,page:String(page.value),q:term.value})
 if(props.ownerId) q.set('owner_id',props.ownerId)
 try{const data=await api<{items:Row[];has_more:boolean}>('/oa-api/finance/accounts?'+q)
 if(stamp===seq){rows.value=data.items;more.value=data.has_more}
 }catch(e){if(stamp===seq){message.value=errorText(e);rows.value=[];more.value=false}}
 finally{if(stamp===seq)loading.value=false}
}
function openList(){open.value=true;page.value=1;void load()}
async function pick(row:Row){
 if(busy.value)return
 busy.value=true;message.value=''
 try{const result=await send<{holder:string;bank:string;account:string}>('/oa-api/finance/accounts/'+row.id+'/use',{})
 emit('select',result);open.value=false;status.value='已带入收款账户，请核对户名和账号。'
 }catch(e){message.value=errorText(e)}finally{busy.value=false}
}
function edit(row?:Row){editor.value={row,holder:row?.holder||'',bank:row?.bank||'',account:''};message.value=''}
async function save(){
 const form=editor.value
 if(!form||busy.value)return
 busy.value=true; message.value=''
 try{await send('/oa-api/finance/accounts'+(form.row?'/'+form.row.id:''),{scope:scope.value,version:form.row?.version||0,holder:form.holder,bank:form.bank,account:form.account},form.row?'PUT':'POST')
 editor.value=null;status.value='收款账户已保存';await load()
 }catch(e){message.value=errorText(e)}finally{busy.value=false}
}
async function retire(){
 const row=retiring.value;if(!row||busy.value)return
 busy.value=true;message.value=''
 try{await send('/oa-api/finance/accounts/'+row.id+'/retire',{version:row.version});retiring.value=undefined;status.value='已停用收款账户';await load()}
 catch(e){message.value=errorText(e)}finally{busy.value=false}
}
function find(){page.value=1;term.value=query.value;void load()}
function turn(by:number){page.value+=by;void load()}
watch(()=>[scope.value,props.ownerId,props.manage],()=>{seq++;open.value=false;editor.value=null;retiring.value=undefined;status.value='';message.value='';rows.value=[];more.value=false;query.value='';term.value='';page.value=1;if(props.manage)void load()},{immediate:true})
</script>
<template>
 <div class="fee-accounts">
  <button v-if="!manage" type="button" class="oa-button" @click="openList">{{supplier?'从供应商账户选择':'带入本人默认账户'}}</button>
  <component :is="manage ? 'section' : BaseModal" v-bind="manage ? {} : {modelValue:open,title:supplier?'选择供应商账户':'本人默认收款账户'}" @cancel="open=false">
   <div class="recipient-body">
    <form v-if="supplier" class="recipient-bar" @submit.prevent="find"><input v-model="query" aria-label="搜索供应商" placeholder="搜索单位名称或开户行"><button class="oa-button" type="submit">查询</button></form>
    <p v-if="loading" role="status">正在读取账户…</p>
    <p v-else-if="!rows.length && !message">{{supplier?'当前没有匹配的供应商账户。':'尚未维护本人默认收款账户。'}}</p>
    <div v-for="row in rows" :key="row.id" class="recipient-row">
     <div><strong>{{row.holder}}</strong><small>{{row.bank}} · 尾号 {{row.last_four}}</small></div>
     <button v-if="!manage" type="button" class="oa-button" :disabled="busy" @click="pick(row)">带入账户</button>
     <template v-if="manage && editable"><button class="oa-button" type="button" @click="edit(row)">修改</button><button class="oa-button" type="button" @click="retiring=row">停用</button></template>
    </div>
    <div v-if="page>1 || more" class="recipient-bar"><button type="button" class="oa-button" :disabled="page<=1 || loading" @click="turn(-1)">上一页</button><span>第 {{page}} 页</span><button type="button" class="oa-button" :disabled="!more || loading" @click="turn(1)">下一页</button></div>
    <button v-if="editable && !loading && (supplier || !rows.length)" type="button" class="oa-button" @click="edit()">添加{{supplier?'供应商账户':'本人默认账户'}}</button>
    <p v-if="message && !editor && !retiring" role="alert">{{message}}</p>
   </div>
  </component>
  <p v-if="status" role="status">{{status}}</p>
  <BaseModal :model-value="!!editor" :title="supplier?'维护供应商账户':'维护本人默认收款账户'" @cancel="!busy && (editor=null)">
   <form v-if="editor" class="recipient-body recipient-fields" @submit.prevent="save">
    <label>收款户名<input v-model.trim="editor.holder" required maxlength="160" autocomplete="off"></label><label>开户行<input v-model.trim="editor.bank" required maxlength="160"></label><label>完整银行账号<input v-model.trim="editor.account" required autocomplete="off" inputmode="numeric" placeholder="输入完整账号，可按四位分组"></label>
    <p v-if="message" role="alert">{{message}}</p><button class="oa-button oa-button-primary" :disabled="busy">{{busy?'正在保存…':'保存账户'}}</button>
   </form>
  </BaseModal>
  <BaseModal :model-value="!!retiring" title="停用收款账户" @cancel="!busy && (retiring=undefined)"><div class="recipient-body"><p>停用「{{retiring?.holder}} · 尾号 {{retiring?.last_four}}」后，新申请将无法选择此账户。</p><p v-if="message" role="alert">{{message}}</p><button class="oa-button" type="button" :disabled="busy" @click="retire">确认停用</button></div></BaseModal>
 </div>
</template>
<style scoped>
.recipient-body{display:grid;gap:12px;padding:16px;min-width:0}
.recipient-bar,.recipient-row{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.recipient-row{padding:8px 0;border-bottom:1px solid var(--yb-line-hairline)}
.recipient-row>div{flex:1;min-width:120px;overflow-wrap:anywhere}.recipient-row small{display:block;color:var(--oa-secondary)}
.recipient-fields label{display:grid;gap:4px}.recipient-fields input,.recipient-bar input{height:32px;padding:0 8px;border-radius:6px;border:1px solid var(--yb-line-control)}[role=alert]{color:var(--oa-danger)}
</style>
