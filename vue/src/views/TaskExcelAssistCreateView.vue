<template>
  <div class="excel-assist-view">
    <div class="excel-assist-card">
      <header class="page-header">
        <div>
          <h2 class="page-title">Excel 辅助创建任务</h2>
          <p class="page-subtitle">
            支持新款批量 SKU 与新款单 SKU 的 Excel 辅助创建。原款开发、采购单 SKU 等类型将在后续版本中支持。
          </p>
        </div>
        <BaseButton variant="secondary" size="sm" @click="goBack">返回任务中心</BaseButton>
      </header>

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

      <p class="flow-label" aria-label="当前任务类型">{{ excelAssistFlowLabel(flow) }}</p>

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
            v-else
            @parsed="onSingleExcelParsed"
            @reset="onSingleExcelReset"
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
                    :class="{ 'has-error': previewRowErrors(idx + 1).length > 0 }"
                  >
                    <td>{{ idx + 1 }}</td>
                    <td>{{ row.product_name || '—' }}</td>
                    <td class="cell-ellipsis">{{ row.design_requirement || '—' }}</td>
                    <td>{{ row.product_i_id || '—' }}</td>
                    <td>
                      <div v-if="row.reference_file_refs?.length" class="ref-thumbs">
                        <span
                          v-for="ref in row.reference_file_refs"
                          :key="ref.ref_id"
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
                        </span>
                      </div>
                      <span v-else>—</span>
                    </td>
                    <td>
                      <span
                        v-for="err in previewRowErrors(idx + 1)"
                        :key="`${err.column}-${err.code}`"
                        class="err-tag"
                      >
                        {{ err.column }} · {{ err.message || err.code }}
                      </span>
                      <span v-if="previewRowErrors(idx + 1).length === 0">—</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
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
import type { BatchPreviewRow, BatchViolation } from '@/services/api/batchSkuApi'
import type { SingleTaskExcelDraft, ExcelAssistViolation } from '@/services/api/excelAssistApi'
import type { Task } from '@/domain/types/task'
import type { TaskBatchItem } from '@/domain/types'
import {
  canSubmitExcelAssistBatch,
  canSubmitExcelAssistSingle,
  excelAssistFlowLabel,
  mapExcelPreviewToBatchItems,
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
]

const flow = ref<ExcelAssistFlow>('new_batch')

const previewRows = ref<BatchPreviewRow[]>([])
const violations = ref<BatchViolation[]>([])
const batchItems = ref<TaskBatchItem[]>([])

const singleDraft = ref<SingleTaskExcelDraft | null>(null)
const singleViolations = ref<ExcelAssistViolation[]>([])

const groupId = ref('')
const dueAt = ref<string | null>(null)
const note = ref('')
const priority = ref('normal')
const submitError = ref('')
const submitting = ref(false)

const priorityOptions = [
  { value: 'low', label: '低' },
  { value: 'normal', label: '普通' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '加急' },
]

const groupOptions = computed(() =>
  filterOwnerTeamOptions(rawTeamOptions.value, resolveDepartmentByTeam),
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
}

function resetSingleExcelState() {
  singleDraft.value = null
  singleViolations.value = []
}

function resetAllExcelState() {
  resetBatchExcelState()
  resetSingleExcelState()
  submitError.value = ''
}

function onBatchExcelParsed(payload: { preview: BatchPreviewRow[]; violations: BatchViolation[] }) {
  previewRows.value = payload.preview
  violations.value = payload.violations
  batchItems.value = mapExcelPreviewToBatchItems(EXCEL_ASSIST_TASK_TYPE, payload.preview, {
    skuCodeType: 'regular',
  })
  submitError.value = ''
}

function onBatchExcelReset() {
  resetBatchExcelState()
  submitError.value = ''
}

function onSingleExcelParsed(payload: {
  draft: SingleTaskExcelDraft
  violations: ExcelAssistViolation[]
}) {
  singleDraft.value = payload.draft
  singleViolations.value = payload.violations
  submitError.value = ''
}

function onSingleExcelReset() {
  resetSingleExcelState()
  submitError.value = ''
}

function previewRowErrors(row: number): BatchViolation[] {
  return violations.value.filter((v) => v.row === row)
}

function isImageMime(mimeType: string): boolean {
  return mimeType.startsWith('image/')
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

function buildCommonTaskFields() {
  const currentUser = permissionsStore.currentUser
  const now = nowISO()
  const preflightOwnerDepartment = resolveOwnerDepartmentForSubmit()
  return {
    businessLane: 'normal' as const,
    workflowLane: 'normal' as const,
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
    syncErpOnCreate: true,
    assetVersions: [] as unknown[],
    businessType: 'NEW_PRODUCT_DEV' as const,
    requiresAssetVersions: true,
    createdAt: now,
    updatedAt: now,
    preflightOwnerDepartment,
  }
}

async function submit() {
  if (!canSubmit.value || submitting.value) return
  submitError.value = ''
  submitting.value = true

  const common = buildCommonTaskFields()
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
  } else {
    const mapped = mapExcelPreviewToSingleTask({
      draft: singleDraft.value!,
      pageNote: note.value,
    })
    payload = {
      ...common,
      ...mapped,
    }
    delete payload.preflightOwnerDepartment
  }

  try {
    const created = await tasksStore.addTask(payload as unknown as Partial<Task>)
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
  background: var(--color-surface, #fff);
  border: 1px solid var(--color-border, #e5e7eb);
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
  color: var(--color-text-secondary, #6b7280);
  font-size: 0.875rem;
  line-height: 1.5;
}

.flow-switch {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.flow-tab {
  border: 1px solid var(--color-border, #d1d5db);
  border-radius: 8px;
  padding: 0.4rem 0.85rem;
  background: var(--color-surface, #fff);
  color: var(--color-text-secondary, #6b7280);
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
}

.flow-tab.active {
  border-color: var(--color-primary, #2563eb);
  background: color-mix(in srgb, var(--color-primary, #2563eb) 8%, white);
  color: var(--color-primary, #2563eb);
  font-weight: 600;
}

.flow-label {
  display: inline-block;
  margin: 0 0 1.25rem;
  padding: 0.35rem 0.75rem;
  border: 1px solid var(--color-primary, #2563eb);
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-primary, #2563eb) 8%, white);
  color: var(--color-primary, #2563eb);
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
  color: var(--color-text-secondary, #6b7280);
}

.preview-table-wrap {
  overflow: auto;
  border: 1px solid var(--color-border, #e5e7eb);
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
  border-bottom: 1px solid var(--color-border, #f3f4f6);
  text-align: left;
  vertical-align: top;
}

.preview-table th {
  background: var(--color-surface-muted, #f9fafb);
  font-weight: 600;
  white-space: nowrap;
}

.preview-table tr.has-error {
  background: color-mix(in srgb, #ef4444 6%, white);
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
  border: 1px solid var(--color-border, #e5e7eb);
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
  color: var(--color-text-secondary, #6b7280);
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
  color: #b91c1c;
}

.ref-thumbs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.ref-thumb-img {
  width: 36px;
  height: 36px;
  object-fit: cover;
  border-radius: 4px;
  border: 1px solid var(--color-border, #e5e7eb);
}

.ref-thumb-file {
  font-size: 0.7rem;
  color: var(--color-text-secondary, #6b7280);
}

.err-tag {
  display: inline-block;
  margin: 0 0.25rem 0.25rem 0;
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
  background: #fee2e2;
  color: #b91c1c;
  font-size: 0.7rem;
}

.meta-card,
.submit-card {
  border: 1px solid var(--color-border, #e5e7eb);
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
  color: #dc2626;
}

.due-at-row {
  display: flex;
  gap: 0.5rem;
}

.native-input {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--color-border, #d1d5db);
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
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.8125rem;
}

.submit-hint {
  margin: 0 0 0.75rem;
  font-size: 0.8125rem;
  color: var(--color-text-secondary, #6b7280);
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
