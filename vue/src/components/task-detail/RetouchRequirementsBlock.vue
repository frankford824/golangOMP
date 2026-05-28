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
                @click="openReferencePreview(file)"
              >
                <img
                  :src="file.previewSrc"
                  :alt="file.fileName"
                  class="reference-thumb-img"
                  loading="lazy"
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

          <ul v-if="sourceItems(item).length > 0" class="source-file-list">
            <li v-for="file in sourceItems(item)" :key="file.key" class="source-file-item">
              <div class="source-file-thumb">
                <img
                  v-if="file.imagePreviewUrl"
                  :src="file.imagePreviewUrl"
                  :alt="file.fileName"
                  class="source-thumb-img"
                  loading="lazy"
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

      <p v-if="downloadErrorByRequirement.get(cacheKey(item))" class="requirement-download-error">
        {{ downloadErrorByRequirement.get(cacheKey(item)) }}
      </p>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, inject, ref } from 'vue'
import FileIconFallback from '@/components/base/FileIconFallback.vue'
import {
  buildRetouchBatchDownloadPlan,
  countRetouchDownloadableAttachments,
  runRetouchBatchDownload,
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

const OPEN_LIGHTBOX_KEY = 'task-detail-open-lightbox'

const props = defineProps<{
  requirements: RetouchRequirement[]
}>()

const openLightbox = inject<(src: string) => void>(OPEN_LIGHTBOX_KEY, () => {})

const downloadingKeys = ref(new Set<string>())
const downloadErrorByRequirement = ref(new Map<number, string>())
const batchLoadingKey = ref<string | null>(null)
const blockBatchError = ref('')

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
  void runDownload(file.key, item, {
    assetId: file.assetId,
    downloadUrl: file.downloadUrl,
    preferredFilename: file.fileName,
  })
}

function handleDownloadSource(file: RetouchSourceFileDisplayItem) {
  const item = findRequirementForKey(file.key)
  void runDownload(file.key, item, {
    assetId: file.assetId,
    downloadUrl: file.downloadUrl,
    preferredFilename: file.fileName,
  })
}

function openReferencePreview(file: RetouchReferenceDisplayItem) {
  if (file.previewSrc) openLightbox(file.previewSrc)
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
  void runBatchDownload('block-all', 'all_attachments', undefined, 'retouch-requirements-all', 'block')
}

function handleBatchRequirementAll(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'all'),
    'requirement_all',
    index,
    `retouch-requirement-${index + 1}-all`,
    item,
  )
}

function handleBatchRequirementReferences(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'references'),
    'requirement_references',
    index,
    `retouch-requirement-${index + 1}-references`,
    item,
  )
}

function handleBatchRequirementSources(item: RetouchRequirement, index: number) {
  void runBatchDownload(
    batchScopeKey(item, 'sources'),
    'requirement_sources',
    index,
    `retouch-requirement-${index + 1}-sources`,
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
  color: #b91c1c;
}

.block-kicker {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-secondary, #64748b);
}

.block-summary {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.requirement-card {
  padding: 14px 16px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-left: 3px solid #3b82f6;
  border-radius: 10px;
  background: #f8fafc;
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
  color: #2563eb;
  background: transparent;
  border: 1px solid #bfdbfe;
  border-radius: 6px;
  cursor: pointer;
}

.batch-link-btn:hover:not(:disabled) {
  background: #eff6ff;
  border-color: #93c5fd;
}

.batch-link-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.requirement-no {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.requirement-counts {
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.requirement-desc {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-primary, #0f172a);
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
  color: var(--text-secondary, #64748b);
}

.requirement-meta dd {
  margin: 0;
  color: var(--text-primary, #0f172a);
}

.requirement-assets {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 12px;
  border-top: 1px dashed var(--border-color, #e2e8f0);
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

.asset-section-label {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.asset-empty {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
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
  border: 1px solid var(--border-color, #dbe3ef);
  border-radius: 8px;
  background: #fff;
}

.reference-thumb-btn {
  padding: 0;
  border: none;
  border-radius: 6px;
  overflow: hidden;
  cursor: pointer;
  background: #f1f5f9;
  aspect-ratio: 1;
}

.reference-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
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
  color: var(--text-primary, #0f172a);
  word-break: break-all;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.reference-file-sub {
  font-size: 11px;
  color: var(--text-secondary, #64748b);
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
  border: 1px solid var(--border-color, #dbe3ef);
  border-radius: 8px;
  background: #fff;
}

.source-file-thumb {
  flex: 0 0 72px;
  width: 72px;
  height: 72px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--border-color, #dbe3ef);
  background: #f8fafc;
}

.source-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
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
  color: var(--text-primary, #0f172a);
  word-break: break-all;
}

.source-file-sub {
  color: var(--text-secondary, #64748b);
}

.asset-download-btn {
  align-self: flex-start;
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  background: #2563eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.asset-download-btn:hover:not(:disabled) {
  background: #1d4ed8;
}

.asset-download-btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.requirement-download-error {
  margin: 10px 0 0;
  font-size: 12px;
  color: #b91c1c;
}
</style>
