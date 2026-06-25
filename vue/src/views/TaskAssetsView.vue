<template>
  <div class="task-assets-view min-h-[100dvh]">
    <div class="page-header">
      <div>
        <h2 class="page-title">任务资产</h2>
        <p class="page-subtitle">
          当前任务 ID：<span class="cell-mono">{{ taskId }}</span>，下方列表为该任务关联的最新资产快照。
        </p>
      </div>
      <div class="page-actions">
        <BaseButton variant="secondary" size="sm" @click="goTaskDetail">
          返回任务详情
        </BaseButton>
        <BaseButton
          v-if="canAccessPage('assets_index')"
          variant="secondary"
          size="sm"
          @click="goAssetsIndex"
        >
          打开资产管理
        </BaseButton>
        <BaseButton size="sm" :disabled="loading" @click="reload">
          {{ loading ? '加载中…' : '刷新列表' }}
        </BaseButton>
      </div>
    </div>

    <section class="content-card">
      <div class="content-head">
        <h3 class="section-title">任务资产列表</h3>
        <span class="section-meta">共 {{ assets.length }} 条</span>
      </div>

      <div v-if="loading" class="state-text">加载中…</div>
      <div v-else-if="error" class="state-text state-error">{{ error }}</div>
      <BaseEmptyState
        v-else-if="!assets.length"
        title="暂无任务资产"
        description="当前任务还没有返回任何资产记录。"
      />
      <div v-else class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>资产 ID</th>
              <th>类型</th>
              <th>SKU 作用域</th>
              <th>上传状态</th>
              <th>版本数</th>
              <th>详情页</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="asset in assets"
              :key="asset.id"
              :class="{ 'row-active': selectedAsset?.id === asset.id }"
              @click="selectAsset(asset)"
            >
              <td class="cell-mono">{{ displayText(asset.id) }}</td>
              <td>{{ assetKind(asset) }}</td>
              <td class="cell-mono">{{ displayText(asset.scope_sku_code) }}</td>
              <td>{{ assetUploadStatus(asset.upload_status) }}</td>
              <td>{{ versionCount(asset) }}</td>
              <td>
                <BaseButton
                  v-if="canAccessPage('asset_detail')"
                  variant="secondary"
                  size="sm"
                  @click.stop="openAssetDetail(asset.id)"
                >
                  打开详情页
                </BaseButton>
                <span v-else class="action-muted">无权限</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="detail-card">
      <div class="content-head">
        <h3 class="section-title">资产快照</h3>
      </div>
      <div v-if="detailLoading" class="state-text">详情加载中…</div>
      <div v-else-if="detailError" class="state-text state-error">{{ detailError }}</div>
      <BaseEmptyState
        v-else-if="!selectedAsset"
        title="未选择资产"
        description="点击任一资产行查看下载与预览元数据。"
      />
      <template v-else>
        <dl class="detail-grid">
          <div class="detail-row">
            <dt>资产 ID</dt>
            <dd class="cell-mono">{{ displayText(selectedAsset.id) }}</dd>
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
              </dl>
            </article>
          </div>
        </div>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import { usePermission } from '@/composables/usePermission'
import {
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

const route = useRoute()
const router = useRouter()
const { canAccessPage } = usePermission()

const taskId = computed(() => String(route.params.id ?? '').trim())
const requestedAssetId = computed(() => {
  const raw = route.query.asset_id
  return typeof raw === 'string' ? raw.trim() : ''
})

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

const selectedAsset = computed(
  () =>
    selectedAssetDetail.value ??
    assets.value.find((item) => String(item.id) === selectedAssetId.value) ??
    null,
)

const selectedVersions = computed<BackendAssetVersion[]>(() => selectedAsset.value?.versions ?? [])

const previewStateLabel = computed(() => {
  if (previewUnavailable.value) return '当前不可预览（仅可下载，非不存在）'
  if (previewNotFound.value) return '预览资源不存在（404）'
  if (previewMeta.value?.preview_available === true) return '可预览'
  if (previewMeta.value?.preview_available === false) return '不可预览'
  if (previewMeta.value?.download_url) return '可预览'
  return '—'
})

function displayText(value: unknown): string {
  if (value == null) return '—'
  const text = String(value).trim()
  return text || '—'
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

function assetDownloadMode(value: unknown): string {
  return assetDownloadModeLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function versionCount(asset: BackendAsset): number {
  return Array.isArray(asset.versions) ? asset.versions.length : 0
}

function syncQuerySelection() {
  const nextQuery: Record<string, string> = {}
  if (selectedAssetId.value.trim()) nextQuery.asset_id = selectedAssetId.value.trim()
  void router.replace({ query: nextQuery })
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

    const previewResult = await fetchTaskAssetPreviewWithDerivedFallback(assetId, taskId.value)
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

function selectAsset(asset: BackendAsset) {
  selectedAssetId.value = String(asset.id)
  syncQuerySelection()
  void loadAssetDetail(selectedAssetId.value)
}

function openAssetDetail(assetId: string) {
  if (!canAccessPage('asset_detail')) return
  void router.push({
    name: 'AssetDetail',
    params: { id: assetId },
    query: { task_id: taskId.value },
  })
}

function goTaskDetail() {
  void router.push({ name: 'TaskDetail', params: { id: taskId.value } })
}

function goAssetsIndex() {
  if (!canAccessPage('assets_index')) return
  void router.push({ name: 'AssetsIndex', query: { task_id: taskId.value } })
}

async function reload() {
  if (!taskId.value) {
    assets.value = []
    error.value = '缺少任务 ID'
    return
  }
  loading.value = true
  error.value = ''
  try {
    const res = await assetsApi.list(taskId.value)
    assets.value = Array.isArray(res.data) ? res.data : []

    if (!assets.value.length) {
      selectedAssetId.value = ''
      selectedAssetDetail.value = null
      return
    }

    const preferredId =
      requestedAssetId.value && assets.value.some((item) => String(item.id) === requestedAssetId.value)
        ? requestedAssetId.value
        : selectedAssetId.value && assets.value.some((item) => String(item.id) === selectedAssetId.value)
          ? selectedAssetId.value
          : String(assets.value[0]?.id ?? '')

    selectedAssetId.value = preferredId
    syncQuerySelection()
    if (preferredId && requestedAssetId.value && requestedAssetId.value === preferredId) {
      await loadAssetDetail(preferredId)
    } else {
      selectedAssetDetail.value = null
      downloadMeta.value = null
      previewMeta.value = null
      previewUnavailable.value = false
      previewNotFound.value = false
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载任务资产失败'
    assets.value = []
    selectedAssetId.value = ''
    selectedAssetDetail.value = null
  } finally {
    loading.value = false
  }
}

watch(
  () => taskId.value,
  () => {
    void reload()
  },
)

onMounted(() => {
  void reload()
})
</script>

<style scoped>
.task-assets-view {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0.5rem 0 1rem;
  background: #f1f5f9;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 700;
  color: #0f172a;
}
.page-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.page-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.content-card,
.detail-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 1rem;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
  padding: 1rem;
}
.content-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.section-title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 700;
  color: #0f172a;
}
.section-meta {
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
.table-wrap {
  overflow-x: auto;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}
.data-table th,
.data-table td {
  padding: 0.65rem 0.75rem;
  border-bottom: 1px solid #e2e8f0;
  text-align: left;
  vertical-align: top;
}
.data-table th {
  font-size: 0.75rem;
  color: #64748b;
  background: #f8fafc;
}
.data-table tbody tr {
  cursor: pointer;
}
.data-table tbody tr:hover {
  background: #f8fafc;
}
.row-active {
  background: #eef2ff;
}
.cell-mono {
  font-family: var(--yb-font-data);
}
.action-muted {
  font-size: 0.75rem;
  color: #94a3b8;
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
.detail-row-full {
  grid-column: 1 / -1;
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

/* Phase 6: light admin task assets skin. Style-only. */
.task-assets-view {
  background: transparent !important;
  color: #111827;
}

.content-card,
.detail-card,
.version-card {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06) !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

.page-title,
.section-title,
.subsection-title,
.version-title,
.detail-row dd {
  color: #111827 !important;
}

.page-subtitle,
.section-meta,
.detail-row dt,
.state-text,
.action-muted {
  color: #6b7280 !important;
}

.data-table th {
  background: #f3f4f6 !important;
  color: #374151 !important;
}

.data-table td {
  color: #111827;
}

.row-active {
  background: #eff6ff !important;
}

.version-pill {
  background: #eff6ff !important;
  border: 1px solid #bfdbfe !important;
  color: #1d4ed8 !important;
}
</style>
