<template>
  <BaseModal
    :model-value="modelValue"
    title="创建定制任务"
    :show-confirm="false"
    panel-class="max-w-[min(1060px,96vw)]"
    @update:model-value="onModalToggle"
  >
    <div class="dialog-body">
      <p v-if="submitError" class="error-banner">{{ submitError }}</p>

      <div class="dense-two-col">
        <section class="dense-col">
          <div class="field-group">
            <label class="field-label">任务类型 *</label>
            <div class="task-kind-switch">
              <button
                type="button"
                class="task-kind-button"
                :class="taskType === 'NEW_PRODUCT_DEV' ? 'is-active' : 'is-inactive'"
                :disabled="submitting"
                @click="taskType = 'NEW_PRODUCT_DEV'"
              >
                新品开发
              </button>
              <button
                type="button"
                class="task-kind-button"
                :class="taskType === 'ORIGINAL_PRODUCT_DEV' ? 'is-active' : 'is-inactive'"
                :disabled="submitting"
                @click="taskType = 'ORIGINAL_PRODUCT_DEV'"
              >
                原有产品改进
              </button>
            </div>
          </div>

          <div class="field-group">
            <label class="field-label">订单号 *</label>
            <input
              v-model.trim="form.orderNumber"
              type="text"
              class="field-input"
              placeholder="请输入关联订单号"
              :disabled="submitting"
            />
            <p v-if="showValidationError && !form.orderNumber.trim()" class="field-hint field-hint-error">
              请填写订单号
            </p>
          </div>

          <div class="field-group">
            <label class="field-label">定制需求描述 *</label>
            <textarea
              v-model="form.designRequirement"
              class="field-textarea"
              rows="3"
              placeholder="详细描述需要的定制点..."
              :disabled="submitting"
            />
            <p v-if="showValidationError && !form.designRequirement.trim()" class="field-hint field-hint-error">
              请填写设计需求说明
            </p>
          </div>

          <section class="upload-section">
            <div class="upload-section-head">
              <h4 class="section-title">附件</h4>
              <p class="section-subtitle">源文件可选，参考图建议上传 1~3 张</p>
            </div>
            <div
              class="source-upload-card"
              tabindex="0"
              @focusin="activateSourceFileReceiver"
              @pointerenter="activateSourceFileReceiver"
              @dragover.prevent="onSourceDragOver"
              @drop.prevent="onSourceDrop"
              @paste="onSourcePaste"
            >
              <div class="source-upload-head">
                <span class="source-upload-title">源文件（可选）</span>
                <button
                  type="button"
                  class="source-pick-btn"
                  :disabled="submitting"
                  @click="triggerSourcePick"
                >
                  选择/拖拽/粘贴源文件
                </button>
              </div>
              <p class="source-upload-hint">
                创建后按 `source` 资产链路上传（单文件上限 {{ maxSourceFileMb }}MB）。
              </p>
              <input
                ref="sourceInputRef"
                type="file"
                class="hidden-input"
                :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                multiple
                @change="onSourceFileChange"
              />
              <ul v-if="sourceFiles.length" class="source-file-list">
                <li v-for="item in sourceFiles" :key="item.key" class="source-file-item">
                  <div class="source-file-main">
                    <span class="source-file-name" :title="item.file.name">{{ item.file.name }}</span>
                    <span class="source-file-meta">{{ prettyFileSize(item.file.size) }}</span>
                    <span class="source-file-status" :class="'status-' + item.status">
                      {{ sourceStatusLabel(item.status) }}
                    </span>
                  </div>
                  <button
                    type="button"
                    class="source-remove-btn"
                    :disabled="submitting || item.status === 'uploading'"
                    @click="removeSourceFile(item.key)"
                  >
                    删除
                  </button>
                </li>
              </ul>
              <p v-else class="source-empty">暂未选择源文件</p>
            </div>
            <div class="refs-wrap">
              <ReferenceUploadPanel v-model="form.referenceFileRefs" />
            </div>
          </section>
        </section>

        <section class="dense-col">
          <div class="field-group">
            <label class="field-label">所属运营组 *</label>
            <div class="select-wrap">
              <select v-model="form.groupId" class="field-select" :disabled="submitting">
                <option value="">请选择归属运营组</option>
                <option
                  v-for="option in groupOptions"
                  :key="`team-${option.value}`"
                  :value="stringValue(option.value)"
                >
                  {{ option.label }}
                </option>
              </select>
            </div>
            <p v-if="showValidationError && !form.groupId.trim()" class="field-hint field-hint-error">
              请选择所属运营组
            </p>
            <p v-if="ownerScopeLockHint" class="field-hint">{{ ownerScopeLockHint }}</p>
            <p v-if="teamLoadError" class="field-hint field-hint-error">{{ teamLoadError }}</p>
          </div>

          <div class="field-group">
            <label class="field-label">指派设计师（可选）</label>
            <div class="select-wrap">
              <select v-model="form.assigneeId" class="field-select" :disabled="submitting">
                <option value="">请选择设计师</option>
                <option
                  v-for="option in assigneeOptions"
                  :key="`designer-${option.value}`"
                  :value="stringValue(option.value)"
                >
                  {{ option.label }}
                </option>
              </select>
            </div>
            <p v-if="designersLoadError" class="field-hint field-hint-error">{{ designersLoadError }}</p>
          </div>

          <section class="type-section">
            <template v-if="taskType === 'ORIGINAL_PRODUCT_DEV'">
              <div class="field-group">
                <label class="field-label">产品信息 *</label>
                <button
                  type="button"
                  class="product-picker-field"
                  :disabled="submitting"
                  @click="showProductPicker = true"
                >
                  {{ form.productName || '从 ERP 选择产品' }}
                </button>
                <p class="field-hint">沿用任务中心 ERP 选品逻辑，SKU 不可手填。</p>
                <div v-if="form.productId" class="selected-product-card">
                  <div class="selected-product-row">
                    <span class="field-label">产品名称</span>
                    <span class="field-value">{{ form.productName || '—' }}</span>
                  </div>
                  <div class="selected-product-row">
                    <span class="field-label">SKU</span>
                    <span class="field-value">{{ form.skuCode || '—' }}</span>
                  </div>
                  <div class="selected-product-row">
                    <span class="field-label">产品分类</span>
                    <span class="field-value">{{ originalProductCategoryDisplay }}</span>
                  </div>
                </div>
              </div>
            </template>

            <template v-else>
              <div class="new-product-grid">
                <div class="field-group">
                  <label class="field-label">产品分类编码 *</label>
                  <div class="select-wrap">
                    <select v-model="form.newCategoryCode" class="field-select" :disabled="submitting">
                      <option value="">请选择分类</option>
                      <option
                        v-for="option in categoryOptions"
                        :key="`category-${option.value}`"
                        :value="stringValue(option.value)"
                      >
                        {{ option.label }}
                      </option>
                    </select>
                  </div>
                  <p v-if="showValidationError && !form.newCategoryCode.trim()" class="field-hint field-hint-error">
                    请选择产品分类编码
                  </p>
                </div>

                <div class="field-group">
                  <label class="field-label">产品名称 *</label>
                  <input
                    v-model.trim="form.productName"
                    type="text"
                    class="field-input"
                    placeholder="请输入目标产品名称"
                    :maxlength="ERP_PRODUCT_NAME_MAX_LENGTH"
                    :disabled="submitting"
                  />
                  <p v-if="showValidationError && !form.productName.trim()" class="field-hint field-hint-error">
                    请填写产品名称
                  </p>
                  <p class="field-hint" :class="{ 'field-hint-error': isErpProductNameTooLong(form.productName) }">
                    {{ erpProductNameHint(form.productName) }}
                  </p>
                </div>

                <div class="field-group">
                  <label class="field-label">产品简称</label>
                  <div class="field-readonly" :title="form.productName || '填写产品名称后自动同步'">
                    {{ form.productName || '填写产品名称后自动同步' }}
                  </div>
                  <p class="field-hint">系统会把产品名称同步为 ERP 简称，两个名称保持一致。</p>
                </div>
              </div>
            </template>
          </section>

          <div class="meta-row-grid">
            <div class="field-group">
              <label class="field-label">任务截止时间 *</label>
              <input
                v-model="dueAtLocal"
                type="date"
                class="field-input"
                :min="dueAtMin"
                :disabled="submitting"
              />
              <p v-if="showValidationError && !dueAtIso" class="field-hint field-hint-error">
                请填写任务截止时间
              </p>
            </div>

            <div class="field-group">
              <label class="field-label">优先级</label>
              <div class="select-wrap">
                <select v-model="form.priority" class="field-select" :disabled="submitting">
                  <option
                    v-for="option in priorityOptions"
                    :key="`priority-${option.value}`"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </option>
                </select>
              </div>
            </div>
          </div>

          <!--
            Round I.g · D3：material_* 只对 task_type=new_product_development 合法；
            原品改款定制禁止收集该字段，避免 `field_not_allowed_for_task_type` 400。
          -->
          <div v-if="taskType === 'NEW_PRODUCT_DEV'" class="field-group">
            <label class="field-label">产品材质（可选）</label>
            <input
              v-model.trim="form.material"
              type="text"
              class="field-input"
              placeholder="请输入材质，可不填"
              :disabled="submitting"
            />
          </div>
        </section>
      </div>
    </div>

    <ProductPickerDialog
      v-model="showProductPicker"
      @select="onProductSelect"
    />

    <template #footer>
      <footer class="modal-footer">
        <button
          type="button"
          class="footer-btn footer-btn-cancel"
          :disabled="submitting"
          @click="closeDialog"
        >
          取消
        </button>
        <button
          type="button"
          class="footer-btn footer-btn-create"
          :disabled="!canSubmit || submitting"
          @click="submit"
        >
          {{ submitting ? '创建中...' : '创建任务' }}
        </button>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { Product } from '@/types'
import BaseModal from '@/components/base/BaseModal.vue'
import ProductPickerDialog from '@/components/products/ProductPickerDialog.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import { DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES, DESIGN_UPLOAD_MAX_FILE_SIZE_MB } from '@/domain/copy/design-upload'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { useTeamOptions } from '@/composables/useTeamOptions'
import { useActorOwnerScope } from '@/composables/useActorOwnerScope'
import { createCustomizationTask, type CustomizationTaskCreatePayload } from '@/services/api/customizationApi'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { formatUploadFailureMessage } from '@/utils/upload-errors'
import { usePermissionsStore } from '@/stores/permissions'
import { humanizeTaskCreateFields, humanizeViolationCode, pickFieldWhitelistViolations } from '@/domain/task-create-fields'
import { UPLOAD_ACCEPT_ATTRIBUTE, isAllowedUploadFile } from '@/domain/constants/upload-types'
import {
  ERP_PRODUCT_NAME_MAX_LENGTH,
  erpProductNameHint,
  isErpProductNameTooLong,
} from '@/domain/erp-product-name'
import { getBeijingDateString, toBeijingEndOfDayISO } from '@/utils/date'
import {
  getFilesFromClipboardEvent,
  getFilesFromDataTransfer,
  hasFileDataTransfer,
  useFileDropPasteReceiver,
} from '@/composables/useFileDropPasteReceiver'

type CreateTaskType = 'ORIGINAL_PRODUCT_DEV' | 'NEW_PRODUCT_DEV'
type SourceUploadStatus = 'pending' | 'uploading' | 'uploaded' | 'failed'

interface SourceFileItem {
  key: string
  file: File
  status: SourceUploadStatus
}

interface CreateResultPayload {
  taskId: string
  sourceUploadFailed: number
}

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  created: [payload: CreateResultPayload]
}>()

const permissionsStore = usePermissionsStore()
const { options: categoryOptions } = useCategoryOptions({ requiredActions: ['task.create'] })
const { teamOptions: rawGroupOptions, loadError: teamLoadError, resolveDepartmentByTeam } = useTeamOptions()
const {
  isOwnerScopeUnrestricted,
  hideOwnerFields,
  lockOwnerDepartment,
  lockOwnerTeam,
  allowedCreateOwnerDepartments,
  allowedCreateOwnerTeams,
  defaultOwnerTeam,
  filterOwnerTeamOptions,
  validateOwnerScope,
} = useActorOwnerScope()

const groupOptions = computed(() => filterOwnerTeamOptions(rawGroupOptions.value, resolveDepartmentByTeam))
const ownerScopeLockHint = computed(() => {
  if (isOwnerScopeUnrestricted.value) return ''
  if (lockOwnerDepartment.value) {
    return `已锁定本部门：${(allowedCreateOwnerDepartments.value ?? []).join('、') || '—'}`
  }
  if (lockOwnerTeam.value) {
    return `已锁定本组：${(allowedCreateOwnerTeams.value ?? []).join('、') || '—'}`
  }
  return ''
})
const { assigneeOptions, loadDesigners, loadError: designersLoadError } = useDesignerOptions({
  includeEmpty: true,
  requiredActions: ['task.create'],
  workflowLane: 'customization',
})

const taskType = ref<CreateTaskType>('ORIGINAL_PRODUCT_DEV')
const showProductPicker = ref(false)
const submitting = ref(false)
const submitError = ref('')
const showValidationError = ref(false)
const sourceInputRef = ref<HTMLInputElement | null>(null)
const sourceFiles = ref<SourceFileItem[]>([])

const { activateFileReceiver: activateSourceFileReceiver } = useFileDropPasteReceiver({
  enabled: computed(() => props.modelValue && !submitting.value),
  onFiles: (files) => {
    addSourceFiles(files)
  },
})

const form = reactive({
  orderNumber: '',
  designRequirement: '',
  referenceFileRefs: [] as (Record<string, unknown> | string)[],
  groupId: '',
  assigneeId: '',
  priority: 'normal',
  dueAt: '',
  material: '',
  productId: null as string | null,
  skuCode: '',
  productName: '',
  productShortName: '',
  productCategoryCode: '',
  productCategoryName: '',
  erpProductSnapshot: undefined as Record<string, unknown> | undefined,
  newCategoryCode: '',
})

const priorityOptions = [
  { value: 'low', label: '低' },
  { value: 'normal', label: '普通' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '加急' },
]

const maxSourceFileMb = DESIGN_UPLOAD_MAX_FILE_SIZE_MB

function stringValue(value: string | number | null | undefined): string {
  return value == null ? '' : String(value)
}

const categoryLabelMap = computed(() => {
  const out = new Map<string, string>()
  for (const option of categoryOptions.value) {
    const value = String(option.value ?? '').trim()
    const label = String(option.label ?? '').trim()
    if (!value || !label) continue
    out.set(value, label)
    out.set(value.toUpperCase(), label)
  }
  return out
})

function isUnclassifiedText(raw: string | null | undefined): boolean {
  const text = String(raw ?? '').trim().toLowerCase()
  if (!text) return false
  return text === '未分类' || text === 'unclassified' || text === 'uncategorized'
}

function normalizeCategoryCodeText(raw: string | null | undefined): string {
  const text = String(raw ?? '').trim()
  if (!text) return ''
  if (text.startsWith('[') && text.endsWith(']')) {
    return text.slice(1, -1).trim()
  }
  return text
}

const originalProductCategoryDisplay = computed(() => {
  const categoryCode = normalizeCategoryCodeText(form.productCategoryCode)
  const categoryName = form.productCategoryName.trim()
  if (isUnclassifiedText(categoryCode) || isUnclassifiedText(categoryName)) {
    return '未分类'
  }
  if (categoryName) {
    const mappedByName =
      categoryLabelMap.value.get(categoryName) ??
      categoryLabelMap.value.get(categoryName.toUpperCase()) ??
      ''
    if (mappedByName && mappedByName !== categoryName) {
      return `${categoryName}（${mappedByName}）`
    }
    return categoryName
  }
  if (categoryCode) {
    const mapped =
      categoryLabelMap.value.get(categoryCode) ??
      categoryLabelMap.value.get(categoryCode.toUpperCase()) ??
      ''
    if (mapped && mapped !== categoryCode) {
      return `${mapped}（${categoryCode}）`
    }
    return categoryCode
  }
  if (categoryName) return categoryName
  // ERP 选品列表里若显示为「未分类」，常见场景是分类字段为空；已选 ERP 产品时统一按未分类展示。
  if (form.productId) return '未分类'
  return '未分类'
})

const dueAtLocal = computed({
  get: () => form.dueAt,
  set: (value: string) => {
    form.dueAt = value
  },
})

const dueAtMin = computed(() => {
  return getBeijingDateString()
})

const dueAtIso = computed(() => {
  return toBeijingEndOfDayISO(form.dueAt) ?? ''
})

const canSubmit = computed(() => {
  if (!form.orderNumber.trim()) return false
  if (!form.designRequirement.trim()) return false
  if (!form.groupId.trim()) return false
  if (!dueAtIso.value) return false
  if (taskType.value === 'ORIGINAL_PRODUCT_DEV') {
    return Boolean(form.productId && form.skuCode.trim())
  }
  return Boolean(
    form.newCategoryCode.trim() &&
      form.productName.trim() &&
      !isErpProductNameTooLong(form.productName),
  )
})

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      resetForm()
      loadDesigners()
    }
  },
)

watch(groupOptions, (options) => {
  if (options.length === 0) return
  const values = options.map((o) => String(o.value ?? ''))
  if (!form.groupId || !values.includes(form.groupId)) {
    const preferred =
      defaultOwnerTeam.value && values.includes(defaultOwnerTeam.value)
        ? defaultOwnerTeam.value
        : String(options[0]?.value ?? '')
    form.groupId = preferred
  }
}, { immediate: true })

watch(taskType, (nextType) => {
  form.productId = null
  form.skuCode = ''
  form.productName = ''
  form.productShortName = ''
  form.productCategoryCode = ''
  form.productCategoryName = ''
  form.erpProductSnapshot = undefined
  form.newCategoryCode = ''
  if (nextType === 'ORIGINAL_PRODUCT_DEV') {
    // Round I.g · D3：切回原品时立刻抹掉 material，防止后续提交再 leak 过 sanitizer
    // 之前的中间态（sanitizer 兜底，但 UI 门禁是第一道防线）。
    form.material = ''
    return
  }
  form.productId = null
})

function onModalToggle(next: boolean) {
  emit('update:modelValue', next)
}

function closeDialog() {
  emit('update:modelValue', false)
}

function resetForm() {
  taskType.value = 'ORIGINAL_PRODUCT_DEV'
  form.orderNumber = ''
  form.designRequirement = ''
  form.referenceFileRefs = []
  form.groupId = String(groupOptions.value[0]?.value ?? '')
  form.assigneeId = ''
  form.priority = 'normal'
  form.dueAt = ''
  form.material = ''
  form.productId = null
  form.skuCode = ''
  form.productName = ''
  form.productShortName = ''
  form.productCategoryCode = ''
  form.productCategoryName = ''
  form.erpProductSnapshot = undefined
  form.newCategoryCode = ''
  sourceFiles.value = []
  submitError.value = ''
  showValidationError.value = false
}

function onProductSelect(product: Product) {
  form.productId = product.id
  form.skuCode = product.sku
  form.productName = product.name
  form.productCategoryCode = normalizeCategoryCodeText(product.categoryCode)
  form.productCategoryName = product.category?.trim() || (product.categoryCode?.trim() ? '' : '未分类')
  form.erpProductSnapshot = {
    product_id: product.id,
    sku_code: product.sku ?? '',
    name: product.name ?? '',
    product_name: product.name ?? '',
    category_code: product.categoryCode ?? '',
    category_name: product.category ?? '',
    image_url: product.imageUrl ?? '',
  }
  showProductPicker.value = false
}

function triggerSourcePick() {
  activateSourceFileReceiver()
  sourceInputRef.value?.click()
}

function buildSourceFileKey(file: File): string {
  return `${file.name}::${file.size}::${file.lastModified}`
}

function onSourceFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (files?.length) addSourceFiles(files)
  input.value = ''
}

function onSourceDragOver(event: DragEvent) {
  if (submitting.value || !hasFileDataTransfer(event.dataTransfer)) return
  activateSourceFileReceiver()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onSourceDrop(event: DragEvent) {
  if (submitting.value) return
  const files = getFilesFromDataTransfer(event.dataTransfer)
  if (!files.length) return
  activateSourceFileReceiver()
  addSourceFiles(files)
}

function onSourcePaste(event: ClipboardEvent) {
  if (submitting.value) return
  const files = getFilesFromClipboardEvent(event)
  if (!files.length) return
  event.preventDefault()
  activateSourceFileReceiver()
  addSourceFiles(files)
}

function addSourceFiles(files: FileList | File[]) {
  if (!files?.length) {
    return
  }
  const existing = new Set(sourceFiles.value.map((item) => item.key))
  for (const file of Array.from(files)) {
    if (!isAllowedUploadFile(file.name)) {
      submitError.value = `不支持的文件类型：${file.name}`
      continue
    }
    if (file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES) {
      submitError.value = `${file.name} 超过 ${maxSourceFileMb}MB，已跳过`
      continue
    }
    const key = buildSourceFileKey(file)
    if (existing.has(key)) continue
    sourceFiles.value.push({ key, file, status: 'pending' })
    existing.add(key)
  }
}

function removeSourceFile(key: string) {
  sourceFiles.value = sourceFiles.value.filter((item) => item.key !== key)
}

function prettyFileSize(size: number): string {
  if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${size} B`
}

function sourceStatusLabel(status: SourceUploadStatus): string {
  if (status === 'uploading') return '上传中'
  if (status === 'uploaded') return '已上传'
  if (status === 'failed') return '上传失败'
  return '待上传'
}

function buildCreatePayload(): CustomizationTaskCreatePayload {
  const currentUser = permissionsStore.currentUser
  const isOriginal = taskType.value === 'ORIGINAL_PRODUCT_DEV'
  // Round I.g · D1：hideOwnerFields 时（Ops/Designer 等），owner_* 由后端按
  // actor 归属派生，前端不再在 payload 里插入字符串。
  const ownerTeam = hideOwnerFields.value ? '' : form.groupId
  const ownerDepartment = hideOwnerFields.value ? undefined : resolveDepartmentByTeam(form.groupId)
  const payload: CustomizationTaskCreatePayload = {
    task_type: taskType.value,
    order_number: form.orderNumber.trim(),
    design_requirement: form.designRequirement.trim(),
    due_at: dueAtIso.value,
    owner_team: ownerTeam,
    owner_org_team: ownerTeam || undefined,
    owner_department: ownerDepartment,
    designer_id: form.assigneeId || null,
    requester_id: currentUser?.id ?? null,
    requester_name: currentUser?.name ?? '',
    priority: form.priority as CustomizationTaskCreatePayload['priority'],
    reference_file_refs: form.referenceFileRefs,
    // Round I.g · D3：原品改款禁止携带 material（sanitizer 同样会兜底）。
    material: isOriginal ? undefined : (form.material.trim() || undefined),
  }
  if (isOriginal) {
    payload.product_id = form.productId
    payload.sku_code = form.skuCode || null
    payload.product_name = form.productName
    payload.erp_product_snapshot = form.erpProductSnapshot
    payload.original_product_category_code = form.productCategoryCode || undefined
  } else {
    payload.category_code = form.newCategoryCode
    payload.product_name = form.productName
    payload.product_short_name = form.productName
  }
  return payload
}

async function uploadSourceFiles(taskId: string): Promise<number> {
  let failed = 0
  for (const item of sourceFiles.value) {
    item.status = 'uploading'
    try {
      await uploadTaskFileViaAssetSession(
        taskId,
        item.file,
        { asset_kind: 'source', remark: item.file.name },
      )
      item.status = 'uploaded'
    } catch (error) {
      item.status = 'failed'
      failed += 1
      submitError.value = formatUploadFailureMessage('reference_upload', error)
    }
  }
  return failed
}

async function submit() {
  showValidationError.value = true
  submitError.value = ''
  if (!canSubmit.value || submitting.value) return

  // Round I.g · D1：submit-guard 兜底。
  const payload = buildCreatePayload()
  const backendTaskType =
    taskType.value === 'ORIGINAL_PRODUCT_DEV' ? 'original_product_development' : 'new_product_development'
  const ownerScopeDeny = validateOwnerScope({
    owner_department: payload.owner_department,
    owner_org_team: payload.owner_org_team,
    owner_team: payload.owner_team,
  })
  if (ownerScopeDeny) {
    submitError.value = ownerScopeDeny
    return
  }

  submitting.value = true
  try {
    const created = await createCustomizationTask(payload)
    const sourceUploadFailed = await uploadSourceFiles(created.task_id)
    emit('created', { taskId: created.task_id, sourceUploadFailed })
    emit('update:modelValue', false)
  } catch (error) {
    // Round I.g · D4：映射后端 violations → 中文文案，403 scope backstop。
    const axiosLike = error as { response?: { status?: number; data?: unknown } }
    const body = (axiosLike?.response?.data ?? {}) as Record<string, unknown>
    const err = (body.error ?? {}) as Record<string, unknown>
    const details = (err.details ?? {}) as Record<string, unknown>
    const traceIdRaw = (err.trace_id ?? body.trace_id) as unknown
    const traceId = typeof traceIdRaw === 'string' && traceIdRaw.trim() ? traceIdRaw.trim() : ''
    const violations = Array.isArray(details.violations)
      ? (details.violations as Array<Record<string, unknown>>)
      : []
    const humanizedMessages = violations
      .map((v) => humanizeViolationCode(String(v.code ?? ''), String(v.field ?? '')))
      .filter((message) => message.length > 0)
    const forbiddenFields = pickFieldWhitelistViolations(violations)
    const denyCode = String((err.deny_code ?? err.code ?? details.deny_code) ?? '')
    if (humanizedMessages.length > 0) {
      submitError.value = humanizedMessages.slice(0, 3).join('；')
    } else if (forbiddenFields.length > 0) {
      const cn = humanizeTaskCreateFields(forbiddenFields).join('、')
      // eslint-disable-next-line no-console
      console.error('[POST /v1/tasks · customization · field_not_allowed_for_task_type]', {
        task_type: backendTaskType,
        violations,
      })
      submitError.value = `提交字段不符合当前任务类型要求：${cn}（任务类型：${backendTaskType}${traceId ? `，追踪编号：${traceId}` : ''}）`
    } else if (
      axiosLike?.response?.status === 403 &&
      (denyCode === 'task_out_of_department_scope' || denyCode === 'PERMISSION_DENIED')
    ) {
      submitError.value = traceId
        ? `您无权在所选部门/组创建任务（追踪编号：${traceId}）`
        : '您无权在所选部门/组创建任务'
    } else {
      submitError.value = error instanceof Error ? error.message : '创建任务失败，请稍后重试'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.dialog-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding-bottom: 0.15rem;
}
.error-banner {
  margin: 0;
  padding: 0.65rem 0.8rem;
  border-radius: 0.75rem;
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.8125rem;
}
.dense-two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
  align-items: start;
}
.dense-col {
  display: flex;
  flex-direction: column;
  gap: 0.62rem;
}
.field-group {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.field-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: #1f2937;
}
.task-kind-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.22rem;
  border-radius: 0.66rem;
  background: #e8edf3;
  padding: 0.18rem;
}
.task-kind-button {
  display: inline-flex;
  justify-content: center;
  align-items: center;
  min-width: 5.85rem;
  height: 1.86rem;
  border: 1px solid transparent;
  border-radius: 0.56rem;
  padding: 0 0.7rem;
  font-size: 0.74rem;
  line-height: 1;
  cursor: pointer;
  background: transparent;
  color: #8a95a8;
  font-weight: 500;
}
.task-kind-button.is-active {
  background: #ffffff;
  border-color: #d2dae7;
  color: #111827;
  font-weight: 600;
  box-shadow: 0 1px 1px rgb(15 23 42 / 0.08);
}
.task-kind-button.is-inactive {
  background: transparent;
  color: #8a95a8;
  border-color: transparent;
}
.task-kind-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.product-picker-field {
  width: 100%;
  height: 2.25rem;
  border-radius: 0.72rem;
  border: 1px solid #dbe2ee;
  background: #ffffff;
  color: #334155;
  font-size: 0.8125rem;
  font-weight: 500;
  text-align: left;
  padding: 0 0.75rem;
  cursor: pointer;
}
.product-picker-field:hover:not(:disabled) {
  border-color: #c8d3e7;
}
.product-picker-field:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.type-section,
.upload-section {
  border: 1px solid #dde4ee;
  border-radius: 0.62rem;
  padding: 0.6rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.new-product-grid {
  display: grid;
  gap: 0.52rem;
}
.upload-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
}
.section-title {
  margin: 0;
  font-size: 0.78rem;
  color: #111827;
  font-weight: 700;
}
.section-subtitle {
  margin: 0;
  font-size: 0.72rem;
  color: #94a3b8;
}
.source-upload-card {
  border: 1px dashed #d6deea;
  border-radius: 0.58rem;
  padding: 0.52rem;
  display: flex;
  flex-direction: column;
  gap: 0.36rem;
  outline: none;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}
.source-upload-card:focus-within,
.source-upload-card:hover {
  border-color: #60a5fa;
  background: #f8fbff;
  box-shadow: 0 0 0 2px rgba(96, 165, 250, 0.12);
}
.source-upload-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.source-upload-title {
  font-size: 0.72rem;
  font-weight: 600;
  color: #1f2937;
}
.source-pick-btn {
  height: 1.85rem;
  border-radius: 999px;
  border: 1px solid #d2dbe9;
  background: #ffffff;
  color: #334155;
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0 0.72rem;
  cursor: pointer;
}
.source-pick-btn:hover:not(:disabled) {
  border-color: #bfcde0;
}
.source-pick-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.source-upload-hint {
  margin: 0;
  font-size: 0.72rem;
  color: #8b98ad;
}
.hidden-input {
  display: none;
}
.source-file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.34rem;
  max-height: 4.9rem;
  overflow: auto;
}
.source-file-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.65rem;
  padding: 0.45rem 0.55rem;
}
.source-file-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.source-file-name {
  max-width: 13.5rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  color: #0f172a;
}
.source-file-meta {
  font-size: 0.66rem;
  color: #94a3b8;
}
.source-file-status {
  font-size: 0.66rem;
  font-weight: 600;
}
.source-file-status.status-pending {
  color: #64748b;
}
.source-file-status.status-uploading {
  color: #2563eb;
}
.source-file-status.status-uploaded {
  color: #15803d;
}
.source-file-status.status-failed {
  color: #b91c1c;
}
.source-remove-btn {
  border: 0;
  background: transparent;
  color: #64748b;
  font-size: 0.7rem;
  cursor: pointer;
}
.source-remove-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.source-empty {
  margin: 0;
  font-size: 0.72rem;
  color: #94a3b8;
}
.refs-wrap {
  margin-top: 0;
}
.field-hint {
  margin: 0;
  font-size: 0.72rem;
  color: #8a97aa;
}
.field-hint-error {
  color: #dc2626;
}
.selected-product-card {
  border: 1px solid #dde4ee;
  border-radius: 0.58rem;
  padding: 0.46rem 0.52rem;
  display: flex;
  flex-direction: column;
  gap: 0.24rem;
}
.selected-product-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.field-value {
  font-size: 0.72rem;
  color: #1f2937;
}
.field-input,
.field-select,
.field-textarea {
  width: 100%;
  border: 1px solid #d7deea;
  border-radius: 0.58rem;
  font-size: 0.78rem;
  color: #334155;
  background: #ffffff;
  transition: border-color 120ms ease;
}
.field-input,
.field-select {
  height: 2.05rem;
  padding: 0 0.62rem;
}
.field-textarea {
  min-height: 4.45rem;
  padding: 0.46rem 0.62rem;
  resize: vertical;
}
.field-input:focus,
.field-select:focus,
.field-textarea:focus {
  outline: none;
  border-color: #b8c6de;
}
.field-input::placeholder,
.field-textarea::placeholder {
  color: #a8b3c5;
}
.field-input:disabled,
.field-select:disabled,
.field-textarea:disabled {
  background: #f8fafc;
  cursor: not-allowed;
}
.field-readonly {
  min-height: 2.05rem;
  display: flex;
  align-items: center;
  border: 1px solid #d7deea;
  border-radius: 0.58rem;
  background: #f8fafc;
  padding: 0.38rem 0.62rem;
  color: #475569;
  font-size: 0.78rem;
  line-height: 1.2rem;
  word-break: break-word;
}
.select-wrap {
  position: relative;
}
.field-select {
  appearance: none;
  padding-right: 1.7rem;
}
.select-wrap::after {
  content: '▾';
  position: absolute;
  right: 0.62rem;
  top: 50%;
  transform: translateY(-50%);
  color: #97a3b7;
  font-size: 0.72rem;
  pointer-events: none;
}
.meta-row-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.62rem;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.46rem;
}
.footer-btn {
  min-width: 3.95rem;
  height: 1.82rem;
  border-radius: 0.52rem;
  border: 1px solid transparent;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
}
.footer-btn-cancel {
  border-color: #d4dce8;
  background: #ffffff;
  color: #475569;
}
.footer-btn-create {
  background: #4f46e5;
  color: #ffffff;
}
.footer-btn-create:disabled {
  background: #d7e2ff;
  color: #f8fbff;
  cursor: not-allowed;
}
.footer-btn-cancel:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
:deep(.upload-panel) {
  border: 1px solid #d7deea;
  border-radius: 0.58rem;
  padding: 0.5rem;
  background: #f9fbff;
}
:deep(.upload-zone) {
  border: 1px dashed #d8dfeb;
  border-radius: 0.52rem;
  padding: 0.8rem;
}
:deep(.upload-zone p) {
  font-size: 0.72rem;
  color: #8b98ad;
}
:deep(.upload-plus) {
  font-size: 1rem;
  color: #7f8da4;
}
:deep(.upload-hint) {
  font-size: 0.68rem;
}
@media (max-width: 1080px) {
  .dense-two-col {
    grid-template-columns: minmax(0, 1fr);
  }
  .meta-row-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
