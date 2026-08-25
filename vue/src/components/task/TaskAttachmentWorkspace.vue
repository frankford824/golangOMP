<template>
  <section class="attachment-workspace" aria-label="参考附件">
    <aside class="attachment-list">
      <header>
        <div><p>参考附件</p><strong>{{ files.length }} 个文件</strong></div>
        <button v-if="canUpload" type="button" :disabled="uploading" @click="$emit('upload')">
          <Plus :size="15" aria-hidden="true" />{{ uploading ? '上传中…' : '补充附件' }}
        </button>
      </header>
      <div v-if="files.length" class="attachment-items">
        <button
          v-for="(file, index) in files"
          :key="fileKey(file, index)"
          type="button"
          :class="{ selected: selectedIndex === index }"
          @click="selectedIndex = index"
        >
          <span class="attachment-thumb">
            <img v-if="previewable(file) && previewUrl(file) && !broken.has(fileKey(file, index))" :src="previewUrl(file)" :alt="fileName(file)" @error="markBroken(file, index)" />
            <FileText v-else :size="22" aria-hidden="true" />
          </span>
          <span><strong>{{ fileName(file) }}</strong><small>{{ fileType(file) }}{{ fileUrl(file) ? '' : ' · 不可下载' }}</small></span>
        </button>
      </div>
      <p v-else class="empty-copy">暂无参考附件。运营人员补充后会显示在这里。</p>
    </aside>

    <article class="attachment-preview">
      <template v-if="selectedFile">
        <header>
          <div><p>当前文件</p><h3>{{ fileName(selectedFile) }}</h3></div>
          <div class="attachment-actions">
            <button v-if="canReplaceSelected" type="button" :disabled="replacing" @click="$emit('replace', selectedFile)">
              <RefreshCw :size="16" aria-hidden="true" />{{ replacing ? '替换中…' : '替换当前参考图' }}
            </button>
            <a v-if="fileUrl(selectedFile)" :href="fileUrl(selectedFile)" target="_blank" rel="noreferrer" download>
              <Download :size="16" aria-hidden="true" />下载文件
            </a>
          </div>
        </header>
        <div class="preview-stage">
          <button
            v-if="previewable(selectedFile) && previewUrl(selectedFile) && !broken.has(fileKey(selectedFile, selectedIndex))"
            type="button"
            class="preview-zoom"
            :aria-label="`放大查看 ${fileName(selectedFile)}`"
            @click="openLightbox(selectedIndex)"
          >
            <img :src="previewUrl(selectedFile)" :alt="fileName(selectedFile)" @error="markBroken(selectedFile, selectedIndex)" />
            <span class="preview-zoom-hint"><Maximize2 :size="14" aria-hidden="true" />点击放大</span>
          </button>
          <div v-else class="preview-fallback"><FileText :size="46" aria-hidden="true" /><strong>{{ fileType(selectedFile) }}</strong><p>{{ fileUrl(selectedFile) ? '该文件不支持网页内预览，可下载后查看。' : '附件记录已保留，但当前账号没有预览或下载权限。' }}</p></div>
        </div>
        <footer><span>{{ selectedIndex + 1 }} / {{ files.length }}</span><p>参考附件仅用于理解运营需求，不会被当作最终成品。</p></footer>
      </template>
      <div v-else class="preview-empty"><Paperclip :size="44" aria-hidden="true" /><strong>还没有参考附件</strong><p>上传图片、PDF 或压缩包后，可在这里集中预览与下载。</p></div>
    </article>

    <ImagePreviewLightbox
      v-model="lightboxOpen"
      :items="lightboxItems"
      :initial-index="lightboxIndex"
      aria-label="参考附件预览"
      fallback-title="参考附件"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Download, FileText, Maximize2, Paperclip, Plus, RefreshCw } from 'lucide-vue-next'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import type { ImagePreviewLightboxItem } from '@/components/media/imagePreviewLightbox'

interface AttachmentFile extends Record<string, unknown> {
  id?: number
  asset_id?: string
  ref_id?: string
  file_name?: string
  filename?: string
  mime_type?: string
  download_url?: string
  preview_url?: string
  url?: string
}

const props = defineProps<{ files: AttachmentFile[]; canUpload?: boolean; canReplace?: boolean; replaceableRefIds?: string[]; uploading?: boolean; replacing?: boolean }>()
defineEmits<{ upload: []; replace: [file: AttachmentFile] }>()
const selectedIndex = ref(0)
const broken = ref(new Set<string>())
const selectedFile = computed(() => props.files[selectedIndex.value] || null)
const replaceableRefIDSet = computed(() => new Set((props.replaceableRefIds || []).map((value) => String(value).trim()).filter(Boolean)))
const canReplaceSelected = computed(() => {
  if (!props.canReplace || !selectedFile.value) return false
  const refID = String(selectedFile.value.ref_id || selectedFile.value.asset_id || '').trim()
  return refID !== '' && replaceableRefIDSet.value.has(refID)
})
watch(() => props.files.length, (length) => { if (!length) selectedIndex.value = 0; else if (selectedIndex.value >= length) selectedIndex.value = length - 1 })

function fileName(file: AttachmentFile) { return String(file.filename || file.file_name || file.asset_id || '参考附件') }
function fileUrl(file: AttachmentFile) { return String(file.download_url || file.preview_url || file.url || '') }
function previewUrl(file: AttachmentFile) { return String(file.preview_url || file.url || file.download_url || '') }
function fileKey(file: AttachmentFile, index: number) { return String(file.id || file.asset_id || `${fileName(file)}-${index}`) }
function previewable(file: AttachmentFile) { return String(file.mime_type || '').startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(fileName(file)) }
function fileType(file: AttachmentFile) { const suffix = fileName(file).split('.').pop(); return String(suffix || '文件').toUpperCase() }
function markBroken(file: AttachmentFile, index: number) { broken.value = new Set(broken.value).add(fileKey(file, index)) }

const lightboxOpen = ref(false)
const lightboxIndex = ref(0)
// 只把图片放进灯箱，索引单独映射，否则 PDF / 压缩包会让左右翻页停在空白页。
const previewableEntries = computed(() => props.files
  .map((file, index) => ({ file, index }))
  .filter(({ file, index }) => previewable(file) && previewUrl(file) && !broken.value.has(fileKey(file, index))))
const lightboxItems = computed<ImagePreviewLightboxItem[]>(() => previewableEntries.value.map(({ file }) => ({
  src: previewUrl(file),
  title: fileName(file),
  alt: fileName(file),
  downloadUrl: fileUrl(file) || undefined,
  fallbackAssetId: file.asset_id ? String(file.asset_id) : undefined,
})))
function openLightbox(fileIndex: number) {
  const position = previewableEntries.value.findIndex((entry) => entry.index === fileIndex)
  if (position < 0) return
  lightboxIndex.value = position
  lightboxOpen.value = true
}
</script>

<style scoped>
.attachment-workspace{height:100%;min-height:0;display:grid;grid-template-columns:minmax(260px,330px) minmax(0,1fr);overflow:hidden;background:rgb(var(--yb-surface-soft))}.attachment-list{min-height:0;display:grid;grid-template-rows:auto 1fr;border-right:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}.attachment-list>header,.attachment-preview>header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:15px;border-bottom:1px solid rgb(var(--yb-border))}.attachment-list header p,.attachment-preview header p{margin:0;color:rgb(var(--yb-text-muted));font-size:10px;font-weight:750}.attachment-list header strong{font-size:14px}.attachment-list header button,.attachment-preview header a{display:inline-flex;align-items:center;justify-content:center;gap:6px;min-height:34px;border:1px solid rgb(var(--yb-border));border-radius:9px;padding:0 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font-size:11px;font-weight:720;text-decoration:none;cursor:pointer}.attachment-items{min-height:0;padding:8px;overflow:auto}.attachment-items>button{width:100%;display:grid;grid-template-columns:56px minmax(0,1fr);align-items:center;gap:10px;border:1px solid transparent;border-radius:10px;padding:7px;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer}.attachment-items>button:hover,.attachment-items>button.selected{border-color:rgb(var(--yb-brand-border));background:rgb(var(--yb-brand-soft))}.attachment-items button>span:last-child{min-width:0;display:grid;gap:4px}.attachment-items strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.attachment-items small{color:rgb(var(--yb-text-muted));font-size:10px}.attachment-thumb{width:56px;height:44px;display:grid;place-items:center;overflow:hidden;border-radius:7px;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-muted))}.attachment-thumb img{width:100%;height:100%;object-fit:cover}.attachment-preview{min-height:0;display:grid;grid-template-rows:auto minmax(0,1fr) auto}.attachment-preview h3{max-width:60vw;margin:3px 0 0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:15px}.attachment-preview header a{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.preview-stage{min-height:0;display:grid;place-items:center;padding:20px;overflow:auto;background:linear-gradient(45deg,rgb(var(--yb-surface-muted)) 25%,transparent 25%),linear-gradient(-45deg,rgb(var(--yb-surface-muted)) 25%,transparent 25%),linear-gradient(45deg,transparent 75%,rgb(var(--yb-surface-muted)) 75%),linear-gradient(-45deg,transparent 75%,rgb(var(--yb-surface-muted)) 75%);background-size:24px 24px;background-position:0 0,0 12px,12px -12px,-12px 0}.preview-stage>img{max-width:100%;max-height:100%;object-fit:contain;border-radius:8px;box-shadow:0 14px 40px rgb(var(--yb-shadow)/.15)}.preview-zoom{position:relative;max-width:100%;max-height:100%;display:grid;padding:0;border:0;background:transparent;cursor:zoom-in}.preview-zoom img{max-width:100%;max-height:100%;object-fit:contain;border-radius:8px;box-shadow:0 14px 40px rgb(var(--yb-shadow)/.15)}.preview-zoom-hint{position:absolute;right:9px;bottom:9px;display:inline-flex;align-items:center;gap:5px;padding:5px 9px;border-radius:999px;background:rgb(var(--yb-surface)/.92);color:rgb(var(--yb-text-secondary));font-size:11px;font-weight:700;opacity:0;transition:opacity .15s ease}.preview-zoom:hover .preview-zoom-hint,.preview-zoom:focus-visible .preview-zoom-hint{opacity:1}.preview-fallback,.preview-empty{display:grid;place-items:center;align-content:center;gap:8px;color:rgb(var(--yb-text-muted));text-align:center}.preview-fallback strong,.preview-empty strong{color:rgb(var(--yb-text));font-size:16px}.preview-fallback p,.preview-empty p{max-width:34ch;margin:0;font-size:12px;line-height:1.6}.attachment-preview>footer{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:11px 15px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-muted));font-size:11px}.attachment-preview>footer p{margin:0}.empty-copy{padding:22px;color:rgb(var(--yb-text-muted));font-size:12px;line-height:1.6}@media(max-width:760px){.attachment-workspace{grid-template-columns:1fr;grid-template-rows:minmax(180px,34vh) minmax(0,1fr)}.attachment-list{border-right:0;border-bottom:1px solid rgb(var(--yb-border))}.attachment-preview h3{max-width:48vw}.attachment-preview>footer{align-items:flex-start;flex-direction:column}}
</style>

<style scoped>
.attachment-actions{display:flex;align-items:center;gap:7px}.attachment-preview header button{display:inline-flex;align-items:center;justify-content:center;gap:6px;min-height:34px;border:1px solid rgb(var(--yb-brand-border));border-radius:9px;padding:0 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand-strong));font-size:11px;font-weight:720;cursor:pointer}.attachment-preview header button:disabled{opacity:.55;cursor:wait}
</style>
