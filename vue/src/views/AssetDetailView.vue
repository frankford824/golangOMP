<template>
  <div class="asset-detail-view min-h-[100dvh]">
    <div class="page-header">
      <div>
        <h2 class="page-title">资产详情</h2>
        <p class="page-subtitle">
          资产 ID：<span class="cell-mono">{{ assetId }}</span>
        </p>
      </div>
      <div class="page-actions">
        <BaseButton
          v-if="taskId && canAccessPage('task_assets')"
          variant="secondary"
          size="sm"
          @click="goTaskAssets"
        >
          返回任务资产页
        </BaseButton>
        <BaseButton
          v-else-if="canAccessPage('assets_index')"
          variant="secondary"
          size="sm"
          @click="goAssetsIndex"
        >
          返回资产管理
        </BaseButton>
        <BaseButton size="sm" :disabled="loading" @click="loadAsset">
          {{ loading ? '加载中…' : '刷新详情' }}
        </BaseButton>
      </div>
    </div>

    <section class="detail-card">
      <div v-if="loading" class="state-text">加载中…</div>
      <div v-else-if="error" class="state-text state-error">{{ error }}</div>
      <BaseEmptyState
        v-else-if="!asset"
        title="资产不存在"
        description="未拿到资产详情，请确认资产 ID 或访问权限。"
      />
      <template v-else>
        <dl class="detail-grid">
          <div class="detail-row">
            <dt>资产 ID</dt>
            <dd class="cell-mono">{{ displayText(asset.id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>任务 ID</dt>
            <dd class="cell-mono">{{ displayText(asset.task_id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>类型</dt>
            <dd>{{ assetKind(asset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>SKU 作用域</dt>
            <dd class="cell-mono">{{ displayText(asset.scope_sku_code) }}</dd>
          </div>
          <div class="detail-row">
            <dt>上传状态</dt>
            <dd>{{ assetUploadStatus(asset.upload_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>归档状态</dt>
            <dd>{{ assetArchiveStatus(asset.archive_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>来源源稿</dt>
            <dd class="cell-mono">{{ displayText(asset.source_asset_id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>替换前稿件</dt>
            <dd class="cell-mono">{{ displayText(asset.previous_asset_id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>当前有效稿件</dt>
            <dd class="cell-mono">{{ displayText(asset.current_asset_id ?? asset.id) }}</dd>
          </div>
          <div class="detail-row">
            <dt>替换人</dt>
            <dd>{{ replacementActorText(asset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>业务线 / 来源部门</dt>
            <dd>{{ laneAndDepartmentText(asset.workflow_lane, asset.source_department) }}</dd>
          </div>
          <div class="detail-row">
            <dt>最后访问</dt>
            <dd>{{ displayText(asset.last_access_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>下载模式</dt>
            <dd>{{ assetDownloadMode(downloadMeta?.download_mode) }}</dd>
          </div>
          <div class="detail-row">
            <dt>预览状态</dt>
            <dd>{{ previewStateLabel }}</dd>
          </div>
        </dl>

        <div class="versions-section">
          <div class="content-head">
            <h3 class="section-title">版本记录</h3>
            <span class="section-meta">共 {{ versions.length }} 条</span>
          </div>
          <BaseEmptyState
            v-if="!versions.length"
            title="暂无版本"
            description="该资产当前未返回版本信息。"
          />
          <div v-else class="version-list">
            <article v-for="version in versions" :key="version.id" class="version-card">
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
import { userAccountDisplay } from '@/domain/user-display'

const route = useRoute()
const router = useRouter()
const { canAccessPage } = usePermission()

const assetId = computed(() => String(route.params.id ?? '').trim())
const taskId = computed(() => {
  const raw = route.query.task_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const loading = ref(false)
const error = ref('')
const asset = ref<BackendAsset | null>(null)
const downloadMeta = ref<Record<string, unknown> | null>(null)
const previewMeta = ref<Record<string, unknown> | null>(null)
/** GET /preview 返回 409：当前不可预览，仅可下载（非资源不存在） */
const previewUnavailable = ref(false)
/** GET /preview 返回 404：预览入口资源不存在 */
const previewNotFound = ref(false)

const versions = computed<BackendAssetVersion[]>(() => asset.value?.versions ?? [])

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

function displayTime(value: unknown): string {
  const text = displayText(value)
  if (text === '—') return text
  return formatDateTimeBeijing(text) || text
}

function replacementActorText(row: BackendAsset): string {
  return userAccountDisplay(row.replacement_actor_name, row.replacement_actor_username)
}

function assetKind(input: BackendAsset | string | null | undefined): string {
  if (typeof input === 'string') return assetKindLabelCn(input)
  if (!input) return '—'
  const record = input as Record<string, unknown>
  return assetKindLabelCn(String(record.asset_kind ?? record.asset_type ?? input.file_role ?? ''))
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

function laneAndDepartmentText(lane: unknown, department: unknown): string {
  const laneRaw = String(lane ?? '').trim().toLowerCase()
  const laneText = laneRaw === 'customization' ? '定制' : laneRaw === 'normal' ? '普通' : '—'
  const deptText = String(department ?? '').trim() || '—'
  return `${laneText} / ${deptText}`
}

function goTaskAssets() {
  if (!taskId.value || !canAccessPage('task_assets')) return
  void router.push({ name: 'TaskAssets', params: { id: taskId.value }, query: { asset_id: assetId.value } })
}

function goAssetsIndex() {
  if (!canAccessPage('assets_index')) return
  const query = taskId.value ? { task_id: taskId.value, asset_id: assetId.value } : { asset_id: assetId.value }
  void router.push({ name: 'AssetsIndex', query })
}

async function loadAsset() {
  if (!assetId.value) {
    asset.value = null
    error.value = '缺少资产 ID'
    return
  }
  loading.value = true
  error.value = ''
  asset.value = null
  downloadMeta.value = null
  previewMeta.value = null
  previewUnavailable.value = false
  previewNotFound.value = false
  try {
    const [assetRes, downloadRes] = await Promise.allSettled([
      assetsApi.getAsset(assetId.value),
      assetsApi.getAssetDownloadMeta(assetId.value),
    ])

    if (assetRes.status === 'fulfilled') {
      asset.value = normalizeAssetDetailFromApi(assetRes.value.data)
    }

    if (downloadRes.status === 'fulfilled') {
      const body = downloadRes.value.data as { data?: Record<string, unknown> } | undefined
      downloadMeta.value = body?.data ?? null
      primeAssetDownloadMetaCache(assetId.value, downloadRes.value.data)
    }

    const tidFromAsset = String(
      (asset.value as Record<string, unknown> | null)?.task_id ??
        (asset.value as BackendAsset | null)?.task_id ??
        '',
    ).trim()
    const taskIdForPreview = (taskId.value || tidFromAsset).trim() || undefined
    const previewResult = await fetchTaskAssetPreviewWithDerivedFallback(
      assetId.value,
      taskIdForPreview,
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

    if (!asset.value) {
      error.value = '未获取到资产详情'
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载资产详情失败'
  } finally {
    loading.value = false
  }
}

watch(
  () => assetId.value,
  () => {
    void loadAsset()
  },
)

onMounted(() => {
  void loadAsset()
})
</script>

<style scoped>
.asset-detail-view {
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
.detail-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 1rem;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
  padding: 1rem;
}
.state-text {
  font-size: 0.875rem;
  color: #475569;
}
.state-error {
  color: #b91c1c;
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

/* Apple Music / iOS liquid glass asset detail skin. Style-only. */
.asset-detail-view {
  background:
    radial-gradient(circle at 14% 0%, rgba(255, 45, 141, 0.16), transparent 30rem),
    radial-gradient(circle at 86% 6%, rgba(100, 210, 255, 0.15), transparent 32rem) !important;
  color: var(--yb-music-text-2);
}

.detail-card,
.version-card,
.preview-media-shell {
  border-color: var(--yb-music-border) !important;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.055)) !important;
  color: var(--yb-music-text-2) !important;
  box-shadow: var(--yb-glass-shadow) !important;
  backdrop-filter: blur(var(--yb-glass-blur));
  -webkit-backdrop-filter: blur(var(--yb-glass-blur));
}

.page-title,
.section-title,
.version-title,
.detail-row dd {
  color: #fff !important;
}

.page-subtitle,
.section-meta,
.detail-row dt,
.state-text {
  color: var(--yb-music-muted) !important;
}

.version-pill {
  background: rgba(100, 210, 255, 0.15) !important;
  border: 1px solid rgba(100, 210, 255, 0.28);
  color: var(--yb-music-cyan) !important;
}
</style>
