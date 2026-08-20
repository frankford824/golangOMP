<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Eye, FolderOpen, Gift, Images, RefreshCw, Trash2, Upload, WalletCards } from 'lucide-vue-next'

import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { buildSelfSupplementPayload, duplicateSupplementFileNames, filterSupplementUploadFiles } from '@aw/features/supplement/supplementUpload'
import { driveUploadRelativePath, filesFromDriveDrop, groupDriveUploadPieceworkItems, isSafeDriveUploadPath } from '@aw/shared/drive/useDriveUpload'
import {
  assetWorkbenchApi,
  type MySettlementMonthRow,
  type MySettlementResult,
  type SettlementSupplementRow,
  type SubmissionFileRow,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const settlementRequest = usePageRequest<MySettlementResult>(
  () => assetWorkbenchApi.mySettlement(),
  null,
  '收入加载失败',
)
const loading = settlementRequest.loading
const error = settlementRequest.error
const result = settlementRequest.data

const currentAmount = computed(() => formatMoney(result.value?.estimated_net_amount ?? 0))
const months = computed(() => result.value?.months ?? [])
const supplementPermission = computed(() => result.value?.supplement_permission ?? null)
const supplements = computed(() => result.value?.supplements ?? [])
const activeSupplements = computed(() => supplements.value.filter((row) => row.status !== 'voided'))
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const selectedDirectoryId = ref(0)
const supplementDate = ref(todayDate())
const selectedFiles = ref<File[]>([])
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadNotice = ref('')
const fileSelectionNotice = ref('')
const supplementDragActive = ref(false)
const selectedSupplementIds = ref<number[]>([])
const supplementDeleteReason = ref('')
const deletingSupplements = ref(false)
const deleteDialogOpen = ref(false)
const deleteDialogMode = ref<'single' | 'batch'>('batch')
const fileInput = ref<HTMLInputElement | null>(null)
const folderInput = ref<HTMLInputElement | null>(null)
const previewDialog = ref({
  open: false,
  title: '',
  previewUrl: '',
  mimeType: '',
  filename: '',
  emptyLabel: '',
  metaRows: [] as Array<[string, string]>,
})
const selectedDirectory = computed(() => uploadDirectories.value.find((item) => item.id === selectedDirectoryId.value) ?? null)
const selectedSupplementItems = computed(() => selectedFiles.value.map((file) => ({ file, relativePath: driveUploadRelativePath(file) })))
const selectedSupplementGroups = computed(() => groupDriveUploadPieceworkItems(selectedSupplementItems.value))
const selectedSupplementWorkNames = computed(() => selectedSupplementGroups.value.map((group) => ({ name: supplementGroupName(group) })))
const duplicateFilenames = computed(() => duplicateSupplementFileNames(selectedSupplementWorkNames.value, supplements.value))
const supplementUploadOpen = computed(() => Boolean(supplementPermission.value?.enabled))
const canUploadSupplement = computed(() =>
  supplementUploadOpen.value && Boolean(selectedDirectory.value) && selectedSupplementGroups.value.length > 0 && Boolean(supplementDate.value) && !uploading.value,
)
const supplementAcceptString = computed(() => {
  const allowed = selectedDirectory.value?.allowed_file_types ?? []
  return allowed.map((value) => value.trim()).filter(Boolean).map((value) => value.includes('/') ? value : `.${value.replace(/^\.+/, '')}`).join(',')
})
const supplementAllowedLabel = computed(() => selectedDirectory.value?.allowed_file_types?.length
  ? selectedDirectory.value.allowed_file_types.join('、')
  : '全部格式')
const deletableSupplements = computed(() => activeSupplements.value.filter((row) => ['draft', 'approved'].includes(row.status)))
const selectedSupplements = computed(() => deletableSupplements.value.filter((row) => selectedSupplementIds.value.includes(row.id)))
const allSupplementsSelected = computed(() => deletableSupplements.value.length > 0 && deletableSupplements.value.every((row) => selectedSupplementIds.value.includes(row.id)))
const selectedSupplementAmount = computed(() => selectedSupplements.value.reduce((total, row) => total + row.gross_amount, 0))

function formatMonthLabel(month: string) {
  const [year, value] = month.split('-')
  if (!year || !value) return month
  return `${year}年${value}月`
}

function normalPieceworkAmount(month: MySettlementMonthRow) {
  return month.gross_amount - month.deduction_amount + month.welfare_amount + month.adjustment_amount
}

function supplementAmount(month: MySettlementMonthRow) {
  return month.supplement_amount
}

function supplementLabel(month: MySettlementMonthRow) {
  return month.supplement_amount > 0 ? '有补交' : '0'
}

function todayDate() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function applySupplementFiles(files: File[] | FileList | null | undefined) {
  const candidates = Array.from(files ?? [])
  const safeFiles = candidates.filter((file) => isSafeDriveUploadPath(driveUploadRelativePath(file)))
  const selection = filterSupplementUploadFiles(safeFiles, selectedDirectory.value?.allowed_file_types ?? [])
  const ignored = selection.ignored + candidates.length - safeFiles.length
  selectedFiles.value = selection.files
  fileSelectionNotice.value = selectedFiles.value.length
    ? `已读取 ${formatInt(selectedFiles.value.length)} 个文件，归为 ${formatInt(selectedSupplementGroups.value.length)} 个补录作品${ignored ? `；忽略 ${formatInt(ignored)} 个空文件、隐藏路径或不符合目录格式限制的文件` : ''}`
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
}

async function dropSupplementFiles(event: DragEvent) {
  supplementDragActive.value = false
  try {
    applySupplementFiles(await filesFromDriveDrop(event.dataTransfer))
  } catch (err) {
    selectedFiles.value = []
    fileSelectionNotice.value = ''
    uploadError.value = resolveApiUserMessage(err, { fallback: '读取文件夹失败，请改用“选择文件夹”' })
  }
}

async function uploadSupplements() {
  const permission = supplementPermission.value
  const directory = selectedDirectory.value
  if (!permission?.enabled || !directory || !supplementDate.value || !selectedSupplementGroups.value.length) return
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
        buildSelfSupplementPayload({ name: supplementGroupName(group) }, sessionIds, supplementDate.value, permission, directory),
      )
      completedWorks += 1
    }
    uploadNotice.value = `补录上传完成：${formatInt(completedWorks)} 个作品，共 ${formatInt(totalFiles)} 个文件，统一计入 ${permission.business_month} 补录工资。`
    selectedFiles.value = []
    if (fileInput.value) fileInput.value.value = ''
    if (folderInput.value) folderInput.value.value = ''
    fileSelectionNotice.value = ''
    await settlementRequest.run()
  } catch (err) {
    selectedFiles.value = uploadGroups.slice(completedWorks).flatMap((group) => group.items.map((item) => item.file))
    uploadError.value = resolveApiUserMessage(err, { fallback: `补录上传中断，已完成 ${completedWorks} 个作品，请核对记录后重试。` })
  } finally {
    uploading.value = false
  }
}

async function openSupplementPreview(row: SettlementSupplementRow, file: SubmissionFileRow) {
  previewDialog.value = {
    open: true,
    title: file.display_name || file.original_filename || row.order_no,
    previewUrl: '',
    mimeType: file.mime_type,
    filename: file.original_filename,
    emptyLabel: '正在准备预览',
    metaRows: [['补录日期', row.supplement_date || '—'], ['计价分类', row.difficulty_class]],
  }
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    previewDialog.value.previewUrl = meta.preview_url || meta.download_url || ''
    previewDialog.value.mimeType = meta.mime_type || file.mime_type
    previewDialog.value.filename = meta.filename || file.original_filename
    previewDialog.value.emptyLabel = meta.preparing ? '预览正在生成，请稍后再试' : (meta.error || '当前文件暂不能预览')
    previewDialog.value.metaRows = [
      ['预览状态', meta.status || file.preview_status],
      ['补录日期', row.supplement_date || '—'],
      ['计价分类', row.difficulty_class],
    ]
  } catch (err) {
    previewDialog.value.emptyLabel = resolveApiUserMessage(err, { fallback: '文件预览加载失败' })
  }
}

function toggleSupplement(row: SettlementSupplementRow) {
  if (!['draft', 'approved'].includes(row.status)) return
  selectedSupplementIds.value = selectedSupplementIds.value.includes(row.id)
    ? selectedSupplementIds.value.filter((id) => id !== row.id)
    : [...selectedSupplementIds.value, row.id]
}

function toggleAllSupplements() {
  selectedSupplementIds.value = allSupplementsSelected.value ? [] : deletableSupplements.value.map((row) => row.id)
}

function openSingleDelete(row: SettlementSupplementRow) {
  selectedSupplementIds.value = [row.id]
  supplementDeleteReason.value = ''
  uploadError.value = ''
  deleteDialogMode.value = 'single'
  deleteDialogOpen.value = true
}

function openBatchDelete() {
  if (!selectedSupplementIds.value.length) {
    uploadError.value = '请先选择要删除的补录记录'
    return
  }
  supplementDeleteReason.value = ''
  uploadError.value = ''
  deleteDialogMode.value = 'batch'
  deleteDialogOpen.value = true
}

function closeDeleteDialog() {
  if (deletingSupplements.value) return
  deleteDialogOpen.value = false
  supplementDeleteReason.value = ''
  if (deleteDialogMode.value === 'single') selectedSupplementIds.value = []
}

async function deleteSelectedSupplements() {
  const reason = supplementDeleteReason.value.trim()
  if (!selectedSupplementIds.value.length) {
    uploadError.value = '请先选择要删除的补录记录'
    return
  }
  if (!reason) {
    uploadError.value = '请填写删除原因'
    return
  }
  deletingSupplements.value = true
  uploadError.value = ''
  uploadNotice.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteSettlementSupplements(selectedSupplementIds.value, reason)
    uploadNotice.value = `已删除 ${formatInt(result.deleted_ids.length)} 条补录，关联文件和补录金额已同步移除。`
    selectedSupplementIds.value = []
    supplementDeleteReason.value = ''
    deleteDialogOpen.value = false
    await settlementRequest.run()
  } catch (err) {
    uploadError.value = resolveApiUserMessage(err, { fallback: '删除补录失败；本次选择没有产生部分删除。' })
  } finally {
    deletingSupplements.value = false
  }
}

async function load() {
  const [, directories] = await Promise.all([
    settlementRequest.run(),
    assetWorkbenchApi.listUploadDirectories().catch(() => [] as UploadDirectoryRow[]),
  ])
  uploadDirectories.value = directories
  if (!selectedDirectoryId.value && directories[0]) selectedDirectoryId.value = directories[0].id
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="aw-page-stack aw-simple-income">
    <div class="aw-console-hero">
      <div class="aw-console-hero__head">
        <div>
          <p class="aw-eyebrow">我的收入</p>
          <h1 class="aw-console-hero__title">这个月能拿</h1>
        </div>
        <button class="aw-console-button" type="button" :disabled="loading" @click="load">
          <RefreshCw :size="16" aria-hidden="true" />
          <span>刷新</span>
        </button>
      </div>
      <div class="aw-simple-income__amount">{{ currentAmount }}</div>
      <p class="aw-simple-income__hint">这里只显示你的金额。每个月固定两条：正常作品工资、补交作品工资。</p>
    </div>

    <div class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">按月查看</p>
          <h3>我的明细</h3>
        </div>
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载我的收入"
        @retry="load"
      >
        <div v-if="months.length" class="aw-simple-income__list">
          <article v-for="month in months" :key="month.business_month" class="aw-simple-income__row">
            <header class="aw-simple-income__month">
              <div>
                <strong>{{ formatMonthLabel(month.business_month) }}</strong>
                <small>{{ month.confirmed ? '这个月已经确认' : '这个月还在累计' }}</small>
              </div>
              <span class="aw-chip" :class="month.confirmed ? 'aw-chip--success' : 'aw-chip--warn'">
                {{ month.confirmed ? '已确认' : '进行中' }}
              </span>
            </header>

            <div class="aw-pay-slip-list" aria-label="工资条">
              <section class="aw-pay-slip">
                <div class="aw-pay-slip__icon" aria-hidden="true">
                  <WalletCards :size="20" />
                </div>
                <div class="aw-pay-slip__copy">
                  <strong>正常作品工资</strong>
                  <span>{{ formatInt(month.item_count) }} 个作品 · {{ formatInt(month.page_count) }} 张</span>
                  <small v-if="month.deduction_amount > 0">已扣 {{ formatMoney(month.deduction_amount) }}</small>
                </div>
                <b class="aw-cell-money">{{ formatMoney(normalPieceworkAmount(month)) }}</b>
              </section>

              <section class="aw-pay-slip">
                <div class="aw-pay-slip__icon aw-pay-slip__icon--supplement" aria-hidden="true">
                  <Gift :size="20" />
                </div>
                <div class="aw-pay-slip__copy">
                  <strong>补交作品工资</strong>
                  <span>漏交后补的作品会单独列在这里</span>
                  <small>{{ supplementLabel(month) }}</small>
                </div>
                <b class="aw-cell-money">{{ formatMoney(supplementAmount(month)) }}</b>
              </section>
            </div>

            <footer class="aw-simple-income__total">
              <span>这个月合计</span>
              <strong>{{ formatMoney(month.net_amount) }}</strong>
            </footer>
          </article>
        </div>
        <div v-else class="aw-empty-state">
          <h3>还没有收入记录</h3>
          <p>交过作品后，这里会显示每个月的金额。</p>
        </div>
      </AsyncBoundary>
    </div>

    <div class="aw-data-surface aw-self-supplement-upload">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">补录上传</p>
          <h3>补交遗漏作品</h3>
          <p class="aw-copy">管理员开放当月权限后，可按正常上传目录选择计价分类；补录工资不会进入正常作品工资。</p>
        </div>
        <span class="aw-chip" :class="supplementUploadOpen ? 'aw-chip--success' : 'aw-chip--neutral'">
          {{ supplementUploadOpen ? `已开放 ${supplementPermission?.business_month}` : '当前未开放' }}
        </span>
      </div>

      <div v-if="supplementUploadOpen" class="aw-form-grid">
        <label>
          补录日期
          <input v-model="supplementDate" type="date" aria-describedby="supplement-date-help" />
          <span id="supplement-date-help" class="aw-copy">指作品原本应该上传、但因遗漏未上传的日期。</span>
        </label>
        <label>
          上传目录 / 计价分类
          <select v-model.number="selectedDirectoryId">
            <option :value="0" disabled>请选择上传目录</option>
            <option v-for="directory in uploadDirectories" :key="directory.id" :value="directory.id">
              {{ directory.name }} · {{ directory.difficulty_class }}类
            </option>
          </select>
        </label>
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
              <button class="aw-secondary-button" type="button" @click="fileInput?.click()">选择文件</button>
              <button class="aw-secondary-button" type="button" @click="folderInput?.click()">
                <FolderOpen :size="15" aria-hidden="true" />
                选择文件夹
              </button>
            </div>
          </div>
          <input ref="fileInput" class="aw-visually-hidden" type="file" :accept="supplementAcceptString" multiple aria-label="选择补录文件" @change="selectSupplementFiles" />
          <input ref="folderInput" class="aw-visually-hidden" type="file" :accept="supplementAcceptString" multiple webkitdirectory directory aria-label="选择补录文件夹" @change="selectSupplementFiles" />
          <span v-if="fileSelectionNotice" class="aw-supplement-file-picker__notice">{{ fileSelectionNotice }}</span>
        </div>
      </div>
      <p v-else class="aw-inline-alert aw-inline-alert--info">当前自然月没有补录权限；请先联系管理员申请，管理员可随时开放或关闭。</p>

      <div v-if="supplementUploadOpen" class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="!canUploadSupplement" @click="uploadSupplements">
          <Upload :size="16" aria-hidden="true" />
          {{ uploading ? `上传中 ${uploadProgress}%` : `上传 ${formatInt(selectedSupplementGroups.length)} 个补录作品` }}
        </button>
        <span v-if="selectedDirectory" class="aw-copy">按 {{ selectedDirectory.difficulty_class }} 类自动计价</span>
      </div>
      <p v-if="duplicateFilenames.length" class="aw-inline-alert aw-inline-alert--warning" role="alert">
        同名提醒：{{ duplicateFilenames.join('、') }} 已存在或本次重复选择，请确认后再上传。
      </p>
      <p v-if="uploadError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ uploadError }}</p>
      <p v-if="uploadNotice" class="aw-inline-alert">{{ uploadNotice }}</p>
    </div>

    <div class="aw-data-surface aw-self-supplement-query">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">补录查询</p>
          <h3>我的补录记录</h3>
        </div>
        <span class="aw-chip aw-chip--neutral">{{ formatInt(activeSupplements.length) }} 条</span>
      </div>
      <div v-if="deletableSupplements.length" class="aw-grid-toolbar aw-supplement-delete-toolbar">
        <label class="aw-inline-check">
          <input type="checkbox" :checked="allSupplementsSelected" @change="toggleAllSupplements" />
          选择全部可删除补录
        </label>
        <span>已选 {{ formatInt(selectedSupplementIds.length) }} 条</span>
        <button
          v-if="selectedSupplementIds.length"
          class="aw-secondary-button aw-secondary-button--danger"
          type="button"
          @click="openBatchDelete"
        >
          <Trash2 :size="15" aria-hidden="true" />
          删除已选 {{ formatInt(selectedSupplementIds.length) }} 条
        </button>
      </div>
      <div v-if="activeSupplements.length" class="aw-simple-income__list">
        <article v-for="row in activeSupplements" :key="row.id" class="aw-simple-income__row">
          <header class="aw-simple-income__month">
            <label class="aw-inline-check aw-supplement-row__main">
              <input
                type="checkbox"
                :checked="selectedSupplementIds.includes(row.id)"
                :disabled="!['draft', 'approved'].includes(row.status)"
                @change="toggleSupplement(row)"
              />
              <span>
                <strong>{{ row.order_no }}</strong>
                <small>{{ row.supplement_date || '未填写日期' }} · {{ row.difficulty_class }}类 · {{ formatInt(row.page_count) }} 张</small>
              </span>
            </label>
            <div class="aw-supplement-row__summary">
              <b class="aw-cell-money">{{ formatMoney(row.gross_amount) }}</b>
              <button
                v-if="['draft', 'approved'].includes(row.status)"
                class="aw-secondary-button aw-secondary-button--danger aw-supplement-row__delete"
                type="button"
                :aria-label="`删除补录 ${row.order_no}`"
                @click="openSingleDelete(row)"
              >
                <Trash2 :size="15" aria-hidden="true" />
                删除
              </button>
            </div>
          </header>
          <div v-if="row.files?.length" class="aw-inline-actions">
            <button
              v-for="file in row.files"
              :key="file.id"
              class="aw-secondary-button"
              type="button"
              @click="openSupplementPreview(row, file)"
            >
              <Eye :size="15" aria-hidden="true" />
              预览 {{ file.display_name || file.original_filename }}
            </button>
          </div>
          <p v-else class="aw-copy">这是一条管理员手工补录记录，没有关联上传文件。</p>
        </article>
      </div>
      <div v-else class="aw-empty-state">
        <h3>当前月还没有补录记录</h3>
        <p>补录权限开放后，在上方选择日期、目录和文件即可上传。</p>
      </div>
    </div>

    <div v-if="deleteDialogOpen" class="aw-dialog-backdrop" role="presentation" @click.self="closeDeleteDialog">
      <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="supplement-delete-title" @keydown.esc.prevent="closeDeleteDialog">
        <div>
          <p class="aw-eyebrow">删除补录</p>
          <h3 id="supplement-delete-title">确认删除{{ deleteDialogMode === 'single' ? '这条' : '选中的' }}补录？</h3>
          <p class="aw-copy">关联补录文件会同步删除，补录金额会从本月未结算金额中移除。此操作不能撤销。</p>
        </div>
        <div class="aw-confirm-dialog__summary">
          <div>
            <span>记录数量</span>
            <strong>{{ formatInt(selectedSupplements.length) }} 条</strong>
          </div>
          <div>
            <span>移除金额</span>
            <strong>{{ formatMoney(selectedSupplementAmount) }}</strong>
          </div>
        </div>
        <label class="aw-field">
          <span>删除原因</span>
          <input v-model="supplementDeleteReason" autofocus placeholder="例如：上传错文件" />
        </label>
        <div class="aw-inline-actions">
          <button
            class="aw-secondary-button aw-secondary-button--danger"
            type="button"
            :disabled="deletingSupplements || !supplementDeleteReason.trim()"
            @click="deleteSelectedSupplements"
          >
            <Trash2 :size="15" aria-hidden="true" />
            {{ deletingSupplements ? '删除中' : `确认删除 ${formatInt(selectedSupplements.length)} 条` }}
          </button>
          <button class="aw-secondary-button" type="button" :disabled="deletingSupplements" @click="closeDeleteDialog">取消</button>
        </div>
      </section>
    </div>

    <WorkbenchPreviewDialog
      :open="previewDialog.open"
      :title="previewDialog.title"
      :preview-url="previewDialog.previewUrl"
      :mime-type="previewDialog.mimeType"
      :filename="previewDialog.filename"
      :empty-label="previewDialog.emptyLabel"
      :meta-rows="previewDialog.metaRows"
      eyebrow="补录文件预览"
      @close="previewDialog.open = false"
    />
  </section>
</template>
