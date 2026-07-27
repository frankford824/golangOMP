<template>
  <section class="resource-matrix" aria-labelledby="resource-matrix-title">
    <header class="matrix-head">
      <div>
        <p class="eyebrow">资源交付链</p>
        <h2 id="resource-matrix-title">当前资源组成</h2>
        <p>从运营参考到最终成品，只展示当前业务有效版本。</p>
      </div>
      <span class="matrix-summary">{{ bundle.groups.length }} 个资源单元</span>
    </header>

    <div v-if="!bundle.groups.length" class="empty">尚未建立资源组。</div>
    <article v-for="group in bundle.groups" :key="group.id" class="group-card">
      <header class="group-title">
        <div>
          <span class="sku-label">{{ group.sku_code || scopeLabel(group) }}</span>
          <strong>{{ group.product_name || '未命名产品' }}</strong>
        </div>
        <div class="group-badges">
          <span>{{ revision(group)?.mode === 'set' ? `套装 · ${orderedItems(group).length} 张` : '单图' }}</span>
          <span v-if="group.migration_incomplete" class="migration-warning">资源待人工确认</span>
          <button v-if="props.enableRevisionHistory" type="button" class="revision-history-button" @click="historyGroup = group">历史修订</button>
        </div>
      </header>

      <div class="stage-rail" aria-label="资源三个阶段">
        <section class="stage-card reference-stage">
          <header><span class="stage-number">01</span><div><h3>运营参考图</h3><p>设计需求与方向依据</p></div></header>
          <div v-if="references(group).length" class="reference-grid">
            <template v-for="(reference, index) in references(group)" :key="reference.id || index">
              <button v-if="imagePreviewable(reference)" type="button" class="visual-tile" :disabled="!reference.preview_url || previewFailed(previewKey(group.id, 'reference', reference.id || index))" @click="openPreview(reference.preview_url, reference.file_name || `参考图 ${index + 1}`)">
                <img v-if="reference.preview_url && !previewFailed(previewKey(group.id, 'reference', reference.id || index))" :src="reference.preview_url" :alt="reference.file_name || `参考图 ${index + 1}`" loading="lazy" @error="markPreviewFailed(previewKey(group.id, 'reference', reference.id || index))" />
                <span v-else class="tile-fallback">{{ fileType(reference.file_name) }}</span>
                <span class="tile-caption">{{ reference.file_name || `参考图 ${index + 1}` }}</span>
              </button>
              <a v-else-if="resourceDownloadURL(reference)" class="visual-tile file-download-tile" :href="resourceDownloadURL(reference)" download @click.stop>
                <span class="tile-fallback">{{ fileType(reference.file_name) }}</span>
                <span class="tile-caption">{{ reference.file_name || `参考文件 ${index + 1}` }} · 下载</span>
              </a>
              <a v-else-if="reference.preview_url" class="visual-tile file-preview-link" :href="reference.preview_url" target="_blank" rel="noopener" @click.stop>
                <span class="tile-fallback">{{ fileType(reference.file_name) }}</span>
                <span class="tile-caption">{{ reference.file_name || `参考文件 ${index + 1}` }} · 打开</span>
              </a>
              <div v-else class="visual-tile is-disabled">
                <span class="tile-fallback">{{ fileType(reference.file_name) }}</span>
                <span class="tile-caption">{{ reference.file_name || `参考文件 ${index + 1}` }} · 暂不可用</span>
              </div>
            </template>
          </div>
          <p v-else class="stage-empty">未提供参考图</p>
        </section>

        <span class="stage-arrow" aria-hidden="true">→</span>

        <section class="stage-card source-stage">
          <header><span class="stage-number">02</span><div><h3>当前有效源文件</h3><p>{{ sourceStageLabel(group) }}</p></div></header>
          <article v-if="revision(group)?.source_file" class="source-file-card">
            <span class="source-icon">{{ fileType(revision(group)?.source_file?.file_name) }}</span>
            <div><strong>{{ revision(group)?.source_file?.file_name }}</strong><span>{{ formatBytes(revision(group)?.source_file?.file_size) }}</span></div>
            <a v-if="revision(group)?.source_file?.download_url" :href="revision(group)?.source_file?.download_url" download @click.stop>下载</a>
          </article>
          <p v-else class="stage-empty">当前没有可用源文件</p>
        </section>

        <span class="stage-arrow" aria-hidden="true">→</span>

        <section class="stage-card final-stage">
          <header><span class="stage-number">03</span><div><h3>最终成品图</h3><p>{{ revision(group)?.mode === 'set' ? '整套按审核顺序交付' : '审核确认的单张成品' }}</p></div></header>
          <div v-if="orderedItems(group).length" class="final-gallery">
            <template v-for="(item, index) in orderedItems(group)" :key="item.id">
              <button v-if="imagePreviewable(item.file)" type="button" class="final-tile" :disabled="!item.file?.preview_url || previewFailed(previewKey(group.id, 'final', item.id))" @click="openPreview(item.file?.preview_url, item.file?.file_name || item.item_name || `成品 ${index + 1}`)">
                <img v-if="item.file?.preview_url && !previewFailed(previewKey(group.id, 'final', item.id))" :src="item.file.preview_url" :alt="item.file.file_name" loading="lazy" @error="markPreviewFailed(previewKey(group.id, 'final', item.id))" />
                <span v-else class="tile-fallback">{{ fileType(item.file?.file_name || item.item_name) }}</span>
                <span class="order-badge">{{ index + 1 }}</span>
                <span v-if="index === 0 && revision(group)?.mode === 'set'" class="cover-badge">封面</span>
                <span class="tile-caption">{{ item.file?.file_name || item.item_name || `成品 ${index + 1}` }}</span>
              </button>
              <a v-else-if="resourceDownloadURL(item.file)" class="final-tile file-download-tile" :href="resourceDownloadURL(item.file)" download @click.stop>
                <span class="tile-fallback">{{ fileType(item.file?.file_name || item.item_name) }}</span>
                <span class="order-badge">{{ index + 1 }}</span>
                <span class="tile-caption">{{ item.file?.file_name || item.item_name || `成品 ${index + 1}` }} · 下载</span>
              </a>
              <a v-else-if="item.file?.preview_url" class="final-tile file-preview-link" :href="item.file.preview_url" target="_blank" rel="noopener" @click.stop>
                <span class="tile-fallback">{{ fileType(item.file.file_name || item.item_name) }}</span>
                <span class="order-badge">{{ index + 1 }}</span>
                <span class="tile-caption">{{ item.file.file_name || item.item_name || `成品 ${index + 1}` }} · 打开</span>
              </a>
              <div v-else class="final-tile is-disabled">
                <span class="tile-fallback">{{ fileType(item.file?.file_name || item.item_name) }}</span>
                <span class="order-badge">{{ index + 1 }}</span>
                <span class="tile-caption">{{ item.file?.file_name || item.item_name || `成品 ${index + 1}` }} · 暂不可用</span>
              </div>
            </template>
          </div>
          <p v-else class="stage-empty">尚无最终成品</p>
        </section>
      </div>

      <footer class="group-footer">
        <span>来源任务 · {{ group.task_no || group.task_id }}</span>
        <span v-if="group.creator_name">创建人 · {{ group.creator_name }}</span>
      </footer>
    </article>

    <ResourceRevisionDrawer v-if="historyGroup" :group="historyGroup" @close="historyGroup = null" />

    <Teleport to="body">
      <div v-if="preview.url" class="preview-layer" :style="{ zIndex: RESOURCE_OVERLAY_Z_INDEX }" role="dialog" aria-modal="true" :aria-label="preview.name" @click.self="closePreview" @keydown.esc="closePreview">
        <button class="preview-close" aria-label="关闭预览" @click="closePreview">×</button>
        <figure><img :src="preview.url" :alt="preview.name" /><figcaption>{{ preview.name }}</figcaption></figure>
      </div>
    </Teleport>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import ResourceRevisionDrawer from '@/components/task/ResourceRevisionDrawer.vue'
import type { ResourceBundle, ResourceFile, ResourceGroup, ResourceReference, ResourceRevision } from '@/services/api/resourceGroupsApi'

const RESOURCE_OVERLAY_Z_INDEX = 7500
const props = defineProps<{ bundle: ResourceBundle; enableRevisionHistory?: boolean }>()

const preview = reactive({ url: '', name: '' })
const failedPreviews = ref(new Set<string>())
const historyGroup = ref<ResourceGroup | null>(null)
const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const orderedItems = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const references = (group: ResourceGroup): ResourceReference[] => [...(revision(group)?.references || [])].sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0))
const scopeLabel = (group: ResourceGroup) => group.scope_kind === 'retouch_requirement' ? '修图需求' : '任务资源'
const sourceStageLabels: Record<string, string> = { audit: '审核人员上传的替换源文件', design: '设计人员提交的源文件', retouch: '修图源文件', migration: '历史任务确认后的源文件', reopen: '重开后提交的源文件' }
const sourceStageLabel = (group: ResourceGroup) => sourceStageLabels[revision(group)?.source_stage || ''] || '当前审核确认版本'
const fileType = (name?: string) => (name?.split('.').pop() || 'FILE').slice(0, 5).toUpperCase()
function imagePreviewable(file?: Pick<ResourceFile, 'file_name' | 'mime_type'> | Pick<ResourceReference, 'file_name' | 'mime_type'> | null) {
  return String(file?.mime_type || '').startsWith('image/')
    || /\.(png|jpe?g|webp|gif)$/i.test(String(file?.file_name || ''))
}
function resourceDownloadURL(file?: Pick<ResourceFile, 'download_url'> | Pick<ResourceReference, 'download_url'> | null) {
  return String(file?.download_url || '')
}
function formatBytes(value?: number | null) { if (!value) return '文件大小未知'; if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB` }
function previewKey(groupID: number, role: 'reference' | 'final', id: number) { return `${groupID}:${role}:${id}` }
function previewFailed(key: string) { return failedPreviews.value.has(key) }
function markPreviewFailed(key: string) { failedPreviews.value = new Set(failedPreviews.value).add(key) }
function openPreview(url?: string, name?: string) { if (!url) return; preview.url = url; preview.name = name || '资源预览' }
function closePreview() { preview.url = ''; preview.name = '' }
</script>

<style scoped>
.resource-matrix{display:grid;gap:18px}.matrix-head,.group-title,.group-footer{display:flex;align-items:center;justify-content:space-between;gap:16px}.eyebrow{margin:0;color:rgb(var(--yb-brand));font-size:11px;font-weight:900;letter-spacing:.13em}.matrix-head h2{margin:3px 0;font-size:22px}.matrix-head p{margin:0;color:rgb(var(--yb-text-muted))}.matrix-summary,.group-badges span{padding:6px 9px;border-radius:999px;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-muted));font-size:12px}.group-card{overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface))}.group-title{padding:16px 18px;border-bottom:1px solid rgb(var(--yb-border))}.group-title>div:first-child{display:grid;gap:4px}.sku-label{color:rgb(var(--yb-brand));font-size:12px;font-weight:850}.group-badges{display:flex;gap:7px;flex-wrap:wrap;align-items:center}.group-badges .migration-warning{background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text))}.revision-history-button{min-height:30px;padding:0 9px;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));font-size:11px;font-weight:760;cursor:pointer}.stage-rail{display:grid;grid-template-columns:minmax(0,.9fr) auto minmax(0,.75fr) auto minmax(0,1.25fr);gap:12px;align-items:stretch;padding:18px}.stage-card{min-width:0;display:grid;align-content:start;gap:14px;padding:14px;border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface-soft))}.stage-card>header{display:flex;gap:10px}.stage-card h3{margin:0;font-size:14px}.stage-card header p{margin:3px 0 0;color:rgb(var(--yb-text-muted));font-size:11px}.stage-number{color:rgb(var(--yb-brand));font-size:11px;font-weight:900}.stage-arrow{align-self:center;color:rgb(var(--yb-brand));font-size:19px}.reference-grid,.final-gallery{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.final-gallery{grid-template-columns:repeat(auto-fill,minmax(88px,1fr))}.visual-tile,.final-tile{position:relative;min-width:0;display:grid;gap:6px;padding:0;border:0;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer;text-decoration:none}.visual-tile img,.final-tile img,.tile-fallback{width:100%;aspect-ratio:4/3;object-fit:cover;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface-muted))}.tile-fallback{display:grid;place-items:center;color:rgb(var(--yb-text-muted));font-weight:900}.file-download-tile .tile-fallback{border-style:dashed;color:rgb(var(--yb-brand));background:rgb(var(--yb-brand-soft))}.is-disabled{cursor:not-allowed;opacity:.55}.tile-caption{overflow:hidden;color:rgb(var(--yb-text-muted));font-size:10px;text-overflow:ellipsis;white-space:nowrap}.order-badge,.cover-badge{position:absolute;top:6px;padding:3px 6px;border-radius:999px;background:rgb(var(--yb-surface)/.94);font-size:10px;font-weight:850}.order-badge{left:6px}.cover-badge{right:6px;color:rgb(var(--yb-brand))}.source-file-card{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:10px;align-items:center;padding:12px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface))}.source-icon{display:grid;place-items:center;width:42px;height:42px;border-radius:10px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-size:11px;font-weight:900}.source-file-card div{min-width:0;display:grid;gap:4px}.source-file-card strong{overflow:hidden;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.source-file-card span{color:rgb(var(--yb-text-muted));font-size:10px}.source-file-card a{color:rgb(var(--yb-brand));font-size:12px;text-decoration:none}.stage-empty{display:grid;place-items:center;min-height:100px;margin:0;border:1px dashed rgb(var(--yb-border));border-radius:11px;color:rgb(var(--yb-text-muted));font-size:12px}.group-footer{padding:12px 18px;border-top:1px solid rgb(var(--yb-border));color:rgb(var(--yb-text-muted));font-size:11px}.empty{padding:36px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:16px;color:rgb(var(--yb-text-muted))}.preview-layer{position:fixed;inset:0;z-index:7500;display:grid;place-items:center;padding:30px;background:rgb(var(--yb-overlay-night)/.78)}.preview-layer figure{max-width:min(1000px,92vw);max-height:88vh;display:grid;gap:10px;margin:0}.preview-layer img{max-width:100%;max-height:80vh;border-radius:14px;object-fit:contain}.preview-layer figcaption{color:rgb(var(--yb-text-inverse));text-align:center}.preview-close{position:fixed;top:22px;right:22px;width:42px;height:42px;border:1px solid rgb(var(--yb-text-inverse)/.3);border-radius:999px;background:rgb(var(--yb-overlay-night)/.4);color:rgb(var(--yb-text-inverse));font-size:25px;cursor:pointer}@media(max-width:980px){.stage-rail{grid-template-columns:1fr}.stage-arrow{transform:rotate(90deg);justify-self:center}.final-gallery{grid-template-columns:repeat(4,minmax(0,1fr))}}@media(max-width:620px){.matrix-head,.group-title,.group-footer{align-items:flex-start;flex-direction:column}.stage-rail{padding:11px}.reference-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.final-gallery{grid-template-columns:repeat(3,minmax(0,1fr))}}
</style>
