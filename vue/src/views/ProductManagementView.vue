<template>
  <div class="product-management-view">
    <header class="pm-header">
      <div>
        <p class="pm-eyebrow">ERP 商品资料对照</p>
        <h1>产品管理</h1>
        <p class="pm-subtitle">按 SKU 维护 ERP 图片、成本与同步状态，默认聚焦缺图、成本异常、待同步和失败项。</p>
      </div>
      <button type="button" class="pm-btn pm-btn--ghost" :disabled="loading" @click="loadRecords">
        {{ loading ? '刷新中' : '刷新' }}
      </button>
    </header>

    <section class="pm-filters">
      <label class="pm-field pm-field--wide">
        <span>搜索</span>
        <input
          v-model.trim="filters.keyword"
          type="search"
          placeholder="SKU、任务号、款式编码、商品名、创建人"
          @keyup.enter="applyFilters"
        />
      </label>
      <label class="pm-field">
        <span>显示范围</span>
        <select v-model="filters.display_scope" @change="applyDisplayScope">
          <option value="combo">组合装</option>
          <option value="single">单品 SKU</option>
          <option value="all">全部</option>
        </select>
      </label>
      <label class="pm-field">
        <span>关注范围</span>
        <select v-model="filters.issue_scope" @change="applyFilters">
          <option value="attention">待处理优先</option>
          <option value="all">全部记录</option>
        </select>
      </label>
      <label class="pm-field">
        <span>图片来源</span>
        <select v-model="filters.image_source" @change="applyFilters">
          <option value="">全部</option>
          <option value="manual">人工指定</option>
          <option value="erp_product_image">专项 ERP 商品图</option>
          <option value="auto_on_close">结单自动同步</option>
          <option value="delivery">SKU 成品图</option>
          <option value="derived_preview">派生预览</option>
          <option value="task_reference">任务参考图</option>
          <option value="missing">缺图</option>
        </select>
      </label>
      <label class="pm-field">
        <span>规格 / 面积 / 成本</span>
        <select v-model="filters.cost_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="missing">缺成本</option>
          <option value="ready">已有成本</option>
        </select>
      </label>
      <label class="pm-field">
        <span>基础资料</span>
        <select v-model="filters.base_sync_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="pending_sync">待同步</option>
          <option value="queued">已入队</option>
          <option value="syncing">同步中</option>
          <option value="cooling_down">冷却中</option>
          <option value="failed">失败</option>
          <option value="synced">已同步</option>
        </select>
      </label>
      <label class="pm-field">
        <span>ERP 图片</span>
        <select v-model="filters.image_sync_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="waiting_image">待上传</option>
          <option value="pending_sync">待同步</option>
          <option value="queued">已入队</option>
          <option value="syncing">同步中</option>
          <option value="cooling_down">冷却中</option>
          <option value="failed">失败</option>
          <option value="synced">已同步</option>
        </select>
      </label>
      <button type="button" class="pm-btn pm-btn--primary" @click="applyFilters">查询</button>
      <button type="button" class="pm-btn pm-btn--ghost" :disabled="batchSyncing || syncableRecords.length === 0" @click="syncCurrentPage">
        {{ batchSyncing ? '同步中' : '同步当前页' }}
      </button>
    </section>

    <div class="pm-summary">
      <span>当前共 <b>{{ pagination.total }}</b> 条</span>
      <span>当前页 <b>{{ records.length }}</b> 条</span>
      <span>显示 <b>{{ visibleGroups.length }}</b> 个{{ displayScopeLabel }}</span>
      <span v-if="comboSyncSummaryText" class="pm-combo-sync">{{ comboSyncSummaryText }}</span>
      <span v-if="error" class="pm-error">{{ error }}</span>
    </div>

    <section class="pm-table-shell" :class="{ 'is-loading': loading }">
      <div class="pm-table-head">
        <span>ERP 图片</span>
        <span>SKU / 款式</span>
        <span>商品与任务</span>
        <span>成本</span>
        <span>创建信息</span>
        <span>同步</span>
        <span>操作</span>
      </div>

      <section v-for="group in visibleGroups" :key="group.group_key" class="pm-combo-group" :class="`pm-combo-group--${group.group_type}`">
        <button
          type="button"
          class="pm-combo-header"
          :class="{ 'is-expanded': isComboGroupExpanded(group), 'is-static': group.group_type !== 'combo' }"
          @click="toggleComboGroup(group)"
        >
          <span v-if="group.group_type === 'combo'" class="pm-combo-thumb" :class="{ 'is-missing': !group.pic_url }">
            <img v-if="group.pic_url" :src="group.pic_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
            <span v-else>父级图</span>
          </span>
          <div class="pm-combo-primary">
            <p class="pm-combo-kicker">{{ group.group_type === 'combo' ? '组合装父级' : '单品 SKU' }}</p>
            <strong>
              <span class="pm-combo-code">{{ groupTitle(group) }}</span>
              <span v-if="group.group_type === 'combo' && comboParentName(group)" class="pm-combo-name">{{ comboParentName(group) }}</span>
            </strong>
            <small v-if="group.group_type === 'combo'">{{ groupSubtitle(group) || '聚水潭组合装父级资料暂无更多字段' }}</small>
            <span v-if="group.group_type === 'combo' && group.properties_value" class="pm-combo-properties">{{ group.properties_value }}</span>
          </div>
          <span class="pm-combo-meta">
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>款式</b>
              {{ comboParentStyle(group) }}
            </span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>品牌/分类</b>
              {{ comboParentCategory(group) }}
            </span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>成本/售价</b>
              {{ comboParentPrice(group) }}
            </span>
            <span class="pm-combo-count">{{ group.children.length }} 个系统 SKU</span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-toggle">
              {{ isComboGroupExpanded(group) ? '收起' : '展开' }}
            </span>
          </span>
        </button>

        <template v-if="shouldShowGroupChildren(group)">
          <article v-for="child in group.children" :key="`${group.group_key}:${child.record.id}`" class="pm-row">
            <div class="pm-image-cell">
              <div class="pm-preview" :class="{ 'pm-preview--missing': !previewLoadableForRecord(child.record) }">
                <AssetPreviewMedia
                  v-if="previewLoadableForRecord(child.record)"
                  :asset-id="assetIDForRecord(child.record) || null"
                  :resolved-preview-url="previewURLForRecord(child.record) || null"
                  :fallback-src="directPreviewURL(child.record.image_preview_url) || null"
                  :alt="child.record.sku_code"
                  img-class="pm-preview-apm"
                  inner-img-class="pm-preview-img"
                  defer-until-visible
                />
                <span v-else>{{ child.record.image_missing_reason || 'ERP 图片待补充' }}</span>
              </div>
              <span class="pm-pill" :class="`pm-source--${child.record.image_source}`">{{ child.record.image_source_label }}</span>
            </div>

            <div class="pm-main-cell">
              <strong class="pm-mono">{{ child.record.sku_code || '-' }}</strong>
              <small>款式 {{ productIIDLabel(child.record) }}</small>
              <small v-if="group.group_type === 'combo'">组合数量 {{ formatQuantity(child.quantity) }}</small>
              <small v-if="child.record.category_name">分类 {{ child.record.category_name }}</small>
            </div>

            <div class="pm-info-cell">
              <strong>{{ child.record.product_name || '未命名商品' }}</strong>
              <button type="button" class="pm-link" @click="openTask(child.record.task_id)">
                {{ child.record.task_no || `任务 ${child.record.task_id}` }}
              </button>
            </div>

            <div class="pm-cost-cell" :class="{ 'is-missing': !hasCost(child.record), 'has-area-warning': hasAreaWarning(child.record) }">
              <span v-if="specSummary(child.record)" class="pm-spec-chip" :title="specSummary(child.record)">
                {{ specSummary(child.record) }}
              </span>
              <span class="pm-metric-row">
                <span class="pm-metric-label">面积</span>
                <span class="pm-area-value">{{ areaTraceSummary(child.record) }}</span>
              </span>
              <span class="pm-metric-row pm-cost-row">
                <span class="pm-metric-label">成本</span>
                <span class="pm-cost-value">{{ formatCost(child.record.cost_price) }}</span>
                <span class="pm-detail-help" tabindex="0" :aria-label="productTraceAria(child.record)">
                  明细
                  <span class="pm-cost-popover" role="tooltip">
                    <strong>面积识别</strong>
                    <span v-for="line in areaTraceLines(child.record)" :key="`area-${line}`">{{ line }}</span>
                    <strong>成本计算</strong>
                    <span v-for="line in costTraceLines(child.record)" :key="`cost-${line}`">{{ line }}</span>
                  </span>
                </span>
              </span>
            </div>

            <div class="pm-info-cell">
              <strong>{{ child.record.creator_name || `用户 ${child.record.creator_id}` }}</strong>
              <small>{{ formatDate(child.record.task_created_at) }}</small>
            </div>

            <div class="pm-sync-cell">
              <span class="pm-pill" :class="`pm-sync--${baseSyncStatus(child.record)}`">
                基础 {{ syncStatusLabel(baseSyncStatus(child.record)) }}
              </span>
              <small>{{ child.record.last_base_synced_at ? formatDate(child.record.last_base_synced_at) : '基础资料尚未同步' }}</small>
              <small v-if="child.record.base_sync_error" class="pm-error-text">{{ child.record.base_sync_error }}</small>
              <span class="pm-pill" :class="`pm-sync--${imageSyncStatus(child.record)}`">
                图片 {{ syncStatusLabel(imageSyncStatus(child.record)) }}
              </span>
              <small>{{ child.record.last_image_synced_at ? formatDate(child.record.last_image_synced_at) : 'ERP 图片尚未同步' }}</small>
              <small v-if="child.record.image_sync_error" class="pm-error-text">{{ child.record.image_sync_error }}</small>
              <div v-if="isRecordSyncing(child.record)" class="pm-sync-progress" aria-hidden="true">
                <span></span>
              </div>
              <small v-if="syncMessageForRecord(child.record)" class="pm-sync-message">{{ syncMessageForRecord(child.record) }}</small>
            </div>

            <div class="pm-actions">
              <button type="button" class="pm-btn pm-btn--small" @click="openTask(child.record.task_id)">打开任务</button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_maintain_image" @click="openCandidates(child.record)">
                选图
              </button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_maintain_image" @click="reparseImage(child.record)">
                重新解析
              </button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_sync_erp || isRecordSyncing(child.record)" @click="requestBaseSync(child.record)">
                {{ syncActionLabel(child.record, 'base', '同步基础') }}
              </button>
              <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!child.record.can_sync_erp || isRecordSyncing(child.record)" @click="requestSync(child.record)">
                {{ syncActionLabel(child.record, 'all', '全部同步') }}
              </button>
              <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!child.record.can_sync_erp || !child.record.image_asset_id || isRecordSyncing(child.record)" @click="requestImageSync(child.record)">
                {{ syncActionLabel(child.record, 'image', '同步图片') }}
              </button>
            </div>
          </article>
        </template>
      </section>

      <div v-if="!loading && visibleGroups.length === 0" class="pm-empty">{{ emptyMessage }}</div>
    </section>

    <footer class="pm-pagination">
      <div class="pm-pagination-info">
        <strong>第 {{ pagination.page }} / {{ totalPages }} 页</strong>
        <span>剩余 {{ remainingPages }} 页 · 共 {{ pagination.total }} 条 · 每页 {{ pagination.page_size }} 条</span>
      </div>
      <div class="pm-pagination-actions">
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasPreviousPage || loading" @click="changePage(1)">
          首页
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasPreviousPage || loading" @click="changePage(filters.page - 1)">
          上一页
        </button>
        <button
          v-for="page in visiblePageNumbers"
          :key="page"
          type="button"
          class="pm-page-btn"
          :class="{ 'is-active': page === pagination.page }"
          :disabled="loading || page === pagination.page"
          @click="changePage(page)"
        >
          {{ page }}
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasNextPage || loading" @click="changePage(filters.page + 1)">
          下一页
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasNextPage || loading" @click="changePage(totalPages)">
          末页
        </button>
      </div>
    </footer>

    <div v-if="candidateModalOpen" class="pm-modal-mask" @click.self="closeCandidates">
      <section class="pm-modal">
        <header>
          <div>
            <p class="pm-eyebrow">当前任务图片候选</p>
            <h2>{{ activeRecord?.sku_code }}</h2>
          </div>
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCandidates">关闭</button>
        </header>
        <div v-if="activeRecord?.can_cross_task_select" class="pm-manual-asset">
          <label class="pm-field">
            <span>跨任务资产 ID</span>
            <input v-model.trim="manualAssetID" inputmode="numeric" placeholder="输入资产 ID 后设为 ERP 图" />
          </label>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="!manualAssetID" @click="setManualImage(Number(manualAssetID))">
            使用该资产
          </button>
        </div>
        <div v-if="candidateLoading" class="pm-empty">候选图加载中...</div>
        <div v-else-if="candidates.length === 0" class="pm-empty">当前任务内暂无可用候选图。</div>
        <div v-else class="pm-candidate-grid">
          <button
            v-for="candidate in candidates"
            :key="`${candidate.asset_id}-${candidate.asset_version_id}`"
            type="button"
            class="pm-candidate"
            @click="setManualImage(candidate.asset_id)"
          >
            <AssetPreviewMedia
              v-if="previewLoadableForCandidate(candidate)"
              :asset-id="assetIDForCandidate(candidate)"
              :resolved-preview-url="previewURLForCandidate(candidate) || null"
              :fallback-src="directPreviewURL(candidate.preview_url) || null"
              :alt="candidate.file_name"
              img-class="pm-candidate-apm"
              inner-img-class="pm-candidate-img"
              defer-until-visible
            />
            <span v-else>无预览</span>
            <strong>{{ candidate.file_name }}</strong>
            <small>{{ candidate.source_label }} · {{ candidate.sku_code || '任务通用' }}</small>
          </button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  productManagementApi,
  type ProductManagementCostTrace,
  type ProductImageCandidate,
  type ProductManagementComboGroup,
  type ProductManagementComboSyncSummary,
  type ProductManagementListParams,
  type ProductManagementRecord,
  type ProductSyncStatus,
} from '@/services/api/productManagementApi'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import { mapWithConcurrency } from '@/utils/batchZipDownload'

type ProductSyncScope = 'all' | 'base' | 'image'
type ProductManagementDisplayScope = 'combo' | 'single' | 'all'
type ProductManagementLocalFilters = Required<
  Pick<
    ProductManagementListParams,
    'keyword' | 'issue_scope' | 'image_source' | 'cost_status' | 'sync_status' | 'base_sync_status' | 'image_sync_status' | 'page' | 'page_size'
  >
> & {
  display_scope: ProductManagementDisplayScope
}

const router = useRouter()
const route = useRoute()

const filters = reactive<ProductManagementLocalFilters>({
  keyword: '',
  display_scope: 'combo',
  issue_scope: 'all',
  image_source: '',
  cost_status: '',
  sync_status: '',
  base_sync_status: '',
  image_sync_status: '',
  page: 1,
  page_size: 20,
})

const records = ref<ProductManagementRecord[]>([])
const comboGroups = ref<ProductManagementComboGroup[]>([])
const comboSyncSummary = ref<ProductManagementComboSyncSummary | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const loading = ref(false)
const error = ref('')
const batchSyncing = ref(false)
const candidateModalOpen = ref(false)
const candidateLoading = ref(false)
const candidates = ref<ProductImageCandidate[]>([])
const activeRecord = ref<ProductManagementRecord | null>(null)
const manualAssetID = ref('')
const recordPreviewURLs = ref<Record<number, string>>({})
const candidatePreviewURLs = ref<Record<number, string>>({})
const syncingRecordScopes = ref<Record<number, ProductSyncScope>>({})
const syncMessages = ref<Record<number, string>>({})
const expandedComboGroups = ref<Record<string, boolean>>({})
const syncPollTokens = new Map<number, number>()
const PREVIEW_RESOLVE_CONCURRENCY = 4
let loadRecordsAbort: AbortController | null = null
let loadRecordsSeq = 0
let recordPreviewResolveSeq = 0
let candidatePreviewResolveSeq = 0
const syncableRecords = computed(() => records.value.filter((item) => item.can_sync_erp))
const totalPages = computed(() => Math.max(1, Math.ceil((pagination.total || 0) / Math.max(1, pagination.page_size || filters.page_size))))
const remainingPages = computed(() => Math.max(0, totalPages.value - pagination.page))
const hasPreviousPage = computed(() => pagination.page > 1)
const hasNextPage = computed(() => pagination.page < totalPages.value)
const visiblePageNumbers = computed(() => {
  const total = totalPages.value
  const current = Math.min(Math.max(1, pagination.page), total)
  const size = 5
  const start = Math.max(1, Math.min(current - Math.floor(size / 2), total - size + 1))
  const end = Math.min(total, start + size - 1)
  const pages: number[] = []
  for (let page = start; page <= end; page += 1) {
    pages.push(page)
  }
  return pages
})
const visibleGroups = computed<ProductManagementComboGroup[]>(() => {
  if (filters.display_scope === 'single') {
    return records.value.map(productManagementSingleGroup)
  }
  const groups = comboGroups.value ?? []
  if (filters.display_scope === 'combo') {
    return groups.filter((group) => group.group_type === 'combo')
  }
  return groups
})
const displayScopeLabel = computed(() => {
  if (filters.display_scope === 'combo') return '组合装'
  if (filters.display_scope === 'single') return '单品 SKU'
  return '条目'
})
const emptyMessage = computed(() => {
  if (records.value.length === 0) return '暂无符合条件的产品记录。'
  if (filters.display_scope === 'combo') return '当前页暂无组合装条目，可切换为单品 SKU 或全部查看。'
  if (filters.display_scope === 'single') return '当前页暂无单品 SKU 条目。'
  return '暂无可展示的产品条目。'
})
const comboSyncSummaryText = computed(() => {
  const state = comboSyncSummary.value
  if (!state) return ''
  if (state.status === 'failed') {
    return `组合关系同步延迟：${state.last_error || '等待自动重试'}`
  }
  if (state.last_success_at) {
    return `组合关系最近同步 ${formatDate(state.last_success_at)}`
  }
  return '组合关系正在建立本地缓存'
})

onMounted(() => {
  const keyword = route.query.keyword
  const issueScope = route.query.issue_scope
  if (typeof keyword === 'string' && keyword.trim()) {
    filters.keyword = keyword.trim()
  }
  if (issueScope === 'all' || issueScope === 'attention') {
    filters.issue_scope = issueScope
  }
  void loadRecords()
})

onBeforeUnmount(() => {
  loadRecordsAbort?.abort()
  loadRecordsAbort = null
  recordPreviewResolveSeq += 1
  candidatePreviewResolveSeq += 1
  syncPollTokens.clear()
})

async function loadRecords(): Promise<void> {
  normalizeIssueScopeForExplicitSuccessFilter()
  loadRecordsAbort?.abort()
  const requestSeq = ++loadRecordsSeq
  const abortController = new AbortController()
  loadRecordsAbort = abortController
  loading.value = true
  error.value = ''
  try {
    const result = await productManagementApi.listComboTree({
      keyword: filters.keyword,
      display_scope: filters.display_scope,
      issue_scope: filters.issue_scope,
      image_source: filters.image_source,
      cost_status: filters.cost_status,
      sync_status: filters.sync_status,
      base_sync_status: filters.base_sync_status,
      image_sync_status: filters.image_sync_status,
      page: filters.page,
      page_size: filters.page_size,
    }, abortController.signal)
    if (abortController.signal.aborted || requestSeq !== loadRecordsSeq) return
    records.value = result.data ?? []
    comboGroups.value = result.groups ?? []
    comboSyncSummary.value = result.combo_sync_summary ?? null
    resetExpandedGroups()
    pagination.page = result.pagination?.page ?? filters.page
    pagination.page_size = result.pagination?.page_size ?? filters.page_size
    pagination.total = result.pagination?.total ?? records.value.length
    void resolveRecordPreviewURLs(records.value)
  } catch (err) {
    if (abortController.signal.aborted || requestSeq !== loadRecordsSeq) return
    error.value = errorMessage(err)
  } finally {
    if (loadRecordsAbort === abortController) {
      loadRecordsAbort = null
    }
    if (requestSeq === loadRecordsSeq) {
      loading.value = false
    }
  }
}

function applyFilters(): void {
  filters.page = 1
  void loadRecords()
}

function applyDisplayScope(): void {
  resetExpandedGroups()
  applyFilters()
}

function normalizeIssueScopeForExplicitSuccessFilter(): void {
  if (filters.issue_scope !== 'attention') return
  if (filters.sync_status === 'synced' || filters.base_sync_status === 'synced' || filters.image_sync_status === 'synced') {
    filters.issue_scope = 'all'
  }
}

function changePage(page: number): void {
  filters.page = Math.min(Math.max(1, page), totalPages.value)
  void loadRecords()
}

function productManagementSingleGroup(record: ProductManagementRecord): ProductManagementComboGroup {
  return {
    group_key: `single:${record.id}`,
    group_type: 'single',
    children: [{ record, quantity: 1 }],
  }
}

function resetExpandedGroups(): void {
  expandedComboGroups.value = {}
}

function isComboGroupExpanded(group: ProductManagementComboGroup): boolean {
  if (group.group_type !== 'combo') return true
  return Boolean(expandedComboGroups.value[group.group_key])
}

function shouldShowGroupChildren(group: ProductManagementComboGroup): boolean {
  if (group.group_type !== 'combo') return true
  return isComboGroupExpanded(group)
}

function toggleComboGroup(group: ProductManagementComboGroup): void {
  if (group.group_type !== 'combo') return
  expandedComboGroups.value = {
    ...expandedComboGroups.value,
    [group.group_key]: !expandedComboGroups.value[group.group_key],
  }
}

function openTask(taskId: number): void {
  void router.push({ name: 'TaskDetail', params: { id: String(taskId) } })
}

async function openCandidates(record: ProductManagementRecord): Promise<void> {
  activeRecord.value = record
  manualAssetID.value = ''
  candidateModalOpen.value = true
  candidateLoading.value = true
  candidates.value = []
  error.value = ''
  try {
    candidates.value = await productManagementApi.listImageCandidates(record.id)
    void resolveCandidatePreviewURLs(candidates.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    candidateLoading.value = false
  }
}

function closeCandidates(): void {
  candidateModalOpen.value = false
  candidates.value = []
  candidatePreviewURLs.value = {}
  activeRecord.value = null
  manualAssetID.value = ''
}

async function setManualImage(assetId: number): Promise<void> {
  if (!activeRecord.value) return
  try {
    const updated = await productManagementApi.setManualImage(activeRecord.value.id, assetId)
    replaceRecord(updated)
    closeCandidates()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function reparseImage(record: ProductManagementRecord): Promise<void> {
  try {
    replaceRecord(await productManagementApi.reparseImage(record.id))
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function requestSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'all')
}

async function requestBaseSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'base')
}

async function requestImageSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'image')
}

async function requestRecordSync(record: ProductManagementRecord, scope: ProductSyncScope): Promise<void> {
  const force = Boolean(record.can_force_override)
  markRecordSyncing(record.id, scope, '已提交同步请求，等待 ERP 返回结果。')
  try {
    const next = await requestRecordSyncByScope(record.id, scope, force)
    replaceRecord(next)
    markRecordSyncing(next.id, scope, syncMessageFromRecord(next, scope))
    startRecordSyncPolling(next, scope)
  } catch (err) {
    error.value = errorMessage(err)
    markRecordSyncDone(record.id, `同步请求失败：${errorMessage(err)}`)
  }
}

async function syncCurrentPage(): Promise<void> {
  if (batchSyncing.value) return
  batchSyncing.value = true
  error.value = ''
  try {
    for (const record of syncableRecords.value) {
      await requestRecordSync(record, 'all')
      await delay(350)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    batchSyncing.value = false
  }
}

async function requestRecordSyncByScope(recordId: number, scope: ProductSyncScope, force: boolean): Promise<ProductManagementRecord> {
  if (scope === 'base') return productManagementApi.requestBaseSync(recordId, force)
  if (scope === 'image') return productManagementApi.requestImageSync(recordId, force)
  return productManagementApi.requestSync(recordId, force)
}

function markRecordSyncing(recordId: number, scope: ProductSyncScope, message: string): void {
  syncingRecordScopes.value = { ...syncingRecordScopes.value, [recordId]: scope }
  syncMessages.value = { ...syncMessages.value, [recordId]: message }
}

function markRecordSyncDone(recordId: number, message: string): void {
  const nextScopes = { ...syncingRecordScopes.value }
  delete nextScopes[recordId]
  syncingRecordScopes.value = nextScopes
  syncMessages.value = { ...syncMessages.value, [recordId]: message }
  syncPollTokens.delete(recordId)
}

function startRecordSyncPolling(record: ProductManagementRecord, scope: ProductSyncScope): void {
  const token = Date.now()
  syncPollTokens.set(record.id, token)
  void pollRecordSync(record, scope, token)
}

async function pollRecordSync(record: ProductManagementRecord, scope: ProductSyncScope, token: number): Promise<void> {
  let current = record
  for (let attempt = 0; attempt < 15; attempt += 1) {
    if (syncPollTokens.get(record.id) !== token) return
    const status = scopedSyncStatus(current, scope)
    if (isFinalSyncStatus(status)) {
      markRecordSyncDone(record.id, syncMessageFromRecord(current, scope))
      return
    }
    markRecordSyncing(record.id, scope, syncMessageFromRecord(current, scope))
    await delay(3000)
    if (syncPollTokens.get(record.id) !== token) return
    const latest = await fetchLatestRecord(current).catch(() => null)
    if (latest) {
      current = latest
      replaceRecord(latest)
    }
  }
  markRecordSyncDone(record.id, '已提交到后台处理，结果未及时返回，请稍后刷新查看。')
}

async function fetchLatestRecord(record: ProductManagementRecord): Promise<ProductManagementRecord | null> {
  const result = await productManagementApi.list({
    keyword: record.sku_code || record.task_no,
    issue_scope: 'all',
    page: 1,
    page_size: 50,
  })
  return (result.data ?? []).find((item) => item.id === record.id) ?? null
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function replaceRecord(next: ProductManagementRecord): void {
  const idx = records.value.findIndex((item) => item.id === next.id)
  if (idx >= 0) {
    records.value.splice(idx, 1, next)
  }
  comboGroups.value = comboGroups.value.map((group) => ({
    ...group,
    children: group.children.map((child) => (child.record.id === next.id ? { ...child, record: next } : child)),
  }))
  void resolveRecordPreviewURLs([next])
}

function hasCost(record: ProductManagementRecord): boolean {
  return typeof record.cost_price === 'number' && record.cost_price > 0
}

function specSummary(record: ProductManagementRecord): string {
  return firstTraceString(record.size_text, record.spec_text)
}

function hasAreaWarning(record: ProductManagementRecord): boolean {
  return Boolean(record.area_trace?.warning)
}

function areaTraceSummary(record: ProductManagementRecord): string {
  const area = record.area_trace?.area_m2
  if (typeof area === 'number' && Number.isFinite(area) && area > 0) {
    return `${formatTraceNumber(area)} ㎡`
  }
  return '面积待核对'
}

function areaTraceLines(record: ProductManagementRecord): string[] {
  const trace = record.area_trace
  if (!trace) {
    return ['暂无面积识别信息。', '请检查任务规格尺寸或在任务详情中补充结构化尺寸。']
  }
  const lines: string[] = []
  const spec = specSummary(record)
  if (spec) {
    lines.push(`规格：${spec}`)
  }
  const dimensionParts = [
    typeof trace.width_m === 'number' ? `宽 ${formatTraceNumber(trace.width_m)}m` : '',
    typeof trace.height_m === 'number' ? `高 ${formatTraceNumber(trace.height_m)}m` : '',
    typeof trace.quantity === 'number' ? `数量 ${formatTraceNumber(trace.quantity)}` : '',
  ].filter(Boolean)
  if (dimensionParts.length > 0) {
    lines.push(`尺寸：${dimensionParts.join('，')}`)
  }
  if (trace.formula) {
    lines.push(`公式：${trace.formula}`)
  } else if (typeof trace.area_m2 === 'number') {
    lines.push(`面积：${formatTraceNumber(trace.area_m2)}㎡`)
  }
  if (trace.source_label || trace.source) {
    lines.push(`来源：${trace.source_label || areaTraceSourceLabel(trace.source)}`)
  }
  if (trace.confidence) {
    lines.push(`可信度：${areaTraceConfidenceLabel(trace.confidence)}`)
  }
  if (trace.warning) {
    lines.push(`提示：${trace.warning}`)
  }
  if (lines.length === 0) {
    lines.push('暂无可展示的面积识别明细。')
  }
  return lines
}

function areaTraceSourceLabel(source?: string): string {
  switch (source) {
    case 'sku_item_variant':
      return 'SKU 子项规格'
    case 'task_detail':
      return '任务规格'
    case 'text_extractor':
      return '规格文本解析'
    case 'missing':
      return '未识别到尺寸'
    default:
      return source || '未知来源'
  }
}

function areaTraceConfidenceLabel(confidence: string): string {
  switch (confidence) {
    case 'high':
      return '高'
    case 'medium':
      return '中'
    case 'low':
      return '低'
    default:
      return confidence
  }
}

function productTraceAria(record: ProductManagementRecord): string {
  const sku = record.sku_code || '当前 SKU'
  return `查看 ${sku} 的面积和成本计算明细`
}

function costTraceLines(record: ProductManagementRecord): string[] {
  const trace: ProductManagementCostTrace | null | undefined = record.cost_trace
  if (!trace) {
    return ['暂无成本计算快照，仅显示当前保存的成本。', '如需核验，请重新触发成本计算或查看任务详情。']
  }
  const input = traceObject(trace.input_snapshot)
  const calculation = traceObject(trace.calculation_snapshot)
  const lines: string[] = []
  const ruleName = firstTraceString(trace.rule_name, traceString(calculation, 'cost_rule_name'), traceString(calculation, 'rule_name'), '未匹配到明确规则')
  const version = trace.matched_rule_version || traceNumber(calculation, 'matched_rule_version')
  lines.push(`规则：${ruleName}${version ? ` v${version}` : ''}`)

  const source = firstTraceString(trace.rule_source, trace.prefill_source, traceString(calculation, 'cost_rule_source'), traceString(calculation, 'prefill_source'))
  if (source) {
    lines.push(`来源：${source}`)
  }

  const inputLine = costTraceInputLine(input)
  if (inputLine) {
    lines.push(`输入：${inputLine}`)
  }

  const explanation = firstTraceString(
    traceString(calculation, 'explanation'),
    traceString(calculation, 'formula'),
    traceString(calculation, 'formula_expression'),
    traceString(calculation, 'calculation_expression'),
  )
  if (explanation) {
    lines.push(`公式：${explanation}`)
  }

  const estimated = traceNumber(calculation, 'estimated_cost')
  const finalCost = firstTraceNumber(record.cost_price, traceNumber(calculation, 'cost_price'))
  const costParts = [
    estimated !== undefined ? `估算 ${formatCost(estimated)}` : '',
    finalCost !== undefined ? `最终 ${formatCost(finalCost)}` : '',
  ].filter(Boolean)
  if (costParts.length > 0) {
    lines.push(`结果：${costParts.join(' / ')}`)
  }

  const manualOverride = trace.manual_cost_override || traceBoolean(calculation, 'manual_cost_override')
  if (manualOverride) {
    const reason = firstTraceString(trace.manual_cost_override_reason, traceString(calculation, 'manual_cost_override_reason'), '未填写原因')
    lines.push(`人工覆盖：是，${reason}`)
  } else if (trace.requires_manual_review || traceBoolean(calculation, 'requires_manual_review')) {
    lines.push('状态：需要人工复核')
  } else {
    lines.push('状态：系统规则自动生成')
  }

  if (trace.snapshot_at) {
    lines.push(`快照：${formatDate(trace.snapshot_at)}`)
  }
  return lines
}

function costTraceInputLine(input: Record<string, unknown>): string {
  const sizeText = firstTraceString(traceString(input, 'spec_text'), traceString(input, 'size_text'))
  const width = traceNumber(input, 'width')
  const height = traceNumber(input, 'height')
  const area = traceNumber(input, 'area')
  const quantity = traceNumber(input, 'quantity')
  const category = firstTraceString(traceString(input, 'category_name'), traceString(input, 'category_code'), traceString(input, 'product_i_id'))
  const parts = [
    category ? `品类 ${category}` : '',
    sizeText ? `尺寸 ${sizeText}` : width !== undefined && height !== undefined ? `尺寸 ${formatTraceNumber(width)}x${formatTraceNumber(height)}` : '',
    area !== undefined ? `面积 ${formatTraceNumber(area)}㎡` : '',
    quantity !== undefined ? `数量 ${formatTraceNumber(quantity)}` : '',
  ].filter(Boolean)
  return parts.join('，')
}

function traceObject(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function traceString(source: Record<string, unknown>, key: string): string {
  const value = source[key]
  return typeof value === 'string' ? value.trim() : ''
}

function traceNumber(source: Record<string, unknown>, key: string): number | undefined {
  return numberFromUnknown(source[key])
}

function traceBoolean(source: Record<string, unknown>, key: string): boolean {
  const value = source[key]
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  if (typeof value === 'string') return ['1', 'true', 'yes', '是'].includes(value.trim().toLowerCase())
  return false
}

function firstTraceString(...values: Array<string | undefined | null>): string {
  for (const value of values) {
    const trimmed = typeof value === 'string' ? value.trim() : ''
    if (trimmed) return trimmed
  }
  return ''
}

function firstTraceNumber(...values: Array<number | undefined | null>): number | undefined {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return undefined
}

function numberFromUnknown(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function formatTraceNumber(value: number): string {
  return value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

function productIIDLabel(record: ProductManagementRecord): string {
  return record.erp_i_id?.trim() || record.product_i_id?.trim() || '未绑定 ERP 款式'
}

function baseSyncStatus(record: ProductManagementRecord): ProductSyncStatus {
  return record.base_sync_status || record.erp_sync_status || 'pending_sync'
}

function imageSyncStatus(record: ProductManagementRecord): ProductSyncStatus {
  return record.image_sync_status || record.erp_sync_status || 'waiting_image'
}

function scopedSyncStatus(record: ProductManagementRecord, scope: ProductSyncScope): ProductSyncStatus {
  if (scope === 'base') return baseSyncStatus(record)
  if (scope === 'image') {
    if (isActiveSyncStatus(record.erp_sync_status)) return record.erp_sync_status
    return imageSyncStatus(record)
  }
  if (isActiveSyncStatus(record.erp_sync_status)) return record.erp_sync_status
  const baseStatus = baseSyncStatus(record)
  const imgStatus = imageSyncStatus(record)
  if (isActiveSyncStatus(baseStatus)) return baseStatus
  if (isActiveSyncStatus(imgStatus)) return imgStatus
  if (baseStatus === 'failed' || imgStatus === 'failed') return 'failed'
  if (baseStatus === 'waiting_image' || imgStatus === 'waiting_image') return 'waiting_image'
  if (baseStatus === 'synced' && (!record.image_required || imgStatus === 'synced')) return 'synced'
  return 'pending_sync'
}

function isActiveSyncStatus(status?: ProductSyncStatus): boolean {
  return status === 'queued' || status === 'syncing' || status === 'cooling_down'
}

function isFinalSyncStatus(status: ProductSyncStatus): boolean {
  return status === 'synced' || status === 'failed' || status === 'waiting_image'
}

function isRecordSyncing(record: ProductManagementRecord): boolean {
  return Boolean(syncingRecordScopes.value[record.id]) || isActiveSyncStatus(scopedSyncStatus(record, 'all'))
}

function syncMessageForRecord(record: ProductManagementRecord): string {
  const existing = syncMessages.value[record.id]
  if (existing) return existing
  const status = scopedSyncStatus(record, 'all')
  if (isActiveSyncStatus(status)) return syncMessageFromStatus(status)
  return ''
}

function syncActionLabel(record: ProductManagementRecord, scope: ProductSyncScope, fallback: string): string {
  if (syncingRecordScopes.value[record.id] === scope) return '同步中'
  return fallback
}

function syncMessageFromRecord(record: ProductManagementRecord, scope: ProductSyncScope): string {
  const status = scopedSyncStatus(record, scope)
  if (status === 'synced') {
    if (scope === 'base') return 'ERP 基础资料已同步成功。'
    if (scope === 'image') return 'ERP 图片已同步成功。'
    return 'ERP 基础资料和图片已同步成功。'
  }
  if (status === 'failed') {
    return firstNonEmptyString(record.image_sync_error, record.base_sync_error, record.last_sync_error, '同步失败，请查看错误信息后重试。')
  }
  if (status === 'waiting_image') return '缺少可同步的 ERP 商品图，请先上传或选择图片。'
  return syncMessageFromStatus(status)
}

function syncMessageFromStatus(status: ProductSyncStatus): string {
  if (status === 'queued') return '已进入同步队列，等待后台处理。'
  if (status === 'syncing') return '正在同步 ERP，请稍候。'
  if (status === 'cooling_down') return '同步请求处于冷却队列，系统会自动继续处理。'
  return '等待同步。'
}

function previewURLForRecord(record: ProductManagementRecord): string {
  return recordPreviewURLs.value[record.id] || directPreviewURL(record.image_preview_url) || ''
}

function previewURLForCandidate(candidate: ProductImageCandidate): string {
  return candidatePreviewURLs.value[candidate.asset_id] || directPreviewURL(candidate.preview_url) || ''
}

function assetIDForRecord(record: ProductManagementRecord): string | undefined {
  const raw = record.image_asset_id ?? assetIDFromPreviewPath(record.image_preview_url)
  const id = Number(raw)
  return Number.isSafeInteger(id) && id > 0 ? String(id) : undefined
}

function assetIDForCandidate(candidate: ProductImageCandidate): string {
  return String(candidate.asset_id)
}

function previewLoadableForRecord(record: ProductManagementRecord): boolean {
  return Boolean(assetIDForRecord(record) || previewURLForRecord(record))
}

function previewLoadableForCandidate(candidate: ProductImageCandidate): boolean {
  return Boolean(assetIDForCandidate(candidate) || previewURLForCandidate(candidate))
}

async function resolveRecordPreviewURLs(items: ProductManagementRecord[]): Promise<void> {
  const seq = ++recordPreviewResolveSeq
  const next = { ...recordPreviewURLs.value }
  await mapWithConcurrency(items, PREVIEW_RESOLVE_CONCURRENCY, async (item) => {
    if (seq !== recordPreviewResolveSeq) return
      const assetID = item.image_asset_id ?? assetIDFromPreviewPath(item.image_preview_url)
      const url = await resolveAssetPreviewURL(assetID, item.image_preview_url)
    if (seq !== recordPreviewResolveSeq) return
      if (url) next[item.id] = url
      else delete next[item.id]
  })
  if (seq !== recordPreviewResolveSeq) return
  recordPreviewURLs.value = next
}

async function resolveCandidatePreviewURLs(items: ProductImageCandidate[]): Promise<void> {
  const seq = ++candidatePreviewResolveSeq
  const next = { ...candidatePreviewURLs.value }
  await mapWithConcurrency(items, PREVIEW_RESOLVE_CONCURRENCY, async (item) => {
    if (seq !== candidatePreviewResolveSeq) return
      const url = await resolveAssetPreviewURL(item.asset_id, item.preview_url)
    if (seq !== candidatePreviewResolveSeq) return
      if (url) next[item.asset_id] = url
      else delete next[item.asset_id]
  })
  if (seq !== candidatePreviewResolveSeq) return
  candidatePreviewURLs.value = next
}

async function resolveAssetPreviewURL(assetID?: number | null, fallback?: string): Promise<string> {
  const fallbackAssetID = assetID ?? assetIDFromPreviewPath(fallback)
  const direct = directPreviewURL(fallback)
  if (direct) return direct
  if (!fallbackAssetID || fallbackAssetID <= 0) return ''
  const result = await fetchAssetPreviewMeta(String(fallbackAssetID)).catch(() => null)
  return result?.status === 'ok' && result.displayUrl ? result.displayUrl : ''
}

function directPreviewURL(raw?: string): string {
  const value = String(raw ?? '').trim()
  if (!value) return ''
  if (isAssetPreviewMetaURL(value)) return ''
  if (/^(https?:|data:|blob:)/i.test(value)) return value
  return ''
}

function isAssetPreviewMetaURL(raw: string): boolean {
  const value = raw.trim()
  if (!value) return false
  try {
    const url = new URL(value, window.location.origin)
    return /^\/v1\/assets\/\d+\/preview\b/i.test(url.pathname)
  } catch {
    return /^\/v1\/assets\/\d+\/preview\b/i.test(value)
  }
}

function assetIDFromPreviewPath(raw?: string): number | undefined {
  const value = String(raw ?? '').trim()
  let path = value
  try {
    path = new URL(value, window.location.origin).pathname
  } catch {
    path = value
  }
  const match = path.match(/\/v1\/assets\/(\d+)\/preview\b/)
  if (!match) return undefined
  const id = Number(match[1])
  return Number.isSafeInteger(id) && id > 0 ? id : undefined
}

function formatCost(value?: number | null): string {
  if (typeof value !== 'number' || value <= 0) return '待维护'
  return `￥${value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function formatQuantity(value?: number): string {
  const qty = typeof value === 'number' && value > 0 ? value : 1
  return qty.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

function groupTitle(group: ProductManagementComboGroup): string {
  if (group.group_type !== 'combo') {
    return group.children[0]?.record.sku_code || '单品 SKU'
  }
  return firstNonEmptyString(group.combo_sku_code, group.combo_name, '未命名组合装')
}

function groupSubtitle(group: ProductManagementComboGroup): string {
  const entityID = firstNonEmptyString(group.entity_sku_id)
  const synced = group.last_synced_at ? `同步 ${formatDate(group.last_synced_at)}` : ''
  return [
    entityID ? `实体 ${entityID}` : '',
    synced,
  ]
    .filter(Boolean)
    .join(' · ')
}

function comboParentName(group: ProductManagementComboGroup): string {
  return firstNonEmptyString(group.combo_name, group.combo_short_name)
}

function comboParentStyle(group: ProductManagementComboGroup): string {
  return firstNonEmptyString(group.erp_i_id, group.entity_sku_id, '未绑定 ERP 款式')
}

function comboParentCategory(group: ProductManagementComboGroup): string {
  const brand = firstNonEmptyString(group.brand)
  const vcName = firstNonEmptyString(group.vc_name)
  if (brand && vcName) return `${brand} / ${vcName}`
  return firstNonEmptyString(brand, vcName, '未返回品牌分类')
}

function comboParentPrice(group: ProductManagementComboGroup): string {
  const cost = formatNullablePrice(group.cost_price)
  const sale = formatNullablePrice(group.sale_price)
  if (cost && sale) return `${cost} / ${sale}`
  return firstNonEmptyString(cost, sale, '未返回价格')
}

function formatNullablePrice(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return ''
  return `￥${value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

function syncStatusLabel(status: ProductSyncStatus): string {
  const labels: Record<ProductSyncStatus, string> = {
    pending_sync: '待同步',
    queued: '已入队',
    syncing: '同步中',
    synced: '已同步',
    failed: '同步失败',
    cooling_down: '冷却中',
    waiting_image: '待上传 ERP 图',
  }
  return labels[status] ?? status
}

function firstNonEmptyString(...values: Array<string | undefined | null>): string {
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (text) return text
  }
  return ''
}

function errorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return '操作失败，请稍后重试。'
}
</script>

<style scoped>
.product-management-view {
  min-height: 100%;
  padding: 1.25rem clamp(0.875rem, 2vw, 1.75rem) 2.5rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-bg-page));
}

.pm-header,
.pm-filters,
.pm-table-shell,
.pm-modal {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.06);
}

.pm-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 1.1rem 1.25rem;
  border-radius: 1rem;
}

.pm-eyebrow {
  margin: 0 0 8px;
  color: rgb(var(--yb-brand));
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
}

.pm-header h1,
.pm-modal h2 {
  margin: 0;
  color: rgb(var(--yb-text));
  font-size: clamp(1.65rem, 2.5vw, 2.25rem);
  font-weight: 900;
  letter-spacing: 0;
}

.pm-subtitle {
  max-width: 760px;
  margin: 0.55rem 0 0;
  color: rgb(var(--yb-text-secondary));
  line-height: 1.65;
}

.pm-filters {
  display: grid;
  grid-template-columns: minmax(18rem, 2fr) repeat(6, minmax(8rem, 1fr)) auto auto;
  gap: 0.75rem;
  margin-top: 0.85rem;
  padding: 0.9rem;
  border-radius: 0.875rem;
}

.pm-filters > .pm-btn {
  align-self: end;
}

.pm-field {
  display: grid;
  gap: 0.35rem;
  color: rgb(var(--yb-text-muted));
  font-size: 12px;
  font-weight: 800;
}

.pm-field input,
.pm-field select {
  width: 100%;
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 12px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  outline: none;
}

.pm-field input:focus,
.pm-field select:focus {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.pm-btn {
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 16px;
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  font-weight: 800;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.pm-btn:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-btn:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.pm-btn--primary {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
}

.pm-btn--primary:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-strong));
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand-strong));
}

.pm-btn--ghost {
  background: rgb(var(--yb-surface-soft));
}

.pm-btn--small {
  min-height: 1.9rem;
  padding: 0 10px;
  font-size: 12px;
}

.pm-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 0.8rem 0;
  color: rgb(var(--yb-text-secondary));
}

.pm-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0.9rem 0 0;
  padding: 0.85rem 1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.875rem;
  color: rgb(var(--yb-text-secondary));
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.05);
}

.pm-pagination-info {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.pm-pagination-info strong {
  color: rgb(var(--yb-text));
  font-weight: 900;
}

.pm-pagination-info span {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 13px;
}

.pm-pagination-actions {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.pm-page-btn {
  min-width: 2.25rem;
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 10px;
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  font-weight: 900;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.pm-page-btn:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-page-btn:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.pm-page-btn.is-active {
  border-color: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand));
  opacity: 1;
}

.pm-page-btn--wide {
  min-width: 4rem;
}

.pm-error,
.pm-error-text {
  color: rgb(var(--yb-danger));
}

.pm-combo-sync {
  color: rgb(var(--yb-brand));
  font-weight: 800;
}

.pm-table-shell {
  overflow: visible;
  border-radius: 1rem;
}

.pm-combo-group {
  border-top: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.pm-combo-group--combo {
  background: linear-gradient(90deg, rgb(var(--yb-brand) / 0.055), rgb(var(--yb-surface)) 42%);
}

.pm-combo-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  border: 0;
  border-bottom: 1px solid rgb(var(--yb-border-brand-quiet));
  padding: 1rem;
  background: linear-gradient(90deg, rgb(var(--yb-surface-blue-subtle)) 0%, rgb(var(--yb-surface-subtle)) 42%, rgb(var(--yb-surface)) 100%);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease;
}

.pm-combo-header:hover:not(.is-static) {
  background: rgb(var(--yb-surface-blue-subtle));
}

.pm-combo-header:focus-visible {
  outline: 3px solid rgb(var(--yb-brand) / 0.18);
  outline-offset: -3px;
}

.pm-combo-header.is-static {
  cursor: default;
}

.pm-combo-primary {
  display: grid;
  flex: 1 1 auto;
  gap: 6px;
  min-width: 0;
}

.pm-combo-thumb {
  display: grid;
  flex: 0 0 92px;
  place-items: center;
  width: 92px;
  height: 66px;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 18px rgb(var(--yb-brand) / 0.08);
}

.pm-combo-thumb img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.pm-combo-thumb.is-missing {
  border-style: dashed;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-subtle));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-header strong {
  display: flex;
  align-items: baseline;
  gap: 10px;
  overflow: hidden;
  color: rgb(var(--yb-text-navy));
  font-size: 15px;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-code {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-navy));
  font-family: var(--yb-font-data);
  font-size: 16px;
  font-weight: 950;
}

.pm-combo-name {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text-deep));
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-header small {
  overflow: hidden;
  color: rgb(var(--yb-text-muted-strong));
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-properties {
  display: block;
  overflow: hidden;
  max-width: 68rem;
  color: rgb(var(--yb-text-soft));
  font-size: 12px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-kicker {
  margin: 0;
  color: rgb(var(--yb-brand));
  font-size: 11px;
  font-weight: 900;
}

.pm-combo-field {
  display: grid;
  gap: 2px;
  min-width: 7rem;
  max-width: 13rem;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 0.75rem;
  padding: 7px 10px;
  color: rgb(var(--yb-text-navy));
  background: rgb(var(--yb-surface) / 0.86);
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-field b {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 11px;
  font-weight: 900;
}

.pm-combo-count {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 999px;
  padding: 5px 10px;
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-meta {
  display: flex;
  flex: 0 0 min(48%, 680px);
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.pm-combo-toggle {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 999px;
  padding: 5px 10px;
  color: rgb(var(--yb-text-soft));
  background: rgb(var(--yb-surface));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-header.is-expanded .pm-combo-toggle {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-subtle));
}

.pm-table-head,
.pm-row {
  display: grid;
  grid-template-columns: 9.5rem minmax(8.5rem, 0.9fr) minmax(18rem, 1.6fr) minmax(8.25rem, 0.75fr) minmax(8rem, 0.8fr) minmax(8.5rem, 0.8fr) minmax(13rem, 0.9fr);
  gap: 0.9rem;
  align-items: center;
}

.pm-table-head {
  padding: 0.75rem 1rem;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-subtle));
  font-size: 12px;
  font-weight: 900;
}

.pm-row {
  position: relative;
  padding: 0.9rem 1rem;
  border-top: 1px solid rgb(var(--yb-border-page-soft));
  background: rgb(var(--yb-surface));
}

.pm-row:hover {
  z-index: 3;
  background: rgb(var(--yb-surface-brand-panel));
}

.pm-image-cell,
.pm-main-cell,
.pm-info-cell,
.pm-sync-cell,
.pm-actions {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.pm-preview {
  display: grid;
  place-items: center;
  width: 8rem;
  height: 5.25rem;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border-muted-blue));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-danger));
  font-size: 12px;
  font-weight: 800;
  text-align: center;
}

.pm-preview :deep(.pm-preview-apm),
.pm-preview :deep(.apm),
.pm-preview :deep(.apm-img),
.pm-preview :deep(.pm-preview-img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: rgb(var(--yb-surface));
}

.pm-preview :deep(.apm-placeholder),
.pm-preview :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.25rem;
}

.pm-preview--missing {
  border-style: dashed;
}

.pm-mono,
.pm-cost-cell {
  font-family: var(--yb-font-data);
}

.pm-main-cell strong,
.pm-info-cell strong {
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-main-cell small,
.pm-info-cell small,
.pm-sync-cell small {
  color: rgb(var(--yb-text-muted));
}

.pm-cost-cell {
  position: relative;
  display: grid;
  width: min(15rem, 100%);
  max-width: 100%;
  gap: 0.4rem;
  color: rgb(var(--yb-success-teal));
  font-weight: 900;
}

.pm-spec-chip {
  display: inline-flex;
  max-width: 100%;
  width: max-content;
  align-items: center;
  overflow: hidden;
  padding: 0.24rem 0.55rem;
  border: 1px solid rgba(var(--yb-brand-cyan-comma), 0.24);
  border-radius: 999px;
  color: rgb(var(--yb-brand-cyan));
  background: rgba(var(--yb-brand-cyan-comma), 0.08);
  font-family: var(--yb-font-text);
  font-size: 0.72rem;
  font-weight: 950;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-metric-row {
  position: relative;
  display: grid;
  width: 100%;
  grid-template-columns: 2.35rem minmax(0, 1fr);
  align-items: center;
  column-gap: 0.4rem;
  min-width: 0;
}

.pm-cost-row {
  grid-template-columns: 2.35rem minmax(0, 1fr) auto;
}

.pm-metric-label {
  color: rgb(var(--yb-text-muted));
  font-family: var(--yb-font-text);
  font-size: 0.72rem;
  font-weight: 850;
  line-height: 1;
  white-space: nowrap;
}

.pm-area-value {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-size: 0.88rem;
  font-weight: 900;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-cost-value {
  min-width: 0;
  overflow: hidden;
  font-size: 1.16rem;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-detail-help {
  position: relative;
  display: inline-flex;
  width: auto;
  min-width: 2.45rem;
  height: 1.35rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.48rem;
  border: 1px solid rgb(var(--yb-success-border-bright));
  border-radius: 999px;
  color: rgb(var(--yb-success-teal));
  background: linear-gradient(135deg, rgb(var(--yb-success-soft)), rgb(var(--yb-success-soft-strong)));
  box-shadow: 0 7px 18px rgba(var(--yb-success-teal-comma), 0.14);
  cursor: help;
  font-family: var(--yb-font-text);
  font-size: 0.72rem;
  font-weight: 950;
  line-height: 1;
  outline: none;
  white-space: nowrap;
}

.pm-detail-help:hover,
.pm-detail-help:focus-visible {
  border-color: rgb(var(--yb-success-emerald-light));
  color: rgb(var(--yb-success-text-dark));
  background: rgb(var(--yb-surface-success-strong));
}

.pm-cost-popover {
  pointer-events: none;
  position: absolute;
  top: 50%;
  left: calc(100% + 0.65rem);
  z-index: 40;
  display: grid;
  width: min(26rem, 52vw);
  min-width: 21rem;
  max-height: 20rem;
  gap: 0.45rem;
  overflow: auto;
  padding: 0.9rem 1rem;
  border: 1px solid rgba(var(--yb-shadow-comma), 0.12);
  border-radius: 0.95rem;
  color: rgb(var(--yb-brand-subtle));
  background: rgba(var(--yb-shadow-comma), 0.96);
  box-shadow: 0 18px 42px rgba(var(--yb-shadow-comma), 0.32);
  opacity: 0;
  transform: translateY(-50%) translateX(-0.25rem) scale(0.98);
  transition: opacity 0.14s ease, transform 0.14s ease;
}

.pm-cost-popover strong {
  color: rgb(var(--yb-surface));
  font-family: var(--yb-font-text);
  font-size: 0.92rem;
}

.pm-cost-popover span {
  color: rgb(var(--yb-text-disabled));
  font-family: var(--yb-font-text);
  font-size: 0.79rem;
  font-weight: 750;
  line-height: 1.45;
}

.pm-detail-help:hover .pm-cost-popover,
.pm-detail-help:focus-visible .pm-cost-popover,
.pm-detail-help:focus-within .pm-cost-popover {
  opacity: 1;
  transform: translateY(-50%) translateX(0) scale(1);
}

.pm-cost-cell.is-missing {
  color: rgb(var(--yb-danger));
}

.pm-cost-cell.is-missing .pm-detail-help {
  border-color: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-wash));
  box-shadow: 0 7px 18px rgba(var(--yb-danger-comma), 0.12);
}

.pm-cost-cell.has-area-warning .pm-area-value {
  color: rgb(var(--yb-warning));
}

.pm-link {
  width: fit-content;
  border: 0;
  padding: 0;
  color: rgb(var(--yb-brand));
  background: transparent;
  font-weight: 800;
  cursor: pointer;
  text-align: left;
}

.pm-pill {
  width: fit-content;
  max-width: 100%;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 999px;
  padding: 4px 9px;
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
  font-size: 12px;
  font-weight: 900;
  white-space: nowrap;
}

.pm-source--manual {
  border-color: rgb(var(--yb-warning-yellow));
  color: rgb(var(--yb-warning-strong));
  background: rgb(var(--yb-warning-highlight));
}

.pm-source--delivery {
  border-color: rgb(var(--yb-success-border-bright));
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-soft));
}

.pm-source--derived_preview {
  border-color: rgb(var(--yb-info-cyan-border-strong));
  color: rgb(var(--yb-info-deep));
  background: rgb(var(--yb-info-soft));
}

.pm-source--task_reference {
  border-color: rgb(var(--yb-purple-border-strong));
  color: rgb(var(--yb-purple-text-deep));
  background: rgb(var(--yb-purple-soft-strong));
}

.pm-source--erp_product_image {
  border-color: rgb(var(--yb-teal-border-strong));
  color: rgb(var(--yb-teal));
  background: rgb(var(--yb-teal-mint));
}

.pm-source--auto_on_close {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-subtle));
}

.pm-source--missing,
.pm-sync--failed {
  border-color: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
}

.pm-sync--synced {
  border-color: rgb(var(--yb-success-border-bright));
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-soft));
}

.pm-sync--queued,
.pm-sync--syncing,
.pm-sync--pending_sync {
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-sync--cooling_down {
  border-color: rgb(var(--yb-warning-border-soft));
  color: rgb(var(--yb-warning-dark));
  background: rgb(var(--yb-warning-soft));
}

.pm-sync--waiting_image {
  border-color: rgb(var(--yb-warning-border-warm));
  color: rgb(var(--yb-warning-orange-text));
  background: rgb(var(--yb-warning-orange-soft));
}

.pm-sync-progress {
  position: relative;
  width: min(13rem, 100%);
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(var(--yb-surface-blue-muted));
}

.pm-sync-progress span {
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgb(var(--yb-brand)), rgb(var(--yb-info-sky)));
  animation: pm-sync-flow 1.1s ease-in-out infinite;
}

.pm-sync-message {
  color: rgb(var(--yb-brand-strong));
  font-weight: 800;
}

@keyframes pm-sync-flow {
  0% {
    transform: translateX(-110%);
  }
  100% {
    transform: translateX(230%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pm-sync-progress span {
    animation: none;
    transform: none;
    width: 100%;
  }
}

.pm-actions {
  grid-template-columns: repeat(2, minmax(5.5rem, 1fr));
}

.pm-empty {
  padding: 36px;
  color: rgb(var(--yb-text-muted));
  text-align: center;
}

.pm-modal-mask {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--yb-shadow) / 0.42);
}

.pm-modal {
  width: min(980px, 100%);
  max-height: min(760px, 88dvh);
  overflow: auto;
  border-radius: 1rem;
  padding: 22px;
}

.pm-modal header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.pm-manual-asset {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto;
  gap: 12px;
  align-items: end;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.875rem;
  background: rgb(var(--yb-brand-soft));
}

.pm-candidate-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 14px;
}

.pm-candidate {
  display: grid;
  gap: 8px;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.875rem;
  padding: 12px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  text-align: left;
  cursor: pointer;
}

.pm-candidate:hover {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-surface-brand-panel));
}

.pm-candidate :deep(.pm-candidate-apm),
.pm-candidate :deep(.apm) {
  width: 100%;
  aspect-ratio: 4 / 3;
}

.pm-candidate :deep(.apm-img),
.pm-candidate :deep(.pm-candidate-img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.pm-candidate :deep(.pm-candidate-apm),
.pm-candidate :deep(.apm-placeholder),
.pm-candidate :deep(.apm-empty) {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
}

.pm-candidate :deep(.apm-placeholder),
.pm-candidate :deep(.apm-empty) {
  min-height: 0;
  height: auto;
  aspect-ratio: 4 / 3;
  padding: 0.25rem;
}

.pm-candidate strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-candidate small {
  color: rgb(var(--yb-text-muted));
}

@media (max-width: 1320px) {
  .pm-filters {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .pm-field--wide {
    grid-column: span 2;
  }

  .pm-table-head {
    display: none;
  }

  .pm-combo-header {
    align-items: flex-start;
  }

  .pm-pagination {
    align-items: flex-start;
    flex-direction: column;
  }

  .pm-pagination-actions {
    justify-content: flex-start;
  }

  .pm-combo-meta {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .pm-row {
    grid-template-columns: 170px repeat(2, minmax(0, 1fr));
    align-items: start;
  }

  .pm-actions {
    grid-column: 2 / -1;
    grid-template-columns: repeat(4, minmax(86px, max-content));
  }
}

@media (max-width: 760px) {
  .product-management-view {
    padding-inline: 12px;
  }

  .pm-header,
  .pm-modal header {
    display: grid;
  }

  .pm-filters,
  .pm-row,
  .pm-manual-asset {
    grid-template-columns: 1fr;
  }

  .pm-field--wide,
  .pm-actions {
    grid-column: auto;
  }

  .pm-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .pm-cost-popover {
    top: calc(100% + 0.65rem);
    left: 0;
    width: min(22rem, calc(100vw - 2rem));
    min-width: min(22rem, calc(100vw - 2rem));
    transform: translateY(0) translateX(-0.25rem) scale(0.98);
  }

  .pm-detail-help:hover .pm-cost-popover,
  .pm-detail-help:focus-visible .pm-cost-popover,
  .pm-detail-help:focus-within .pm-cost-popover {
    transform: translateY(0) translateX(0) scale(1);
  }

  .pm-combo-header {
    flex-wrap: wrap;
  }

  .pm-combo-thumb {
    flex-basis: 64px;
    width: 64px;
    height: 46px;
  }

  .pm-preview {
    width: 100%;
    height: 160px;
  }
}
</style>
