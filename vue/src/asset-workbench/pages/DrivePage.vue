<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronRight, Download, Folder, FolderOpen, HardDrive, Search, Upload, X } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DriveDirectoryRow,
  type DriveFileRow,
  type DriveOrderRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import DriveThumb from '@aw/shared/drive/DriveThumb.vue'
import DriveUploadDialog from '@aw/shared/drive/DriveUploadDialog.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'

const session = useAssetWorkbenchSessionStore()
const router = useRouter()
const route = useRoute()

const isAdmin = computed(() => session.bootstrap?.is_admin === true)

const UNASSIGNED_KEY = 'unassigned'

interface SelectedDir {
  key: string
  id: number | null
  name: string
  unassigned: boolean
}

const directories = ref<DriveDirectoryRow[]>([])
const dirLoading = ref(false)
const dirError = ref('')

const selectedDir = ref<SelectedDir | null>(null)
const orders = ref<DriveOrderRow[]>([])
const ordersLoading = ref(false)

const selectedOrder = ref<string | null>(null)
const files = ref<DriveFileRow[]>([])
const filesLoading = ref(false)
const fileTotal = ref(0)
const filePage = ref(1)
const pageSize = 60

const selectedFile = ref<DriveFileRow | null>(null)
const highlightFileId = ref<number | null>(null)

const searchQuery = ref('')
const searchActive = ref(false)
const searchLoading = ref(false)
const searchResults = ref<DriveFileRow[]>([])
const searchTotal = ref(0)

const previewOpen = ref(false)
const previewFile = shallowRef<DriveFileRow | null>(null)
const previewUrl = ref('')

const uploadOpen = ref(false)
const currentDirRow = computed(() =>
  selectedDir.value ? directories.value.find((item) => dirKey(item) === selectedDir.value?.key) ?? null : null,
)

function dirKey(dir: DriveDirectoryRow): string {
  return dir.directory_id == null ? UNASSIGNED_KEY : String(dir.directory_id)
}

function orderLabel(orderNo: string): string {
  return orderNo && orderNo.trim() ? orderNo : '无订单号'
}

function formatSize(size: number): string {
  if (!size) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

async function loadDirectories() {
  dirLoading.value = true
  dirError.value = ''
  try {
    directories.value = await assetWorkbenchApi.driveDirectories()
  } catch (err) {
    dirError.value = err instanceof Error ? err.message : '目录加载失败'
  } finally {
    dirLoading.value = false
  }
}

async function selectDir(dir: DriveDirectoryRow, keepFile = false) {
  const next: SelectedDir = {
    key: dirKey(dir),
    id: dir.directory_id ?? null,
    name: dir.name,
    unassigned: dir.directory_id == null,
  }
  selectedDir.value = next
  selectedOrder.value = null
  files.value = []
  fileTotal.value = 0
  if (!keepFile) selectedFile.value = null
  ordersLoading.value = true
  try {
    orders.value = await assetWorkbenchApi.driveOrders(
      next.unassigned ? { unassigned: true } : { dir_id: next.id ?? undefined },
    )
  } catch {
    orders.value = []
  } finally {
    ordersLoading.value = false
  }
}

async function selectOrder(orderNo: string, keepFile = false) {
  selectedOrder.value = orderNo
  filePage.value = 1
  if (!keepFile) selectedFile.value = null
  await loadFiles()
}

async function loadFiles() {
  if (!selectedDir.value || selectedOrder.value == null) return
  filesLoading.value = true
  try {
    const result = await assetWorkbenchApi.driveFiles({
      dir_id: selectedDir.value.unassigned ? undefined : selectedDir.value.id ?? undefined,
      unassigned: selectedDir.value.unassigned,
      order_no: selectedOrder.value,
      page: filePage.value,
      page_size: pageSize,
    })
    files.value = result.items
    fileTotal.value = result.total
  } catch {
    files.value = []
    fileTotal.value = 0
  } finally {
    filesLoading.value = false
  }
}

const totalPages = computed(() => Math.max(1, Math.ceil(fileTotal.value / pageSize)))

async function changePage(delta: number) {
  const next = filePage.value + delta
  if (next < 1 || next > totalPages.value) return
  filePage.value = next
  await loadFiles()
}

function selectFile(file: DriveFileRow) {
  selectedFile.value = file
  highlightFileId.value = file.id
}

async function openPreview(file: DriveFileRow) {
  selectFile(file)
  previewFile.value = file
  previewUrl.value = ''
  previewOpen.value = true
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    if (meta.preview_url) previewUrl.value = meta.preview_url
  } catch {
    previewUrl.value = ''
  }
}

function closePreview() {
  previewOpen.value = false
  previewFile.value = null
  previewUrl.value = ''
}

async function downloadFile(file: DriveFileRow) {
  try {
    const meta = await assetWorkbenchApi.getFileDownload(file.id)
    if (meta.download_url) window.open(meta.download_url, '_blank', 'noopener,noreferrer')
  } catch {
    /* ignore */
  }
}

const previewMetaRows = computed<Array<[string, string]>>(() => {
  const file = previewFile.value
  if (!file) return []
  return [
    ['订单号', orderLabel(file.order_no)],
    ['所在目录', file.upload_directory_name],
    ['批次', file.submission_no],
    ['大小', formatSize(file.file_size)],
  ]
})

async function runSearch() {
  const q = searchQuery.value.trim()
  if (!q) {
    clearSearch()
    return
  }
  searchActive.value = true
  searchLoading.value = true
  try {
    const result = await assetWorkbenchApi.driveSearch({ q, page: 1, page_size: 60 })
    searchResults.value = result.items
    searchTotal.value = result.total
  } catch {
    searchResults.value = []
    searchTotal.value = 0
  } finally {
    searchLoading.value = false
  }
}

function clearSearch() {
  searchActive.value = false
  searchQuery.value = ''
  searchResults.value = []
  searchTotal.value = 0
}

async function revealFile(file: DriveFileRow) {
  clearSearch()
  const targetKey = file.upload_directory_id == null ? UNASSIGNED_KEY : String(file.upload_directory_id)
  let dir = directories.value.find((item) => dirKey(item) === targetKey)
  if (!dir) {
    await loadDirectories()
    dir = directories.value.find((item) => dirKey(item) === targetKey)
  }
  if (dir) {
    await selectDir(dir, true)
    await selectOrder(file.order_no, true)
    selectFile(file)
    highlightFileId.value = file.id
    window.setTimeout(() => {
      if (highlightFileId.value === file.id) highlightFileId.value = null
    }, 2400)
  }
}

function resetToRoot() {
  selectedDir.value = null
  selectedOrder.value = null
  orders.value = []
  files.value = []
  fileTotal.value = 0
  selectedFile.value = null
}

function backToDir() {
  selectedOrder.value = null
  files.value = []
  fileTotal.value = 0
  selectedFile.value = null
}

function goToBatch(file: DriveFileRow) {
  void router.push({ path: '/submissions', query: { focus: String(file.submission_id) } })
}

function uploadHere() {
  if (!selectedDir.value) return
  uploadOpen.value = true
}

async function onUploaded() {
  uploadOpen.value = false
  const prevKey = selectedDir.value?.key ?? null
  const prevOrder = selectedOrder.value
  await loadDirectories()
  if (prevKey) {
    const refreshed = directories.value.find((item) => dirKey(item) === prevKey)
    if (refreshed) {
      await selectDir(refreshed, true)
      if (prevOrder != null) await selectOrder(prevOrder, true)
    }
  }
}

onMounted(async () => {
  await loadDirectories()
  const locateId = Number(route.query.file_id)
  if (locateId > 0) {
    try {
      const file = await assetWorkbenchApi.driveLocate(locateId)
      await revealFile(file)
    } catch {
      /* ignore */
    }
  }
  const q = typeof route.query.q === 'string' ? route.query.q : ''
  if (q) {
    searchQuery.value = q
    await runSearch()
  }
})
</script>

<template>
  <section class="aw-drive aw-token-scope">
    <header class="aw-drive__toolbar">
      <div class="aw-drive__title">
        <HardDrive :size="20" aria-hidden="true" />
        <div>
          <p class="aw-eyebrow">素材网盘</p>
          <h2>{{ isAdmin ? '全站素材网盘' : '我的素材网盘' }}</h2>
        </div>
      </div>
      <form class="aw-drive__search" @submit.prevent="runSearch">
        <Search :size="16" aria-hidden="true" />
        <input
          v-model="searchQuery"
          type="search"
          :placeholder="isAdmin ? '全站搜图：订单号 / 文件名 / 批次号…' : '搜我上传的图：订单号 / 文件名…'"
          @keyup.enter="runSearch"
        />
        <button v-if="searchActive" class="aw-drive__search-clear" type="button" aria-label="清除搜索" @click="clearSearch">
          <X :size="14" aria-hidden="true" />
        </button>
      </form>
      <button
        v-if="isAdmin"
        class="aw-primary-button"
        type="button"
        :disabled="!selectedDir"
        :title="selectedDir ? '上传到当前文件夹' : '先进入一个文件夹'"
        @click="uploadHere"
      >
        <Upload :size="16" aria-hidden="true" />
        上传到此处
      </button>
    </header>

    <!-- Search results -->
    <div v-if="searchActive" class="aw-drive-search-results">
      <div class="aw-drive-search-results__head">
        <span>搜索「{{ searchQuery }}」</span>
        <span class="aw-drive-search-results__count">{{ searchLoading ? '搜索中…' : `共 ${searchTotal} 张` }}</span>
      </div>
      <p v-if="!searchLoading && searchResults.length === 0" class="aw-drive-empty">没有匹配的图片</p>
      <ul v-else class="aw-drive-hit-list">
        <li v-for="hit in searchResults" :key="hit.id" class="aw-drive-hit">
          <button class="aw-drive-hit__thumb" type="button" @click="openPreview(hit)">
            <DriveThumb :file-id="hit.id" :filename="hit.original_filename" :mime-type="hit.mime_type" :preview-status="hit.preview_status" />
          </button>
          <div class="aw-drive-hit__body">
            <strong>{{ hit.original_filename }}</strong>
            <span class="aw-drive-hit__path">
              全部 <ChevronRight :size="12" /> {{ hit.upload_directory_name }} <ChevronRight :size="12" /> {{ orderLabel(hit.order_no) }}
            </span>
            <small>批次 {{ hit.submission_no }} · {{ formatSize(hit.file_size) }} · {{ hit.business_month }}</small>
          </div>
          <div class="aw-drive-hit__actions">
            <button class="aw-secondary-button" type="button" @click="revealFile(hit)">
              <FolderOpen :size="14" aria-hidden="true" />
              在文件夹中显示
            </button>
            <button class="aw-link-button" type="button" @click="goToBatch(hit)">查看批次</button>
          </div>
        </li>
      </ul>
    </div>

    <!-- Browse: breadcrumb + Finder columns -->
    <template v-else>
      <nav class="aw-drive__breadcrumb" aria-label="路径">
        <button class="aw-drive__crumb" type="button" :class="{ 'is-active': !selectedDir }" @click="resetToRoot">全部</button>
        <template v-if="selectedDir">
          <ChevronRight :size="14" aria-hidden="true" />
          <button class="aw-drive__crumb" type="button" :class="{ 'is-active': selectedOrder == null }" @click="backToDir">
            {{ selectedDir.name }}
          </button>
        </template>
        <template v-if="selectedDir && selectedOrder != null">
          <ChevronRight :size="14" aria-hidden="true" />
          <span class="aw-drive__crumb is-active">{{ orderLabel(selectedOrder) }}</span>
        </template>
      </nav>

      <div class="aw-drive__body">
        <!-- Column 1: directories -->
        <div class="aw-drive-column">
          <p class="aw-drive-column__label">文件夹</p>
          <div class="aw-drive-column__scroll">
            <p v-if="dirLoading" class="aw-drive-empty">加载中…</p>
            <p v-else-if="dirError" class="aw-drive-empty">{{ dirError }}</p>
            <p v-else-if="directories.length === 0" class="aw-drive-empty">暂无文件夹</p>
            <button
              v-for="dir in directories"
              :key="dirKey(dir)"
              class="aw-drive-column__item"
              :class="{ 'is-active': selectedDir?.key === dirKey(dir) }"
              type="button"
              @click="selectDir(dir)"
            >
              <Folder :size="16" aria-hidden="true" />
              <span class="aw-drive-column__name">{{ dir.name }}</span>
              <span class="aw-chip aw-chip--neutral aw-drive-column__count">{{ dir.file_count }}</span>
              <ChevronRight :size="14" class="aw-drive-column__chevron" aria-hidden="true" />
            </button>
          </div>
        </div>

        <!-- Column 2: orders -->
        <div class="aw-drive-column">
          <p class="aw-drive-column__label">订单号</p>
          <div class="aw-drive-column__scroll">
            <p v-if="!selectedDir" class="aw-drive-empty">← 选择一个文件夹</p>
            <p v-else-if="ordersLoading" class="aw-drive-empty">加载中…</p>
            <p v-else-if="orders.length === 0" class="aw-drive-empty">暂无订单</p>
            <button
              v-for="order in orders"
              :key="order.order_no || '__empty__'"
              class="aw-drive-column__item"
              :class="{ 'is-active': selectedOrder === order.order_no }"
              type="button"
              @click="selectOrder(order.order_no)"
            >
              <Folder :size="16" aria-hidden="true" />
              <span class="aw-drive-column__name">{{ orderLabel(order.order_no) }}</span>
              <span class="aw-chip aw-chip--neutral aw-drive-column__count">{{ order.file_count }}</span>
              <ChevronRight :size="14" class="aw-drive-column__chevron" aria-hidden="true" />
            </button>
          </div>
        </div>

        <!-- Column 3: files grid -->
        <div class="aw-drive-column aw-drive-column--files">
          <p class="aw-drive-column__label">
            图片
            <span v-if="selectedOrder != null && fileTotal > 0" class="aw-drive-column__sub">{{ fileTotal }} 张</span>
          </p>
          <div class="aw-drive-column__scroll">
            <p v-if="selectedOrder == null" class="aw-drive-empty">← 选择一个订单</p>
            <p v-else-if="filesLoading" class="aw-drive-empty">加载中…</p>
            <p v-else-if="files.length === 0" class="aw-drive-empty">该订单暂无图片</p>
            <div v-else class="aw-drive-files">
              <button
                v-for="file in files"
                :key="file.id"
                class="aw-drive-file-card"
                :class="{
                  'is-selected': selectedFile?.id === file.id,
                  'is-highlight': highlightFileId === file.id,
                }"
                type="button"
                @click="selectFile(file)"
                @dblclick="openPreview(file)"
              >
                <span class="aw-drive-file-card__media">
                  <DriveThumb :file-id="file.id" :filename="file.original_filename" :mime-type="file.mime_type" :preview-status="file.preview_status" />
                </span>
                <span class="aw-drive-file-card__name">{{ file.original_filename }}</span>
              </button>
            </div>
          </div>
          <div v-if="selectedOrder != null && totalPages > 1" class="aw-drive-pager">
            <button class="aw-grid-button" type="button" :disabled="filePage <= 1" @click="changePage(-1)">上一页</button>
            <span>{{ filePage }} / {{ totalPages }}</span>
            <button class="aw-grid-button" type="button" :disabled="filePage >= totalPages" @click="changePage(1)">下一页</button>
          </div>
        </div>

        <!-- Detail panel -->
        <aside class="aw-drive__detail">
          <template v-if="selectedFile">
            <button class="aw-drive__detail-preview" type="button" @click="openPreview(selectedFile)">
              <DriveThumb :file-id="selectedFile.id" :filename="selectedFile.original_filename" :mime-type="selectedFile.mime_type" :preview-status="selectedFile.preview_status" />
              <span class="aw-drive__detail-hint">点击预览</span>
            </button>
            <h3 class="aw-drive__detail-name">{{ selectedFile.original_filename }}</h3>
            <dl class="aw-material-detail__list">
              <div><dt>订单号</dt><dd>{{ orderLabel(selectedFile.order_no) }}</dd></div>
              <div><dt>目录</dt><dd>{{ selectedFile.upload_directory_name }}</dd></div>
              <div><dt>批次</dt><dd>{{ selectedFile.submission_no }}</dd></div>
              <div><dt>大小</dt><dd>{{ formatSize(selectedFile.file_size) }}</dd></div>
              <div><dt>月份</dt><dd>{{ selectedFile.business_month }}</dd></div>
            </dl>
            <div class="aw-drive__detail-actions">
              <button class="aw-primary-button" type="button" @click="downloadFile(selectedFile)">
                <Download :size="16" aria-hidden="true" />
                下载
              </button>
              <button class="aw-secondary-button" type="button" @click="goToBatch(selectedFile)">查看批次</button>
            </div>
          </template>
          <div v-else class="aw-drive-empty aw-drive__detail-empty">
            选择一张图片查看详情
          </div>
        </aside>
      </div>
    </template>

    <DriveUploadDialog
      :open="uploadOpen"
      :directory-id="selectedDir && !selectedDir.unassigned ? selectedDir.id ?? undefined : undefined"
      :directory-name="selectedDir?.name ?? ''"
      :difficulty-class="currentDirRow?.difficulty_class"
      :default-order-no="selectedOrder ?? ''"
      @close="uploadOpen = false"
      @uploaded="onUploaded"
    />

    <WorkbenchPreviewDialog
      :open="previewOpen"
      :title="previewFile?.original_filename || ''"
      eyebrow="素材预览"
      :preview-url="previewUrl"
      :mime-type="previewFile?.mime_type"
      :filename="previewFile?.original_filename"
      :meta-rows="previewMetaRows"
      download-label="下载"
      @close="closePreview"
      @download="previewFile && downloadFile(previewFile)"
    />
  </section>
</template>
