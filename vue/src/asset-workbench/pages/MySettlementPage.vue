<script setup lang="ts">
import SupplementUploadPanel from '@aw/features/supplement/SupplementUploadPanel.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { HttpAppError } from '@/services/http'
import { Eye, Gift, RefreshCw, Trash2, WalletCards } from 'lucide-vue-next'

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
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const settlementRequest = usePageRequest<MySettlementResult>(
  async (signal) => {
    try { return await assetWorkbenchApi.mySettlement(signal) }
    catch (err) {
      if ([401, 403].includes((err as HttpAppError).status ?? 0)) {
        settlementRequest.reset()
        accessClosed.value = true
      }
      throw err
    }
  },
  null,
  '收入加载失败',
)
const loading = settlementRequest.loading
const error = settlementRequest.error
const result = settlementRequest.data
const accessClosed = ref(false)
let refreshTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const updatedLabel = computed(() => formatShanghaiDateTime(result.value?.generated_at, '尚未更新'))

function settlementStateLabel(month: MySettlementMonthRow) {
  return { accumulating: '累计中', pending_confirmation: '待确认', partially_confirmed: '部分确认', confirmed: '已确认' }[month.settlement_state ?? (month.confirmed ? 'confirmed' : 'accumulating')]
}

async function refreshIncome() {
  if (document.visibilityState === 'hidden' || loading.value || accessClosed.value || disposed) return
  await settlementRequest.run()
}
function resumeIncome() { void refreshIncome() }
async function pollIncome() {
  await refreshIncome()
  if (!disposed && !accessClosed.value) refreshTimer = setTimeout(pollIncome, Math.max(15, result.value?.refresh_after_seconds ?? 30) * 1000)
}

const currentAmount = computed(() => formatMoney(result.value?.estimated_net_amount ?? 0))
const months = computed(() => result.value?.months ?? [])
const supplementPermissions = computed(() => (result.value?.supplement_permissions
  ?? (result.value?.supplement_permission ? [result.value.supplement_permission] : []))
  .filter((row) => row.enabled).sort((a, b) => b.business_month.localeCompare(a.business_month)))
const supplements = computed(() => result.value?.supplements ?? [])
const activeSupplements = computed(() => supplements.value.filter((row) => row.status !== 'voided'))
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const uploadError = ref('')
const uploadNotice = ref('')
const selectedSupplementIds = ref<number[]>([])
const supplementDeleteReason = ref('')
const deletingSupplements = ref(false)
const deleteDialogOpen = ref(false)
const deleteDialogMode = ref<'single' | 'batch'>('batch')
const previewDialog = ref({
  open: false,
  title: '',
  previewUrl: '',
  mimeType: '',
  filename: '',
  emptyLabel: '',
  metaRows: [] as Array<[string, string]>,
})
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
  return (month.actual_net_amount ?? month.net_amount) - supplementAmount(month)
}

function supplementAmount(month: MySettlementMonthRow) {
  return month.supplement_amount - (month.supplement_actual_deduction_amount ?? 0)
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
}

onMounted(() => {
  void load()
  refreshTimer = setTimeout(pollIncome, 30_000)
  window.addEventListener('focus', resumeIncome)
  document.addEventListener('visibilitychange', resumeIncome)
})
onBeforeUnmount(() => {
  disposed = true
  clearTimeout(refreshTimer)
  window.removeEventListener('focus', resumeIncome)
  document.removeEventListener('visibilitychange', resumeIncome)
})
</script>

<template>
  <section class="aw-page-stack aw-simple-income">
    <div class="aw-console-hero">
      <div class="aw-console-hero__head">
        <div>
          <p class="aw-eyebrow">{{ result?.current_month && !accessClosed ? '结算月 · ' + formatMonthLabel(result.current_month) : '我的收入' }}</p>
          <h1 class="aw-console-hero__title">本月预计收入</h1>
        </div>
        <button class="aw-console-button" type="button" :disabled="loading" @click="load">
          <RefreshCw :size="16" aria-hidden="true" />
          <span>刷新</span>
        </button>
      </div>
      <div v-if="!accessClosed" class="aw-simple-income__amount">{{ result ? currentAmount : '—' }}</div>
      <p v-if="accessClosed" role="alert">结算页面暂未开放，请联系管理员。</p>
      <p v-else class="aw-simple-income__hint">预估不等于已发放；已确认也不代表实际到账。页面可见时约每30秒更新，回到页面时刷新。</p>
      <p v-if="!accessClosed" class="aw-copy">北京时间更新于：{{ updatedLabel }}{{ error ? '；更新失败，当前为上次成功快照，请重试。' : '' }}</p>
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
                <small>{{ month.confirmed ? '本月记录已确认，不代表实际到账' : '包含累计中或待确认记录，金额仍可能变化' }}</small>
              </div>
              <span class="aw-chip" :class="month.confirmed ? 'aw-chip--success' : 'aw-chip--warn'">
                {{ settlementStateLabel(month) }}
              </span>
            </header>

            <div class="aw-pay-slip-list" aria-label="工资条">
              <section class="aw-pay-slip">
                <div class="aw-pay-slip__icon" aria-hidden="true">
                  <WalletCards :size="20" />
                </div>
                <div class="aw-pay-slip__copy">
                  <strong>正常作品工资</strong>
                  <span>{{ formatInt(month.item_count) }} 个作品 · {{ formatInt(month.page_count) }} 个计价单位</span>
                  <small v-if="month.deduction_amount - (month.supplement_deduction_amount ?? 0) > 0 || month.gross_amount - normalPieceworkAmount(month) + month.welfare_amount + month.adjustment_amount > 0.005">
                    质检应扣 {{ formatMoney(month.deduction_amount - (month.supplement_deduction_amount ?? 0)) }} · 实际扣款 {{ formatMoney(month.gross_amount + month.welfare_amount + month.adjustment_amount - normalPieceworkAmount(month)) }}
                  </small>
                  <small>计件毛额 {{ formatMoney(month.gross_amount) }} · 福利 {{ formatMoney(month.welfare_amount) }} · 调整 {{ formatMoney(month.adjustment_amount) }}</small>
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
                  <small>{{ formatInt(month.supplement_item_count ?? 0) }} 个作品 · {{ formatInt(month.supplement_page_count ?? 0) }} 个计价单位</small>
                  <small v-if="month.supplement_deduction_amount || month.supplement_actual_deduction_amount">
                    质检应扣 {{ formatMoney(month.supplement_deduction_amount ?? 0) }} · 实际扣款 {{ formatMoney(month.supplement_actual_deduction_amount ?? 0) }}
                  </small>
                </div>
                <b class="aw-cell-money">{{ formatMoney(supplementAmount(month)) }}</b>
              </section>
            </div>

            <p v-if="month.pending_pricing_count" class="aw-inline-alert">另有 {{ formatInt(month.pending_pricing_count) }} 个作品待定级或未计价，尚未计入金额，请联系管理员核对。</p>
            <p class="aw-copy">累计中 {{ formatMoney(month.estimated_amount ?? 0) }} · 待确认 {{ formatMoney(month.pending_confirmation_amount ?? 0) }} · 已确认 {{ formatMoney(month.confirmed_net_amount ?? 0) }}</p>
            <footer class="aw-simple-income__total">
              <span>应发净额 {{ formatMoney(month.net_amount) }}</span>
              <strong>实发净额 {{ formatMoney(month.actual_net_amount ?? month.net_amount) }}</strong>
            </footer>
          </article>
        </div>
        <div v-else class="aw-empty-state">
          <h3>还没有收入记录</h3>
          <p>交过作品后，这里会显示每个月的金额。</p>
        </div>
      </AsyncBoundary>
    </div>

    <SupplementUploadPanel :permissions="supplementPermissions" :directories="uploadDirectories" :supplements="supplements"
      :ready="!!result" :error="error" :access-closed="accessClosed" @refresh="load" @submitted="settlementRequest.run()" />

    <div class="aw-data-surface aw-self-supplement-query">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">补录查询</p>
          <h3>我的补录记录</h3>
        </div>
        <span v-if="result" class="aw-chip aw-chip--neutral">{{ formatInt(activeSupplements.length) }} 条</span>
      </div>
      <p v-if="uploadError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ uploadError }}</p>
      <p v-if="uploadNotice" class="aw-inline-alert">{{ uploadNotice }}</p>
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
      <p v-if="!result && !accessClosed" class="aw-inline-alert aw-inline-alert--info" role="status">{{ error ? '补录记录读取失败，请刷新重试。' : '正在读取补录记录…' }}</p>
      <div v-else-if="activeSupplements.length" class="aw-simple-income__list">
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
                <small>结算月 {{ row.business_month }} · {{ row.supplement_date || '未填写日期' }} · {{ row.difficulty_class }}类 · {{ formatInt(row.page_count) }} 张</small>
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
      <div v-else-if="result" class="aw-empty-state">
        <h3>还没有补录记录</h3>
        <p>补录权限开放后，在上方选择日期、目录和文件即可上传。</p>
      </div>
    </div>

    <div v-if="deleteDialogOpen" class="aw-dialog-backdrop" role="presentation" @click.self="closeDeleteDialog">
      <section class="aw-confirm-dialog aw-supplement-delete-dialog" role="dialog" aria-modal="true" aria-labelledby="supplement-delete-title" @keydown.esc.prevent="closeDeleteDialog">
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
        <section class="aw-supplement-delete-selection" aria-label="待删除作品">
          <h4>待删除作品</h4>
          <ul>
            <li v-for="row in selectedSupplements" :key="row.id">
              <strong>{{ row.order_no }}</strong>
              <small>{{ row.business_month }} 结算 · {{ formatMoney(row.gross_amount) }}</small>
            </li>
          </ul>
        </section>
        <label class="aw-field">
          <span>删除原因</span>
          <input v-model="supplementDeleteReason" autofocus placeholder="例如：上传错文件" />
        </label>
        <div class="aw-inline-actions">
          <button class="aw-secondary-button" type="button" :disabled="deletingSupplements" @click="closeDeleteDialog">取消</button>
          <button
            class="aw-secondary-button aw-secondary-button--danger"
            type="button"
            :disabled="deletingSupplements || !supplementDeleteReason.trim()"
            @click="deleteSelectedSupplements"
          >
            <Trash2 :size="15" aria-hidden="true" />
            {{ deletingSupplements ? '删除中' : `确认删除 ${formatInt(selectedSupplements.length)} 条` }}
          </button>
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
