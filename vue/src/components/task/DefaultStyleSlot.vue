<template>
  <section class="default-style-slot" aria-label="默认款" :aria-busy="busy">
    <strong>默认款</strong>
    <div class="slot-files">
      <span v-if="!group.default_style && !group.pending_default_style">未上传</span>
      <div v-for="entry in displayed" :key="entry.revision.id" class="slot-line">
        <span>{{ entry.label }} · {{ entry.revision.removed ? '移除默认款' : entry.revision.file?.file_name || '文件不可用' }}</span>
        <button v-if="entry.revision.file?.preview_url" type="button" @click="previewFile(entry.revision.file)">预览</button>
        <button v-if="entry.revision.file?.download_url" type="button" @click="downloadFile(entry.revision.file)">下载</button>
      </div>
    </div>
    <div v-if="editable" class="slot-actions">
      <input v-model.trim="reason" aria-label="默认款变更原因" placeholder="填写变更原因" maxlength="512" :disabled="busy">
      <label class="upload-label">
        {{ busy ? '处理中…' : hasFile ? '替换' : '上传' }}
        <input type="file" aria-label="上传默认款" :accept="FINAL_UPLOAD_ACCEPT_ATTRIBUTE" :disabled="busy || !reason" @change="upload">
      </label>
      <button v-if="hasFile" type="button" :disabled="busy || !reason" @click="remove">移除</button>
      <button v-if="historyEnabled" type="button" :disabled="busy" @click="toggleHistory">{{ showHistory ? '收起历史' : '历史' }}</button>
      <button v-if="retryChange" type="button" :disabled="busy" @click="runChange(retryChange)">重试保存</button>
    </div>
    <p v-if="error" role="alert">{{ error }}</p>
    <ol v-if="showHistory" class="slot-history">
      <li v-for="item in history" :key="item.id">
        <span>{{ item.removed ? '移除' : item.file?.file_name || '文件不可用' }} · {{ item.reason }} · {{ new Date(item.created_at).toLocaleString() }}</span>
        <button v-if="item.file?.preview_url" type="button" @click="previewFile(item.file)">预览</button>
        <button v-if="item.file?.download_url" type="button" @click="downloadFile(item.file)">下载</button>
      </li>
      <li v-if="!history.length">暂无记录</li>
      <li v-if="historyHasMore"><button type="button" :disabled="busy" @click="loadHistory(history.at(-1)?.id)">查看更多</button></li>
    </ol>
    <ImagePreviewLightbox v-model="previewOpen" :items="previewItems" aria-label="默认款预览" />
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import { FINAL_UPLOAD_ACCEPT_ATTRIBUTE } from '@/domain/resource-workflow-files'
import { resourceGroupsApi, type DefaultStyleRevision, type ResourceFile, type ResourceGroup } from '@/services/api/resourceGroupsApi'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'
import { generateActionId } from '@/utils/uuid'

const props = defineProps<{ group: ResourceGroup; editable?: boolean; historyEnabled?: boolean }>()
const emit = defineEmits<{ updated: [group: ResourceGroup]; busy: [active: boolean] }>()
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
  ...(props.group.default_style ? [{ label: '当前生效', revision: props.group.default_style }] : []),
  ...(props.group.pending_default_style ? [{ label: '结单后生效', revision: props.group.pending_default_style }] : []),
])
const hasFile = computed(() => Boolean((props.group.pending_default_style ?? props.group.default_style)?.file))

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
  }
}

function command(change: Change['change']): Change {
  return { change, reason: reason.value, key: generateActionId(), group: { ...props.group } }
}

async function save(change: Change) {
  retryChange.value = change
  const result = await resourceGroupsApi.updateDefaultStyle(change.group, change.change, change.reason, change.key)
  retryChange.value = null
  reason.value = ''
  emit('updated', { ...props.group, lock_version: result.group_lock_version })
  const bundle = await resourceGroupsApi.taskBundle(change.group.task_id)
  const group = bundle.groups.find((item) => item.id === change.group.id)
  if (group?.lock_version === result.group_lock_version) emit('updated', group)
  if (showHistory.value) await loadHistoryPage()
}

function runChange(change: Change) {
  return withBusy(() => save(change))
}

async function upload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !reason.value || !props.editable) return
  await withBusy(async () => {
    const group = props.group
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

function remove() {
  if (reason.value && props.editable) void runChange(command({ remove: true }))
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
.default-style-slot{display:flex;flex-wrap:wrap;align-items:center;gap:6px 12px;padding:8px 10px;border:1px solid rgb(var(--yb-border));border-radius:8px}
.slot-files,.slot-history{min-width:0;flex:1 1 180px}
.slot-line,.slot-actions{display:flex;flex-wrap:wrap;align-items:center;gap:6px}
.slot-line span{overflow-wrap:anywhere}
.slot-actions input:not([type=file]){width:10em;padding:4px;border:1px solid rgb(var(--yb-border));border-radius:5px;background:rgb(var(--yb-surface));color:inherit}
.upload-label,.slot-line button,.slot-actions button,.slot-history button{position:relative;padding:3px 6px;color:rgb(var(--yb-brand-strong));cursor:pointer}
.upload-label:focus-within{outline:2px solid currentColor}
.upload-label:has(:disabled),button:disabled{opacity:.55;cursor:default}
.upload-label input{position:absolute;width:1px;height:1px;opacity:0}
.slot-history,.default-style-slot p{flex-basis:100%;margin:0}
.default-style-slot p{color:rgb(var(--yb-danger-text))}
</style>
