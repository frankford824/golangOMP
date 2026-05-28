<template>
  <section class="retouch-requirements-block">
    <p class="block-kicker">P 图需求明细</p>
    <article v-for="(item, index) in requirements" :key="item.id || index" class="requirement-card">
      <header class="requirement-card-head">
        <span class="requirement-no">需求 {{ index + 1 }}</span>
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
          <p class="asset-section-label">本条参考图</p>
          <AssetThumbStrip
            v-if="referenceThumbItems(item).length > 0"
            :items="referenceThumbItems(item)"
            empty-text="暂无本条参考图"
            size="sm"
          />
          <p v-else class="asset-empty">暂无本条参考图</p>
        </div>

        <div class="asset-section">
          <p class="asset-section-label">本条素材文件</p>
          <ul v-if="sourceFileItems(item).length > 0" class="source-file-list">
            <li v-for="file in sourceFileItems(item)" :key="file.key" class="source-file-item">
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
                <a
                  v-if="file.downloadUrl"
                  class="source-download-link"
                  :href="file.downloadUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  下载
                </a>
              </div>
            </li>
          </ul>
          <p v-else class="asset-empty">暂无本条素材文件</p>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import FileIconFallback from '@/components/base/FileIconFallback.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import {
  retouchRequirementReferenceRefsToThumbItems,
  retouchSourceAssetsToDisplayItems,
  type RetouchSourceFileDisplayItem,
} from '@/domain/retouch-requirement-assets'
import type { RetouchRequirement } from '@/domain/types/retouch-requirement'

const props = defineProps<{
  requirements: RetouchRequirement[]
}>()

const thumbCache = computed(() => {
  const map = new Map<number, AssetThumbItem[]>()
  for (const item of props.requirements) {
    const key = item.id || item.sortOrder
    map.set(
      key,
      retouchRequirementReferenceRefsToThumbItems(item.referenceFileRefs ?? [], `req-ref-${key}`),
    )
  }
  return map
})

const sourceCache = computed(() => {
  const map = new Map<number, RetouchSourceFileDisplayItem[]>()
  for (const item of props.requirements) {
    const key = item.id || item.sortOrder
    map.set(key, retouchSourceAssetsToDisplayItems(item.sourceAssets ?? []))
  }
  return map
})

function cacheKey(item: RetouchRequirement): number {
  return item.id || item.sortOrder
}

function referenceThumbItems(item: RetouchRequirement): AssetThumbItem[] {
  return thumbCache.value.get(cacheKey(item)) ?? []
}

function sourceFileItems(item: RetouchRequirement): RetouchSourceFileDisplayItem[] {
  return sourceCache.value.get(cacheKey(item)) ?? []
}

function hasOptionalFields(item: RetouchRequirement): boolean {
  return Boolean(item.skuCode?.trim() || item.spec?.trim() || item.remark?.trim())
}
</script>

<style scoped>
.retouch-requirements-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.block-kicker {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-secondary, #64748b);
}

.requirement-card {
  padding: 12px 14px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  background: #f8fafc;
}

.requirement-card-head {
  margin-bottom: 6px;
}

.requirement-no {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
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
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px dashed var(--border-color, #e2e8f0);
}

.asset-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.asset-section-label {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.asset-empty {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
}

.source-file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.source-file-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.source-file-thumb {
  flex: 0 0 72px;
  width: 72px;
  height: 72px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--border-color, #dbe3ef);
  background: #fff;
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
  gap: 4px;
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

.source-download-link {
  color: #2563eb;
  text-decoration: none;
  width: fit-content;
}

.source-download-link:hover {
  text-decoration: underline;
}
</style>
