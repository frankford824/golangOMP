<template>
  <section class="retouch-requirements-block">
    <header class="block-head">
      <div class="block-head-text">
        <p class="block-kicker">P 图需求明细</p>
        <p v-if="requirements.length > 1" class="block-summary">
          共 {{ requirements.length }} 条需求
        </p>
      </div>
      <button
        v-if="totalDownloadableCount > 0"
        type="button"
        class="batch-link-btn"
        :disabled="Boolean(batchLoadingKey)"
        @click="handleBatchAllAttachments"
      >
        {{ batchLoadingKey === 'block-all' ? '打包中…' : '下载全部附件' }}
      </button>
    </header>
    <p v-if="blockBatchError" class="block-batch-error">{{ blockBatchError }}</p>

    <article
      v-for="(item, index) in requirements"
      :key="item.id || index"
      class="requirement-card"
    >
      <header class="requirement-card-head">
        <div class="requirement-card-head-main">
          <span class="requirement-no">需求 {{ index + 1 }}</span>
          <span class="requirement-counts">
            参考图 {{ referenceItems(item).length }} · 素材 {{ sourceItems(item).length }}
          </span>
        </div>
        <button
          v-if="requirementDownloadableCount(item) > 0"
          type="button"
          class="batch-link-btn"
          :disabled="Boolean(batchLoadingKey)"
          @click="handleBatchRequirementAll(item, index)"
        >
          {{ batchLoadingKey === batchScopeKey(item, 'all') ? '打包中…' : '下载本需求全部' }}
        </button>
      </header>

      <p class="requirement-desc">{{ item.description }}</p>

      <dl v-if="hasOptionalFields(item)" class="requirement-meta">
        <div v-if="item.skuCode">
          <dt>SKU / 款号</dt>
          <dd>{{ item.skuCode }}</dd>
        </div>
        <div v-if="item.spec">
          <dt>规格</dt>
          <dd>{{ item.spec }}</dd>
        </div>
        <div v-if="item.remark">
          <dt>备注</dt>
          <dd>{{ item.remark }}</dd>
        </div>
      </dl>

      <div class="requirement-assets">
        <div class="asset-section">
          <div class="asset-section-head">
            <p class="asset-section-label">本条参考图（{{ referenceItems(item).length }}）</p>
            <div class="asset-section-actions">
              <label
                v-if="canUploadRequirementFiles(item)"
                class="batch-link-btn upload-file-label"
                :class="{ 'is-disabled': isRequirementUploading(item, 'reference') }"
              >
                <input
                  type="file"
                  class="hidden-upload-input"
                  multiple
                  :disabled="isRequirementUploading(item, 'reference')"
                  :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                  aria-label="补传本条参考图"
                  @change="handleRequirementUploadInput(item, 'reference', $event)"
                />
                <span>{{ isRequirementUploading(item, 'reference') ? '上传中…' : '补传参考图' }}</span>
              </label>
              <button
                v-if="referenceItems(item).length > 0"
                type="button"
                class="batch-link-btn"
                :disabled="Boolean(batchLoadingKey)"
                @click="handleBatchRequirementReferences(item, index)"
              >
                {{
                  batchLoadingKey === batchScopeKey(item, 'references')
                    ? '打包中…'
                    : '下载全部参考图'
                }}
              </button>
            </div>
          </div>

          <div v-if="referenceItems(item).length > 0" class="reference-grid" role="list">
            <div
              v-for="file in referenceItems(item)"
              :key="file.key"
              class="reference-card"
              role="listitem"
            >
              <button
                type="button"
                class="reference-thumb-btn"
                :title="file.fileName"
                @click="openReferencePreview(item, file)"
              >
                <AssetPreviewMedia
                  :asset-id="file.assetId || null"
                  :resolved-preview-url="file.previewSrc"
                  :fallback-src="file.previewSrc"
                  :alt="file.fileName"
                  img-class="reference-thumb-media"
                  inner-img-class="reference-thumb-img"
                  :defer-until-visible="true"
                  @open-full="(url, context) => openReferencePreview(item, file, url, context)"
                />
              </button>
              <div class="reference-card-meta">
                <span class="reference-file-name" :title="file.fileName">{{ file.fileName }}</span>
                <span v-if="file.sizeText || file.mimeType" class="reference-file-sub">
                  {{ [file.mimeType, file.sizeText].filter(Boolean).join(' · ') }}
                </span>
                <button
                  type="button"
                  class="asset-download-btn"
                  :disabled="isDownloading(file.key)"
                  @click="handleDownloadReference(file)"
                >
                  {{ isDownloading(file.key) ? '下载中…' : '下载' }}
                </button>
              </div>
            </div>
          </div>
          <p v-else class="asset-empty">暂无本条参考图</p>
        </div>

        <div class="asset-section">
          <div class="asset-section-head">
            <p class="asset-section-label">本条素材文件（{{ sourceItems(item).length }}）</p>
            <div class="asset-section-actions">
              <label
                v-if="canUploadRequirementFiles(item)"
                class="batch-link-btn upload-file-label"
                :class="{ 'is-disabled': isRequirementUploading(item, 'source') }"
              >
                <input
                  type="file"
                  class="hidden-upload-input"
                  multiple
                  :disabled="isRequirementUploading(item, 'source')"
                  :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                  aria-label="补传本条素材文件"
                  @change="handleRequirementUploadInput(item, 'source', $event)"
                />
                <span>{{ isRequirementUploading(item, 'source') ? '上传中…' : '补传素材' }}</span>
              </label>
              <button
                v-if="sourceItems(item).length > 0"
                type="button"
                class="batch-link-btn"
                :disabled="Boolean(batchLoadingKey)"
                @click="handleBatchRequirementSources(item, index)"
              >
                {{
                  batchLoadingKey === batchScopeKey(item, 'sources') ? '打包中…' : '下载全部素材'
                }}
              </button>
            </div>
          </div>

          <ul v-if="sourceItems(item).length > 0" class="source-file-list">
            <li v-for="file in sourceItems(item)" :key="file.key" class="source-file-item">
              <div class="source-file-thumb">
                <AssetPreviewMedia
                  v-if="file.imagePreviewUrl"
                  :asset-id="file.previewAssetId || file.assetId || null"
                  :resolved-preview-url="file.imagePreviewUrl"
                  :fallback-src="file.imagePreviewUrl"
                  :alt="file.fileName"
                  img-class="source-thumb-media"
                  inner-img-class="source-thumb-img"
                  :defer-until-visible="true"
                  @open-full="(url, context) => openSourcePreview(item, file, url, context)"
                />
                <FileIconFallback
                  v-else
                  :name="file.fileName"
                  :url="file.downloadUrl || null"
                  :size-text="file.sizeText"
                />
              </div>
              <div class="source-file-meta">
                <span class="source-file-name" :title="file.fileName">{{ file.fileName }}</span>
                <span v-if="file.sizeText || file.mimeType" class="source-file-sub">
                  {{ [file.mimeType, file.sizeText].filter(Boolean).join(' · ') }}
                </span>
                <button
                  type="button"
                  class="asset-download-btn"
                  :disabled="isDownloading(file.key)"
                  @click="handleDownloadSource(file)"
                >
                  {{ isDownloading(file.key) ? '下载中…' : '下载' }}
                </button>
              </div>
            </li>
          </ul>
          <p v-else class="asset-empty">暂无本条素材文件</p>
        </div>
      </div>

      <p v-if="uploadStatusByRequirement.get(cacheKey(item))" class="requirement-upload-status">
        {{ uploadStatusByRequirement.get(cacheKey(item)) }}
      </p>
      <p v-if="uploadErrorByRequirement.get(cacheKey(item))" class="requirement-download-error">
        {{ uploadErrorByRequirement.get(cacheKey(item)) }}
      </p>
      <p v-if="downloadErrorByRequirement.get(cacheKey(item))" class="requirement-download-error">
        {{ downloadErrorByRequirement.get(cacheKey(item)) }}
      </p>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, inject, ref } from 'vue'
import FileIconFallback from '@/components/base/FileIconFallback.vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import {
  IMAGE_PREVIEW_LIGHTBOX_KEY,
  type ImagePreviewLightboxItem,
  type OpenImagePreviewLightbox,
} from '@/components/media/imagePreviewLightbox'
import {
  buildRetouchBatchDownloadPlan,
  countRetouchDownloadableAttachments,
  resolveRetouchBatchZipPrefix,
  runRetouchBatchDownload,
  resolveRetouchSingleAttachmentFilename,
  validateRetouchBatchDownloadPlan,
  type RetouchBatchDownloadScope,
} from '@/domain/retouch-requirement-batch-download'
import {
  retouchRequirementReferenceRefsToDisplayItems,
  retouchSourceAssetsToDisplayItems,
  type RetouchReferenceDisplayItem,
  type RetouchSourceFileDisplayItem,
} from '@/domain/retouch-requirement-assets'
import type { RetouchRequirement } from '@/domain/types/retouch-requirement'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'
import { snapshotAndResetFileInput } from '@/utils/file-input'
import { uploadReferenceFileRef, uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { formatUploadFailureMessage } from '@/utils/upload-errors'
import { UPLOAD_ACCEPT_ATTRIBUTE, isAllowedUploadFile } from '@/domain/constants/upload-types'
import {
  REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  isAcceptableReferenceFile,
  referenceFileTooLargeMessage,
} from '@/domain/constants/reference-upload'
import {
  DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES,
  designUploadTooLargeMessage,
} from '@/domain/copy/design-upload'

const props = defineProps<{
  requirements: RetouchRequirement[]
  taskTitle?: string
  taskId?: string | number | null
  canUpload?: boolean
}>()

const emit = defineEmits<{
  uploaded: []
}>()

const openLightbox = inject<OpenImagePreviewLightbox>(IMAGE_PREVIEW_LIGHTBOX_KEY, () => {})

const downloadingKeys = ref(new Set<string>())
const downloadErrorByRequirement = ref(new Map<number, string>())
const batchLoadingKey = ref<string | null>(null)
const blockBatchError = ref('')
type RequirementUploadKind = 'reference' | 'source'
const uploadingKeys = ref(new Set<string>())
const uploadStatusByRequirement = ref(new Map<number, string>())
const uploadErrorByRequirement = ref(new Map<number, string>())

const totalDownloadableCount = computed(() => countRetouchDownloadableAttachments(props.requirements))

const referenceCache = computed(() => {
  const map = new Map<number, RetouchReferenceDisplayItem[]>()
  for (const item of props.requirements) {
    const key = cacheKey(item)
    map.set(
      key,
      retouchRequirementReferenceRefsToDisplayItems(
        item.referenceFileRefs ?? [],
        `req-ref-${key}`,
      ),
    )
  }
  return map
})

const sourceCache = computed(() => {
  const map = new Map<number, RetouchSourceFileDisplayItem[]>()
  for (const item of props.requirements) {
    const key = cacheKey(item)
    map.set(key, retouchSourceAssetsToDisplayItems(item.sourceAssets ?? []))
  }
  return map
})

function cacheKey(item: RetouchRequirement): number {
  return item.id || item.sortOrder
}

function uploadKey(item: RetouchRequirement, kind: RequirementUploadKind): string {
  return `${cacheKey(item)}:${kind}`
}

function referenceItems(item: RetouchRequirement): RetouchReferenceDisplayItem[] {
  return referenceCache.value.get(cacheKey(item)) ?? []
}

function sourceItems(item: RetouchRequirement): RetouchSourceFileDisplayItem[] {
  return sourceCache.value.get(cacheKey(item)) ?? []
}

function hasOptionalFields(item: RetouchRequirement): boolean {
  return Boolean(item.skuCode?.trim() || item.spec?.trim() || item.remark?.trim())
}

function isDownloading(key: string): boolean {
  return downloadingKeys.value.has(key)
}

function setRequirementError(item: RetouchRequirement, message: string) {
  const next = new Map(downloadErrorByRequirement.value)
  if (message) next.set(cacheKey(item), message)
  else next.delete(cacheKey(item))
  downloadErrorByRequirement.value = next
}

function setRequirementUploadStatus(item: RetouchRequirement, message: string) {
  const next = new Map(uploadStatusByRequirement.value)
  if (message) next.set(cacheKey(item), message)
  else next.delete(cacheKey(item))
  uploadStatusByRequirement.value = next
}

function setRequirementUploadError(item: RetouchRequirement, message: string) {
  const next = new Map(uploadErrorByRequirement.value)
  if (message) next.set(cacheKey(item), message)
  else next.delete(cacheKey(item))
  uploadErrorByRequirement.value = next
}

function canUploadRequirementFiles(item: RetouchRequirement): boolean {
  return props.canUpload === true && Boolean(String(props.taskId ?? '').trim()) && item.id > 0
}

function isRequirementUploading(item: RetouchRequirement, kind: RequirementUploadKind): boolean {
  return uploadingKeys.value.has(uploadKey(item, kind))
}

async function handleRequirementUploadInput(
  item: RetouchRequirement,
  kind: RequirementUploadKind,
  event: Event,
) {
  const input = event.target as HTMLInputElement
  const files = snapshotAndResetFileInput(input)
  await uploadRequirementFiles(item, kind, files)
}

function validateRequirementFiles(kind: RequirementUploadKind, files: File[]) {
  const valid: File[] = []
  const errors: string[] = []
  for (const file of files) {
    if (kind === 'reference') {
      if (!isAcceptableReferenceFile(file)) {
        errors.push(`参考图无效：${file.name}`)
        continue
      }
      if (file.size > REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES) {
        errors.push(referenceFileTooLargeMessage(file.name))
        continue
      }
    } else {
      if (!isAllowedUploadFile(file.name)) {
        errors.push(`不支持的素材类型：${file.name}`)
        continue
      }
      if (file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES) {
        errors.push(designUploadTooLargeMessage(file.name))
        continue
      }
    }
    valid.push(file)
  }
  return { valid, errors }
}

async function uploadRequirementFiles(
  item: RetouchRequirement,
  kind: RequirementUploadKind,
  files: File[],
) {
  if (!files.length || !canUploadRequirementFiles(item)) return
  const taskId = String(props.taskId ?? '').trim()
  const requirementId = item.id
  const key = uploadKey(item, kind)
  const { valid, errors } = validateRequirementFiles(kind, files)
  setRequirementUploadError(item, errors.join('；'))
  setRequirementUploadStatus(item, '')
  if (!valid.length) return

  const nextUploading = new Set(uploadingKeys.value)
  nextUploading.add(key)
  uploadingKeys.value = nextUploading

  let uploaded = 0
  const failures: string[] = []
  try {
    for (const file of valid) {
      setRequirementUploadStatus(
        item,
        `正在上传${kind === 'reference' ? '参考图' : '素材'}：${file.name}`,
      )
      try {
        if (kind === 'reference') {
          await uploadReferenceFileRef(file, {
            taskId,
            retouchRequirementId: requirementId,
          })
        } else {
          await uploadTaskFileViaAssetSession(
            taskId,
            file,
            { asset_kind: 'source', remark: file.name },
            { retouchRequirementId: requirementId },
          )
        }
        uploaded += 1
      } catch (error) {
        failures.push(
          `${file.name}：${formatUploadFailureMessage(
            kind === 'reference' ? 'reference_upload' : 'main_complete',
            error,
          )}`,
        )
      }
    }
  } finally {
    const doneUploading = new Set(uploadingKeys.value)
    doneUploading.delete(key)
    uploadingKeys.value = doneUploading
  }

  if (uploaded > 0) {
    setRequirementUploadStatus(
      item,
      `已补传 ${uploaded} 个${kind === 'reference' ? '参考图' : '素材文件'}`,
    )
    emit('uploaded')
  } else {
    setRequirementUploadStatus(item, '')
  }
  if (failures.length) {
    setRequirementUploadError(item, [...errors, ...failures].join('；'))
  } else if (!errors.length) {
    setRequirementUploadError(item, '')
  }
}

function findRequirementForKey(fileKey: string): RetouchRequirement | undefined {
  return props.requirements.find(
    (item) =>
      referenceItems(item).some((row) => row.key === fileKey) ||
      sourceItems(item).some((row) => row.key === fileKey),
  )
}

async function runDownload(
  fileKey: string,
  item: RetouchRequirement | undefined,
  options: { assetId?: string; downloadUrl?: string; preferredFilename: string },
) {
  if (downloadingKeys.value.has(fileKey)) return
  const next = new Set(downloadingKeys.value)
  next.add(fileKey)
  downloadingKeys.value = next
  if (item) setRequirementError(item, '')

  const result = await downloadAssetFileWithOriginalFilename({
    assetId: options.assetId,
    downloadUrl: options.downloadUrl,
    preferredFilename: options.preferredFilename,
  })

  const done = new Set(downloadingKeys.value)
  done.delete(fileKey)
  downloadingKeys.value = done

  if (!result.ok && item) {
    setRequirementError(item, result.message ?? '下载失败，请稍后重试')
  }
}

function handleDownloadReference(file: RetouchReferenceDisplayItem) {
  const item = findRequirementForKey(file.key)
  const index = item ? requirementIndex(item) : 0
  void runDownload(file.key, item, {
    assetId: file.assetId,
    downloadUrl: file.downloadUrl,
    preferredFilename: resolveRetouchSingleAttachmentFilename(item, index, file.fileName),
  })
}

function handleDownloadSource(file: RetouchSourceFileDisplayItem) {
  const item = findRequirementForKey(file.key)
  const index = item ? requirementIndex(item) : 0
  void runDownload(file.key, item, {
    assetId: file.assetId,
    downloadUrl: file.downloadUrl,
    preferredFilename: resolveRetouchSingleAttachmentFilename(item, index, file.fileName, file.hasOriginalFilename),
  })
}

function referencePreviewGallery(item: RetouchRequirement): ImagePreviewLightboxItem[] {
  return referenceItems(item)
    .map((file, index) => {
      const src = file.previewSrc?.trim()
      if (!src) return null
      const title = file.fileName || `参考图 ${index + 1}`
      return {
        src,
        previewAssetId: file.assetId,
        resolvedPreviewUrl: src,
        fallbackSrc: src,
        title,
        alt: title,
        preferredFilename: title,
        downloadUrl: file.downloadUrl || src,
      }
    })
    .filter((row) => row != null) as ImagePreviewLightboxItem[]
}

function openReferencePreview(
  item: RetouchRequirement,
  file: RetouchReferenceDisplayItem,
  url?: string,
  context?: {
    assetId?: string
    fallbackAssetId?: string
    fallbackSrc?: string
    resolvedPreviewUrl?: string
  },
) {
  const activeUrl = String(url || file.previewSrc || '').trim()
  if (!activeUrl && !file.assetId) return
  const gallery = referencePreviewGallery(item)
  const index = Math.max(0, gallery.findIndex((row) => row.src === file.previewSrc || row.previewAssetId === file.assetId))
  if (gallery[index]) {
    gallery[index] = {
      ...gallery[index],
      src: activeUrl || gallery[index].src,
      previewAssetId: context?.assetId || gallery[index].previewAssetId,
      fallbackAssetId: context?.fallbackAssetId || gallery[index].fallbackAssetId,
      fallbackSrc: context?.fallbackSrc || gallery[index].fallbackSrc,
      resolvedPreviewUrl: context?.resolvedPreviewUrl || gallery[index].resolvedPreviewUrl,
    }
  }
  openLightbox(activeUrl, {
    title: file.fileName || `需求 ${requirementIndex(item) + 1} 参考图`,
    items: gallery,
    index,
  })
}

function sourcePreviewGallery(item: RetouchRequirement): ImagePreviewLightboxItem[] {
  return sourceItems(item)
    .map((file, index) => {
      const src = file.imagePreviewUrl?.trim() || ''
      const previewAssetId = file.previewAssetId || file.assetId
      if (!src && !previewAssetId) return null
      const title = file.fileName || `素材 ${index + 1}`
      return {
        src,
        previewAssetId,
        resolvedPreviewUrl: src || undefined,
        fallbackSrc: src || undefined,
        title,
        alt: title,
        preferredFilename: title,
        downloadUrl: file.downloadUrl || src,
      }
    })
    .filter((row) => row != null) as ImagePreviewLightboxItem[]
}

function openSourcePreview(
  item: RetouchRequirement,
  file: RetouchSourceFileDisplayItem,
  url: string,
  context?: {
    assetId?: string
    fallbackAssetId?: string
    fallbackSrc?: string
    resolvedPreviewUrl?: string
  },
) {
  const activeUrl = url.trim()
  if (!activeUrl && !file.previewAssetId && !file.assetId) return
  const gallery = sourcePreviewGallery(item)
  const index = Math.max(0, gallery.findIndex((row) => row.previewAssetId === (file.previewAssetId || file.assetId) || row.src === file.imagePreviewUrl))
  if (gallery[index]) {
    gallery[index] = {
      ...gallery[index],
      src: activeUrl || gallery[index].src,
      previewAssetId: context?.assetId || gallery[index].previewAssetId,
      fallbackAssetId: context?.fallbackAssetId || gallery[index].fallbackAssetId,
      fallbackSrc: context?.fallbackSrc || gallery[index].fallbackSrc,
      resolvedPreviewUrl: context?.resolvedPreviewUrl || gallery[index].resolvedPreviewUrl,
    }
  }
  openLightbox(activeUrl, {
    title: file.fileName || '素材图',
    items: gallery,
    index,
  })
}

function requirementDownloadableCount(item: RetouchRequirement): number {
  return buildRetouchBatchDownloadPlan(props.requirements, 'requirement_all', requirementIndex(item))
    .entries.length
}

function requirementIndex(item: RetouchRequirement): number {
  const key = cacheKey(item)
  const index = props.requirements.findIndex((row) => cacheKey(row) === key)
  return index >= 0 ? index : 0
}

function batchScopeKey(
  item: RetouchRequirement,
  kind: 'all' | 'references' | 'sources',
): string {
  return `req-${cacheKey(item)}-${kind}`
}

function setBlockBatchError(message: string) {
  blockBatchError.value = message
}

async function runBatchDownload(
  loadingKey: string,
  scope: RetouchBatchDownloadScope,
  requirementIndexArg: number | undefined,
  zipPrefix: string,
  errorTarget: RetouchRequirement | 'block',
) {
  if (batchLoadingKey.value) return
  batchLoadingKey.value = loadingKey
  if (errorTarget === 'block') setBlockBatchError('')
  else setRequirementError(errorTarget, '')

  const plan = buildRetouchBatchDownloadPlan(props.requirements, scope, requirementIndexArg)
  const validation = validateRetouchBatchDownloadPlan(plan)
  if (!validation.ok) {
    const msg = validation.message ?? '无法批量下载'
    if (errorTarget === 'block') setBlockBatchError(msg)
    else if (errorTarget) setRequirementError(errorTarget, msg)
    batchLoadingKey.value = null
    return
  }

  const result = await runRetouchBatchDownload(plan, zipPrefix)
  batchLoadingKey.value = null

  if (!result.ok) {
    const msg = result.message ?? '批量下载失败，请稍后重试'
    if (errorTarget === 'block') setBlockBatchError(msg)
    else if (errorTarget) setRequirementError(errorTarget, msg)
    return
  }
  if (result.message) {
    if (errorTarget === 'block') setBlockBatchError(result.message)
    else if (errorTarget) setRequirementError(errorTarget, result.message)
  }
}

function handleBatchAllAttachments() {
  void runBatchDownload(
    'block-all',
    'all_attachments',
    undefined,
    resolveRetouchBatchZipPrefix(props.requirements, 'all_attachments', undefined, props.taskTitle),
    'block',
  )
}

function handleBatchRequirementAll(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'all'),
    'requirement_all',
    index,
    resolveRetouchBatchZipPrefix(props.requirements, 'requirement_all', index, props.taskTitle),
    item,
  )
}

function handleBatchRequirementReferences(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'references'),
    'requirement_references',
    index,
    resolveRetouchBatchZipPrefix(props.requirements, 'requirement_references', index, props.taskTitle),
    item,
  )
}

function handleBatchRequirementSources(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'sources'),
    'requirement_sources',
    index,
    resolveRetouchBatchZipPrefix(props.requirements, 'requirement_sources', index, props.taskTitle),
    item,
  )
}
</script>

<style scoped>
.retouch-requirements-block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.block-head-text {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
}

.block-batch-error {
  margin: 0;
  font-size: 12px;
  color: rgb(var(--yb-danger-text));
}

.block-kicker {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.block-summary {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.requirement-card {
  padding: 14px 16px;
  border: 1px solid var(--border-color, rgb(var(--yb-border-slate)));
  border-left: 3px solid rgb(var(--yb-brand-bright));
  border-radius: 10px;
  background: rgb(var(--yb-surface-subtle));
}

.requirement-card-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.requirement-card-head-main {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.batch-link-btn {
  flex-shrink: 0;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  color: rgb(var(--yb-brand));
  background: transparent;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 6px;
  cursor: pointer;
}

.batch-link-btn:hover:not(:disabled) {
  background: rgb(var(--yb-brand-soft));
  border-color: rgb(var(--yb-brand-border-strong));
}

.batch-link-btn:disabled,
.batch-link-btn.is-disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.upload-file-label {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 24px;
}

.requirement-no {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
}

.requirement-counts {
  font-size: 12px;
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.requirement-desc {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
  white-space: pre-wrap;
}

.requirement-meta {
  margin: 10px 0 0;
  display: grid;
  gap: 6px;
}

.requirement-meta div {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 8px;
  font-size: 13px;
}

.requirement-meta dt {
  margin: 0;
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.requirement-meta dd {
  margin: 0;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
}

.requirement-assets {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 12px;
  border-top: 1px dashed var(--border-color, rgb(var(--yb-border-slate)));
}

.asset-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.asset-section-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.asset-section-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.hidden-upload-input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.asset-section-label {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
}

.asset-empty {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary, rgb(var(--yb-text-placeholder)));
}

.reference-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 10px;
}

.reference-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border: 1px solid var(--border-color, rgb(var(--yb-border-subtle)));
  border-radius: 8px;
  background: rgb(var(--yb-surface));
}

.reference-thumb-btn {
  padding: 0;
  border: none;
  border-radius: 6px;
  overflow: hidden;
  cursor: pointer;
  background: rgb(var(--yb-surface-slate));
  aspect-ratio: 1;
}

.reference-thumb-media,
.reference-thumb-btn :deep(.reference-thumb-media),
.reference-thumb-btn :deep(.apm),
.reference-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.reference-thumb-btn :deep(.apm-placeholder),
.reference-thumb-btn :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.25rem;
}

.reference-card-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.reference-file-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
  word-break: break-all;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.reference-file-sub {
  font-size: 11px;
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.source-file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.source-file-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border-color, rgb(var(--yb-border-subtle)));
  border-radius: 8px;
  background: rgb(var(--yb-surface));
}

.source-file-thumb {
  flex: 0 0 72px;
  width: 72px;
  height: 72px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--border-color, rgb(var(--yb-border-subtle)));
  background: rgb(var(--yb-surface-subtle));
}

.source-thumb-media,
.source-file-thumb :deep(.source-thumb-media),
.source-file-thumb :deep(.apm),
.source-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.source-file-thumb :deep(.apm-placeholder),
.source-file-thumb :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.2rem;
}

.source-file-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
}

.source-file-name {
  font-weight: 500;
  color: var(--text-primary, rgb(var(--yb-text-navy)));
  word-break: break-all;
}

.source-file-sub {
  color: var(--text-secondary, rgb(var(--yb-text-muted-strong)));
}

.asset-download-btn {
  align-self: flex-start;
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand));
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.asset-download-btn:hover:not(:disabled) {
  background: rgb(var(--yb-brand-strong));
}

.asset-download-btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.requirement-upload-status {
  margin: 10px 0 0;
  font-size: 12px;
  color: rgb(var(--yb-brand));
}

.requirement-download-error {
  margin: 10px 0 0;
  font-size: 12px;
  color: rgb(var(--yb-danger-text));
}
</style>
