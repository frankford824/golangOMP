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
const filtersOpen = ref(false)
const selectedIds = ref<number[]>([])
const deleteReason = ref('')
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

const deletableRows = computed(() => rows.value.filter((row) => ['draft', 'approved'].includes(row.status)))
const selectedRows = computed(() => rows.value.filter((row) => selectedIds.value.includes(row.id)))
const selectedAmount = computed(() => selectedRows.value.reduce((sum, row) => sum + row.gross_amount, 0))
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

function openSingleDelete(row: SettlementSupplementRow) {
  selectedIds.value = [row.id]
  deleteReason.value = ''
  error.value = ''
  deleteDialogMode.value = 'single'
  deleteDialogOpen.value = true
}

function openBatchDelete() {
  if (!selectedIds.value.length) {
    error.value = '请先选择要删除的补录记录'
    return
  }
  deleteReason.value = ''
  error.value = ''
  deleteDialogMode.value = 'batch'
  deleteDialogOpen.value = true
}

function closeDeleteDialog() {
  if (deleting.value) return
  deleteDialogOpen.value = false
  deleteReason.value = ''
  if (deleteDialogMode.value === 'single') selectedIds.value = []
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
    deleteDialogOpen.value = false
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
  <section class="aw-page-stack aw-supplement-console">
    <header class="aw-supplement-console__header">
      <div>
        <h1>补录查询与删除</h1>
      </div>
      <div class="aw-supplement-console__header-actions">
        <span>{{ formatInt(total) }} 条记录</span>
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="load()">
          <RefreshCw :size="15" aria-hidden="true" />
          {{ loading ? '刷新中' : '刷新' }}
        </button>
      </div>
    </header>

    <SettlementHubTabs />

    <div class="aw-supplement-console__workbench">
      <button
        class="aw-supplement-console__filter-toggle"
        type="button"
        :aria-expanded="filtersOpen"
        @click="filtersOpen = !filtersOpen"
      >
        <span>筛选条件</span>
        <span>{{ filtersOpen ? '收起' : '展开' }}</span>
      </button>
      <form class="aw-supplement-filter-bar" :class="{ 'is-open': filtersOpen }" @submit.prevent="load(true)">
        <label>
          <span>结算月</span>
          <input v-model="businessMonth" type="month" />
        </label>
        <PersonnelPicker
          v-model="payeeUserId"
          label="补录人员"
          hint="姓名或人员编码"
          clearable
          compact
          @selected="load(true)"
          @cleared="load(true)"
        />
        <label class="aw-supplement-filter-bar__name">
          <span>文件/作品名称</span>
          <input v-model="orderNo" placeholder="输入文件名" />
        </label>
        <label>
          <span>状态</span>
          <select v-model="status">
            <option value="">全部状态</option>
            <option value="draft">草稿</option>
            <option value="approved">已批准</option>
            <option value="in_batch">批次中</option>
            <option value="settled">已结算</option>
            <option value="voided">已作废</option>
          </select>
        </label>
        <fieldset class="aw-supplement-filter-bar__dates">
          <legend>补录日期</legend>
          <input v-model="dateFrom" type="date" aria-label="补录日期起" />
          <span>至</span>
          <input v-model="dateTo" type="date" aria-label="补录日期止" />
        </fieldset>
        <div class="aw-supplement-filter-bar__actions">
          <button class="aw-primary-button" type="submit" :disabled="loading">查询</button>
          <button class="aw-secondary-button" type="button" :disabled="loading" @click="resetQuery">重置</button>
        </div>
      </form>

      <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
      <p v-if="notice" class="aw-inline-alert" role="status">{{ notice }}</p>

      <div class="aw-grid-toolbar aw-supplement-delete-toolbar">
        <label class="aw-inline-check">
          <input type="checkbox" :checked="allDeletableSelected" :disabled="!deletableRows.length" @change="toggleAll" />
          选择本页可删除记录
        </label>
        <span>已选 {{ formatInt(selectedIds.length) }} 条</span>
        <button
          v-if="selectedIds.length"
          class="aw-secondary-button aw-secondary-button--danger"
          type="button"
          @click="openBatchDelete"
        >
          <Trash2 :size="15" aria-hidden="true" />
          删除已选 {{ formatInt(selectedIds.length) }} 条
        </button>
      </div>

      <div class="aw-supplement-table-wrap">
        <table class="aw-supplement-table">
          <thead>
            <tr>
              <th class="aw-supplement-table__check"><span class="aw-sr-only">选择</span></th>
              <th>文件 / 作品</th>
              <th>补录人员</th>
              <th>补录日期</th>
              <th>分类 / 张数</th>
              <th>状态</th>
              <th class="aw-supplement-table__money">金额</th>
              <th class="aw-supplement-table__action">操作</th>
            </tr>
          </thead>
          <tbody v-if="rows.length">
            <tr v-for="row in rows" :key="row.id" :class="{ 'is-selected': selectedIds.includes(row.id) }">
              <td class="aw-supplement-table__check">
                <input
                  type="checkbox"
                  :aria-label="`选择补录 ${row.order_no}`"
                  :checked="selectedIds.includes(row.id)"
                  :disabled="!['draft', 'approved'].includes(row.status)"
                  @change="toggleRow(row)"
                />
              </td>
              <td>
                <strong class="aw-supplement-table__filename">{{ row.order_no }}</strong>
                <div v-if="row.files?.length" class="aw-supplement-table__files">
                  <button v-for="file in row.files" :key="file.id" type="button" @click="openPreview(row, file)">
                    <Eye :size="13" aria-hidden="true" />
                    {{ file.display_name || file.original_filename }}
                  </button>
                </div>
                <small v-else>手工补录，无关联文件</small>
              </td>
              <td><span class="aw-supplement-table__person">人员 {{ row.payee_user_id }}</span></td>
              <td>{{ row.supplement_date || '未填写' }}</td>
              <td>{{ row.difficulty_class }} 类 · {{ formatInt(row.page_count) }} 张</td>
              <td><span :class="chipClass(supplementStatusMeta(row.status).tone)">{{ supplementStatusMeta(row.status).label }}</span></td>
              <td class="aw-supplement-table__money"><strong>{{ formatMoney(row.gross_amount) }}</strong></td>
              <td class="aw-supplement-table__action">
                <button
                  v-if="['draft', 'approved'].includes(row.status)"
                  class="aw-supplement-table__delete"
                  type="button"
                  :aria-label="`删除补录 ${row.order_no}`"
                  @click="openSingleDelete(row)"
                >
                  <Trash2 :size="14" aria-hidden="true" />
                  删除
                </button>
                <span v-else>不可删除</span>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!rows.length" class="aw-empty-state">
          <h3>没有匹配的补录记录</h3>
          <p>调整查询条件后重试。</p>
        </div>
      </div>

      <footer v-if="total > 0" class="aw-supplement-console__footer">
        <span>共 {{ formatInt(total) }} 条</span>
        <label>
          每页
          <select v-model.number="pageSize" @change="load(true)">
            <option :value="20">20 条</option>
            <option :value="50">50 条</option>
            <option :value="100">100 条</option>
          </select>
        </label>
        <div class="aw-drive-pager">
          <button class="aw-secondary-button" type="button" :disabled="!canPrev || loading" @click="changePage(-1)">上一页</button>
          <span>第 {{ formatInt(page) }} 页</span>
          <button class="aw-secondary-button" type="button" :disabled="!canNext || loading" @click="changePage(1)">下一页</button>
        </div>
      </footer>
    </div>

    <div v-if="deleteDialogOpen" class="aw-dialog-backdrop" role="presentation" @click.self="closeDeleteDialog">
      <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="admin-supplement-delete-title" @keydown.esc.prevent="closeDeleteDialog">
        <div>
          <p class="aw-eyebrow">删除补录</p>
          <h3 id="admin-supplement-delete-title">确认删除{{ deleteDialogMode === 'single' ? '这条' : '选中的' }}补录？</h3>
          <p class="aw-copy">关联补录文件会同步删除，补录金额会从未结算金额中移除。此操作不能撤销。</p>
        </div>
        <div class="aw-confirm-dialog__summary">
          <div><span>记录数量</span><strong>{{ formatInt(selectedRows.length) }} 条</strong></div>
          <div><span>移除金额</span><strong>{{ formatMoney(selectedAmount) }}</strong></div>
        </div>
        <label class="aw-field"><span>删除原因</span><input v-model="deleteReason" autofocus placeholder="例如：客户端上传错文件" /></label>
        <div class="aw-inline-actions">
          <button
            class="aw-secondary-button aw-secondary-button--danger"
            type="button"
            :disabled="deleting || !deleteReason.trim()"
            @click="deleteSelected"
          >
            <Trash2 :size="15" aria-hidden="true" />
            {{ deleting ? '删除中' : `确认删除 ${formatInt(selectedRows.length)} 条` }}
          </button>
          <button class="aw-secondary-button" type="button" :disabled="deleting" @click="closeDeleteDialog">取消</button>
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
