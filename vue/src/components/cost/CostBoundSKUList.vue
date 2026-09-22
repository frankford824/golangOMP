<template>
  <aside class="bound-skus" aria-label="方案关联SKU">
    <header><h2>关联 SKU <small>{{ total }} 个</small></h2><p>当前绑定款式下的 SKU。成本为已记录金额，不随方案编辑即时变化。</p></header>
    <form @submit.prevent="search"><label>搜索 SKU / 产品 / 款式<input v-model.trim="keyword" placeholder="输入编码或产品名称" /></label><button :disabled="loading || !group">搜索</button></form>
    <p v-if="error" role="alert">{{error}} <button @click="load">重试</button></p>
    <p v-else-if="loading" role="status">正在读取关联 SKU…</p>
    <p v-else-if="!items.length" class="empty">{{group?'没有匹配的 SKU；请检查已绑定款式或搜索条件。':'先从左侧选择方案。'}}</p>
    <div v-else class="sku-table"><table><thead><tr><th>SKU / 产品</th><th>款式编码</th><th>当前成本</th></tr></thead><tbody><tr v-for="item in items" :key="item.id"><td><strong>{{item.sku_code}}</strong><span>{{item.product_name || '未填写产品名称'}}</span></td><td>{{item.style_code}}</td><td>{{item.cost_price==null?'待确认':`¥${item.cost_price.toFixed(3).replace(/0+$/,'').replace(/\.$/,'')}`}}</td></tr></tbody></table></div>
    <footer><button :disabled="page<=1 || loading" @click="page--;load()">上一页</button><span>{{page}} / {{Math.max(1,Math.ceil(total/20))}}</span><button :disabled="page*20>=total || loading" @click="page++;load()">下一页</button></footer>
  </aside>
</template>
<script setup lang="ts">
import {ref,watch} from 'vue'
import {costManagementApi} from '@/services/api/costManagementApi'
const props=defineProps<{group:string;revision:number}>()
const keyword=ref(''),query=ref(''),page=ref(1),total=ref(0),loading=ref(false),error=ref('')
const items=ref<Awaited<ReturnType<typeof costManagementApi.listBoundSKUs>>['data']>([])
let request=0
async function load(){const ticket=++request;items.value=[];error.value='';if(!props.group){total.value=0;loading.value=false;return}loading.value=true
 try{const data=await costManagementApi.listBoundSKUs({rule_group:props.group,keyword:query.value,page:page.value,page_size:20});if(ticket!==request)return;items.value=data.data||[];total.value=data.pagination?.total||0}
 catch(e){if(ticket===request){error.value=e instanceof Error?e.message:'读取失败';total.value=0}}
 finally{if(ticket===request)loading.value=false}}
function search(){query.value=keyword.value;page.value=1;load()}
watch(()=>[props.group,props.revision],()=>{keyword.value='';query.value='';page.value=1;total.value=0;load()},{immediate:true})
</script>
<style scoped>
.bound-skus{display:flex;flex-direction:column;gap:14px;padding:18px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface));min-width:0;min-height:0;overflow:auto}.bound-skus h2{font-size:16px;margin:0}.bound-skus small,.bound-skus p{font-size:12px;color:rgb(var(--yb-text-muted));font-weight:400}.bound-skus form{display:flex;gap:8px;align-items:end}.bound-skus label{display:grid;gap:6px;font-size:12px;flex:1;min-width:0}.bound-skus input{width:100%;min-width:0;box-sizing:border-box;border:1px solid rgb(var(--yb-border));border-radius:8px;padding:9px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.bound-skus button{border:1px solid rgb(var(--yb-border));border-radius:8px;padding:8px 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));cursor:pointer;white-space:nowrap}.bound-skus button:disabled{opacity:.4;cursor:default}.sku-table{overflow:auto;flex:1;min-height:0}table{width:100%;border-collapse:collapse;font-size:12px}th{text-align:left;color:rgb(var(--yb-text-muted));font-weight:500}th,td{padding:11px 6px;border-bottom:1px solid rgb(var(--yb-border));vertical-align:top}td strong{display:block;font-size:12px;color:rgb(var(--yb-brand))}td span{display:block;color:rgb(var(--yb-text-muted));margin-top:5px;overflow-wrap:anywhere;max-width:240px}td:last-child{white-space:nowrap}footer{display:flex;align-items:center;justify-content:space-between;font-size:12px;margin-top:auto}.empty{padding:30px 0}
</style>
