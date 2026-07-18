<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Eye, RefreshCw, Trash2 } from 'lucide-vue-next'

import { assetWorkbenchApi, type SettlementSupplementRow, type SubmissionFileRow } from '@aw/shared/api/assetWorkbenchApi'
import SettlementHubTabs from '@aw/shared/console/SettlementHubTabs.vue'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { chipClass, supplementStatusMeta } from '@aw/shared/format/status'
import PersonnelPicker from '@aw/shared/ui/PersonnelPicker.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const rows = ref<SettlementSupplementRow[]>([])
const total = ref(0)
const loading = ref(false)
const deleting = ref(false)
const error = ref('')
const notice = ref('')
const page = ref(1)
const pageSize = ref(20)
const businessMonth = ref(currentBusinessMonth())
const payeeUserId = ref(0)
const orderNo = ref('')
const status = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const selectedIds = ref<number[]>([])
const deleteReason = ref('')
const previewDialog = ref({
  open: false,
  title: '',
  previewUrl: '',
  mimeType: '',
  filename: '',
  emptyLabel: '',
  metaRows: [] as Array<[string, string]>,
})

const deletableRows = computed(() => rows.value.filter((row) => ['draft', 'approved'].includes(row.status)))
const selectedRows = computed(() => rows.value.filter((row) => selectedIds.value.includes(row.id)))
const allDeletableSelected = computed(() => deletableRows.value.length > 0 && deletableRows.value.every((row) => selectedIds.value.includes(row.id)))
const canPrev = computed(() => page.value > 1)
const canNext = computed(() => page.value * pageSize.value < total.value)

function buildParams() {
  const params: Record<string, unknown> = {
    page: page.value,
    page_size: pageSize.value,
    sort_by: 'supplement_date',
    sort_dir: 'desc',
  }
  if (businessMonth.value) params.business_month = businessMonth.value
  if (payeeUserId.value > 0) params.payee_user_id = payeeUserId.value
  if (orderNo.value.trim()) params.order_no = orderNo.value.trim()
  if (status.value) params.status = status.value
  if (dateFrom.value) params.supplement_date_from = dateFrom.value
  if (dateTo.value) params.supplement_date_to = dateTo.value
  return params
}

async function load(resetPage = false) {
  if (resetPage) page.value = 1
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.listSettlementSupplements(buildParams())
    rows.value = result.items
    total.value = result.total
    selectedIds.value = selectedIds.value.filter((id) => result.items.some((row) => row.id === id && ['draft', 'approved'].includes(row.status)))
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '补录查询失败' })
  } finally {
    loading.value = false
  }
}

async function resetQuery() {
  businessMonth.value = currentBusinessMonth()
  payeeUserId.value = 0
  orderNo.value = ''
  status.value = ''
  dateFrom.value = ''
  dateTo.value = ''
  pageSize.value = 20
  selectedIds.value = []
  await load(true)
}

function toggleRow(row: SettlementSupplementRow) {
  if (!['draft', 'approved'].includes(row.status)) return
  selectedIds.value = selectedIds.value.includes(row.id)
    ? selectedIds.value.filter((id) => id !== row.id)
    : [...selectedIds.value, row.id]
}

function toggleAll() {
  selectedIds.value = allDeletableSelected.value ? [] : deletableRows.value.map((row) => row.id)
}

function selectOnly(row: SettlementSupplementRow) {
  selectedIds.value = [row.id]
  deleteReason.value = ''
}

async function deleteSelected() {
  const reason = deleteReason.value.trim()
  if (!selectedIds.value.length) {
    error.value = '请先选择要删除的补录记录'
    return
  }
  if (!reason) {
    error.value = '请填写删除原因'
    return
  }
  deleting.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteSettlementSupplements(selectedIds.value, reason)
    notice.value = `已删除 ${formatInt(result.deleted_ids.length)} 条补录，关联文件和补录金额已同步移除。`
    selectedIds.value = []
    deleteReason.value = ''
    await load()
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '删除补录失败；本次选择没有产生部分删除。' })
  } finally {
    deleting.value = false
  }
}

async function changePage(delta: number) {
  const next = page.value + delta
  if (next < 1) return
  page.value = next
  await load()
}

async function openPreview(row: SettlementSupplementRow, file: SubmissionFileRow) {
  previewDialog.value = {
    open: true,
    title: file.display_name || file.original_filename || row.order_no,
    previewUrl: '',
    mimeType: file.mime_type,
    filename: file.original_filename,
    emptyLabel: '正在准备预览',
    metaRows: [['补录人员', String(row.payee_user_id)], ['补录日期', row.supplement_date || '—']],
  }
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    previewDialog.value.previewUrl = meta.preview_url || meta.download_url || ''
    previewDialog.value.mimeType = meta.mime_type || file.mime_type
    previewDialog.value.filename = meta.filename || file.original_filename
    previewDialog.value.emptyLabel = meta.preparing ? '预览正在生成，请稍后再试' : (meta.error || '当前文件暂不能预览')
  } catch (err) {
    previewDialog.value.emptyLabel = resolveApiUserMessage(err, { fallback: '文件预览加载失败' })
  }
}

onMounted(() => void load())
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-console-hero">
      <div class="aw-console-hero__head">
        <div>
          <p class="aw-eyebrow">补录模块</p>
          <h1 class="aw-console-hero__title">补录查询与删除</h1>
          <p class="aw-copy">按人员、日期、文件名和状态查询；删除时同步移除关联文件与未结算补录金额。</p>
        </div>
        <button class="aw-console-button" type="button" :disabled="loading" @click="load()">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
      </div>
    </div>

    <SettlementHubTabs />

    <div class="aw-data-surface">
      <div class="aw-form-grid">
        <label>结算月<input v-model="businessMonth" type="month" /></label>
        <PersonnelPicker v-model="payeeUserId" label="补录人员" hint="可按姓名或人员编码查找" @selected="load(true)" />
        <label>文件/作品名称<input v-model="orderNo" placeholder="精确文件名" @keyup.enter="load(true)" /></label>
        <label>
          状态
          <select v-model="status">
            <option value="">全部状态</option>
            <option value="draft">草稿</option>
            <option value="approved">已批准</option>
            <option value="in_batch">批次中</option>
            <option value="settled">已结算</option>
            <option value="voided">已作废</option>
          </select>
        </label>
        <label>补录日期起<input v-model="dateFrom" type="date" /></label>
        <label>补录日期止<input v-model="dateTo" type="date" /></label>
      </div>
      <div class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="loading" @click="load(true)">查询</button>
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="resetQuery">重置</button>
        <span class="aw-chip aw-chip--neutral">共 {{ formatInt(total) }} 条</span>
      </div>
    </div>

    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
    <p v-if="notice" class="aw-inline-alert" role="status">{{ notice }}</p>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <label class="aw-inline-check">
          <input type="checkbox" :checked="allDeletableSelected" :disabled="!deletableRows.length" @change="toggleAll" />
          选择本页可删除记录
        </label>
        <span>已选 {{ formatInt(selectedIds.length) }} 条</span>
      </div>

      <div v-if="rows.length" class="aw-simple-income__list">
        <article v-for="row in rows" :key="row.id" class="aw-simple-income__row">
          <header class="aw-simple-income__month">
            <label class="aw-inline-check aw-supplement-row__main">
              <input
                type="checkbox"
                :checked="selectedIds.includes(row.id)"
                :disabled="!['draft', 'approved'].includes(row.status)"
                @change="toggleRow(row)"
              />
              <span>
                <strong>{{ row.order_no }}</strong>
                <small>人员 {{ row.payee_user_id }} · {{ row.supplement_date || '未填写日期' }} · {{ row.difficulty_class }}类 · {{ formatInt(row.page_count) }} 张</small>
              </span>
            </label>
            <div class="aw-supplement-row__summary">
              <span :class="chipClass(supplementStatusMeta(row.status).tone)">{{ supplementStatusMeta(row.status).label }}</span>
              <b class="aw-cell-money">{{ formatMoney(row.gross_amount) }}</b>
            </div>
          </header>
          <div v-if="row.files?.length" class="aw-inline-actions">
            <button v-for="file in row.files" :key="file.id" class="aw-secondary-button" type="button" @click="openPreview(row, file)">
              <Eye :size="15" aria-hidden="true" />
              {{ file.display_name || file.original_filename }}
            </button>
          </div>
          <p v-else class="aw-copy">手工补录，无关联文件</p>
          <button
            v-if="['draft', 'approved'].includes(row.status)"
            class="aw-secondary-button"
            type="button"
            @click="selectOnly(row)"
          >
            <Trash2 :size="15" aria-hidden="true" />
            删除此条
          </button>
        </article>
      </div>
      <div v-else class="aw-empty-state"><h3>没有匹配的补录记录</h3><p>调整查询条件后重试。</p></div>

      <div v-if="total > 0" class="aw-drive-pager">
        <button class="aw-secondary-button" type="button" :disabled="!canPrev || loading" @click="changePage(-1)">上一页</button>
        <span>第 {{ formatInt(page) }} 页</span>
        <button class="aw-secondary-button" type="button" :disabled="!canNext || loading" @click="changePage(1)">下一页</button>
      </div>
    </div>

    <div v-if="selectedRows.length" class="aw-data-surface">
      <div class="aw-panel__head">
        <div><h3>删除选中的补录</h3><p class="aw-copy">此操作会同步删除关联文件，并从未结算补录金额中移除。</p></div>
        <span class="aw-chip aw-chip--warn">{{ formatInt(selectedRows.length) }} 条</span>
      </div>
      <label class="aw-field"><span>删除原因</span><input v-model="deleteReason" placeholder="例如：客户端上传错文件" /></label>
      <div class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="deleting" @click="deleteSelected">
          {{ deleting ? '删除中' : `确认删除 ${formatInt(selectedRows.length)} 条` }}
        </button>
        <button class="aw-secondary-button" type="button" :disabled="deleting" @click="selectedIds = []">取消选择</button>
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
