<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Download, Eye, RefreshCw, Search } from 'lucide-vue-next'

import {
  resourceGroupsApi,
  type FlatResourceItem,
  type ResourceDownloadInfo,
  type ResourceGroup,
} from '@/services/api/resourceGroupsApi'
import { useGlobalDownload } from '@aw/shared/download/useGlobalDownload'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import CanonicalResourceThumb from './CanonicalResourceThumb.vue'
import {
  canonicalGroupCover,
  canonicalGroupFinals,
  canonicalPreviewErrorMessage,
  canonicalPreviewUnavailableMessage,
  canonicalResourceRoleLabel,
  canonicalResourceRoleOptions,
  currentCanonicalRevision,
  resolveCanonicalPreview,
  type CanonicalResourceRole,
} from './canonicalResource'

const props = defineProps<{ initialQuery?: string }>()
const { queueCanonicalGroup, queueCanonicalResource } = useGlobalDownload()
const query = ref(props.initialQuery || '')
const role = ref<CanonicalResourceRole>('')
const loading = ref(false)
const error = ref('')
const groups = ref<ResourceGroup[]>([])
const flatItems = ref<FlatResourceItem[]>([])
const viewMode = ref<'group' | 'flat'>('group')
const page = ref(1)
const pageSize = 36
const total = ref(0)
const groupCache = new Map<number, ResourceGroup>()
let requestID = 0

const previewOpen = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const previewUrl = ref('')
const previewTitle = ref('')
const previewMimeType = ref('')
const previewRows = ref<Array<[string, string]>>([])
const previewTarget = ref<{ kind: 'item'; value: FlatResourceItem } | { kind: 'group'; value: ResourceGroup } | null>(null)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

watch(() => props.initialQuery, (value) => {
  if (value === undefined || value === query.value) return
  query.value = value
  void search()
})

async function load() {
  const currentRequest = ++requestID
  loading.value = true
  error.value = ''
  try {
    const result = await resourceGroupsApi.list({
      q: query.value.trim() || undefined,
      resource_role: role.value || undefined,
      page: page.value,
      page_size: pageSize,
    })
    if (currentRequest !== requestID) return
    groups.value = result.items || []
    flatItems.value = result.flat_items || []
    viewMode.value = result.view_mode || (role.value ? 'flat' : 'group')
    total.value = Number(result.total || 0)
    for (const group of groups.value) groupCache.set(group.id, group)
  } catch (cause) {
    if (currentRequest !== requestID) return
    groups.value = []
    flatItems.value = []
    total.value = 0
    error.value = cause instanceof Error ? cause.message : '资源加载失败'
  } finally {
    if (currentRequest === requestID) loading.value = false
  }
}

function search() {
  page.value = 1
  return load()
}

function changeRole() {
  page.value = 1
  void load()
}

function changePage(next: number) {
  if (next < 1 || next > totalPages.value || next === page.value) return
  page.value = next
  void load()
}

async function getGroup(groupID: number) {
  const cached = groupCache.get(groupID)
  if (cached) return cached
  const group = await resourceGroupsApi.get(groupID)
  groupCache.set(groupID, group)
  return group
}

async function openItemPreview(item: FlatResourceItem) {
  previewTarget.value = { kind: 'item', value: item }
  previewOpen.value = true
  previewLoading.value = true
  previewError.value = ''
  previewUrl.value = ''
  previewTitle.value = item.file_name
  previewMimeType.value = item.mime_type || ''
  previewRows.value = itemRows(item)
  try {
    const meta = await resolveCanonicalPreview(item, getGroup)
    previewUrl.value = String(meta.download_url || '')
    previewMimeType.value = meta.mime_type || item.mime_type || ''
    if (!previewUrl.value) previewError.value = canonicalPreviewUnavailableMessage(item.resource_role)
  } catch (cause) {
    previewError.value = canonicalPreviewErrorMessage(cause, item.resource_role)
  } finally {
    previewLoading.value = false
  }
}

async function openGroupPreview(group: ResourceGroup) {
  const cover = canonicalGroupCover(group)
  previewTarget.value = { kind: 'group', value: group }
  previewOpen.value = true
  previewLoading.value = true
  previewError.value = ''
  previewUrl.value = ''
  previewTitle.value = groupTitle(group)
  previewMimeType.value = cover?.mimeType || ''
  previewRows.value = groupRows(group)
  try {
    if (!cover) throw new Error('当前资源组没有可预览的最终成品')
    let meta: ResourceDownloadInfo | null = null
    if (cover.taskAssetId) meta = await resourceGroupsApi.previewTaskAsset(cover.taskAssetId)
    previewUrl.value = String(meta?.download_url || cover.previewUrl || cover.downloadUrl || '')
    previewMimeType.value = meta?.mime_type || cover.mimeType || ''
    if (!previewUrl.value) previewError.value = canonicalPreviewUnavailableMessage('final')
  } catch (cause) {
    previewError.value = canonicalPreviewErrorMessage(cause, 'final')
  } finally {
    previewLoading.value = false
  }
}

function downloadItem(item: FlatResourceItem) {
  const result = queueCanonicalResource(item, getGroup)
  error.value = result.duplicate ? '该资源已在下载中心' : ''
}

function downloadGroup(group: ResourceGroup) {
  const result = queueCanonicalGroup(group)
  error.value = result.duplicate ? '该资源组已在下载中心' : ''
}

function downloadPreviewTarget() {
  const target = previewTarget.value
  if (!target) return
  if (target.kind === 'item') downloadItem(target.value)
  else downloadGroup(target.value)
}

function groupTitle(group: ResourceGroup) {
  return group.product_name || group.sku_code || group.task_no || `资源组 ${group.id}`
}

function groupRows(group: ResourceGroup): Array<[string, string]> {
  const revision = currentCanonicalRevision(group)
  return [
    ['资源组', String(group.id)],
    ['任务', group.task_no || String(group.task_id)],
    ['SKU', group.sku_code || '任务级资源'],
    ['当前版本', revision ? String(revision.id) : '—'],
    ['资源结构', `${revision?.references.length || 0} 参考图 · ${revision?.source_file ? 1 : 0} 源文件 · ${revision?.items.length || 0} 成品`],
  ]
}

function itemRows(item: FlatResourceItem): Array<[string, string]> {
  return [
    ['资源类型', canonicalResourceRoleLabel(item.resource_role)],
    ['资源组', String(item.group_id)],
    ['当前版本', String(item.revision_id)],
    ['资源成员', String(item.resource_item_id)],
    ['任务', item.task_no || String(item.task_id)],
    ['SKU', item.sku_code || '任务级资源'],
    ['文件名', item.file_name],
  ]
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium' }).format(date)
}

onMounted(load)
</script>

<template>
  <section class="aw-resource-library" aria-label="主工程资源库">
    <header class="aw-resource-library__header">
      <div>
        <p class="aw-eyebrow">主工程当前资源</p>
        <h3>资源库</h3>
        <span>列表、预览和下载均使用主工程当前资源组与 OSS 权限链路。</span>
      </div>
      <button class="aw-secondary-button" type="button" :disabled="loading" @click="load">
        <RefreshCw :size="15" aria-hidden="true" />
        {{ loading ? '刷新中…' : '刷新' }}
      </button>
    </header>

    <form class="aw-resource-library__filters" @submit.prevent="search">
      <label class="aw-resource-library__search">
        <Search :size="17" aria-hidden="true" />
        <input v-model="query" type="search" aria-label="搜索主工程资源" placeholder="搜索 SKU、任务号或文件名" />
      </label>
      <label>
        <span>资源类型</span>
        <select v-model="role" aria-label="资源类型" @change="changeRole">
          <option v-for="option in canonicalResourceRoleOptions" :key="option.value || 'all'" :value="option.value">{{ option.label }}</option>
        </select>
      </label>
      <button class="aw-primary-button" type="submit">搜索</button>
    </form>

    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
    <p v-if="loading && !groups.length && !flatItems.length" class="aw-drive-empty" role="status">正在读取主工程当前资源…</p>
    <p v-else-if="!loading && !groups.length && !flatItems.length" class="aw-drive-empty">没有找到符合条件的资源</p>

    <div v-if="viewMode === 'flat' && flatItems.length" class="aw-resource-library__grid aw-resource-library__grid--flat">
      <article v-for="item in flatItems" :key="`${item.group_id}:${item.revision_id}:${item.resource_role}:${item.resource_item_id}`" class="aw-resource-library-card">
        <button class="aw-resource-library-card__preview" type="button" @click="openItemPreview(item)">
          <CanonicalResourceThumb :item="item" :alt="item.file_name" />
          <span class="aw-resource-library-card__role">{{ canonicalResourceRoleLabel(item.resource_role) }}</span>
        </button>
        <div class="aw-resource-library-card__body">
          <strong :title="item.file_name">{{ item.file_name }}</strong>
          <span>{{ item.sku_code || '任务级资源' }} · {{ item.task_no || item.task_id }}</span>
          <small>{{ item.resource_owner_name || '资源所属人待补充' }} · {{ formatDate(item.resource_created_at) }}</small>
        </div>
        <div class="aw-resource-library-card__actions">
          <button type="button" @click="openItemPreview(item)"><Eye :size="14" aria-hidden="true" />预览</button>
          <button type="button" @click="downloadItem(item)"><Download :size="14" aria-hidden="true" />下载</button>
        </div>
      </article>
    </div>

    <div v-else-if="groups.length" class="aw-resource-library__grid">
      <article v-for="group in groups" :key="group.id" class="aw-resource-library-card aw-resource-library-card--group">
        <button class="aw-resource-library-card__preview" type="button" @click="openGroupPreview(group)">
          <CanonicalResourceThumb :file="canonicalGroupCover(group)" :alt="groupTitle(group)" />
          <span class="aw-resource-library-card__role">{{ currentCanonicalRevision(group)?.mode === 'set' ? '套装' : '单图' }}</span>
        </button>
        <div class="aw-resource-library-card__body">
          <strong :title="groupTitle(group)">{{ groupTitle(group) }}</strong>
          <span>{{ group.sku_code || '任务级资源' }} · {{ group.task_no || group.task_id }}</span>
          <small class="aw-resource-library-card__structure">
            <b>参考图 {{ currentCanonicalRevision(group)?.references.length || 0 }}</b>
            <b>源文件 {{ currentCanonicalRevision(group)?.source_file ? 1 : 0 }}</b>
            <b>成品 {{ canonicalGroupFinals(group).length }}</b>
          </small>
        </div>
        <div class="aw-resource-library-card__actions">
          <button type="button" @click="openGroupPreview(group)"><Eye :size="14" aria-hidden="true" />预览</button>
          <button type="button" :disabled="!group.finalized_revision_id" @click="downloadGroup(group)"><Download :size="14" aria-hidden="true" />整组下载</button>
        </div>
      </article>
    </div>

    <nav v-if="totalPages > 1" class="aw-drive-pager" aria-label="资源分页">
      <button class="aw-grid-button" type="button" :disabled="loading || page <= 1" @click="changePage(page - 1)">上一页</button>
      <span>第 {{ page }} / {{ totalPages }} 页 · 共 {{ total }} {{ viewMode === 'flat' ? '项资源' : '个资源组' }}</span>
      <button class="aw-grid-button" type="button" :disabled="loading || page >= totalPages" @click="changePage(page + 1)">下一页</button>
    </nav>

    <WorkbenchPreviewDialog
      :open="previewOpen"
      :title="previewTitle"
      :preview-url="previewUrl"
      :mime-type="previewMimeType"
      :filename="previewTitle"
      eyebrow="主工程当前资源"
      :empty-label="previewLoading ? '正在加载预览…' : previewError || '当前资源暂不支持在线预览'"
      :meta-rows="previewRows"
      download-label="下载当前资源"
      windowed
      @close="previewOpen = false"
      @download="downloadPreviewTarget"
    />
  </section>
</template>
