<template>
  <section v-if="!isPurchase" class="detail-block h-full flex flex-col rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-sm p-6">
    <div class="audit-head">
      <div class="audit-title-line">
        <span class="block-icon">A</span>
        <h3 class="block-title">{{ blockTitle }}</h3>
        <span v-if="task.auditSubStatus" class="status-inline status-inline--inline">
          <span class="status-dot" :class="auditStatusDotClass" />
          {{ getAuditSubStatusLabel(task.auditSubStatus) }}
        </span>
      </div>
      <div
        v-if="
          hasTaskScopeAccess &&
          isCurrentHandler &&
          can(['task.audit.review', 'task.audit.claim', 'task.audit.takeover']) &&
          canAuditTask &&
          (showAuditA || showAuditB)
        "
        class="audit-actions-row"
      >
        <BaseButton
          size="sm"
          variant="primary"
          :disabled="auditActionBusy"
          :loading="auditActionBusy"
          @click="passAudit"
        >
          审核通过
        </BaseButton>
        <BaseButton
          size="sm"
          variant="danger"
          :disabled="auditActionBusy"
          @click="openRejectDialog"
        >
          审核打回
        </BaseButton>
        <BaseButton
          v-if="can(['task.audit.review', 'task.audit.claim', 'task.audit.takeover']) && showAuditA"
          size="sm"
          variant="secondary"
          :disabled="auditActionBusy"
          :loading="auditActionBusy"
          @click="transferToB"
        >
          转交复核
        </BaseButton>
      </div>
    </div>

    <div class="flag-stack">
      <div v-if="task.needOutsource" class="outsource-flag">
        已标记外协意图（need_outsource）
      </div>
      <div v-if="showCustomizationJobCard" class="customization-flag">
        已进入定制链路
        <span v-if="task.customizationSourceType" class="flag-meta">
          {{ task.customizationSourceType }}
        </span>
      </div>
    </div>

    <div v-if="showCustomizationJobCard" ref="customizationJobCardRef" class="customization-job-card">
      <div class="customization-job-head">
        <div>
          <p class="customization-job-title">当前定制任务</p>
          <p class="customization-job-hint">
            任务详情侧展示该任务最新 customization job 快照；打回将回到美工处理。
          </p>
        </div>
        <div class="customization-review-actions">
          <BaseButton
            size="sm"
            variant="secondary"
            :disabled="customizationJobLoading"
            @click="loadLatestCustomizationJob"
          >
            {{ customizationJobLoading ? '加载中…' : '刷新定制任务' }}
          </BaseButton>
        </div>
      </div>

      <p v-if="customizationJobError" class="block-error customization-job-error">
        {{ customizationJobError }}
      </p>
      <template v-else-if="latestCustomizationJob">
        <dl class="customization-review-grid">
          <div class="info-pair">
            <dt>Job ID</dt>
            <dd class="mono">{{ dash(latestCustomizationJob.id) }}</dd>
          </div>
          <div class="info-pair">
            <dt>当前状态</dt>
            <dd>
              <span class="job-status-pill" :class="jobStatusToneClass(latestCustomizationJob.status)">
                {{ dash(latestCustomizationJob.status) }}
              </span>
            </dd>
          </div>
          <div class="info-pair">
            <dt>来源业务线</dt>
            <dd>{{ workflowLaneText(latestCustomizationJob.workflow_lane) }}</dd>
          </div>
          <div class="info-pair">
            <dt>来源部门</dt>
            <dd>{{ dash(latestCustomizationJob.source_department) }}</dd>
          </div>
          <div class="info-pair">
            <dt>审核决策</dt>
            <dd>{{ dash(latestCustomizationJob.customization_review_decision) }}</dd>
          </div>
          <div class="info-pair">
            <dt>价格身份</dt>
            <dd>{{ pricingIdentityText(latestCustomizationJob) }}</dd>
          </div>
        </dl>
        <div class="pricing-stack">
          <div class="pricing-sect pricing-sect--review">
            <h3 class="pricing-sect-title">审核阶段参考</h3>
            <p class="pricing-sect-note">判级与参考定价基础；不等于执行后最终结算价。</p>
            <dl class="customization-review-grid pricing-sect-grid">
              <div class="info-pair">
                <dt>定制等级</dt>
                <dd>{{ customizationLevelText(latestCustomizationJob) }}</dd>
              </div>
              <div class="info-pair">
                <dt>审核参考单价</dt>
                <dd>{{ priceDisplay(latestCustomizationJob.review_reference_unit_price) }}</dd>
              </div>
              <div class="info-pair">
                <dt>审核参考系数</dt>
                <dd>{{ factorDisplay(latestCustomizationJob.review_reference_weight_factor) }}</dd>
              </div>
            </dl>
          </div>
          <div class="pricing-sect pricing-sect--exec">
            <h3 class="pricing-sect-title">执行阶段结算快照</h3>
            <p class="pricing-sect-note">进入执行后冻结的结算口径；含结算用工类型。</p>
            <dl class="customization-review-grid pricing-sect-grid">
              <div class="info-pair">
                <dt>结算用工类型</dt>
                <dd>{{ employmentTypeLabelCn(String(latestCustomizationJob.pricing_worker_type ?? '')) }}</dd>
              </div>
              <div class="info-pair">
                <dt>执行冻结单价</dt>
                <dd>{{ priceDisplay(latestCustomizationJob.unit_price) }}</dd>
              </div>
              <div class="info-pair">
                <dt>执行冻结系数</dt>
                <dd>{{ factorDisplay(latestCustomizationJob.weight_factor) }}</dd>
              </div>
            </dl>
          </div>
        </div>
        <dl class="customization-review-grid">
          <div class="info-pair">
            <dt>替换前资产</dt>
            <dd class="mono">{{ dash(latestCustomizationJob.previous_asset_id) }}</dd>
          </div>
          <div class="info-pair">
            <dt>当前有效资产</dt>
            <dd class="mono">{{ dash(latestCustomizationJob.current_asset_id) }}</dd>
          </div>
          <div class="info-pair">
            <dt>替换人</dt>
            <dd>{{ replacementActorText(latestCustomizationJob) }}</dd>
          </div>
          <div class="info-pair">
            <dt>最新更新时间</dt>
            <dd>{{ formatTime(latestCustomizationJob.updated_at ?? latestCustomizationJob.created_at) }}</dd>
          </div>
        </dl>
      </template>
      <BaseEmptyState
        v-else
        title="暂无定制 Job"
        :description="customizationJobEmptyHint"
      />
    </div>

    <div
      v-if="showCustomizationReviewEntry"
      class="customization-review-card"
    >
      <div class="customization-review-head">
        <div>
          <p class="customization-review-title">定制审核</p>
          <p class="customization-review-hint">
            当前任务处于定制流程阶段，可提交定制审核决策。
          </p>
        </div>
        <div class="customization-review-actions">
          <BaseButton size="sm" variant="primary" @click="openCustomizationDialog">
            提交定制审核
          </BaseButton>
        </div>
      </div>
      <dl class="customization-review-grid">
        <div class="info-pair">
          <dt>定制来源</dt>
          <dd>{{ dash(task.customizationSourceType) }}</dd>
        </div>
        <div class="info-pair">
          <dt>美工处理人</dt>
          <dd>{{ dash(task.lastCustomizationOperatorId) }}</dd>
        </div>
        <div class="info-pair">
          <dt>仓库驳回分类</dt>
          <dd>{{ dash(task.warehouseRejectCategory) }}</dd>
        </div>
        <div class="info-pair">
          <dt>仓库驳回原因</dt>
          <dd>{{ dash(task.warehouseRejectReason) }}</dd>
        </div>
      </dl>
    </div>

    <p v-if="blockSuccess" class="block-success">{{ blockSuccess }}</p>
    <p v-if="blockError" class="block-error">{{ blockError }}</p>

    <div
      v-if="
        hasTaskScopeAccess &&
        isCurrentHandler &&
        can(['task.audit.review', 'task.audit.claim', 'task.audit.takeover'])
      "
      class="audit-reference-box"
    >
      <p class="customization-review-title">审核参考图区</p>
      <p class="customization-review-hint">
        仅绑定 owner_module_key=audit，使用 replace 策略，不会创建或覆盖设计资产版本链。
      </p>
      <ReferenceUploadPanel
        v-model="auditReferenceFileRefs"
        :task-id="task.id"
        owner-module-key="audit"
        upload-policy="replace"
      />
    </div>

    <!-- 打回原因输入（内联） -->
    <div v-if="showRejectInput" class="reject-box">
      <label class="field-label">打回原因 <span class="required">*</span></label>
      <textarea
        v-model="rejectReason"
        class="reason-textarea"
        placeholder="请填写打回原因（必填）..."
        rows="3"
        maxlength="300"
      />
      <div class="reject-actions">
        <BaseButton
          size="sm"
          variant="ghost"
          :disabled="auditActionBusy"
          @click="showRejectInput = false"
        >取消</BaseButton>
        <BaseButton
          size="sm"
          variant="danger"
          :disabled="!rejectReason.trim() || auditActionBusy"
          :loading="auditActionBusy"
          @click="submitReject"
        >
          确认打回
        </BaseButton>
      </div>
    </div>

    <CustomizationReviewForm
      v-model="customizationDialogOpen"
      mode="initial"
      :task-id="task.id"
      :default-reviewer-id="currentUser?.id ?? null"
      :can-upload-source="can('task.customization.review.asset_upload')"
      :target-sku-code="task.sku ?? undefined"
      :loading="customizationSubmitting"
      :error="customizationError"
      @submit="submitCustomizationReview"
    />
  </section>
</template>

<script setup lang="ts">
import { ref, computed, inject, watch, nextTick, onBeforeUnmount } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { getAuditSubStatusLabel } from '@/domain/enums/task-status'
import { canAudit, canTransferToAuditB } from '@/domain/task-actions'
import { getTaskActionAvailability } from '@/domain/task-action-availability'
import {
  denyCodeText,
  formatTaskActionDenyMessage,
  isCustomizationStageMismatch,
} from '@/domain/task-action-deny'
import { usePermission } from '@/composables/usePermission'
import { useSubmitGuard } from '@/composables/useSubmitGuard'
import { useTasksStore } from '@/stores/tasks'
import type { CustomizationJobRaw } from '@/services/apiTypes'
import { listCustomizationJobs } from '@/services/api/customizationApi'
import { formatDateBeijing } from '@/utils/date'
import { employmentTypeLabelCn } from '@/domain/mappers/read-model-labels-cn'
import { userAccountDisplay } from '@/domain/user-display'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import CustomizationReviewForm, {
  type CustomizationReviewPayload,
} from '@/components/customization/CustomizationReviewForm.vue'

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[AuditOutsourceBlock] 必须在 TaskDetailView 内使用')

const task = computed(() => injected.value!)
const { can, currentUser, canAccessTask } = usePermission()
const tasksStore = useTasksStore()

const isPurchase = computed(() => task.value.businessType === 'PURCHASE_TASK')
const isCustomizationTask = computed(
  () =>
    task.value.workflowLane === 'customization' ||
    task.value.businessLane === 'customization' ||
    task.value.customizationRequired === true,
)
const blockTitle = computed(() => isCustomizationTask.value ? '定制审核' : '审核与定制')
const canAuditTask = computed(() => canAudit(task.value))
const canTransfer = computed(() => canTransferToAuditB(task.value))
const actionAvailability = computed(() => getTaskActionAvailability(task.value))
const showAuditA = computed(() => actionAvailability.value.canShowAuditA)
const showAuditB = computed(() => actionAvailability.value.canShowAuditB)
const hasTaskScopeAccess = computed(() => canAccessTask(task.value))
const isCurrentHandler = computed(() => {
  const handlerId = String(task.value.currentHandlerId ?? '').trim()
  if (!handlerId) return true
  return String(currentUser.value?.id ?? '').trim() === handlerId
})

const showRejectInput = ref(false)
const rejectReason = ref('')
const blockError = ref('')
const blockSuccess = ref('')
const auditReferenceFileRefs = ref<(Record<string, unknown> | string)[]>([])
const customizationDialogOpen = ref(false)
const { submitting: customizationSubmitting, guard: customizationGuard } = useSubmitGuard()
const customizationError = ref('')
// 审核三连按钮（通过 / 打回 / 转交复核）共用一个 guard，因为它们在 UI 上互斥，
// 任意一个进行中时其他按钮必须同时 disabled，避免绕过某个按钮 :disabled 再触发。
const { submitting: auditActionBusy, guard: auditGuard } = useSubmitGuard()
const customizationJobLoading = ref(false)
const customizationJobError = ref('')
const latestCustomizationJob = ref<CustomizationJobRaw | null>(null)
const customizationJobCardRef = ref<HTMLElement | null>(null)
let customizationJobIo: IntersectionObserver | null = null

function disconnectCustomizationJobIo() {
  customizationJobIo?.disconnect()
  customizationJobIo = null
}

function bindCustomizationJobIo() {
  disconnectCustomizationJobIo()
  if (!showCustomizationJobCard.value) return
  void nextTick(() => {
    const el = customizationJobCardRef.value
    if (!el) return
    customizationJobIo = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && showCustomizationJobCard.value) {
            void loadLatestCustomizationJob()
            disconnectCustomizationJobIo()
          }
        }
      },
      { root: null, rootMargin: '120px', threshold: 0.02 },
    )
    customizationJobIo.observe(el)
  })
}

const auditStatusDotClass = computed(() => {
  const s = task.value.auditSubStatus
  if (s === 'PASSED') return 'dot-green'
  if (s === 'REJECTED') return 'dot-red'
  if (s === 'IN_PROGRESS') return 'dot-blue'
  return 'dot-grey'
})

const showCustomizationReviewEntry = computed(
  () =>
    hasTaskScopeAccess.value &&
    task.value.status === 'PendingCustomizationReview',
)

const showCustomizationJobCard = computed(
  () =>
    hasTaskScopeAccess.value &&
    (
      task.value.customizationRequired === true ||
      task.value.status === 'Outsourcing' ||
      task.value.status === 'PendingCustomizationReview' ||
      task.value.status === 'PendingCustomizationProduction' ||
      task.value.status === 'PendingEffectReview' ||
      task.value.status === 'PendingEffectRevision' ||
      task.value.status === 'PendingProductionTransfer' ||
      task.value.status === 'PendingWarehouseQC' ||
      task.value.status === 'RejectedByWarehouse'
    ),
)

const customizationJobEmptyHint = computed(() => {
  const status = task.value.status
  if (status === 'Outsourcing') {
    return '当前任务处于定制链路，请刷新定制任务列表确认是否已生成 customization job。'
  }
  if (status === 'PendingCustomizationReview') {
    return '当前任务已进入定制流程，请点击“提交定制审核”完成决策。'
  }
  return '当前任务还没有关联的 customization job，请确认任务分道字段与后端建单状态。'
})

watch(
  () => [task.value.id, showCustomizationJobCard.value] as const,
  () => {
    latestCustomizationJob.value = null
    customizationJobError.value = ''
    disconnectCustomizationJobIo()
    if (showCustomizationJobCard.value) {
      bindCustomizationJobIo()
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => disconnectCustomizationJobIo())

/** 根据任务状态推导 approve 的 stage（openapi: A, B） */
function getStageFromTask(t: Task): string {
  if (t.status === 'PendingAuditB' || t.status === 'RejectedByAuditB') return 'B'
  return 'A'
}

function getRejectStageFromTask(t: Task): 'A' | 'B' {
  if (t.status === 'PendingAuditB' || t.status === 'RejectedByAuditB') return 'B'
  return 'A'
}

/** 根据 stage 推导 next_status */
function getNextStatusFromStage(stage: string): string {
  if (stage === 'A') return 'PendingWarehouseReceive'
  return 'PendingWarehouseReceive'
}

async function passAudit() {
  blockError.value = ''
  blockSuccess.value = ''
  const stage = getStageFromTask(task.value)
  const nextStatus = getNextStatusFromStage(stage)
  await auditGuard(async () => {
    try {
      await tasksStore.passAudit(task.value.id, { stage, next_status: nextStatus })
    } catch (e) {
      blockError.value = formatTaskActionDenyMessage(e, '操作失败')
    }
  })
}

function openRejectDialog() {
  rejectReason.value = ''
  blockError.value = ''
  blockSuccess.value = ''
  showRejectInput.value = true
}

async function submitReject() {
  if (!rejectReason.value.trim()) return
  blockError.value = ''
  blockSuccess.value = ''
  await auditGuard(async () => {
    try {
      await tasksStore.rejectAudit(task.value.id, {
        stage: getRejectStageFromTask(task.value),
        comment: rejectReason.value.trim(),
      })
      showRejectInput.value = false
      rejectReason.value = ''
    } catch (e) {
      blockError.value = formatTaskActionDenyMessage(e, '操作失败')
    }
  })
}

async function transferToB() {
  if (!canTransfer.value) return
  blockError.value = ''
  blockSuccess.value = ''
  await auditGuard(async () => {
    try {
      await tasksStore.transferToAuditB(task.value.id)
    } catch (e) {
      blockError.value = formatTaskActionDenyMessage(e, '操作失败')
    }
  })
}

function dash(value: unknown): string {
  const text = String(value ?? '').trim()
  return text || '—'
}

function formatTime(value: unknown): string {
  const text = String(value ?? '').trim()
  if (!text) return '—'
  return formatDateBeijing(text) || text
}

function normalizeJobStatus(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  if (raw.includes('_')) return raw.toLowerCase()
  return raw
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .replace(/[\s-]+/g, '_')
    .toLowerCase()
}

function jobStatusToneClass(value: unknown): string {
  const status = normalizeJobStatus(value)
  if (status.includes('rejected')) return 'job-status-danger'
  if (status.includes('completed')) return 'job-status-success'
  if (status.includes('review')) return 'job-status-warning'
  return 'job-status-default'
}

function customizationLevelText(job: CustomizationJobRaw): string {
  const code = String(job.customization_level_code ?? '').trim()
  const name = String(job.customization_level_name ?? '').trim()
  if (!code && !name) return '—'
  if (!code) return name
  if (!name) return code
  return `${name} (${code})`
}

function pricingIdentityText(job: CustomizationJobRaw): string {
  const employmentType = String(job.employment_type ?? '').trim()
  if (employmentType) return employmentType
  const pricingWorkerType = String(job.pricing_worker_type ?? '').trim()
  return pricingWorkerType || '—'
}

function workflowLaneText(value: unknown): string {
  const lane = String(value ?? '').trim().toLowerCase()
  if (lane === 'customization') return '定制'
  if (lane === 'normal') return '普通'
  return '—'
}

function priceDisplay(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return `¥${value.toFixed(2)}`
  }
  return '—'
}

function factorDisplay(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value.toFixed(2)
  }
  return '—'
}

function replacementActorText(job: CustomizationJobRaw): string {
  return userAccountDisplay(job.replacement_actor_name, job.replacement_actor_username)
}

function openCustomizationDialog() {
  blockError.value = ''
  blockSuccess.value = ''
  customizationError.value = ''
  customizationDialogOpen.value = true
}

async function loadLatestCustomizationJob() {
  if (!showCustomizationJobCard.value) {
    latestCustomizationJob.value = null
    return
  }
  customizationJobLoading.value = true
  customizationJobError.value = ''
  try {
    const body = await listCustomizationJobs({ task_id: task.value.id, page: 1, page_size: 20 })
    const root = (body && typeof body === 'object' ? body : {}) as { data?: unknown }
    const list = Array.isArray(root.data) ? (root.data as CustomizationJobRaw[]) : []
    latestCustomizationJob.value = list[0] ?? null
  } catch (e) {
    customizationJobError.value = formatTaskActionDenyMessage(e, '加载定制任务失败')
    latestCustomizationJob.value = null
  } finally {
    customizationJobLoading.value = false
  }
}

/**
 * 接收 `CustomizationReviewForm` 组装好的 payload，
 * 由本组件负责调用 store + 成功/失败分支文案。
 * 前端不推导 task_status，仅透传后端已验签的字段集。
 *
 * 并发防抖：`customizationGuard` 会吞掉二次点击（submit-guard）；
 * `CustomizationReviewForm` 的确认按钮也绑了 `:disabled+:loading`，双保险。
 *
 * 乐观漂移：若后端回 403 with deny_code=customization_stage_mismatch，
 * 说明该任务已被其他审核员提交过；走「静默成功」分支：关闭弹窗、明确告知
 * 用户并刷新任务详情，而不是抛出原始 403 文案。
 */
async function submitCustomizationReview(payload: CustomizationReviewPayload) {
  customizationError.value = ''
  await customizationGuard(async () => {
    try {
      await tasksStore.submitCustomizationReview(task.value.id, payload)
      customizationDialogOpen.value = false
      const decision = payload.customization_review_decision
      blockSuccess.value =
        decision === 'return_to_designer'
          ? '定制审核已提交，任务已打回美工处理。'
          : '定制审核已提交，任务快照已刷新。'
      await tasksStore.loadTaskById(task.value.id).catch(() => {})
      await loadLatestCustomizationJob()
    } catch (e) {
      if (isCustomizationStageMismatch(e)) {
        customizationDialogOpen.value = false
        blockSuccess.value =
          denyCodeText('customization_stage_mismatch') ?? '该任务已被审核处理过，正在刷新列表…'
        await tasksStore.loadTaskById(task.value.id).catch(() => {})
        await loadLatestCustomizationJob()
        return
      }
      customizationError.value = formatTaskActionDenyMessage(e, '定制审核提交失败')
    }
  })
}
</script>

<style scoped>
.block-success {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-ui-soft));
  border-radius: 6px;
}
.block-error {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
  border-radius: 6px;
}
.audit-head {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 0.35rem;
}
.audit-title-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem 0.6rem;
  min-width: 0;
}
.status-inline--inline {
  margin: 0;
  font-size: 0.75rem;
}
.audit-actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
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
.block-title { font-size: 0.875rem; font-weight: 600; color: rgb(var(--yb-text-deep)); margin: 0; }
.status-inline { display: flex; align-items: center; gap: 0.375rem; font-size: 0.75rem; color: rgb(var(--yb-text-muted)); }
.status-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.dot-green { color: rgb(var(--yb-success-emerald)); }
.dot-blue { color: rgb(var(--yb-brand)); }
.dot-red { color: rgb(var(--yb-danger)); }
.dot-grey { color: rgb(var(--yb-text-faint)); }
.outsource-flag {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  background: rgb(var(--yb-surface-slate));
  border: 1px solid rgb(var(--yb-border-slate));
  color: rgb(var(--yb-text-slate));
  border-radius: 9999px;
  font-size: 0.6875rem;
  margin-bottom: 0.25rem;
}
.flag-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.customization-flag {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.125rem 0.5rem;
  background: rgb(var(--yb-indigo-surface));
  border: 1px solid rgb(var(--yb-border-indigo));
  color: rgb(var(--yb-indigo-text));
  border-radius: 9999px;
  font-size: 0.6875rem;
  margin-bottom: 0.25rem;
}
.flag-meta {
  color: rgb(var(--yb-indigo-accent));
}
.customization-review-card {
  margin-top: 0.25rem;
  padding: 0.875rem;
  border: 1px solid rgb(var(--yb-indigo-soft));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
}
.customization-job-card {
  margin-top: 0.25rem;
  padding: 0.875rem;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
}
.customization-job-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.customization-job-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-deep));
}
.customization-job-hint {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}
.customization-job-error {
  margin-bottom: 0.75rem;
}
.customization-review-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.customization-review-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  justify-content: flex-end;
}
.customization-review-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-deep));
}
.customization-review-hint {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}
.customization-review-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.625rem 1rem;
  margin: 0;
}
.pricing-stack {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0.35rem 0 0.5rem;
}
.pricing-sect-grid {
  margin: 0;
  gap: 0.35rem 0.75rem;
}
.pricing-sect {
  padding: 0.55rem 0.65rem;
  border-radius: 0.6rem;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
}
.pricing-sect--exec {
  background: rgb(var(--yb-purple-soft));
  border-color: rgb(var(--yb-purple-border));
}
.pricing-sect-title {
  margin: 0 0 0.2rem;
  font-size: 0.78rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}
.pricing-sect-note {
  margin: 0 0 0.45rem;
  font-size: 0.68rem;
  line-height: 1.45;
  color: rgb(var(--yb-text-muted-strong));
}
.info-pair dt {
  margin: 0 0 0.125rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}
.info-pair dd {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-navy));
}
.mono {
  font-family: var(--yb-font-data);
}
.job-status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  padding: 0 0.5rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 700;
  border: 1px solid transparent;
}
.job-status-default {
  background: rgb(var(--yb-border-slate));
  color: rgb(var(--yb-text-slate));
}
.job-status-warning {
  background: rgb(var(--yb-warning-badge-soft));
  color: rgb(var(--yb-warning-dark));
}
.job-status-danger {
  background: rgb(var(--yb-danger-soft-hover));
  color: rgb(var(--yb-danger-text));
}
.job-status-success {
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-deep));
}
.reject-box { background: rgb(var(--yb-danger-soft)); border: 1px solid rgb(var(--yb-danger-border)); border-radius: 0.5rem; padding: 0.625rem 0.75rem; }
.field-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--yb-text-muted-strong));
  display: block;
  margin-bottom: 0.375rem;
}
.required { color: rgb(var(--yb-danger)); }
.reason-textarea {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.5rem;
  font-size: 0.8125rem;
  resize: vertical;
  box-sizing: border-box;
  outline: none;
}
.reason-textarea:focus { border-color: rgb(var(--yb-text-placeholder)); }
.reject-actions { display: flex; gap: 0.5rem; margin-top: 0.5rem; justify-content: flex-end; }
</style>
