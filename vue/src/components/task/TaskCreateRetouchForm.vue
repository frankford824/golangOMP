<template>
  <section class="type-section retouch-form">
    <p class="form-intro">
      请按需求逐条填写 P 图说明：直接描述本条修改要求，额外说明可写在备注；每条需求可单独上传参考图与素材文件，创建任务后自动绑定到该需求。
    </p>

    <p v-if="pickError" class="pick-error">{{ pickError }}</p>

    <section class="requirements-section">
      <div class="requirements-header">
        <div>
          <h4 class="requirements-title">P 图需求明细</h4>
          <p class="field-hint">至少填写 1 条需求描述（必填）；备注与本条附件为可选项。</p>
        </div>
        <button type="button" class="add-req-btn" @click="addRequirement">添加需求</button>
      </div>

      <article
        v-for="(item, index) in retouchRequirements"
        :key="index"
        class="requirement-card"
      >
        <div class="requirement-card-head">
          <span class="requirement-index">需求 {{ index + 1 }}</span>
          <button
            v-if="retouchRequirements.length > 1"
            type="button"
            class="remove-req-btn"
            @click="removeRequirement(index)"
          >
            删除
          </button>
        </div>
        <BaseTextarea
          v-model="item.description"
          label="需求描述"
          :rows="3"
          placeholder="直接描述本条 P 图修改要求"
        />
        <BaseInput
          v-model="item.remark"
          label="备注（可选）"
          placeholder="如有款号、尺寸等额外说明可写在这里"
        />

        <div class="req-upload-block">
          <div class="req-upload-head">
            <span class="req-upload-label">本条参考图（可选）</span>
            <button type="button" class="req-pick-btn" @click="triggerReferencePick(index)">选择文件</button>
          </div>
          <p class="field-hint">支持多文件；创建任务后按本条需求绑定。</p>
          <input
            :ref="(el) => setReferenceInputRef(index, el)"
            type="file"
            class="hidden-input"
            multiple
            @change="onReferenceFileChange(index, $event)"
          />
          <ul v-if="pendingReferenceList(item).length" class="pending-file-list">
            <li v-for="(file, fi) in pendingReferenceList(item)" :key="`${index}-ref-${fi}-${file.name}`" class="pending-file-item">
              <span class="pending-file-name" :title="file.name">{{ file.name }}</span>
              <span class="pending-file-meta">{{ prettyFileSize(file.size) }}</span>
              <button type="button" class="pending-remove-btn" @click="removeReferenceFile(index, fi)">删除</button>
            </li>
          </ul>
        </div>

        <div class="req-upload-block">
          <div class="req-upload-head">
            <span class="req-upload-label">本条素材文件（可选）</span>
            <button type="button" class="req-pick-btn" @click="triggerSourcePick(index)">选择文件</button>
          </div>
          <p class="field-hint">支持 PSD / AI / EPS / ZIP / 图片等；创建任务后按 source 资产绑定到本条需求。</p>
          <input
            :ref="(el) => setSourceInputRef(index, el)"
            type="file"
            class="hidden-input"
            :accept="UPLOAD_ACCEPT_ATTRIBUTE"
            multiple
            @change="onSourceFileChange(index, $event)"
          />
          <ul v-if="pendingSourceList(item).length" class="pending-file-list">
            <li v-for="(file, fi) in pendingSourceList(item)" :key="`${index}-src-${fi}-${file.name}`" class="pending-file-item">
              <span class="pending-file-name" :title="file.name">{{ file.name }}</span>
              <span class="pending-file-meta">{{ prettyFileSize(file.size) }}</span>
              <button type="button" class="pending-remove-btn" @click="removeSourceFile(index, fi)">删除</button>
            </li>
          </ul>
        </div>
      </article>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import type { TaskCreateFormModel } from '@/domain/types'
import type { RetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import { createEmptyRetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import {
  REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  isAcceptableReferenceFile,
  referenceFileTooLargeMessage,
} from '@/domain/constants/reference-upload'
import { UPLOAD_ACCEPT_ATTRIBUTE, isAllowedUploadFile } from '@/domain/constants/upload-types'
import { DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES } from '@/domain/copy/design-upload'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'

const pickError = ref('')

const props = defineProps<{
  form: TaskCreateFormModel
}>()

const emit = defineEmits<{
  'update:form': [value: TaskCreateFormModel]
}>()

const localForm = computed({
  get: () => props.form,
  set: (value: TaskCreateFormModel) => emit('update:form', value),
})

const retouchRequirements = computed(() => localForm.value.retouchRequirements ?? [])

watch(
  () => localForm.value.retouchRequirements,
  (items) => {
    if (!Array.isArray(items) || items.length === 0) {
      localForm.value = {
        ...localForm.value,
        retouchRequirements: [createEmptyRetouchRequirementDraft(1)],
      }
    }
  },
  { immediate: true },
)

function addRequirement() {
  const next = [...(localForm.value.retouchRequirements ?? [])]
  next.push(createEmptyRetouchRequirementDraft(next.length + 1))
  localForm.value = { ...localForm.value, retouchRequirements: next }
}

function removeRequirement(index: number) {
  const next = [...(localForm.value.retouchRequirements ?? [])]
  if (next.length <= 1) return
  next.splice(index, 1)
  localForm.value = {
    ...localForm.value,
    retouchRequirements: next.map((item, i) => ({ ...item, sortOrder: i + 1 })),
  }
}

const referenceInputRefs = ref<Record<number, HTMLInputElement | null>>({})
const sourceInputRefs = ref<Record<number, HTMLInputElement | null>>({})

function setReferenceInputRef(index: number, el: Element | ComponentPublicInstance | null) {
  referenceInputRefs.value[index] = el instanceof HTMLInputElement ? el : null
}

function setSourceInputRef(index: number, el: Element | ComponentPublicInstance | null) {
  sourceInputRefs.value[index] = el instanceof HTMLInputElement ? el : null
}

function pendingReferenceList(item: RetouchRequirementDraft): File[] {
  return item.pendingReferenceFiles ?? []
}

function pendingSourceList(item: RetouchRequirementDraft): File[] {
  return item.pendingSourceFiles ?? []
}

function updateRequirementAt(index: number, patch: Partial<RetouchRequirementDraft>) {
  const next = [...(localForm.value.retouchRequirements ?? [])]
  const current = next[index]
  if (!current) return
  next[index] = { ...current, ...patch }
  localForm.value = { ...localForm.value, retouchRequirements: next }
}

function triggerReferencePick(index: number) {
  pickError.value = ''
  referenceInputRefs.value[index]?.click()
}

function triggerSourcePick(index: number) {
  pickError.value = ''
  sourceInputRefs.value[index]?.click()
}

function onReferenceFileChange(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files?.length) return
  const current = localForm.value.retouchRequirements?.[index]
  if (!current) return
  const existing = [...(current.pendingReferenceFiles ?? [])]
  for (const file of Array.from(files)) {
    if (!isAcceptableReferenceFile(file)) {
      pickError.value = `参考图无效：${file.name}`
      continue
    }
    if (file.size > REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES) {
      pickError.value = referenceFileTooLargeMessage(file.name)
      continue
    }
    if (existing.some((f) => f.name === file.name && f.size === file.size && f.lastModified === file.lastModified)) {
      continue
    }
    existing.push(file)
  }
  updateRequirementAt(index, { pendingReferenceFiles: existing })
  input.value = ''
}

function onSourceFileChange(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files?.length) return
  const current = localForm.value.retouchRequirements?.[index]
  if (!current) return
  const existing = [...(current.pendingSourceFiles ?? [])]
  for (const file of Array.from(files)) {
    if (!isAllowedUploadFile(file.name)) {
      pickError.value = `不支持的素材类型：${file.name}`
      continue
    }
    if (file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES) {
      pickError.value = `${file.name} 超过单文件大小上限，已跳过`
      continue
    }
    if (existing.some((f) => f.name === file.name && f.size === file.size && f.lastModified === file.lastModified)) {
      continue
    }
    existing.push(file)
  }
  updateRequirementAt(index, { pendingSourceFiles: existing })
  input.value = ''
}

function removeReferenceFile(index: number, fileIndex: number) {
  const current = localForm.value.retouchRequirements?.[index]
  if (!current) return
  const next = [...(current.pendingReferenceFiles ?? [])]
  next.splice(fileIndex, 1)
  updateRequirementAt(index, { pendingReferenceFiles: next })
}

function removeSourceFile(index: number, fileIndex: number) {
  const current = localForm.value.retouchRequirements?.[index]
  if (!current) return
  const next = [...(current.pendingSourceFiles ?? [])]
  next.splice(fileIndex, 1)
  updateRequirementAt(index, { pendingSourceFiles: next })
}

function prettyFileSize(size: number): string {
  if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${size} B`
}
</script>

<style scoped>
.retouch-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-intro {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-secondary, #64748b);
}

.field-hint {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary, #64748b);
}

.requirements-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.requirements-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.requirements-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.add-req-btn,
.remove-req-btn {
  border: 1px solid var(--border-color, #dbe3ef);
  background: #fff;
  color: var(--text-primary, #0f172a);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.requirement-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  background: #f8fafc;
}

.requirement-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.requirement-index {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.pick-error {
  margin: 0;
  font-size: 12px;
  color: #b91c1c;
}

.hidden-input {
  display: none;
}

.req-upload-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 4px;
  border-top: 1px dashed var(--border-color, #e2e8f0);
}

.req-upload-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.req-upload-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.req-pick-btn,
.pending-remove-btn {
  border: 1px solid var(--border-color, #dbe3ef);
  background: #fff;
  color: var(--text-primary, #0f172a);
  border-radius: 6px;
  padding: 3px 8px;
  font-size: 12px;
  cursor: pointer;
}

.pending-file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.pending-file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.pending-file-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pending-file-meta {
  color: var(--text-secondary, #64748b);
  flex-shrink: 0;
}
</style>
