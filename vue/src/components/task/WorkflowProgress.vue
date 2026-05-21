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
    const warehouseStatuses = ['PendingWarehouseReceive', 'PendingWarehouseQC', 'PendingProductionTransfer', 'RejectedByWarehouse'] as const
    const isWarehouseStage = warehouseStatuses.includes(custStatus as typeof warehouseStatuses[number]) ||
      mainStatus === 'WAREHOUSE_PENDING' ||
      mainStatus === 'WAREHOUSE_PROCESSING' ||
      mainStatus === 'READY_TO_CLOSE' ||
      mainStatus === 'CLOSED'

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
        state: custStatus === 'PendingCustomizationProduction'
          ? 'pending'
          : custStatus === 'PendingCustomizationReview'
            ? 'current'
            : 'done',
      },
      {
        key: 'warehouse',
        label: '仓库接收',
        subLabel: t.warehouseSubStatus ? getWarehouseSubStatusLabel(t.warehouseSubStatus) : undefined,
        state: isWarehouseStage
          ? mainStatus === 'READY_TO_CLOSE' || mainStatus === 'CLOSED' || legacyStatus === 'Completed'
            ? 'done'
            : 'current'
          : 'pending',
      },
      {
        key: 'close',
        label: '结单',
        state: mainStatus === 'CLOSED' || legacyStatus === 'Completed' ? 'done' : mainStatus === 'READY_TO_CLOSE' ? 'current' : 'pending',
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

  return [
    {
      key: 'created',
      label: '创建',
      state: mainStatus && mainStatus !== 'DRAFT' ? 'done' : 'current',
    },
    {
      key: 'design',
      label: '设计',
      subLabel: t.designSubStatus ? getDesignSubStatusLabel(t.designSubStatus) : undefined,
      state: resolveStepState(['INFO_PENDING'], ['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING', 'READY_TO_CLOSE', 'CLOSED'], mainStatus),
    },
    {
      key: 'audit',
      label: '审核',
      subLabel: t.auditSubStatus ? getAuditSubStatusLabel(t.auditSubStatus) : undefined,
      state:
        t.auditSubStatus === 'NOT_REQUIRED'
          ? 'skipped'
          : resolveStepState(['INFO_PENDING'], ['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING', 'READY_TO_CLOSE', 'CLOSED'], mainStatus),
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
  indicatorColorFinish: '#22c55e',
  indicatorColorProcess: '#64d2ff',
  indicatorColorWait: 'rgba(15, 23, 42, 0.95)',
  indicatorBorderColorFinish: '#86efac',
  indicatorBorderColorProcess: '#bae6fd',
  indicatorBorderColorWait: 'rgba(100, 116, 139, 0.65)',
  indicatorTextColorFinish: '#03120b',
  indicatorTextColorProcess: '#031420',
  indicatorTextColorWait: '#64748b',
  headerTextColorFinish: '#eafbf0',
  headerTextColorProcess: '#e0f7ff',
  headerTextColorWait: '#94a3b8',
  descriptionTextColorFinish: '#86efac',
  descriptionTextColorProcess: '#7dd3fc',
  descriptionTextColorWait: '#64748b',
  splitorColorFinish: 'rgba(100, 210, 255, 0.62)',
  splitorColorProcess: 'rgba(100, 210, 255, 0.46)',
  splitorColorWait: 'rgba(71, 85, 105, 0.6)',
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
  border: 2px solid #d1d5db;
  background: #fff;
  color: #9ca3af;
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.18s ease,
    opacity 0.18s ease;
}
.step-done .step-dot {
  border-color: #86efac;
  background: #22c55e;
  color: #03120b;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12), 0 8px 18px -14px rgba(34, 197, 94, 0.9);
}
.step-current .step-dot {
  border-color: #64d2ff;
  background: #64d2ff;
  color: #031420;
  box-shadow: 0 0 0 5px rgba(100, 210, 255, 0.15), 0 0 22px rgba(100, 210, 255, 0.32);
  animation: workflow-current-breath 2.8s ease-in-out infinite;
}
.step-skipped .step-dot {
  border-color: #e5e7eb;
  background: #f9fafb;
  color: #d1d5db;
}
.step-line {
  width: 2px;
  flex: 1;
  min-height: 1.25rem;
  background: rgba(100, 116, 139, 0.35);
  margin: 2px 0;
}
.step-done .step-line,
.step-done + .step-row .step-line {
  background: linear-gradient(180deg, #22c55e 0%, #64d2ff 100%);
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
  color: #b8c4d9;
  line-height: 1.25rem;
}
.step-done .step-label { color: #a7fbc1; }
.step-current .step-label { color: #e0f7ff; font-weight: 800; }
.step-skipped .step-label { color: #64748b; text-decoration: line-through; }
.step-sublabel {
  font-size: 0.6875rem;
  color: #92a0b8;
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
  border: 1px solid rgba(148, 113, 255, 0.28);
  border-radius: 999px;
  background:
    linear-gradient(135deg, rgba(35, 47, 69, 0.78), rgba(20, 28, 44, 0.82)),
    radial-gradient(circle at 18% 35%, rgba(255, 45, 146, 0.16), transparent 34%),
    radial-gradient(circle at 86% 45%, rgba(100, 210, 255, 0.18), transparent 34%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.10),
    0 10px 28px -26px rgba(125, 211, 252, 0.58);
}

.workflow-progress--naive :deep(.n-step) {
  width: auto !important;
  min-width: 0;
  flex: 0 1 auto !important;
  align-items: center;
}

.workflow-progress--naive :deep(.n-step-indicator) {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  filter: drop-shadow(0 0 10px rgba(100, 210, 255, 0.16));
}

.workflow-progress--naive :deep(.n-step--process-status .n-step-indicator) {
  filter: drop-shadow(0 0 14px rgba(100, 210, 255, 0.34));
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
  border: 1px solid rgba(125, 211, 252, 0.42);
  border-radius: 999px;
  background:
    linear-gradient(135deg, rgba(47, 83, 130, 0.48), rgba(38, 45, 76, 0.62));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    0 0 0 1px rgba(148, 113, 255, 0.10),
    0 0 24px rgba(100, 210, 255, 0.18);
}

.workflow-progress--naive :deep(.n-step--finish-status .n-step-content-header__title) {
  color: #bfffd2;
}

.workflow-progress--naive :deep(.n-step--wait-status) {
  opacity: 0.72;
}

.workflow-progress--naive :deep(.n-step--wait-status .n-step-content-header__title) {
  color: #8fa0b8;
}

.workflow-progress--naive :deep(.n-step-splitor) {
  width: clamp(1.25rem, 2.4vw, 2.5rem) !important;
  min-width: clamp(1.25rem, 2.4vw, 2.5rem) !important;
  max-width: 2.5rem;
  flex: 0 0 clamp(1.25rem, 2.4vw, 2.5rem) !important;
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
  content: '';
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.72), transparent);
  animation: workflow-line-sweep 3.8s ease-out infinite;
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
    width: 0.9rem !important;
    min-width: 0.9rem !important;
    flex-basis: 0.9rem !important;
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
  border: 1px solid rgba(148, 163, 184, 0.55);
  background: rgba(100, 116, 139, 0.22);
  color: rgba(203, 213, 225, 0.72);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
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
  border-color: #86efac;
  background: #22c55e;
  color: #03120b;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12), 0 8px 18px -14px rgba(34, 197, 94, 0.9);
}
.step-current .step-dot--sm {
  border-color: #64d2ff;
  background: #64d2ff;
  color: #031420;
  transform: scale(1.08);
  box-shadow: 0 0 0 5px rgba(100, 210, 255, 0.15), 0 0 22px rgba(100, 210, 255, 0.32);
  animation: workflow-current-breath 2.8s ease-in-out infinite;
}
.step-skipped .step-dot--sm {
  border-color: rgba(100, 116, 139, 0.34);
  background: rgba(71, 85, 105, 0.18);
  color: rgba(100, 116, 139, 0.7);
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
  color: #cbd5e1;
}
.workflow-progress--horizontal .step-chip.step-done .step-label {
  color: #eafbf0;
}
.workflow-progress--horizontal .step-chip.step-current .step-label {
  color: #e0f7ff;
  font-weight: 900;
}
.workflow-progress--horizontal .step-chip.step-skipped .step-label {
  color: #64748b;
  text-decoration: line-through;
}
.step-sublabel-inline {
  font-size: 0.625rem;
  color: #92a0b8;
  font-weight: 500;
}
.step-sep {
  position: relative;
  width: clamp(1.25rem, 4vw, 3.75rem);
  height: 0.25rem;
  overflow: hidden;
  border-radius: 999px;
  color: transparent;
  background: rgba(30, 41, 59, 0.92);
  user-select: none;
  padding: 0;
}
.step-chip.step-done + .step-sep {
  background: linear-gradient(90deg, #22c55e 0%, #64d2ff 100%);
}
.step-chip.step-done + .step-sep::after,
.step-chip.step-current + .step-sep::after {
  content: '';
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.72), transparent);
  opacity: 0;
  transform: translateX(-120%);
  animation: workflow-line-sweep 3.4s ease-out 0.6s 1 both;
}
.step-chip.step-current + .step-sep::after {
  opacity: 0.55;
  animation-iteration-count: infinite;
  animation-duration: 3.8s;
}

@keyframes workflow-current-breath {
  0%,
  100% {
    box-shadow: 0 0 0 5px rgba(100, 210, 255, 0.12), 0 0 18px rgba(100, 210, 255, 0.24);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(100, 210, 255, 0.07), 0 0 28px rgba(100, 210, 255, 0.4);
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
    animation: none !important;
  }
}
</style>
