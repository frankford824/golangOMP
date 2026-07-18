<template>
  <main class="detail-page">
    <nav class="breadcrumb" aria-label="当前位置"><button @click="router.push('/asset-center')">资产中心</button><span>/</span><span>{{ group?.sku_code || '资源详情' }}</span></nav>
    <header class="detail-hero">
      <div>
        <div class="hero-badges"><span>{{ modeLabel }}</span><span v-if="group?.business_lane">{{ group.business_lane === 'customization' ? '定制' : '常规' }}</span></div>
        <p class="eyebrow">{{ group?.sku_code || '未绑定 SKU' }}</p>
        <h1>{{ group?.product_name || 'SKU 资源详情' }}</h1>
        <p>当前生效资源已按运营参考、设计源文件和最终成品三个阶段整理。</p>
      </div>
      <div class="hero-actions">
        <button v-if="group" class="secondary" @click="router.push(`/tasks/${group.task_id}`)">查看来源任务</button>
        <button class="secondary" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新' }}</button>
        <button class="primary" :disabled="downloading || !group" @click="downloadAll">{{ downloading ? '准备中…' : '下载全部成品' }}</button>
      </div>
    </header>

    <section v-if="group" class="provenance-strip" aria-label="资源来源">
      <div><span>来源任务</span><strong>{{ group.task_no || group.task_id }}</strong></div>
      <div><span>创建人</span><strong>{{ group.creator_name || '—' }}</strong></div>
      <div><span>交付方式</span><strong>{{ modeLabel }}</strong></div>
      <div><span>最终成品</span><strong>{{ finalCount }} 张</strong></div>
    </section>

    <div v-if="error" class="state-card error" role="alert">{{ error }}</div>
    <div v-if="loading && !group" class="state-card">正在加载当前有效资源…</div>
    <SkuResourceMatrix v-else-if="group" :bundle="{ task_id: group.task_id, workflow_revision: 0, groups: [group] }" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue'
import { resourceGroupsApi, type ResourceGroup } from '@/services/api/resourceGroupsApi'

const route = useRoute()
const router = useRouter()
const group = ref<ResourceGroup | null>(null)
const loading = ref(false)
const downloading = ref(false)
const error = ref('')
const groupId = computed(() => Number(route.params.id))
const activeRevision = computed(() => group.value?.finalized_revision || group.value?.working_revision)
const modeLabel = computed(() => activeRevision.value?.mode === 'set' ? '套装资源' : '单图资源')
const finalCount = computed(() => activeRevision.value?.items?.length || 0)

async function load() {
  loading.value = true
  error.value = ''
  try { group.value = await resourceGroupsApi.get(groupId.value) }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '资源详情加载失败。' }
  finally { loading.value = false }
}
async function downloadAll() {
  if (!group.value) return
  downloading.value = true
  error.value = ''
  try {
    const result = await resourceGroupsApi.batchDownload([group.value.id])
    result.items.sort((a, b) => a.sort_order - b.sort_order).forEach((item, index) => setTimeout(() => {
      const link = document.createElement('a'); link.href = item.download_url || ''; link.download = item.filename; link.click()
    }, index * 120))
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '下载清单生成失败。' }
  finally { downloading.value = false }
}
onMounted(load)
</script>

<style scoped>
.detail-page{max-width:1320px;margin:0 auto;padding:28px;display:grid;gap:20px}.breadcrumb{display:flex;align-items:center;gap:8px;color:rgb(var(--yb-text-muted));font-size:12px}.breadcrumb button{padding:0;border:0;background:transparent;color:rgb(var(--yb-brand));cursor:pointer}.detail-hero{display:flex;align-items:flex-start;justify-content:space-between;gap:22px;padding:22px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface))}.detail-hero h1{margin:4px 0;font-size:30px}.detail-hero p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{color:rgb(var(--yb-brand));font-size:12px;font-weight:900;letter-spacing:.08em}.hero-badges,.hero-actions{display:flex;gap:8px;flex-wrap:wrap}.hero-badges span{padding:5px 8px;border-radius:999px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-size:11px;font-weight:800}.hero-actions button{min-height:40px;padding:0 14px;border-radius:10px;cursor:pointer}.primary{border:1px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.provenance-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface));overflow:hidden}.provenance-strip div{display:grid;gap:4px;padding:14px 17px}.provenance-strip div+div{border-left:1px solid rgb(var(--yb-border))}.provenance-strip span{color:rgb(var(--yb-text-muted));font-size:11px}.provenance-strip strong{font-size:13px}.state-card{padding:34px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:14px;color:rgb(var(--yb-text-muted))}.state-card.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:760px){.detail-page{padding:16px}.detail-hero{flex-direction:column}.provenance-strip{grid-template-columns:1fr 1fr}.provenance-strip div+div{border-left:0}.provenance-strip div:nth-child(even){border-left:1px solid rgb(var(--yb-border))}.provenance-strip div:nth-child(n+3){border-top:1px solid rgb(var(--yb-border))}.hero-actions{width:100%}.hero-actions button{flex:1}}
</style>
