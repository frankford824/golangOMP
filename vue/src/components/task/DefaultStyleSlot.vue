<template>
  <section class="default-style-slot" aria-label="默认款" :aria-busy="busy">
    <header class="slot-header">
      <span class="slot-icon" aria-hidden="true"><Image :size="19" /></span>
      <div class="slot-heading">
        <strong>默认款</strong>
        <small>{{ editable ? '每个资源组可保留一份默认款。' : '有审核权限的用户可在符合条件的任务中上传或替换。' }}</small>
      </div>
      <div v-if="editable" class="slot-tools">
        <button v-if="historyEnabled" type="button" class="slot-button" :disabled="busy" :aria-expanded="showHistory" @click="toggleHistory">
          <History :size="15" aria-hidden="true" />{{ showHistory ? '收起历史' : '历史' }}
        </button>
        <button v-if="!draft && !retryChange" ref="uploadButton" type="button" class="slot-button primary" :disabled="busy" @click="fileInput?.click()">
          <Upload :size="15" aria-hidden="true" />{{ hasFile ? '替换默认款' : '上传默认款' }}
        </button>
      </div>
    </header>
    <input v-if="editable" ref="fileInput" class="slot-file-input" type="file" aria-label="选择默认款文件"
      :accept="FINAL_UPLOAD_ACCEPT_ATTRIBUTE" :disabled="busy || Boolean(retryChange)" @change="selectFile">

    <div v-if="displayed.length" class="slot-files">
      <div v-for="entry in displayed" :key="entry.revision.id" class="slot-line">
        <span class="file-icon" aria-hidden="true"><FileText :size="18" /></span>
        <div class="file-info">
          <strong>{{ entry.revision.removed ? '移除默认款' : entry.revision.file?.file_name || '文件不可用' }}</strong>
          <span class="slot-status" :class="{ pending: entry.pending }">{{ entry.pending ? '结单后生效' : '当前生效' }}</span>
        </div>
        <div class="slot-tools">
          <button v-if="entry.revision.file?.preview_url" type="button" class="slot-button" :disabled="busy" @click="previewFile(entry.revision.file)">预览</button>
          <button v-if="entry.revision.file?.download_url" type="button" class="slot-button" :disabled="busy" @click="downloadFile(entry.revision.file)">下载</button>
        </div>
      </div>
    </div>
    <div v-else-if="!draft" class="slot-empty">
      <strong>还没有默认款</strong>
      <span v-if="editable">可上传图片、设计文件、PDF 或 ZIP。</span>
    </div>

    <form v-if="draft || retryChange" class="slot-form" @submit.prevent="submitDraft">
      <div class="draft-file">
        <span>{{ draft?.kind === 'upload' ? draft.file.name : '移除默认款' }}</span>
        <button v-if="draft?.kind === 'upload' && !retryChange" type="button" class="slot-button" :disabled="busy" @click="fileInput?.click()">更换文件</button>
      </div>
      <label class="reason-label">
        <span>变更原因 <small>必填</small></span>
        <input ref="reasonInput" v-model.trim="reason" aria-label="默认款变更原因" placeholder="例如：更新标准样图"
          maxlength="512" :disabled="busy || Boolean(retryChange)" required>
      </label>
      <div class="slot-tools form-actions">
        <button type="button" class="slot-button" :disabled="busy" @click="cancelDraft">取消</button>
        <button type="submit" class="slot-button primary" :disabled="busy || !reason">
          {{ busy ? operationLabel : retryChange ? '重试保存' : draft?.kind === 'remove' ? '确认移除' : '保存默认款' }}
        </button>
      </div>
    </form>
    <button v-else-if="editable && hasFile" class="slot-button remove-action" type="button" :disabled="busy" @click="startRemove">移除默认款</button>

    <p v-if="notice" class="slot-notice" role="status">{{ notice }}</p>
    <p v-if="error" class="slot-error" role="alert">{{ error }}</p>

    <ol v-if="showHistory" class="slot-history">
      <li v-for="item in history" :key="item.id">
        <span>{{ item.removed ? '移除' : item.file?.file_name || '文件不可用' }} · {{ item.reason }} · {{ new Date(item.created_at).toLocaleString() }}</span>
        <button v-if="item.file?.preview_url" type="button" class="slot-button" :disabled="busy" @click="previewFile(item.file)">预览</button>
        <button v-if="item.file?.download_url" type="button" class="slot-button" :disabled="busy" @click="downloadFile(item.file)">下载</button>
      </li>
      <li v-if="!history.length">暂无记录</li>
      <li v-if="historyHasMore"><button type="button" class="slot-button" :disabled="busy" @click="loadHistory(history.at(-1)?.id)">查看更多</button></li>
    </ol>
    <ImagePreviewLightbox v-model="previewOpen" :items="previewItems" aria-label="默认款预览" />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { FileText, History, Image, Upload } from 'lucide-vue-next'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import { FINAL_UPLOAD_ACCEPT_ATTRIBUTE, isSupportedFinalUploadFilename } from '@/domain/resource-workflow-files'
import { resourceGroupsApi, type DefaultStyleRevision, type ResourceFile, type ResourceGroup } from '@/services/api/resourceGroupsApi'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'
import { generateActionId } from '@/utils/uuid'

const props = defineProps<{ group: ResourceGroup; editable?: boolean; historyEnabled?: boolean }>()
const emit = defineEmits<{ updated: [group: ResourceGroup]; busy: [active: boolean]; dirtyChange: [dirty: boolean] }>()
const fileInput = ref<HTMLInputElement | null>(null)
const reasonInput = ref<HTMLInputElement | null>(null)
const uploadButton = ref<HTMLButtonElement | null>(null)
let focusUploadAfterSave = false
const draft = ref<{ kind: 'upload'; file: File } | { kind: 'remove' } | null>(null)
const operationLabel = ref('保存中…')
const notice = ref('')
const reason = ref('')
const busy = ref(false)
const error = ref('')
const showHistory = ref(false)
const history = ref<DefaultStyleRevision[]>([])
const historyHasMore = ref(false)
const previewOpen = ref(false)
const previewItems = ref<{ src: string; title: string; alt: string }[]>([])
type Change = { change: { task_asset_id: number } | { remove: true }; reason: string; key: string; group: ResourceGroup }
const retryChange = ref<Change | null>(null)

const displayed = computed(() => [
  ...(props.group.default_style ? [{ pending: false, revision: props.group.default_style }] : []),
  ...(props.group.pending_default_style ? [{ pending: true, revision: props.group.pending_default_style }] : []),
])
const hasFile = computed(() => Boolean((props.group.pending_default_style ?? props.group.default_style)?.file))
watch(() => Boolean(draft.value || retryChange.value), (dirty) => emit('dirtyChange', dirty))
onBeforeUnmount(() => emit('dirtyChange', false))

async function withBusy(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  emit('busy', true)
  error.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '操作失败，请稍后重试。'
  } finally {
    busy.value = false
    emit('busy', false)
    if (focusUploadAfterSave) {
      focusUploadAfterSave = false
      void nextTick(() => uploadButton.value?.focus())
    }
  }
}

function command(change: Change['change']): Change {
  return { change, reason: reason.value, key: generateActionId(), group: { ...props.group } }
}

async function save(change: Change) {
  operationLabel.value = '保存中…'
  retryChange.value = change
  const result = await resourceGroupsApi.updateDefaultStyle(change.group, change.change, change.reason, change.key)
  retryChange.value = null
  draft.value = null
  reason.value = ''
  focusUploadAfterSave = true
  emit('updated', { ...props.group, lock_version: result.group_lock_version })
  notice.value = '默认款变更已保存。'
  try {
    const bundle = await resourceGroupsApi.taskBundle(change.group.task_id)
    const group = bundle.groups.find((item) => item.id === change.group.id)
    if (!group || group.lock_version !== result.group_lock_version) {
      notice.value = '变更已保存。请刷新任务查看最新文件。'
    } else {
      emit('updated', group)
      if (group.pending_default_style?.id === result.revision_id) notice.value = '已保存，结单后生效。'
    }
  } catch {
    notice.value = '变更已保存。请刷新任务查看最新文件。'
  }
  if (showHistory.value) await loadHistoryPage()
}

function selectFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !props.editable || busy.value || retryChange.value) return
  error.value = ''
  notice.value = ''
  if (!isSupportedFinalUploadFilename(file.name) || !file.size) {
    error.value = '请选择非空的图片、设计文件、PDF 或 ZIP。'
    return
  }
  draft.value = { kind: 'upload', file }
  void nextTick(() => reasonInput.value?.focus())
}

function startRemove() {
  if (!props.editable || busy.value) return
  notice.value = ''
  error.value = ''
  draft.value = { kind: 'remove' }
  void nextTick(() => reasonInput.value?.focus())
}

function cancelDraft() {
  draft.value = null
  retryChange.value = null
  reason.value = ''
  error.value = ''
  void nextTick(() => uploadButton.value?.focus())
}

async function submitDraft() {
  if (!props.editable || !reason.value || busy.value) return
  if (retryChange.value) {
    const change = retryChange.value
    await withBusy(() => save(change))
    return
  }
  const action = draft.value
  if (!action) return
  notice.value = ''
  if (action.kind === 'remove') {
    await withBusy(() => save(command({ remove: true })))
    return
  }
  await withBusy(async () => {
    operationLabel.value = '正在上传…'
    const group = props.group
    const file = action.file
    const result = await uploadTaskFileViaAssetSession(String(group.task_id), file, {
      asset_kind: 'default_style',
      target_sku_code: group.scope_kind === 'sku' ? group.sku_code : undefined,
      remark: file.name,
    }, { retouchRequirementId: group.retouch_requirement_id ?? undefined })
    const assetId = Number(result.version?.id || result.version?.version_id)
    if (!Number.isSafeInteger(assetId) || assetId <= 0) throw new Error('上传结果缺少文件编号，请刷新后重试。')
    await save(command({ task_asset_id: assetId }))
  })
}

async function loadHistoryPage(beforeId?: number) {
  const result = await resourceGroupsApi.defaultStyleHistory(props.group.id, beforeId)
  history.value = beforeId ? [...history.value, ...result.items] : result.items
  historyHasMore.value = result.items.length === 50
}

function loadHistory(beforeId?: number) {
  return withBusy(() => loadHistoryPage(beforeId))
}

async function toggleHistory() {
  showHistory.value = !showHistory.value
  if (showHistory.value) await loadHistory()
}

function previewFile(file: ResourceFile) {
  return withBusy(async () => {
    const meta = await resourceGroupsApi.previewTaskAsset(file.task_asset_id)
    const url = meta.download_url
    if (!url) throw new Error('文件暂时无法预览。')
    previewItems.value = [{ src: url, title: file.file_name, alt: file.file_name }]
    previewOpen.value = true
  })
}

function downloadFile(file: ResourceFile) {
  return withBusy(async () => {
    const result = await downloadAssetFileWithOriginalFilename({ taskAssetId: String(file.task_asset_id), preferredFilename: file.file_name })
    if (!result.ok) throw new Error(result.message || '下载失败。')
  })
}
</script>

<style scoped>
.default-style-slot {
  display: grid;
  gap: 12px;
  min-width: 0;
  padding: 14px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 12px;
  background: rgb(var(--yb-surface));
}
.slot-header, .slot-line, .draft-file, .slot-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.slot-header, .slot-line { flex-wrap: wrap; }
.slot-icon, .file-icon {
  display: grid;
  flex: 0 0 auto;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 8px;
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}
.slot-heading, .file-info { display: grid; flex: 1 1 130px; min-width: 0; gap: 3px; }
.slot-heading strong { font-size: 15px; }
.slot-heading small, .slot-empty span { color: rgb(var(--yb-text-muted)); font-size: 12px; }
.slot-file-input { display: none; }
.slot-files, .slot-form, .slot-empty { display: grid; gap: 8px; }
.slot-line, .slot-empty, .slot-form {
  min-width: 0;
  padding: 10px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 8px;
  background: rgb(var(--yb-surface-soft));
}
.slot-empty { border-style: dashed; }
.slot-empty strong, .file-info strong, .draft-file span { overflow-wrap: anywhere; font-size: 13px; }
.slot-status {
  width: fit-content;
  padding: 2px 6px;
  border-radius: 5px;
  color: rgb(var(--yb-success-strong));
  background: rgb(var(--yb-success-soft));
  font-size: 11px;
}
.slot-status.pending {
  color: rgb(var(--yb-warning-text));
  background: rgb(var(--yb-warning-soft));
}
.slot-tools { flex-wrap: wrap; }
.slot-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 34px;
  padding: 5px 9px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 7px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
}
.slot-button.primary {
  border-color: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  background: rgb(var(--yb-brand));
}
.slot-button:disabled { opacity: .58; cursor: default; }
.remove-action { justify-self: start; color: rgb(var(--yb-danger-text)); }
.draft-file { justify-content: space-between; flex-wrap: wrap; }
.reason-label { display: grid; gap: 4px; color: rgb(var(--yb-text)); font-size: 12px; }
.reason-label small { color: rgb(var(--yb-text-muted)); }
.reason-label input {
  width: 100%;
  min-height: 36px;
  padding: 7px 9px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 7px;
  color: inherit;
  background: rgb(var(--yb-surface));
  box-sizing: border-box;
}
.form-actions { justify-content: flex-end; }
.slot-notice, .slot-error { margin: 0; font-size: 12px; }
.slot-notice { color: rgb(var(--yb-success-strong)); }
.slot-error { color: rgb(var(--yb-danger-text)); }
.slot-history { display: grid; gap: 6px; padding-inline-start: 18px; margin: 0; font-size: 12px; }
.slot-history li { overflow-wrap: anywhere; }
@media (max-width: 540px) {
  .slot-header > .slot-tools { flex-basis: 100%; }
}
</style>
