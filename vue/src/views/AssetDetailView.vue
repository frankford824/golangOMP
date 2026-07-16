<template>
  <main class="external-detail-page">
    <header class="page-header">
      <div>
        <p class="eyebrow">外部素材库</p>
        <h1>外部素材详情</h1>
        <p>{{ asset ? fileName : '正在加载素材信息…' }}</p>
      </div>
      <div class="page-actions">
        <BaseButton variant="secondary" size="sm" @click="goBack">返回资产中心</BaseButton>
        <BaseButton size="sm" :disabled="loading" @click="loadAsset">{{ loading ? '加载中…' : '刷新' }}</BaseButton>
      </div>
    </header>

    <div v-if="loading && !asset" class="state-card">正在加载外部素材…</div>
    <div v-else-if="error" class="state-card error" role="alert">{{ error }}</div>
    <BaseEmptyState v-else-if="!asset" title="素材不存在" description="请确认素材编号或访问权限。" />

    <template v-else>
      <section class="preview-card" aria-label="素材预览">
        <img v-if="previewURL" :src="previewURL" :alt="fileName" />
        <div v-else class="preview-placeholder">
          <strong>{{ availabilityLabel }}</strong>
          <span>当前没有可直接展示的预览图，可尝试下载原文件。</span>
        </div>
        <div class="preview-actions">
          <a v-if="previewURL" :href="previewURL" target="_blank" rel="noreferrer">打开预览</a>
          <a v-if="downloadURL" :href="downloadURL">下载原文件</a>
        </div>
      </section>

      <section class="detail-card">
        <header><div><p class="eyebrow">素材信息</p><h2>{{ fileName }}</h2></div><span class="status-pill">{{ availabilityLabel }}</span></header>
        <dl class="detail-grid">
          <div><dt>资源编号</dt><dd class="mono">{{ resourceID }}</dd></div>
          <div><dt>文件类型</dt><dd>{{ fileTypeLabel }}</dd></div>
          <div><dt>产品名称</dt><dd>{{ text(record.product_name ?? record.product_name_snapshot) }}</dd></div>
          <div><dt>来源部门</dt><dd>{{ text(record.source_department) }}</dd></div>
          <div><dt>收录时间</dt><dd>{{ displayTime(record.created_at ?? record.uploaded_at) }}</dd></div>
          <div class="full"><dt>外部路径</dt><dd>{{ originPath }}</dd></div>
        </dl>
      </section>
    </template>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import { normalizeAssetDetailFromApi } from '@/domain/mappers/asset-detail-from-api'
import { fetchAssetPreviewMeta, primeAssetDownloadMetaCache } from '@/domain/asset-access'
import { assetsApi } from '@/services/api/assetsApi'
import type { BackendAsset } from '@/services/apiTypes'
import { formatDateTimeBeijing } from '@/utils/date'

const route = useRoute()
const router = useRouter()
const assetId = computed(() => String(route.params.id ?? '').trim())
const loading = ref(false)
const error = ref('')
const asset = ref<BackendAsset | null>(null)
const downloadMeta = ref<Record<string, unknown> | null>(null)
const previewMeta = ref<Record<string, unknown> | null>(null)
const record = computed(() => (asset.value || {}) as Record<string, unknown>)

const resourceID = computed(() => String(record.value.resource_id ?? record.value.resourceId ?? assetId.value))
const fileName = computed(() => text(record.value.file_name ?? record.value.original_filename ?? record.value.filename, '未命名素材'))
const originPath = computed(() => text(record.value.origin_path ?? record.value.originPath ?? record.value.product_name, fileName.value))
const previewURL = computed(() => text(previewMeta.value?.download_url ?? record.value.preview_url, ''))
const downloadURL = computed(() => text(downloadMeta.value?.download_url ?? record.value.download_url, ''))
const fileTypeLabel = computed(() => {
  const match = /\.([a-z0-9]{2,8})(?:$|[?#])/i.exec(fileName.value)
  return match?.[1]?.toUpperCase() || text(record.value.mime_type, '文件')
})
const availabilityLabel = computed(() => {
  if (previewURL.value) return '可预览、可下载'
  if (downloadURL.value) return '可下载'
  const status = String(record.value.external_preview_status ?? record.value.oss_sync_status ?? '').toLowerCase()
  if (status === 'pending') return '正在准备'
  if (status === 'failed') return '暂时不可用'
  return '按需准备'
})

function text(value: unknown, fallback = '—') {
  const normalized = String(value ?? '').trim()
  return normalized || fallback
}
function displayTime(value: unknown) {
  const raw = text(value, '')
  return raw ? formatDateTimeBeijing(raw) || raw : '—'
}
function goBack() { void router.push('/asset-center') }

async function loadAsset() {
  if (!assetId.value.startsWith('ext-')) { asset.value = null; error.value = '该入口只展示外部素材；任务资源请从资源组详情查看。'; return }
  loading.value = true; error.value = ''; asset.value = null; downloadMeta.value = null; previewMeta.value = null
  try {
    const [assetResult, downloadResult, previewResult] = await Promise.allSettled([
      assetsApi.getAsset(assetId.value),
      assetsApi.getAssetDownloadMeta(assetId.value),
      fetchAssetPreviewMeta(assetId.value),
    ])
    if (assetResult.status === 'fulfilled') asset.value = normalizeAssetDetailFromApi(assetResult.value.data)
    if (downloadResult.status === 'fulfilled') {
      const body = downloadResult.value.data as { data?: Record<string, unknown> } | undefined
      downloadMeta.value = body?.data ?? null
      primeAssetDownloadMetaCache(assetId.value, downloadResult.value.data)
    }
    if (previewResult.status === 'fulfilled' && previewResult.value.status === 'ok') {
      previewMeta.value = { download_url: previewResult.value.displayUrl }
    }
    if (!asset.value) error.value = '未获取到外部素材详情。'
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '加载外部素材失败。' }
  finally { loading.value = false }
}

watch(assetId, () => { void loadAsset() })
onMounted(() => { void loadAsset() })
</script>

<style scoped>
.external-detail-page{max-width:1100px;margin:0 auto;padding:28px;display:grid;gap:20px}.page-header,.detail-card>header{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}.page-header h1,.detail-card h2{margin:3px 0}.page-header p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}.page-actions,.preview-actions{display:flex;gap:8px;flex-wrap:wrap}.state-card,.preview-card,.detail-card{border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));padding:20px}.state-card{min-height:180px;display:grid;place-items:center;color:rgb(var(--yb-text-muted))}.state-card.error{color:rgb(var(--yb-danger-text));background:rgb(var(--yb-danger-soft))}.preview-card{display:grid;gap:14px}.preview-card img{width:100%;max-height:520px;object-fit:contain;border-radius:12px;background:rgb(var(--yb-surface-soft))}.preview-placeholder{min-height:260px;display:grid;place-content:center;gap:8px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:12px;color:rgb(var(--yb-text-muted))}.preview-placeholder strong{color:rgb(var(--yb-text))}.preview-actions a{display:inline-flex;align-items:center;min-height:40px;padding:0 15px;border-radius:10px;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));text-decoration:none;font-weight:750}.status-pill{padding:6px 10px;border-radius:999px;background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong));font-size:12px}.detail-grid{display:grid;grid-template-columns:1fr 1fr;gap:0;margin:18px 0 0}.detail-grid>div{display:grid;grid-template-columns:110px 1fr;gap:12px;padding:13px 0;border-top:1px solid rgb(var(--yb-border))}.detail-grid .full{grid-column:1/-1}.detail-grid dt{color:rgb(var(--yb-text-muted))}.detail-grid dd{margin:0;overflow-wrap:anywhere}.mono{font-family:var(--yb-font-mono)}@media(max-width:720px){.external-detail-page{padding:16px}.page-header,.detail-card>header{flex-direction:column}.detail-grid{grid-template-columns:1fr}.detail-grid .full{grid-column:auto}.detail-grid>div{grid-template-columns:1fr;gap:5px}}
</style>
