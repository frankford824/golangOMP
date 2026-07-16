<template>
  <main class="groups-page assets-index-view" data-surface-ready="true">
    <header><div><p class="eyebrow">成品资料</p><h1>资产中心</h1><p>按任务和 SKU 查看当前有效源文件与最终成品。</p></div><button :disabled="loading" @click="load">刷新</button></header>
    <form class="filters" @submit.prevent="search"><label>关键词<input v-model.trim="filters.q" placeholder="任务号、SKU 或文件名" /></label><label>SKU<input v-model.trim="filters.sku_code" placeholder="精确 SKU" /></label><label>业务类型<select v-model="filters.business_lane"><option value="">全部</option><option value="normal">常规</option><option value="customization">定制</option><option value="retouch">修图</option></select></label><label>文件类型<select v-model="filters.format_category"><option value="">全部</option><option value="image">图片</option><option value="design">设计源文件</option><option value="document">文档</option><option value="archive">压缩包</option></select></label><button>查询</button></form>
    <div v-if="error" class="error" role="alert"><span>{{ error }}</span><button @click="load">重试</button></div>
    <div v-if="loading" class="empty">正在加载资源组…</div>
    <div v-else-if="!result.items.length" class="empty">没有找到符合条件的任务资源。</div>
    <section v-else class="grid" aria-label="资源组列表">
      <button v-for="group in result.items" :key="group.id" class="resource-card" @click="openGroup(group.id)">
        <span class="card-head"><span>{{ group.task_no || `任务 ${group.task_id}` }}</span><strong>{{ group.sku_code || scopeLabel(group) }}</strong></span>
        <span class="preview-strip"><template v-for="item in finals(group).slice(0,4)" :key="item.id"><img v-if="coverURL(item) && !brokenImages.has(item.id)" :src="coverURL(item)" :alt="item.file?.file_name || item.item_name || '成品预览'" loading="lazy" @error="markImageBroken(item.id)" /><span v-else class="preview-fallback">{{ fileInitial(item.file?.file_name || item.item_name) }}</span></template><em v-if="!finals(group).length">暂无最终成品</em></span>
        <span class="card-footer"><span>{{ revision(group)?.mode === 'set' ? `套装 · ${finals(group).length} 张` : '单图' }}</span><span>{{ laneLabel(group.business_lane) }}</span><span v-if="group.migration_incomplete" class="warn">资源待人工确认</span></span>
      </button>
    </section>
    <nav v-if="result.total > result.page_size" class="pager" aria-label="资源组分页"><button :disabled="loading || result.page <= 1" @click="goPage(result.page - 1)">上一页</button><span>第 {{ result.page }} / {{ totalPages }} 页 · 共 {{ result.total }} 组</span><button :disabled="loading || result.page >= totalPages" @click="goPage(result.page + 1)">下一页</button></nav>
  </main>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { resourceGroupsApi, type ResourceGroup, type ResourceRevision, type ResourceRevisionItem } from '@/services/api/resourceGroupsApi'
const router = useRouter(); const loading = ref(false); const error = ref(''); const brokenImages = ref(new Set<number>())
const filters = reactive({ q: '', sku_code: '', business_lane: '', format_category: '' })
const initialPageSize = import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
  ? Math.max(80, Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE || 100))
  : 24
const result = reactive({ items: [] as ResourceGroup[], page: 1, page_size: initialPageSize, total: 0 })
const totalPages = computed(() => Math.max(1, Math.ceil(result.total / result.page_size)))
const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const finals = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const scopeLabel = (group: ResourceGroup) => group.scope_kind === 'retouch_requirement' ? `修图需求 ${group.retouch_requirement_id}` : '任务资源'
const laneLabel = (lane?: string) => ({ normal: '常规', customization: '定制', retouch: '修图' }[lane || ''] || '')
const coverURL = (item: ResourceRevisionItem) => item.file?.preview_url || item.file?.download_url || ''
const fileInitial = (name?: string) => (name || '文件').slice(0, 2).toUpperCase()
function markImageBroken(id: number) { brokenImages.value = new Set(brokenImages.value).add(id) }
function openGroup(id: number) { void router.push(`/asset-center/${id}`) }
async function load() { loading.value = true; error.value = ''; try { const next = await resourceGroupsApi.list({ q: filters.q || undefined, sku_code: filters.sku_code || undefined, business_lane: filters.business_lane || undefined, format_category: filters.format_category || undefined, page: result.page, page_size: result.page_size }); Object.assign(result, next) } catch (cause) { error.value = cause instanceof Error ? cause.message : '资产中心加载失败。' } finally { loading.value = false } }
function search() { result.page = 1; void load() }
function goPage(page: number) { result.page = Math.max(1, Math.min(totalPages.value, page)); void load() }
onMounted(load)
</script>
<style scoped>
.groups-page{max-width:1260px;margin:0 auto;padding:28px;display:grid;gap:20px}.groups-page>header{display:flex;align-items:flex-start;justify-content:space-between}.groups-page h1{margin:4px 0;font-size:31px}.groups-page p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}button,.filters input,.filters select{min-height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 13px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.filters{display:grid;grid-template-columns:2fr 1fr 1fr 1fr auto;align-items:end;gap:8px}.filters label{display:grid;gap:5px;font-size:12px;color:rgb(var(--yb-text-muted))}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(290px,1fr));gap:14px}.resource-card{min-height:230px;display:grid;align-content:space-between;gap:14px;padding:17px;text-align:left;cursor:pointer;transition:.15s}.resource-card:hover,.resource-card:focus-visible{transform:translateY(-2px);border-color:rgb(var(--yb-brand-border));outline:2px solid rgb(var(--yb-brand-soft));outline-offset:2px}.card-head{display:grid;gap:4px}.card-head>span,.card-footer{font-size:12px;color:rgb(var(--yb-text-muted))}.preview-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));min-height:92px;gap:7px;align-items:stretch}.preview-strip img,.preview-fallback{width:100%;height:92px;object-fit:cover;border-radius:9px;background:rgb(var(--yb-surface-muted))}.preview-fallback{display:grid;place-items:center;color:rgb(var(--yb-text-muted));font-weight:800}.preview-strip em{grid-column:1/-1;align-self:center;text-align:center;font-style:normal;color:rgb(var(--yb-text-muted))}.card-footer{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap}.warn{color:rgb(var(--yb-warning-text))}.pager{display:flex;align-items:center;justify-content:center;gap:14px}.empty,.error{padding:35px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:14px;color:rgb(var(--yb-text-muted))}.error{display:flex;align-items:center;justify-content:center;gap:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:850px){.filters{grid-template-columns:1fr 1fr}.filters button{grid-column:1/-1}}@media(max-width:700px){.groups-page{padding:16px}.groups-page>header{flex-direction:column;gap:12px}.filters{grid-template-columns:1fr}.filters button{grid-column:auto}.grid{grid-template-columns:1fr}.pager{justify-content:space-between}.pager span{font-size:12px;text-align:center}}
</style>
