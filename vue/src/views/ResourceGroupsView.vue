<template>
  <main class="groups-page assets-index-view" data-surface-ready="true">
    <header class="page-heading yb-page-surface yb-page-header-row" data-page-header="asset-center">
      <div class="yb-page-heading-copy">
        <h1 class="yb-page-title">SKU 资产中心</h1>
        <p class="yb-page-subtitle">SKU 生成后立即可检索；图片、来源任务、规格、组合关系与系统计算成本在同一张卡片持续补齐。</p>
      </div>
      <div class="page-actions">
        <button class="primary-button" @click="packageOpen = true">生产打包</button>
        <button class="quiet-button" :disabled="loading" @click="load">
          {{ loading ? '刷新中…' : '刷新' }}
        </button>
      </div>
    </header>

    <section class="search-workbench" aria-label="资产检索">
      <form class="search-row" @submit.prevent="search">
        <label class="search-field">
          <span class="search-icon" aria-hidden="true">⌕</span>
          <input v-model.trim="filters.q" aria-label="搜索资产" placeholder="搜索 SKU、产品名称或文件名" />
        </label>
        <button class="primary-button">搜索</button>
        <button type="button" class="filter-button" :class="{ active: filterDrawerOpen || activeFilters.length }" @click="filterDrawerOpen = true">
          筛选<span v-if="activeFilters.length"> {{ activeFilters.length }}</span>
        </button>
      </form>
      <div v-if="activeFilters.length" class="active-filters" aria-label="已应用筛选">
        <button v-for="item in activeFilters" :key="item.key" type="button" @click="clearFilter(item.key)">
          {{ item.label }} <span aria-hidden="true">×</span>
        </button>
        <button type="button" class="clear-filters" @click="clearAllFilters">清除全部</button>
      </div>
    </section>

    <div v-if="error" class="error" role="alert"><span>{{ error }}</span><button @click="load">重试</button></div>
    <div v-if="loading && !result.items.length && !result.flat_items.length" class="empty loading-state" role="status">
      <span class="loading-spinner" aria-hidden="true" />正在检索 SKU 与资源…
    </div>
    <div v-else-if="loading" class="results-refreshing" role="status"><span class="loading-spinner" aria-hidden="true" />正在更新结果，当前列表仍可查看</div>

    <template v-if="!loading || result.items.length || result.flat_items.length">
    <template v-if="isFlatMode">
      <div v-if="!result.flat_items.length" class="empty">没有找到符合条件的资源。</div>
      <section v-else class="grid flat-grid" aria-label="匹配资源列表">
        <button v-for="(item, index) in result.flat_items" :key="`${item.group_id}-${item.resource_role}-${index}`" class="resource-card flat-card" @click="openGroup(item.group_id)">
          <span class="cover">
            <img v-if="item.preview_url && !brokenFlat.has(index)" :src="item.preview_url" :alt="item.file_name" loading="lazy" @error="markFlatBroken(index)" />
            <span v-else class="preview-fallback"><span class="file-mark">{{ fileInitial(item.file_name) }}</span><small>暂无预览</small></span>
            <span class="mode-badge">{{ roleLabel(item.resource_role) }}</span>
          </span>
            <span class="card-body">
              <strong class="sku-code">{{ item.sku_code || '未绑定 SKU' }}</strong>
              <span class="product-name">{{ item.file_name }}</span>
              <span class="provenance">来源任务 · {{ item.task_no || item.task_id }}</span>
              <span class="provenance">{{ item.resource_owner_name || '资源所属人待补充' }} · {{ formatResourceDate(item.resource_created_at) }}</span>
            </span>
        </button>
      </section>
    </template>

    <template v-else>
      <div v-if="!result.items.length" class="empty">没有找到符合条件的 SKU 资源。</div>
      <section v-else class="grid" aria-label="SKU 资源列表">
        <button v-for="group in result.items" :key="group.id" class="resource-card sku-asset-card" @click="openGroup(group.id)">
          <span class="cover">
            <AssetPreviewMedia
              v-if="protectedCoverURL(group) && !brokenImages.has(group.id)"
              :asset-id="String(coverAssetID(group))"
              :fallback-src="protectedCoverURL(group)"
              :alt="coverName(group)"
              img-class="resource-cover-media"
              inner-img-class="resource-cover-image"
              defer-until-visible
            />
            <img v-else-if="coverURL(group) && !brokenImages.has(group.id)" :src="coverURL(group)" :alt="coverName(group)" loading="lazy" @error="markImageBroken(group.id)" />
            <span v-else class="preview-fallback"><span class="file-mark">{{ fileInitial(coverName(group)) }}</span><small>暂无成品预览</small></span>
            <span class="mode-badge">{{ revision(group)?.mode === 'set' ? '套装' : '单图' }}</span>
            <span class="item-count">{{ finals(group).length }} 张成品</span>
          </span>
          <span class="card-body sku-card-body">
            <span class="sku-heading">
              <span>
                <strong class="sku-code">{{ displaySKU(group) }}</strong>
                <span class="product-name">{{ productTitle(group) }}</span>
              </span>
              <span class="sync-pill" :class="syncTone(group)">{{ syncLabel(group) }}</span>
            </span>
            <span class="sku-origin-line">
              <span>{{ laneLabel(group.business_lane) || '常规任务' }}</span>
              <span>任务 {{ group.task_no || group.task_id }}</span>
              <span>{{ group.creator_name || '创建人待补充' }}</span>
            </span>
            <span class="sku-facts" :class="{ 'has-erp-cost': liveCost(group.id) }">
              <span><small>规格 / 尺寸</small><b>{{ specificationText(group) }}</b></span>
              <span><small>计价面积</small><b>{{ areaText(group) }}</b></span>
              <span><small>系统计算成本</small><b>{{ costText(group) }}</b></span>
              <span v-if="liveCost(group.id)">
                <small>ERP 实际成本</small>
                <b :class="{ 'cost-mismatch': liveCost(group.id)?.status === 'mismatched' }">{{ erpCostText(liveCost(group.id)) }}</b>
              </span>
            </span>
            <span v-if="comboText(group)" class="combo-line"><small>组合编码</small><b>{{ comboText(group) }}</b></span>
            <span class="cost-rule-line">{{ costRuleText(group) }}</span>
            <span v-if="group.migration_incomplete" class="warn">资源待人工确认</span>
          </span>
        </button>
      </section>
    </template>
    </template>

    <nav v-if="result.total > result.page_size" class="pager" aria-label="资源分页">
      <button :disabled="loading || result.page <= 1" @click="goPage(result.page - 1)">上一页</button>
      <span>第 {{ result.page }} / {{ totalPages }} 页 · 共 {{ result.total }} {{ isFlatMode ? '项资源' : '个 SKU' }}</span>
      <button :disabled="loading || result.page >= totalPages" @click="goPage(result.page + 1)">下一页</button>
    </nav>

    <Teleport to="body">
      <div v-if="filterDrawerOpen" class="drawer-layer" @keydown.esc="filterDrawerOpen = false">
        <button class="drawer-backdrop" aria-label="关闭筛选" @click="filterDrawerOpen = false" />
        <aside class="filter-drawer" role="dialog" aria-modal="true" aria-labelledby="asset-filter-title">
          <header><div><p class="eyebrow">缩小结果范围</p><h2 id="asset-filter-title">资产筛选</h2></div><button class="icon-button" aria-label="关闭" @click="filterDrawerOpen = false">×</button></header>
          <form @submit.prevent="searchFromDrawer">
            <section class="filter-section">
              <h3>资源属性</h3>
              <label>资源类型<select v-model="filters.resource_role"><option value="">全部资源</option><option value="reference">参考图</option><option value="source">设计源文件</option><option value="final">最终成品</option></select></label>
              <label>资源文件格式<select v-model="filters.file_format"><option value="">全部格式</option><option value="jpg">JPG</option><option value="jpeg">JPEG</option><option value="png">PNG</option><option value="tif">TIF</option><option value="tiff">TIFF</option><option value="psd">PSD</option><option value="ai">AI</option><option value="pdf">PDF</option><option value="zip">ZIP</option></select></label>
              <label>资源创建开始时间<input v-model="filters.created_from" type="date" /></label>
              <label>资源创建结束时间<input v-model="filters.created_to" type="date" /></label>
              <label>资源所属人<select v-model="filters.resource_owner_id"><option value="">全部人员</option><option v-for="item in creatorOptions" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
            </section>
            <footer><button type="button" class="quiet-button" @click="resetDrawer">重置</button><button class="primary-button">应用筛选</button></footer>
          </form>
        </aside>
      </div>
    </Teleport>
    <ProductionPackageDialog :open="packageOpen" @close="packageOpen = false" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import ProductionPackageDialog from '@/components/assets/ProductionPackageDialog.vue'
import { useTaskFilterOptions } from '@/composables/useTaskFilterOptions'
import { resourceGroupsApi, type FlatResourceItem, type ProductCostReconciliation, type ResourceGroup, type ResourceRevision } from '@/services/api/resourceGroupsApi'

type FilterKey = 'resource_role' | 'file_format' | 'created_from' | 'created_to' | 'resource_owner_id'

const router = useRouter()
const loading = ref(false)
const error = ref('')
const filterDrawerOpen = ref(false)
const packageOpen = ref(false)
const brokenImages = ref(new Set<number>())
const brokenFlat = ref(new Set<number>())
const liveCosts = ref(new Map<number, ProductCostReconciliation>())
let liveCostRequestVersion = 0
const filters = reactive({ q: '', resource_role: '' as '' | 'reference' | 'source' | 'final', file_format: '', created_from: '', created_to: '', resource_owner_id: '' })
const initialPageSize = import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true' ? Math.max(80, Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE || 100)) : 24
const result = reactive({ items: [] as ResourceGroup[], flat_items: [] as FlatResourceItem[], view_mode: 'group' as 'group' | 'flat', page: 1, page_size: initialPageSize, total: 0 })
const { creatorOptions: rawCreatorOptions } = useTaskFilterOptions(true, '全部')
const creatorOptions = computed(() => rawCreatorOptions.value.filter((item) => item.value))
const totalPages = computed(() => Math.max(1, Math.ceil(result.total / result.page_size)))
const isFlatMode = computed(() => result.view_mode === 'flat' || !!filters.resource_role || !!filters.file_format || !!filters.created_from || !!filters.created_to || !!filters.resource_owner_id)
const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const finals = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const laneLabel = (lane?: string) => ({ normal: '常规', customization: '定制' }[lane || ''] || '')
const roleLabel = (role: string) => ({ reference: '参考图', source: '源文件', final: '最终成品' }[role] || role)
const displaySKU = (group: ResourceGroup) => group.sku_code || group.sku_profile?.sku_code || 'SKU 待关联'
const productTitle = (group: ResourceGroup) => group.sku_profile?.product_name || group.product_name || (group.scope_kind === 'retouch_requirement' ? '修图成品' : '未命名产品')
const coverName = (group: ResourceGroup) => finals(group)[0]?.file?.file_name || finals(group)[0]?.item_name || group.product_name || '资源'
const coverURL = (group: ResourceGroup) => finals(group)[0]?.file?.preview_url || finals(group)[0]?.file?.download_url || ''
const protectedCoverURL = (group: ResourceGroup) => coverURL(group).startsWith('/') ? coverURL(group) : ''
const coverAssetID = (group: ResourceGroup) => finals(group)[0]?.file?.task_asset_id || finals(group)[0]?.task_asset_id || 0
const specificationText = (group: ResourceGroup) => group.sku_profile?.size_text || group.sku_profile?.spec_text || '规格待补充'
const areaText = (group: ResourceGroup) => {
  const area = group.sku_profile?.area_trace?.area_m2
  return typeof area === 'number' && Number.isFinite(area) ? `${area.toFixed(3)} ㎡` : '面积待核对'
}
const costText = (group: ResourceGroup) => {
  const cost = group.sku_profile?.cost_price
  return typeof cost === 'number' && Number.isFinite(cost) ? `¥ ${cost.toFixed(2)}` : '成本待计算'
}
const liveCost = (groupID: number) => liveCosts.value.get(groupID)
const erpCostText = (cost?: ProductCostReconciliation) => {
  if (typeof cost?.erp_cost_price === 'number' && Number.isFinite(cost.erp_cost_price)) return `¥ ${cost.erp_cost_price.toFixed(2)}`
  return cost?.status === 'unavailable' ? 'ERP 查询失败' : 'ERP 未维护'
}
const comboText = (group: ResourceGroup) => (group.sku_profile?.combo_sku_codes || []).join('、')
const costRuleText = (group: ResourceGroup) => {
  const trace = group.sku_profile?.cost_trace
  if (trace?.requires_manual_review) return '成本需要人工确认'
  if (trace?.rule_name) return `成本规则 · ${trace.rule_name}${trace.matched_rule_version ? `（第 ${trace.matched_rule_version} 版）` : ''}`
  return '尚未关联成本规则'
}
const syncLabel = (group: ResourceGroup) => ({ synced: 'ERP 已同步', queued: '等待同步', syncing: '正在同步', failed: '同步失败', cooling_down: '稍后重试', pending_sync: '待同步' }[group.sku_profile?.erp_sync_status || ''] || 'ERP 待关联')
const syncTone = (group: ResourceGroup) => group.sku_profile?.erp_sync_status === 'synced' ? 'is-synced' : group.sku_profile?.erp_sync_status === 'failed' ? 'is-failed' : 'is-pending'
const fileInitial = (name?: string) => (name || '文件').split('.').pop()?.slice(0, 4).toUpperCase() || 'FILE'
const formatResourceDate = (value?: string) => value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium' }).format(new Date(value)) : '时间待补充'
const filterLabels: Record<FilterKey, string> = { resource_role: '资源类型', file_format: '文件格式', created_from: '创建开始', created_to: '创建结束', resource_owner_id: '资源所属人' }
const activeFilters = computed(() => (Object.keys(filterLabels) as FilterKey[]).flatMap((key) => {
  const value = String(filters[key] || '')
  if (!value) return []
  const display = key === 'resource_owner_id' ? creatorOptions.value.find((item) => item.value === value)?.label || value : key === 'resource_role' ? roleLabel(value) : value
  return [{ key, label: `${filterLabels[key]}：${display}` }]
}))

function markImageBroken(id: number) { brokenImages.value = new Set(brokenImages.value).add(id) }
function markFlatBroken(index: number) { brokenFlat.value = new Set(brokenFlat.value).add(index) }
function openGroup(id: number) { void router.push(`/asset-center/${id}`) }
function clearFilter(key: FilterKey) { filters[key] = ''; search() }
function clearAllFilters() { (Object.keys(filterLabels) as FilterKey[]).forEach((key) => { filters[key] = '' }); search() }
function resetDrawer() { (Object.keys(filterLabels) as FilterKey[]).forEach((key) => { filters[key] = '' }) }
function searchFromDrawer() { filterDrawerOpen.value = false; search() }
async function load() {
  const requestVersion = ++liveCostRequestVersion
  liveCosts.value = new Map()
  loading.value = true
  error.value = ''
  try {
    const exactSKUQuery = /^(?=.*\d)[A-Z0-9_-]{5,}$/i.test(filters.q)
    const next = await resourceGroupsApi.list({ q: exactSKUQuery ? undefined : filters.q || undefined, sku_code: exactSKUQuery ? filters.q.toUpperCase() : undefined, resource_role: filters.resource_role || undefined, file_format: filters.file_format || undefined, created_from: filters.created_from || undefined, created_to: filters.created_to || undefined, resource_owner_id: filters.resource_owner_id || undefined, page: result.page, page_size: result.page_size })
    result.items = next.items || []; result.flat_items = next.flat_items || []; result.view_mode = next.view_mode || (isFlatMode.value ? 'flat' : 'group'); result.page = next.page; result.page_size = next.page_size; result.total = next.total
    brokenImages.value = new Set(); brokenFlat.value = new Set()
    if (exactSKUQuery && result.view_mode === 'group' && result.items.length <= 20) void refreshLiveCosts(result.items, requestVersion)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '资产中心加载失败。' } finally { loading.value = false }
}
async function refreshLiveCosts(groups: ResourceGroup[], requestVersion: number) {
  await Promise.all(groups.map(async (group) => {
    try {
      const cost = await resourceGroupsApi.costReconciliation(group.id)
      if (requestVersion !== liveCostRequestVersion) return
      const next = new Map(liveCosts.value)
      next.set(group.id, cost)
      liveCosts.value = next
    } catch {
      // The card keeps the system calculation visible; ERP lookup failure is available in group details.
    }
  }))
}
function search() { result.page = 1; void load() }
function goPage(page: number) { result.page = Math.max(1, Math.min(totalPages.value, page)); void load() }
onMounted(load)
</script>

<style scoped>
.groups-page{max-width:1320px;margin:0 auto;padding:28px;display:grid;gap:20px}.page-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20px}.page-actions{display:flex;gap:10px;flex-wrap:wrap}.page-heading h1{margin:4px 0;font-size:31px}.page-heading p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{margin:0;font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}.quiet-button,.primary-button,.filter-button,.pager button,.error button,.icon-button{min-height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 14px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.primary-button{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.search-workbench{display:grid;gap:10px;padding:14px;border:1px solid rgb(var(--yb-border));border-radius:16px;background:rgb(var(--yb-surface))}.search-row{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:9px}.search-field{position:relative}.search-field input{width:100%;min-height:42px;border:1px solid rgb(var(--yb-border));border-radius:11px;padding:0 16px 0 40px;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text))}.search-icon{position:absolute;left:14px;top:9px;color:rgb(var(--yb-text-muted));font-size:20px}.filter-button.active{border-color:rgb(var(--yb-brand-border));background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.active-filters{display:flex;align-items:center;gap:7px;flex-wrap:wrap}.active-filters button{border:0;border-radius:999px;padding:6px 10px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));cursor:pointer}.active-filters .clear-filters{background:transparent;color:rgb(var(--yb-text-muted))}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:16px}.resource-card{min-width:0;padding:0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:16px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));text-align:left;cursor:pointer;transition:transform .18s,border-color .18s,box-shadow .18s}.resource-card:hover,.resource-card:focus-visible{transform:translateY(-3px);border-color:rgb(var(--yb-brand-border));box-shadow:0 12px 30px rgb(var(--yb-shadow)/.1);outline:2px solid rgb(var(--yb-brand-soft));outline-offset:2px}.cover{position:relative;display:block;aspect-ratio:16/11;background:rgb(var(--yb-surface-muted))}.cover img,.preview-fallback{width:100%;height:100%;object-fit:cover}.preview-fallback{display:grid;place-items:center;align-content:center;gap:6px;color:rgb(var(--yb-text-muted))}.file-mark{font-size:18px;font-weight:900}.preview-fallback small{font-size:11px}.mode-badge,.item-count{position:absolute;top:10px;padding:5px 8px;border-radius:999px;background:rgb(var(--yb-surface)/.94);font-size:11px;font-weight:800}.mode-badge{left:10px;color:rgb(var(--yb-brand))}.item-count{right:10px;color:rgb(var(--yb-text))}.card-body{display:grid;gap:7px;padding:14px}.sku-code{font-size:13px;color:rgb(var(--yb-brand))}.product-name{min-height:22px;font-weight:800;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.card-meta{display:flex;gap:8px;font-size:12px;color:rgb(var(--yb-text-muted))}.card-meta span+span::before{content:'·';margin-right:8px}.provenance{font-size:11px;color:rgb(var(--yb-text-faint))}.warn{font-size:12px;color:rgb(var(--yb-warning-text))}.pager{display:flex;align-items:center;justify-content:center;gap:14px}.empty,.error{padding:38px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:16px;color:rgb(var(--yb-text-muted))}.error{display:flex;align-items:center;justify-content:center;gap:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.drawer-layer{position:fixed;inset:0;z-index:90;display:flex;justify-content:flex-end}.drawer-backdrop{position:absolute;inset:0;border:0;background:rgb(var(--yb-overlay-night)/.42)}.filter-drawer{position:relative;width:min(390px,100vw);height:100%;display:grid;grid-template-rows:auto 1fr;background:rgb(var(--yb-surface));box-shadow:-18px 0 48px rgb(var(--yb-shadow)/.18)}.filter-drawer>header{display:flex;align-items:center;justify-content:space-between;padding:22px;border-bottom:1px solid rgb(var(--yb-border))}.filter-drawer h2{margin:3px 0 0}.icon-button{width:40px;padding:0;font-size:24px}.filter-drawer form{min-height:0;display:grid;grid-template-rows:1fr auto;overflow:auto}.filter-section{display:grid;grid-template-columns:1fr;gap:14px;padding:22px}.filter-section h3{margin:0;font-size:14px}.filter-section label{display:grid;gap:6px;font-size:12px;color:rgb(var(--yb-text-muted))}.filter-section input,.filter-section select{box-sizing:border-box;width:100%;height:42px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 11px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.filter-drawer footer{position:sticky;bottom:0;display:flex;justify-content:flex-end;gap:10px;padding:16px 22px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}@media(max-width:700px){.groups-page{padding:16px}.page-heading{align-items:stretch;flex-direction:column}.search-row{grid-template-columns:1fr auto}.search-row .primary-button{display:none}.grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.card-body{padding:11px}.product-name{font-size:13px}}@media(max-width:430px){.grid{grid-template-columns:1fr 1fr}.mode-badge,.item-count{top:7px}.mode-badge{left:7px}.item-count{right:7px}.provenance,.card-meta{font-size:10px}}
</style>
<style scoped>
.results-refreshing{position:sticky;top:.5rem;z-index:4;display:flex;align-items:center;justify-content:center;gap:.55rem;width:max-content;max-width:calc(100% - 2rem);margin:-.25rem auto;padding:.55rem .85rem;border:1px solid rgb(var(--yb-brand-border));border-radius:999px;background:rgb(var(--yb-surface)/.94);color:rgb(var(--yb-brand));font-size:.76rem;font-weight:750;box-shadow:0 .5rem 1.4rem rgb(var(--yb-shadow)/.1);backdrop-filter:blur(12px)}.loading-state{display:flex;align-items:center;justify-content:center;gap:.65rem}.loading-spinner{width:1rem;height:1rem;border:2px solid rgb(var(--yb-brand-border));border-top-color:rgb(var(--yb-brand));border-radius:50%;animation:asset-search-spin .75s linear infinite}@keyframes asset-search-spin{to{transform:rotate(360deg)}}
</style>
<style scoped>
.groups-page {
  max-width: none;
  margin: 0;
  padding: 0;
  gap: 1rem;
}

.page-heading h1 {
  margin: 0;
  font-size: clamp(1.75rem, 1.8vw, 2.25rem);
}

.page-heading .yb-page-subtitle {
  margin: 0.5rem 0 0;
  color: rgb(var(--yb-text-muted));
}

.grid:not(.flat-grid) {
  grid-template-columns: repeat(auto-fill, minmax(19rem, 1fr));
}

.sku-asset-card {
  display: grid;
  grid-template-rows: auto 1fr;
}

:deep(.resource-cover-media) {
  width: 100%;
  height: 100%;
}

:deep(.resource-cover-media .resource-cover-image),
:deep(.resource-cover-media .apm-img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.sku-card-body {
  gap: 0.8rem;
}

.sku-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.sku-heading > span:first-child {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.sku-code {
  font-size: 0.82rem;
  letter-spacing: 0.035em;
}

.product-name {
  min-height: 0;
  font-size: 1rem;
}

.sync-pill {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 0.32rem 0.55rem;
  font-size: 0.68rem;
  font-weight: 800;
  color: rgb(var(--yb-text-muted));
  background: rgb(var(--yb-surface-muted));
}

.sync-pill.is-synced {
  color: rgb(var(--yb-success-text));
  background: rgb(var(--yb-success-soft));
}

.sync-pill.is-failed {
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
}

.sku-origin-line {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem 0.75rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.72rem;
}

.sku-origin-line span + span::before {
  content: '·';
  margin-right: 0.75rem;
  color: rgb(var(--yb-text-faint));
}

.sku-facts {
  display: grid;
  grid-template-columns: 1.3fr 0.9fr 0.85fr;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-soft));
}

.sku-facts.has-erp-cost {
  grid-template-columns: 1.25fr 0.85fr 0.85fr 0.85fr;
}

.sku-facts > span {
  min-width: 0;
  display: grid;
  gap: 0.25rem;
  padding: 0.65rem 0.7rem;
}

.sku-facts > span + span {
  border-left: 1px solid rgb(var(--yb-border));
}

.sku-facts small,
.combo-line small {
  color: rgb(var(--yb-text-muted));
  font-size: 0.65rem;
}

.sku-facts b,
.combo-line b {
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sku-facts b.cost-mismatch {
  color: rgb(var(--yb-danger-text));
}

.combo-line {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.cost-rule-line {
  color: rgb(var(--yb-text-muted));
  font-size: 0.7rem;
}

@media (max-width: 700px) {
  .groups-page {
    padding: 0;
  }

  .page-heading {
    align-items: flex-start;
  }

  .search-row {
    grid-template-columns: minmax(0, 1fr) auto auto;
  }

  .search-row .primary-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .search-field {
    min-width: 0;
  }

  .search-field input {
    box-sizing: border-box;
  }

  .grid:not(.flat-grid) {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 430px) {
  .search-row {
    grid-template-columns: 1fr 1fr;
  }

  .search-field {
    grid-column: 1 / -1;
  }

  .search-row .primary-button,
  .search-row .filter-button {
    width: 100%;
  }
}
</style>
