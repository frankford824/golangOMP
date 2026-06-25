<template>
  <!-- 横向：任务详情顶栏嵌入，收窄不独占全宽 -->
  <NSteps
    v-if="variant === 'horizontal'"
    class="workflow-progress workflow-progress--horizontal workflow-progress--naive"
    :current="naiveCurrent"
    status="process"
    size="small"
    content-placement="right"
    :theme-overrides="stepThemeOverrides"
  >
    <NStep
      v-for="step in steps"
      :key="step.key"
      :title="step.label"
      :description="step.subLabel || ''"
      :status="naiveStepStatus(step)"
    />
  </NSteps>
  <!-- 纵向：原时间线 -->
  <div v-else class="workflow-progress">
    <div
      v-for="(step, i) in steps"
      :key="step.key"
      class="step-row"
      :class="{
        'step-done': step.state === 'done',
        'step-current': step.state === 'current',
        'step-pending': step.state === 'pending',
        'step-skipped': step.state === 'skipped',
      }"
    >
      <div class="step-indicator">
        <div class="step-dot">
          <svg v-if="step.state === 'done'" viewBox="0 0 12 12" fill="none" class="w-3 h-3">
            <path d="M2 6l3 3 5-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <div v-if="i < steps.length - 1" class="step-line" />
      </div>
      <div class="step-content">
        <span class="step-label">{{ step.label }}</span>
        <span v-if="step.subLabel" class="step-sublabel">{{ step.subLabel }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NStep, NSteps } from 'naive-ui'
import type { Task } from '@/domain/types/task'
import {
  getDesignSubStatusLabel,
  getAuditSubStatusLabel,
  getWarehouseSubStatusLabel,
} from '@/domain/enums/task-status'

type StepState = 'done' | 'current' | 'pending' | 'skipped'
type NaiveStepStatus = 'process' | 'finish' | 'error' | 'wait'

interface Step {
  key: string
  label: string
  subLabel?: string
  state: StepState
}

const props = withDefaults(
  defineProps<{
    task: Task
    /** vertical：原侧栏时间线；horizontal：顶栏紧凑步骤 */
    variant?: 'vertical' | 'horizontal'
  }>(),
  { variant: 'vertical' },
)

const isPurchase = computed(() => props.task.businessType === 'PURCHASE_TASK')
const isRetouch = computed(
  () => props.task.businessType === 'RETOUCH_TASK' || props.task.taskType === 'RETOUCH_TASK',
)
const isCustomization = computed(() =>
  props.task.workflowLane === 'customization' ||
  props.task.businessLane === 'customization' ||
  props.task.customizationRequired === true,
)

const steps = computed((): Step[] => {
  const t = props.task

  const mainStatus = t.mainStatus
  const legacyStatus = t.status

  if (isPurchase.value) {
    return [
      {
        key: 'created',
        label: '创建',
        state: mainStatus && mainStatus !== 'DRAFT' ? 'done' : mainStatus === 'DRAFT' ? 'current' : 'pending',
      },
      {
        key: 'warehouse',
        label: '仓库接收',
        subLabel: t.warehouseSubStatus ? getWarehouseSubStatusLabel(t.warehouseSubStatus) : undefined,
        state: resolveStepState(
          ['CREATED', 'INFO_PENDING', 'WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING'],
          ['READY_TO_CLOSE', 'CLOSED'],
          mainStatus,
        ),
      },
      {
        key: 'close',
        label: '结单',
        state: mainStatus === 'CLOSED' ? 'done' : mainStatus === 'READY_TO_CLOSE' ? 'current' : 'pending',
      },
    ]
  }

  if (isCustomization.value) {
    const ls = legacyStatus ?? ''
    const custStatus = ls
    // 定制 lane 的 Legacy status 会映射到 mainStatus=WAREHOUSE_PENDING（含生产/审核），
    // 流程条必须以 task_status 为准，不能用 mainStatus 判断仓库节点是否进行中。
    const customizationWarehouseActiveStatuses = [
      'PendingWarehouseReceive',
      'PendingWarehouseQC',
      'PendingProductionTransfer',
      'RejectedByWarehouse',
    ] as const
    const isCustomizationWarehouseActive = customizationWarehouseActiveStatuses.includes(
      custStatus as (typeof customizationWarehouseActiveStatuses)[number],
    ) || mainStatus === 'WAREHOUSE_PROCESSING'
    const isCustomizationWarehouseDone =
      custStatus === 'PendingClose' ||
      legacyStatus === 'Completed' ||
      legacyStatus === 'Archived' ||
      mainStatus === 'READY_TO_CLOSE' ||
      mainStatus === 'CLOSED'
    const warehouseStepState: StepState = isCustomizationWarehouseDone
      ? 'done'
      : isCustomizationWarehouseActive
        ? 'current'
        : 'pending'

    return [
      {
        key: 'created',
        label: '创建',
        state: mainStatus && mainStatus !== 'DRAFT' ? 'done' : 'current',
      },
      {
        key: 'customization_submit',
        label: '美工提交设计稿',
        subLabel: custStatus === 'PendingCustomizationProduction' ? '待美工提交' : undefined,
        state: custStatus === 'PendingCustomizationProduction' ? 'current' : 'done',
      },
      {
        key: 'cust_review',
        label: '定制审核',
        subLabel: custStatus === 'PendingCustomizationReview' ? '待审核' : undefined,
        state: custStatus === 'PendingCustomizationProduction'
          ? 'pending'
          : custStatus === 'PendingCustomizationReview'
            ? 'current'
            : 'done',
      },
      {
        key: 'warehouse',
        label: '仓库接收',
        subLabel: customizationWarehouseSubLabel(warehouseStepState, t.warehouseSubStatus),
        state: warehouseStepState,
      },
      {
        key: 'close',
        label: '结单',
        state: mainStatus === 'CLOSED' || legacyStatus === 'Completed' || legacyStatus === 'Archived'
          ? 'done'
          : mainStatus === 'READY_TO_CLOSE' || custStatus === 'PendingClose'
            ? 'current'
            : 'pending',
      },
    ]
  }

  if (isRetouch.value) {
    const retouchMod = t.moduleSummaries?.find(
      (m: { module_key: string }) => m.module_key === 'retouch',
    )
    const modState = retouchMod?.state ?? ''

    const SUBMITTED_TASK_STATUSES = [
      'PendingAuditA', 'PendingAuditB', 'PendingWarehouseReceive',
      'Completed', 'Archived',
    ]
    const retouchSubmitted =
      modState === 'submitted' ||
      modState === 'closed' ||
      SUBMITTED_TASK_STATUSES.includes(legacyStatus ?? '')
    const retouchStepState: StepState = retouchSubmitted
      ? 'done'
      : modState === 'in_progress' || legacyStatus === 'InProgress'
        ? 'current'
        : 'pending'

    return [
      {
        key: 'created',
        label: '创建',
        state: 'done' as StepState,
      },
      {
        key: 'retouch',
        label: '精修',
        state: retouchStepState,
      },
      {
        key: 'done',
        label: '完成',
        state: retouchSubmitted ? 'done' : 'pending',
      },
    ]
  }

  const designStepState = resolveNormalDesignStepState(t, mainStatus, legacyStatus)
  const auditStepState = resolveNormalAuditStepState(t, mainStatus, legacyStatus)

  return [
    {
      key: 'created',
      label: '创建',
      state: mainStatus && mainStatus !== 'DRAFT' ? 'done' : 'current',
    },
    {
      key: 'design',
      label: '设计',
      subLabel: normalDesignProgressSubLabel(t, designStepState),
      state: designStepState,
    },
    {
      key: 'audit',
      label: '审核',
      subLabel: t.auditSubStatus ? getAuditSubStatusLabel(t.auditSubStatus) : undefined,
      state:
        t.auditSubStatus === 'NOT_REQUIRED'
          ? 'skipped'
          : auditStepState,
    },
    {
      key: 'warehouse',
      label: '仓库接收',
      subLabel: t.warehouseSubStatus ? getWarehouseSubStatusLabel(t.warehouseSubStatus) : undefined,
      state: resolveStepState(['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING'], ['READY_TO_CLOSE', 'CLOSED'], mainStatus),
    },
    {
      key: 'close',
      label: '结单',
      state: mainStatus === 'CLOSED' ? 'done' : mainStatus === 'READY_TO_CLOSE' ? 'current' : 'pending',
    },
  ]
})

const naiveCurrent = computed(() => {
  const index = steps.value.findIndex((step) => step.state === 'current')
  if (index >= 0) return index + 1
  const lastDoneFromEnd = [...steps.value].reverse().findIndex((step) => step.state === 'done')
  if (lastDoneFromEnd >= 0) return steps.value.length - lastDoneFromEnd
  return 1
})

function naiveStepStatus(step: Step): NaiveStepStatus {
  if (step.state === 'done') return 'finish'
  if (step.state === 'current') return 'process'
  return 'wait'
}

const stepThemeOverrides = {
  stepHeaderFontWeight: '800',
  stepHeaderFontSizeSmall: '13px',
  indicatorSizeSmall: '20px',
  indicatorIconSizeSmall: '13px',
  indicatorIndexFontSizeSmall: '11px',
  indicatorColorFinish: 'rgb(var(--yb-success-bright))',
  indicatorColorProcess: 'rgb(var(--yb-brand))',
  indicatorColorWait: 'rgb(var(--yb-border))',
  indicatorBorderColorFinish: 'rgb(var(--yb-success-border-bright))',
  indicatorBorderColorProcess: 'rgb(var(--yb-brand-border-strong))',
  indicatorBorderColorWait: 'rgb(var(--yb-border-strong))',
  indicatorTextColorFinish: 'rgb(var(--yb-text-inverse))',
  indicatorTextColorProcess: 'rgb(var(--yb-text-inverse))',
  indicatorTextColorWait: 'rgb(var(--yb-text-muted))',
  headerTextColorFinish: 'rgb(var(--yb-success-strong))',
  headerTextColorProcess: 'rgb(var(--yb-brand))',
  headerTextColorWait: 'rgb(var(--yb-text-muted))',
  descriptionTextColorFinish: 'rgb(var(--yb-success))',
  descriptionTextColorProcess: 'rgb(var(--yb-brand-strong))',
  descriptionTextColorWait: 'rgb(var(--yb-text-faint))',
  splitorColorFinish: 'rgb(var(--yb-brand-border-strong))',
  splitorColorProcess: 'rgb(var(--yb-brand-border))',
  splitorColorWait: 'rgb(var(--yb-border-strong))',
}

function resolveStepState(
  activeStatuses: string[],
  doneStatuses: string[],
  mainStatus?: string,
): StepState {
  if (!mainStatus) return 'pending'
  if (doneStatuses.includes(mainStatus)) return 'done'
  if (activeStatuses.includes(mainStatus)) return 'current'
  return 'pending'
}

function resolveNormalDesignStepState(
  task: Task,
  mainStatus?: string,
  legacyStatus?: string,
): StepState {
  if (['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING', 'READY_TO_CLOSE', 'CLOSED'].includes(mainStatus ?? '')) {
    return 'done'
  }
  if (legacyStatus === 'PendingAuditA' || legacyStatus === 'PendingAuditB') {
    return 'done'
  }
  switch (task.designSubStatus) {
    case 'PENDING_AUDIT':
    case 'APPROVED':
    case 'FINALIZED':
      return 'done'
    case 'PENDING_ASSIGN':
    case 'IN_PROGRESS':
    case 'REJECTED':
      return 'current'
    case 'NOT_REQUIRED':
      return 'skipped'
    default:
      return mainStatus === 'INFO_PENDING' ? 'current' : 'pending'
  }
}

function resolveNormalAuditStepState(
  task: Task,
  mainStatus?: string,
  legacyStatus?: string,
): StepState {
  if (['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING', 'READY_TO_CLOSE', 'CLOSED'].includes(mainStatus ?? '')) {
    return 'done'
  }
  if (legacyStatus === 'PendingAuditA' || legacyStatus === 'PendingAuditB') {
    return 'current'
  }
  switch (task.auditSubStatus) {
    case 'PENDING':
    case 'IN_PROGRESS':
      return 'current'
    case 'PASSED':
    case 'TRANSFERRED':
    case 'HANDED_OVER':
      return 'done'
    case 'REJECTED':
      return 'pending'
    case 'NOT_REQUIRED':
      return 'skipped'
    default:
      return 'pending'
  }
}

function normalDesignProgressSubLabel(task: Task, state: StepState): string | undefined {
  switch (task.designSubStatus) {
    case 'PENDING_ASSIGN':
      return '待指派'
    case 'IN_PROGRESS':
      return '设计中'
    case 'REJECTED':
      return '需修改'
    case 'PENDING_AUDIT':
      return '已提交'
    case 'APPROVED':
      return '已通过'
    case 'FINALIZED':
      return '已定稿'
    case 'NOT_REQUIRED':
      return state === 'skipped' ? '无需设计' : undefined
    default:
      return task.designSubStatus ? getDesignSubStatusLabel(task.designSubStatus) : undefined
  }
}

/** 定制任务：仓库未进入流程时不展示 workflow.not_triggered → NOT_REQUIRED 的「无需仓库」。 */
function customizationWarehouseSubLabel(
  warehouseStepState: StepState,
  warehouseSubStatus?: Task['warehouseSubStatus'],
): string | undefined {
  if (warehouseStepState === 'pending') return undefined
  if (!warehouseSubStatus || warehouseSubStatus === 'NOT_REQUIRED') {
    return warehouseStepState === 'current' ? '待接收' : undefined
  }
  return getWarehouseSubStatusLabel(warehouseSubStatus)
}
</script>

<style scoped>
.workflow-progress {
  display: flex;
  flex-direction: column;
  gap: 0;
}
.step-row {
  display: flex;
  gap: 0.625rem;
  align-items: flex-start;
}
.step-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-shrink: 0;
  width: 1.25rem;
}
.step-dot {
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 2px solid rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-faint));
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.18s ease,
    opacity 0.18s ease;
}
.step-done .step-dot {
  border-color: rgb(var(--yb-success-border-bright));
  background: rgb(var(--yb-success-bright));
  color: rgb(var(--yb-success-contrast));
  box-shadow: 0 0 0 3px rgb(var(--yb-success-bright) / 0.12), 0 8px 18px -14px rgb(var(--yb-success-bright) / 0.9);
}
.step-current .step-dot {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}
.step-skipped .step-dot {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-border-strong));
}
.step-line {
  width: 2px;
  flex: 1;
  min-height: 1.25rem;
  background: rgb(var(--yb-text-muted-strong) / 0.35);
  margin: 2px 0;
}
.step-done .step-line,
.step-done + .step-row .step-line {
  background: rgb(var(--yb-brand-border-strong));
}
.step-content {
  padding: 0 0 0.875rem 0;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}
.step-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: rgb(var(--yb-text-muted));
  line-height: 1.25rem;
}
.step-done .step-label { color: rgb(var(--yb-success-strong)); }
.step-current .step-label { color: rgb(var(--yb-brand)); font-weight: 800; }
.step-skipped .step-label { color: rgb(var(--yb-text-faint)); text-decoration: line-through; }
.step-sublabel {
  font-size: 0.6875rem;
  color: rgb(var(--yb-text-faint));
}

.workflow-progress--horizontal {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.125rem 0.25rem;
  padding: 0.35rem 0.5rem;
}
.workflow-progress--naive {
  width: auto;
  max-width: 100%;
  min-width: 0;
  flex-wrap: nowrap;
  justify-content: center;
  gap: 0.28rem;
  padding: 0.44rem 0.62rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 999px;
  background: rgb(var(--yb-surface-soft));
  font-family: var(--yb-font-text);
  box-shadow: none;
}

.workflow-progress--naive :deep(.n-step),
.workflow-progress--naive :deep(.n-step-content),
.workflow-progress--naive :deep(.n-step-content-header__title),
.workflow-progress--naive :deep(.n-step-content__description) {
  font-family: var(--yb-font-text);
}

.workflow-progress--naive :deep(.n-step) {
  width: auto;
  min-width: 0;
  flex: 0 1 auto;
  align-items: center;
}

.workflow-progress--naive :deep(.n-step-indicator) {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  filter: none;
}

.workflow-progress--naive :deep(.n-step--process-status .n-step-indicator) {
  filter: none;
}

.workflow-progress--naive :deep(.n-step-content) {
  flex: 0 1 auto;
  min-width: 0;
  overflow: visible;
  padding-left: 0.36rem;
}

.workflow-progress--naive :deep(.n-step-content-header) {
  min-width: 0;
  align-items: center;
  gap: 0;
}

.workflow-progress--naive :deep(.n-step-content-header__title) {
  max-width: none;
  overflow: visible;
  color: inherit;
  text-overflow: clip;
  white-space: nowrap;
  letter-spacing: 0;
}

.workflow-progress--naive :deep(.n-step-content__description) {
  max-width: none;
  overflow: visible;
  margin-top: 0.1rem;
  font-size: 0.625rem;
  line-height: 1.1;
  opacity: 0.82;
  text-overflow: clip;
  white-space: nowrap;
}

.workflow-progress--naive :deep(.n-step--process-status) {
  padding: 0.22rem 0.72rem 0.22rem 0.32rem;
  border: 1px solid rgb(var(--yb-brand-border-strong));
  border-radius: 999px;
  background: rgb(var(--yb-brand-soft));
  box-shadow: none;
}

.workflow-progress--naive :deep(.n-step--finish-status .n-step-content-header__title) {
  color: rgb(var(--yb-success-strong));
}

.workflow-progress--naive :deep(.n-step--wait-status) {
  opacity: 0.85;
}

.workflow-progress--naive :deep(.n-step--wait-status .n-step-content-header__title) {
  color: rgb(var(--yb-text-muted));
}

.workflow-progress--naive :deep(.n-step-splitor) {
  width: clamp(1.25rem, 2.4vw, 2.5rem);
  min-width: clamp(1.25rem, 2.4vw, 2.5rem);
  max-width: 2.5rem;
  flex: 0 0 clamp(1.25rem, 2.4vw, 2.5rem);
  height: 2px;
  margin: 0 0.4rem;
  border-radius: 999px;
  opacity: 0.86;
}

.workflow-progress--naive :deep(.n-step--process-status .n-step-splitor) {
  position: relative;
  overflow: hidden;
}

.workflow-progress--naive :deep(.n-step--process-status .n-step-splitor)::after {
  display: none;
}

@media (max-width: 900px) {
  .workflow-progress--naive {
    padding-inline: 0.5rem;
    gap: 0.1rem;
  }

  .workflow-progress--naive :deep(.n-step-content) {
    padding-left: 0.28rem;
  }

  .workflow-progress--naive :deep(.n-step-content-header__title) {
    max-width: 3.7rem;
    font-size: 0.72rem;
  }

  .workflow-progress--naive :deep(.n-step-content__description) {
    display: none;
  }

  .workflow-progress--naive :deep(.n-step-splitor) {
    width: 0.9rem;
    min-width: 0.9rem;
    flex-basis: 0.9rem;
    margin-inline: 0.18rem;
  }
}

.step-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  max-width: 100%;
  border-radius: 999px;
  opacity: 0.78;
  transition:
    opacity 0.16s ease,
    transform 0.16s ease,
    color 0.16s ease;
}
.step-chip:hover,
.step-chip:focus-within {
  opacity: 1;
  transform: translateY(-1px);
}
.step-chip.step-done,
.step-chip.step-current {
  opacity: 1;
}
.step-dot--sm {
  position: relative;
  width: 0.95rem;
  height: 0.95rem;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid rgb(var(--yb-text-placeholder) / 0.55);
  background: rgb(var(--yb-text-muted-strong) / 0.22);
  color: rgb(var(--yb-text-disabled) / 0.72);
  box-shadow: inset 0 1px 0 rgb(var(--yb-surface) / 0.08);
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.18s ease;
}
.step-check {
  width: 0.5rem;
  height: 0.5rem;
}
.step-done .step-dot--sm {
  border-color: rgb(var(--yb-success-border-bright));
  background: rgb(var(--yb-success-bright));
  color: rgb(var(--yb-success-contrast));
  box-shadow: 0 0 0 3px rgb(var(--yb-success-bright) / 0.12), 0 8px 18px -14px rgb(var(--yb-success-bright) / 0.9);
}
.step-current .step-dot--sm {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  transform: scale(1.05);
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}
.step-skipped .step-dot--sm {
  border-color: rgb(var(--yb-text-muted-strong) / 0.34);
  background: rgb(var(--yb-text-soft) / 0.18);
  color: rgb(var(--yb-text-muted-strong) / 0.7);
}
.step-chip-text {
  display: inline-flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0 0.15rem;
  min-width: 0;
}
.workflow-progress--horizontal .step-label {
  font-size: 0.75rem;
  font-weight: 750;
  padding: 0;
  line-height: 1.2;
  color: rgb(var(--yb-text-muted));
}
.workflow-progress--horizontal .step-chip.step-done .step-label {
  color: rgb(var(--yb-success-strong));
}
.workflow-progress--horizontal .step-chip.step-current .step-label {
  color: rgb(var(--yb-brand));
  font-weight: 900;
}
.workflow-progress--horizontal .step-chip.step-skipped .step-label {
  color: rgb(var(--yb-text-faint));
  text-decoration: line-through;
}
.step-sublabel-inline {
  font-size: 0.625rem;
  color: rgb(var(--yb-text-faint));
  font-weight: 500;
}
.step-sep {
  position: relative;
  width: clamp(1.25rem, 4vw, 3.75rem);
  height: 0.25rem;
  overflow: hidden;
  border-radius: 999px;
  color: transparent;
  background: rgb(var(--yb-border-strong));
  user-select: none;
  padding: 0;
}
.step-chip.step-done + .step-sep {
  background: rgb(var(--yb-brand-border-strong));
}
.step-chip.step-done + .step-sep::after,
.step-chip.step-current + .step-sep::after {
  display: none;
}

@keyframes workflow-current-breath {
  0%,
  100% {
    box-shadow: 0 0 0 5px rgb(var(--yb-workflow-glow) / 0.12), 0 0 18px rgb(var(--yb-workflow-glow) / 0.24);
  }
  50% {
    box-shadow: 0 0 0 8px rgb(var(--yb-workflow-glow) / 0.07), 0 0 28px rgb(var(--yb-workflow-glow) / 0.4);
  }
}

@keyframes workflow-line-sweep {
  0% {
    opacity: 0;
    transform: translateX(-120%);
  }
  18%,
  72% {
    opacity: 0.62;
  }
  100% {
    opacity: 0;
    transform: translateX(240%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .step-current .step-dot,
  .step-current .step-dot--sm,
  .step-chip.step-done + .step-sep::after,
  .step-chip.step-current + .step-sep::after {
    animation: none;
  }
}
</style>
