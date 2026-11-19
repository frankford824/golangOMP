<template>
  <main class="groups-page assets-index-view" data-surface-ready="true">
    <header class="page-heading yb-page-surface yb-page-header-row" data-page-header="asset-center">
      <div class="yb-page-heading-copy">
        <h1 class="yb-page-title">SKU 资产中心</h1>
      </div>
      <DesktopSlot target="actions"><div class="page-actions">
        <div v-if="oaEnabled && !isFlatMode" class="asset-view-modes" role="group" aria-label="结果视图">
          <button type="button" class="quiet-button" :aria-pressed="!showLedger" @click="setLedger(false)">图库网格</button>
          <button type="button" class="quiet-button" :aria-pressed="showLedger" @click="setLedger(true)">规格台账</button>
        </div>
        <button class="quiet-button asset-refresh" :disabled="loading" @click="load()">
          {{ loading ? '刷新中…' : '刷新' }}
        </button>
      </div></DesktopSlot>
    </header>

    <section class="search-workbench" aria-label="资产检索">
      <form class="search-row" @submit.prevent="search">
        <label class="search-field">
          <span class="search-icon" :class="{ 'material-symbols-outlined': oaEnabled }" aria-hidden="true">{{ oaEnabled ? 'search' : '⌕' }}</span>
          <input v-model.trim="filters.q" aria-label="搜索资产" placeholder="搜索 SKU、产品名称或文件名" />
        </label>
        <button class="primary-button">搜索</button>
        <button type="button" class="filter-button" :class="{ active: filterDrawerOpen || activeFilters.length }" @click="openFilterDrawer()">
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

    <div ref="resultsViewport" class="asset-results-region desktop-region-scroll" role="region" aria-label="资产检索结果" tabindex="0">
    <div v-if="error" class="error" role="alert"><span>{{ error }}</span><button @click="load()">重试</button></div>
    <div v-if="loading && !result.items.length && !result.flat_items.length && !externalItems.length" class="empty loading-state" role="status">
      <span class="loading-spinner" aria-hidden="true" />正在检索 SKU 与资源…
    </div>
    <div v-else-if="loading" class="results-refreshing" role="status"><span class="loading-spinner" aria-hidden="true" />正在更新结果，当前列表仍可查看</div>

    <template v-if="!loading || result.items.length || result.flat_items.length">
    <template v-if="isFlatMode">
      <OAEmpty v-if="oaEnabled && !result.flat_items.length && !error" kind="files" title="没有找到符合条件的资源" description="可调整搜索词或筛选条件，再查看结果。" />
      <div v-else-if="!result.flat_items.length && !error" class="empty">没有找到符合条件的资源。</div>
      <section v-else-if="result.flat_items.length" class="grid asset-grid flat-grid" aria-label="匹配资源列表">
        <component :is="oaEnabled ? RouterLink : 'button'" v-for="(item, index) in result.flat_items" :key="`${item.group_id}-${item.resource_role}-${index}`" class="resource-card flat-card" :to="oaEnabled ? assetDetailTarget(item.group_id) : undefined" @click="openLegacyAsset(item.group_id)">
          <span class="cover">
            <AssetPreviewMedia v-if="item.task_asset_id && (item.resource_role !== 'default_style' || item.preview_url)" :task-asset-id="String(item.task_asset_id)" :alt="item.file_name" :quiet-loading="oaEnabled" :show-recovery-actions="!oaEnabled" rendition="thumbnail" :open-full-on-click="false" />
            <span v-else class="preview-fallback"><ImageIcon v-if="oaEnabled" :size="28" aria-hidden="true" /><span v-else class="file-mark">{{ fileInitial(item.file_name) }}</span><small>暂无预览</small></span>
            <span class="mode-badge">{{ roleLabel(item.resource_role) }}</span>
          </span>
            <span class="card-body">
              <strong class="sku-code">{{ item.sku_code || '未绑定 SKU' }}</strong>
              <span class="product-name">{{ item.file_name }}</span>
              <span class="provenance">来源任务 · {{ item.task_no || item.task_id }}</span>
              <span class="provenance">{{ item.resource_owner_name || '资源所属人待补充' }} · {{ formatResourceDate(item.resource_created_at) }}</span>
            </span>
        </component>
      </section>
    </template>

    <template v-else>
      <OAEmpty v-if="oaEnabled && !result.items.length && !externalItems.length && !error" kind="files" title="没有找到符合条件的 SKU 资源" description="可调整搜索词或筛选条件，再查看结果。" />
      <div v-else-if="!result.items.length && !externalItems.length && !error" class="empty">没有找到符合条件的 SKU 资源。</div>
      <div v-else-if="oaEnabled && showLedger && result.items.length" class="asset-ledger-scroll" role="region" aria-label="SKU 规格台账" tabindex="0">
        <table class="asset-ledger">
          <thead>
            <tr><th scope="col">SKU / 产品</th><th scope="col">规格 / 尺寸</th><th scope="col">计价面积</th><th scope="col" class="ledger-amount">系统计算成本</th><th scope="col" class="ledger-amount">ERP 实际成本</th><th scope="col">同步状态</th><th scope="col">操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="group in result.items" :key="group.id">
              <td><RouterLink :to="assetDetailTarget(group.id)" class="sku-code">{{ displaySKU(group) }}</RouterLink><span :title="productTitle(group)">{{ productTitle(group) }}</span></td>
              <td>{{ group.sku_profile?.size_text || group.sku_profile?.spec_text || '—' }}</td>
              <td>{{ group.sku_profile?.area_trace?.area_m2 == null ? '—' : areaText(group) }}</td>
              <td class="ledger-amount">{{ group.sku_profile?.cost_price == null ? '—' : costText(group) }}</td>
              <td class="ledger-amount">{{ liveCost(group.id) ? erpCostText(liveCost(group.id)) : '—' }}</td>
              <td><span class="sync-pill" :class="syncTone(group)">{{ syncLabel(group) }}</span></td>
              <td><button type="button" class="quiet-button ledger-preview-button" @click="previewGroupID = group.id">速览</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <section v-else-if="result.items.length || (!oaEnabled && externalItems.length)" class="grid asset-grid" aria-label="SKU 资源列表">
        <component :is="oaEnabled ? RouterLink : 'button'" v-for="group in result.items" :key="group.id" class="resource-card sku-asset-card" :aria-label="oaEnabled ? `查看 ${displaySKU(group)} ${productTitle(group)} 资产详情` : undefined" :to="oaEnabled ? assetDetailTarget(group.id) : undefined" @click="openLegacyAsset(group.id)">
          <span class="cover">
            <AssetPreviewMedia
              v-if="coverAssetID(group) > 0"
              :task-asset-id="String(coverAssetID(group))"
              :open-full-on-click="false"
              :alt="coverName(group)"
              img-class="resource-cover-media"
              inner-img-class="resource-cover-image"
              :quiet-loading="oaEnabled"
              :show-recovery-actions="!oaEnabled"
              defer-until-visible
            />
            <template v-else-if="oaEnabled && coverReference(group)">
              <AssetPreviewMedia
                :task-asset-id="coverReference(group)?.formal_task_asset_id?.toString()"
                :resolved-preview-url="referencePreviewURL(group)"
                :alt="coverReference(group)?.file_name || '参考图'"
                :open-full-on-click="false"
                img-class="resource-cover-media"
                inner-img-class="resource-cover-image"
                quiet-loading
                :show-recovery-actions="false"
                defer-until-visible
              />
              <span class="mode-badge reference-badge">参考图</span>
            </template>
            <span v-else-if="oaEnabled" class="preview-fallback"><ImageIcon :size="28" aria-hidden="true" /><small>暂无图片</small></span>
            <span v-else class="preview-fallback"><span class="file-mark">{{ fileInitial(coverName(group)) }}</span><small>暂无成品预览</small></span>
            <span v-if="!oaEnabled || coverAssetID(group) || !coverReference(group)" class="mode-badge">{{ revision(group)?.mode === 'set' ? '套装' : '单图' }}</span>
            <span class="item-count">{{ revision(group)?.final_output_waived ? '无需成品图' : `${finals(group).length} 张成品` }}</span>
          </span>
          <span class="card-body sku-card-body">
            <span class="sku-heading">
              <strong class="sku-code">{{ displaySKU(group) }}</strong>
              <span class="sync-pill" :class="syncTone(group)">{{ syncLabel(group) }}</span>
              <span class="product-name" :title="productTitle(group)">{{ productTitle(group) }}</span>
            </span>
            <span class="sku-origin-line">
              <span>{{ laneLabel(group.business_lane) || '常规任务' }}</span>
              <span>任务 {{ group.task_no || group.task_id }}</span>
              <span>{{ group.creator_name || '创建人待补充' }}</span>
              <span v-if="group.default_style?.file">有默认款</span>
            </span>
            <span class="sku-facts" :class="{ 'has-erp-cost': liveCost(group.id) }">
              <span><small>规格 / 尺寸</small><b>{{ specificationText(group) }}</b></span>
              <span><small>计价面积</small><b>{{ areaText(group) }}</b></span>
              <span><small>系统成本</small><b>{{ costText(group) }}</b></span>
              <span v-if="liveCost(group.id)">
                <small>ERP 实际成本</small>
                <b :class="{ 'cost-mismatch': liveCost(group.id)?.status === 'mismatched' }">{{ erpCostText(liveCost(group.id)) }}</b>
              </span>
            </span>
            <span v-if="comboText(group)" class="combo-line"><small>组合编码</small><b>{{ comboText(group) }}</b></span>
            <span class="cost-rule-line">{{ costRuleText(group) }}</span>
            <span v-if="group.migration_incomplete" class="warn">资源待人工确认</span>
          </span>
        </component>
      </section>
      <section v-if="externalItems.length" class="external-results" aria-label="外部资源匹配">
        <header>
          <div><strong>外部资源匹配</strong><span>ERP / OSS / NAS 已索引资源</span></div>
          <span>{{ externalItems.length }} 个文件</span>
        </header>
        <div class="grid asset-grid external-grid">
          <component :is="oaEnabled ? RouterLink : 'button'" v-for="asset in externalItems" :key="externalResourceID(asset)" class="resource-card external-card" :to="oaEnabled ? assetDetailTarget(externalResourceID(asset)) : undefined" @click="openLegacyAsset(externalResourceID(asset))">
            <span class="cover external-cover">
              <AssetPreviewMedia
                v-if="externalResourceID(asset)"
                :asset-id="externalResourceID(asset)"
                :alt="externalFileName(asset)"
                img-class="external-cover-media"
                :quiet-loading="oaEnabled"
                :show-recovery-actions="!oaEnabled"
                rendition="thumbnail"
                :open-full-on-click="false"
              />
              <span v-else class="preview-fallback"><ImageIcon v-if="oaEnabled" :size="28" aria-hidden="true" /><span v-else class="file-mark">{{ fileInitial(externalFileName(asset)) }}</span><small>外部资源</small></span>
              <span class="mode-badge">外部</span>
            </span>
            <span class="card-body">
              <strong class="sku-code">{{ asset.sku_code || asset.primary_sku_code || applied.q }}</strong>
              <span class="product-name">{{ externalFileName(asset) }}</span>
              <span class="provenance">{{ asset.source_label || '外部资源' }} · {{ asset.oss_sync_status === 'ready' ? 'OSS 已就绪' : '只读索引' }}</span>
            </span>
          </component>
        </div>
      </section>
    </template>
    </template>
    </div>

    <nav v-if="result.total > result.page_size" class="pager" aria-label="资源分页">
      <span>共 {{ result.total }} {{ isFlatMode ? '项资源' : '个 SKU' }}</span>
      <div class="pager-controls">
        <button type="button" :disabled="loading || result.page <= 1" @click="goPage(result.page - 1)">上一页</button>
        <span>第 {{ result.page }} / {{ totalPages }} 页</span>
        <button type="button" :disabled="loading || result.page >= totalPages" @click="goPage(result.page + 1)">下一页</button>
      </div>
    </nav>

    <Teleport to="body">
      <div v-if="previewGroup" class="drawer-layer">
        <button class="drawer-backdrop" tabindex="-1" aria-label="关闭资产速览" @click="previewGroupID = null" />
        <aside ref="previewDrawerPanel" class="filter-drawer asset-preview-drawer" role="dialog" aria-modal="true" aria-labelledby="asset-preview-title" @keydown.up.prevent="stepPreview(-1)" @keydown.down.prevent="stepPreview(1)">
          <header>
            <h2 id="asset-preview-title">资产速览</h2>
            <button type="button" class="icon-button" aria-label="关闭资产速览" @click="previewGroupID = null">×</button>
          </header>
          <div class="asset-preview-content">
            <h3>{{ displaySKU(previewGroup) }}</h3>
            <p>{{ productTitle(previewGroup) }}</p>
            <div v-if="coverAssetID(previewGroup) || coverReference(previewGroup)" class="preview-media-frame">
              <AssetPreviewMedia
                :task-asset-id="coverAssetID(previewGroup) ? String(coverAssetID(previewGroup)) : coverReference(previewGroup)?.formal_task_asset_id?.toString()"
                :resolved-preview-url="coverAssetID(previewGroup) ? undefined : referencePreviewURL(previewGroup)"
                :alt="coverAssetID(previewGroup) ? coverName(previewGroup) : coverReference(previewGroup)?.file_name || '参考图'"
                :open-full-on-click="false"
                quiet-loading
              />
              <small v-if="!coverAssetID(previewGroup)">参考图</small>
            </div>
            <dl>
              <div><dt>规格 / 尺寸</dt><dd>{{ specificationText(previewGroup) }}</dd></div>
              <div><dt>计价面积</dt><dd>{{ areaText(previewGroup) }}</dd></div>
              <div><dt>系统计算成本</dt><dd>{{ costText(previewGroup) }}</dd></div>
              <div><dt>同步状态</dt><dd>{{ syncLabel(previewGroup) }}</dd></div>
            </dl>
            <RouterLink :to="assetDetailTarget(previewGroup.id)" @click.exact="previewGroupID = null">前往资产详情</RouterLink>
          </div>
          <footer>
            <button type="button" class="quiet-button" :disabled="previewGroupIndex <= 0" @click="stepPreview(-1)">上一项 ↑</button>
            <button type="button" class="quiet-button" :disabled="previewGroupIndex >= result.items.length - 1" @click="stepPreview(1)">下一项 ↓</button>
          </footer>
        </aside>
      </div>
      <div v-if="filterDrawerOpen" class="drawer-layer">
        <button class="drawer-backdrop" tabindex="-1" aria-label="关闭筛选" @click="filterDrawerOpen = false" />
        <aside ref="filterDrawerPanel" class="filter-drawer" role="dialog" aria-modal="true" aria-labelledby="asset-filter-title">
          <header><h2 id="asset-filter-title">资产筛选</h2><button class="icon-button" aria-label="关闭" @click="filterDrawerOpen = false">×</button></header>
          <form @submit.prevent="searchFromDrawer">
            <section class="filter-section">
              <h3>资源属性</h3>
              <label>资源类型<select v-model="filters.resource_role"><option value="">全部资源</option><option value="reference">参考图</option><option value="source">设计源文件</option><option value="final">最终成品</option><option value="default_style">默认款</option></select></label>
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
  </main>
</template>

<script setup lang="ts">
import DesktopSlot from '@/oa/components/DesktopSlot.vue'
import OAEmpty from '@/oa/components/OAEmpty.vue'
import { computed, nextTick, onActivated, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter, type LocationQuery } from 'vue-router'
import { ImageIcon } from 'lucide-vue-next'
import { oaEnabled } from '@/oa/access'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import { useTaskFilterOptions } from '@/composables/useTaskFilterOptions'
import { useModalA11y } from '@/composables/useModalA11y'
import type { BackendAsset } from '@/services/apiTypes'
import { assetsApi, type AssetBatchSearchManifest } from '@/services/api/assetsApi'
import { resourceGroupsApi, type FlatResourceItem, type ProductCostReconciliation, type ResourceGroup, type ResourceRevision } from '@/services/api/resourceGroupsApi'

type FilterKey = 'resource_role' | 'file_format' | 'created_from' | 'created_to' | 'resource_owner_id'
const resourceRoles = ['reference', 'source', 'final', 'default_style'] as const
type ResourceRole = typeof resourceRoles[number] | ''

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const showLedger = ref(false)
const previewGroupID = ref<number | null>(null)
const previewDrawerPanel = ref<HTMLElement | null>(null)
const resultsViewport = ref<HTMLElement | null>(null)
const error = ref('')
const filterDrawerOpen = ref(false)
const filterDrawerPanel = ref<HTMLElement | null>(null)
useModalA11y(filterDrawerOpen, filterDrawerPanel, { onEsc: () => { filterDrawerOpen.value = false } })
const brokenImages = ref(new Set<number>())
const brokenFlat = ref(new Set<number>())
const liveCosts = ref(new Map<number, ProductCostReconciliation>())
const externalItems = ref<BackendAsset[]>([])
let liveCostRequestVersion = 0
const filters = reactive({ q: '', resource_role: '' as ResourceRole, file_format: '', created_from: '', created_to: '', resource_owner_id: '' })
const applied = reactive({ ...filters })
let requestedPage = 1
let hasLoadedQuery = false
const listPath = ref('/asset-center')
const initialPageSize = import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true' ? Math.max(80, Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE || 100)) : 24
const result = reactive({ items: [] as ResourceGroup[], flat_items: [] as FlatResourceItem[], view_mode: 'group' as 'group' | 'flat', page: 1, page_size: initialPageSize, total: 0 })
const previewGroupIndex = computed(() => result.items.findIndex(group => group.id === previewGroupID.value))
const previewGroup = computed(() => result.items[previewGroupIndex.value])
useModalA11y(computed(() => Boolean(previewGroup.value)), previewDrawerPanel, { onEsc: () => { previewGroupID.value = null } })
function stepPreview(direction: -1 | 1) {
  const group = result.items[previewGroupIndex.value + direction]
  if (group) previewGroupID.value = group.id
}
const { creatorOptions: rawCreatorOptions } = useTaskFilterOptions(true, '全部')
const creatorOptions = computed(() => rawCreatorOptions.value.filter((item) => item.value))
const totalPages = computed(() => Math.max(1, Math.ceil(result.total / result.page_size)))
const hasResourceFilters = computed(() => Boolean(applied.resource_role || applied.file_format || applied.created_from || applied.created_to || applied.resource_owner_id))
const isFlatMode = computed(() => result.view_mode === 'flat' || hasResourceFilters.value)
const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const finals = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const laneLabel = (lane?: string) => ({ normal: '常规', customization: '定制' }[lane || ''] || '')
const roleLabel = (role: string) => ({ reference: '参考图', source: '源文件', final: '最终成品', default_style: '默认款' }[role] || role)
const displaySKU = (group: ResourceGroup) => group.sku_code || group.sku_profile?.sku_code || 'SKU 待关联'
const productTitle = (group: ResourceGroup) => group.sku_profile?.product_name || group.product_name || (group.scope_kind === 'retouch_requirement' ? '修图成品' : '未命名产品')
const coverFinal = (group: ResourceGroup) => oaEnabled
  ? finals(group).find((item) => item.file?.availability !== 'historical_unavailable' && (item.file?.task_asset_id || item.task_asset_id))
  : finals(group)[0]
const coverName = (group: ResourceGroup) => coverFinal(group)?.file?.file_name || coverFinal(group)?.item_name || group.product_name || '资源'
const coverAssetID = (group: ResourceGroup) => coverFinal(group)?.file?.task_asset_id || coverFinal(group)?.task_asset_id || 0
const coverReference = (group: ResourceGroup) => revision(group)?.references?.find(
  (reference) => reference.availability !== 'historical_unavailable' && (reference.formal_task_asset_id || reference.preview_url),
)
function referencePreviewURL(group: ResourceGroup) {
  const reference = coverReference(group)
  return reference?.formal_task_asset_id ? undefined : reference?.preview_url
}
const specificationText = (group: ResourceGroup) => group.sku_profile?.size_text || group.sku_profile?.spec_text || '规格待补充'
const areaText = (group: ResourceGroup) => {
  const area = group.sku_profile?.area_trace?.area_m2
  return typeof area === 'number' && Number.isFinite(area) ? `${area.toFixed(3)} ㎡` : '面积待核对'
}
const costText = (group: ResourceGroup) => {
  const cost = group.sku_profile?.cost_price
  return typeof cost === 'number' && Number.isFinite(cost) ? `¥${cost.toFixed(2)}` : '成本待计算'
}
const liveCost = (groupID: number) => liveCosts.value.get(groupID)
const erpCostText = (cost?: ProductCostReconciliation) => {
  if (typeof cost?.erp_cost_price === 'number' && Number.isFinite(cost.erp_cost_price)) return `¥${cost.erp_cost_price.toFixed(2)}`
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
  const value = applied[key]
  if (!value) return []
  const display = key === 'resource_owner_id' ? creatorOptions.value.find((item) => item.value === value)?.label || value : key === 'resource_role' ? roleLabel(value) : value
  return [{ key, label: `${filterLabels[key]}：${display}` }]
}))

// Keep the selected row and scroll on the list's own browser history entry.
onBeforeRouteLeave((to) => {
  const viewport = resultsViewport.value
  if (!oaEnabled || !viewport || window.history.state?.current !== route.fullPath || !['AssetDetail', 'ExternalAssetDetail'].includes(String(to.name))) return
  const href = router.resolve(to).href
  const anchor = Array.from(viewport.querySelectorAll<HTMLAnchorElement>('a[href]'))
    .find(link => link.getAttribute('href') === href)
  window.history.replaceState({ ...window.history.state, oaAssetListPosition: {
    path: route.fullPath, top: viewport.scrollTop, href: anchor?.getAttribute('href'),
  } }, '')
})
function restoreListPosition() {
  if (!oaEnabled || route.name !== 'AssetsIndex') return
  const position = window.history.state?.oaAssetListPosition
  const viewport = resultsViewport.value
  if (!viewport || position?.path !== route.fullPath || !Number.isFinite(position.top)) return
  viewport.scrollTop = position.top
  if (position.href) {
    Array.from(viewport.querySelectorAll<HTMLAnchorElement>('a[href]'))
      .find(link => link.getAttribute('href') === position.href)?.focus({ preventScroll: true })
  }
}
onActivated(() => { void nextTick(restoreListPosition) })

function assetDetailTarget(id: number | string) {
  return {
    name: typeof id === 'number' ? 'AssetDetail' : 'ExternalAssetDetail',
    params: { id },
    query: listPath.value === '/asset-center' ? undefined : { assetFrom: listPath.value },
  }
}
function openLegacyAsset(id: number | string) {
  if (!oaEnabled) void router.push(assetDetailTarget(id))
}
function externalResourceID(asset: BackendAsset) { return String(asset.resource_id || asset.id || '') }
function externalFileName(asset: BackendAsset) { return asset.file_name || asset.original_filename || externalResourceID(asset) || '外部资源' }
function openFilterDrawer() {
  Object.assign(filters, { ...applied, q: filters.q })
  filterDrawerOpen.value = true
}
function clearFilter(key: FilterKey) {
  navigateList({ ...applied, [key]: '' }, 1)
}
function clearAllFilters() {
  navigateList({ q: applied.q, resource_role: '', file_format: '', created_from: '', created_to: '', resource_owner_id: '' }, 1)
}
function resetDrawer() {
  for (const key of Object.keys(filterLabels) as FilterKey[]) filters[key] = ''
}
function searchFromDrawer() {
  filterDrawerOpen.value = false
  navigateList({ ...filters }, 1, showLedger.value, 'refresh')
}
function queryText(query: LocationQuery, key: string) {
  const value = query[key]
  return typeof value === 'string' ? value.trim() : ''
}
function navigateList(next: typeof applied, page: number, ledger = showLedger.value, navigation: 'push' | 'replace' | 'refresh' = 'replace') {
  const query = Object.fromEntries(Object.entries(next).filter(([,value]) => value !== ''))
  if (page > 1) query.page = String(page)
  if (ledger) query.view = 'ledger'
  const target = { name: 'AssetsIndex', query }
  if (router.resolve(target).fullPath === route.fullPath) {
    if (navigation === 'refresh') void load()
    return
  }
  if (oaEnabled && navigation === 'push') {
    void router.push(target)
  } else {
    void router.replace(target)
  }
}
function setLedger(ledger: boolean) {
  navigateList({ ...applied }, requestedPage, ledger, 'push')
}
watch(() => route.name === 'AssetsIndex' ? route.query : null, (query, previous) => {
  if (!query) {
    filterDrawerOpen.value = false
    previewGroupID.value = null
    return
  }
  listPath.value = route.fullPath
  const nextPage = Number(queryText(query, 'page'))
  const page = Number.isSafeInteger(nextPage) && nextPage > 1 ? nextPage : 1
  const resourceRole = queryText(query, 'resource_role')
  const next: typeof applied = {
    q: queryText(query, 'q'),
    resource_role: resourceRoles.find(role => role === resourceRole) ?? '',
    file_format: queryText(query, 'file_format'),
    created_from: queryText(query, 'created_from'),
    created_to: queryText(query, 'created_to'),
    resource_owner_id: queryText(query, 'resource_owner_id'),
  }
  const needsLoad = !hasLoadedQuery || page !== requestedPage || Object.entries(next).some(([key, value]) => applied[key as keyof typeof applied] !== value)
  showLedger.value = query.view === 'ledger'
  if (!needsLoad) {
    if (!previous) Object.assign(filters, applied)
    return
  }
  hasLoadedQuery = true
  requestedPage = page
  Object.assign(applied, next)
  Object.assign(filters, next)
  void nextTick(() => { if (resultsViewport.value) resultsViewport.value.scrollTop = 0 })
  void load(true)
}, { immediate: true })
async function load(restoreScroll = false) {
  const requestVersion = ++liveCostRequestVersion
  liveCosts.value = new Map()
  loading.value = true
  error.value = ''
  try {
    const exactSKUQuery = /^(?=.*\d)[A-Z0-9_-]{5,}$/i.test(applied.q)
    const externalSearch = exactSKUQuery
      ? assetsApi.batchSearchAssets({ terms: [applied.q.toUpperCase()], format_filter: 'all', asset_kind: 'all' }).catch(() => null)
      : Promise.resolve(null)
    const [next, externalResponse] = await Promise.all([
      resourceGroupsApi.list({ q: exactSKUQuery ? undefined : applied.q || undefined, sku_code: exactSKUQuery ? applied.q.toUpperCase() : undefined, resource_role: applied.resource_role || undefined, file_format: applied.file_format || undefined, created_from: applied.created_from || undefined, created_to: applied.created_to || undefined, resource_owner_id: applied.resource_owner_id || undefined, page: requestedPage, page_size: result.page_size }),
      externalSearch,
    ])
    if (requestVersion !== liveCostRequestVersion) return
    result.items = next.items || []; result.flat_items = next.flat_items || []; result.view_mode = next.view_mode || (hasResourceFilters.value ? 'flat' : 'group'); result.page = next.page; result.page_size = next.page_size; result.total = next.total
    const externalBody = externalResponse?.data
    const externalManifest = externalBody && typeof externalBody === 'object' && 'data' in externalBody
      ? externalBody.data as AssetBatchSearchManifest | undefined
      : externalBody as AssetBatchSearchManifest | undefined
    const externalResults = externalManifest?.results
    if (exactSKUQuery && (!externalResults?.length || externalResults.some(row => row.status === 'error'))) {
      error.value = '历史素材查询失败，请重试。'
    }
    const seenExternal = new Set<string>()
    externalItems.value = (externalManifest?.results || [])
      .flatMap((row) => row.assets || [])
      .filter((asset) => asset.source_type === 'external' && externalResourceID(asset).startsWith('ext-'))
      .filter((asset) => {
        const id = externalResourceID(asset)
        if (seenExternal.has(id)) return false
        seenExternal.add(id)
        return true
      })
    brokenImages.value = new Set(); brokenFlat.value = new Set()
    if (exactSKUQuery && result.view_mode === 'group' && result.items.length <= 20) void refreshLiveCosts(result.items, requestVersion)
  } catch (cause) {
    if (requestVersion !== liveCostRequestVersion) return
    externalItems.value = []
    error.value = cause instanceof Error ? cause.message : '资产中心加载失败。'
  } finally {
    if (requestVersion === liveCostRequestVersion) {
      loading.value = false
      if (restoreScroll) void nextTick(restoreListPosition)
    }
  }
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
function goPage(page: number) {
  navigateList({ ...applied }, Math.max(1, Math.min(totalPages.value, page)), showLedger.value, 'push')
}
function search() {
  navigateList({ ...applied, q: filters.q }, 1, showLedger.value, 'refresh')
}
</script>

<style scoped>
.groups-page{max-width:1320px;margin:0 auto;padding:28px;display:grid;gap:20px}.page-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20px}.page-actions{display:flex;gap:10px;flex-wrap:wrap}.page-heading h1{margin:4px 0;font-size:31px}.page-heading p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{margin:0;font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}.quiet-button,.primary-button,.filter-button,.pager button,.error button,.icon-button{min-height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 14px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.primary-button{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.search-workbench{display:grid;gap:10px;padding:14px;border:1px solid rgb(var(--yb-border));border-radius:16px;background:rgb(var(--yb-surface))}.search-row{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:9px}.search-field{position:relative}.search-field input{width:100%;min-height:42px;border:1px solid rgb(var(--yb-border));border-radius:11px;padding:0 16px 0 40px;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text))}.search-icon{position:absolute;left:14px;top:9px;color:rgb(var(--yb-text-muted));font-size:20px}.filter-button.active{border-color:rgb(var(--yb-brand-border));background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.active-filters{display:flex;align-items:center;gap:7px;flex-wrap:wrap}.active-filters button{border:0;border-radius:999px;padding:6px 10px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));cursor:pointer}.active-filters .clear-filters{background:transparent;color:rgb(var(--yb-text-muted))}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:16px}.resource-card{min-width:0;padding:0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:16px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));text-align:left;cursor:pointer;transition:transform .18s,border-color .18s,box-shadow .18s}.resource-card:hover,.resource-card:focus-visible{transform:translateY(-3px);border-color:rgb(var(--yb-brand-border));box-shadow:0 12px 30px rgb(var(--yb-shadow)/.1);outline:2px solid rgb(var(--yb-brand-soft));outline-offset:2px}.cover{position:relative;display:block;aspect-ratio:16/11;background:rgb(var(--yb-surface-muted))}.cover img,.preview-fallback{width:100%;height:100%;object-fit:cover}.preview-fallback{display:grid;place-items:center;align-content:center;gap:6px;color:rgb(var(--yb-text-muted))}.file-mark{font-size:18px;font-weight:900}.preview-fallback small{font-size:11px}.mode-badge,.item-count{position:absolute;top:10px;padding:5px 8px;border-radius:999px;background:rgb(var(--yb-surface)/.94);font-size:11px;font-weight:800}.mode-badge{left:10px;color:rgb(var(--yb-brand))}.item-count{right:10px;color:rgb(var(--yb-text))}.card-body{display:grid;gap:7px;padding:14px}.sku-code{font-size:13px;color:rgb(var(--yb-brand))}.product-name{min-height:22px;font-weight:800;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.card-meta{display:flex;gap:8px;font-size:12px;color:rgb(var(--yb-text-muted))}.card-meta span+span::before{content:'·';margin-right:8px}.provenance{font-size:11px;color:rgb(var(--yb-text-faint))}.warn{font-size:12px;color:rgb(var(--yb-warning-text))}.pager{display:flex;align-items:center;justify-content:center;gap:14px}.empty,.error{padding:38px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:16px;color:rgb(var(--yb-text-muted))}.error{display:flex;align-items:center;justify-content:center;gap:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.drawer-layer{position:fixed;inset:0;z-index:90;display:flex;justify-content:flex-end}.drawer-backdrop{position:absolute;inset:0;border:0;background:rgb(var(--yb-overlay-night)/.42)}.filter-drawer{position:relative;width:min(390px,100vw);height:100%;display:grid;grid-template-rows:auto 1fr;background:rgb(var(--yb-surface));box-shadow:-18px 0 48px rgb(var(--yb-shadow)/.18)}.filter-drawer>header{display:flex;align-items:center;justify-content:space-between;padding:22px;border-bottom:1px solid rgb(var(--yb-border))}.filter-drawer h2{margin:3px 0 0}.icon-button{width:40px;padding:0;font-size:24px}.filter-drawer form{min-height:0;display:grid;grid-template-rows:1fr auto;overflow:auto}.filter-section{display:grid;grid-template-columns:1fr;gap:14px;padding:22px}.filter-section h3{margin:0;font-size:14px}.filter-section label{display:grid;gap:6px;font-size:12px;color:rgb(var(--yb-text-muted))}.filter-section input,.filter-section select{box-sizing:border-box;width:100%;height:42px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:0 11px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.filter-drawer footer{position:sticky;bottom:0;display:flex;justify-content:flex-end;gap:10px;padding:16px 22px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}@media(max-width:700px){.groups-page{padding:16px}.page-heading{align-items:stretch;flex-direction:column}.search-row{grid-template-columns:1fr auto}.search-row .primary-button{display:none}.grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.card-body{padding:11px}.product-name{font-size:13px}}@media(max-width:430px){.grid{grid-template-columns:1fr 1fr}.mode-badge,.item-count{top:7px}.mode-badge{left:7px}.item-count{right:7px}.provenance,.card-meta{font-size:10px}}
.pager {
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.45rem 0.75rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-secondary));
  font-size: 0.8125rem;
}
.pager-controls { display: flex; align-items: center; gap: 0.55rem; }
.pager button { min-height: 2rem; padding-inline: 0.65rem; }
.pager button:disabled { opacity: 0.5; cursor: default; }

@media (min-width: 769px) {
  .search-row button,
  .pager button,
  .filter-drawer footer button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
}
@media (min-width: 901px) {
  .groups-page {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    block-size: calc(100dvh - 6rem);
    min-block-size: 23rem;
    padding-block: 0;
  }
  .page-heading,
  .search-workbench,
  .pager { flex: 0 0 auto; }
  .asset-results-region {
    display: grid;
    align-content: start;
    flex: 1 1 auto;
    padding: 0.2rem;
    gap: 0.65rem;
  }
}
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

.external-results {
  display: grid;
  gap: 0.75rem;
  grid-column: 1 / -1;
}

.external-results > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.78rem;
}

.external-results > header div {
  display: grid;
  gap: 0.15rem;
}

.external-results > header strong {
  color: rgb(var(--yb-text));
  font-size: 0.92rem;
}

.external-grid {
  width: 100%;
}

.external-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: auto 1fr;
}

.external-cover {
  width: 100%;
  overflow: hidden;
}

:deep(.external-cover-media) {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.external-cover :deep(.apm-img) {
  object-fit: contain;
  border-radius: 0;
}

.external-card .card-body {
  align-content: start;
}

.external-card .product-name {
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 1.4;
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
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 4px 8px;
}

.sku-heading > .sku-code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sku-heading > .product-name { grid-column: 1 / -1; width: 100%; }

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
html.oa-live-ui .asset-view-modes { display: flex; gap: 4px; }
html.oa-live-ui #app .page-actions :is(.asset-view-modes button,.asset-refresh) { border-radius: 6px; }
html.oa-live-ui #app .page-actions .asset-refresh { border: 1px solid rgb(var(--yb-border-strong)); }
html.oa-live-ui .sku-facts > span { padding-inline: clamp(4px, .35vw, 9px); }
html.oa-live-ui .sku-facts small { white-space: nowrap; }
html.oa-live-ui .sku-facts b { font-weight: 600; }
html.oa-live-ui .asset-view-modes [aria-pressed="true"] { background: rgb(var(--yb-brand-soft)); color: rgb(var(--yb-brand)); border-color: currentColor; }
html.oa-live-ui .search-field input,
html.oa-live-ui .search-row > button { min-height: 36px; height: 36px; border-radius: 6px; }
html.oa-live-ui .search-icon { font-size: 18px; top: 9px; }
html.oa-live-ui .asset-grid a.resource-card { display: grid; text-decoration: none; }
html.oa-live-ui #app .assets-index-view :is(.mode-badge, .item-count, .sync-pill) { font-weight: 500; }
html.oa-live-ui #app .assets-index-view .asset-ledger th { font-size: 13px; }
html.oa-live-ui .resource-card .cover { aspect-ratio: 16 / 10; pointer-events: none; }
html.oa-live-ui .reference-badge { color: rgb(var(--yb-text-muted)); }
html.oa-live-ui :is(.sku-facts small, .sku-facts b, .preview-fallback small, .mode-badge, .item-count) { font-size: 12px; }
html.oa-live-ui .groups-page { grid-template-columns: minmax(0, 1fr); }
html.oa-live-ui .asset-results-region { min-width: 0; }
.asset-ledger-scroll { min-width: 0; overflow-x: auto; }
.ledger-preview-button, .asset-preview-drawer footer .quiet-button { min-height: 32px; border-radius: 6px; }
.asset-preview-drawer { grid-template-rows: auto minmax(0, 1fr) auto; }
.asset-preview-drawer h2 { font-size: var(--oa-h2-size); font-weight: 600; }
.asset-preview-content { overflow: auto; min-height: 0; padding: 16px 22px; }
.asset-preview-content h3 { margin: 0; font: 600 15px/1.4 var(--yb-font-mono); }
.asset-preview-content p { margin: 4px 0 12px; font-size: 14px; }
.preview-media-frame { aspect-ratio: 16 / 10; overflow: hidden; position: relative; background: rgb(var(--yb-surface-muted)); }
.preview-media-frame :deep(.apm) { width: 100%; height: 100%; }
.preview-media-frame > small { position: absolute; inset: 8px auto auto 8px; padding: 3px 6px; border-radius: 6px; background: rgb(var(--yb-surface-muted)); font-size: 12px; }
.asset-preview-content dl { margin: 12px 0; font-size: 13px; }
.asset-preview-content dl > div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid rgb(var(--yb-border)); }
.asset-preview-content dd { margin: 0; text-align: right; }
.asset-preview-content a { display: inline-flex; min-height: 32px; align-items: center; color: var(--oa-accent); font-size: 14px; }
.asset-preview-drawer footer { position: static; }
.asset-ledger { width: 100%; min-width: 720px; table-layout: fixed; border-collapse: collapse; font-size: 13px; line-height: 1.5; background: rgb(var(--yb-surface)); }
.asset-ledger :is(th, td) { padding: 8px 12px; border-bottom: 1px solid rgb(var(--yb-border)); text-align: left; white-space: nowrap; }
.asset-ledger th { background: rgb(var(--yb-surface-muted)); font-weight: 600; }
.asset-ledger th:first-child { width: 220px; box-sizing: border-box; }
@media (min-width: 1280px) { .asset-ledger th:first-child { width: 404px; } }
.asset-ledger td:first-child span { display: block; overflow: hidden; text-overflow: ellipsis; }
.asset-ledger :is(th,td).ledger-amount { text-align: right; }
.asset-ledger th.ledger-amount { width: 128px; }
.asset-ledger :is(td:nth-child(n+2):nth-child(-n+5), .sku-code) { font-family: var(--yb-font-mono, monospace); font-variant-numeric: tabular-nums; }
</style>
