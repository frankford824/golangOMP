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
        <span>成本</span>
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

      <article v-for="record in records" :key="record.id" class="pm-row">
        <div class="pm-image-cell">
          <div class="pm-preview" :class="{ 'pm-preview--missing': !previewURLForRecord(record) }">
            <img v-if="previewURLForRecord(record)" :src="previewURLForRecord(record)" :alt="record.sku_code" loading="lazy" />
            <span v-else>{{ record.image_missing_reason || 'ERP 图片待补充' }}</span>
          </div>
          <span class="pm-pill" :class="`pm-source--${record.image_source}`">{{ record.image_source_label }}</span>
        </div>

        <div class="pm-main-cell">
          <strong class="pm-mono">{{ record.sku_code || '-' }}</strong>
          <small>款式 {{ productIIDLabel(record) }}</small>
          <small v-if="record.category_name">分类 {{ record.category_name }}</small>
        </div>

        <div class="pm-info-cell">
          <strong>{{ record.product_name || '未命名商品' }}</strong>
          <button type="button" class="pm-link" @click="openTask(record.task_id)">
            {{ record.task_no || `任务 ${record.task_id}` }}
          </button>
        </div>

        <div class="pm-cost-cell" :class="{ 'is-missing': !hasCost(record) }">
          {{ formatCost(record.cost_price) }}
        </div>

        <div class="pm-info-cell">
          <strong>{{ record.creator_name || `用户 ${record.creator_id}` }}</strong>
          <small>{{ formatDate(record.task_created_at) }}</small>
        </div>

        <div class="pm-sync-cell">
          <span class="pm-pill" :class="`pm-sync--${baseSyncStatus(record)}`">
            基础 {{ syncStatusLabel(baseSyncStatus(record)) }}
          </span>
          <small>{{ record.last_base_synced_at ? formatDate(record.last_base_synced_at) : '基础资料尚未同步' }}</small>
          <small v-if="record.base_sync_error" class="pm-error-text">{{ record.base_sync_error }}</small>
          <span class="pm-pill" :class="`pm-sync--${imageSyncStatus(record)}`">
            图片 {{ syncStatusLabel(imageSyncStatus(record)) }}
          </span>
          <small>{{ record.last_image_synced_at ? formatDate(record.last_image_synced_at) : 'ERP 图片尚未同步' }}</small>
          <small v-if="record.image_sync_error" class="pm-error-text">{{ record.image_sync_error }}</small>
          <div v-if="isRecordSyncing(record)" class="pm-sync-progress" aria-hidden="true">
            <span></span>
          </div>
          <small v-if="syncMessageForRecord(record)" class="pm-sync-message">{{ syncMessageForRecord(record) }}</small>
        </div>

        <div class="pm-actions">
          <button type="button" class="pm-btn pm-btn--small" @click="openTask(record.task_id)">打开任务</button>
          <button type="button" class="pm-btn pm-btn--small" :disabled="!record.can_maintain_image" @click="openCandidates(record)">
            选图
          </button>
          <button type="button" class="pm-btn pm-btn--small" :disabled="!record.can_maintain_image" @click="reparseImage(record)">
            重新解析
          </button>
          <button type="button" class="pm-btn pm-btn--small" :disabled="!record.can_sync_erp || isRecordSyncing(record)" @click="requestBaseSync(record)">
            {{ syncActionLabel(record, 'base', '同步基础') }}
          </button>
          <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!record.can_sync_erp || isRecordSyncing(record)" @click="requestSync(record)">
            {{ syncActionLabel(record, 'all', '全部同步') }}
          </button>
          <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!record.can_sync_erp || !record.image_asset_id || isRecordSyncing(record)" @click="requestImageSync(record)">
            {{ syncActionLabel(record, 'image', '同步图片') }}
          </button>
        </div>
      </article>

      <div v-if="!loading && records.length === 0" class="pm-empty">暂无符合条件的产品记录。</div>
    </section>

    <footer class="pm-pagination">
      <button type="button" class="pm-btn pm-btn--ghost" :disabled="filters.page <= 1 || loading" @click="changePage(filters.page - 1)">
        上一页
      </button>
      <span>第 {{ filters.page }} 页</span>
      <button
        type="button"
        class="pm-btn pm-btn--ghost"
        :disabled="records.length < filters.page_size || loading"
        @click="changePage(filters.page + 1)"
      >
        下一页
      </button>
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
            <img
              v-if="previewURLForCandidate(candidate)"
              :src="previewURLForCandidate(candidate)"
              :alt="candidate.file_name"
              loading="lazy"
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
  type ProductImageCandidate,
  type ProductManagementListParams,
  type ProductManagementRecord,
  type ProductSyncStatus,
} from '@/services/api/productManagementApi'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'

type ProductSyncScope = 'all' | 'base' | 'image'

const router = useRouter()
const route = useRoute()

const filters = reactive<
  Required<
    Pick<
      ProductManagementListParams,
      'keyword' | 'issue_scope' | 'image_source' | 'cost_status' | 'sync_status' | 'base_sync_status' | 'image_sync_status' | 'page' | 'page_size'
    >
  >
>({
  keyword: '',
  issue_scope: 'attention',
  image_source: '',
  cost_status: '',
  sync_status: '',
  base_sync_status: '',
  image_sync_status: '',
  page: 1,
  page_size: 20,
})

const records = ref<ProductManagementRecord[]>([])
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
const syncPollTokens = new Map<number, number>()
const syncableRecords = computed(() => records.value.filter((item) => item.can_sync_erp))

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
  syncPollTokens.clear()
})

async function loadRecords(): Promise<void> {
  normalizeIssueScopeForExplicitSuccessFilter()
  loading.value = true
  error.value = ''
  try {
    const result = await productManagementApi.list({
      keyword: filters.keyword,
      issue_scope: filters.issue_scope,
      image_source: filters.image_source,
      cost_status: filters.cost_status,
      sync_status: filters.sync_status,
      base_sync_status: filters.base_sync_status,
      image_sync_status: filters.image_sync_status,
      page: filters.page,
      page_size: filters.page_size,
    })
    records.value = result.data ?? []
    pagination.page = result.pagination?.page ?? filters.page
    pagination.page_size = result.pagination?.page_size ?? filters.page_size
    pagination.total = result.pagination?.total ?? records.value.length
    void resolveRecordPreviewURLs(records.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  filters.page = 1
  void loadRecords()
}

function normalizeIssueScopeForExplicitSuccessFilter(): void {
  if (filters.issue_scope !== 'attention') return
  if (filters.sync_status === 'synced' || filters.base_sync_status === 'synced' || filters.image_sync_status === 'synced') {
    filters.issue_scope = 'all'
  }
}

function changePage(page: number): void {
  filters.page = Math.max(1, page)
  void loadRecords()
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
  void resolveRecordPreviewURLs([next])
}

function hasCost(record: ProductManagementRecord): boolean {
  return typeof record.cost_price === 'number' && record.cost_price > 0
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

async function resolveRecordPreviewURLs(items: ProductManagementRecord[]): Promise<void> {
  const next = { ...recordPreviewURLs.value }
  await Promise.all(
    items.map(async (item) => {
      const assetID = item.image_asset_id ?? assetIDFromPreviewPath(item.image_preview_url)
      const url = await resolveAssetPreviewURL(assetID, item.image_preview_url)
      if (url) next[item.id] = url
      else delete next[item.id]
    }),
  )
  recordPreviewURLs.value = next
}

async function resolveCandidatePreviewURLs(items: ProductImageCandidate[]): Promise<void> {
  const next = { ...candidatePreviewURLs.value }
  await Promise.all(
    items.map(async (item) => {
      const url = await resolveAssetPreviewURL(item.asset_id, item.preview_url)
      if (url) next[item.asset_id] = url
      else delete next[item.asset_id]
    }),
  )
  candidatePreviewURLs.value = next
}

async function resolveAssetPreviewURL(assetID?: number | null, fallback?: string): Promise<string> {
  const direct = directPreviewURL(fallback)
  if (direct) return direct
  if (!assetID || assetID <= 0) return ''
  const result = await fetchAssetPreviewMeta(String(assetID)).catch(() => null)
  return result?.status === 'ok' && result.displayUrl ? result.displayUrl : ''
}

function directPreviewURL(raw?: string): string {
  const value = String(raw ?? '').trim()
  if (!value) return ''
  if (/^(https?:|data:|blob:)/i.test(value)) return value
  return ''
}

function assetIDFromPreviewPath(raw?: string): number | undefined {
  const match = String(raw ?? '').match(/\/v1\/assets\/(\d+)\/preview\b/)
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
  color: #111827;
  background: #f5f6f8;
}

.pm-header,
.pm-filters,
.pm-table-shell,
.pm-modal {
  border: 1px solid #e5e7eb;
  background: #ffffff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
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
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
}

.pm-header h1,
.pm-modal h2 {
  margin: 0;
  color: #111827;
  font-size: clamp(1.65rem, 2.5vw, 2.25rem);
  font-weight: 900;
  letter-spacing: 0;
}

.pm-subtitle {
  max-width: 760px;
  margin: 0.55rem 0 0;
  color: #4b5563;
  line-height: 1.65;
}

.pm-filters {
  display: grid;
  grid-template-columns: minmax(18rem, 2fr) repeat(4, minmax(8rem, 1fr)) auto;
  gap: 0.75rem;
  margin-top: 0.85rem;
  padding: 0.9rem;
  border-radius: 0.875rem;
}

.pm-field {
  display: grid;
  gap: 0.35rem;
  color: #6b7280;
  font-size: 12px;
  font-weight: 800;
}

.pm-field input,
.pm-field select {
  width: 100%;
  min-height: 2.25rem;
  border: 1px solid #d1d5db;
  border-radius: 0.625rem;
  padding: 0 12px;
  color: #111827;
  background: #ffffff;
  outline: none;
}

.pm-field input:focus,
.pm-field select:focus {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

.pm-btn {
  min-height: 2.25rem;
  border: 1px solid #d1d5db;
  border-radius: 0.625rem;
  padding: 0 16px;
  color: #374151;
  background: #ffffff;
  font-weight: 800;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.pm-btn:hover:not(:disabled) {
  border-color: #93c5fd;
  color: #1d4ed8;
  background: #eff6ff;
}

.pm-btn:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.pm-btn--primary {
  border-color: #2563eb;
  background: #2563eb;
  color: #ffffff;
}

.pm-btn--primary:hover:not(:disabled) {
  border-color: #1d4ed8;
  color: #ffffff;
  background: #1d4ed8;
}

.pm-btn--ghost {
  background: #f9fafb;
}

.pm-btn--small {
  min-height: 1.9rem;
  padding: 0 10px;
  font-size: 12px;
}

.pm-summary,
.pm-pagination {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 0.8rem 0;
  color: #4b5563;
}

.pm-error,
.pm-error-text {
  color: #dc2626;
}

.pm-table-shell {
  overflow: hidden;
  border-radius: 1rem;
}

.pm-table-head,
.pm-row {
  display: grid;
  grid-template-columns: 9.5rem minmax(8.5rem, 0.9fr) minmax(18rem, 1.6fr) 6.5rem minmax(8rem, 0.8fr) minmax(8.5rem, 0.8fr) minmax(13rem, 0.9fr);
  gap: 0.9rem;
  align-items: center;
}

.pm-table-head {
  padding: 0.75rem 1rem;
  color: #64748b;
  background: #f8fafc;
  font-size: 12px;
  font-weight: 900;
}

.pm-row {
  padding: 0.9rem 1rem;
  border-top: 1px solid #e5e7eb;
  background: #ffffff;
}

.pm-row:hover {
  background: #f8fbff;
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
  border: 1px solid #dbe3ee;
  border-radius: 0.75rem;
  background: #f8fafc;
  color: #dc2626;
  font-size: 12px;
  font-weight: 800;
  text-align: center;
}

.pm-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #ffffff;
}

.pm-preview--missing {
  border-style: dashed;
}

.pm-mono,
.pm-cost-cell {
  font-family: "SF Mono", "IBM Plex Mono", Consolas, monospace;
}

.pm-main-cell strong,
.pm-info-cell strong {
  overflow: hidden;
  color: #111827;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-main-cell small,
.pm-info-cell small,
.pm-sync-cell small {
  color: #6b7280;
}

.pm-cost-cell {
  color: #047857;
  font-weight: 900;
}

.pm-cost-cell.is-missing {
  color: #dc2626;
}

.pm-link {
  width: fit-content;
  border: 0;
  padding: 0;
  color: #2563eb;
  background: transparent;
  font-weight: 800;
  cursor: pointer;
  text-align: left;
}

.pm-pill {
  width: fit-content;
  max-width: 100%;
  border: 1px solid #dbeafe;
  border-radius: 999px;
  padding: 4px 9px;
  color: #1d4ed8;
  background: #eff6ff;
  font-size: 12px;
  font-weight: 900;
  white-space: nowrap;
}

.pm-source--manual {
  border-color: #facc15;
  color: #854d0e;
  background: #fef9c3;
}

.pm-source--delivery {
  border-color: #86efac;
  color: #166534;
  background: #dcfce7;
}

.pm-source--derived_preview {
  border-color: #7dd3fc;
  color: #075985;
  background: #e0f2fe;
}

.pm-source--task_reference {
  border-color: #c4b5fd;
  color: #5b21b6;
  background: #ede9fe;
}

.pm-source--erp_product_image {
  border-color: #5eead4;
  color: #0f766e;
  background: #ccfbf1;
}

.pm-source--auto_on_close {
  border-color: #93c5fd;
  color: #1d4ed8;
  background: #dbeafe;
}

.pm-source--missing,
.pm-sync--failed {
  border-color: #fecaca;
  color: #b91c1c;
  background: #fef2f2;
}

.pm-sync--synced {
  border-color: #86efac;
  color: #166534;
  background: #dcfce7;
}

.pm-sync--queued,
.pm-sync--syncing,
.pm-sync--pending_sync {
  border-color: #bfdbfe;
  color: #1d4ed8;
  background: #eff6ff;
}

.pm-sync--cooling_down {
  border-color: #fde68a;
  color: #92400e;
  background: #fffbeb;
}

.pm-sync--waiting_image {
  border-color: #fed7aa;
  color: #c2410c;
  background: #fff7ed;
}

.pm-sync-progress {
  position: relative;
  width: min(13rem, 100%);
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: #e5edf7;
}

.pm-sync-progress span {
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #38bdf8);
  animation: pm-sync-flow 1.1s ease-in-out infinite;
}

.pm-sync-message {
  color: #1d4ed8;
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
  color: #6b7280;
  text-align: center;
}

.pm-modal-mask {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(15, 23, 42, 0.42);
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
  border: 1px solid #bfdbfe;
  border-radius: 0.875rem;
  background: #eff6ff;
}

.pm-candidate-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 14px;
}

.pm-candidate {
  display: grid;
  gap: 8px;
  border: 1px solid #d1d5db;
  border-radius: 0.875rem;
  padding: 12px;
  color: #111827;
  background: #ffffff;
  text-align: left;
  cursor: pointer;
}

.pm-candidate:hover {
  border-color: #2563eb;
  background: #f8fbff;
}

.pm-candidate img {
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: contain;
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  background: #f8fafc;
}

.pm-candidate strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-candidate small {
  color: #6b7280;
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

  .pm-preview {
    width: 100%;
    height: 160px;
  }
}
</style>
