<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { FolderOpen, Images, Upload, X } from 'lucide-vue-next'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { assetWorkbenchApi, type SettlementSupplementRow, type SupplementPermissionRow, type UploadDirectoryRow } from '@aw/shared/api/assetWorkbenchApi'
import { formatInt } from '@aw/shared/format/number'
import { driveUploadRelativePath, filesFromDriveDrop, groupDriveUploadPieceworkItems, isSafeDriveUploadPath } from '@aw/shared/drive/useDriveUpload'
import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { buildSelfSupplementPayload, duplicateSupplementFileNames, filterSupplementUploadFiles } from './supplementUpload'

const props = defineProps<{
  permissions: Pick<SupplementPermissionRow, 'business_month' | 'payee_user_id'>[]
  directories: UploadDirectoryRow[]
  supplements?: SettlementSupplementRow[]
  ready: boolean
  error?: string
  accessClosed?: boolean
}>()
const emit = defineEmits<{ submitted: []; busy: [value: boolean]; refresh: []; ordinaryUpload: [] }>()
const supplementPermissions = computed(() => props.permissions)
const supplementMonth = ref('')
const supplementPermission = computed(() => props.permissions.find(row => row.business_month === supplementMonth.value))
const uploadDirectories = computed(() => props.directories)
const supplements = computed(() => props.supplements ?? [])
const selectedDirectoryId = ref(0)
const supplementDate = ref(todayDate())
const selectedFiles = ref<File[]>([])
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadNotice = ref('')
const fileSelectionNotice = ref('')
const supplementDragActive = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const folderInput = ref<HTMLInputElement | null>(null)
const selectedDirectory = computed(() => uploadDirectories.value.find((item) => item.id === selectedDirectoryId.value) ?? null)
const selectedSupplementItems = computed(() => selectedFiles.value.map((file) => ({ file, relativePath: driveUploadRelativePath(file) })))
const selectedSupplementGroups = computed(() => groupDriveUploadPieceworkItems(selectedSupplementItems.value))
const selectedSupplementWorkNames = computed(() => selectedSupplementGroups.value.map((group) => ({ name: supplementGroupName(group) })))
const duplicateFilenames = computed(() => duplicateSupplementFileNames(selectedSupplementWorkNames.value, supplements.value.filter((row) => row.business_month === supplementMonth.value)))
const supplementUploadOpen = computed(() => Boolean(supplementPermission.value))
const supplementAccessLabel = computed(() => {
  if (props.accessClosed) return '不可访问'
  if (!props.ready) return props.error ? '读取失败' : '正在读取'
  return supplementUploadOpen.value ? `已开放 ${supplementPermission.value?.business_month}` : '当前未开放'
})
const canUploadSupplement = computed(() =>
  props.ready && supplementUploadOpen.value && Boolean(selectedDirectory.value) && selectedSupplementGroups.value.length > 0 && Boolean(supplementDate.value) && !uploading.value,
)
const supplementAcceptString = computed(() => {
  const allowed = selectedDirectory.value?.allowed_file_types ?? []
  return allowed.map((value) => value.trim()).filter(Boolean).map((value) => value.includes('/') ? value : `.${value.replace(/^\.+/, '')}`).join(',')
})
const supplementAllowedLabel = computed(() => selectedDirectory.value?.allowed_file_types?.length
  ? selectedDirectory.value.allowed_file_types.join('、')
  : '全部格式')
watch(supplementPermissions, (permissions) => {
  if (permissions.some((row) => row.business_month === supplementMonth.value)) return
  if (selectedFiles.value.length) {
    supplementMonth.value = ''
    uploadError.value = '原选择的补录月份已关闭或锁定。待上传文件已保留，请重新确认结算月后上传。'
  } else {
    supplementMonth.value = permissions[0]?.business_month ?? ''
  }
}, { immediate: true })


watch(uploadDirectories, (rows) => {
  if (!rows.some(row => row.id === selectedDirectoryId.value)) selectedDirectoryId.value = rows[0]?.id ?? 0
}, { immediate: true })
watch(uploading, value => emit('busy', value), { flush: 'sync' })

function todayDate() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function supplementQueueFileKey(file: File) {
  return `${driveUploadRelativePath(file).toLocaleLowerCase('zh-CN')}::${file.size}::${file.lastModified}`
}

function applySupplementFiles(files: File[] | FileList | null | undefined) {
  const candidates = Array.from(files ?? [])
  const safeFiles = candidates.filter((file) => isSafeDriveUploadPath(driveUploadRelativePath(file)))
  const selection = filterSupplementUploadFiles(safeFiles, selectedDirectory.value?.allowed_file_types ?? [])
  const ignored = selection.ignored + candidates.length - safeFiles.length
  const queuedKeys = new Set(selectedFiles.value.map(supplementQueueFileKey))
  const additions: File[] = []
  let repeated = 0
  for (const file of selection.files) {
    const key = supplementQueueFileKey(file)
    if (queuedKeys.has(key)) {
      repeated += 1
      continue
    }
    queuedKeys.add(key)
    additions.push(file)
  }
  selectedFiles.value = [...selectedFiles.value, ...additions]
  fileSelectionNotice.value = selectedFiles.value.length
    ? `本次新增 ${formatInt(additions.length)} 个文件；待上传队列共 ${formatInt(selectedFiles.value.length)} 个文件，归为 ${formatInt(selectedSupplementGroups.value.length)} 个补录作品${repeated ? `；跳过 ${formatInt(repeated)} 个队列内重复文件` : ''}${ignored ? `；忽略 ${formatInt(ignored)} 个空文件、隐藏路径或不符合目录格式限制的文件` : ''}`
    : ''
  uploadError.value = selectedFiles.value.length
    ? ''
    : ignored
      ? `没有读取到可上传文件，已忽略 ${formatInt(ignored)} 个空文件、隐藏路径或不符合目录格式限制的文件（允许：${supplementAllowedLabel.value}）`
      : '没有读取到可上传文件'
  uploadNotice.value = ''
}

function supplementGroupName(group: { isFolder: boolean; items: Array<{ file: File; relativePath: string }> }) {
  const first = group.items[0]
  if (!first) return '补录作品'
  return group.isFolder ? first.relativePath.split('/')[0] || first.file.name : first.file.name
}

function selectSupplementFiles(event: Event) {
  const input = event.target as HTMLInputElement | null
  applySupplementFiles(input?.files)
  if (input) input.value = ''
}

async function dropSupplementFiles(event: DragEvent) {
  supplementDragActive.value = false
  if (uploading.value) return
  try {
    const files = await filesFromDriveDrop(event.dataTransfer)
    if (!uploading.value) applySupplementFiles(files)
  } catch (err) {
    uploadError.value = resolveApiUserMessage(err, { fallback: '读取文件夹失败，请改用“选择文件夹”' })
  }
}

function removeSupplementQueueGroup(group: { items: Array<{ file: File }> }) {
  const groupFiles = new Set(group.items.map((item) => item.file))
  selectedFiles.value = selectedFiles.value.filter((item) => !groupFiles.has(item))
  fileSelectionNotice.value = selectedFiles.value.length
    ? `待上传队列共 ${formatInt(selectedFiles.value.length)} 个文件，归为 ${formatInt(selectedSupplementGroups.value.length)} 个补录作品`
    : ''
  uploadError.value = ''
}

function clearSupplementQueue() {
  selectedFiles.value = []
  fileSelectionNotice.value = ''
  uploadError.value = ''
  if (fileInput.value) fileInput.value.value = ''
  if (folderInput.value) folderInput.value.value = ''
}

async function uploadSupplements() {
  const permission = supplementPermission.value
  const directory = selectedDirectory.value
  const date = supplementDate.value
  if (!props.ready || !permission || uploading.value || !directory || !date || !selectedSupplementGroups.value.length) return
  uploading.value = true
  uploadProgress.value = 0
  uploadError.value = ''
  uploadNotice.value = ''
  let completedWorks = 0
  let processedFiles = 0
  const uploadGroups = selectedSupplementGroups.value.map((group) => ({ ...group, items: [...group.items] }))
  const totalFiles = uploadGroups.reduce((count, group) => count + group.items.length, 0)
  try {
    for (const group of uploadGroups) {
      const sessionIds: string[] = []
      const uploadBatchId = crypto.randomUUID?.() ?? `${Date.now()}-${completedWorks}`
      for (const item of group.items) {
        const fileIndex = processedFiles
        const uploaded = await uploadWorkbenchFile(item.file, {
          uploadDirectoryId: directory.id,
          uploadBatchId,
          expectedBusinessMonth: permission.business_month,
          relativePath: item.relativePath,
          isFolderUpload: group.isFolder,
          onProgress: (progress) => {
            uploadProgress.value = Math.round(((fileIndex + progress.percent / 100) / totalFiles) * 100)
          },
        })
        sessionIds.push(uploaded.sessionId)
        processedFiles += 1
      }
      await assetWorkbenchApi.createSettlementSupplement(
        buildSelfSupplementPayload({ name: supplementGroupName(group) }, sessionIds, date, permission, directory),
      )
      completedWorks += 1
    }
    uploadNotice.value = `补录上传完成：${formatInt(completedWorks)} 个作品，共 ${formatInt(totalFiles)} 个文件，归档到「${directory.name}」，计入 ${permission.business_month}。`
    selectedFiles.value = []
    if (fileInput.value) fileInput.value.value = ''
    if (folderInput.value) folderInput.value.value = ''
    fileSelectionNotice.value = ''
    emit('submitted')
  } catch (err) {
    selectedFiles.value = uploadGroups.slice(completedWorks).flatMap((group) => group.items.map((item) => item.file))
    uploadError.value = resolveApiUserMessage(err, { fallback: `补录上传中断，已完成 ${completedWorks} 个作品，请核对记录后重试。` })
    emit('refresh')
  } finally {
    uploading.value = false
  }
}

</script>

<template>
    <div class="aw-data-surface aw-self-supplement-upload">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">补录上传</p>
          <h3>补交遗漏作品</h3>
          <p class="aw-copy">选择管理员已开放的结算月补交遗漏作品；补录工资计入所选月份，单独列示。已有生成或确认批次的月份不能补录。</p>
        </div>
        <span class="aw-chip" :class="supplementUploadOpen ? 'aw-chip--success' : 'aw-chip--neutral'">
          {{ supplementAccessLabel }}
        </span>
      </div>

      <p v-if="accessClosed" class="aw-inline-alert aw-inline-alert--info">结算页面暂未开放，无法读取补录权限。</p>
      <p v-else-if="!ready" class="aw-inline-alert aw-inline-alert--info" role="status">{{ error ? '补录权限读取失败，请刷新重试。' : '正在读取补录权限…' }} <button v-if="error" class="aw-grid-button" type="button" @click="emit('refresh')">重试</button></p>
      <div v-else-if="supplementPermissions.length" class="aw-form-grid">
        <label>
          补录结算月
          <select v-model="supplementMonth" :disabled="uploading" aria-describedby="supplement-month-help">
            <option value="" disabled>请重新选择结算月</option>
            <option v-for="permission in supplementPermissions" :key="permission.business_month" :value="permission.business_month">{{ permission.business_month }}</option>
          </select>
          <span id="supplement-month-help" class="aw-copy">本次补录工资归入此月；补录日期不会自动改变结算月。</span>
        </label>
        <label>
          补录日期
          <input v-model="supplementDate" type="date" :disabled="uploading" aria-describedby="supplement-date-help" />
          <span id="supplement-date-help" class="aw-copy">指作品原本应该上传、但因遗漏未上传的日期。</span>
        </label>
        <label>
          上传目录 / 计价分类
          <select v-model.number="selectedDirectoryId" :disabled="uploading" aria-describedby="aw-supplement-price-hint">
            <option :value="0" disabled>请选择上传目录</option>
            <option v-for="directory in uploadDirectories" :key="directory.id" :value="directory.id">
              {{ directory.name }} · {{ directory.difficulty_class }}类
            </option>
          </select>
        </label>
        <div class="aw-form-grid__full">
          <p id="aw-supplement-price-hint" class="aw-inline-alert aw-inline-alert--info">
            补录前需匹配生效单价，匹配依据是当前人员类型、岗级和上传目录的计价分类。没有匹配单价的作品无法补录。
          </p>
          <p v-if="selectedDirectory?.difficulty_class === '0'" class="aw-copy" role="status">
            所选目录是 0 类。补录前请确认当前岗级已有适用的 0 类单价。只交不计费文件时，请走
            <RouterLink class="aw-text-link" to="/upload" @click="emit('ordinaryUpload')">上传成品</RouterLink>。
          </p>
        </div>
        <div class="aw-form-grid__full aw-supplement-file-picker">
          <span class="aw-supplement-file-picker__label">选择补录文件</span>
          <div
            class="aw-supplement-file-picker__dropzone"
            :class="{ 'is-active': supplementDragActive }"
            @dragenter.prevent="supplementDragActive = true"
            @dragover.prevent="supplementDragActive = true"
            @dragleave.self="supplementDragActive = false"
            @drop.prevent="dropSupplementFiles"
          >
            <Images :size="24" aria-hidden="true" />
            <div>
              <strong>拖入文件、压缩包或整个文件夹</strong>
              <span>文件夹按正常交稿规则计为 1 个作品；允许：{{ supplementAllowedLabel }}。</span>
            </div>
            <div class="aw-supplement-file-picker__actions">
              <button class="aw-secondary-button" type="button" :disabled="uploading" @click="fileInput?.click()">选择文件</button>
              <button class="aw-secondary-button" type="button" :disabled="uploading" @click="folderInput?.click()">
                <FolderOpen :size="15" aria-hidden="true" />
                选择文件夹
              </button>
            </div>
          </div>
          <input ref="fileInput" class="aw-visually-hidden" type="file" :disabled="uploading" :accept="supplementAcceptString" multiple aria-label="选择补录文件" @change="selectSupplementFiles" />
          <input ref="folderInput" class="aw-visually-hidden" type="file" :disabled="uploading" :accept="supplementAcceptString" multiple webkitdirectory directory aria-label="选择补录文件夹" @change="selectSupplementFiles" />
          <span v-if="fileSelectionNotice" class="aw-supplement-file-picker__notice">{{ fileSelectionNotice }}</span>
        </div>
        <section v-if="selectedSupplementGroups.length" class="aw-form-grid__full aw-supplement-upload-queue" aria-label="补录待上传队列">
          <header class="aw-supplement-upload-queue__header">
            <div>
              <strong>待上传队列</strong>
              <span>{{ formatInt(selectedFiles.length) }} 个文件 · {{ formatInt(selectedSupplementGroups.length) }} 个补录作品</span>
            </div>
            <button class="aw-grid-button" type="button" :disabled="uploading" @click="clearSupplementQueue">清空队列</button>
          </header>
          <div class="aw-supplement-upload-queue__groups">
            <article v-for="(group, groupIndex) in selectedSupplementGroups" :key="`${supplementGroupName(group)}-${groupIndex}`" class="aw-supplement-upload-queue__group">
              <div class="aw-supplement-upload-queue__group-head">
                <div>
                  <strong :title="supplementGroupName(group)">{{ supplementGroupName(group) }}</strong>
                  <span>{{ group.isFolder ? '文件夹作品' : '单文件作品' }} · {{ formatInt(group.items.length) }} 个文件</span>
                </div>
                <button
                  type="button"
                  :disabled="uploading"
                  :aria-label="`${group.isFolder ? '移除待上传文件夹' : '移除待上传文件'} ${supplementGroupName(group)}`"
                  @click="removeSupplementQueueGroup(group)"
                >
                  <X :size="14" aria-hidden="true" />
                </button>
              </div>
              <ul>
                <li v-for="item in group.items" :key="supplementQueueFileKey(item.file)">
                  <span :title="item.relativePath">{{ item.relativePath }}</span>
                </li>
              </ul>
            </article>
          </div>
        </section>
      </div>
      <p v-else class="aw-inline-alert aw-inline-alert--info">当前没有可用的补录权限；请联系管理员开放尚未生成结算批次的月份。</p>

      <div v-if="supplementUploadOpen" class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="!canUploadSupplement" @click="uploadSupplements">
          <Upload :size="16" aria-hidden="true" />
          {{ uploading ? `上传中 ${uploadProgress}%` : `上传 ${formatInt(selectedSupplementGroups.length)} 个补录作品` }}
        </button>
        <span v-if="selectedDirectory" class="aw-copy">计价分类：{{ selectedDirectory.difficulty_class }} 类</span>
      </div>
      <p v-if="duplicateFilenames.length" class="aw-inline-alert aw-inline-alert--warning" role="alert">
        同名提醒：{{ duplicateFilenames.join('、') }} 已存在补录记录，请确认后再上传。
      </p>
      <p v-if="uploadError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ uploadError }}</p>
      <p v-if="uploadNotice" class="aw-inline-alert">{{ uploadNotice }}</p>
    </div>

</template>
