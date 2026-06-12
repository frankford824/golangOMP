<template>
  <div class="design-asset-result">
    <div class="design-asset-result-upper" :class="designAssetLayoutClass">
      <div v-if="showReferencePane" class="design-asset-pane design-asset-pane--refs" aria-label="参考图">
        <div class="asset-section ref-rail-section">
          <div class="ref-section-label-row">
            <span class="section-label">{{ batchUi ? '参考图（当前商品）' : '参考图' }}</span>
            <span v-if="referenceEntryCount > 1" class="ref-pane-hint">缩略图预览</span>
          </div>
          <AssetThumbStrip
            :items="referenceThumbItems"
            empty-text="暂无参考图"
            size="sm"
          />
        </div>
      </div>

      <div class="design-asset-pane design-asset-pane--timeline" aria-label="版本时间线">
        <div class="asset-section asset-section--timeline-only">
          <p v-if="sharedAssetVersions.length > 0" class="shared-asset-note">
            共享稿件（未绑定 SKU）{{ sharedAssetVersions.length }} 个版本，未并入当前 SKU 版本。
          </p>
          <div v-if="sharedAssetVersions.length > 0" class="shared-asset-strip">
            <button
              v-for="(v, i) in sharedAssetVersions"
              :key="'shared-ver-' + v.id"
              type="button"
              class="shared-asset-chip"
              @click="onSharedVersionClick(v)"
            >
              共享 V{{ v.rootVersionNo ?? i + 1 }} · {{ versionTotalFileCount(v) }} 文件
            </button>
          </div>
          <div class="version-header">
            <span class="section-label">版本时间线</span>
          </div>
          <div class="timeline-scroll-body">
          <div
            v-for="group in scopedAssetVersionGroups"
            :key="'grp-' + group.assetNo"
            class="version-group"
          >
            <div class="version-group-label">
              <span class="version-group-kind" :class="'kind-' + group.assetKind">
                {{ group.kindLabel }}
              </span>
              <span class="version-group-no">{{ group.assetNo }}</span>
            </div>
            <div class="version-strip version-strip-scroll">
              <button
                v-for="v in group.versions"
                :key="v.id"
                type="button"
                class="version-btn"
                :class="{
                  'version-active': flatIndexOfVersion(v) === activeVersionIdx,
                  'version-disabled': isVersionUnavailable(v),
                }"
                :disabled="isVersionUnavailable(v)"
                :title="isVersionUnavailable(v) ? '该版本上传未完成，暂无可预览或可下载文件' : ''"
                @click="onActivateVersion(flatIndexOfVersion(v))"
              >
                V{{ v.rootVersionNo ?? 1 }}
                <span v-if="isAuditReplacementVersion(v)" class="version-replace-tag">替换</span>
                <span v-if="versionTotalFileCount(v) > 1" class="version-file-count">{{ versionTotalFileCount(v) }}文件</span>
                <span v-if="isVersionUnavailable(v)" class="version-unavailable-tag">不可看</span>
              </button>
            </div>
          </div>
          <p v-if="!scopedAssetVersionGroups.length" class="timeline-empty">暂无版本记录</p>
          </div>
        </div>
      </div>
    </div>

    <section
      v-if="activeVersion"
      class="design-asset-result-manuscript"
      :aria-label="manuscriptSectionLabel"
    >
      <div class="manuscript-head">
        <h4 class="manuscript-title">{{ manuscriptSectionLabel }}</h4>
        <p v-if="activeVersionMeta" class="manuscript-meta">{{ activeVersionMeta }}</p>
      </div>
      <template v-if="isVersionUnavailable(activeVersion)">
        <div class="nonpreview-panel">
          <p class="nonpreview-name">该历史版本上传未完成</p>
          <p class="nonpreview-hint">当前未返回可预览图或下载地址，通常是失败上传留下的占位版本。</p>
        </div>
      </template>
      <template v-else-if="manuscriptCards.length">
        <div class="manuscript-grid" role="list">
          <article
            v-for="card in manuscriptCards"
            :key="card.key"
            class="manuscript-card"
            role="listitem"
          >
            <button
              v-if="card.previewAssetId"
              type="button"
              class="manuscript-card-visual manuscript-card-visual--image"
              :title="card.label"
            >
              <AssetPreviewMedia
                :asset-id="card.previewAssetId"
                :fallback-src="card.previewFallbackSrc || null"
                :alt="card.label"
                img-class="manuscript-preview-media"
                inner-img-class="manuscript-preview-img"
                :defer-until-visible="true"
                @open-full="onPreviewClick"
              />
            </button>
            <button
              v-else-if="card.previewSrc"
              type="button"
              class="manuscript-card-visual manuscript-card-visual--image"
              :title="card.label"
              @click="onPreviewClick(card.previewSrc!)"
            >
              <img :src="card.previewSrc" :alt="card.label" loading="lazy" />
            </button>
            <div
              v-else
              class="manuscript-card-visual manuscript-card-visual--file"
              :title="card.label"
            >
              <span class="manuscript-file-ext">{{ card.extension || '文件' }}</span>
            </div>
            <p class="manuscript-card-label">{{ card.label }}</p>
            <div v-if="card.downloadAssetId || card.downloadHref" class="manuscript-card-actions">
              <AssetDownloadLink
                variant="button"
                :asset-id="card.downloadAssetId"
                :href="card.downloadHref"
              />
            </div>
          </article>
        </div>
      </template>
      <p v-else class="manuscript-empty">当前版本暂无可展示稿件</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskAssetVersion } from '@/domain/types/task'
import AssetDownloadLink from '@/components/media/AssetDownloadLink.vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import { formatDateTimeBeijingOffsetAware } from '@/utils/date'
import {
  downloadHrefForAssetPreviewSlot,
  rasterFallbackForAssetPreviewSlot,
  slotUsesAssetPreviewMedia,
} from '@/domain/task-asset-preview-slot'
import { versionTotalFileCount } from '@/utils/task-ui-labels'

export type AssetRootGroup = {
  assetNo: string
  assetKind: string
  kindLabel: string
  versions: TaskAssetVersion[]
}

type ManuscriptCard = {
  key: string
  label: string
  /** 栅格位可走预览 API 时用 assetRootId + AssetPreviewMedia（与上传态预览区一致） */
  previewAssetId?: string
  previewFallbackSrc?: string
  /** 无 assetRootId 预览链时的浏览器直链缩略图 */
  previewSrc?: string
  extension?: string
  downloadHref?: string
  downloadAssetId?: string
}

const props = defineProps<{
  isRetouchTask: boolean
  batchUi: boolean
  showReferencePane: boolean
  referenceThumbItems: AssetThumbItem[]
  referenceEntryCount: number
  designAssetLayoutClass: string[]
  scopedAssetVersionGroups: AssetRootGroup[]
  sharedAssetVersions: TaskAssetVersion[]
  activeVersionIdx: number
  activeVersion: TaskAssetVersion | null
  flatIndexOfVersion: (v: TaskAssetVersion) => number
  isVersionUnavailable: (v: TaskAssetVersion) => boolean
  isAuditReplacementVersion: (v: TaskAssetVersion) => boolean
}>()

const emit = defineEmits<{
  activateVersion: [index: number]
  openLightbox: [src: string]
  openSharedVersion: [version: TaskAssetVersion]
}>()

const manuscriptSectionLabel = computed(() =>
  props.isRetouchTask ? '精修稿件' : '设计稿件',
)

const activeVersionMeta = computed(() => {
  const v = props.activeVersion
  if (!v) return ''
  const parts: string[] = []
  if (props.isAuditReplacementVersion(v)) parts.push('审核替换')
  if (v.uploaderName) parts.push(v.uploaderName)
  if (v.uploadedAt) parts.push(formatDateTimeBeijingOffsetAware(v.uploadedAt))
  const n = versionTotalFileCount(v)
  if (n > 1) parts.push(`${n} 个文件`)
  return parts.join(' · ')
})

const manuscriptCards = computed((): ManuscriptCard[] => {
  const version = props.activeVersion
  if (!version) return []
  const cards: ManuscriptCard[] = []
  const previews = version.fileRefs ?? []
  previews.forEach((_, index) => {
    const fileIdx = index
    const downloadAssetId = version.assetRootId?.trim() || undefined
    const downloadHref =
      downloadHrefForAssetPreviewSlot(version, fileIdx) ||
      rasterFallbackForAssetPreviewSlot(version, fileIdx)

    if (slotUsesAssetPreviewMedia(version, fileIdx) && downloadAssetId) {
      cards.push({
        key: `preview-${index}`,
        label: `图 ${index + 1}`,
        previewAssetId: downloadAssetId,
        previewFallbackSrc: rasterFallbackForAssetPreviewSlot(version, fileIdx),
        downloadHref,
        downloadAssetId,
      })
      return
    }

    const raster = rasterFallbackForAssetPreviewSlot(version, fileIdx)
    if (!raster) return
    cards.push({
      key: `preview-${index}`,
      label: `图 ${index + 1}`,
      previewSrc: raster,
      downloadHref: downloadHref || raster,
      downloadAssetId,
    })
  })
  const sources = version.nonPreviewFiles ?? []
  sources.forEach((item, index) => {
    const label = item.label?.trim() || `源文件 ${index + 1}`
    const ext = label.includes('.') ? label.split('.').pop()?.toUpperCase() : ''
    cards.push({
      key: `source-${index}`,
      label,
      extension: ext || '文件',
      downloadHref: item.url?.trim() || undefined,
      downloadAssetId: version.assetRootId || undefined,
    })
  })
  return cards
})

function onActivateVersion(index: number) {
  emit('activateVersion', index)
}

function onPreviewClick(src: string) {
  emit('openLightbox', src)
}

function onSharedVersionClick(version: TaskAssetVersion) {
  emit('openSharedVersion', version)
}
</script>

<style scoped>
.design-asset-result {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
}

.design-asset-result-upper {
  width: 100%;
  min-width: 0;
}

.design-asset-result-upper.design-asset-main--split {
  display: grid;
  grid-template-columns: minmax(0, 1.18fr) minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
}

.design-asset-result-upper.design-asset-main--split.design-asset-main--drafts-only {
  grid-template-columns: 1fr;
}

.design-asset-pane--refs,
.design-asset-pane--timeline {
  align-self: start;
}

.design-asset-pane--timeline .asset-section--timeline-only {
  margin-top: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}

.timeline-scroll-body {
  max-height: min(12.5rem, 38vh);
  overflow-x: hidden;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding-right: 0.2rem;
  margin-top: 0.125rem;
}

.timeline-scroll-body .version-group:last-child {
  margin-bottom: 0;
}

.timeline-empty {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: #9ca3af;
}

.manuscript-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-bottom: 0.5rem;
}

.manuscript-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgb(71 85 105);
}

.manuscript-meta {
  margin: 0;
  font-size: 0.6875rem;
  color: #94a3b8;
}

.design-asset-result-manuscript {
  border-radius: 0.625rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
  padding: 0.75rem;
  min-width: 0;
}

.manuscript-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(7.5rem, 1fr));
  gap: 0.625rem;
}

.manuscript-card {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
  border-radius: 0.5rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  padding: 0.45rem;
}

.manuscript-card-visual {
  width: 100%;
  aspect-ratio: 1;
  border-radius: 0.375rem;
  overflow: hidden;
  border: none;
  padding: 0;
  background: rgb(241 245 249);
}

.manuscript-card-visual--image {
  cursor: pointer;
}

.manuscript-card-visual--image img,
.manuscript-preview-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.manuscript-card-visual--image :deep(.manuscript-preview-media) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.manuscript-card-visual--image :deep(.manuscript-preview-media .apm) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.manuscript-card-visual--image :deep(.manuscript-preview-media .apm-img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.manuscript-card-visual--image :deep(.manuscript-preview-media .apm-placeholder),
.manuscript-card-visual--image :deep(.manuscript-preview-media .apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.25rem;
  font-size: 0.625rem;
}

.manuscript-card-visual--file {
  display: flex;
  align-items: center;
  justify-content: center;
}

.manuscript-file-ext {
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: rgb(100 116 139);
}

.manuscript-card-label {
  margin: 0;
  font-size: 0.6875rem;
  color: rgb(71 85 105);
  line-height: 1.3;
  word-break: break-all;
}

.manuscript-card-actions {
  margin-top: auto;
}

.manuscript-empty {
  margin: 0;
  font-size: 0.75rem;
  color: #9ca3af;
}

.design-asset-pane--refs,
.design-asset-pane--timeline {
  border-radius: 0.625rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
  padding: 0.75rem;
  min-width: 0;
}

.ref-section-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.375rem;
}

.section-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(71 85 105);
  text-transform: uppercase;
  display: block;
  margin-bottom: 0.375rem;
}

.ref-pane-hint {
  font-size: 0.6875rem;
  font-weight: 500;
  color: #64748b;
}

.shared-asset-note {
  margin: 0 0 0.375rem;
  font-size: 0.75rem;
  color: #64748b;
}

.shared-asset-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-bottom: 0.5rem;
}

.shared-asset-chip {
  border: 1px solid #dbeafe;
  background: #eff6ff;
  color: #1d4ed8;
  border-radius: 999px;
  font-size: 0.6875rem;
  padding: 0.18rem 0.5rem;
  cursor: pointer;
}

.version-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.375rem;
}

.version-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.375rem;
  min-width: 0;
}

.version-group-label {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
  font-size: 0.6875rem;
}

.version-group-kind {
  padding: 0.15rem 0.4rem;
  border-radius: 0.25rem;
  font-weight: 600;
  border: 1px solid transparent;
}

.version-group-kind.kind-delivery {
  background: #eff6ff;
  color: #1d4ed8;
  border-color: #bfdbfe;
}

.version-group-kind.kind-source {
  background: #fef3c7;
  color: #a16207;
  border-color: #fde68a;
}

.version-group-no {
  color: #94a3b8;
  font-weight: 500;
}

.version-strip {
  display: flex;
  gap: 0.375rem;
  flex-wrap: wrap;
  min-width: 0;
}

.version-strip-scroll {
  flex-wrap: nowrap;
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.version-btn {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.35rem 0.6rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(248 250 252);
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(71 85 105);
  cursor: pointer;
  flex-shrink: 0;
}

.version-btn.version-active {
  border-color: rgb(30 41 59);
  color: rgb(15 23 42);
  background: rgb(241 245 249);
}

.version-btn:disabled {
  cursor: not-allowed;
}

.version-file-count,
.version-replace-tag,
.version-unavailable-tag {
  font-size: 0.5625rem;
  line-height: 1;
  border-radius: 999px;
  padding: 0.1rem 0.32rem;
}

.nonpreview-panel {
  width: 100%;
  min-height: 80px;
  border-radius: 4px;
  border: 1px dashed rgb(203 213 225);
  background: rgb(248 250 252);
  padding: 1rem;
  text-align: center;
}

.nonpreview-name {
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(51 65 85);
  margin: 0 0 0.25rem;
}

.nonpreview-hint {
  font-size: 0.75rem;
  color: rgb(148 163 184);
  margin: 0;
}

@media (max-width: 1023px) {
  .design-asset-result-upper.design-asset-main--split:not(.design-asset-main--drafts-only) {
    grid-template-columns: 1fr;
  }
}
</style>
