<script setup lang="ts">
import ArchiveListingPage from './ArchiveListingPage.vue'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Download, FileArchive, FolderOpen, X } from 'lucide-vue-next'
import { assetWorkbenchApi, type SubmissionFileRow, type ArchiveVirtualFile, type ArchiveVirtualFolder } from '@aw/shared/api/assetWorkbenchApi'
import { formatFileSize } from '@aw/shared/format/number'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import ArchiveVirtualThumb from './ArchiveVirtualThumb.vue'
import { createArchiveEntryObjectUrl, downloadArchiveEntryBlob } from './archiveEntryBlob'
import { canOpenArchive } from './archiveFormat'

const props = defineProps<{ file: Pick<SubmissionFileRow, 'id' | 'display_name' | 'original_filename' | 'file_type'>; downloadOriginal?: () => void }>()
const emit = defineEmits<{ close: [] }>()
const title = computed(() => props.file.display_name || props.file.original_filename)
const archivePath = ref('')
const archiveFolders = ref<ArchiveVirtualFolder[]>([])
const archiveFiles = ref<ArchiveVirtualFile[]>([])
const archiveLoading = ref(false)
const archiveError = ref('')
const entry = ref<ArchiveVirtualFile | null>(null)
const previewUrl = ref('')
const previewHint = ref('')
let selectedTile: HTMLElement | null = null
let listing: AbortController | undefined
let preview: AbortController | undefined

const archiveBreadcrumbs = computed(() => {
  const crumbs = [{ label: '根目录', path: '' }]
  const segments = archivePath.value.split('/').filter(Boolean)
  let current = ''
  for (const segment of segments) {
    current = current ? `${current}/${segment}` : segment
    crumbs.push({ label: segment, path: current })
  }
  return crumbs
})

async function openArchiveFolder(path: string) {
  listing?.abort()
  closeEntry()
  const controller = new AbortController()
  listing = controller
  archivePath.value = path
  archiveError.value = ''
  archiveFolders.value = []
  archiveFiles.value = []
  archiveLoading.value = true
  try {
    if (!canOpenArchive(props.file)) throw new Error('该压缩包暂不支持在线打开，可下载后查看。')
    const result = await assetWorkbenchApi.browseArchiveFile(props.file.id, path, controller.signal)
    if (controller.signal.aborted) return
    archivePath.value = result.path
    archiveFolders.value = result.folders
    archiveFiles.value = result.files
  } catch (err) {
    if (!controller.signal.aborted) archiveError.value = err instanceof Error ? err.message : '压缩包打开失败'
  } finally {
    if (!controller.signal.aborted) archiveLoading.value = false
  }
}

function closeEntry() {
  preview?.abort()
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = ''
  entry.value = null
}

async function dismissEntry() {
  closeEntry()
  await nextTick()
  selectedTile?.focus({ preventScroll: true })
}

async function openArchiveVirtualFile(member: ArchiveVirtualFile, event: MouseEvent) {
  selectedTile = event.currentTarget as HTMLElement
  closeEntry()
  entry.value = member
  previewHint.value = '该格式暂不能在线预览，可下载后查看'
  if (!member.preview_url) return
  const controller = new AbortController()
  preview = controller
  previewHint.value = '正在加载预览…'
  try {
    await nextTick()
    if (controller.signal.aborted) return
    const url = await createArchiveEntryObjectUrl(props.file.id, member, controller.signal)
    if (controller.signal.aborted) URL.revokeObjectURL(url)
    else previewUrl.value = url
  } catch (err) {
    if (!controller.signal.aborted) previewHint.value = err instanceof Error ? err.message : '预览加载失败'
  }
}

watch(() => props.file.id, () => openArchiveFolder(''), { immediate: true })
onBeforeUnmount(() => { listing?.abort(); closeEntry() })
</script>

<template>
  <section
    class="aw-token-scope aw-upload-ledger-archive"
    role="dialog"
    :inert="Boolean(entry)"
    :aria-modal="entry ? 'false' : 'true'"
    aria-label="压缩包内容"
  >
    <div class="aw-upload-ledger-archive__backdrop" @click="emit('close')" />
    <article class="aw-upload-ledger-archive__panel">
      <header class="aw-upload-ledger-archive__head">
        <div>
          <p class="aw-eyebrow">压缩包内容</p>
          <h3 :title="title">{{ title }}</h3>
        </div>
        <div class="aw-upload-ledger-archive__actions">
          <button v-if="downloadOriginal" class="aw-secondary-button" type="button" @click="downloadOriginal()">
            <Download :size="16" aria-hidden="true" />
            下载压缩包
          </button>
          <button class="aw-secondary-button" type="button" aria-label="关闭压缩包内容" @click="emit('close')">
            <X :size="16" aria-hidden="true" />
            关闭
          </button>
        </div>
      </header>
      <div class="aw-upload-ledger-archive__body">
        <nav class="aw-upload-ledger-archive__breadcrumbs" aria-label="压缩包路径">
          <button
            v-for="(crumb, index) in archiveBreadcrumbs"
            :key="`${crumb.path || '__root__'}:${index}`"
            type="button"
            :class="{ 'is-active': index === archiveBreadcrumbs.length - 1 }"
            :aria-current="index === archiveBreadcrumbs.length - 1 ? 'location' : undefined"
            @click="openArchiveFolder(crumb.path)"
          >
            {{ crumb.label }}
          </button>
        </nav>
        <p v-if="archiveLoading" class="aw-drive-empty">正在读取压缩包…</p>
        <div v-else-if="archiveError" class="aw-upload-ledger-archive__empty">
          <FileArchive :size="34" aria-hidden="true" />
          <strong>{{ archiveError }}</strong>
        </div>
        <template v-else>
          <p class="aw-upload-ledger-archive__count">{{ archiveFolders.length }} 个文件夹 · {{ archiveFiles.length }} 个文件</p>
          <ArchiveListingPage :key="file.id + archivePath" :folders="archiveFolders" :files="archiveFiles" v-slot="{ folders: visibleFolders, files: visibleFiles }">
            <div v-if="visibleFolders.length" class="aw-upload-ledger-archive__grid">
              <button
                v-for="folder in visibleFolders"
                :key="folder.path"
                class="aw-upload-ledger-archive__folder"
                type="button"
                @click="openArchiveFolder(folder.path)"
              >
                <span class="aw-upload-ledger-archive__symbol"><FolderOpen :size="34" aria-hidden="true" /></span>
                <strong :title="folder.name">{{ folder.name }}</strong>
                <small>{{ folder.file_count }} 个文件</small>
              </button>
            </div>
            <div v-if="visibleFiles.length" class="aw-upload-ledger-archive__grid">
              <button
                v-for="member in visibleFiles"
                :key="member.path"
                class="aw-upload-ledger-archive__file"
                type="button"
                @click="openArchiveVirtualFile(member, $event)"
              >
                <ArchiveVirtualThumb :file-id="file.id" :file="member" :paused="Boolean(entry)" />
                <strong :title="member.name">{{ member.name }}</strong>
                <small>{{ member.file_type }} · {{ formatFileSize(member.file_size) }}</small>
              </button>
            </div>
          </ArchiveListingPage>
          <p v-if="archiveFolders.length === 0 && archiveFiles.length === 0" class="aw-drive-empty">压缩包内没有可显示的文件</p>
        </template>
      </div>
    </article>
  </section>
  <WorkbenchPreviewDialog v-if="entry" :key="entry.path" :open="true" eyebrow="压缩包内容" download-label="下载文件" :title="entry.name"
    :preview-url="previewUrl" :empty-label="previewHint" :mime-type="entry.mime_type"
    :meta-rows="[['内部路径', entry.path], ['文件大小', formatFileSize(entry.file_size)]]"
    @close="dismissEntry" @download="downloadArchiveEntryBlob(file.id, entry)" />
</template>
