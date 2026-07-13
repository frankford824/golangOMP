<template>
  <div class="asset-detail-view min-h-[100dvh]">
    <div class="page-header">
      <div>
        <h2 class="page-title">资产详情</h2>
        <p class="page-subtitle">
          所属任务：<span class="cell-mono">{{ asset ? businessTaskNo(asset) : '加载中' }}</span>
          <span v-if="asset"> · SKU：<span class="cell-mono">{{ businessSku(asset) }}</span></span>
        </p>
      </div>
      <div class="page-actions">
        <BaseButton
          v-if="assetCanBeReplaced(asset)"
          variant="primary"
          size="sm"
          :disabled="replacementUploading"
          @click="startReplaceAsset"
        >
          {{ replacementUploading ? '上传中' : '修改资源' }}
        </BaseButton>
        <BaseButton
          v-if="assetCanBeDeleted(asset)"
          variant="danger"
          size="sm"
          :disabled="deletionRunning"
          @click="startDeleteAsset"
        >
          {{ deletionRunning ? '删除中' : '删除资源' }}
        </BaseButton>
        <BaseButton
          v-if="assetTaskId"
          variant="primary"
          size="sm"
          @click="goTaskDetail"
        >
          打开对应任务
        </BaseButton>
        <BaseButton
          v-if="taskId && canAccessPage('task_assets')"
          variant="secondary"
          size="sm"
          @click="goTaskAssets"
        >
          返回任务资产页
        </BaseButton>
        <BaseButton
          v-else-if="canAccessPage('assets_index')"
          variant="secondary"
          size="sm"
          @click="goAssetsIndex"
        >
          返回资产管理
        </BaseButton>
        <BaseButton size="sm" :disabled="loading" @click="loadAsset">
          {{ loading ? '加载中…' : '刷新详情' }}
        </BaseButton>
        <input
          ref="replacementFileInput"
          type="file"
          class="hidden-file-input"
          aria-label="选择替换资源文件"
          tabindex="-1"
          @change="handleReplacementFile"
        />
      </div>
    </div>

    <div v-if="replacementStatus || replacementError || deletionStatus || deletionError" class="replace-message">
      <span v-if="replacementStatus" class="replace-message-ok">{{ replacementStatus }}</span>
      <span v-if="replacementError" class="replace-message-error">{{ replacementError }}</span>
      <span v-if="deletionStatus" class="replace-message-ok">{{ deletionStatus }}</span>
      <span v-if="deletionError" class="replace-message-error">{{ deletionError }}</span>
    </div>

    <section class="detail-card">
      <div v-if="loading" class="state-text">加载中…</div>
      <div v-else-if="error" class="state-text state-error">{{ error }}</div>
      <BaseEmptyState
        v-else-if="!asset"
        title="资产不存在"
        description="未拿到资产详情，请确认资产 ID 或访问权限。"
      />
      <template v-else>
        <div
          v-if="!isExternalAsset(asset)"
          class="asset-usable-summary"
          :class="assetUsableToneClass(asset)"
        >
          <div>
            <span class="asset-usable-label">使用状态</span>
            <strong>{{ assetUsableLabel(asset) }}</strong>
          </div>
          <span v-if="assetCanBeReplaced(asset) || assetCanBeDeleted(asset)" class="asset-editable-label">
            {{ assetCanBeDeleted(asset) ? '可修改 / 可删除资源' : '可修改资源' }}
          </span>
        </div>
        <dl class="detail-grid">
          <div class="detail-row">
            <dt>资源来源</dt>
            <dd>{{ assetSourceLabel(asset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>{{ isExternalAsset(asset) ? '文件名' : 'SKU' }}</dt>
            <dd :class="{ 'cell-mono': !isExternalAsset(asset) }">
              {{ isExternalAsset(asset) ? assetFileName(asset) : businessSku(asset) }}
            </dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>所属任务号</dt>
            <dd class="cell-mono">{{ businessTaskNo(asset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>任务创建运营</dt>
            <dd>{{ taskCreatorLabel(asset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>使用状态</dt>
            <dd>
              <span class="detail-state-pill" :class="assetUsableToneClass(asset)">
                {{ assetUsableLabel(asset) }}
              </span>
            </dd>
          </div>
          <div v-if="!isExternalAsset(asset) && asset.cleanup_after_at" class="detail-row">
            <dt>旧版清理时间</dt>
            <dd>{{ displayTime(asset.cleanup_after_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>文件类型</dt>
            <dd>{{ imageBusinessTypeLabel(asset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>产生环节</dt>
            <dd>{{ assetSourceModuleLabel(asset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>上传时间</dt>
            <dd>{{ displayTime(asset.uploaded_at ?? asset.created_at) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>任务创建时间</dt>
            <dd>{{ displayTime(asset.task_created_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>文件名</dt>
            <dd>{{ assetFileName(asset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>产品名称</dt>
            <dd>{{ assetProductLabel(asset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>上传状态</dt>
            <dd>{{ assetUploadStatus(asset.upload_status) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>归档状态</dt>
            <dd>{{ assetArchiveStatus(asset.archive_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>{{ isExternalAsset(asset) ? '资源编号' : '系统资产号' }}</dt>
            <dd class="cell-mono">{{ displayText(assetResourceId(asset)) }}</dd>
          </div>
          <div v-if="!isExternalAsset(asset)" class="detail-row">
            <dt>当前有效稿件</dt>
            <dd class="cell-mono">{{ displayText(asset.current_asset_id ?? asset.id) }}</dd>
          </div>
          <div v-if="isExternalAsset(asset)" class="detail-row">
            <dt>外部资源状态</dt>
            <dd>{{ externalAssetStatusLabel(asset) }}</dd>
          </div>
          <div v-if="isExternalAsset(asset)" class="detail-row detail-row-full">
            <dt>外部路径</dt>
            <dd>{{ externalOriginPath(asset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>业务线 / 来源部门</dt>
            <dd>{{ laneAndDepartmentText(asset.workflow_lane, asset.source_department) }}</dd>
          </div>
          <div class="detail-row">
            <dt>最后访问</dt>
            <dd>{{ displayText(asset.last_access_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>下载模式</dt>
            <dd>{{ assetDownloadMode(downloadMeta?.download_mode) }}</dd>
          </div>
          <div class="detail-row">
            <dt>预览状态</dt>
            <dd>{{ previewStateLabel }}</dd>
          </div>
        </dl>

        <div class="versions-section">
          <div class="content-head">
            <h3 class="section-title">版本记录</h3>
            <span class="section-meta">共 {{ versions.length }} 条</span>
          </div>
          <BaseEmptyState
            v-if="!versions.length"
            title="暂无版本"
            description="该资产当前未返回版本信息。"
          />
          <div v-else class="version-list">
            <article v-for="version in versions" :key="version.id" class="version-card">
              <div class="version-top">
                <span class="version-title">版本 {{ displayText(version.version ?? version.id) }}</span>
                <span class="version-pill">{{ assetKind(version.file_role) }}</span>
              </div>
              <dl class="version-grid">
                <div class="detail-row">
                  <dt>文件名</dt>
                  <dd>{{ displayText(version.file_name) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>MIME</dt>
                  <dd>{{ displayText(version.mime_type) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>下载模式</dt>
                  <dd>{{ assetDownloadMode(version.download_mode) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>可预览</dt>
                  <dd>{{ version.preview_available === true ? '是' : version.preview_available === false ? '否' : '—' }}</dd>
                </div>
                <div class="detail-row">
                  <dt>创建时间</dt>
                  <dd>{{ displayTime(version.created_at) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>上传人</dt>
                  <dd>{{ versionCreatorLabel(version) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>使用状态</dt>
                  <dd>
                    <span class="detail-state-pill" :class="versionUsableToneClass(version)">
                      {{ versionUsableLabel(version) }}
                    </span>
                  </dd>
                </div>
                <div v-if="version.cleanup_after_at" class="detail-row">
                  <dt>清理时间</dt>
                  <dd>{{ displayTime(version.cleanup_after_at) }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </div>
      </template>
    </section>

    <AssetReplacementDialog
      v-model="replacementDialogOpen"
      :task-no="asset ? businessTaskNo(asset) : ''"
      :sku="asset ? businessSku(asset) : ''"
      :asset-kind="asset ? assetKind(asset) : ''"
      :current-file-name="asset ? assetFileName(asset) : ''"
      :selected-file="replacementSelectedFile"
      :uploading="replacementUploading"
      :status="replacementStatus"
      :error="replacementError"
      @choose-file="chooseReplacementFile"
      @confirm="confirmReplacement"
      @cancel="cancelReplacement"
    />
    <AssetDeletionDialog
      v-model="deletionDialogOpen"
      v-model:reason="deletionReason"
      :task-no="asset ? businessTaskNo(asset) : ''"
      :sku="asset ? businessSku(asset) : ''"
      :asset-kind="asset ? assetKind(asset) : ''"
      :current-file-name="asset ? assetFileName(asset) : ''"
      :reason-error="deletionReasonError"
      :deleting="deletionRunning"
      :status="deletionStatus"
      :error="deletionError"
      @confirm="confirmDeletion"
      @cancel="cancelDeletion"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import AssetDeletionDialog from '@/components/assets/AssetDeletionDialog.vue'
import AssetReplacementDialog from '@/components/assets/AssetReplacementDialog.vue'
import { usePermission } from '@/composables/usePermission'
import {
  assetArchiveStatusLabelCn,
  assetDownloadModeLabelCn,
  assetUploadStatusLabelCn,
} from '@/domain/mappers/read-model-labels-cn'
import { normalizeAssetDetailFromApi } from '@/domain/mappers/asset-detail-from-api'
import {
  fetchAssetPreviewMeta,
  fetchTaskAssetPreviewWithDerivedFallback,
  invalidateAssetAccessCache,
  primeAssetDownloadMetaCache,
} from '@/domain/asset-access'
import { assetsApi, type AssetKind } from '@/services/api/assetsApi'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import { assetReplacementScopeSKUCode, assetReplacementSuccessMessage, assetReplacementUnavailableReason, canReplaceAssetResource } from '@/domain/asset-replacement'
import { assetDeletionSuccessMessage, assetDeletionUnavailableReason, canDeleteAssetResource } from '@/domain/asset-deletion'
import { formatDateTimeBeijing } from '@/utils/date'
import { userAccountDisplay } from '@/domain/user-display'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'

const route = useRoute()
const router = useRouter()
const { canAccessPage, frontendRoles } = usePermission()

const ASSET_CENTER_FILE_TYPE_LABELS: Record<string, string> = {
  delivery: '成品图',
  reference: '参考图',
  source: '源文件',
  preview: '预览图',
  design_thumb: '预览图',
}

const assetId = computed(() => String(route.params.id ?? '').trim())
const taskId = computed(() => {
  const raw = route.query.task_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const loading = ref(false)
const error = ref('')
const asset = ref<BackendAsset | null>(null)
const downloadMeta = ref<Record<string, unknown> | null>(null)
const previewMeta = ref<Record<string, unknown> | null>(null)
/** GET /preview 返回 409：当前不可预览，仅可下载（非资源不存在） */
const previewUnavailable = ref(false)
/** GET /preview 返回 404：预览入口资源不存在 */
const previewNotFound = ref(false)
const previewPreparing = ref(false)
const replacementFileInput = ref<HTMLInputElement | null>(null)
const replacementDialogOpen = ref(false)
const replacementSelectedFile = ref<File | null>(null)
const replacementUploading = ref(false)
const replacementStatus = ref('')
const replacementError = ref('')
const deletionDialogOpen = ref(false)
const deletionReason = ref('')
const deletionReasonError = ref('')
const deletionRunning = ref(false)
const deletionStatus = ref('')
const deletionError = ref('')

const versions = computed<BackendAssetVersion[]>(() => asset.value?.versions ?? [])
const assetTaskId = computed(() => positiveID(asset.value?.task_id) || positiveID(taskId.value))

const previewStateLabel = computed(() => {
  if (previewPreparing.value) return '正在准备预览'
  if (previewUnavailable.value) return '当前不可预览（仅可下载，非不存在）'
  if (previewNotFound.value) return '预览资源不存在（404）'
  if (previewMeta.value?.preview_available === true) return '可预览'
  if (previewMeta.value?.preview_available === false) return '不可预览'
  if (previewMeta.value?.download_url) return '可预览'
  return '—'
})

function displayText(value: unknown): string {
  if (value == null) return '—'
  const text = String(value).trim()
  return text || '—'
}

function positiveID(value: unknown): string {
  const text = String(value ?? '').trim()
  if (!text) return ''
  const numeric = Number(text)
  if (Number.isFinite(numeric) && numeric <= 0) return ''
  return text
}

function displayTime(value: unknown): string {
  const text = displayText(value)
  if (text === '—') return text
  return formatDateTimeBeijing(text) || text
}

function rawAssetSourceType(row: BackendAsset | null | undefined): string {
  const record = row as Record<string, unknown> | null | undefined
  return String(record?.source_type ?? record?.sourceType ?? '').trim().toLowerCase()
}

function isExternalAsset(row: BackendAsset | null | undefined): boolean {
  if (!row) return false
  const record = row as Record<string, unknown>
  const resourceID = String(record.resource_id ?? record.resourceId ?? '').trim()
  return rawAssetSourceType(row) === 'external' || resourceID.startsWith('ext-')
}

function assetResourceId(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  const resourceID = String(record.resource_id ?? record.resourceId ?? '').trim()
  if (resourceID) return resourceID
  const id = String(row.id ?? '').trim()
  return isExternalAsset(row) && id && !id.startsWith('ext-') ? `ext-${id}` : id
}

function assetSourceLabel(row: BackendAsset | null | undefined): string {
  const record = row as Record<string, unknown> | null | undefined
  const label = String(record?.source_label ?? record?.sourceLabel ?? '').trim()
  if (label) return label
  return isExternalAsset(row) ? '外部资源' : '系统资源'
}

function externalAssetStatusLabel(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  const oss = String(record.oss_sync_status ?? record.ossSyncStatus ?? '').trim()
  const preview = String(record.external_preview_status ?? record.externalPreviewStatus ?? '').trim()
  const hasDisplayUrl = ['download_url', 'downloadUrl', 'preview_url', 'previewUrl'].some((key) => {
    const value = record[key]
    return typeof value === 'string' && value.trim().length > 5
  })
  const canPreview = record.preview_available === true || record.previewAvailable === true || hasDisplayUrl
  if (preview === 'ready' || canPreview) return '可预览'
  if (oss === 'ready') return '可下载'
  if (preview === 'pending') return '正在准备预览'
  if (oss === 'pending') return '正在准备下载'
  if (preview === 'failed' || oss === 'failed') return '外部资源暂时不可用'
  return '按需准备'
}

function rawUsableState(row: Record<string, unknown>): string {
  const state = String(row.usable_state ?? row.usableState ?? '').trim()
  if (state) return state
  const flow = String(row.flow_review_status ?? row.flowReviewStatus ?? '').trim()
  if (flow === 'approved') return 'ready_for_use'
  if (flow === 'pending_review') return 'pending_review'
  if (flow === 'rejected') return 'rejected'
  if (flow === 'superseded') return 'history'
  if (flow === 'cleaned') return 'cleaned'
  return 'not_applicable'
}

function usableLabelFromState(state: string): string {
  if (state === 'ready_for_use') return '可直接使用'
  if (state === 'pending_review') return '待审核'
  if (state === 'rejected') return '审核未通过'
  if (state === 'history') return '历史版本'
  if (state === 'cleaned') return '文件已清理'
  return '不进入审核流'
}

function assetUsableLabel(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  const label = String(record.usable_label ?? record.usableLabel ?? '').trim()
  return label || usableLabelFromState(rawUsableState(record))
}

function versionUsableLabel(version: BackendAssetVersion): string {
  const record = version as Record<string, unknown>
  const label = String(record.usable_label ?? record.usableLabel ?? '').trim()
  return label || usableLabelFromState(rawUsableState(record))
}

function usableToneClass(row: Record<string, unknown>): string {
  const state = rawUsableState(row)
  if (state === 'ready_for_use') return 'detail-state-pill--ready'
  if (state === 'pending_review') return 'detail-state-pill--pending'
  if (state === 'rejected') return 'detail-state-pill--rejected'
  if (state === 'history') return 'detail-state-pill--history'
  if (state === 'cleaned') return 'detail-state-pill--cleaned'
  return 'detail-state-pill--neutral'
}

function assetUsableToneClass(row: BackendAsset): string {
  return usableToneClass(row as Record<string, unknown>)
}

function versionUsableToneClass(version: BackendAssetVersion): string {
  return usableToneClass(version as Record<string, unknown>)
}

function rawAssetKind(row: BackendAsset | null | undefined): string {
  if (!row) return ''
  const record = row as Record<string, unknown>
  return String(record.asset_kind ?? record.asset_type ?? row.file_role ?? '').trim().toLowerCase()
}

function assetScopeSkuCode(row: BackendAsset | null | undefined): string {
  if (!row) return ''
  return assetReplacementScopeSKUCode(row as Record<string, unknown>)
}

function assetCanBeReplaced(row: BackendAsset | null | undefined): boolean {
  return canReplaceAssetResource(assetReplacementGate(row))
}

function assetCanBeDeleted(row: BackendAsset | null | undefined): boolean {
  return canDeleteAssetResource(assetReplacementGate(row), frontendRoles.value)
}

function assetMutationId(row: BackendAsset | null | undefined): string {
  if (!row) return ''
  const record = row as Record<string, unknown>
  return positiveID(record.asset_id ?? record.assetId) || positiveID(assetResourceId(row))
}

function assetReplacementGate(row: BackendAsset | null | undefined) {
  const record = (row ?? {}) as Record<string, unknown>
  return {
    isExternal: Boolean(row && isExternalAsset(row)),
    taskId: assetTaskId.value,
    assetId: assetMutationId(row),
    assetKind: rawAssetKind(row),
    usableState: rawUsableState(record),
    taskStatus: record.task_status ?? record.taskStatus,
    isArchived: record.is_archived ?? record.isArchived,
    archiveStatus: record.archive_status ?? record.archiveStatus,
  }
}

function assetReplacementUnavailableMessage(row: BackendAsset | null | undefined): string {
  return assetReplacementUnavailableReason(assetReplacementGate(row))
}

function assetDeletionUnavailableMessage(row: BackendAsset | null | undefined): string {
  return assetDeletionUnavailableReason(assetReplacementGate(row), frontendRoles.value)
}

function externalOriginPath(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  const path = String(record.origin_path ?? record.originPath ?? record.product_name ?? '').trim()
  return path || assetFileName(row)
}

function businessSku(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  for (const key of ['scope_sku_code', 'sku_code', 'primary_sku_code', 'target_sku_code'] as const) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return '未绑定 SKU'
}

function businessTaskNo(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  for (const key of ['task_no', 'taskNo'] as const) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  const id = positiveID(row.task_id)
  return id ? `任务 ${id}` : '未绑定任务'
}

function taskCreatorLabel(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  return userAccountDisplay(
    record.task_creator_username,
    record.task_creator_name,
    record.creator_username,
    record.creator_name,
    record.created_by_username,
    record.created_by_name,
  )
}

function assetFileName(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  for (const key of ['file_name', 'original_filename', 'filename'] as const) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return `${assetKind(row)} #${row.id}`
}

function fileFormatLabel(row: BackendAsset): string {
  const filename = assetFileName(row)
  const match = /\.([a-z0-9]{2,8})(?:$|[?#])/i.exec(filename)
  if (match?.[1]) return match[1].toUpperCase()
  const record = row as Record<string, unknown>
  const mime = String(record.mime_type ?? '').trim()
  if (mime.includes('/')) {
    const subtype = mime.split('/').pop()?.split(/[;+]/)[0]?.trim()
    if (subtype) return subtype.toUpperCase().replace('JPEG', 'JPG')
  }
  return '文件'
}

function imageBusinessTypeLabel(row: BackendAsset): string {
  return `${assetKind(row)} / ${fileFormatLabel(row)}`
}

function assetSourceModuleKey(row: BackendAsset | null | undefined): string {
  if (!row) return ''
  const record = row as Record<string, unknown>
  return String(record.source_module_key ?? record.module_key ?? '').trim().toLowerCase()
}

function assetSourceModuleLabel(row: BackendAsset | null | undefined): string {
  if (!row) return '—'
  if (isExternalAsset(row)) return '外部资源'
  switch (assetSourceModuleKey(row)) {
    case 'basic_info':
      return '基础信息参考'
    case 'design':
      return '设计提交'
    case 'audit':
      return '常规审核修订'
    case 'customization':
      return '定制链路上传'
    case 'retouch':
      return '精修需求素材'
    case 'warehouse':
      return '仓库处理'
    case 'procurement':
      return '采购资料'
    default:
      return '历史资产'
  }
}

function assetProductLabel(row: BackendAsset): string {
  const record = row as Record<string, unknown>
  const product = String(record.product_name ?? record.product_name_snapshot ?? '').trim()
  return product || '—'
}

function versionCreatorLabel(version: BackendAssetVersion): string {
  const record = version as Record<string, unknown>
  const actor = record.created_by && typeof record.created_by === 'object'
    ? record.created_by as Record<string, unknown>
    : {}
  return userAccountDisplay(actor.username, actor.name, record.created_by_username, record.created_by_name)
}

function assetKind(input: BackendAsset | string | null | undefined): string {
  if (typeof input === 'string') return assetFileTypeLabel(input)
  if (!input) return '—'
  const record = input as Record<string, unknown>
  return assetFileTypeLabel(record.asset_kind ?? record.asset_type ?? input.file_role)
}

function assetFileTypeLabel(value: unknown): string {
  const key = String(value ?? '').trim().toLowerCase()
  if (!key) return '—'
  return ASSET_CENTER_FILE_TYPE_LABELS[key] ?? String(value ?? '').trim()
}

function assetUploadStatus(value: unknown): string {
  return assetUploadStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetArchiveStatus(value: unknown): string {
  return assetArchiveStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetDownloadMode(value: unknown): string {
  return assetDownloadModeLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function laneAndDepartmentText(lane: unknown, department: unknown): string {
  const laneRaw = String(lane ?? '').trim().toLowerCase()
  const laneText = laneRaw === 'customization' ? '定制' : laneRaw === 'normal' ? '普通' : '—'
  const deptText = String(department ?? '').trim() || '—'
  return `${laneText} / ${deptText}`
}

function goTaskAssets() {
  if (!taskId.value || !canAccessPage('task_assets')) return
  void router.push({ name: 'TaskAssets', params: { id: taskId.value }, query: { asset_id: assetId.value } })
}

function goAssetsIndex() {
  if (!canAccessPage('assets_index')) return
  const query: Record<string, string> = taskId.value
    ? { task_id: taskId.value, asset_id: assetId.value }
    : { asset_id: assetId.value }
  if (isExternalAsset(asset.value)) query.source = 'external'
  void router.push({ name: 'AssetsIndex', query })
}

function goTaskDetail() {
  if (!assetTaskId.value) return
  void router.push({ name: 'TaskDetail', params: { id: assetTaskId.value } })
}

function startReplaceAsset() {
  if (!assetCanBeReplaced(asset.value)) {
    replacementStatus.value = ''
    replacementError.value = assetReplacementUnavailableMessage(asset.value)
    return
  }
  replacementStatus.value = ''
  replacementError.value = ''
  replacementSelectedFile.value = null
  replacementDialogOpen.value = true
}

function chooseReplacementFile() {
  if (replacementFileInput.value) {
    replacementFileInput.value.value = ''
    replacementFileInput.value.click()
  }
}

function handleReplacementFile(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (file) replacementSelectedFile.value = file
  if (input) input.value = ''
}

function cancelReplacement() {
  if (replacementUploading.value) return
  replacementDialogOpen.value = false
  replacementSelectedFile.value = null
  replacementStatus.value = ''
  replacementError.value = ''
}

async function confirmReplacement() {
  const file = replacementSelectedFile.value
  const row = asset.value
  if (!file || !row || replacementUploading.value) return

  const assetID = assetMutationId(row)
  const kind = rawAssetKind(row)
  const unavailableMessage = assetReplacementUnavailableMessage(row)
  if (unavailableMessage || !assetTaskId.value || !assetID || (kind !== 'delivery' && kind !== 'source' && kind !== 'reference')) {
    replacementStatus.value = ''
    replacementError.value = unavailableMessage || '当前资源缺少任务或资产信息，不能直接修改'
    return
  }

  replacementUploading.value = true
  replacementStatus.value = '正在上传并生成新版本'
  replacementError.value = ''
  try {
    await uploadTaskFileViaAssetSession(
      assetTaskId.value,
      file,
      {
        asset_id: assetID,
        asset_kind: kind as AssetKind,
        target_sku_code: assetScopeSkuCode(row) || undefined,
        remark: `资产详情修改资源：${file.name}`,
      },
      {
        onProgress: (progress) => {
          const percent = Number(progress.percent)
          replacementStatus.value = Number.isFinite(percent)
            ? `正在上传并生成新版本 ${Math.max(0, Math.min(100, Math.round(percent)))}%`
            : '正在上传并生成新版本'
        },
      },
    )
    const successMessage = assetReplacementSuccessMessage(assetReplacementGate(row))
    invalidateAssetAccessCache(assetID)
    await loadAsset()
    replacementStatus.value = successMessage
    replacementDialogOpen.value = false
    replacementSelectedFile.value = null
  } catch (err) {
    replacementStatus.value = ''
    replacementError.value = resolveApiUserMessage(err, { fallback: '修改资源失败，请稍后重试' })
  } finally {
    replacementUploading.value = false
  }
}

function startDeleteAsset() {
  const unavailableMessage = assetDeletionUnavailableMessage(asset.value)
  if (unavailableMessage) {
    deletionStatus.value = ''
    deletionError.value = unavailableMessage
    return
  }
  deletionReason.value = ''
  deletionReasonError.value = ''
  deletionStatus.value = ''
  deletionError.value = ''
  deletionDialogOpen.value = true
}

function cancelDeletion() {
  if (deletionRunning.value) return
  deletionDialogOpen.value = false
  deletionReason.value = ''
  deletionReasonError.value = ''
  deletionStatus.value = ''
  deletionError.value = ''
}

async function confirmDeletion() {
  const row = asset.value
  const reason = deletionReason.value.trim()
  if (!row || deletionRunning.value) return
  if (!reason) {
    deletionReasonError.value = '请填写删除原因'
    return
  }
  const unavailableMessage = assetDeletionUnavailableMessage(row)
  const mutationID = assetMutationId(row)
  if (unavailableMessage || !mutationID) {
    deletionError.value = unavailableMessage || '当前资源缺少资产信息，不能删除'
    return
  }

  deletionRunning.value = true
  deletionReasonError.value = ''
  deletionStatus.value = '正在删除资源及历史版本'
  deletionError.value = ''
  try {
    await assetsApi.deleteAsset(mutationID, { reason })
    invalidateAssetAccessCache(assetResourceId(row))
    invalidateAssetAccessCache(mutationID)
    deletionStatus.value = assetDeletionSuccessMessage(assetReplacementGate(row).taskStatus)
    deletionDialogOpen.value = false
    await router.push({ name: 'AssetsIndex', query: { asset_notice: 'deleted' } })
  } catch (err) {
    deletionStatus.value = ''
    deletionError.value = resolveApiUserMessage(err, { fallback: '删除资源失败，请稍后重试' })
  } finally {
    deletionRunning.value = false
  }
}

async function loadAsset() {
  if (!assetId.value) {
    asset.value = null
    error.value = '缺少资产 ID'
    return
  }
  loading.value = true
  error.value = ''
  asset.value = null
  downloadMeta.value = null
  previewMeta.value = null
  previewUnavailable.value = false
  previewNotFound.value = false
  previewPreparing.value = false
  try {
    const [assetRes, downloadRes] = await Promise.allSettled([
      assetsApi.getAsset(assetId.value),
      assetsApi.getAssetDownloadMeta(assetId.value),
    ])

    if (assetRes.status === 'fulfilled') {
      asset.value = normalizeAssetDetailFromApi(assetRes.value.data)
    }

    if (downloadRes.status === 'fulfilled') {
      const body = downloadRes.value.data as { data?: Record<string, unknown> } | undefined
      downloadMeta.value = body?.data ?? null
      primeAssetDownloadMetaCache(assetId.value, downloadRes.value.data)
    }

    const tidFromAsset = String(
      (asset.value as Record<string, unknown> | null)?.task_id ??
        (asset.value as BackendAsset | null)?.task_id ??
        '',
    ).trim()
    const taskIdForPreview = (taskId.value || tidFromAsset).trim() || undefined
    const previewResult = isExternalAsset(asset.value)
      ? await fetchAssetPreviewMeta(assetId.value)
      : await fetchTaskAssetPreviewWithDerivedFallback(
          assetId.value,
          taskIdForPreview,
        )
    if (previewResult.status === 'ok' && previewResult.displayUrl) {
      previewMeta.value = {
        download_url: previewResult.displayUrl,
        preview_available: true,
      }
    } else if (previewResult.status === 'preparing') {
      previewPreparing.value = true
    } else if (previewResult.status === 'unavailable') {
      previewUnavailable.value = true
    } else if (previewResult.status === 'not_found') {
      previewNotFound.value = true
    }

    if (!asset.value) {
      error.value = '未获取到资产详情'
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载资产详情失败'
  } finally {
    loading.value = false
  }
}

watch(
  () => assetId.value,
  () => {
    void loadAsset()
  },
)

onMounted(() => {
  void loadAsset()
})
</script>

<style scoped>
.asset-detail-view {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0.5rem 0 1rem;
  background: rgb(var(--yb-surface-slate));
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}
.page-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-muted-strong));
}
.page-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.hidden-file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}
.replace-message {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 0.875rem;
  background: rgb(var(--yb-brand-soft));
  padding: 0.65rem 0.85rem;
  font-size: 0.8125rem;
  font-weight: 700;
}
.replace-message-ok {
  color: rgb(var(--yb-brand-strong));
}
.replace-message-error {
  color: rgb(var(--yb-danger-text));
}
.detail-card {
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 1rem;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
  padding: 1rem;
}
.state-text {
  font-size: 0.875rem;
  color: rgb(var(--yb-text-soft));
}
.state-error {
  color: rgb(var(--yb-danger-text));
}
.cell-mono {
  font-family: var(--yb-font-data);
}
.detail-grid,
.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 0.5rem 1rem;
  margin: 0;
}
.detail-row {
  margin: 0;
}
.detail-row-full {
  grid-column: 1 / -1;
}
.detail-row dt {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  margin-bottom: 0.2rem;
}
.detail-row dd {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-navy));
  word-break: break-word;
}
.asset-usable-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 1rem;
  border-radius: 0.875rem;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  padding: 0.75rem 0.9rem;
}
.asset-usable-label {
  display: block;
  margin-bottom: 0.18rem;
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.72rem;
  font-weight: 700;
}
.asset-usable-summary strong {
  color: inherit;
  font-size: 1rem;
  font-weight: 900;
}
.asset-usable-summary.detail-state-pill--ready {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-ui-soft));
  color: rgb(var(--yb-success-strong));
}
.asset-usable-summary.detail-state-pill--pending {
  border-color: rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
}
.asset-usable-summary.detail-state-pill--rejected {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}
.asset-usable-summary.detail-state-pill--history,
.asset-usable-summary.detail-state-pill--cleaned,
.asset-usable-summary.detail-state-pill--neutral {
  border-color: rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-muted-strong));
}
.asset-editable-label {
  flex: 0 0 auto;
  border-radius: 999px;
  background: rgb(var(--yb-teal));
  color: rgb(var(--yb-surface));
  padding: 0.24rem 0.62rem;
  font-size: 0.72rem;
  font-weight: 900;
}
.detail-state-pill {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  border-radius: 9999px;
  padding: 0.18rem 0.56rem;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.75rem;
  font-weight: 800;
  line-height: 1.15;
}
.detail-state-pill--ready {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-ui-soft));
  color: rgb(var(--yb-success-strong));
}
.detail-state-pill--pending {
  border-color: rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
}
.detail-state-pill--rejected {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}
.detail-state-pill--history,
.detail-state-pill--cleaned,
.detail-state-pill--neutral {
  border-color: rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-muted-strong));
}
.versions-section {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--yb-border-slate));
}
.content-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.section-title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}
.section-meta {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}
.version-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 0.75rem;
}
.version-card {
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.875rem;
  padding: 0.875rem;
  background: rgb(var(--yb-surface-subtle));
}
.version-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}
.version-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}
.version-pill {
  border-radius: 9999px;
  background: rgb(var(--yb-indigo-soft));
  color: rgb(var(--yb-indigo-text));
  padding: 0.15rem 0.55rem;
  font-size: 0.6875rem;
  font-weight: 600;
}

/* Phase 6: light admin asset detail skin. Style-only. */
.asset-detail-view {
  background: transparent;
  color: rgb(var(--yb-text));
}

.detail-card,
.version-card,
.preview-media-shell {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.page-title,
.section-title,
.version-title,
.detail-row dd {
  color: rgb(var(--yb-text));
}

.page-subtitle,
.section-meta,
.detail-row dt,
.state-text {
  color: rgb(var(--yb-text-muted));
}

.version-pill {
  background: rgb(var(--yb-brand-soft));
  border: 1px solid rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand-strong));
}
</style>
