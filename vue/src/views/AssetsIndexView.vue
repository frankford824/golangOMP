<template>
  <div class="assets-index-view min-h-[100dvh] pb-16">
    <header class="ac-header">
      <div class="ac-nav-box">
        <h1 class="ac-brand">资产管理</h1>
        <div class="ac-search-wrap">
          <svg
            class="ac-search-icon"
            width="18"
            height="18"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            aria-hidden="true"
          >
            <circle cx="9" cy="9" r="7" />
            <path d="M14 14l4.5 4.5" />
          </svg>
          <input
            v-model="filters.keyword"
            type="search"
            class="ac-search-input"
            placeholder="搜索资产 ID、任务 ID、SKU…"
            autocomplete="off"
            enterkeyhint="search"
          />
        </div>
        <div class="ac-header-actions">
          <span class="ac-aria-hint" aria-live="polite">{{ copyHint }}</span>
          <button
            type="button"
            class="ac-icon-btn"
            :aria-expanded="filtersExpanded"
            aria-controls="asset-filters-panel"
            @click="filtersExpanded = !filtersExpanded"
          >
            筛选
          </button>
          <button type="button" class="ac-icon-btn" :disabled="loading" @click="reload">
            {{ loading ? '刷新中' : '刷新' }}
          </button>
        </div>
      </div>

      <div
        v-show="filtersExpanded"
        id="asset-filters-panel"
        class="ac-filters-panel"
      >
        <div class="ac-filters-grid">
          <BaseInput v-model="filters.taskId" label="任务 ID" placeholder="按任务 ID 过滤" />
          <BaseSelect
            v-model="filters.assetKind"
            label="资产类型"
            placeholder="全部类型"
            :options="assetKindOptions"
            clearable
          />
          <BaseInput
            v-model="filters.scopeSkuCode"
            label="SKU 作用域"
            placeholder="请输入 SKU 编码"
          />
        </div>
        <p v-if="keywordAutoQueryHint" class="ac-filter-hint">{{ keywordAutoQueryHint }}</p>
      </div>
    </header>

    <div
      v-if="!loading && !error"
      class="ac-status-bar"
      role="status"
    >
      当前共 <b>{{ listTotal }}</b> 条，当前页返回 <b>{{ assets.length }}</b> 条，展示 <b>{{ pagedAssets.length }}</b> 条
    </div>

    <main class="ac-grid">
      <div v-if="loading" class="ac-loading-state">
        <p class="ac-loading-title">正在加载</p>
        <p class="ac-loading-sub">请稍候…</p>
      </div>
      <div v-else-if="error" class="ac-loading-state ac-state-error">{{ error }}</div>
      <div v-else-if="!pagedAssets.length" class="ac-grid-empty">
        <BaseEmptyState
          title="暂无资产"
          description="请输入关键词或调整筛选条件；若无匹配将显示此提示。"
        />
      </div>
      <template v-else>
        <article
          v-for="asset in pagedAssets"
          :key="asset.id"
          class="ac-card"
          :class="{ 'ac-card--active': selectedAsset?.id === asset.id }"
          @click="selectAsset(asset)"
        >
          <div class="ac-card-img-box">
            <AssetPreviewMedia
              :asset-id="String(asset.id)"
              :resolved-preview-url="listCardResolvedPreviewUrl(asset)"
              defer-until-visible
              alt=""
              img-class="ac-card-apm"
              inner-img-class="ac-card-preview-img"
              @open-full="(u) => (previewLightboxSrc = u)"
            />
          </div>
          <div class="ac-card-info">
            <h2 class="ac-card-title" :title="cardTitle(asset)">{{ cardTitle(asset) }}</h2>
            <div class="ac-card-meta">
              <span class="ac-mono">ID: {{ asset.id }}</span>
              <button
                type="button"
                class="ac-copy-tag"
                @click.stop="copyAssetId(String(asset.id))"
              >
                复制
              </button>
              <div class="ac-card-spec">{{ cardSpecLine(asset) }}</div>
            </div>
          </div>
          <div class="ac-card-footer">
            <div>
              <div class="ac-footer-label">版本</div>
              <div class="ac-footer-stat">{{ versionCount(asset) }}</div>
            </div>
            <div class="ac-footer-right">
              <span class="ac-footer-tag">{{ assetKind(asset) }}</span>
            </div>
          </div>
          <div class="ac-card-actions">
            <button
              type="button"
              class="ac-card-link-btn"
              @click.stop="openAssetDetail(String(asset.id))"
            >
              打开详情页
            </button>
          </div>
        </article>
      </template>
    </main>

    <div v-if="!loading && !error && listTotal > 0" class="ac-pagination">
      <label class="ac-page-size">
        每页
        <select v-model.number="listPageSize" class="ac-page-size-select">
          <option :value="20">20</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
        条
      </label>
      <button
        type="button"
        class="ac-pg-btn"
        :disabled="listPage <= 1"
        @click="goListPage(listPage - 1)"
      >
        上一页
      </button>
      <span class="ac-pg-meta">
        第 {{ listPage }} / {{ listTotalPages }} 页（本页 {{ pagedAssets.length }} / 总计 {{ listTotal }}）
      </span>
      <label class="ac-page-jump">
        跳至
        <input
          v-model.number="listJumpPage"
          type="number"
          min="1"
          :max="listTotalPages"
          class="ac-page-jump-input"
          @keyup.enter="jumpListPage"
        />
        页
      </label>
      <button type="button" class="ac-pg-btn" @click="jumpListPage">跳转</button>
      <button
        type="button"
        class="ac-pg-btn"
        :disabled="listPage >= listTotalPages"
        @click="goListPage(listPage + 1)"
      >
        下一页
      </button>
    </div>

    <BaseModal
      v-model="detailModalOpen"
      title="资产详情"
      :show-confirm="false"
      cancel-text="关闭"
      panel-class="max-w-4xl"
    >
      <div v-if="detailLoading" class="state-text">详情加载中…</div>
      <div v-else-if="detailError" class="state-text state-error">{{ detailError }}</div>
      <BaseEmptyState
        v-else-if="!selectedAsset"
        title="未选择资产"
        description="请从列表中选择一条资产。"
      />
      <template v-else>
        <section class="preview-panel">
          <h4 class="subsection-title">预览内容</h4>
          <div class="preview-media-shell">
            <AssetPreviewMedia
              :asset-id="selectedAssetIdForPreview"
              :fallback-src="selectedPreviewFallbackUrl"
              :resolved-preview-url="selectedPreviewFallbackUrl"
              alt="资产预览"
              inner-img-class="preview-media-img"
              @open-full="(u) => (previewLightboxSrc = u)"
            />
          </div>
          <div class="preview-actions">
            <AssetDownloadLink
              v-if="selectedAssetIdForPreview || selectedPreviewFallbackUrl"
              variant="button"
              :asset-id="selectedAssetIdForPreview"
              :href="selectedPreviewFallbackUrl"
            >
              下载文件
            </AssetDownloadLink>
            <span class="preview-state-hint">预览状态：{{ previewStateLabel }}</span>
          </div>
        </section>

        <dl class="detail-grid">
          <div class="detail-row">
            <dt>资产 ID</dt>
            <dd class="cell-mono">{{ selectedAsset.id }}</dd>
          </div>
          <div class="detail-row">
            <dt>任务 ID</dt>
            <dd class="cell-mono">{{ displayText(selectedAsset.task_id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>类型</dt>
            <dd>{{ assetKind(selectedAsset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>SKU 作用域</dt>
            <dd class="cell-mono">{{ displayText(selectedAsset.scope_sku_code) }}</dd>
          </div>
          <div class="detail-row">
            <dt>上传状态</dt>
            <dd>{{ assetUploadStatus(selectedAsset.upload_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>归档状态</dt>
            <dd>{{ assetArchiveStatus(selectedAsset.archive_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>来源源稿</dt>
            <dd class="cell-mono">{{ displayText(selectedAsset.source_asset_id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>最后访问</dt>
            <dd>{{ displayText(selectedAsset.last_access_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>下载模式</dt>
            <dd>{{ assetDownloadMode(downloadMeta?.download_mode) }}</dd>
          </div>
          <div class="detail-row">
            <dt>预览可用</dt>
            <dd>{{ previewStateLabel }}</dd>
          </div>
        </dl>

        <div class="versions-section">
          <h4 class="subsection-title">版本记录</h4>
          <BaseEmptyState
            v-if="!selectedVersions.length"
            title="暂无版本"
            description="该资产当前未返回嵌套版本。"
          />
          <div v-else class="version-list">
            <article v-for="version in selectedVersions" :key="version.id" class="version-card">
              <div class="version-top">
                <span class="version-title">版本 {{ displayText(version.version ?? version.id) }}</span>
                <span class="version-pill">{{ assetKind(version.file_role) }}</span>
              </div>
              <dl class="version-grid">
                <div class="detail-row">
                  <dt>文件名</dt>
                  <dd>{{ displayText(version.file_name) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>MIME</dt>
                  <dd>{{ displayText(version.mime_type) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>下载模式</dt>
                  <dd>{{ assetDownloadMode(version.download_mode) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>可预览</dt>
                  <dd>{{ version.preview_available === true ? '是' : version.preview_available === false ? '否' : '—' }}</dd>
                </div>
                <div class="detail-row">
                  <dt>创建时间</dt>
                  <dd>{{ displayTime(version.created_at) }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </div>
      </template>
    </BaseModal>

    <div
      v-if="previewLightboxSrc"
      class="preview-lightbox"
      role="dialog"
      aria-modal="true"
      aria-label="大图预览"
      @click="previewLightboxSrc = null"
    >
      <img :src="previewLightboxSrc" alt="资产预览大图" class="preview-lightbox-img" @click.stop />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import AssetDownloadLink from '@/components/media/AssetDownloadLink.vue'
import { usePermission } from '@/composables/usePermission'
import {
  assetArchiveStatusLabelCn,
  assetDownloadModeLabelCn,
  assetKindLabelCn,
  assetUploadStatusLabelCn,
} from '@/domain/mappers/read-model-labels-cn'
import { normalizeAssetDetailFromApi } from '@/domain/mappers/asset-detail-from-api'
import {
  fetchTaskAssetPreviewWithDerivedFallback,
  primeAssetDownloadMetaCache,
} from '@/domain/asset-access'
import { assetsApi } from '@/services/api/assetsApi'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import { formatDateTimeBeijing } from '@/utils/date'

const route = useRoute()
const router = useRouter()
const { canAccessPage } = usePermission()
const loading = ref(false)
const error = ref('')
const assets = ref<BackendAsset[]>([])
const selectedAssetId = ref('')
const detailLoading = ref(false)
const detailError = ref('')
const selectedAssetDetail = ref<BackendAsset | null>(null)
const downloadMeta = ref<Record<string, unknown> | null>(null)
const previewMeta = ref<Record<string, unknown> | null>(null)
const previewUnavailable = ref(false)
const previewNotFound = ref(false)
const detailModalOpen = ref(false)
const listPage = ref(1)
const listJumpPage = ref(1)
const listPageSize = ref(20)
const listTotal = ref(0)
const AUTO_RELOAD_DELAY_MS = 400
let reloadTimer: ReturnType<typeof setTimeout> | null = null
const previewLightboxSrc = ref<string | null>(null)
const filtersExpanded = ref(false)
const copyHint = ref('')

const filters = reactive({
  keyword: '',
  taskId: '',
  assetKind: '',
  scopeSkuCode: '',
})

const assetKindOptions: BaseSelectOption[] = [
  { value: 'reference', label: '运营参考图（reference）' },
  { value: 'source', label: '设计源文件 / 审核修订源文件（source）' },
  { value: 'delivery', label: '最终成品图（delivery）' },
  { value: 'preview', label: '预览辅助（preview）' },
  { value: 'design_thumb', label: '预览辅助（design_thumb）' },
]

const requestedTaskId = computed(() => {
  const raw = route.query.task_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const requestedAssetId = computed(() => {
  const raw = route.query.asset_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const selectedAsset = computed(
  () =>
    selectedAssetDetail.value ??
    assets.value.find((item) => String(item.id) === selectedAssetId.value) ??
    null,
)

const selectedVersions = computed<BackendAssetVersion[]>(() => selectedAsset.value?.versions ?? [])

const effectiveSearchKeyword = computed(
  () => filters.keyword.trim() || filters.taskId.trim() || filters.scopeSkuCode.trim(),
)

const keywordAutoQueryHint = computed(() => {
  if (!filters.taskId.trim() && !filters.scopeSkuCode.trim()) return ''
  return '任务 ID / SKU 作用域将作为 keyword 交给后端统一搜索'
})

const listTotalPages = computed(() =>
  Math.max(1, Math.ceil(listTotal.value / listPageSize.value)),
)

const pagedAssets = computed(() => {
  const selectedKind = filters.assetKind.trim()
  if (!selectedKind) return assets.value
  return assets.value.filter((asset) => {
    const record = asset as Record<string, unknown>
    return String(record.asset_kind ?? record.asset_type ?? asset.file_role ?? '') === selectedKind
  })
})

watch(listTotalPages, (tp) => {
  if (listPage.value > tp) listPage.value = tp
})

watch(
  () => [filters.keyword, filters.taskId, filters.assetKind, filters.scopeSkuCode],
  () => {
    listPage.value = 1
    scheduleReload()
  },
)

watch(listPageSize, () => {
  listPage.value = 1
  scheduleReload()
})

watch(listPage, (p) => {
  listJumpPage.value = p
  scheduleReload()
})

const previewStateLabel = computed(() => {
  if (previewUnavailable.value) return '当前不可预览（仅可下载，非不存在）'
  if (previewNotFound.value) return '预览资源不存在（404）'
  if (previewMeta.value?.preview_available === true) return '可预览'
  if (previewMeta.value?.preview_available === false) return '不可预览'
  if (previewMeta.value?.download_url) return '可预览'
  return '—'
})

const selectedAssetIdForPreview = computed(() => {
  if (previewMeta.value?.download_url || previewUnavailable.value || previewNotFound.value) {
    return undefined
  }
  const id = String(selectedAsset.value?.id ?? '').trim()
  return id || undefined
})

const selectedPreviewFallbackUrl = computed(() => {
  const url = String(previewMeta.value?.download_url ?? '').trim()
  return url || undefined
})

function cardTitle(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const fn = r.file_name
  if (typeof fn === 'string' && fn.trim()) return fn.trim()
  return `${assetKind(asset)} #${asset.id}`
}

function firstDisplayImageUrl(asset: BackendAsset): string {
  const vers = asset.versions
  if (!Array.isArray(vers)) return ''
  const preferred = vers.find(
    (v) =>
      v.preview_available === true &&
      typeof v.download_url === 'string' &&
      v.download_url.trim().length > 5,
  )
  if (preferred?.download_url) return preferred.download_url.trim()
  const anyUrl = vers.find(
    (v) => typeof v.download_url === 'string' && v.download_url.trim().length > 5,
  )
  return anyUrl?.download_url?.trim() ?? ''
}

/** 列表 DTO 已带可展示 URL 时跳过 GET /preview */
function listCardResolvedPreviewUrl(asset: BackendAsset): string | undefined {
  const fromVers = firstDisplayImageUrl(asset)
  if (fromVers) return fromVers
  const r = asset as Record<string, unknown>
  for (const key of ['download_url', 'downloadUrl', 'preview_url', 'previewUrl'] as const) {
    const v = r[key]
    if (typeof v === 'string' && v.trim().length > 5) return v.trim()
  }
  return undefined
}

function cardSpecLine(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const sku = r.scope_sku_code
  if (typeof sku === 'string' && sku.trim()) return sku.trim()
  const task = asset.task_id
  if (task != null && String(task).trim()) return `任务 ${task}`
  return assetKind(asset)
}

async function copyAssetId(id: string) {
  try {
    await navigator.clipboard.writeText(id)
    copyHint.value = '已复制资产 ID'
    window.setTimeout(() => {
      copyHint.value = ''
    }, 1200)
  } catch {
    copyHint.value = '复制失败'
    window.setTimeout(() => {
      copyHint.value = ''
    }, 1200)
  }
}

function displayText(value: unknown): string {
  if (value == null) return '—'
  const text = String(value).trim()
  return text || '—'
}

function displayTime(value: unknown): string {
  const text = displayText(value)
  if (text === '—') return text
  return formatDateTimeBeijing(text) || text
}

function assetKind(asset: BackendAsset | string | null | undefined): string {
  if (typeof asset === 'string') return assetKindLabelCn(asset)
  if (!asset) return '—'
  const record = asset as Record<string, unknown>
  return assetKindLabelCn(String(record.asset_kind ?? record.asset_type ?? asset.file_role ?? ''))
}

function assetUploadStatus(value: unknown): string {
  return assetUploadStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetArchiveStatus(value: unknown): string {
  return assetArchiveStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetDownloadMode(value: unknown): string {
  return assetDownloadModeLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function versionCount(asset: BackendAsset): number {
  return Array.isArray(asset.versions) ? asset.versions.length : 0
}

function selectAsset(asset: BackendAsset) {
  selectedAssetId.value = String(asset.id)
  syncQuerySelection()
  detailModalOpen.value = true
  void loadAssetDetail(String(asset.id))
}

function goListPage(next: number) {
  const clamped = Math.min(Math.max(1, next), listTotalPages.value)
  listPage.value = clamped
}

function jumpListPage() {
  const target = Number(listJumpPage.value)
  if (!Number.isFinite(target)) {
    listJumpPage.value = listPage.value
    return
  }
  goListPage(Math.trunc(target))
}

function scheduleReload() {
  if (reloadTimer) {
    clearTimeout(reloadTimer)
    reloadTimer = null
  }
  reloadTimer = setTimeout(() => {
    void reload()
  }, AUTO_RELOAD_DELAY_MS)
}

function syncQuerySelection() {
  const nextQuery: Record<string, string> = {}
  if (filters.taskId.trim()) nextQuery.task_id = filters.taskId.trim()
  if (selectedAssetId.value.trim()) nextQuery.asset_id = selectedAssetId.value.trim()
  void router.replace({ query: nextQuery })
}

function openAssetDetail(assetId: string) {
  if (!canAccessPage('asset_detail')) return
  const query: Record<string, string> = {}
  if (filters.taskId.trim()) query.task_id = filters.taskId.trim()
  void router.push({ name: 'AssetDetail', params: { id: assetId }, query })
}

async function loadAssetDetail(assetId: string) {
  detailLoading.value = true
  detailError.value = ''
  selectedAssetDetail.value = null
  downloadMeta.value = null
  previewMeta.value = null
  previewUnavailable.value = false
  previewNotFound.value = false
  try {
    const [assetRes, downloadRes] = await Promise.allSettled([
      assetsApi.getAsset(assetId),
      assetsApi.getAssetDownloadMeta(assetId),
    ])

    if (assetRes.status === 'fulfilled') {
      selectedAssetDetail.value = normalizeAssetDetailFromApi(assetRes.value.data)
    }

    if (downloadRes.status === 'fulfilled') {
      const body = downloadRes.value.data as { data?: Record<string, unknown> } | undefined
      downloadMeta.value = body?.data ?? null
      primeAssetDownloadMetaCache(assetId, downloadRes.value.data)
    }

    const tid =
      filters.taskId.trim() ||
      String(
        (selectedAssetDetail.value as Record<string, unknown> | null)?.task_id ??
          assets.value.find((a) => String(a.id) === assetId)?.task_id ??
          '',
      ).trim()
    const previewResult = await fetchTaskAssetPreviewWithDerivedFallback(
      assetId,
      tid || undefined,
    )
    if (previewResult.status === 'ok' && previewResult.displayUrl) {
      previewMeta.value = {
        download_url: previewResult.displayUrl,
        preview_available: true,
      }
    } else if (previewResult.status === 'unavailable') {
      previewUnavailable.value = true
    } else if (previewResult.status === 'not_found') {
      previewNotFound.value = true
    }

    if (!selectedAssetDetail.value) {
      selectedAssetDetail.value =
        assets.value.find((item) => String(item.id) === assetId) ?? null
    }
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : '加载资产详情失败'
    selectedAssetDetail.value =
      assets.value.find((item) => String(item.id) === assetId) ?? null
  } finally {
    detailLoading.value = false
  }
}

async function reload() {
  loading.value = true
  error.value = ''
  try {
    const res = await assetsApi.searchAssets({
      keyword: effectiveSearchKeyword.value || undefined,
      page: listPage.value,
      size: listPageSize.value,
    })
    const body = res.data
    const backendItems = Array.isArray(body?.data) ? body.data : []
    const backendTotal = Number(body?.total)
    const backendPage = Number(body?.page)
    const backendSize = Number(body?.size)

    assets.value = backendItems
    listTotal.value = Number.isFinite(backendTotal) && backendTotal >= 0 ? backendTotal : backendItems.length
    if (Number.isFinite(backendPage) && backendPage > 0) {
      listPage.value = Math.trunc(backendPage)
    }
    if (Number.isFinite(backendSize) && backendSize > 0) {
      listPageSize.value = Math.trunc(backendSize)
    }
    if (!assets.value.length) {
      selectedAssetId.value = ''
      selectedAssetDetail.value = null
      detailModalOpen.value = false
      syncQuerySelection()
    } else {
      let nextId = ''
      if (requestedAssetId.value && assets.value.some((item) => String(item.id) === requestedAssetId.value)) {
        nextId = requestedAssetId.value
      } else if (
        selectedAssetId.value &&
        assets.value.some((item) => String(item.id) === selectedAssetId.value)
      ) {
        nextId = selectedAssetId.value
      }
      selectedAssetId.value = nextId
      syncQuerySelection()
      detailModalOpen.value = false
      selectedAssetDetail.value = null
      downloadMeta.value = null
      previewMeta.value = null
      previewUnavailable.value = false
      previewNotFound.value = false
      detailError.value = ''
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载资产列表失败'
    assets.value = []
    listTotal.value = 0
    selectedAssetId.value = ''
    selectedAssetDetail.value = null
    detailModalOpen.value = false
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (requestedTaskId.value) {
    filters.taskId = requestedTaskId.value
  }
  void reload()
})

onBeforeUnmount(() => {
  if (reloadTimer) {
    clearTimeout(reloadTimer)
    reloadTimer = null
  }
})
</script>

<style scoped>
.assets-index-view {
  --ac-bg: #f5f5f7;
  --ac-card: #fff;
  --ac-text: #1d1d1f;
  --ac-sec: #86868b;
  --ac-accent: #0071e3;
  /** 主内容最大宽度；随主栏变宽可增至约 6 列，典型宽度约 5 列 */
  --ac-content-max: 100%;
  /** 列宽下限：窄屏为整行一列；宽约 1400px 内容区时约 5 列 */
  --ac-grid-min: 220px;
  background: var(--ac-bg);
  color: var(--ac-text);
  padding: 0 0 3.75rem;
}

.ac-header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(245, 245, 247, 0.8);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  padding: 14px 0;
}

.ac-nav-box {
  max-width: var(--ac-content-max);
  margin: 0 auto;
  padding: 0 clamp(30px, 3vw, 50px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.ac-brand {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: var(--ac-text);
}

.ac-search-wrap {
  position: relative;
  flex: 1;
  min-width: 200px;
  max-width: 450px;
}

.ac-search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--ac-sec);
  pointer-events: none;
}

.ac-search-input {
  width: 100%;
  padding: 10px 16px 10px 40px;
  border: none;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.05);
  font-size: 16px;
  outline: none;
  transition: background 0.3s, box-shadow 0.3s;
  color: var(--ac-text);
}

.ac-search-input:focus {
  background: #fff;
  box-shadow: 0 0 0 4px rgba(0, 113, 227, 0.1);
}

.ac-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ac-aria-hint {
  font-size: 12px;
  color: var(--ac-accent);
  min-height: 1em;
}

.ac-icon-btn {
  padding: 8px 14px;
  border-radius: 10px;
  border: 1px solid rgba(0, 0, 0, 0.06);
  background: #fff;
  color: var(--ac-accent);
  font-weight: 500;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.ac-icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ac-filters-panel {
  max-width: var(--ac-content-max);
  margin: 12px auto 0;
  padding: 0 clamp(30px, 3vw, 50px) 16px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.ac-filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.ac-filter-hint {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--ac-sec);
}

.ac-status-bar {
  max-width: var(--ac-content-max);
  margin: 20px auto 0;
  padding: 0 clamp(30px, 3vw, 50px);
  font-size: 13px;
  color: var(--ac-sec);
}

.ac-status-bar b {
  color: var(--ac-text);
  font-weight: 600;
}

.ac-grid {
  width: 100%;
  max-width: var(--ac-content-max);
  margin: 24px auto;
  padding: 0 clamp(30px, 3vw, 50px);
  display: grid;
  grid-template-columns: repeat(
    auto-fill,
    minmax(min(100%, var(--ac-grid-min)), 1fr)
  );
  gap: clamp(16px, 2vw, 22px);
  align-items: stretch;
}

.ac-grid-empty {
  grid-column: 1 / -1;
}

.ac-loading-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 80px 20px;
  color: var(--ac-sec);
}

.ac-loading-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ac-text);
  margin: 0 0 8px;
}

.ac-loading-sub {
  margin: 0;
  font-size: 14px;
}

.ac-state-error {
  color: #b91c1c;
}

.ac-card {
  background: var(--ac-card);
  border-radius: 22px;
  padding: clamp(16px, 2vw, 24px);
  min-width: 0;
  transition: transform 0.35s, box-shadow 0.35s;
  display: flex;
  flex-direction: column;
  cursor: pointer;
  border: 1px solid rgba(0, 0, 0, 0.04);
}

.ac-card:hover {
  transform: scale(1.02) translateY(-5px);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.06);
}

.ac-card--active {
  box-shadow: 0 0 0 2px var(--ac-accent);
}

.ac-card-img-box {
  width: 100%;
  aspect-ratio: 1;
  background: #fbfbfb;
  border-radius: 14px;
  overflow: hidden;
  margin-bottom: 18px;
  display: flex;
  align-items: stretch;
  justify-content: center;
}

.ac-card-img-box :deep(.ac-card-apm) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.ac-card-img-box :deep(.apm-placeholder) {
  border: none;
  background: transparent;
  font-size: 11px;
  min-height: 100%;
  border-radius: 0;
}

.ac-card-preview-img {
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 0;
}

.ac-card-info {
  flex: 1;
}

.ac-card-title {
  font-size: 17px;
  font-weight: 600;
  margin: 0 0 6px;
  line-height: 1.3;
  max-height: 44px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  color: var(--ac-text);
}

.ac-card-meta {
  font-size: 13px;
  color: var(--ac-sec);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.ac-mono {
  margin-right: 4px;
}

.ac-copy-tag {
  color: var(--ac-accent);
  font-size: 11px;
  cursor: pointer;
  margin-left: 8px;
  padding: 0;
  border: none;
  background: none;
  font-family: inherit;
  opacity: 0;
  transition: opacity 0.2s;
}

.ac-card:hover .ac-copy-tag {
  opacity: 1;
}

.ac-card-spec {
  margin-top: 4px;
  opacity: 0.85;
}

.ac-card-footer {
  margin-top: auto;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  padding-top: 15px;
  border-top: 1px solid #f2f2f2;
}

.ac-footer-label {
  font-size: 11px;
  color: var(--ac-sec);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.ac-footer-stat {
  font-size: 26px;
  font-weight: 700;
  color: #334155;
  letter-spacing: -1px;
  line-height: 1.1;
}

.ac-footer-right {
  flex: 1;
  max-width: 60%;
  min-width: 0;
  overflow: hidden;
  text-align: right;
}

.ac-footer-tag {
  display: block;
  font-size: 15px;
  font-weight: 500;
  color: var(--ac-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-card-actions {
  margin-top: 12px;
}

.ac-card-link-btn {
  width: 100%;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: #fff;
  color: var(--ac-accent);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.ac-pagination {
  max-width: var(--ac-content-max);
  margin: 32px auto;
  padding: 0 clamp(30px, 3vw, 50px);
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.ac-pg-btn {
  padding: 10px 20px;
  border-radius: 10px;
  border: 1px solid rgba(0, 0, 0, 0.05);
  background: #fff;
  color: var(--ac-accent);
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.ac-pg-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.ac-pg-meta {
  font-size: 14px;
  color: var(--ac-sec);
}

.ac-page-jump {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ac-sec);
}

.ac-page-jump-input {
  width: 3.5rem;
  height: 32px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  background: #fff;
  padding: 0 8px;
  font-size: 13px;
  color: var(--ac-text);
  outline: none;
}

.ac-page-jump-input:focus {
  border-color: rgba(0, 113, 227, 0.55);
  box-shadow: 0 0 0 2px rgba(0, 113, 227, 0.12);
}

.ac-page-size {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ac-sec);
}

.ac-page-size-select {
  height: 32px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  padding: 0 8px;
  font-size: 13px;
  background: #fff;
  color: var(--ac-text);
}

.cell-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Courier New", monospace;
}

.detail-grid,
.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 0.5rem 1rem;
  margin: 0;
}

.detail-row {
  margin: 0;
}

.detail-row dt {
  font-size: 0.75rem;
  color: #64748b;
  margin-bottom: 0.2rem;
}

.detail-row dd {
  margin: 0;
  font-size: 0.8125rem;
  color: #0f172a;
  word-break: break-word;
}

.versions-section {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid #e2e8f0;
}

.subsection-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 700;
  color: #334155;
}

.version-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 0.75rem;
}

.version-card {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.875rem;
  background: #f8fafc;
}

.version-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.version-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #0f172a;
}

.version-pill {
  border-radius: 9999px;
  background: #e0e7ff;
  color: #4338ca;
  padding: 0.15rem 0.55rem;
  font-size: 0.6875rem;
  font-weight: 600;
}

.preview-panel {
  margin-bottom: 1rem;
}

.preview-media-shell {
  width: 100%;
  min-height: 12rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.75rem;
  background: #f8fafc;
  padding: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-media-shell :deep(.apm) {
  width: 100%;
  min-height: 10rem;
}

.preview-media-shell :deep(.apm-img),
.preview-media-img {
  width: 100%;
  max-height: min(44vh, 360px);
  object-fit: contain;
}

.preview-actions {
  margin-top: 0.65rem;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.preview-state-hint {
  font-size: 0.75rem;
  color: #64748b;
}

.state-text {
  font-size: 0.875rem;
  color: #475569;
}

.state-error {
  color: #b91c1c;
}

.preview-lightbox {
  position: fixed;
  inset: 0;
  z-index: 10000;
  background: rgba(0, 0, 0, 0.85);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: zoom-out;
}

.preview-lightbox-img {
  max-width: 90vw;
  max-height: 90vh;
  object-fit: contain;
  border-radius: 12px;
}
</style>
