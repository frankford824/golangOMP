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
import type { ClientMaterialRow, SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import CanonicalResourceThumb from './CanonicalResourceThumb.vue'
import ResourceGroupMaterialCard from './ResourceGroupMaterialCard.vue'
import {
  canonicalGroupCover,
  canonicalPreviewErrorMessage,
  canonicalPreviewUnavailableMessage,
  canonicalResourceRoleLabel,
  canonicalResourceRoleOptions,
  currentCanonicalRevision,
  resolveCanonicalPreview,
  type CanonicalResourceRole,
} from './canonicalResource'

const props = defineProps<{
  initialQuery?: string
  canPublish?: boolean
  clientMaterials?: ClientMaterialRow[]
  publishing?: boolean
}>()
const emit = defineEmits<{
  publish: [payload: { asset: SystemAssetRow; selection: { finalizedRevisionId: number; coverRevisionItemId: number } }]
  'batch-publish': [payload: { assets: SystemAssetRow[] }]
}>()
const { queueCanonicalGroup, queueCanonicalResource } = useGlobalDownload()
const query = ref(props.initialQuery || '')
const role = ref<CanonicalResourceRole>('')
const businessLane = ref<'' | 'normal' | 'customization'>('')
const publicationStatus = ref<'' | 'published' | 'unpublished'>('')
const loading = ref(false)
const error = ref('')
const groups = ref<ResourceGroup[]>([])
const flatItems = ref<FlatResourceItem[]>([])
const selectedGroupIDs = ref(new Set<number>())
const viewMode = ref<'group' | 'flat'>('group')
const page = ref(1)
const pageSize = 36
const statusScanPageSize = 200
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
const publicationByGroupID = computed(() => {
  const byGroupID = new Map<number, ClientMaterialRow>()
  for (const material of props.clientMaterials || []) {
    if (material.resource_group_id) byGroupID.set(material.resource_group_id, material)
  }
  return byGroupID
})
const selectedGroups = computed(() => groups.value.filter((group) => selectedGroupIDs.value.has(group.id) && canBatchPublish(group)))
const batchEligibleGroups = computed(() => groups.value.filter(canBatchPublish))
const allBatchEligibleSelected = computed(() => (
  batchEligibleGroups.value.length > 0
  && batchEligibleGroups.value.every((group) => selectedGroupIDs.value.has(group.id))
))
const setGroupCount = computed(() => groups.value.filter((group) => group.finalized_revision_id && !canBatchPublish(group)).length)

watch(() => props.initialQuery, (value) => {
  if (value === undefined || value === query.value) return
  query.value = value
  void search()
})

watch(() => props.clientMaterials, () => {
  selectedGroupIDs.value = new Set([...selectedGroupIDs.value].filter((groupID) => !publicationByGroupID.value.get(groupID)?.enabled))
  if (publicationStatus.value) {
    page.value = 1
    void load()
  }
})

function listParams(targetPage: number, targetPageSize: number) {
  return {
    q: query.value.trim() || undefined,
    resource_role: role.value || undefined,
    business_lane: businessLane.value || undefined,
    page: targetPage,
    page_size: targetPageSize,
  } as const
}

function matchesPublicationStatus(groupID: number) {
  const published = publicationByGroupID.value.get(groupID)?.enabled === true
  return publicationStatus.value === 'published' ? published : !published
}

async function loadStatusFiltered(currentRequest: number) {
  const allGroups: ResourceGroup[] = []
  const allFlatItems: FlatResourceItem[] = []
  let scanPage = 1
  let serverTotal = 0
  let scanned = 0
  let resolvedViewMode: 'group' | 'flat' = role.value ? 'flat' : 'group'

  do {
    const result = await resourceGroupsApi.list(listParams(scanPage, statusScanPageSize))
    if (currentRequest !== requestID) return null
    resolvedViewMode = result.view_mode || resolvedViewMode
    const pageGroups = result.items || []
    const pageFlatItems = result.flat_items || []
    allGroups.push(...pageGroups)
    allFlatItems.push(...pageFlatItems)
    serverTotal = Number(result.total || 0)
    const received = resolvedViewMode === 'flat' ? pageFlatItems.length : pageGroups.length
    scanned += received
    if (!received) break
    scanPage += 1
  } while (scanned < serverTotal)

  const filteredGroups = allGroups.filter((group) => matchesPublicationStatus(group.id))
  const filteredFlatItems = allFlatItems.filter((item) => matchesPublicationStatus(item.group_id))
  const filteredTotal = resolvedViewMode === 'flat' ? filteredFlatItems.length : filteredGroups.length
  const offset = (page.value - 1) * pageSize
  return {
    groups: filteredGroups.slice(offset, offset + pageSize),
    flatItems: filteredFlatItems.slice(offset, offset + pageSize),
    viewMode: resolvedViewMode,
    total: filteredTotal,
  }
}

async function load() {
  const currentRequest = ++requestID
  loading.value = true
  error.value = ''
  try {
    if (publicationStatus.value) {
      const result = await loadStatusFiltered(currentRequest)
      if (currentRequest !== requestID || !result) return
      groups.value = result.groups
      flatItems.value = result.flatItems
      viewMode.value = result.viewMode
      total.value = result.total
    } else {
      const result = await resourceGroupsApi.list(listParams(page.value, pageSize))
      if (currentRequest !== requestID) return
      groups.value = result.items || []
      flatItems.value = result.flat_items || []
      viewMode.value = result.view_mode || (role.value ? 'flat' : 'group')
      total.value = Number(result.total || 0)
    }
    for (const group of groups.value) groupCache.set(group.id, group)
    selectedGroupIDs.value = new Set([...selectedGroupIDs.value].filter((groupID) => groups.value.some((group) => group.id === groupID)))
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
  selectedGroupIDs.value = new Set()
  return load()
}

function changeRole() {
  page.value = 1
  selectedGroupIDs.value = new Set()
  void load()
}

function changeBusinessLane() {
  page.value = 1
  selectedGroupIDs.value = new Set()
  void load()
}

function changePublicationStatus() {
  page.value = 1
  selectedGroupIDs.value = new Set()
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

function groupAsMaterialAsset(group: ResourceGroup): SystemAssetRow {
  const revision = currentCanonicalRevision(group)
  const cover = canonicalGroupCover(group)
  return {
    id: group.id,
    source_type: 'task_resource_group',
    source_label: '任务资源组',
    resource_id: `group:${group.id}`,
    resource_group_id: group.id,
    finalized_revision_id: group.finalized_revision_id || undefined,
    cover_revision_item_id: revision?.items[0]?.id,
    resource_mode: revision?.mode,
    resource_item_count: revision?.items.length || 0,
    scope_sku_code: group.sku_code,
    sku_code: group.sku_code,
    product_name: groupTitle(group),
    task_no: group.task_no,
    business_lane: group.business_lane,
    file_name: cover?.filename,
    mime_type: cover?.mimeType,
    preview_available: Boolean(cover),
  }
}

function publicationForGroup(group: ResourceGroup) {
  return publicationByGroupID.value.get(group.id) || null
}

function publishGroup(group: ResourceGroup, selection: { finalizedRevisionId: number; coverRevisionItemId: number }) {
  emit('publish', { asset: groupAsMaterialAsset(group), selection })
}

function canBatchPublish(group: ResourceGroup) {
  const revision = currentCanonicalRevision(group)
  return Boolean(props.canPublish && group.finalized_revision_id && revision?.mode === 'single' && revision.items.length === 1)
}

function toggleGroup(group: ResourceGroup, checked: boolean) {
  if (!canBatchPublish(group)) return
  const next = new Set(selectedGroupIDs.value)
  if (checked) next.add(group.id)
  else next.delete(group.id)
  selectedGroupIDs.value = next
}

function toggleAllBatchEligible() {
  selectedGroupIDs.value = allBatchEligibleSelected.value
    ? new Set()
    : new Set(batchEligibleGroups.value.map((group) => group.id))
}

function batchPublishSelected() {
  if (props.publishing || !selectedGroups.value.length) return
  emit('batch-publish', { assets: selectedGroups.value.map(groupAsMaterialAsset) })
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
        <span>业务分类</span>
        <select v-model="businessLane" aria-label="业务分类" @change="changeBusinessLane">
          <option value="">全部分类</option>
          <option value="normal">常规</option>
          <option value="customization">定制</option>
        </select>
      </label>
      <label>
        <span>资源类型</span>
        <select v-model="role" aria-label="资源类型" @change="changeRole">
          <option v-for="option in canonicalResourceRoleOptions" :key="option.value || 'all'" :value="option.value">{{ option.label }}</option>
        </select>
      </label>
      <label v-if="canPublish">
        <span>上架状态</span>
        <select v-model="publicationStatus" aria-label="上架状态" @change="changePublicationStatus">
          <option value="">全部状态</option>
          <option value="published">已上架</option>
          <option value="unpublished">未上架</option>
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

    <template v-else-if="groups.length">
      <section v-if="canPublish" class="aw-resource-library__batch" aria-label="资源库批量上架">
        <div>
          <strong>{{ selectedGroups.length ? `已选 ${selectedGroups.length} 个资源组` : '批量上架' }}</strong>
          <span>单图资源可直接批量上架；套装需要单个选择客户封面。</span>
        </div>
        <div class="aw-resource-library__batch-actions">
          <button class="aw-secondary-button" type="button" :disabled="publishing || !batchEligibleGroups.length" @click="toggleAllBatchEligible">
            {{ allBatchEligibleSelected ? '取消全选' : '全选本页单图' }}
          </button>
          <button class="aw-primary-button" type="button" :disabled="publishing || !selectedGroups.length" @click="batchPublishSelected">
            {{ publishing ? '上架中…' : `批量上架${selectedGroups.length ? `（${selectedGroups.length}）` : ''}` }}
          </button>
        </div>
        <small v-if="setGroupCount">本页 {{ setGroupCount }} 个套装未加入批量范围，请使用卡片上的“发布到客户端”。</small>
      </section>
      <div class="aw-resource-library__grid">
        <div
          v-for="group in groups"
          :key="group.id"
          class="aw-resource-library__group-item"
          :class="{ 'is-selected': selectedGroupIDs.has(group.id) }"
        >
          <label v-if="canPublish" class="aw-resource-library__group-check" :title="canBatchPublish(group) ? '选择后批量上架' : '套装资源需单个选择客户封面'">
            <input
              type="checkbox"
              :checked="selectedGroupIDs.has(group.id)"
              :disabled="publishing || !canBatchPublish(group)"
              :aria-label="canBatchPublish(group) ? `选择批量上架：${groupTitle(group)}` : `不可批量上架：${groupTitle(group)}`"
              @change="toggleGroup(group, ($event.target as HTMLInputElement).checked)"
            />
          </label>
          <ResourceGroupMaterialCard
            :asset="groupAsMaterialAsset(group)"
            :published="publicationForGroup(group)"
            :cover-file="canonicalGroupCover(group)"
            :can-publish="canPublish && Boolean(group.finalized_revision_id)"
            :publishing="publishing"
            @preview="openGroupPreview(group)"
            @download="downloadGroup(group)"
            @publish="publishGroup(group, $event)"
          />
        </div>
      </div>
    </template>

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
