<template>
  <BaseModal
    :model-value="modelValue"
    title="创建任务"
    :show-confirm="false"
    cancel-text="关闭"
    panel-class="max-w-[min(1060px,96vw)] !max-h-[94vh] create-task-modal-panel"
    @update:model-value="handleModalUpdate"
  >
    <div v-if="submitError" class="submit-error-banner">
      {{ submitError }}
    </div>
    <div class="modal-grid" :class="{ 'is-batch': isBatchLayout }">
      <div class="form-section">
        <section class="create-type-panel">
          <div class="create-type-header">
            <div>
              <p class="eyebrow">任务分组</p>
              <div class="mode-switch">
                <button
                  v-for="group in taskGroupOptions"
                  :key="group.value"
                  type="button"
                  class="task-kind-button"
                  :class="taskGroup === group.value ? 'is-active' : 'is-inactive'"
                  @click="selectTaskGroup(group.value)"
                >
                  {{ group.label }}
                </button>
              </div>
            </div>
          </div>

          <div class="task-kind-switch">
            <button
              v-for="option in visibleCreateTypes"
              :key="option.value"
              type="button"
              class="task-kind-button"
              :class="createType === option.value ? 'is-active' : 'is-inactive'"
              @click="selectCreateType(option.value)"
            >
              <component :is="option.icon" class="w-4 h-4" />
              <span>{{ option.label }}</span>
            </button>
          </div>
        </section>

        <div v-if="canChooseSkuMode" class="field-group">
          <label class="field-label">创建模式</label>
          <div class="mode-switch">
            <button
              type="button"
              class="task-kind-button"
              :class="form.skuMode !== 'multiple' ? 'is-active' : 'is-inactive'"
              @click="form.skuMode = 'single'"
            >
              单个 SKU
            </button>
            <button
              type="button"
              class="task-kind-button"
              :class="form.skuMode === 'multiple' ? 'is-active' : 'is-inactive'"
              @click="form.skuMode = 'multiple'"
            >
              批量 SKU
            </button>
          </div>
        </div>

        <div class="create-workspace">
          <div class="form-fields">
            <!-- 批量：公共信息在左，Excel 流程在右 -->
            <template v-if="isBatchLayout">
              <section v-if="batchPreviewRows.length > 0" class="batch-preview-section">
                <div class="batch-preview-header">
                  <h4 class="batch-section-title">解析预览</h4>
                  <span>合计 {{ batchPreviewRows.length }} 行 · 错误 {{ excelViolations.length }} 条</span>
                </div>
                <div class="batch-preview-table-wrap">
                  <table class="batch-preview-table">
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
                        v-for="(row, idx) in batchPreviewRows"
                        :key="`batch-preview-${idx}`"
                        :class="{ 'has-error': previewRowErrors(idx + 1).length }"
                      >
                        <td>{{ idx + 1 }}</td>
                        <td>{{ row.product_name || '—' }}</td>
                        <td class="batch-cell-ellipsis">{{ row.design_requirement || '—' }}</td>
                        <td>{{ row.product_i_id || '—' }}</td>
                        <td>
                          <div v-if="row.reference_file_refs?.length" class="batch-ref-thumbs">
                            <span
                              v-for="ref in row.reference_file_refs"
                              :key="ref.ref_id"
                              class="batch-ref-thumb-item"
                              :title="ref.filename"
                            >
                              <img
                                v-if="isImageMimeType(ref.mime_type)"
                                :src="ref.download_url"
                                :alt="ref.filename"
                                class="batch-ref-thumb-img"
                              />
                              <span v-else class="batch-ref-thumb-file">{{ ref.filename }}</span>
                            </span>
                          </div>
                          <span v-else>—</span>
                        </td>
                        <td>
                          <span
                            v-for="err in previewRowErrors(idx + 1)"
                            :key="`${err.column}-${err.code}`"
                            class="batch-err-tag"
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

              <section class="batch-meta-compact">
                <div class="batch-meta-card field-group">
                  <label class="field-label">任务截止时间</label>
                  <input
                    v-model="dueAtLocal"
                    type="date"
                    class="native-input"
                    :min="dueAtMin"
                  />
                </div>
                <div class="batch-meta-card field-group">
                  <BaseSelect
                    v-if="isDeptAdminPlus"
                    v-model="form.priority"
                    label="优先级"
                    :options="priorityOptions"
                  />
                  <label v-else class="urgent-toggle">
                    <input v-model="urgentModel" type="checkbox" />
                    是否加急
                  </label>
                </div>
                <div class="batch-meta-card">
                  <BaseTextarea v-model="form.note" label="备注" :rows="1" placeholder="备注" />
                </div>
              </section>
            </template>

            <!-- 单 SKU / 原品 -->
            <template v-else>
              <TaskCreateOriginalForm
                v-if="usesOriginalProductForm"
                v-model:form="form"
              />
              <section v-else-if="taskKind === 'RETOUCH_TASK'" class="type-section retouch-form">
                <div class="form-card upload-card">
                  <label class="field-label">图片/附件 <span class="required">*</span></label>
                  <ReferenceUploadPanel v-model="referenceRefsModel" compact />
                </div>
                <div class="form-card">
                  <BaseTextarea
                    v-model="form.designRequirement"
                    label="修改要求"
                    :rows="4"
                    placeholder="请填写 P 图修改要求，例如去背景、补光、替换文字"
                  />
                </div>
              </section>
              <TaskCreateNewProductForm
                v-else-if="taskKind === 'NEW_PRODUCT_DEV'"
                v-model:form="form"
              />
              <TaskCreatePurchaseForm
                v-else-if="taskKind === 'PURCHASE_TASK'"
                v-model:form="form"
              />

              <section v-if="requiresDesignSource" class="v1-extra-section">
                <h4 class="batch-section-title">设计来源校验</h4>
                <p class="batch-bridge-hint">补图/常规定制必须先命中设计源，未命中时禁止提交。</p>
                <DesignSourcePicker @verified="designSourceVerified = $event" />
              </section>

              <section v-if="requiresErpVerification" class="v1-extra-section">
                <h4 class="batch-section-title">ERP 产品校验</h4>
                <p class="batch-bridge-hint">客户定制必须先命中 ERP 商品，避免创建无归属产品的定制任务。</p>
                <ErpProductPicker @verified="erpProductVerified = $event" />
              </section>

              <section v-if="isCustomizationFlow" class="v1-extra-section customization-card-grid">
                <h4 class="batch-section-title">新增业务信息</h4>
                <BaseInput
                  v-model="form.orderNumber"
                  label="关联订单号"
                  placeholder="客户定制/补图可填写订单号"
                />
                <BaseTextarea
                  v-model="copyContentModel"
                  label="文案内容"
                  :rows="1"
                  placeholder="需要上图或定制呈现的文案"
                />
                <BaseInput
                  v-model="styleKeywordsModel"
                  label="风格关键词"
                  placeholder="例如：极简、节日、复古"
                />
              </section>

              <section class="meta-card-grid">
                <div class="field-group">
                  <label class="field-label">任务截止时间</label>
                  <input
                    v-model="dueAtLocal"
                    type="date"
                    class="native-input"
                    :min="dueAtMin"
                  />
                </div>

                <div class="field-group">
                  <BaseSelect
                    v-if="isDeptAdminPlus"
                    v-model="form.priority"
                    label="优先级"
                    :options="priorityOptions"
                  />
                  <label v-else class="urgent-toggle">
                    <input v-model="urgentModel" type="checkbox" />
                    是否加急
                  </label>
                </div>
                <BaseTextarea v-model="form.note" label="备注" :rows="1" placeholder="备注" />
              </section>
            </template>
          </div>

          <aside class="create-context-panel" :class="contextPanelClass">
            <div class="context-panel-header">
              <span class="context-dot"></span>
              <div>
                <p class="eyebrow">{{ isCustomizationFlow ? '定制上下文' : '创建上下文' }}</p>
                <h4>{{ contextPanelTitle }}</h4>
              </div>
            </div>

            <ExcelBatchSkuPanel
              v-if="isBatchLayout"
              :task-type="taskKind === 'PURCHASE_TASK' ? 'purchase_task' : 'new_product_development'"
              :hide-preview="true"
              @parsed="onExcelParsed"
              @reset="onExcelReset"
            />

            <div v-else class="context-card-list">
              <div
                v-for="item in contextPanelItems"
                :key="item.title"
                class="context-card"
              >
                <p class="context-card-title">{{ item.title }}</p>
                <p class="context-card-body">{{ item.body }}</p>
              </div>
            </div>

            <section v-if="showSyncErpToggle" class="erp-sync-toggle-card">
              <div class="erp-sync-toggle-head">
                <label class="field-label erp-sync-title">ERP 同步</label>
                <span class="erp-sync-badge">创建策略</span>
              </div>
              <div class="erp-sync-toggle-row">
                <div class="erp-sync-control">
                  <span class="erp-sync-main-label">创建后立即同步 ERP</span>
                  <BaseSwitch :model-value="form.syncErpOnCreate ?? true" @update:model-value="form.syncErpOnCreate = $event" class="erp-switch">
                    {{ form.syncErpOnCreate ? '开启' : '关闭' }}
                  </BaseSwitch>
                </div>
                <p class="erp-sync-toggle-hint" :class="{ warning: form.syncErpOnCreate === false }">
                  {{
                    form.syncErpOnCreate === false
                      ? '已关闭立即同步：本次创建不会立刻同步 ERP，将在后续流程触发同步。'
                      : '创建成功后将立即同步最小商品资料到 ERP。'
                  }}
                </p>
              </div>
            </section>

            <section class="submit-check-section">
              <div class="submit-check-header">
                <div>
                  <h4 class="summary-title">提交校验</h4>
                  <p class="submit-check-hint">
                    {{ canSubmit ? '所有必填项已满足，可以创建任务。' : '还有必填项未满足，请检查表单。' }}
                  </p>
                </div>
                <BaseButton
                  v-if="taskKind !== 'ORIGINAL_PRODUCT_DEV' && taskKind !== 'RETOUCH_TASK'"
                  variant="secondary"
                  size="sm"
                  :disabled="!canSubmit || preparingSku"
                  @click="prepareSkuPreview"
                >
                  {{ preparingSku ? '预展示中...' : isBatchLayout ? '预展示批量 SKU' : '预展示 SKU' }}
                </BaseButton>
              </div>
              <ul v-if="validationIssues.length" class="issue-list">
                <li v-for="msg in validationIssues" :key="msg" class="issue-item">
                  {{ msg }}
                </li>
              </ul>
            </section>
          </aside>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="summary-footer">
        <BaseButton
          variant="secondary"
          class="submit-btn"
          :disabled="!isDraftDirty || savingDraft || loadingDraft"
          :loading="savingDraft"
          @click="saveDraft"
        >
          保存草稿
        </BaseButton>
        <BaseButton
          variant="primary"
          class="submit-btn"
          :disabled="!canSubmit || submitting || loadingDraft"
          :loading="submitting"
          @click="submit"
        >
          创建任务
        </BaseButton>
      </div>
    </template>
  </BaseModal>

  <CloseDraftConfirmModal
    :open="showCloseConfirm"
    @save="saveAndClose"
    @discard="discardAndClose"
    @cancel="showCloseConfirm = false"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Box, Images, Palette, Sparkles, ShoppingCart, Wand2 } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import type { Task } from '@/domain/types'
import type { TaskCreateFormModel, TaskKind } from '@/domain/types'
import { canSubmitTask } from '@/domain/task-create-rules'
import { pickFieldWhitelistViolations } from '@/domain/task-create-fields'
import { defaultBatchTemplateValues } from '@/domain/batch-task-create'
import { useTasksStore } from '@/stores/tasks'
import { generateActionId } from '@/utils/uuid'
import { usePermissionsStore } from '@/stores/permissions'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseSwitch from '@/components/base/BaseSwitch.vue'
import TaskCreateOriginalForm from '@/components/task/TaskCreateOriginalForm.vue'
import TaskCreateNewProductForm from '@/components/task/TaskCreateNewProductForm.vue'
import TaskCreatePurchaseForm from '@/components/task/TaskCreatePurchaseForm.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import ExcelBatchSkuPanel from '@/components/task-create/ExcelBatchSkuPanel.vue'
import CloseDraftConfirmModal from '@/components/task-create/CloseDraftConfirmModal.vue'
import DesignSourcePicker from '@/components/task-create/DesignSourcePicker.vue'
import ErpProductPicker from '@/components/task-create/ErpProductPicker.vue'
import { useTaskDraft } from '@/composables/useTaskDraft'
import { useTeamOptions } from '@/composables/useTeamOptions'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { useAuth } from '@/composables/useAuth'
import { useActorOwnerScope } from '@/composables/useActorOwnerScope'
import { tasksApi } from '@/services/api/tasksApi'
import type { BatchPreviewRow, BatchViolation } from '@/services/api/batchSkuApi'
import { getBeijingDateString, nowISO, taskBeijingDateKey, toBeijingEndOfDayISO } from '@/utils/date'
import { humanizeTaskCreateFields, humanizeViolationCode } from '@/domain/task-create-fields'
import { normalizePriorityForApi } from '@/domain/task-priority'
import { buildCategoryPatchFields } from '@/domain/category-payload'

const props = defineProps<{
  modelValue: boolean
  defaultTaskKind?: TaskKind
  initialDraftId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  created: [taskId: string]
}>()

const { teamOptions: rawTeamOptions, resolveDepartmentByTeam } = useTeamOptions()
const {
  hideOwnerFields,
  defaultOwnerTeam,
  filterOwnerTeamOptions,
  validateOwnerScope,
} = useActorOwnerScope()

// Round I.g · D1：下拉按 actor DataScope 过滤。Global/HR 不限制；
// DA 只看本部门内的 team；GroupLeader 只看 managed_teams ∪ [actor.team]。
const groupOptions = computed(() => filterOwnerTeamOptions(rawTeamOptions.value, resolveDepartmentByTeam))
const { assigneeOptions, loadDesigners } = useDesignerOptions({
  includeEmpty: true,
  requiredActions: ['task.create'],
  workflowLane: 'normal',
})

const submitError = ref('')
const batchItemsError = ref('')
const fieldErrors = ref<Record<string, string>>({})
const preparingSku = ref(false)
const showCloseConfirm = ref(false)
const draftId = ref('')
const savedDraftSnapshot = ref('')
const loadingDraft = ref(false)
const {
  save: createDraft,
  update: updateDraft,
  getById: getDraftById,
  saving: savingDraft,
} = useTaskDraft()

function handleModalOpened(): void {
  submitError.value = ''
  actionId.value = generateActionId()
  loadDesigners()
  showCloseConfirm.value = false
  const nextDraftId = String(props.initialDraftId ?? '').trim()
  if (nextDraftId) {
    void hydrateDraft(nextDraftId)
  } else {
    savedDraftSnapshot.value = JSON.stringify(buildDraftPayload())
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) handleModalOpened()
  },
)

onMounted(() => {
  if (props.modelValue) handleModalOpened()
})

const priorityOptions = [
  { value: 'low', label: '低' },
  { value: 'normal', label: '普通' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '加急' },
]

const router = useRouter()
const tasksStore = useTasksStore()
const permissionsStore = usePermissionsStore()
const { isDeptAdminPlus } = useAuth()

type TaskGroup = 'normal' | 'customization'
type CreateType =
  | 'original'
  | 'new_single'
  | 'new_batch'
  | 'purchase_single'
  | 'retouch'
  | 'customer_customization'
  | 'regular_customization'

const taskGroup = ref<TaskGroup>('normal')
const createType = ref<CreateType>('original')
const designSourceVerified = ref(false)
const erpProductVerified = ref(false)
const batchPreviewRows = ref<BatchPreviewRow[]>([])
const excelViolations = ref<Array<{ row: number; column: string; code: string; message?: string }>>([])

const taskGroupOptions: Array<{ value: TaskGroup; label: string }> = [
  { value: 'normal', label: '常规任务' },
  { value: 'customization', label: '定制任务' },
]

const createTypeOptions: Array<{
  value: CreateType
  group: TaskGroup
  label: string
  kind: TaskKind
  icon: unknown
}> = [
  {
    value: 'original',
    group: 'normal',
    label: '原款开发',
    kind: 'ORIGINAL_PRODUCT_DEV',
    icon: Box,
  },
  {
    value: 'new_single',
    group: 'normal',
    label: '新款单 SKU',
    kind: 'NEW_PRODUCT_DEV',
    icon: Sparkles,
  },
  {
    value: 'new_batch',
    group: 'normal',
    label: '新款批量',
    kind: 'NEW_PRODUCT_DEV',
    icon: Images,
  },
  {
    value: 'purchase_single',
    group: 'normal',
    label: '采购单 SKU',
    kind: 'PURCHASE_TASK',
    icon: ShoppingCart,
  },
  {
    value: 'retouch',
    group: 'normal',
    label: 'P 图任务',
    kind: 'RETOUCH_TASK',
    icon: Wand2,
  },
  {
    value: 'customer_customization',
    group: 'customization',
    label: '客户定制',
    kind: 'ORIGINAL_PRODUCT_DEV',
    icon: Palette,
  },
  {
    value: 'regular_customization',
    group: 'customization',
    label: '常规定制',
    kind: 'NEW_PRODUCT_DEV',
    icon: Sparkles,
  },
]

const taskKind = ref<TaskKind>(props.defaultTaskKind ?? 'ORIGINAL_PRODUCT_DEV')
const submitting = ref(false)
const ARCHIVE_PREFILL_TASK_KIND_WHITELIST: readonly TaskKind[] = ['NEW_PRODUCT_DEV', 'PURCHASE_TASK']

const form = ref<TaskCreateFormModel>({
  productId: null,
  productName: '',
  sku: null,
  productImageUrl: undefined,
  productCategoryName: undefined,
  productCategoryCode: undefined,
  groupId: '',
  groupName: '',
  assigneeId: null,
  assigneeName: null,
  designRequirement: '',
  referenceFileRefs: [],
  dueAt: null,
  priority: 'normal',
  customizationRequired: false,
  customizationSourceType: undefined,
  note: '',
  costPriceMode: 'manual',
  category: undefined,
  material: undefined,
  materialOther: undefined,
  productShortName: undefined,
  productReferenceUrl: undefined,
  costUnitPrice: undefined,
  quantity: undefined,
  basePriceAmount: undefined,
  productChannel: undefined,
  costPriceAmount: undefined,
  costPriceCurrency: 'CNY',
  syncErpOnCreate: true,
  purchaseQuantity: undefined,
  purchaseUnit: undefined,
  prefillSpecText: undefined,
  skuMode: 'single',
  batchItems: [],
  batchTemplate: undefined,
  batchTemplateSaved: false,
})

const isBatchLayout = computed(
  () => form.value.skuMode === 'multiple' && taskKind.value !== 'ORIGINAL_PRODUCT_DEV',
)

const visibleCreateTypes = computed(() => createTypeOptions.filter((option) => option.group === taskGroup.value))
const isCustomizationFlow = computed(() => taskGroup.value === 'customization')
const canChooseSkuMode = computed(() => false)
const contextPanelClass = computed(() => ({
  'is-customization': isCustomizationFlow.value,
  'is-retouch': taskKind.value === 'RETOUCH_TASK',
}))
const contextPanelTitle = computed(() => {
  if (isBatchLayout.value) return 'Excel 批量流程'
  if (createType.value === 'customer_customization') return '客户上下文'
  if (createType.value === 'regular_customization') return '蓝图解析'
  if (taskKind.value === 'RETOUCH_TASK') return 'P 图任务只保留必要字段'
  if (taskKind.value === 'PURCHASE_TASK') return '成本与采购规则'
  if (taskKind.value === 'NEW_PRODUCT_DEV') return 'SKU 创建状态'
  return 'ERP 产品主档'
})
const contextPanelItems = computed(() => {
  if (createType.value === 'customer_customization') {
    return [
      { title: '客户来单校验', body: '先命中 ERP 商品，再填写订单号、文案和风格关键词。' },
      { title: '风险提示', body: '定制需求建议包含颜色、尺寸、用途和交付约束，降低返工。' },
    ]
  }
  if (createType.value === 'regular_customization') {
    return [
      { title: '设计来源', body: '常规定制必须先命中设计源，未命中时禁止提交。' },
      { title: '蓝图建议', body: '先确认蓝图版本，再提交定制说明和补充资料。' },
    ]
  }
  if (taskKind.value === 'RETOUCH_TASK') {
    return [
      { title: '最小字段', body: '只需要上传图片/附件并填写修改要求，无需 SKU、成本或分类。' },
      { title: '上传提示', body: '上传文件会以小缩略图横向展示，单文件不超过 300MB。' },
    ]
  }
  if (taskKind.value === 'PURCHASE_TASK') {
    return [
      { title: 'SKU 状态', body: 'SKU 由后端创建任务后生成，创建前可先做预展示。' },
      { title: '成本方式', body: '手动录入显示成本输入；按模板时由系统规则计算。' },
      { title: 'ERP 同步', body: '默认创建后立即同步 ERP，可手动关闭；关闭后将在后续流程触发同步。' },
    ]
  }
  if (taskKind.value === 'NEW_PRODUCT_DEV') {
    return [
      { title: 'SKU 状态', body: '新品 SKU 由后端生成，当前表单只收集分类、名称、需求和规格。' },
      { title: '参考图', body: '参考图采用按钮式上传，不再占用整行大面积空间。' },
      { title: 'ERP 同步', body: '默认创建后立即同步 ERP，可手动关闭；关闭后将在后续流程触发同步。' },
    ]
  }
  return [
    { title: '主实体', body: '从 ERP 选择已有产品，SKU、图片和分类自动带入。' },
    { title: '修改范围', body: '主表单只保留修改要求、尺寸覆盖和参考图，减少无关输入。' },
  ]
})
const usesOriginalProductForm = computed(
  () =>
    taskKind.value === 'ORIGINAL_PRODUCT_DEV' &&
    (createType.value === 'original' || createType.value === 'customer_customization'),
)
const requiresDesignSource = computed(() => createType.value === 'regular_customization')
const requiresErpVerification = computed(() => createType.value === 'customer_customization')
const showSyncErpToggle = computed(() =>
  createType.value === 'new_single' ||
  createType.value === 'new_batch' ||
  createType.value === 'purchase_single',
)

const copyContentModel = computed({
  get: () => form.value.copyContent ?? '',
  set: (value: string) => {
    form.value.copyContent = value.trim() ? value : undefined
  },
})
const styleKeywordsModel = computed({
  get: () => form.value.styleKeywords ?? '',
  set: (value: string) => {
    form.value.styleKeywords = value.trim() ? value : undefined
  },
})

function applyCreateType(option: (typeof createTypeOptions)[number]) {
  createType.value = option.value
  taskKind.value = option.kind
  form.value.skuMode = option.value === 'new_batch' ? 'multiple' : 'single'
  form.value.customizationRequired = option.group === 'customization'
  form.value.customizationSourceType = option.group === 'customization'
    ? option.kind === 'ORIGINAL_PRODUCT_DEV'
      ? 'existing_product'
      : 'new_product'
    : undefined
  designSourceVerified.value = false
  erpProductVerified.value = false
  batchPreviewRows.value = []
  excelViolations.value = []
  if (option.value !== 'new_batch') {
    form.value.batchItems = []
    form.value.batchTemplateSaved = false
  }
}

function selectTaskGroup(group: TaskGroup) {
  taskGroup.value = group
  const first = createTypeOptions.find((option) => option.group === group)
  if (first) applyCreateType(first)
}

function selectCreateType(value: CreateType) {
  const option = createTypeOptions.find((item) => item.value === value)
  if (option) applyCreateType(option)
}

const batchTemplateModel = computed({
  get: () => form.value.batchTemplate ?? defaultBatchTemplateValues(taskKind.value),
  set: (v) => {
    form.value.batchTemplate = v
  },
})

watch(groupOptions, (opts) => {
  if (opts.length === 0) return
  const currentId = form.value.groupId
  const optValues = opts.map((o) => String(o.value))
  if (!currentId || !optValues.includes(currentId)) {
    const preferred = defaultOwnerTeam.value && optValues.includes(defaultOwnerTeam.value)
      ? opts.find((o) => String(o.value) === defaultOwnerTeam.value)!
      : opts[0]
    form.value.groupId = preferred.value as string
    form.value.groupName = preferred.label
  }
}, { immediate: true })
watch(() => form.value.groupId, (id) => {
  form.value.groupName = id || ''
})
watch(() => form.value.assigneeId, (id) => {
  const opt = assigneeOptions.value.find((o) => o.value === id)
  form.value.assigneeName = opt?.label ?? null
})
watch(taskKind, (mode) => {
  form.value.referenceFileRefs = []
  form.value.prefillSpecText = undefined
  form.value.skuMode = createType.value === 'new_batch' ? 'multiple' : 'single'
  form.value.batchItems = []
  form.value.batchTemplateSaved = false
  form.value.batchTemplate = defaultBatchTemplateValues(mode)
  // 产品绑定快照：切任务类型时始终清空，避免跨分型污染 product_selection
  form.value.productId = null
  form.value.productName = ''
  form.value.sku = null
  form.value.productImageUrl = undefined
  form.value.productCategoryName = undefined
  form.value.productCategoryCode = undefined
  form.value.erpProductSnapshot = undefined
  // 字段白名单清理：隐藏字段必须在 v-if 变 false 时同步清空，
  // 避免残留值被 buildCreatePayload 带入 POST 体触发 400。
  if (mode !== 'NEW_PRODUCT_DEV' && mode !== 'PURCHASE_TASK') {
    form.value.material = undefined
    form.value.materialOther = undefined
    form.value.category = undefined
    form.value.productShortName = undefined
  }
  if (mode !== 'PURCHASE_TASK') {
    form.value.costPriceAmount = undefined
    form.value.purchaseQuantity = undefined
    form.value.basePriceAmount = undefined
    form.value.productChannel = undefined
    form.value.costPriceMode = 'manual'
  }
  if (mode === 'PURCHASE_TASK') {
    form.value.designRequirement = ''
  }
})

watch(
  () => form.value.skuMode,
  (mode) => {
    if (mode !== 'multiple' || taskKind.value === 'ORIGINAL_PRODUCT_DEV') return
    batchPreviewRows.value = []
    form.value.batchItems = []
    form.value.batchTemplateSaved = false
    form.value.batchTemplate = defaultBatchTemplateValues(taskKind.value)
    form.value.referenceFileRefs = []
  },
)

const actionId = ref(generateActionId())

const dueAtLocal = computed({
  get: () => {
    return taskBeijingDateKey(form.value.dueAt)
  },
  set: (v: string) => {
    if (!v) {
      form.value.dueAt = null
      return
    }
    form.value.dueAt = toBeijingEndOfDayISO(v)
  },
})

const dueAtMin = computed(() => {
  return getBeijingDateString()
})

const urgentModel = computed({
  get: () => form.value.priority === 'critical',
  set: (checked: boolean) => {
    form.value.priority = checked ? 'critical' : 'normal'
  },
})

const referenceRefsModel = computed({
  get: () => form.value.referenceFileRefs,
  set: (value: (string | Record<string, unknown>)[]) => {
    form.value.referenceFileRefs = value
  },
})

const isDraftDirty = computed(() => JSON.stringify(buildDraftPayload()) !== savedDraftSnapshot.value)

function previewRowErrors(row: number): BatchViolation[] {
  return excelViolations.value.filter((v) => v.row === row)
}

function isImageMimeType(mimeType: string | undefined): boolean {
  return String(mimeType ?? '').startsWith('image/')
}

function resolveOwnerDepartmentForSubmit(): string | undefined {
  const fromTeam = resolveDepartmentByTeam(form.value.groupId)
  if (fromTeam) return fromTeam
  const fromUser = permissionsStore.currentUser?.departmentId
  if (fromUser && fromUser !== '未分配') return fromUser
  return undefined
}

const validationIssues = computed<string[]>(() => {
  const issues: string[] = []
  const f = form.value
  if (!f.dueAt) issues.push('未填写任务截止时间')

  if (!isBatchLayout.value) {
    // 单个模式：补充最关键的几项
    if (taskKind.value === 'NEW_PRODUCT_DEV') {
      if (!f.category) issues.push('未选择产品款式编码')
      if (!f.productName) issues.push('未填写产品名称')
      if (!f.designRequirement?.trim()) issues.push('未填写设计需求')
    } else if (taskKind.value === 'PURCHASE_TASK') {
      if (!f.category) issues.push('未选择产品款式编码')
      if (!f.productName) issues.push('未填写产品名称')
      if (!f.prefillSpecText?.trim()) issues.push('未填写规格尺寸')
      if (f.purchaseQuantity == null) issues.push('未填写采购数量')
      if (f.costPriceMode === 'manual' && (f.costPriceAmount == null || Number.isNaN(f.costPriceAmount))) {
        issues.push('成本计价方式为手动录入时未填写成本')
      }
    } else if (taskKind.value === 'RETOUCH_TASK') {
      if ((f.referenceFileRefs ?? []).length === 0) issues.push('请上传图片/附件')
      if (!f.designRequirement?.trim()) issues.push('未填写修改要求')
    } else if (taskKind.value === 'ORIGINAL_PRODUCT_DEV') {
      if (!f.sku) issues.push('未绑定原品 SKU')
      if (!f.productId) issues.push('未选择 ERP 产品')
      if (!f.designRequirement?.trim()) issues.push('未填写修改要求')
    }
  } else {
    if ((f.batchItems ?? []).length === 0) issues.push('请先上传并解析 Excel')
    if (excelViolations.value.length > 0) issues.push('Excel 存在行级错误，请修正后重新上传')
  }
  if (requiresDesignSource.value && !designSourceVerified.value) issues.push('请先校验设计源')
  if (requiresErpVerification.value && !erpProductVerified.value) issues.push('请先校验 ERP 商品')

  // 若 canSubmit 为 true，则忽略上面推断的 issues；右侧仅展示最多 3 条关键阻塞原因
  return canSubmit.value ? [] : issues.slice(0, 3)
})

const canSubmit = computed(() => {
  if (form.value.skuMode === 'multiple' && taskKind.value !== 'ORIGINAL_PRODUCT_DEV') {
    return Boolean(
      form.value.groupId &&
        form.value.dueAt &&
        (form.value.batchItems ?? []).length >= 2 &&
        excelViolations.value.length === 0,
    )
  }
  if (requiresDesignSource.value && !designSourceVerified.value) return false
  if (requiresErpVerification.value && !erpProductVerified.value) return false
  return canSubmitTask(taskKind.value, form.value)
})

function onExcelParsed(payload: { preview: BatchPreviewRow[]; violations: BatchViolation[] }) {
  batchPreviewRows.value = payload.preview
  excelViolations.value = payload.violations
  form.value.batchItems = payload.preview.map((row, idx) => ({
    clientKey: `excel-${idx + 1}`,
    productName: row.product_name ?? '',
    designRequirement: row.design_requirement ?? '',
    productIId: row.product_i_id ?? undefined,
    referenceFileRefs: (row.reference_file_refs ?? []).map((ref) => ({ ...ref })) as Record<string, unknown>[],
  }))
  form.value.batchTemplateSaved = payload.preview.length > 0
}

function onExcelReset() {
  batchPreviewRows.value = []
  excelViolations.value = []
  form.value.batchItems = []
  form.value.batchTemplateSaved = false
}

function handleModalUpdate(nextOpen: boolean) {
  if (nextOpen) {
    emit('update:modelValue', true)
    return
  }
  if (isDraftDirty.value) {
    showCloseConfirm.value = true
    return
  }
  emit('update:modelValue', false)
}

function buildDraftPayload() {
  return {
    task_type: taskKind.value,
    payload: {
      task_kind: taskKind.value,
      form: form.value as unknown as Record<string, unknown>,
    },
  }
}

function resolveCreateTypeFromDraft(
  draftTaskKind: string,
  draftForm: Record<string, unknown>,
): (typeof createTypeOptions)[number] | undefined {
  const isCustomization = Boolean(draftForm.customizationRequired)
  const skuMode = String(draftForm.skuMode ?? 'single')
  if (draftTaskKind === 'ORIGINAL_PRODUCT_DEV') {
    const value = isCustomization ? 'customer_customization' : 'original'
    return createTypeOptions.find((option) => option.value === value)
  }
  if (draftTaskKind === 'NEW_PRODUCT_DEV') {
    const value = skuMode === 'multiple' ? 'new_batch' : isCustomization ? 'regular_customization' : 'new_single'
    return createTypeOptions.find((option) => option.value === value)
  }
  if (draftTaskKind === 'PURCHASE_TASK') {
    return createTypeOptions.find((option) => option.value === 'purchase_single')
  }
  if (draftTaskKind === 'RETOUCH_TASK') {
    return createTypeOptions.find((option) => option.value === 'retouch')
  }
  return undefined
}

function normalizeDraftForm(payload: unknown): Record<string, unknown> {
  if (!payload || typeof payload !== 'object') return {}
  const body = payload as Record<string, unknown>
  const form = body.form
  return form && typeof form === 'object' ? (form as Record<string, unknown>) : {}
}

async function hydrateDraft(id: string): Promise<void> {
  loadingDraft.value = true
  try {
    const draft = await getDraftById(id)
    const draftPayload = draft.payload && typeof draft.payload === 'object' ? draft.payload : {}
    const draftTaskKind = String(
      (draftPayload as Record<string, unknown>).task_kind ?? draft.task_type ?? taskKind.value,
    ).trim()
    const draftForm = normalizeDraftForm(draftPayload)
    const createTypeOption = resolveCreateTypeFromDraft(draftTaskKind, draftForm)
    if (createTypeOption) {
      taskGroup.value = createTypeOption.group
      applyCreateType(createTypeOption)
    }
    const merged = { ...form.value, ...draftForm } as TaskCreateFormModel
    if (!Array.isArray(merged.referenceFileRefs)) merged.referenceFileRefs = []
    if (!Array.isArray(merged.batchItems)) merged.batchItems = []
    merged.skuMode = merged.skuMode === 'multiple' ? 'multiple' : 'single'
    form.value = merged
    draftId.value = draft.id
    submitError.value = ''
    savedDraftSnapshot.value = JSON.stringify(buildDraftPayload())
  } catch (error) {
    submitError.value = error instanceof Error ? error.message : '草稿加载失败，请稍后重试'
  } finally {
    loadingDraft.value = false
  }
}

async function saveDraft() {
  const payload = buildDraftPayload()
  const next = draftId.value ? await updateDraft(draftId.value, payload) : await createDraft(payload)
  draftId.value = next.id
  savedDraftSnapshot.value = JSON.stringify(buildDraftPayload())
}

async function saveAndClose() {
  await saveDraft()
  showCloseConfirm.value = false
  emit('update:modelValue', false)
}

function discardAndClose() {
  showCloseConfirm.value = false
  draftId.value = ''
  savedDraftSnapshot.value = ''
  emit('update:modelValue', false)
}

async function prepareSkuPreview() {
  if (taskKind.value === 'ORIGINAL_PRODUCT_DEV') return
  if (!canSubmit.value || preparingSku.value) return

  preparingSku.value = true
  try {
    const currentUser = permissionsStore.currentUser
    const businessType = taskKind.value
    const isBatch = form.value.skuMode === 'multiple'

    const referenceFileRefs = (
      isBatch ? (batchTemplateModel.value.referenceFileRefs ?? []) : form.value.referenceFileRefs
    ).filter(
      (ref): ref is string | Record<string, unknown> =>
        typeof ref === 'string' || (typeof ref === 'object' && ref !== null),
    )

    const normalizedPriority = normalizePriorityForApi(form.value.priority)
    const topCategoryCode = String(
      isBatch
        ? (batchTemplateModel.value.categoryCode ?? form.value.category ?? '')
        : (form.value.category ?? form.value.productCategoryCode ?? ''),
    ).trim()
    if (!topCategoryCode) {
      throw new Error('请先选择产品款式编码，再预展示 SKU')
    }

    const base: Partial<Task> = {
      sku: form.value.sku,
      productId: form.value.productId,
      productName: form.value.productName,
      productSource: 'new',
      taskType: businessType,
      assigneeId: form.value.assigneeId,
      assigneeName: form.value.assigneeName,
      requesterId: currentUser?.id ?? 'anonymous',
      requesterName: currentUser?.name ?? '未知用户',
      groupId: form.value.groupId,
      groupName: form.value.groupName,
      ownerDepartment: resolveOwnerDepartmentForSubmit(),
      ownerOrgTeam: form.value.groupId || undefined,
      designRequirement: businessType === 'PURCHASE_TASK' ? undefined : form.value.designRequirement || undefined,
      referenceFileRefs: referenceFileRefs as unknown as Task['referenceFileRefs'],
      dueAt: form.value.dueAt,
      priority: normalizedPriority,
      customizationRequired: false,
      customizationSourceType: undefined,
      note: form.value.note,
      assetVersions: [],
      businessType,
      requiresAssetVersions: businessType !== 'PURCHASE_TASK',
      ...(topCategoryCode ? { category: topCategoryCode } : {}),
      purchaseInfo:
        businessType === 'PURCHASE_TASK'
          ? {
              status: 'PendingPurchase',
              supplierName: form.value.purchaseSupplierName ?? '',
              quantity: form.value.purchaseQuantity,
              unit: form.value.purchaseUnit ?? undefined,
              purchasePrice: (() => {
                const pick =
                  form.value.purchasePriceAmount != null && !Number.isNaN(form.value.purchasePriceAmount)
                    ? form.value.purchasePriceAmount
                    : form.value.costPriceAmount != null && !Number.isNaN(form.value.costPriceAmount)
                      ? form.value.costPriceAmount
                      : form.value.basePriceAmount != null && !Number.isNaN(form.value.basePriceAmount)
                        ? form.value.basePriceAmount
                        : undefined
                return pick != null ? { amount: pick, currency: form.value.purchasePriceCurrency || 'CNY' } : undefined
              })(),
              expectedArrivalAt: form.value.purchaseExpectedAt ?? undefined,
              warehouseLocationCode: form.value.warehouseLocationCode ?? undefined,
              warehouseLocationName: form.value.warehouseLocationName ?? undefined,
            }
          : undefined,
      costPrice:
        businessType === 'PURCHASE_TASK' && form.value.costPriceAmount != null
          ? { amount: form.value.costPriceAmount, currency: form.value.costPriceCurrency || 'CNY' }
          : undefined,
    }

    if (businessType === 'PURCHASE_TASK') {
      Object.assign(base, {
        costPriceMode: form.value.costPriceMode,
      })
    }

    if (businessType === 'NEW_PRODUCT_DEV') {
      Object.assign(base, {
        category: topCategoryCode || form.value.category,
        material: form.value.material,
        materialOther: form.value.materialOther,
        productShortName: form.value.productShortName,
      })
    }

    if (isBatch) {
      const normalizedItems = form.value.batchItems ?? []
      Object.assign(base, {
        skuMode: 'multiple',
        sku: null,
        productName: '',
        designRequirement: undefined,
        batchItems: normalizedItems,
        ...(businessType === 'PURCHASE_TASK'
          ? {
              purchaseInfo: undefined,
              costPrice: undefined,
              basePriceAmount: undefined,
            }
          : {}),
      })
    }

    // prepare-product-codes 端点当前仍使用 category_code 字段做编码前缀；
    // 这里传入的值已是 i_id 选择结果，保持过渡期兼容。
    const res = await tasksStore.prepareProductCodes(base, undefined)
    if (!isBatch) {
      form.value.sku = res.skuCode ?? null
    } else {
      const skuItems = res.skuItems ?? []
      const items = form.value.batchItems ?? []
      items.forEach((it, idx) => {
        const code = skuItems[idx]
        if (businessType === 'PURCHASE_TASK') it.purchaseSku = code ?? undefined
        else it.newSku = code ?? undefined
      })
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : '预展示 SKU 失败'
    submitError.value = msg
  } finally {
    preparingSku.value = false
  }
}

function getPrefillBusinessPatchPayload(): Record<string, unknown> {
  if (!ARCHIVE_PREFILL_TASK_KIND_WHITELIST.includes(taskKind.value)) return {}

  const f = form.value
  const tpl = f.batchTemplate
  const categoryFromForm = isBatchLayout.value
    ? (tpl?.categoryCode ?? '')
    : (f.category ?? '')
  const category = String(categoryFromForm).trim()
  const specText = (f.prefillSpecText ?? '').trim()

  let costPrice: number | undefined
  if (taskKind.value === 'PURCHASE_TASK') {
    const n = isBatchLayout.value ? tpl?.costPriceAmount : f.costPriceAmount
    if (typeof n === 'number' && Number.isFinite(n)) costPrice = n
  }

  const patch: Record<string, unknown> = {}
  Object.assign(patch, buildCategoryPatchFields(category))
  if (specText) patch.spec_text = specText
  if (typeof costPrice === 'number' && Number.isFinite(costPrice)) patch.cost_price = costPrice

  return patch
}

async function submit() {
  if (!canSubmit.value || submitting.value) return
  submitError.value = ''
  batchItemsError.value = ''
  fieldErrors.value = {}
  submitting.value = true

  // Round I.g · D1：submit-guard 兜底，避免脏 groupId 绕过下拉过滤直达 axios。
  const preflightOwnerDepartment = resolveOwnerDepartmentForSubmit()
  const ownerScopeDeny = validateOwnerScope({
    owner_department: preflightOwnerDepartment,
    owner_org_team: form.value.groupId,
    owner_team: form.value.groupId,
  })
  if (ownerScopeDeny) {
    submitError.value = ownerScopeDeny
    submitting.value = false
    return
  }

  const currentUser = permissionsStore.currentUser
  const now = nowISO()
  const businessType = taskKind.value
  const isBatch = form.value.skuMode === 'multiple' && businessType !== 'ORIGINAL_PRODUCT_DEV'

  const referenceFileRefs = (
    isBatch
      ? (batchTemplateModel.value.referenceFileRefs ?? [])
      : form.value.referenceFileRefs
  ).filter(
    (ref): ref is string | Record<string, unknown> =>
      typeof ref === 'string' || (typeof ref === 'object' && ref !== null),
  )
  const normalizedPriority = normalizePriorityForApi(form.value.priority)
  const topCategoryCode = String(
    isBatch
      ? (batchTemplateModel.value.categoryCode ?? form.value.category ?? form.value.productCategoryCode ?? '')
      : (form.value.category ?? form.value.productCategoryCode ?? ''),
  ).trim()

  const base = {
    sku: form.value.sku,
    productId: form.value.productId,
    productName: form.value.productName,
    erpProductSnapshot: form.value.erpProductSnapshot,
    productSource: businessType === 'ORIGINAL_PRODUCT_DEV' ? 'existing' : 'new',
    taskType: businessType,
    status: form.value.assigneeId ? 'InProgress' : 'PendingAssign',
    assigneeId: form.value.assigneeId,
    assigneeName: form.value.assigneeName,
    designerId: form.value.assigneeId,
    designerName: form.value.assigneeName,
    creatorId: currentUser?.id ?? null,
    creatorName: currentUser?.name ?? null,
    // currentHandlerId / currentHandlerName 为后端投影字段（src/domain/types/task.ts:227），
    // 新建态没有审核节点责任人；前端本地回写会污染 mergeListRowWithCachedDetail 的 ?? fallback，
    // 导致列表刷新后 current_handler_id 永远被脏值兜住、审核队列 pending 过滤漏掉该任务。
    requesterId: currentUser?.id ?? 'anonymous',
    requesterName: currentUser?.name ?? '未知用户',
    groupId: form.value.groupId,
    groupName: form.value.groupName,
    // Round I.g · D1：无组织管理角色的 actor（Ops/Designer/Member）完全不发
    // owner_* 字段，由后端按 actor 归属派生；DA/组长/Global 等按下拉选中值提交。
    ownerDepartment: hideOwnerFields.value ? undefined : preflightOwnerDepartment,
    ownerOrgTeam: hideOwnerFields.value ? undefined : (form.value.groupId || undefined),
    designRequirement:
      businessType === 'PURCHASE_TASK' ? undefined : form.value.designRequirement || undefined,
    referenceFileRefs,
    dueAt: form.value.dueAt,
    priority: normalizedPriority,
    customizationRequired: form.value.customizationRequired,
    customizationSourceType: form.value.customizationSourceType,
    note: form.value.note,
    assetVersions: [],
    businessType,
    workflowLane: taskGroup.value,
    taskCreateType: createType.value,
    orderNumber: form.value.orderNumber,
    copyContent: form.value.copyContent,
    styleKeywords: form.value.styleKeywords,
    designSourceVerified: designSourceVerified.value,
    erpProductVerified: erpProductVerified.value,
    syncErpOnCreate: form.value.syncErpOnCreate !== false,
    ...(topCategoryCode ? { category: topCategoryCode, productCategoryCode: topCategoryCode } : {}),
    requiresAssetVersions: businessType !== 'PURCHASE_TASK',
    createdAt: now,
    updatedAt: now,
    costPrice:
      businessType === 'PURCHASE_TASK' && form.value.costPriceAmount != null
        ? { amount: form.value.costPriceAmount, currency: form.value.costPriceCurrency || 'CNY' }
        : undefined,
    purchaseInfo:
      businessType === 'PURCHASE_TASK'
        ? {
            status: 'PendingPurchase',
            supplierName: form.value.purchaseSupplierName ?? '',
            quantity: form.value.purchaseQuantity,
            unit: form.value.purchaseUnit ?? undefined,
            purchasePrice: (() => {
              const pick =
                form.value.purchasePriceAmount != null && !Number.isNaN(form.value.purchasePriceAmount)
                  ? form.value.purchasePriceAmount
                  : form.value.costPriceAmount != null && !Number.isNaN(form.value.costPriceAmount)
                    ? form.value.costPriceAmount
                    : form.value.basePriceAmount != null && !Number.isNaN(form.value.basePriceAmount)
                      ? form.value.basePriceAmount
                      : undefined
              return pick != null ? { amount: pick, currency: form.value.purchasePriceCurrency || 'CNY' } : undefined
            })(),
            expectedArrivalAt: form.value.purchaseExpectedAt ?? undefined,
            warehouseLocationCode: form.value.warehouseLocationCode ?? undefined,
            warehouseLocationName: form.value.warehouseLocationName ?? undefined,
          }
        : undefined,
  }

  if (businessType === 'PURCHASE_TASK' && !isBatch) {
    Object.assign(base, {
      costPriceMode: form.value.costPriceMode,
    })
  }

  if (businessType === 'NEW_PRODUCT_DEV' && !isBatch) {
    Object.assign(base, {
      category: form.value.category,
      material: form.value.material,
      materialOther: form.value.materialOther,
      productShortName: form.value.productShortName,
    })
  }

  if (isBatch) {
    const normalizedItems = form.value.batchItems ?? []
    Object.assign(base, {
      skuMode: 'multiple',
      sku: null,
      productName: '',
      designRequirement: undefined,
      batchItems: normalizedItems,
      batchExcelImported: true,
      ...(businessType === 'PURCHASE_TASK'
        ? {
            purchaseInfo: undefined,
            costPrice: undefined,
            basePriceAmount: undefined,
          }
        : {}),
    })
  }
  try {
    const created = await tasksStore.addTask(base as unknown as Partial<Task>, actionId.value)
    let procurementSyncFailed = false
    if (businessType === 'PURCHASE_TASK' && !isBatch) {
      const procurementPrice =
        form.value.costPriceAmount != null && Number.isFinite(form.value.costPriceAmount)
          ? form.value.costPriceAmount
          : form.value.purchasePriceAmount != null && Number.isFinite(form.value.purchasePriceAmount)
            ? form.value.purchasePriceAmount
            : undefined
      const procurementQuantity =
        form.value.purchaseQuantity != null && Number.isFinite(form.value.purchaseQuantity)
          ? form.value.purchaseQuantity
          : undefined
      if (
        procurementPrice != null &&
        procurementQuantity != null &&
        procurementQuantity > 0
      ) {
        try {
          const supplierName = String(form.value.purchaseSupplierName ?? '').trim()
          await tasksStore.bootstrapProcurement(created.id, {
            procurement_price: procurementPrice,
            quantity: procurementQuantity,
            ...(supplierName ? { supplier_name: supplierName } : {}),
            purchase_remark: String(form.value.note ?? '').trim() || undefined,
          })
        } catch {
          procurementSyncFailed = true
        }
      } else {
        procurementSyncFailed = true
      }
    }
    const prefillPatchPayload = getPrefillBusinessPatchPayload()
    let prefillSyncFailed = false
    if (Object.keys(prefillPatchPayload).length > 0) {
      try {
        await tasksApi.patchBusinessInfo(created.id, prefillPatchPayload)
      } catch {
        // 不回滚创建结果：引导用户到详情继续补录。
        prefillSyncFailed = true
      }
    }
    // 只刷新当前任务，避免整表刷新覆盖详情页所需字段
    await tasksStore.loadTaskById(created.id)
    emit('update:modelValue', false)
    emit('created', created.id)
    void router.push({
      path: `/tasks/${created.id}`,
      query: {
        fromCreate: '1',
        ...(prefillSyncFailed ? { prefillSyncFailed: '1' } : {}),
        ...(procurementSyncFailed ? { procurementSyncFailed: '1' } : {}),
      },
    })
  } catch (e) {
    const axiosLike = e as { response?: { data?: unknown } }
    const payload = (axiosLike?.response?.data ?? {}) as Record<string, unknown>
    const err = (payload.error ?? {}) as Record<string, unknown>
    const details = (err.details ?? {}) as Record<string, unknown>
    const traceIdRaw = (err.trace_id ?? payload.trace_id) as unknown
    const traceId = typeof traceIdRaw === 'string' && traceIdRaw.trim() ? traceIdRaw.trim() : ''

    const violations = Array.isArray(details.violations)
      ? (details.violations as Array<Record<string, unknown>>)
      : []
    const nextFieldErrors: Record<string, string> = {}
    const humanizedMessages: string[] = []
    for (const v of violations) {
      const fieldRaw = String(v.field ?? '')
      const field = fieldRaw.replace(/\[(\d+)\]/g, '.$1')
      const code = String(v.code ?? '')
      const humanized = humanizeViolationCode(code, fieldRaw)
      const message = humanized || String(v.message ?? v.code ?? '字段校验失败')
      if (field) nextFieldErrors[field] = message
      if (humanized) humanizedMessages.push(humanized)
    }
    fieldErrors.value = nextFieldErrors
    if (nextFieldErrors.batch_items || nextFieldErrors['batch_items.0']) {
      batchItemsError.value = nextFieldErrors.batch_items ?? nextFieldErrors['batch_items.0']
    }

    const forbiddenFields = pickFieldWhitelistViolations(violations)
    const denyCode = String((err.deny_code ?? err.code ?? details.deny_code) ?? '')
    const httpStatus = (axiosLike as { response?: { status?: number } })?.response?.status
    if (humanizedMessages.length > 0) {
      // eslint-disable-next-line no-console
      console.error('[POST /v1/tasks · violations]', { task_type: businessType, violations })
      const lines = humanizedMessages.slice(0, 3).join('；')
      submitError.value = traceId ? `${lines}（追踪编号：${traceId}）` : lines
    } else if (forbiddenFields.length > 0) {
      const cnNames = humanizeTaskCreateFields(forbiddenFields).join('、')
      // eslint-disable-next-line no-console
      console.error('[POST /v1/tasks · field_not_allowed_for_task_type]', {
        task_type: businessType,
        violations,
      })
      const base = `提交字段不符合当前任务类型要求：${cnNames}（任务类型：${businessType}${traceId ? `，追踪编号：${traceId}` : ''}）`
      submitError.value = base
    } else if (
      httpStatus === 403 &&
      (denyCode === 'task_out_of_department_scope' || denyCode === 'PERMISSION_DENIED')
    ) {
      submitError.value = traceId
        ? `您无权在所选部门/组创建任务（追踪编号：${traceId}）`
        : '您无权在所选部门/组创建任务'
    } else {
      const msg = e instanceof Error ? e.message : '创建失败，请稍后重试'
      let fallback: string
      if (msg.includes('timeout') || msg.includes('exceeded')) {
        fallback = '请求超时，请检查网络后重试'
      } else if (msg.includes('500') || msg.includes('Internal Server Error')) {
        fallback = '服务器错误，请稍后重试'
      } else {
        fallback = msg
      }
      submitError.value = traceId ? `${fallback}（追踪编号：${traceId}）` : fallback
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.modal-grid {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  max-height: none;
  min-height: 0;
}
.modal-grid.is-batch {
  max-height: none;
  min-height: 0;
}
.form-section {
  min-width: 0;
  max-height: none;
  min-height: 0;
  overflow: visible;
  padding-right: 0.35rem;
}
.create-type-panel {
  border: 1px solid #e6eaf0;
  border-radius: 1rem;
  padding: 0.75rem;
  background: #f6f7f9;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.create-type-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}
.eyebrow {
  margin: 0 0 0.35rem;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #8a94a3;
}
.create-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 1rem;
  align-items: start;
}
.form-fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
}
.batch-preview-section {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
}
.batch-preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.5rem;
}
.batch-preview-header span {
  color: #8a94a3;
  font-size: 0.72rem;
}
.batch-preview-table-wrap {
  overflow-x: auto;
}
.batch-preview-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.72rem;
}
.batch-preview-table th {
  background: #f4f5f7;
  color: #5b6573;
  font-weight: 800;
}
.batch-preview-table th,
.batch-preview-table td {
  padding: 0.35rem 0.42rem;
  text-align: left;
  border-top: 1px solid #eef1f5;
}
.batch-preview-table tr.has-error {
  background: #fff1f2;
  color: #b91c1c;
}
.batch-cell-ellipsis {
  max-width: 11rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.batch-ref-thumbs {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
  max-width: 7rem;
}
.batch-ref-thumb-item {
  display: inline-flex;
  align-items: center;
}
.batch-ref-thumb-img {
  width: 1.75rem;
  height: 1.75rem;
  object-fit: cover;
  border-radius: 0.25rem;
  border: 1px solid #e6eaf0;
}
.batch-ref-thumb-file {
  max-width: 4rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.62rem;
  color: #5b6573;
  background: #f4f5f7;
  border-radius: 0.25rem;
  padding: 0.1rem 0.25rem;
}
.batch-err-tag {
  display: inline-block;
  margin-right: 0.2rem;
}
.batch-meta-compact {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
  align-items: stretch;
  background: #f8fafc;
}
.batch-meta-card {
  min-width: 0;
  min-height: 4.35rem;
  border: 1px solid #e6eaf0;
  border-radius: 0.75rem;
  padding: 0.5rem;
  background: #fff;
}
.batch-meta-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}
.batch-meta-card :deep(input),
.batch-meta-card :deep(.relative > div) {
  height: 2.4rem;
  border-radius: 0.75rem;
  background: #f8fafc;
}
.batch-meta-card :deep(textarea) {
  min-height: 2.4rem;
  border-radius: 0.75rem;
  background: #f8fafc;
  box-shadow: none;
  resize: vertical;
}
.batch-public {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.875rem;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
  align-items: stretch;
  background: #f8fafc;
}
.batch-public .batch-section-title {
  grid-column: 1 / -1;
}
.batch-field-card {
  min-width: 0;
  min-height: 4.35rem;
  border: 1px solid #e6eaf0;
  border-radius: 0.75rem;
  padding: 0.5rem;
  background: #fff;
}
.batch-field-card.upload-field {
  background: #eef5ff;
}
.batch-public > :deep(.flex.flex-col.gap-1),
.batch-field-card :deep(.flex.flex-col.gap-1) {
  min-width: 0;
}
.batch-two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.batch-template-card {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 0.75rem;
  background: #fafafa;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.template-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}
.template-saved-tag {
  font-size: 0.75rem;
  color: #059669;
  font-weight: 500;
}
.prefill-section {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: #f8fafc;
}
.v1-extra-section,
.meta-card-grid,
.form-card {
  border: 1px solid #e6eaf0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
}
.meta-card-grid,
.form-card {
  min-height: 5.25rem;
}
.meta-card-grid > .field-group,
.meta-card-grid > :deep(.flex.flex-col.gap-1) {
  min-width: 0;
}
.v1-extra-section :deep(.flex.flex-col.gap-1),
.form-card :deep(.flex.flex-col.gap-1),
.batch-public :deep(.flex.flex-col.gap-1),
.batch-field-card :deep(.flex.flex-col.gap-1),
.meta-card-grid :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}
.v1-extra-section :deep(input),
.form-card :deep(input),
.batch-public :deep(input),
.batch-field-card :deep(input),
.meta-card-grid :deep(input),
.v1-extra-section :deep(.relative > div),
.form-card :deep(.relative > div),
.batch-public :deep(.relative > div),
.batch-field-card :deep(.relative > div),
.meta-card-grid :deep(.relative > div) {
  height: 2.75rem;
  border-radius: 0.75rem;
  background: #f8fafc;
}
.batch-field-card :deep(input),
.batch-field-card :deep(.relative > div),
.batch-field-card .native-input {
  height: 2.4rem;
}
.v1-extra-section :deep(textarea),
.form-card :deep(textarea),
.batch-public :deep(textarea),
.batch-field-card :deep(textarea),
.meta-card-grid :deep(textarea) {
  border-radius: 0.75rem;
  background: #f8fafc;
  box-shadow: none;
  resize: vertical;
}
.batch-field-card :deep(textarea) {
  min-height: 2.4rem;
  padding-top: 0.45rem;
  padding-bottom: 0.45rem;
}
.customization-card-grid,
.meta-card-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.customization-card-grid .batch-section-title,
.customization-card-grid :deep(textarea),
.meta-card-grid :deep(textarea) {
  grid-column: 1 / -1;
}
.erp-sync-toggle-card {
  border: 1px solid #d9dee7;
  border-radius: 0.875rem;
  padding: 0.7rem 0.8rem;
  background: linear-gradient(180deg, #f9fafb 0%, #f3f4f6 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}
.erp-sync-toggle-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.3rem;
}
.erp-sync-title {
  color: #111827;
  font-weight: 600;
}
.erp-sync-badge {
  font-size: 0.65rem;
  line-height: 1;
  padding: 0.2rem 0.4rem;
  border-radius: 999px;
  color: #6b7280;
  background: #e5e7eb;
  border: 1px solid #d1d5db;
}
.erp-sync-toggle-row {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}
.erp-sync-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}
.erp-sync-main-label {
  font-size: 0.82rem;
  font-weight: 500;
  color: #1f2937;
}
.erp-sync-toggle-hint {
  margin: 0;
  font-size: 0.72rem;
  color: #64748b;
  line-height: 1.35;
}
.erp-sync-toggle-hint.warning {
  color: #b45309;
}
.erp-sync-toggle-card :deep(.erp-switch) {
  width: auto;
  min-width: 4.7rem;
  justify-content: center;
}
.erp-sync-toggle-card :deep(.erp-switch[aria-pressed='true']) {
  background: #111827;
  color: #f9fafb;
}
.erp-sync-toggle-card :deep(.erp-switch[aria-pressed='false']) {
  background: #e5e7eb;
  color: #4b5563;
}
.erp-sync-toggle-card :deep(.erp-switch[aria-pressed='true'] span.inline-block) {
  background: #22c55e;
}
.retouch-form {
  display: grid;
  grid-template-columns: 0.95fr 1.05fr;
  gap: 0.75rem;
}
.retouch-form .upload-card {
  background: #eef5ff;
}
.batch-section-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #0f172a;
}
.batch-items-error {
  margin: 0;
  font-size: 0.8125rem;
  color: #dc2626;
}
.batch-bridge-hint {
  margin: 0;
  padding: 0.35rem 0.5rem;
  border-radius: 6px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  color: #475569;
  font-size: 0.75rem;
  line-height: 1.35;
}
.field-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #334155;
}
.required {
  color: #dc2626;
}
.task-kind-switch {
  @apply inline-flex items-center gap-1 rounded-xl bg-white/70 p-1 whitespace-nowrap overflow-x-auto;
}
.task-kind-button {
  @apply inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs transition-all duration-200 active:scale-95 whitespace-nowrap;
}
.task-kind-button.is-inactive {
  @apply text-slate-500 hover:text-slate-900 hover:bg-white/40;
}
.task-kind-button.is-active {
  @apply bg-white shadow-md text-slate-900 font-bold;
}
.mode-switch {
  display: inline-flex;
  gap: 0.25rem;
  padding: 0.25rem;
  border: 1px solid #e6eaf0;
  border-radius: 0.75rem;
  background: #fff;
}
.field-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.field-hint-error {
  color: #dc2626;
}
.submit-error-banner {
  margin-bottom: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  font-size: 0.875rem;
  color: #dc2626;
}
.native-input {
  width: 100%;
  padding: 0.4rem 0.625rem;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 0.875rem;
}
.switch-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: #334155;
}
.submit-check-section {
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
}
.submit-check-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
}
.submit-check-hint {
  margin: 0;
  font-size: 0.78rem;
  color: #64748b;
}
.summary-title {
  margin: 0 0 0.25rem;
  font-size: 0.8rem;
  font-weight: 600;
  color: #0f172a;
}
.issue-list {
  margin: 0.15rem 0 0;
  padding-left: 1.1rem;
  font-size: 0.78rem;
  color: #b91c1c;
}
.issue-item::marker {
  color: #f97316;
}
.issue-ok {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: #16a34a;
}
.issue-warn {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: #b45309;
}
.summary-footer {
  flex-shrink: 0;
  background: #fff;
  padding: 1rem 1.25rem;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}
.submit-btn {
  min-width: 6.75rem;
}
.create-context-panel {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
  max-height: calc(94vh - 7.5rem);
  overflow-y: auto;
  border-radius: 1rem;
  border: 1px solid #e6eaf0;
  background: #f7f8fa;
  padding: 0.875rem;
}
.create-context-panel.is-customization {
  background: #faf7ff;
}
.create-context-panel.is-retouch {
  background: #171c22;
  color: #fff;
}
.context-panel-header {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
}
.context-panel-header h4 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 800;
  color: #171c22;
}
.create-context-panel.is-retouch .context-panel-header h4,
.create-context-panel.is-retouch .eyebrow {
  color: #fff;
}
.context-dot {
  width: 0.7rem;
  height: 0.7rem;
  margin-top: 0.2rem;
  border-radius: 999px;
  background: #2f80ed;
  box-shadow: 0 0 0 4px rgba(47, 128, 237, 0.12);
}
.create-context-panel.is-customization .context-dot {
  background: #7c3aed;
  box-shadow: 0 0 0 4px rgba(124, 58, 237, 0.12);
}
.context-card-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.context-card {
  border: 1px solid #e6eaf0;
  border-radius: 0.75rem;
  padding: 0.7rem;
  background: #fff;
}
.create-context-panel.is-retouch .context-card {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.08);
}
.context-card-title {
  margin: 0 0 0.25rem;
  font-size: 0.78rem;
  font-weight: 800;
  color: #171c22;
}
.context-card-body {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.45;
  color: #5b6573;
}
.create-context-panel.is-retouch .context-card-title,
.create-context-panel.is-retouch .context-card-body {
  color: #fff;
}
@media (max-width: 900px) {
  .create-workspace {
    grid-template-columns: 1fr;
  }
  .create-context-panel {
    position: static;
  }
  .customization-card-grid,
  .meta-card-grid,
  .retouch-form,
  .batch-meta-compact,
  .batch-two-col {
    grid-template-columns: 1fr;
  }
}
</style>
