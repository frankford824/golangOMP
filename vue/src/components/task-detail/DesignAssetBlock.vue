<template>
  <section
    ref="designAssetSectionRef"
    class="detail-block detail-block--asset h-full flex flex-col rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-sm p-6"
    :class="{ 'detail-block--result': showResultDisplayState }"
  >
    <div class="block-header">
      <div class="flex items-center gap-2">
        <span class="block-icon">D</span>
        <h3 class="block-title">{{ designAssetBlockTitle }}</h3>
      </div>
      <div v-if="designAssetStatusRow && !isPurchase" class="status-inline">
        <span class="status-dot" :class="designAssetStatusRow.dotClass" />
        {{ designAssetStatusRow.text }}
      </div>
    </div>

    <!-- 设计师 / 美工处理人（采购任务无设计节点，不展示） -->
    <div v-if="designerLine && !isPurchase" class="info-row-simple">
      <span class="row-label">{{ designerRoleLabel }}</span>
      <span>{{ designerLine }}</span>
    </div>
    <div v-else-if="canShowCustomizationClaim && !isPurchase" class="info-row-simple customization-claim-row">
      <span class="row-label">{{ designerRoleLabel }}</span>
      <div class="customization-claim-actions">
        <BaseButton
          variant="primary"
          size="sm"
          :disabled="customizationClaiming"
          @click="onCustomizationClaim"
        >
          {{ customizationClaimButtonLabel }}
        </BaseButton>
        <p v-if="customizationClaimError" class="customization-claim-error">{{ customizationClaimError }}</p>
      </div>
    </div>
    <div v-if="task.needOutsource && !isPurchase" class="outsource-flag">
      已标记外协意图（need_outsource）
    </div>

    <DesignAssetResultBlock
      v-if="showResultDisplayState"
      :is-retouch-task="isRetouchTask"
      :batch-ui="batchUi"
      :show-reference-pane="showReferencePane"
      :reference-thumb-items="referenceThumbItems"
      :reference-entry-count="referencePaneEntries.length"
      :design-asset-layout-class="designAssetLayoutClass"
      :scoped-asset-version-groups="scopedAssetVersionGroups"
      :shared-asset-versions="sharedAssetVersions"
      :active-version-idx="activeVersionIdx"
      :active-version="activeVersion"
      :flat-index-of-version="flatIndexOfVersion"
      :is-version-unavailable="isVersionUnavailable"
      :is-audit-replacement-version="isAuditReplacementVersion"
      @activate-version="activateVersion"
      @open-lightbox="openLightbox"
      @open-shared-version="openSharedVersionPreview"
    />

    <template v-else>
    <!-- 并列商品切换在「商品与编码信息」卡片；本区随详情页共用 productIndex -->
    <!-- 分栏：左参考图（审核工作台式）、右版本/交付/源文件与上传（采购任务仅单列参考） -->
    <div :class="designAssetLayoutClass">
      <div v-if="showReferencePane" class="design-asset-pane design-asset-pane--refs" aria-label="参考图">
    <!-- 参考图：后端资产 + 并列商品 referenceFileRefs（采购任务展示创建时上传的参考图） -->
    <div class="asset-section ref-rail-section">
      <div class="ref-section-label-row">
        <span class="section-label">{{ batchUi ? '参考图（当前商品）' : '参考图' }}</span>
        <span v-if="referencePaneEntries.length > 1" class="ref-pane-hint">缩略图预览</span>
      </div>
      <AssetThumbStrip
        :items="referenceThumbItems"
        empty-text="暂无参考图"
        size="sm"
      />
      <p
        v-if="
          !isPurchase &&
          referenceDisplayList.length === 0 &&
          currentReferenceRefs.length === 0
        "
        class="text-xs text-[rgb(var(--yb-text-muted))] mt-1"
      >
        暂无参考图
      </p>
      <p
        v-if="
          isPurchase &&
          referenceDisplayList.length === 0 &&
          currentReferenceRefs.length === 0
        "
        class="text-xs text-[rgb(var(--yb-text-muted))] mt-1"
      >
        暂无参考图
      </p>
    </div>
      </div>

      <div v-if="!isPurchase" class="design-asset-pane design-asset-pane--drafts" aria-label="设计稿与版本">
    <!-- 资产版本列表（采购任务不展示） -->
    <div v-if="scopedAssetVersions.length > 0" class="asset-section">
      <p v-if="sharedAssetVersions.length > 0" class="shared-asset-note">
        共享稿件（未绑定 SKU）{{ sharedAssetVersions.length }} 个版本，未并入当前 SKU 版本。
      </p>
      <div v-if="sharedAssetVersions.length > 0" class="shared-asset-strip">
        <button
          v-for="(v, i) in sharedAssetVersions"
          :key="'shared-ver-' + v.id"
          type="button"
          class="shared-asset-chip"
          @click="openSharedVersionPreview(v)"
        >
          共享 V{{ v.rootVersionNo ?? i + 1 }} · {{ versionTotalFileCount(v) }} 文件
        </button>
      </div>
      <div class="version-header">
        <span class="section-label">版本时间线</span>
      </div>
      <!-- 按资产根分组展示版本序列：交付 / 设计原稿 各自独立一条 -->
      <div
        v-for="group in scopedAssetVersionGroups"
        :key="'grp-' + group.assetNo"
        class="version-group"
      >
        <div class="version-group-label">
          <span class="version-group-kind" :class="'kind-' + group.assetKind">
            {{ group.kindLabel }}
          </span>
          <span class="version-group-no">{{ group.assetNo }}</span>
        </div>
        <div class="version-strip version-strip-scroll">
          <button
            v-for="v in group.versions"
            :key="v.id"
            type="button"
            class="version-btn"
            :class="{
              'version-active': flatIndexOfVersion(v) === activeVersionIdx,
              'version-disabled': isVersionUnavailable(v),
            }"
            :disabled="isVersionUnavailable(v)"
            :title="isVersionUnavailable(v) ? '该版本上传未完成，暂无可预览或可下载文件' : ''"
            @click="activateVersion(flatIndexOfVersion(v))"
          >
            V{{ v.rootVersionNo ?? 1 }}
            <span v-if="isAuditReplacementVersion(v)" class="version-replace-tag">替换</span>
            <span v-if="versionTotalFileCount(v) > 1" class="version-file-count">{{ versionTotalFileCount(v) }}文件</span>
            <span v-if="isVersionUnavailable(v)" class="version-unavailable-tag">不可看</span>
          </button>
        </div>
      </div>
      <!-- 当前版本：版本内多文件缩略图 -->
      <div v-if="activeVersion && versionTotalFileCount(activeVersion) > 0" class="version-preview">
        <template v-if="isVersionUnavailable(activeVersion)">
          <div class="nonpreview-panel">
            <p class="nonpreview-name">该历史版本上传未完成</p>
            <p class="nonpreview-hint">当前未返回可预览图或下载地址，通常是失败上传留下的占位版本。</p>
          </div>
        </template>
        <template v-else>
          <AssetThumbStrip
            :items="activeVersionThumbItems"
            empty-text="暂无可预览文件"
            size="sm"
            @select="onActiveVersionThumbSelect"
          />
          <div
            v-if="
              activeVersion.assetRootId ||
              downloadHrefForAssetPreviewSlot(activeVersion, activeFileIdx) ||
              activeNonPreviewItem?.url
            "
            class="version-preview-dl"
          >
            <AssetDownloadLink
              variant="button"
              :asset-id="activeVersion.assetRootId"
              :href="downloadHrefForAssetPreviewSlot(activeVersion, activeFileIdx) || activeNonPreviewItem?.url"
            />
          </div>
          <p v-if="activeNonPreviewItem" class="nonpreview-hint">
            当前选中文件为源文件，不支持在线预览，请使用下载按钮查看。
          </p>
        <div class="version-meta">
          <span v-if="isAuditReplacementVersion(activeVersion)" class="meta-item meta-item--replace">审核替换</span>
          <span v-if="isAuditReplacementVersion(activeVersion)" class="meta-sep">·</span>
          <span class="meta-item">{{ activeVersion.uploaderName }}</span>
          <span class="meta-sep">·</span>
          <span class="meta-item">{{ formatDate(activeVersion.uploadedAt) }}</span>
          <span v-if="versionTotalFileCount(activeVersion) > 1" class="meta-sep">·</span>
          <span v-if="versionTotalFileCount(activeVersion) > 1" class="meta-item">{{ versionTotalFileCount(activeVersion) }} 个文件</span>
        </div>
        </template>
      </div>
    </div>
    <div v-else class="empty-assets">
      <template v-if="batchUi && scopedAssetVersions.length === 0">
        当前商品暂无版本记录；可切换商品查看或{{ uploadActionLabel }}
      </template>
      <template v-else>暂无版本，请{{ uploadActionLabel }}</template>
    </div>

    <!-- 交付设计稿上传（与 DesignWorkbench 共用 DesignAssetPanel） -->
    <DesignAssetPanel
      v-if="!isPurchase && canShowDesignUploadPanel"
      :task-id="task.id"
      :can-upload="true"
      :can-submit-audit="canSubmitFromDesignPanel"
      :submit-button-label="submitButtonLabel"
      :upload-button-label="uploadButtonLabel"
      :submit-hint-idle="submitHintIdle"
      :upload-context-label="designPanelCaption"
      :delivery-remark-suffix="designRemarkSuffix"
      :active-sku-code="activeSkuCodeForPanel || undefined"
      :staging-bucket-key="deliveryStagingBucketKeyForPanel || undefined"
      :get-delivery-remark-suffix-by-sku="getDeliveryRemarkSuffixBySkuForPanel"
      :resolve-staging-target-sku="resolveDeliveryStagingTargetSku"
      @success="onDeliveryPanelSuccess"
    />
      </div>
    </div>
    </template>

  </section>
</template>

<script setup lang="ts">
import { ref, computed, inject, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task, TaskAssetVersion } from '@/domain/types/task'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { TASK_DETAIL_PRODUCT_INDEX_KEY } from '@/composables/task-detail-product-index'
import { getDesignSubStatusLabel } from '@/domain/enums/task-status'
import { getCustomizationDetailStatusLabel } from '@/domain/task-center-card-status'
import { retouchDesignAssetStatusDisplay } from '@/domain/retouch-display'
import {
  canSubmitAudit,
  canUploadDesignDelivery,
} from '@/domain/task-actions'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import { useTasksStore } from '@/stores/tasks'
import BaseButton from '@/components/base/BaseButton.vue'
import {
  canClaimCustomizationOnTaskDetail,
  taskCenterClaimButtonLabel,
} from '@/domain/task-center-claim'
import { formatTaskActionDenyMessage } from '@/domain/task-action-deny'
import { assetsApi } from '@/services/api/assetsApi'
import DesignAssetPanel from '@/components/business/DesignAssetPanel.vue'
import DesignAssetResultBlock from '@/components/task-detail/DesignAssetResultBlock.vue'
import { formatDateTimeBeijingOffsetAware } from '@/utils/date'
import { toRelativeAssetUrl } from '@/utils/url'
import { resolveBackendPreviewAssetId } from '@/domain/asset-access'
import {
  downloadHrefForAssetPreviewSlot,
} from '@/domain/task-asset-preview-slot'
import {
  isTaskAssetVersionUnavailable,
  preferredTaskAssetVersionIndex,
} from '@/domain/task-asset-version-selection'
import AssetDownloadLink from '@/components/media/AssetDownloadLink.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import {
  IMAGE_PREVIEW_LIGHTBOX_KEY,
  type ImagePreviewLightboxItem,
  type OpenImagePreviewLightbox,
} from '@/components/media/imagePreviewLightbox'
import { versionTotalFileCount } from '@/utils/task-ui-labels'
import {
  activeSkuCodeForSelection,
  assetVersionMatchesActiveSku,
  assetVersionIsSharedForBatch,
  backendAssetMatchesObject,
  deliveryRemarkSuffix,
  designUploadCaption,
  referenceRefsForObject,
  selectionFromProductIndex,
  taskHasSkuItemsForBatchUi,
} from '@/domain/task-batch-assets'
import { taskDesignerDisplayName } from '@/domain/task-actors'
import { referenceFileRefDedupeKey } from '@/domain/mappers/reference-file-refs'

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[DesignAssetBlock] 必须在 TaskDetailView 内使用')

const productIdxCtx = inject(TASK_DETAIL_PRODUCT_INDEX_KEY, null)

const task = computed(() => injected.value!)

const designerLine = computed(() => {
  const s = taskDesignerDisplayName(task.value)
  return s === '-' ? '' : s
})

const { can, currentUser, frontendRoles } = usePermission()
const permissionsStore = usePermissionsStore()
const tasksStore = useTasksStore()

const customizationClaiming = ref(false)
const customizationClaimError = ref('')

const canShowCustomizationClaim = computed(() => {
  if (!isCustomizationTask.value || isPurchase.value) return false
  return canClaimCustomizationOnTaskDetail(
    task.value,
    (roles) => permissionsStore.hasAnyRole(roles),
    permissionsStore.isCustomizationOperator,
  )
})

const customizationClaimButtonLabel = computed(() =>
  taskCenterClaimButtonLabel(task.value, customizationClaiming.value),
)

async function onCustomizationClaim() {
  if (!canShowCustomizationClaim.value || customizationClaiming.value) return
  customizationClaimError.value = ''
  customizationClaiming.value = true
  try {
    await tasksStore.claimCustomizationModule(task.value.id)
    await tasksStore.loadTaskById(task.value.id)
  } catch (e) {
    customizationClaimError.value = formatTaskActionDenyMessage(
      e,
      '任务已被他人接单，请刷新详情后重试',
    )
    await tasksStore.loadTaskById(task.value.id)
  } finally {
    customizationClaiming.value = false
  }
}

const isPurchase = computed(
  () =>
    task.value.businessType === 'PURCHASE_TASK' || task.value.taskType === 'PURCHASE_TASK',
)
const isRetouchTask = computed(
  () =>
    task.value.businessType === 'RETOUCH_TASK' || task.value.taskType === 'RETOUCH_TASK',
)
const isCustomizationTask = computed(
  () =>
    task.value.workflowLane === 'customization' ||
    task.value.businessLane === 'customization' ||
    task.value.customizationRequired === true,
)

const retouchModuleSummary = computed(() =>
  task.value.moduleSummaries?.find((m: { module_key: string }) => m.module_key === 'retouch'),
)

const retouchModuleState = computed(() => retouchModuleSummary.value?.state ?? '')

const retouchModuleCanUpload = computed(() => {
  if (!isRetouchTask.value) return false
  const mod = retouchModuleSummary.value
  if (mod?.state === 'in_progress') return true
  if (task.value.designerId) return true
  return false
})

const isDesignChainTask = computed(() => !isPurchase.value && !isRetouchTask.value)
const designAssetBlockTitle = computed(() => isCustomizationTask.value ? '定制稿与资产' : '设计与资产')
const designerRoleLabel = computed(() => isCustomizationTask.value ? '美工处理人' : '设计师')
const uploadActionLabel = computed(() => isCustomizationTask.value ? '上传定制设计稿' : '上传设计稿')
const submitButtonLabel = computed(() => {
  if (isRetouchTask.value) return '提交精修'
  if (isCustomizationTask.value) return '提交定制审核'
  return undefined
})
const uploadButtonLabel = computed(() =>
  isCustomizationTask.value ? '上传/拖拽/粘贴定制设计稿' : '上传/拖拽/粘贴设计稿',
)
const submitHintIdle = computed(() =>
  isCustomizationTask.value
    ? '提交后定制设计稿将进入定制审核队列，本次文件锁定为新版本'
    : '',
)
const customizationSubmitRoles = [
  'CustomizationOperator',
  'customization_operator',
  'Ops',
  'ops',
  'Admin',
  'admin',
  'SuperAdmin',
  'super_admin',
  'HRAdmin',
  'hr_admin',
  'RoleAdmin',
  'role_admin',
  'DepartmentAdmin',
  'department_admin',
  'TeamLead',
  'team_lead',
  'DesignDirector',
  'design_director',
] as const
const canUseCustomizationSubmit = computed(() => {
  const role = String(currentUser.value?.role ?? '').trim()
  const roles = frontendRoles.value.map((item) => String(item).trim())
  const normalized = new Set([role, ...roles].map((item) => item.toLowerCase()).filter(Boolean))
  return customizationSubmitRoles.some((candidate) => normalized.has(String(candidate).toLowerCase()))
})
const canShowDesignUploadPanel = computed(() => {
  if (!can('design.upload')) return false
  if (isRetouchTask.value) return retouchModuleCanUpload.value
  if (isCustomizationTask.value && !canUseCustomizationSubmit.value) return false
  return canUploadDesignDelivery(task.value)
})

/** 设计链：已有版本且已离开上传/提交审核操作态 */
const showDesignResultDisplayState = computed(() => {
  if (!isDesignChainTask.value) return false
  if (scopedAssetVersions.value.length === 0) return false
  if (canUploadDesignDelivery(task.value)) return false
  if (canSubmitAudit(task.value)) return false
  return true
})

/** 精修：已有版本且模块已进入提交/完成类状态 */
const showRetouchResultDisplayState = computed(() => {
  if (!isRetouchTask.value || isPurchase.value) return false
  if (scopedAssetVersions.value.length === 0) return false
  const state = retouchModuleState.value
  if (state === 'submitted' || state === 'closed' || state === 'completed') return true
  const ts = task.value.status
  if (ts === 'PendingAuditA' || ts === 'PendingAuditB' || ts === 'Completed' || ts === 'Archived') {
    return true
  }
  return false
})

const showResultDisplayState = computed(
  () => showDesignResultDisplayState.value || showRetouchResultDisplayState.value,
)

const batchUi = computed(
  () => taskHasSkuItemsForBatchUi(task.value) && !isPurchase.value,
)

const assetObjectSel = computed(() =>
  selectionFromProductIndex(task.value, productIdxCtx?.productIndex.value ?? 0),
)

const designPanelCaption = computed(() =>
  batchUi.value ? designUploadCaption(assetObjectSel.value, task.value) : '',
)
const designRemarkSuffix = computed(() =>
  batchUi.value ? deliveryRemarkSuffix(assetObjectSel.value, task.value) : '',
)

const activeSkuCodeForPanel = computed(() => {
  if (!batchUi.value) return ''
  return activeSkuCodeForSelection(task.value, assetObjectSel.value) ?? ''
})

const deliveryStagingBucketKeyForPanel = computed(() => {
  if (!batchUi.value) return ''
  const t = task.value
  const sku = activeSkuCodeForPanel.value.trim()
  if (sku) return `${t.id}::${sku}`
  const idx = productIdxCtx?.productIndex.value ?? 0
  return `${t.id}::__row_${idx}`
})

function resolveDeliveryStagingTargetSku(bucketKey: string): string | undefined {
  const t = task.value
  const prefix = `${t.id}::`
  if (!bucketKey.startsWith(prefix)) return undefined
  const rest = bucketKey.slice(prefix.length).trim()
  if (rest.startsWith('__row_')) {
    const i = Number(rest.slice(6))
    if (Number.isFinite(i)) return t.skuItems?.[i]?.skuCode?.trim() || undefined
    return undefined
  }
  return rest || undefined
}

const getDeliveryRemarkSuffixBySkuForPanel = computed((): ((skuCode: string) => string) | undefined => {
  if (!batchUi.value) return undefined
  const t = task.value
  return (skuCode: string) => {
    const idx = (t.skuItems ?? []).findIndex((it) => it.skuCode?.trim() === skuCode.trim())
    if (idx < 0) return ''
    return deliveryRemarkSuffix(selectionFromProductIndex(t, idx), t)
  }
})

const activeVersionIdx = ref(0)
const activeFileIdx = ref(0)
const hasManualActiveVersionSelection = ref(false)
const openLightbox = inject<OpenImagePreviewLightbox>(IMAGE_PREVIEW_LIGHTBOX_KEY, () => {})

/** 后端资产列表（GET /v1/tasks/{id}/assets） */
const backendAssets = ref<BackendAsset[]>([])
const designAssetSectionRef = ref<HTMLElement | null>(null)
let backendAssetsIo: IntersectionObserver | null = null

function disconnectBackendAssetsObserver() {
  backendAssetsIo?.disconnect()
  backendAssetsIo = null
}

function bindBackendAssetsObserver() {
  disconnectBackendAssetsObserver()
  void nextTick(() => {
    const el = designAssetSectionRef.value
    if (!el || !task.value?.id) return
    backendAssetsIo = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && task.value?.id) {
            void loadBackendAssets()
            disconnectBackendAssetsObserver()
          }
        }
      },
      { root: null, rootMargin: '160px', threshold: 0.02 },
    )
    backendAssetsIo.observe(el)
  })
}

async function loadBackendAssets() {
  if (!task.value?.id) return
  try {
    const res = await assetsApi.list(task.value.id)
    const data = res?.data as BackendAsset[] | { data?: BackendAsset[]; items?: BackendAsset[] } | undefined
    const list = Array.isArray(data) ? data : (data?.data ?? data?.items ?? [])
    backendAssets.value = Array.isArray(list) ? list : []
  } catch {
    backendAssets.value = []
  }
}

const scopedBackendAssets = computed(() => {
  if (!batchUi.value) return backendAssets.value
  return backendAssets.value.filter((a) =>
    backendAssetMatchesObject(a, assetObjectSel.value, task.value),
  )
})

function backendAssetKind(a: BackendAsset): string {
  const rec = a as Record<string, unknown>
  return String(
    rec.asset_kind ?? rec.assetKind ?? rec.asset_type ?? rec.assetType ?? rec.file_role ?? '',
  ).toLowerCase()
}

function backendAssetVersions(a: BackendAsset): BackendAssetVersion[] {
  const rec = a as Record<string, unknown>
  const list = rec.versions
  if (Array.isArray(list)) return list as BackendAssetVersion[]
  const current = rec.current_version ?? rec.currentVersion
  return current && typeof current === 'object' ? [current as BackendAssetVersion] : []
}

// 展示 URL：统一使用后端下发 download_url；无该字段时仅兼容 public_url。
function pickDisplayUrl(v: BackendAssetVersion): string {
  const rec = v as Record<string, unknown>
  const d1 = typeof rec.download_url === 'string' ? rec.download_url.trim() : ''
  const d2 = typeof rec.downloadUrl === 'string' ? rec.downloadUrl.trim() : ''
  const dlRaw = d1 || d2
  const dl = dlRaw ? (toRelativeAssetUrl(dlRaw) ?? dlRaw) : ''
  if (dl) return dl
  const publicUrl = toRelativeAssetUrl(v.public_url) ?? v.public_url
  return publicUrl ?? ''
}
function refDisplayItem(v: BackendAssetVersion, asset: BackendAsset) {
  const rec = v as Record<string, unknown>
  const assetRec = asset as Record<string, unknown>
  const previewOk = v.preview_available === true
  const mode = (v.download_mode ?? 'public') as string
  const publicUrl =
    mode !== 'private_network' ? (toRelativeAssetUrl(v.public_url) ?? v.public_url) : undefined
  const fileNameRaw =
    typeof rec.file_name === 'string'
      ? rec.file_name
      : typeof rec.original_filename === 'string'
        ? rec.original_filename
        : ''
  return {
    assetId: String(assetRec.asset_id ?? assetRec.assetId ?? asset.id ?? '').trim(),
    url: pickDisplayUrl(v),
    lan_url: v.lan_url,
    tailscale_url: v.tailscale_url,
    public_url: publicUrl,
    access_hint: v.access_hint,
    preview_available: previewOk,
    download_mode: v.download_mode,
    previewAssetId: resolveBackendPreviewAssetId(asset, v),
    fileName: fileNameRaw.trim(),
  }
}
// v0.6 对齐：K 节 不通过 file_role 推断可预览性，以 preview_available 为准
const referenceDisplayList = computed(() => {
  const refs = scopedBackendAssets.value.filter((a) => backendAssetKind(a) === 'reference')
  const out: ReturnType<typeof refDisplayItem>[] = []
  refs.forEach((a) => {
    const vers = backendAssetVersions(a)
    vers.forEach((v) => out.push(refDisplayItem(v, a)))
  })
  return out
})

onMounted(() => bindBackendAssetsObserver())
onBeforeUnmount(() => disconnectBackendAssetsObserver())

watch(() => task.value?.id, (id) => {
  disconnectBackendAssetsObserver()
  if (id) bindBackendAssetsObserver()
})

const currentReferenceRefs = computed((): ReferenceFileRef[] => {
  if (!batchUi.value) return task.value.referenceFileRefs ?? []
  return referenceRefsForObject(task.value, assetObjectSel.value)
})

type RefPaneEntry =
  | { kind: 'legacy'; url: string; ref: ReferenceFileRef }
  | { kind: 'api'; item: ReturnType<typeof refDisplayItem> }

/** 后端 current reference 优先；legacy reference_file_refs 只做历史兼容兜底。 */
const referencePaneEntries = computed((): RefPaneEntry[] => {
  const out: RefPaneEntry[] = []
  const seen = new Set<string>()
  const pushEntry = (entry: RefPaneEntry, ref: ReferenceFileRef) => {
    const key = referenceFileRefDedupeKey(ref, out.length)
    if (seen.has(key)) return
    seen.add(key)
    out.push(entry)
  }
  for (const item of referenceDisplayList.value) {
    pushEntry(
      { kind: 'api', item },
      { asset_id: item.assetId || undefined, download_url: item.url, filename: item.fileName },
    )
  }
  if (out.length === 0) {
    for (const refObj of currentReferenceRefs.value) {
      const u = (typeof refObj.download_url === 'string' ? refObj.download_url : '').trim()
      if (u) pushEntry({ kind: 'legacy', url: u, ref: refObj }, refObj)
    }
  }
  return out
})

const referenceThumbItems = computed((): AssetThumbItem[] =>
  referencePaneEntries.value.map((entry, index) => {
    if (entry.kind === 'legacy') {
      return {
        key: `leg-${index}-${entry.url}`,
        src: entry.url,
        alt: `参考图 ${index + 1}`,
        label: entry.ref.filename?.trim() || `参考图 ${index + 1}`,
      }
    }
    return {
      key: `api-${index}-${entry.item.url || String(index)}`,
      src: entry.item.url,
      previewAssetId: entry.item.preview_available ? entry.item.previewAssetId : '',
      alt: `参考图 ${index + 1}`,
      label: entry.item.fileName?.trim() || `参考图 ${index + 1}`,
      unavailable: !entry.item.preview_available && !entry.item.url,
    }
  }),
)

/** 是否展示左侧参考轨（与原先参考区块 v-if 条件一致） */
const showReferencePane = computed(
  () =>
    referenceDisplayList.value.length > 0 ||
    currentReferenceRefs.value.length > 0 ||
    isPurchase.value ||
    batchUi.value,
)

/** 采购单列；无参考时右栏通栏；否则左参考 + 右设计稿（对齐审核工作台分栏） */
const designAssetLayoutClass = computed(() => {
  if (isPurchase.value) return ['design-asset-main', 'design-asset-main--stack']
  if (!showReferencePane.value) {
    return ['design-asset-main', 'design-asset-main--split', 'design-asset-main--drafts-only']
  }
  return ['design-asset-main', 'design-asset-main--split']
})

/**
 * 版本时间线（方案 A）：按「资产根（asset_no）」独立成组。
 * - 交付（delivery）与 设计原稿（source）两类根同级并列，各自有 V1/V2/V3 序列；
 * - 参考图在左参考轨，不重复进时间线；
 * - preview/design_thumb 系统派生物不进时间线（上游 normalizer 已过滤）。
 */
function isTimelineEligibleKind(kind: string | undefined): boolean {
  const k = (kind ?? '').trim().toLowerCase()
  return k === 'delivery' || k === 'source'
}

const scopedAssetVersions = computed(() => {
  const all = task.value.assetVersions.filter((v) => isTimelineEligibleKind(v.assetKind))
  if (!batchUi.value) return all
  return all.filter((v) =>
    assetVersionMatchesActiveSku(v, assetObjectSel.value, task.value),
  )
})

const sharedAssetVersions = computed(() => {
  if (!batchUi.value) return []
  return task.value.assetVersions.filter(
    (v) => isTimelineEligibleKind(v.assetKind) && assetVersionIsSharedForBatch(v, task.value),
  )
})

type AssetRootGroup = {
  assetNo: string
  assetKind: string
  kindLabel: string
  versions: TaskAssetVersion[]
}

function assetKindLabel(kind: string): string {
  if (kind === 'delivery') return '交付'
  if (kind === 'source') return isCustomizationTask.value ? '定制原稿' : '设计原稿'
  return kind || '其他'
}

/** 将 scopedAssetVersions 按 assetNo 分组，保持在原列表中的首次出现顺序。 */
const scopedAssetVersionGroups = computed((): AssetRootGroup[] => {
  const list = scopedAssetVersions.value
  const byKey = new Map<string, AssetRootGroup>()
  const order: string[] = []
  for (const v of list) {
    const key = v.assetNo?.trim() || v.assetRootId?.trim() || `__orphan_${v.id}`
    if (!byKey.has(key)) {
      const kind = (v.assetKind ?? '').toLowerCase()
      byKey.set(key, {
        assetNo: v.assetNo?.trim() || key,
        assetKind: kind,
        kindLabel: assetKindLabel(kind),
        versions: [],
      })
      order.push(key)
    }
    byKey.get(key)!.versions.push(v)
  }
  return order.map((k) => byKey.get(k)!)
})

/** 活动版本在 scopedAssetVersions 扁平列表中的下标（button 渲染在 group 里，需要回查） */
function flatIndexOfVersion(v: TaskAssetVersion): number {
  return scopedAssetVersions.value.findIndex((x) => x.id === v.id)
}

function isVersionUnavailable(v: TaskAssetVersion): boolean {
  return isTaskAssetVersionUnavailable(v)
}

/**
 * 是否为「审核替换」版本：
 * 同一资产根（同 asset_no）内 `version_no > 1` 即代表在 V1 基础上的重新提交。
 * 旧逻辑用 `type === 'final'` 误判了定稿交付稿，这里以根内版本号为准。
 */
function isAuditReplacementVersion(v: TaskAssetVersion): boolean {
  const n = v.rootVersionNo
  if (typeof n === 'number') return n > 1
  // 兜底：无根内版本号时仅 revision 视为替换；'final' 不再等同于替换
  return v.type === 'revision'
}

function activateVersion(nextIdx: number) {
  const next = scopedAssetVersions.value[nextIdx]
  if (!next || isVersionUnavailable(next)) return
  hasManualActiveVersionSelection.value = true
  activeVersionIdx.value = nextIdx
  activeFileIdx.value = 0
}

function selectPreferredActiveVersion(versions = scopedAssetVersions.value) {
  const preferredIdx = preferredTaskAssetVersionIndex(versions, isVersionUnavailable)
  activeVersionIdx.value = preferredIdx >= 0 ? preferredIdx : 0
  activeFileIdx.value = 0
}

watch(
  () => [task.value?.id, productIdxCtx?.productIndex.value] as const,
  () => {
    hasManualActiveVersionSelection.value = false
    selectPreferredActiveVersion()
  },
  { immediate: true },
)

watch(
  () => scopedAssetVersions.value,
  (versions) => {
    if (!versions.length) {
      activeVersionIdx.value = 0
      activeFileIdx.value = 0
      return
    }
    if (!hasManualActiveVersionSelection.value || activeVersionIdx.value >= versions.length) {
      selectPreferredActiveVersion(versions)
      return
    }
    const current = versions[activeVersionIdx.value]
    if (current && !isVersionUnavailable(current)) return
    selectPreferredActiveVersion(versions)
  },
  { immediate: true },
)

const activeVersion = computed((): TaskAssetVersion | null => {
  const versions = scopedAssetVersions.value
  if (!versions.length) return null
  return versions[activeVersionIdx.value] ?? versions[versions.length - 1]
})

const activeNonPreviewItem = computed(() => {
  const v = activeVersion.value
  if (!v?.nonPreviewFiles?.length) return null
  const idx = activeFileIdx.value - v.fileRefs.length
  if (idx < 0 || idx >= v.nonPreviewFiles.length) return null
  return v.nonPreviewFiles[idx] ?? null
})

const activeVersionThumbItems = computed((): AssetThumbItem[] => {
  const version = activeVersion.value
  if (!version) return []
  const previews = (version.fileRefs ?? [])
    .filter((src) => String(src ?? '').trim().length > 0)
    .map((src, index) => ({
      key: `preview-${index}`,
      src,
      alt: `版本图 ${index + 1}`,
      label: `图 ${index + 1}`,
    }))
  const sources = (version.nonPreviewFiles ?? []).map((item, index) => ({
    key: `source-${index}`,
    alt: item.label || `源文件 ${index + 1}`,
    label: item.label || `源文件 ${index + 1}`,
    downloadUrl: item.url || '',
    unavailable: true,
  }))
  return [...previews, ...sources]
})

function versionPreviewItems(version: TaskAssetVersion | null | undefined): ImagePreviewLightboxItem[] {
  return (version?.fileRefs ?? [])
    .map((src, index) => {
      const url = String(src ?? '').trim()
      const title = `版本图 ${index + 1}`
      return url ? { src: url, title, alt: title, downloadUrl: url } : null
    })
    .filter((item) => item != null) as ImagePreviewLightboxItem[]
}

function openSharedVersionPreview(version: TaskAssetVersion) {
  const items = versionPreviewItems(version)
  if (!items.length) return
  openLightbox(items[0].src, {
    title: items[0].title,
    items,
    index: 0,
  })
}

function onActiveVersionThumbSelect(key: string) {
  if (key.startsWith('preview-')) {
    const next = Number(key.slice('preview-'.length))
    if (Number.isFinite(next)) {
      activeFileIdx.value = Math.max(0, next)
    }
    return
  }
  if (!key.startsWith('source-')) return
  const sourceIdx = Number(key.slice('source-'.length))
  if (!Number.isFinite(sourceIdx)) return
  const base = activeVersion.value?.fileRefs?.length ?? 0
  activeFileIdx.value = base + Math.max(0, sourceIdx)
}

const canSubmitDesign = computed(() => canSubmitAudit(task.value))
const canSubmitFromDesignPanel = computed(() => {
  if (!canSubmitDesign.value) return false
  if (isCustomizationTask.value) {
    return (
      can('task.customization.submit') ||
      can('customization.submit') ||
      can('design.submit')
    )
  }
  return can('design.submit')
})

const designAssetStatusRow = computed(() => {
  const rt = retouchDesignAssetStatusDisplay(task.value)
  if (rt) return rt
  const customizationLabel = getCustomizationDetailStatusLabel(task.value)
  if (customizationLabel) {
    return { text: customizationLabel, dotClass: 'dot-blue' }
  }
  if (!task.value.designSubStatus) return null
  const s = task.value.designSubStatus
  if (s === 'FINALIZED' || s === 'APPROVED') return { text: getDesignSubStatusLabel(s), dotClass: 'dot-green' }
  if (s === 'REJECTED') return { text: getDesignSubStatusLabel(s), dotClass: 'dot-red' }
  if (s === 'IN_PROGRESS' || s === 'PENDING_AUDIT') return { text: getDesignSubStatusLabel(s), dotClass: 'dot-blue' }
  return { text: getDesignSubStatusLabel(s), dotClass: 'dot-grey' }
})

function formatDate(iso: string): string {
  return formatDateTimeBeijingOffsetAware(iso)
}

async function onDeliveryPanelSuccess() {
  await loadBackendAssets()
  const tid = task.value.id
  await tasksStore.loadTaskById(tid)
  hasManualActiveVersionSelection.value = false
  selectPreferredActiveVersion()
}
</script>

<style scoped>
.block-title { font-size: 0.875rem; font-weight: 600; color: rgb(var(--yb-text-deep)); margin: 0; }
.status-inline { display: flex; align-items: center; gap: 0.375rem; font-size: 0.75rem; color: rgb(var(--yb-text-muted)); }
.status-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.dot-green { color: rgb(var(--yb-success-emerald)); }
.dot-blue { color: rgb(var(--yb-brand)); }
.dot-red { color: rgb(var(--yb-danger)); }
.dot-grey { color: rgb(var(--yb-text-faint)); }
.info-row-simple {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: baseline;
  gap: 0.5rem 1.5rem;
  font-size: 0.8125rem;
  margin-bottom: 0.375rem;
}
.row-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--yb-text-muted-strong));
  min-width: 4rem;
}
.customization-claim-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 0.5rem 1.5rem;
  font-size: 0.8125rem;
  margin-bottom: 0.5rem;
}
.customization-claim-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.375rem;
}
.outsource-flag {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  background: rgb(var(--yb-surface-slate));
  border: 1px solid rgb(var(--yb-border-slate));
  color: rgb(var(--yb-text-slate));
  border-radius: 9999px;
  font-size: 0.6875rem;
  margin-bottom: 0.5rem;
}
.design-asset-main {
  width: 100%;
  min-width: 0;
  margin-top: 0.5rem;
}
.design-asset-main--stack {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
/* 双栏：参考侧略宽，与设计稿列视觉平衡 */
.design-asset-main--split {
  display: grid;
  grid-template-columns: minmax(0, 1.18fr) minmax(0, 1fr);
  gap: 1rem;
  align-items: stretch;
}
.design-asset-main--split.design-asset-main--drafts-only {
  grid-template-columns: 1fr;
}
.design-asset-pane--refs,
.design-asset-pane--drafts {
  border-radius: 0.625rem;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  padding: 0.75rem;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.design-asset-pane--refs .ref-rail-section,
.design-asset-pane--drafts > .asset-section:first-of-type {
  margin-top: 0;
}
.ref-section-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.375rem;
}
.ref-section-label-row .section-label {
  margin-bottom: 0;
}
.ref-pane-hint {
  font-size: 0.6875rem;
  font-weight: 500;
  color: rgb(var(--yb-text-muted-strong));
  white-space: nowrap;
}
.design-asset-pane--drafts .version-preview {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  margin-top: 0.25rem;
  gap: 0.25rem;
}
@media (max-width: 1023px) {
  .design-asset-main--split:not(.design-asset-main--drafts-only) {
    grid-template-columns: 1fr;
  }
}
.object-switch-bar {
  margin-top: 0.5rem;
  padding: 0.5rem 0.625rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
}
.object-switch-label {
  display: block;
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-muted-strong));
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 0.375rem;
}
.object-switch-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
.object-switch-tab {
  padding: 0.25rem 0.55rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface));
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(var(--yb-text-soft));
  cursor: pointer;
}
.object-switch-tab-active {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}
.asset-section { margin-top: 0.75rem; }
.shared-asset-note {
  margin: 0 0 0.375rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}
.shared-asset-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-bottom: 0.5rem;
}
.shared-asset-chip {
  border: 1px solid rgb(var(--yb-brand-subtle));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  border-radius: 999px;
  font-size: 0.6875rem;
  padding: 0.18rem 0.5rem;
}
.section-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-soft));
  text-transform: uppercase;
  display: block;
  margin-bottom: 0.375rem;
}
.submit-error { font-size: 0.75rem; color: rgb(var(--yb-danger)); }
.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
}
.block-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 0.375rem;
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-placeholder));
  font-size: 0.75rem;
  flex-shrink: 0;
}
.version-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.375rem; }
.version-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.375rem;
  min-width: 0;
}
.version-group-label {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
  font-size: 0.6875rem;
  line-height: 1;
}
.version-group-kind {
  padding: 0.15rem 0.4rem;
  border-radius: 0.25rem;
  font-weight: 600;
  border: 1px solid transparent;
}
.version-group-kind.kind-delivery {
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  border-color: rgb(var(--yb-brand-border));
}
.version-group-kind.kind-source {
  background: rgb(var(--yb-warning-badge-soft));
  color: rgb(var(--yb-warning-badge-text));
  border-color: rgb(var(--yb-warning-border-soft));
}
.version-group-no { color: rgb(var(--yb-text-placeholder)); font-weight: 500; }
.version-strip { display: flex; gap: 0.375rem; flex-wrap: wrap; min-width: 0; }
.version-strip-scroll {
  flex-wrap: nowrap;
  overflow-x: auto;
  padding-bottom: 0.25rem;
  -webkit-overflow-scrolling: touch;
}
.version-btn {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.35rem 0.6rem;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface-subtle));
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-soft));
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  flex-shrink: 0;
}
.version-btn.version-active {
  border-color: rgb(var(--yb-text-deep));
  color: rgb(var(--yb-text-navy));
  background: rgb(var(--yb-surface-slate));
}
.version-btn:disabled {
  cursor: not-allowed;
}
.version-btn.version-disabled {
  border-style: dashed;
  color: rgb(var(--yb-text-placeholder));
  background: rgb(var(--yb-surface-subtle));
}
.version-file-count {
  font-size: 0.5625rem;
  font-weight: 400;
  background: rgb(var(--yb-border-slate));
  color: rgb(var(--yb-text-soft));
  border-radius: 9999px;
  padding: 0 0.3rem;
}
.version-replace-tag {
  font-size: 0.5625rem;
  line-height: 1;
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 9999px;
  padding: 0.1rem 0.32rem;
}
.version-unavailable-tag {
  font-size: 0.5625rem;
  line-height: 1;
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
  border: 1px solid rgb(var(--yb-danger-border));
  border-radius: 9999px;
  padding: 0.1rem 0.32rem;
}
.nonpreview-panel {
  width: 100%;
  min-height: 120px;
  border-radius: 4px;
  border: 1px dashed rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface-subtle));
  padding: 1rem;
  text-align: center;
}
.nonpreview-name {
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(var(--yb-text-slate));
  margin: 0 0 0.25rem;
  word-break: break-all;
}
.nonpreview-hint { font-size: 0.75rem; color: rgb(var(--yb-text-placeholder)); margin: 0 0 0.5rem; }
.nonpreview-dl {
  margin-top: 0.5rem;
}
.version-preview-dl {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid rgb(var(--yb-border-slate));
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.version-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.375rem;
  font-size: 0.6875rem;
  color: rgb(var(--yb-text-placeholder));
}
.meta-item { color: rgb(var(--yb-text-muted-strong)); }
.meta-item--replace { color: rgb(var(--yb-brand-strong)); font-weight: 600; }
.meta-sep { color: rgb(var(--yb-text-disabled)); }
.empty-assets { font-size: 0.75rem; color: rgb(var(--yb-text-faint)); padding: 0.5rem 0; }

/* Task detail light skin override. Style-only: keep data flow, uploads, and audit logic unchanged. */
section.detail-block.detail-block--asset.detail-block--asset {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
  color: rgb(var(--yb-text));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.block-title,
.info-row-simple,
.empty-assets {
  color: rgb(var(--yb-text));
}

.status-inline,
.row-label,
.section-label,
.ref-pane-hint,
.shared-asset-note {
  color: rgb(var(--yb-text-muted));
}

.block-icon {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-body));
}

.outsource-flag,
.object-switch-bar,
.design-asset-pane--refs,
.design-asset-pane--drafts,
.nonpreview-panel {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
  color: rgb(var(--yb-text));
  box-shadow: none;
}

.detail-block--result :deep(.design-asset-pane--drafts > .asset-section:first-of-type) {
  min-height: 0;
}

.design-asset-pane--drafts > .asset-section:first-of-type {
  min-height: 16.25rem;
  border-radius: 0.75rem;
  padding: 0.85rem;
  background: rgb(var(--yb-surface-soft));
  border: 1px solid rgb(var(--yb-border));
}

.detail-block--result :deep(.design-asset-result-manuscript),
.detail-block--result :deep(.design-asset-pane--refs),
.detail-block--result :deep(.design-asset-pane--timeline) {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
  color: rgb(var(--yb-text));
  box-shadow: none;
}

.detail-block--result :deep(.manuscript-title),
.detail-block--result :deep(.manuscript-card-label),
.detail-block--result :deep(.nonpreview-name) {
  color: rgb(var(--yb-text));
}

.detail-block--result :deep(.manuscript-meta),
.detail-block--result :deep(.manuscript-empty),
.detail-block--result :deep(.timeline-empty),
.detail-block--result :deep(.nonpreview-hint) {
  color: rgb(var(--yb-text-muted));
}

.detail-block--result :deep(.manuscript-card) {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
}

.detail-block--result :deep(.manuscript-card-visual) {
  background: rgb(var(--yb-surface-muted));
}

.detail-block--result :deep(.version-group) {
  border-radius: 0.625rem;
  padding: 0.35rem 0.45rem;
  background: rgb(var(--yb-surface-soft));
  border: 1px solid rgb(var(--yb-border));
}

.detail-block--result :deep(.version-btn) {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border-strong));
  color: rgb(var(--yb-text-body));
}

.detail-block--result :deep(.version-btn.version-active) {
  color: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
}

.version-header {
  border-bottom: 1px solid rgb(var(--yb-border));
  padding-bottom: 0.45rem;
}

.version-group {
  border-radius: 0.625rem;
  padding: 0.35rem 0.45rem;
  background: rgb(var(--yb-surface-soft));
  border: 1px solid rgb(var(--yb-border));
}

.version-group-kind.kind-delivery {
  background: rgb(var(--yb-brand-soft));
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand-strong));
}

.version-group-kind.kind-source {
  background: rgb(var(--yb-pink-soft));
  border-color: rgb(var(--yb-pink-border));
  color: rgb(var(--yb-pink-text));
}

.version-group-no,
.meta-item,
.nonpreview-hint {
  color: rgb(var(--yb-text-muted));
}

.version-btn {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border-strong));
  color: rgb(var(--yb-text-body));
}

.version-btn:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand-soft));
}

.version-btn.version-active {
  color: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  box-shadow: none;
}

.version-btn.version-disabled {
  color: rgb(var(--yb-text-faint));
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.version-file-count,
.version-replace-tag,
.version-unavailable-tag {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.version-preview {
  border-radius: 0.75rem;
  padding: 0.75rem;
  background: rgb(var(--yb-surface-soft));
  border: 1px solid rgb(var(--yb-border));
}

.version-preview-dl {
  border-top-color: rgb(var(--yb-border));
}

.nonpreview-name {
  color: rgb(var(--yb-text));
}

.meta-item--replace {
  color: rgb(var(--yb-brand));
}

.meta-sep {
  color: rgb(var(--yb-border-strong));
}

:deep(.thumb-btn) {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
  box-shadow: none;
}

:deep(.thumb-btn:hover) {
  border-color: rgb(var(--yb-brand-border-strong));
}

:deep(.thumb-placeholder),
:deep(.thumb-empty),
:deep(.apm-placeholder),
:deep(.apm-empty) {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-muted));
}

:deep(.asset-dl-link--button) {
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand));
  border-color: rgb(var(--yb-brand));
  box-shadow: none;
}

:deep(.asset-dl-link--button:hover) {
  background: rgb(var(--yb-brand-strong));
  border-color: rgb(var(--yb-brand-strong));
}
</style>
