<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Eye, Gift, RefreshCw, Upload, WalletCards } from 'lucide-vue-next'

import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { buildSelfSupplementPayload, duplicateSupplementFileNames } from '@aw/features/supplement/supplementUpload'
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
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const selectedDirectoryId = ref(0)
const supplementDate = ref(todayDate())
const selectedFiles = ref<File[]>([])
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadNotice = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
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
const duplicateFilenames = computed(() => duplicateSupplementFileNames(selectedFiles.value, supplements.value))
const supplementUploadOpen = computed(() => Boolean(supplementPermission.value?.enabled))
const canUploadSupplement = computed(() =>
  supplementUploadOpen.value && Boolean(selectedDirectory.value) && selectedFiles.value.length > 0 && Boolean(supplementDate.value) && !uploading.value,
)

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

function selectSupplementFiles(event: Event) {
  const input = event.target as HTMLInputElement | null
  selectedFiles.value = Array.from(input?.files ?? []).filter((file) => file.size > 0)
  uploadError.value = selectedFiles.value.length ? '' : '没有读取到可上传图片'
  uploadNotice.value = ''
}

async function uploadSupplements() {
  const permission = supplementPermission.value
  const directory = selectedDirectory.value
  if (!permission?.enabled || !directory || !supplementDate.value || !selectedFiles.value.length) return
  uploading.value = true
  uploadProgress.value = 0
  uploadError.value = ''
  uploadNotice.value = ''
  let success = 0
  const uploadFiles = [...selectedFiles.value]
  try {
    for (const [index, file] of uploadFiles.entries()) {
      const uploaded = await uploadWorkbenchFile(file, {
        uploadDirectoryId: directory.id,
        expectedBusinessMonth: permission.business_month,
        relativePath: file.name,
        onProgress: (progress) => {
          uploadProgress.value = Math.round(((index + progress.percent / 100) / uploadFiles.length) * 100)
        },
      })
      await assetWorkbenchApi.createSettlementSupplement(
        buildSelfSupplementPayload(file, uploaded.sessionId, supplementDate.value, permission, directory),
      )
      success += 1
    }
    uploadNotice.value = `补录上传完成：${formatInt(success)} 个作品，统一计入 ${permission.business_month} 补录工资。`
    selectedFiles.value = []
    if (fileInput.value) fileInput.value.value = ''
    await settlementRequest.run()
  } catch (err) {
    selectedFiles.value = uploadFiles.slice(success)
    uploadError.value = resolveApiUserMessage(err, { fallback: `补录上传中断，已完成 ${success} 个作品，请核对记录后重试。` })
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
          <input v-model="supplementDate" type="date" />
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
        <label>
          选择补录图片
          <input ref="fileInput" type="file" accept="image/*" multiple @change="selectSupplementFiles" />
        </label>
      </div>
      <p v-else class="aw-inline-alert aw-inline-alert--info">当前自然月没有补录权限；请先联系管理员申请，管理员可随时开放或关闭。</p>

      <div v-if="supplementUploadOpen" class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="!canUploadSupplement" @click="uploadSupplements">
          <Upload :size="16" aria-hidden="true" />
          {{ uploading ? `上传中 ${uploadProgress}%` : `上传 ${formatInt(selectedFiles.length)} 个补录作品` }}
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
        <span class="aw-chip aw-chip--neutral">{{ formatInt(supplements.length) }} 条</span>
      </div>
      <div v-if="supplements.length" class="aw-simple-income__list">
        <article v-for="row in supplements" :key="row.id" class="aw-simple-income__row">
          <header class="aw-simple-income__month">
            <div>
              <strong>{{ row.order_no }}</strong>
              <small>{{ row.supplement_date || '未填写日期' }} · {{ row.difficulty_class }}类 · {{ formatInt(row.page_count) }} 张</small>
            </div>
            <b class="aw-cell-money">{{ formatMoney(row.gross_amount) }}</b>
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
        <p>补录权限开放后，在上方选择日期、目录和图片即可上传。</p>
      </div>
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
