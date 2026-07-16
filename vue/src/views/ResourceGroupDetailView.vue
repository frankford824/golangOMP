<template>
  <main class="group-detail detail-page">
    <header><div><p class="eyebrow">资产详情</p><h1>{{ group?.sku_code || `资源组 ${groupId}` }}</h1><p>{{ group?.task_no || `任务 ${group?.task_id || '—'}` }}</p></div><div><button @click="router.push('/asset-center')">返回资产中心</button><button :disabled="loading" @click="load">刷新</button></div></header>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="loading && !group" class="state">正在加载…</div>
    <template v-else-if="group">
      <SkuResourceMatrix :bundle="{task_id:group.task_id,workflow_revision:0,groups:[group]}" />
      <section class="download-card"><div><h2>下载最终成品</h2><p>下载当前审核通过的版本；套装将完整下载。</p></div><button :disabled="downloading" @click="downloadAll">{{ downloading?'准备中…':'下载全部成品' }}</button></section>
    </template>
  </main>
</template>
<script setup lang="ts">
import { computed,onMounted,ref } from 'vue';import{useRoute,useRouter}from'vue-router';import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue';import{resourceGroupsApi,type ResourceGroup}from'@/services/api/resourceGroupsApi'
const route=useRoute(),router=useRouter(),group=ref<ResourceGroup|null>(null),loading=ref(false),downloading=ref(false),error=ref('');const groupId=computed(()=>Number(route.params.id))
async function load(){loading.value=true;error.value='';try{group.value=await resourceGroupsApi.get(groupId.value)}catch(cause){error.value=cause instanceof Error?cause.message:'资源组加载失败。'}finally{loading.value=false}}
async function downloadAll(){if(!group.value)return;downloading.value=true;error.value='';try{const result=await resourceGroupsApi.batchDownload([group.value.id]);result.items.sort((a,b)=>a.sort_order-b.sort_order).forEach((item,index)=>setTimeout(()=>{const a=document.createElement('a');a.href=item.download_url||'';a.download=item.file_name;a.click()},index*120))}catch(cause){error.value=cause instanceof Error?cause.message:'下载清单生成失败。'}finally{downloading.value=false}}
onMounted(load)
</script>
<style scoped>
.detail-page{max-width:1120px;margin:0 auto;padding:28px;display:grid;gap:20px}.detail-page>header,.detail-page>header>div:last-child,.download-card{display:flex;align-items:flex-start;justify-content:space-between;gap:10px}.detail-page h1{margin:4px 0}.detail-page p,.download-card p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}button{min-height:40px;padding:0 14px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.download-card{align-items:center;padding:18px;border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface))}.download-card h2{margin:0 0 4px}.state,.error{padding:30px;text-align:center;border-radius:13px}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:700px){.detail-page{padding:16px}.detail-page>header,.download-card{flex-direction:column}}
</style>
