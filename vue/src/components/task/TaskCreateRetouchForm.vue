<template>
  <section class="type-section retouch-form">
    <div class="form-notice" role="note">
      <p class="form-notice-title">填写说明</p>
      <ul class="form-notice-list">
        <li>请逐条填写 P 图需求，每条需求独立描述修改要求。</li>
        <li>补充说明可写在需求描述中，整单说明可写到底部备注。</li>
        <li>每条需求可单独上传参考图与素材文件，创建后自动绑定到该需求。</li>
      </ul>
    </div>

    <p v-if="pickError" class="pick-error">{{ pickError }}</p>

    <section class="requirements-section">
      <header class="requirements-header">
        <div class="requirements-header-text">
          <h4 class="requirements-title">P 图需求明细</h4>
          <p class="requirements-subtitle">至少 1 条需求描述（必填）；参考图与素材文件可选</p>
        </div>
        <button type="button" class="add-req-btn" @click="addRequirement">添加需求</button>
      </header>

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
            删除本条
          </button>
        </div>

        <div class="requirement-card-body">
          <div class="requirement-col requirement-col--main">
            <div class="requirement-desc-input">
              <BaseTextarea
                v-model="item.description"
                label="需求描述"
                :rows="7"
                placeholder="请直接描述本条 P 图修改要求"
              />
              <p class="requirement-desc-hint">
                补充说明可写在需求描述中，整单说明可写到底部备注。
              </p>
            </div>
          </div>

          <div class="requirement-col requirement-col--files">
            <div
              class="req-upload-panel"
              :class="{ 'req-upload-panel-active': isActiveRetouchFileTarget(index, 'reference') }"
              tabindex="0"
              @focusin="activateRetouchFileReceiver(index, 'reference')"
              @pointerenter="activateRetouchFileReceiver(index, 'reference')"
              @dragover.prevent="onRetouchDragOver(index, 'reference', $event)"
              @drop.prevent="onRetouchDrop(index, 'reference', $event)"
              @paste="onRetouchPaste(index, 'reference', $event)"
            >
              <div class="req-upload-toolbar">
                <div class="req-upload-title-wrap">
                  <span class="req-upload-label">本条参考图</span>
                  <span class="req-upload-optional">可选</span>
                </div>
                <button
                  type="button"
                  class="req-pick-btn"
                  title="选择参考图，也可拖拽到此区域或 Ctrl+V 粘贴"
                  @click="triggerReferencePick(index)"
                >
                  选择参考图
                </button>
              </div>
              <input
                :ref="(el) => setReferenceInputRef(index, el)"
                type="file"
                class="hidden-input"
                multiple
                @change="onReferenceFileChange(index, $event)"
              />
              <p class="req-upload-helper">
                点击选择、拖拽到此处，或 Ctrl+V 粘贴参考图/附件。
              </p>
              <ul v-if="pendingReferenceList(item).length" class="pending-file-list">
                <li
                  v-for="(file, fi) in pendingReferenceList(item)"
                  :key="`${index}-ref-${fi}-${file.name}`"
                  class="pending-file-item"
                >
                  <span class="pending-file-name" :title="file.name">{{ file.name }}</span>
                  <span class="pending-file-meta">{{ prettyFileSize(file.size) }}</span>
                  <button type="button" class="pending-remove-btn" @click="removeReferenceFile(index, fi)">
                    移除
                  </button>
                </li>
              </ul>
              <p v-else class="req-upload-empty">暂未添加参考图，可直接拖入或粘贴。</p>
            </div>

            <div
              class="req-upload-panel"
              :class="{ 'req-upload-panel-active': isActiveRetouchFileTarget(index, 'source') }"
              tabindex="0"
              @focusin="activateRetouchFileReceiver(index, 'source')"
              @pointerenter="activateRetouchFileReceiver(index, 'source')"
              @dragover.prevent="onRetouchDragOver(index, 'source', $event)"
              @drop.prevent="onRetouchDrop(index, 'source', $event)"
              @paste="onRetouchPaste(index, 'source', $event)"
            >
              <div class="req-upload-toolbar">
                <div class="req-upload-title-wrap">
                  <span class="req-upload-label">本条素材文件</span>
                  <span class="req-upload-optional">可选</span>
                </div>
                <button
                  type="button"
                  class="req-pick-btn"
                  title="选择素材文件，也可拖拽到此区域或 Ctrl+V 粘贴"
                  @click="triggerSourcePick(index)"
                >
                  选择素材
                </button>
              </div>
              <input
                :ref="(el) => setSourceInputRef(index, el)"
                type="file"
                class="hidden-input"
                :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                multiple
                @change="onSourceFileChange(index, $event)"
              />
              <p class="req-upload-helper">
                支持 PSD / AI / ZIP / 图片等素材，点击选择、拖拽到此处，或 Ctrl+V 粘贴。
              </p>
              <ul v-if="pendingSourceList(item).length" class="pending-file-list">
                <li
                  v-for="(file, fi) in pendingSourceList(item)"
                  :key="`${index}-src-${fi}-${file.name}`"
                  class="pending-file-item"
                >
                  <span class="pending-file-name" :title="file.name">{{ file.name }}</span>
                  <span class="pending-file-meta">{{ prettyFileSize(file.size) }}</span>
                  <button type="button" class="pending-remove-btn" @click="removeSourceFile(index, fi)">
                    移除
                  </button>
                </li>
              </ul>
              <p v-else class="req-upload-empty">暂未添加素材文件，可直接拖入或粘贴。</p>
            </div>
          </div>
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
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import {
  getFilesFromClipboardEvent,
  getFilesFromDataTransfer,
  hasFileDataTransfer,
  useFileDropPasteReceiver,
} from '@/composables/useFileDropPasteReceiver'

const pickError = ref('')
type RetouchUploadKind = 'reference' | 'source'

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
const activeUploadTarget = ref<{ index: number; kind: RetouchUploadKind } | null>(null)

const { activateFileReceiver } = useFileDropPasteReceiver({
  enabled: () => activeUploadTarget.value != null,
  onFiles: (files) => {
    const target = activeUploadTarget.value
    if (!target) return
    if (target.kind === 'reference') addReferenceFiles(target.index, files)
    else addSourceFiles(target.index, files)
  },
})

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
  activateRetouchFileReceiver(index, 'reference')
  referenceInputRefs.value[index]?.click()
}

function triggerSourcePick(index: number) {
  pickError.value = ''
  activateRetouchFileReceiver(index, 'source')
  sourceInputRefs.value[index]?.click()
}

function activateRetouchFileReceiver(index: number, kind: RetouchUploadKind) {
  activeUploadTarget.value = { index, kind }
  activateFileReceiver()
}

function isActiveRetouchFileTarget(index: number, kind: RetouchUploadKind): boolean {
  return activeUploadTarget.value?.index === index && activeUploadTarget.value?.kind === kind
}

function onRetouchDragOver(index: number, kind: RetouchUploadKind, event: DragEvent) {
  if (!hasFileDataTransfer(event.dataTransfer)) return
  activateRetouchFileReceiver(index, kind)
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onRetouchDrop(index: number, kind: RetouchUploadKind, event: DragEvent) {
  activateRetouchFileReceiver(index, kind)
  const files = getFilesFromDataTransfer(event.dataTransfer)
  if (!files.length) return
  if (kind === 'reference') addReferenceFiles(index, files)
  else addSourceFiles(index, files)
}

function onRetouchPaste(index: number, kind: RetouchUploadKind, event: ClipboardEvent) {
  const files = getFilesFromClipboardEvent(event)
  if (!files.length) return
  event.preventDefault()
  activateRetouchFileReceiver(index, kind)
  if (kind === 'reference') addReferenceFiles(index, files)
  else addSourceFiles(index, files)
}

function onReferenceFileChange(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (files?.length) addReferenceFiles(index, files)
  input.value = ''
}

function addReferenceFiles(index: number, files: FileList | File[]) {
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
}

function onSourceFileChange(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (files?.length) addSourceFiles(index, files)
  input.value = ''
}

function addSourceFiles(index: number, files: FileList | File[]) {
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
  gap: 14px;
  width: 100%;
  min-width: 0;
}

.form-notice {
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid rgb(var(--yb-brand-subtle));
  background: rgb(var(--yb-surface-blue-tint));
}

.form-notice-title {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
  color: rgb(var(--yb-brand-deep));
}

.form-notice-list {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 12px;
  line-height: 1.55;
  color: rgb(var(--yb-text-soft));
}

.form-notice-list li + li {
  margin-top: 2px;
}

.requirements-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  min-width: 0;
}

.requirements-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 4px;
  border-bottom: 1px solid rgb(var(--yb-border-slate));
}

.requirements-header-text {
  min-width: 0;
}

.requirements-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}

.requirements-subtitle {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.4;
  color: rgb(var(--yb-text-muted-strong));
}

.add-req-btn,
.remove-req-btn,
.req-pick-btn,
.pending-remove-btn {
  flex-shrink: 0;
  border: 1px solid rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-navy));
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease;
}

.add-req-btn {
  padding: 6px 14px;
  font-weight: 500;
}

.add-req-btn:hover,
.req-pick-btn:hover,
.remove-req-btn:hover {
  border-color: rgb(var(--yb-text-placeholder));
  background: rgb(var(--yb-surface-subtle));
}

.requirement-card {
  width: 100%;
  min-width: 0;
  border: 1px solid rgb(var(--yb-text-disabled));
  border-radius: 12px;
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
  overflow: hidden;
}

.requirement-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  background: rgb(var(--yb-surface-subtle));
  border-bottom: 1px solid rgb(var(--yb-border-slate));
}

.requirement-index {
  font-size: 13px;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}

.remove-req-btn {
  padding: 4px 10px;
  color: rgb(var(--yb-danger-text));
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-surface));
}

.remove-req-btn:hover {
  background: rgb(var(--yb-danger-soft));
  border-color: rgb(var(--yb-danger-border-hover));
}

.requirement-card-body {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr);
  gap: 0;
  min-width: 0;
}

.requirement-col {
  min-width: 0;
  padding: 14px;
}

.requirement-col--main {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-right: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-near-white));
}

.requirement-col--files {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: rgb(var(--yb-surface-canvas));
}

.requirement-desc-input :deep(textarea) {
  min-height: 10rem;
}

.requirement-desc-hint {
  margin: 0;
  font-size: 11px;
  line-height: 1.45;
  color: rgb(var(--yb-text-muted-strong));
}

.req-upload-panel {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 8px;
  background: rgb(var(--yb-surface));
  outline: none;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.req-upload-panel-active,
.req-upload-panel:focus-within {
  border-color: rgb(var(--yb-brand-accent));
  background: rgb(var(--yb-surface-brand-panel));
  box-shadow: 0 0 0 2px rgb(var(--yb-brand-accent) / 0.14);
}

.req-upload-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.req-upload-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.req-upload-label {
  font-size: 12px;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}

.req-upload-optional {
  font-size: 11px;
  color: rgb(var(--yb-text-muted-strong));
  padding: 1px 6px;
  border-radius: 999px;
  background: rgb(var(--yb-surface-slate));
}

.req-pick-btn {
  padding: 4px 10px;
  white-space: nowrap;
}

.req-upload-helper {
  margin: 0;
  font-size: 11px;
  line-height: 1.45;
  color: rgb(var(--yb-text-soft));
}

.req-upload-empty {
  margin: 0;
  font-size: 11px;
  line-height: 1.4;
  color: rgb(var(--yb-text-placeholder));
}

.pick-error {
  margin: 0;
  font-size: 12px;
  color: rgb(var(--yb-danger-text));
}

.hidden-input {
  display: none;
}

.pending-file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 120px;
  overflow-y: auto;
}

.pending-file-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  padding: 4px 6px;
  border-radius: 4px;
  background: rgb(var(--yb-surface-subtle));
}

.pending-file-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pending-file-meta {
  color: rgb(var(--yb-text-muted-strong));
  flex-shrink: 0;
}

.pending-remove-btn {
  padding: 2px 6px;
  font-size: 11px;
}

@media (max-width: 900px) {
  .requirement-card-body {
    grid-template-columns: 1fr;
  }

  .requirement-col--main {
    border-right: none;
    border-bottom: 1px solid rgb(var(--yb-border-slate));
  }
}
</style>
