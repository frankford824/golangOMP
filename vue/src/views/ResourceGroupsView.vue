<template>
  <main class="groups-page assets-index-view" data-surface-ready="true">
    <header>
      <div>
        <p class="eyebrow">成品资料</p>
        <h1>资产中心</h1>
        <p>默认按 SKU 资源组展示；按资源类型或文件格式筛选时平铺匹配资源。</p>
      </div>
      <button :disabled="loading" @click="load">刷新</button>
    </header>
    <form class="filters" @submit.prevent="search">
      <label>任务号<input v-model.trim="filters.task_no" placeholder="任务号" /></label>
      <label>关键词<input v-model.trim="filters.q" placeholder="SKU 或文件名" /></label>
      <label>SKU<input v-model.trim="filters.sku_code" placeholder="精确 SKU" /></label>
      <label>创建人 ID<input v-model.trim="filters.creator_id" type="number" min="1" placeholder="创建人" /></label>
      <label>业务类型
        <select v-model="filters.business_lane">
          <option value="">全部</option>
          <option value="normal">常规</option>
          <option value="customization">定制</option>
        </select>
      </label>
      <label>资源类型
        <select v-model="filters.resource_role">
          <option value="">全部（SKU 卡片）</option>
          <option value="reference">参考图</option>
          <option value="source">源文件</option>
          <option value="final">最终成品</option>
        </select>
      </label>
      <label>文件格式
        <select v-model="filters.format_category">
          <option value="">全部</option>
          <option value="image">图片</option>
          <option value="design">设计源文件</option>
          <option value="pdf">PDF / 文档</option>
          <option value="archive">压缩包</option>
        </select>
      </label>
      <button>查询</button>
    </form>
    <div v-if="error" class="error" role="alert"><span>{{ error }}</span><button @click="load">重试</button></div>
    <div v-if="loading" class="empty">正在加载资源组…</div>
    <template v-else-if="isFlatMode">
      <div v-if="!result.flat_items.length" class="empty">没有找到符合条件的资源。</div>
      <section v-else class="grid flat-grid" aria-label="匹配资源列表">
        <button v-for="(item, index) in result.flat_items" :key="`${item.group_id}-${item.resource_role}-${index}`" class="resource-card flat-card" @click="openGroup(item.group_id)">
          <span class="card-head"><span>{{ item.task_no || `任务 ${item.task_id}` }}</span><strong>{{ item.sku_code || '资源' }}</strong></span>
          <span class="cover">
            <img v-if="item.preview_url && !brokenFlat.has(index)" :src="item.preview_url" :alt="item.file_name" loading="lazy" @error="markFlatBroken(index)" />
            <span v-else class="preview-fallback">{{ fileInitial(item.file_name) }}</span>
          </span>
          <span class="card-footer"><span>{{ roleLabel(item.resource_role) }}</span><span>{{ item.file_name }}</span></span>
        </button>
      </section>
    </template>
    <template v-else>
      <div v-if="!result.items.length" class="empty">没有找到符合条件的任务资源。</div>
      <section v-else class="grid" aria-label="资源组列表">
        <button v-for="group in result.items" :key="group.id" class="resource-card" @click="openGroup(group.id)">
          <span class="card-head"><span>{{ group.task_no || `任务 ${group.task_id}` }}</span><strong>{{ group.sku_code || scopeLabel(group) }}</strong></span>
          <span class="cover">
            <img v-if="coverURL(group) && !brokenImages.has(group.id)" :src="coverURL(group)" :alt="coverName(group)" loading="lazy" @error="markImageBroken(group.id)" />
            <span v-else class="preview-fallback">{{ fileInitial(coverName(group)) }}</span>
          </span>
          <span class="card-footer">
            <span>{{ revision(group)?.mode === 'set' ? `套装 · ${finals(group).length} 张成品` : `单图 · ${finals(group).length || 0} 张成品` }}</span>
            <span>{{ laneLabel(group.business_lane) }}</span>
            <span v-if="group.migration_incomplete" class="warn">资源待人工确认</span>
          </span>
        </button>
      </section>
    </template>
    <nav v-if="result.total > result.page_size" class="pager" aria-label="资源组分页">
      <button :disabled="loading || result.page <= 1" @click="goPage(result.page - 1)">上一页</button>
      <span>第 {{ result.page }} / {{ totalPages }} 页 · 共 {{ result.total }} {{ isFlatMode ? '项' : '组' }}</span>
      <button :disabled="loading || result.page >= totalPages" @click="goPage(result.page + 1)">下一页</button>
    </nav>
  </main>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { resourceGroupsApi, type FlatResourceItem, type ResourceGroup, type ResourceRevision } from '@/services/api/resourceGroupsApi'

const router = useRouter()
const loading = ref(false)
const error = ref('')
const brokenImages = ref(new Set<number>())
const brokenFlat = ref(new Set<number>())
const filters = reactive({
  q: '',
  sku_code: '',
  task_no: '',
  creator_id: '',
  business_lane: '',
  resource_role: '' as '' | 'reference' | 'source' | 'final',
  format_category: '',
})
const initialPageSize = import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
  ? Math.max(80, Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE || 100))
  : 24
const result = reactive({
  items: [] as ResourceGroup[],
  flat_items: [] as FlatResourceItem[],
  view_mode: 'group' as 'group' | 'flat',
  page: 1,
  page_size: initialPageSize,
  total: 0,
})
const totalPages = computed(() => Math.max(1, Math.ceil(result.total / result.page_size)))
const isFlatMode = computed(() => result.view_mode === 'flat' || !!filters.resource_role || !!filters.format_category)
const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const finals = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const scopeLabel = (group: ResourceGroup) => group.scope_kind === 'retouch_requirement' ? `修图需求 ${group.retouch_requirement_id}` : '任务资源'
const laneLabel = (lane?: string) => ({ normal: '常规', customization: '定制' }[lane || ''] || '')
const roleLabel = (role: string) => ({ reference: '参考图', source: '源文件', final: '最终成品' }[role] || role)
const coverName = (group: ResourceGroup) => finals(group)[0]?.file?.file_name || finals(group)[0]?.item_name || '资源'
const coverURL = (group: ResourceGroup) => {
  const first = finals(group)[0]
  return first?.file?.preview_url || first?.file?.download_url || ''
}
const fileInitial = (name?: string) => (name || '文件').slice(0, 2).toUpperCase()
function markImageBroken(id: number) { brokenImages.value = new Set(brokenImages.value).add(id) }
function markFlatBroken(index: number) { brokenFlat.value = new Set(brokenFlat.value).add(index) }
function openGroup(id: number) { void router.push(`/asset-center/${id}`) }
async function load() {
  loading.value = true
  error.value = ''
  try {
    const next = await resourceGroupsApi.list({
      q: filters.q || undefined,
      sku_code: filters.sku_code || undefined,
      task_no: filters.task_no || undefined,
      creator_id: filters.creator_id ? Number(filters.creator_id) : undefined,
      business_lane: filters.business_lane || undefined,
      resource_role: filters.resource_role || undefined,
      format_category: filters.format_category || undefined,
      page: result.page,
      page_size: result.page_size,
    })
    result.items = next.items || []
    result.flat_items = next.flat_items || []
    result.view_mode = next.view_mode || (filters.resource_role || filters.format_category ? 'flat' : 'group')
    result.page = next.page
    result.page_size = next.page_size
    result.total = next.total
    brokenImages.value = new Set()
    brokenFlat.value = new Set()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '资产中心加载失败。'
  } finally {
    loading.value = false
  }
}
function search() { result.page = 1; void load() }
function goPage(page: number) { result.page = Math.max(1, Math.min(totalPages.value, page)); void load() }
onMounted(load)
</script>
<style scoped>
.groups-page{max-width:1260px;margin:0 auto;padding:28px;display:grid;gap:20px}.groups-page>header{display:flex;align-items:flex-start;justify-content:space-between}.groups-page h1{margin:4px 0;font-size:31px}.groups-page p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}button,.filters input,.filters select{min-height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 13px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.filters{display:grid;grid-template-columns:repeat(4,minmax(0,1fr)) auto;align-items:end;gap:8px}.filters label{display:grid;gap:5px;font-size:12px;color:rgb(var(--yb-text-muted))}.filters button{align-self:end}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:14px}.resource-card{min-height:220px;display:grid;align-content:space-between;gap:12px;padding:16px;text-align:left;cursor:pointer;transition:.15s}.resource-card:hover,.resource-card:focus-visible{transform:translateY(-2px);border-color:rgb(var(--yb-brand-border));outline:2px solid rgb(var(--yb-brand-soft));outline-offset:2px}.card-head{display:grid;gap:4px}.card-head>span,.card-footer{font-size:12px;color:rgb(var(--yb-text-muted))}.cover{display:block;min-height:140px}.cover img,.preview-fallback{width:100%;height:140px;object-fit:cover;border-radius:12px;background:rgb(var(--yb-surface-muted))}.preview-fallback{display:grid;place-items:center;color:rgb(var(--yb-text-muted));font-weight:800}.card-footer{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap}.warn{color:rgb(var(--yb-warning-text))}.pager{display:flex;align-items:center;justify-content:center;gap:14px}.empty,.error{padding:35px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:14px;color:rgb(var(--yb-text-muted))}.error{display:flex;align-items:center;justify-content:center;gap:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:950px){.filters{grid-template-columns:1fr 1fr}.filters button{grid-column:1/-1}}@media(max-width:700px){.groups-page{padding:16px}.groups-page>header{flex-direction:column;gap:12px}.filters{grid-template-columns:1fr}.filters button{grid-column:auto}.grid{grid-template-columns:1fr}}
</style>
