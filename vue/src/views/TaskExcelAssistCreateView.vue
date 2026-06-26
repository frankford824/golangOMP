<template>
  <div class="excel-assist-view">
    <div class="excel-assist-card">
      <header class="page-header">
        <div>
          <h2 class="page-title">Excel 辅助创建任务</h2>
          <p class="page-subtitle">
            支持新款批量 SKU、新款单 SKU、采购单 SKU 与原款开发的 Excel 辅助创建。每次上传 Excel 仅创建 1 个任务。
          </p>
        </div>
        <BaseButton variant="secondary" size="sm" @click="goBack">返回任务中心</BaseButton>
      </header>

      <div class="flow-switch task-group-switch" role="tablist" aria-label="任务分组">
        <button
          v-for="opt in taskGroupOptions"
          :key="opt.value"
          type="button"
          role="tab"
          class="flow-tab"
          :class="{ active: taskGroup === opt.value }"
          :aria-selected="taskGroup === opt.value"
          @click="selectTaskGroup(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <div class="flow-switch" role="tablist" aria-label="Excel 辅助创建类型">
        <button
          v-for="opt in flowOptions"
          :key="opt.value"
          type="button"
          role="tab"
          class="flow-tab"
          :class="{ active: flow === opt.value }"
          :aria-selected="flow === opt.value"
          @click="switchFlow(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <p class="flow-label" aria-label="当前任务类型">
        {{ excelAssistFlowLabel(flow) }} · {{ currentFlowLaneLabel }}
      </p>

      <div class="layout-grid">
        <div class="main-column">
          <ExcelBatchSkuPanel
            v-if="flow === 'new_batch'"
            task-type="new_product_development"
            :hide-preview="true"
            @parsed="onBatchExcelParsed"
            @reset="onBatchExcelReset"
          />
          <ExcelSingleSkuPanel
            v-else-if="flow === 'new_single'"
            @parsed="onSingleExcelParsed"
            @reset="onSingleExcelReset"
          />
          <ExcelSingleSkuPanel
            v-else-if="flow === 'purchase_single'"
            task-type="purchase_task"
            @parsed="onPurchaseExcelParsed"
            @reset="onPurchaseExcelReset"
          />
          <ExcelSingleSkuPanel
            v-else
            task-type="original_product_development"
            @parsed="onOriginalExcelParsed"
            @reset="onOriginalExcelReset"
          />

          <section v-if="flow === 'new_batch' && previewRows.length > 0" class="preview-section">
            <div class="preview-header">
              <h3 class="section-title">解析预览</h3>
              <span class="preview-meta">
                合计 {{ previewRows.length }} 行 · 错误 {{ violations.length }} 条
              </span>
            </div>
            <div class="preview-table-wrap">
              <table class="preview-table">
                <thead>
                  <tr>
                    <th>行</th>
                    <th>产品名</th>
                    <th>设计要求</th>
                    <th>产品款式编码</th>
                    <th>参考图</th>
                    <th>错误</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(row, idx) in previewRows"
                    :key="`preview-npd-${idx}`"
                    :class="{ 'has-error': previewRowErrors(previewRowExcelNumber(row, idx)).length > 0 }"
                  >
                    <td>{{ previewRowExcelNumber(row, idx) }}</td>
                    <td>{{ row.product_name || '—' }}</td>
                    <td class="cell-ellipsis">{{ row.design_requirement || '—' }}</td>
                    <td>{{ row.product_i_id || '—' }}</td>
                    <td>
                      <div class="row-ref-editor">
                        <input
                          :ref="(el) => setBatchReferenceInput(idx, el)"
                          type="file"
                          class="hidden-input"
                          :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                          multiple
                          :disabled="batchRefUploadingRow === idx"
                          @change="handleBatchReferenceUpload(idx, $event)"
                        />
                        <div v-if="row.reference_file_refs?.length" class="ref-thumbs">
                          <span
                            v-for="(ref, refIdx) in row.reference_file_refs"
                            :key="ref.ref_id || `${idx}-${refIdx}`"
                            class="ref-thumb-item"
                            :title="ref.filename"
                          >
                            <img
                              v-if="isImageMime(ref.mime_type)"
                              :src="ref.download_url"
                              :alt="ref.filename"
                              class="ref-thumb-img"
                            />
                            <span v-else class="ref-thumb-file">{{ ref.filename }}</span>
                            <span class="ref-thumb-actions">
                              <button
                                type="button"
                                class="ref-action"
                                :disabled="idx === 0 || batchRefUploadingRow != null"
                                @click="moveBatchReference(idx, refIdx, -1)"
                              >
                                上移
                              </button>
                              <button
                                type="button"
                                class="ref-action"
                                :disabled="idx >= previewRows.length - 1 || batchRefUploadingRow != null"
                                @click="moveBatchReference(idx, refIdx, 1)"
                              >
                                下移
                              </button>
                              <button
                                type="button"
                                class="ref-action ref-action-danger"
                                :disabled="batchRefUploadingRow != null"
                                @click="removeBatchReference(idx, refIdx)"
                              >
                                删除
                              </button>
                            </span>
                          </span>
                        </div>
                        <span v-else class="empty-ref-text">未上传</span>
                        <button
                          type="button"
                          class="ref-upload-btn"
                          :disabled="batchRefUploadingRow != null"
                          @click="openBatchReferenceUpload(idx)"
                        >
                          {{ batchRefUploadingRow === idx ? '上传中...' : '补传参考图' }}
                        </button>
                      </div>
                    </td>
                    <td>
                      <span
                        v-for="err in previewRowErrors(previewRowExcelNumber(row, idx))"
                        :key="`${err.row}-${err.column}-${err.code}`"
                        class="err-tag"
                      >
                        {{ formatBatchViolationMessage(err) }}
                      </span>
                      <span v-if="previewRowErrors(previewRowExcelNumber(row, idx)).length === 0">—</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-if="batchRefAdjustStatus" class="batch-ref-status">{{ batchRefAdjustStatus }}</p>
            <p v-if="batchRefAdjustError" class="batch-ref-error">{{ batchRefAdjustError }}</p>
          </section>

          <section v-if="flow === 'purchase_single' && purchaseDraft" class="preview-section">
            <div class="preview-header">
              <h3 class="section-title">解析预览</h3>
              <span class="preview-meta">单任务 · 错误 {{ purchaseViolations.length }} 条</span>
            </div>
            <dl class="single-draft-grid">
              <div class="draft-row">
                <dt>产品款式编码</dt>
                <dd>{{ purchaseDraft.product_i_id || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>产品名称</dt>
                <dd>{{ purchaseDraft.product_name || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>数量</dt>
                <dd>{{ purchaseDraft.quantity ?? '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>规格尺寸</dt>
                <dd>{{ purchaseDraft.spec_text || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>Excel 备注</dt>
                <dd>{{ purchaseDraft.remark || '—' }}</dd>
              </div>
            </dl>
            <ul v-if="purchaseViolations.length > 0" class="single-violation-list">
              <li v-for="(err, idx) in purchaseViolations" :key="`${err.code}-${idx}`">
                <span v-if="err.row">第 {{ err.row }} 行 · </span>
                {{ err.column ? `${err.column} · ` : '' }}{{ err.message || err.code }}
              </li>
            </ul>
          </section>

          <section v-if="flow === 'original_single' && originalDraft" class="preview-section">
            <div class="preview-header">
              <h3 class="section-title">解析预览</h3>
              <span class="preview-meta">单任务 · 错误 {{ originalViolations.length }} 条</span>
            </div>
            <dl class="single-draft-grid">
              <div class="draft-row">
                <dt>SKU编码</dt>
                <dd>{{ originalDraft.sku_code || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>ERP商品名称</dt>
                <dd>{{ originalDraft.product_name || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>修改要求</dt>
                <dd>{{ originalDraft.change_request || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>规格尺寸</dt>
                <dd>{{ originalDraft.spec_text || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>类目</dt>
                <dd>
                  {{
                    originalDraft.category_name ||
                    originalDraft.category_code ||
                    '—'
                  }}
                </dd>
              </div>
              <div class="draft-row">
                <dt>Excel 备注</dt>
                <dd>{{ originalDraft.remark || '—' }}</dd>
              </div>
            </dl>
            <ul v-if="originalViolations.length > 0" class="single-violation-list">
              <li v-for="(err, idx) in originalViolations" :key="`${err.code}-${idx}`">
                <span v-if="err.row">第 {{ err.row }} 行 · </span>
                {{ err.column ? `${err.column} · ` : '' }}{{ err.message || err.code }}
              </li>
            </ul>
          </section>

          <section v-if="flow === 'new_single' && singleDraft" class="preview-section">
            <div class="preview-header">
              <h3 class="section-title">解析预览</h3>
              <span class="preview-meta">单任务 · 错误 {{ singleViolations.length }} 条</span>
            </div>
            <dl class="single-draft-grid">
              <div class="draft-row">
                <dt>产品款式编码</dt>
                <dd>{{ singleDraft.product_i_id || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>产品名称</dt>
                <dd>{{ singleDraft.product_name || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>设计要求</dt>
                <dd>{{ singleDraft.design_requirement || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>规格尺寸</dt>
                <dd>{{ singleDraft.spec_text || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>材质</dt>
                <dd>{{ singleDraft.material || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>材质备注</dt>
                <dd>{{ singleDraft.material_other || '—' }}</dd>
              </div>
              <div class="draft-row">
                <dt>Excel 备注</dt>
                <dd>{{ singleDraft.remark || '—' }}</dd>
              </div>
            </dl>
            <ul v-if="singleViolations.length > 0" class="single-violation-list">
              <li v-for="(err, idx) in singleViolations" :key="`${err.code}-${idx}`">
                <span v-if="err.row">第 {{ err.row }} 行 · </span>
                {{ err.column ? `${err.column} · ` : '' }}{{ err.message || err.code }}
              </li>
            </ul>
          </section>
        </div>

        <aside class="side-column">
          <section class="meta-card">
            <h3 class="section-title">任务信息</h3>
            <div v-if="!hideOwnerFields" class="field-group">
              <label class="field-label">所属组 <span class="required">*</span></label>
              <BaseSelect
                v-model="groupId"
                :options="groupOptions"
                placeholder="请选择所属组"
              />
            </div>
            <div class="field-group">
              <label class="field-label">任务截止时间 <span class="required">*</span></label>
              <div class="due-at-row">
                <input v-model="dueAtDate" type="date" class="native-input" :min="dueAtMin" />
                <select v-model="dueAtHour" class="native-input due-hour-select">
                  <option v-for="opt in dueHourOptions" :key="opt.value" :value="opt.value">
                    {{ opt.label }}
                  </option>
                </select>
              </div>
            </div>
            <div class="field-group">
              <BaseSelect
                v-if="isDeptAdminPlus"
                v-model="priority"
                label="优先级"
                :options="priorityOptions"
              />
              <label v-else class="urgent-toggle">
                <input v-model="urgentChecked" type="checkbox" />
                是否加急
              </label>
            </div>
            <div class="field-group">
              <BaseTextarea v-model="note" label="备注" :rows="2" placeholder="可选" />
            </div>
          </section>

          <section class="submit-card">
            <p v-if="submitError" class="error-banner">{{ submitError }}</p>
            <p class="submit-hint">
              {{
                canSubmit
                  ? '可以确认创建任务。'
                  : submitBlockReason || '请先完成 Excel 解析并满足必填项。'
              }}
            </p>
            <BaseButton
              variant="primary"
              class="submit-btn"
              :disabled="!canSubmit || submitting"
              :loading="submitting"
              @click="submit"
            >
              确认创建任务
            </BaseButton>
          </section>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import ExcelBatchSkuPanel from '@/components/task-create/ExcelBatchSkuPanel.vue'
import ExcelSingleSkuPanel from '@/components/task-create/ExcelSingleSkuPanel.vue'
import { useTasksStore } from '@/stores/tasks'
import { usePermissionsStore } from '@/stores/permissions'
import { useTeamOptions } from '@/composables/useTeamOptions'
import { useActorOwnerScope } from '@/composables/useActorOwnerScope'
import { useAuth } from '@/composables/useAuth'
import { formatBatchViolationMessage, type BatchPreviewRow, type BatchViolation, type ReferenceFileRef as BatchReferenceFileRef } from '@/services/api/batchSkuApi'
import type { SingleTaskExcelDraft, ExcelAssistViolation } from '@/services/api/excelAssistApi'
import type { Task } from '@/domain/types/task'
import type { TaskBatchItem } from '@/domain/types'
import { uploadReferenceFileRef } from '@/services/upload/assetUploadFlow'
import {
  canSubmitExcelAssistBatch,
  canSubmitExcelAssistOriginalSingle,
  canSubmitExcelAssistPurchaseSingle,
  canSubmitExcelAssistSingle,
  excelAssistFlowLabel,
  mapExcelPreviewToBatchItems,
  mapExcelPreviewToOriginalSingleTask,
  mapExcelPreviewToPurchaseSingleTask,
  mapExcelPreviewToSingleTask,
  type ExcelAssistFlow,
} from '@/domain/task-excel-assist'
import { normalizePriorityForApi } from '@/domain/task-priority'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import {
  getBeijingDateString,
  nowISO,
  taskBeijingDateKey,
  taskBeijingHour,
  toBeijingHourISO,
} from '@/utils/date'
import { formatUploadFailureMessage } from '@/utils/upload-errors'
import { generateActionId } from '@/utils/uuid'
import {
  REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  REFERENCE_UPLOAD_MAX_FILE_SIZE_MB,
  isAcceptableReferenceFile,
  referenceFileTooLargeMessage,
} from '@/domain/constants/reference-upload'
import { UPLOAD_ACCEPT_ATTRIBUTE, isAllowedUploadFile } from '@/domain/constants/upload-types'

const EXCEL_ASSIST_TASK_TYPE = 'new_product_development' as const

const router = useRouter()
const tasksStore = useTasksStore()
const permissionsStore = usePermissionsStore()
const { teamOptions: rawTeamOptions, resolveDepartmentByTeam } = useTeamOptions()
const { filterOwnerTeamOptions, validateOwnerScope, defaultOwnerTeam, hideOwnerFields } =
  useActorOwnerScope()
const { isDeptAdminPlus } = useAuth()

const flowOptions: { value: ExcelAssistFlow; label: string }[] = [
  { value: 'new_batch', label: '新款批量 SKU' },
  { value: 'new_single', label: '新款单 SKU' },
  { value: 'purchase_single', label: '采购单 SKU' },
  { value: 'original_single', label: '原款开发' },
]

type TaskGroup = 'normal' | 'customization'
const taskGroupOptions: { value: TaskGroup; label: string }[] = [
  { value: 'normal', label: '常规任务' },
  { value: 'customization', label: '定制任务' },
]
const taskGroup = ref<TaskGroup>('normal')
const flow = ref<ExcelAssistFlow>('new_batch')

const previewRows = ref<BatchPreviewRow[]>([])
const violations = ref<BatchViolation[]>([])
const batchItems = ref<TaskBatchItem[]>([])
const batchReferenceInputs = ref<Record<number, HTMLInputElement | null>>({})
const batchRefUploadingRow = ref<number | null>(null)
const batchRefAdjustStatus = ref('')
const batchRefAdjustError = ref('')

const singleDraft = ref<SingleTaskExcelDraft | null>(null)
const singleViolations = ref<ExcelAssistViolation[]>([])

const purchaseDraft = ref<SingleTaskExcelDraft | null>(null)
const purchaseViolations = ref<ExcelAssistViolation[]>([])

const originalDraft = ref<SingleTaskExcelDraft | null>(null)
const originalViolations = ref<ExcelAssistViolation[]>([])

const groupId = ref('')
const dueAt = ref<string | null>(null)
const note = ref('')
const priority = ref('normal')
const submitError = ref('')
const submitting = ref(false)
const actionId = ref(generateActionId())

const priorityOptions = [
  { value: 'low', label: '低' },
  { value: 'normal', label: '普通' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '加急' },
]

const groupOptions = computed(() =>
  filterOwnerTeamOptions(rawTeamOptions.value, resolveDepartmentByTeam),
)
const currentSkuCodeType = computed(() =>
  taskGroup.value === 'customization' ? 'customization' : 'regular',
)
const currentSkuPrefix = computed(() => (taskGroup.value === 'customization' ? 'DZ' : 'CG'))
const currentFlowLaneLabel = computed(() =>
  flow.value === 'original_single'
    ? taskGroup.value === 'customization'
      ? '定制归类'
      : '常规归类'
    : `${currentSkuPrefix.value} 编码`,
)

const dueAtHourFallback = 18
const dueHourOptions = Array.from({ length: 24 }, (_, hour) => ({
  value: String(hour),
  label: `${String(hour).padStart(2, '0')}:00`,
}))

const dueAtMin = computed(() => getBeijingDateString())

const dueAtDate = computed({
  get: () => taskBeijingDateKey(dueAt.value),
  set: (v: string) => {
    if (!v) {
      dueAt.value = null
      return
    }
    const parsed = Number.parseInt(dueAtHour.value, 10)
    const hour = Number.isFinite(parsed) ? parsed : dueAtHourFallback
    dueAt.value = toBeijingHourISO(v, hour)
  },
})

const dueAtHour = computed({
  get: () => {
    const hour = taskBeijingHour(dueAt.value)
    return String(hour ?? dueAtHourFallback)
  },
  set: (v: string) => {
    const parsed = Number.parseInt(v, 10)
    const hour =
      Number.isFinite(parsed) && parsed >= 0 && parsed <= 23 ? parsed : dueAtHourFallback
    const currentDate = taskBeijingDateKey(dueAt.value)
    if (!currentDate) return
    dueAt.value = toBeijingHourISO(currentDate, hour)
  },
})

const urgentChecked = computed({
  get: () => priority.value === 'critical',
  set: (checked: boolean) => {
    priority.value = checked ? 'critical' : 'normal'
  },
})

const canSubmit = computed(() => {
  if (flow.value === 'new_batch') {
    return canSubmitExcelAssistBatch({
      taskType: EXCEL_ASSIST_TASK_TYPE,
      batchItems: batchItems.value,
      violations: violations.value,
      groupId: groupId.value,
      dueAt: dueAt.value,
    })
  }
  if (flow.value === 'purchase_single') {
    return canSubmitExcelAssistPurchaseSingle({
      draft: purchaseDraft.value,
      violations: purchaseViolations.value,
      groupId: groupId.value,
      dueAt: dueAt.value,
    })
  }
  if (flow.value === 'original_single') {
    return canSubmitExcelAssistOriginalSingle({
      draft: originalDraft.value,
      violations: originalViolations.value,
      groupId: groupId.value,
      dueAt: dueAt.value,
    })
  }
  return canSubmitExcelAssistSingle({
    draft: singleDraft.value,
    violations: singleViolations.value,
    groupId: groupId.value,
    dueAt: dueAt.value,
  })
})

const submitBlockReason = computed(() => {
  if (!groupId.value.trim()) return '请选择所属组'
  if (!dueAt.value) return '请填写任务截止时间'
  if (flow.value === 'new_batch') {
    if (batchItems.value.length < 2) return '批量模式至少需要 2 行有效数据'
    if (violations.value.length > 0) return 'Excel 存在行级错误，请修正后重新上传'
    return ''
  }
  if (flow.value === 'purchase_single') {
    if (!purchaseDraft.value) return '请先上传并解析 Excel'
    if (purchaseViolations.value.length > 0) return 'Excel 存在错误，请修正后重新上传'
    return ''
  }
  if (flow.value === 'original_single') {
    if (!originalDraft.value) return '请先上传并解析 Excel'
    if (originalViolations.value.length > 0) return 'Excel 存在错误，请修正后重新上传'
    return ''
  }
  if (!singleDraft.value) return '请先上传并解析 Excel'
  if (singleViolations.value.length > 0) return 'Excel 存在错误，请修正后重新上传'
  return ''
})

watch(
  groupOptions,
  (opts) => {
    if (opts.length === 0) return
    const current = groupId.value
    const values = opts.map((o) => String(o.value))
    if (!current || !values.includes(current)) {
      const preferred =
        defaultOwnerTeam.value && values.includes(defaultOwnerTeam.value)
          ? opts.find((o) => String(o.value) === defaultOwnerTeam.value)!
          : opts[0]
      groupId.value = String(preferred.value)
    }
  },
  { immediate: true },
)

function switchFlow(next: ExcelAssistFlow) {
  if (flow.value === next) return
  flow.value = next
  resetAllExcelState()
}

function resetBatchExcelState() {
  previewRows.value = []
  violations.value = []
  batchItems.value = []
  batchRefAdjustStatus.value = ''
  batchRefAdjustError.value = ''
  batchReferenceInputs.value = {}
}

function resetSingleExcelState() {
  singleDraft.value = null
  singleViolations.value = []
}

function resetPurchaseExcelState() {
  purchaseDraft.value = null
  purchaseViolations.value = []
}

function resetOriginalExcelState() {
  originalDraft.value = null
  originalViolations.value = []
}

function resetAllExcelState() {
  resetBatchExcelState()
  resetSingleExcelState()
  resetPurchaseExcelState()
  resetOriginalExcelState()
  submitError.value = ''
  actionId.value = generateActionId()
}

function onBatchExcelParsed(payload: { preview: BatchPreviewRow[]; violations: BatchViolation[] }) {
  actionId.value = generateActionId()
  previewRows.value = payload.preview
  violations.value = payload.violations
  syncBatchItemsFromPreview()
  batchRefAdjustStatus.value = ''
  batchRefAdjustError.value = ''
  submitError.value = ''
}

function onBatchExcelReset() {
  resetBatchExcelState()
  submitError.value = ''
  actionId.value = generateActionId()
}

function onSingleExcelParsed(payload: {
  draft: SingleTaskExcelDraft
  violations: ExcelAssistViolation[]
}) {
  actionId.value = generateActionId()
  singleDraft.value = payload.draft
  singleViolations.value = payload.violations
  submitError.value = ''
}

function onSingleExcelReset() {
  resetSingleExcelState()
  submitError.value = ''
  actionId.value = generateActionId()
}

function onPurchaseExcelParsed(payload: {
  draft: SingleTaskExcelDraft
  violations: ExcelAssistViolation[]
}) {
  actionId.value = generateActionId()
  purchaseDraft.value = payload.draft
  purchaseViolations.value = payload.violations
  submitError.value = ''
}

function onPurchaseExcelReset() {
  resetPurchaseExcelState()
  submitError.value = ''
  actionId.value = generateActionId()
}

function onOriginalExcelParsed(payload: {
  draft: SingleTaskExcelDraft
  violations: ExcelAssistViolation[]
}) {
  actionId.value = generateActionId()
  originalDraft.value = payload.draft
  originalViolations.value = payload.violations
  submitError.value = ''
}

function onOriginalExcelReset() {
  resetOriginalExcelState()
  submitError.value = ''
  actionId.value = generateActionId()
}

function previewRowErrors(row: number): BatchViolation[] {
  return violations.value.filter((v) => v.row === row)
}

function previewRowExcelNumber(row: BatchPreviewRow, index: number): number {
  return row.source_row && row.source_row > 0 ? row.source_row : index + 2
}

function isImageMime(mimeType: string): boolean {
  return mimeType.startsWith('image/')
}

function syncBatchItemsFromPreview() {
  batchItems.value = mapExcelPreviewToBatchItems(EXCEL_ASSIST_TASK_TYPE, previewRows.value, {
    skuCodeType: currentSkuCodeType.value,
  })
}

watch(taskGroup, () => {
  batchItems.value = batchItems.value.map((item) => ({
    ...item,
    skuCodeType: currentSkuCodeType.value,
  }))
})

function setBatchReferenceInput(index: number, el: unknown) {
  batchReferenceInputs.value[index] = el instanceof HTMLInputElement ? el : null
}

function batchRowReferenceRefs(index: number): BatchReferenceFileRef[] {
  return [...(previewRows.value[index]?.reference_file_refs ?? [])]
}

function updateBatchRowReferenceRefs(index: number, refs: BatchReferenceFileRef[]) {
  previewRows.value = previewRows.value.map((row, idx) =>
    idx === index
      ? { ...row, reference_file_refs: refs.length > 0 ? refs : undefined }
      : row,
  )
  syncBatchItemsFromPreview()
}

function openBatchReferenceUpload(index: number) {
  batchRefAdjustError.value = ''
  batchReferenceInputs.value[index]?.click()
}

async function handleBatchReferenceUpload(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (!files.length) return
  if (batchRefUploadingRow.value != null) return
  batchRefAdjustStatus.value = ''
  batchRefAdjustError.value = ''

  const oversized = files.filter((f) => f.size > REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES)
  const unsupported = files.filter((f) => !isAllowedUploadFile(f.name))
  const validFiles = files.filter(
    (f) =>
      isAllowedUploadFile(f.name) &&
      isAcceptableReferenceFile(f) &&
      f.size <= REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  )
  const errors: string[] = []
  if (oversized.length > 0) {
    errors.push(
      oversized.length === 1
        ? referenceFileTooLargeMessage(oversized[0]?.name)
        : `有 ${oversized.length} 个文件超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_MB}MB，已拒绝上传`,
    )
  }
  if (unsupported.length > 0) {
    errors.push(
      unsupported.length === 1
        ? `不支持的文件类型：${unsupported[0]?.name ?? ''}`
        : `有 ${unsupported.length} 个文件类型不受支持，已拒绝上传`,
    )
  }
  if (errors.length > 0) {
    batchRefAdjustError.value = errors.join('；')
  }
  if (!validFiles.length) return

  batchRefUploadingRow.value = index
  try {
    const refs = batchRowReferenceRefs(index)
    for (const file of validFiles) {
      const uploaded = await uploadReferenceFileRef(file)
      refs.push(uploaded as unknown as BatchReferenceFileRef)
    }
    updateBatchRowReferenceRefs(index, refs)
    batchRefAdjustStatus.value = `第 ${index + 1} 行已补传 ${validFiles.length} 个参考图`
  } catch (err) {
    batchRefAdjustError.value = formatUploadFailureMessage('reference_upload', err)
  } finally {
    batchRefUploadingRow.value = null
  }
}

function removeBatchReference(rowIndex: number, refIndex: number) {
  const refs = batchRowReferenceRefs(rowIndex)
  if (refIndex < 0 || refIndex >= refs.length) return
  refs.splice(refIndex, 1)
  updateBatchRowReferenceRefs(rowIndex, refs)
  batchRefAdjustStatus.value = `已从第 ${rowIndex + 1} 行移除参考图`
  batchRefAdjustError.value = ''
}

function moveBatchReference(rowIndex: number, refIndex: number, offset: -1 | 1) {
  const targetIndex = rowIndex + offset
  if (targetIndex < 0 || targetIndex >= previewRows.value.length) return
  const sourceRefs = batchRowReferenceRefs(rowIndex)
  if (refIndex < 0 || refIndex >= sourceRefs.length) return
  const [ref] = sourceRefs.splice(refIndex, 1)
  const targetRefs = batchRowReferenceRefs(targetIndex)
  targetRefs.push(ref)
  previewRows.value = previewRows.value.map((row, idx) => {
    if (idx === rowIndex) {
      return { ...row, reference_file_refs: sourceRefs.length > 0 ? sourceRefs : undefined }
    }
    if (idx === targetIndex) {
      return { ...row, reference_file_refs: targetRefs }
    }
    return row
  })
  syncBatchItemsFromPreview()
  batchRefAdjustStatus.value = `参考图已移动到第 ${targetIndex + 1} 行`
  batchRefAdjustError.value = ''
}

function resolveOwnerDepartmentForSubmit(): string | undefined {
  const fromTeam = resolveDepartmentByTeam(groupId.value)
  if (fromTeam) return fromTeam
  const fromUser = permissionsStore.currentUser?.departmentId
  if (fromUser && fromUser !== '未分配') return fromUser
  return undefined
}

function resolveGroupName(): string {
  const opt = groupOptions.value.find((o) => String(o.value) === groupId.value)
  return opt?.label ? String(opt.label) : groupId.value
}

function goBack() {
  void router.push({ name: 'TaskList' })
}

function selectTaskGroup(group: TaskGroup) {
  taskGroup.value = group
}

function buildCommonTaskFields(forFlow: ExcelAssistFlow) {
  const currentUser = permissionsStore.currentUser
  const now = nowISO()
  const preflightOwnerDepartment = resolveOwnerDepartmentForSubmit()
  const isPurchase = forFlow === 'purchase_single'
  const isOriginal = forFlow === 'original_single'
  const businessType = isPurchase
    ? ('PURCHASE_TASK' as const)
    : isOriginal
      ? ('ORIGINAL_PRODUCT_DEV' as const)
      : ('NEW_PRODUCT_DEV' as const)
  const isCustomization = taskGroup.value === 'customization'
  return {
    businessLane: taskGroup.value,
    workflowLane: taskGroup.value,
    customizationRequired: isCustomization,
    customizationSourceType: isCustomization
      ? isOriginal
        ? 'existing_product'
        : 'new_product'
      : undefined,
    skuCodeType: currentSkuCodeType.value,
    status: 'PendingAssign' as const,
    groupId: groupId.value,
    groupName: resolveGroupName(),
    ownerDepartment: hideOwnerFields.value ? undefined : preflightOwnerDepartment,
    ownerOrgTeam: hideOwnerFields.value ? undefined : groupId.value,
    requesterId: currentUser?.id ?? 'anonymous',
    requesterName: currentUser?.name ?? '未知用户',
    creatorId: currentUser?.id ?? null,
    creatorName: currentUser?.name ?? null,
    dueAt: dueAt.value,
    priority: normalizePriorityForApi(priority.value),
    syncErpOnCreate: !isOriginal && !isPurchase ? true : isPurchase ? true : undefined,
    assetVersions: [] as unknown[],
    businessType,
    requiresAssetVersions: !isPurchase,
    createdAt: now,
    updatedAt: now,
    preflightOwnerDepartment,
  }
}

async function submit() {
  if (!canSubmit.value || submitting.value) return
  submitError.value = ''
  submitting.value = true

  const common = buildCommonTaskFields(flow.value)
  const ownerScopeDeny = validateOwnerScope({
    owner_department: common.preflightOwnerDepartment,
    owner_org_team: groupId.value,
    owner_team: groupId.value,
  })
  if (ownerScopeDeny) {
    submitError.value = ownerScopeDeny
    submitting.value = false
    return
  }

  let payload: Record<string, unknown>

  if (flow.value === 'new_batch') {
    payload = {
      taskType: 'NEW_PRODUCT_DEV',
      skuMode: 'multiple' as const,
      productName: '',
      productSource: 'new',
      note: note.value,
      batchItems: batchItems.value,
      batchExcelImported: true,
      ...common,
    }
    delete payload.preflightOwnerDepartment
  } else if (flow.value === 'purchase_single') {
    const mapped = mapExcelPreviewToPurchaseSingleTask({
      draft: purchaseDraft.value!,
      pageNote: note.value,
    })
    payload = {
      ...mapped,
      ...common,
    }
    delete payload.preflightOwnerDepartment
  } else if (flow.value === 'original_single') {
    const mapped = mapExcelPreviewToOriginalSingleTask({
      draft: originalDraft.value!,
      pageNote: note.value,
    })
    payload = {
      ...mapped,
      ...common,
    }
    delete payload.preflightOwnerDepartment
    delete payload.syncErpOnCreate
  } else {
    const mapped = mapExcelPreviewToSingleTask({
      draft: singleDraft.value!,
      pageNote: note.value,
    })
    payload = {
      ...mapped,
      ...common,
    }
    delete payload.preflightOwnerDepartment
  }

  try {
    const created = await tasksStore.addTask(payload as unknown as Partial<Task>, actionId.value)
    await tasksStore.loadTaskById(created.id)
    void router.push({ path: `/tasks/${created.id}` })
  } catch (err) {
    submitError.value = resolveApiUserMessage(err)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  if (!dueAt.value) {
    const today = getBeijingDateString()
    dueAt.value = toBeijingHourISO(today, dueAtHourFallback)
  }
})
</script>

<style scoped>
.excel-assist-view {
  padding: 1.25rem;
  max-width: 1280px;
  margin: 0 auto;
}

.excel-assist-card {
  background: var(--color-surface, rgb(var(--yb-surface)));
  border: 1px solid var(--color-border, rgb(var(--yb-border)));
  border-radius: 12px;
  padding: 1.25rem 1.5rem 1.5rem;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.page-title {
  margin: 0;
  font-size: 1.35rem;
  font-weight: 600;
}

.page-subtitle {
  margin: 0.35rem 0 0;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
  font-size: 0.875rem;
  line-height: 1.5;
}

.flow-switch {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.task-group-switch {
  margin-bottom: 0.5rem;
}

.flow-tab {
  border: 1px solid var(--color-border, rgb(var(--yb-border-strong)));
  border-radius: 8px;
  padding: 0.4rem 0.85rem;
  background: var(--color-surface, rgb(var(--yb-surface)));
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
}

.flow-tab.active {
  border-color: var(--color-primary, rgb(var(--yb-brand)));
  background: color-mix(in srgb, var(--color-primary, rgb(var(--yb-brand))) 8%, white);
  color: var(--color-primary, rgb(var(--yb-brand)));
  font-weight: 600;
}

.flow-label {
  display: inline-block;
  margin: 0 0 1.25rem;
  padding: 0.35rem 0.75rem;
  border: 1px solid var(--color-primary, rgb(var(--yb-brand)));
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-primary, rgb(var(--yb-brand))) 8%, white);
  color: var(--color-primary, rgb(var(--yb-brand)));
  font-size: 0.875rem;
  font-weight: 600;
}

.layout-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 1.25rem;
  align-items: start;
}

.main-column {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
}

.side-column {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.section-title {
  margin: 0 0 0.75rem;
  font-size: 0.95rem;
  font-weight: 600;
}

.preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.preview-meta {
  font-size: 0.8rem;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
}

.preview-table-wrap {
  overflow: auto;
  border: 1px solid var(--color-border, rgb(var(--yb-border)));
  border-radius: 8px;
}

.preview-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.preview-table th,
.preview-table td {
  padding: 0.5rem 0.6rem;
  border-bottom: 1px solid var(--color-border, rgb(var(--yb-surface-muted)));
  text-align: left;
  vertical-align: top;
}

.preview-table th {
  background: var(--color-surface-muted, rgb(var(--yb-surface-soft)));
  font-weight: 600;
  white-space: nowrap;
}

.preview-table tr.has-error {
  background: color-mix(in srgb, rgb(var(--yb-red)) 6%, white);
}

.cell-ellipsis {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.single-draft-grid {
  display: grid;
  gap: 0.5rem;
  margin: 0;
  padding: 0.75rem;
  border: 1px solid var(--color-border, rgb(var(--yb-border)));
  border-radius: 8px;
}

.draft-row {
  display: grid;
  grid-template-columns: 7rem minmax(0, 1fr);
  gap: 0.5rem;
  font-size: 0.8125rem;
}

.draft-row dt {
  margin: 0;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
  font-weight: 500;
}

.draft-row dd {
  margin: 0;
  word-break: break-word;
}

.single-violation-list {
  margin: 0.5rem 0 0;
  padding-left: 1.1rem;
  font-size: 0.8125rem;
  color: rgb(var(--yb-danger-text));
}

.ref-thumbs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.row-ref-editor {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  min-width: 190px;
}

.hidden-input {
  display: none;
}

.ref-thumb-item {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  gap: 0.2rem;
  align-items: flex-start;
  max-width: 88px;
}

.ref-thumb-img {
  width: 56px;
  height: 56px;
  object-fit: cover;
  border-radius: 4px;
  border: 1px solid var(--color-border, rgb(var(--yb-border)));
}

.ref-thumb-file {
  font-size: 0.7rem;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
}

.ref-thumb-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.2rem;
}

.ref-action,
.ref-upload-btn {
  border: 1px solid var(--color-border, rgb(var(--yb-border-strong)));
  border-radius: 4px;
  background: var(--color-surface, rgb(var(--yb-surface)));
  color: var(--color-text-secondary, rgb(var(--yb-text-secondary)));
  font-size: 0.68rem;
  line-height: 1.2;
  padding: 0.16rem 0.3rem;
  cursor: pointer;
}

.ref-action:disabled,
.ref-upload-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.ref-action-danger {
  color: rgb(var(--yb-danger-text));
  border-color: rgb(var(--yb-danger-border));
}

.ref-upload-btn {
  align-self: flex-start;
  color: var(--color-primary, rgb(var(--yb-brand)));
  border-color: color-mix(in srgb, var(--color-primary, rgb(var(--yb-brand))) 35%, rgb(var(--yb-border-strong)));
}

.empty-ref-text {
  font-size: 0.75rem;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
}

.batch-ref-status,
.batch-ref-error {
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
}

.batch-ref-status {
  color: rgb(var(--yb-success-teal));
}

.batch-ref-error {
  color: rgb(var(--yb-danger-text));
}

.err-tag {
  display: inline-block;
  margin: 0 0.25rem 0.25rem 0;
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
  background: rgb(var(--yb-danger-soft-hover));
  color: rgb(var(--yb-danger-text));
  font-size: 0.7rem;
}

.meta-card,
.submit-card {
  border: 1px solid var(--color-border, rgb(var(--yb-border)));
  border-radius: 8px;
  padding: 1rem;
}

.field-group {
  margin-bottom: 0.75rem;
}

.field-group:last-child {
  margin-bottom: 0;
}

.field-label {
  display: block;
  margin-bottom: 0.35rem;
  font-size: 0.8125rem;
  font-weight: 500;
}

.required {
  color: rgb(var(--yb-danger));
}

.due-at-row {
  display: flex;
  gap: 0.5rem;
}

.native-input {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--color-border, rgb(var(--yb-border-strong)));
  border-radius: 6px;
  padding: 0.4rem 0.55rem;
  font-size: 0.875rem;
}

.due-hour-select {
  flex: 0 0 5.5rem;
}

.urgent-toggle {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.875rem;
}

.error-banner {
  margin: 0 0 0.5rem;
  padding: 0.5rem 0.65rem;
  border-radius: 6px;
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
  font-size: 0.8125rem;
}

.submit-hint {
  margin: 0 0 0.75rem;
  font-size: 0.8125rem;
  color: var(--color-text-secondary, rgb(var(--yb-text-muted)));
}

.submit-btn {
  width: 100%;
}

@media (max-width: 960px) {
  .layout-grid {
    grid-template-columns: 1fr;
  }
}
</style>
