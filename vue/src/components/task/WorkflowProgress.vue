<template>
  <!-- 横向：任务详情顶栏嵌入，收窄不独占全宽 -->
  <div
    v-if="variant === 'horizontal'"
    class="workflow-progress workflow-progress--horizontal"
    role="list"
  >
    <template v-for="(step, i) in steps" :key="step.key">
      <div
        class="step-chip"
        role="listitem"
        :class="{
          'step-done': step.state === 'done',
          'step-current': step.state === 'current',
          'step-pending': step.state === 'pending',
          'step-skipped': step.state === 'skipped',
        }"
      >
        <span class="step-dot step-dot--sm">
          <svg v-if="step.state === 'done'" viewBox="0 0 12 12" fill="none" class="step-check">
            <path
              d="M2 6l3 3 5-5"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </span>
        <span class="step-chip-text">
          <span class="step-label">{{ step.label }}</span>
          <span v-if="step.subLabel" class="step-sublabel-inline">{{ step.subLabel }}</span>
        </span>
      </div>
      <span v-if="i < steps.length - 1" class="step-sep" aria-hidden="true">›</span>
    </template>
  </div>
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
import type { Task } from '@/domain/types/task'
import {
  getDesignSubStatusLabel,
  getAuditSubStatusLabel,
  getWarehouseSubStatusLabel,
} from '@/domain/enums/task-status'

type StepState = 'done' | 'current' | 'pending' | 'skipped'

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
const isCustomization = computed(() => props.task.workflowLane === 'customization')

const CUSTOMIZATION_STEP_ORDER = [
  'PendingCustomizationReview',
  'PendingCustomizationProduction',
  'PendingEffectReview',
  'PendingEffectRevision',
  'PendingProductionTransfer',
  'PendingWarehouseQC',
  'RejectedByWarehouse',
  'PendingWarehouseReceive',
  'Completed',
] as const

function resolveCustomizationStepState(
  activeStatuses: readonly string[],
  doneStatuses: readonly string[],
  legacyStatus?: string,
  mainStatus?: string,
): StepState {
  if (mainStatus === 'CLOSED' || legacyStatus === 'Completed') return 'done'
  if (doneStatuses.includes(legacyStatus ?? '')) return 'done'
  if (mainStatus && ['READY_TO_CLOSE', 'CLOSED'].includes(mainStatus)) return 'done'
  if (activeStatuses.includes(legacyStatus ?? '')) return 'current'
  if (mainStatus && ['WAREHOUSE_PENDING', 'WAREHOUSE_PROCESSING'].includes(mainStatus)) {
    if (activeStatuses.some((s) => s.startsWith('PendingWarehouse') || s === 'RejectedByWarehouse')) return 'current'
    return 'done'
  }
  return 'pending'
}

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
    const idx = CUSTOMIZATION_STEP_ORDER.indexOf(custStatus as typeof CUSTOMIZATION_STEP_ORDER[number])
    const pastStep = (stepStatuses: readonly string[]): boolean =>
      idx >= 0 && stepStatuses.every((s) => {
        const si = CUSTOMIZATION_STEP_ORDER.indexOf(s as typeof CUSTOMIZATION_STEP_ORDER[number])
        return si >= 0 && idx > si
      })

    return [
      {
        key: 'created',
        label: '创建',
        state: mainStatus && mainStatus !== 'DRAFT' ? 'done' : 'current',
      },
      {
        key: 'cust_review',
        label: '定制审核',
        state: resolveCustomizationStepState(
          ['PendingCustomizationReview'],
          pastStep(['PendingCustomizationReview']) ? [custStatus] : [],
          custStatus, mainStatus,
        ),
      },
      {
        key: 'cust_production',
        label: '定制作图',
        state: resolveCustomizationStepState(
          ['PendingCustomizationProduction'],
          pastStep(['PendingCustomizationProduction']) ? [custStatus] : [],
          custStatus, mainStatus,
        ),
      },
      {
        key: 'effect_review',
        label: '效果审核',
        state: resolveCustomizationStepState(
          ['PendingEffectReview', 'PendingEffectRevision'],
          pastStep(['PendingEffectReview', 'PendingEffectRevision']) ? [custStatus] : [],
          custStatus, mainStatus,
        ),
      },
      {
        key: 'production_transfer',
        label: '转生产',
        state: resolveCustomizationStepState(
          ['PendingProductionTransfer'],
          pastStep(['PendingProductionTransfer']) ? [custStatus] : [],
          custStatus, mainStatus,
        ),
      },
      {
        key: 'warehouse',
        label: '云仓接收',
        subLabel: t.warehouseSubStatus ? getWarehouseSubStatusLabel(t.warehouseSubStatus) : undefined,
        state: resolveCustomizationStepState(
          ['PendingWarehouseQC', 'RejectedByWarehouse', 'PendingWarehouseReceive'],
          [],
          custStatus, mainStatus,
        ),
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
  transition: border-color 0.2s, background 0.2s;
}
.step-done .step-dot {
  border-color: #059669;
  background: #059669;
  color: #fff;
}
.step-current .step-dot {
  border-color: #1890ff;
  background: #eff6ff;
  color: #1890ff;
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
  background: #e5e7eb;
  margin: 2px 0;
}
.step-done .step-line,
.step-done + .step-row .step-line {
  background: #059669;
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
  color: #374151;
  line-height: 1.25rem;
}
.step-done .step-label { color: #059669; }
.step-current .step-label { color: #1d4ed8; font-weight: 600; }
.step-skipped .step-label { color: #9ca3af; text-decoration: line-through; }
.step-sublabel {
  font-size: 0.6875rem;
  color: #6b7280;
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
.step-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  max-width: 100%;
}
.step-dot--sm {
  width: 1rem;
  height: 1rem;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 2px solid #d1d5db;
  background: #fff;
  color: #9ca3af;
}
.step-check {
  width: 0.5rem;
  height: 0.5rem;
}
.step-done .step-dot--sm {
  border-color: #059669;
  background: #059669;
  color: #fff;
}
.step-current .step-dot--sm {
  border-color: #1890ff;
  background: #eff6ff;
  color: #1890ff;
}
.step-skipped .step-dot--sm {
  border-color: #e5e7eb;
  background: #f9fafb;
  color: #d1d5db;
}
.step-chip-text {
  display: inline-flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0 0.15rem;
  min-width: 0;
}
.workflow-progress--horizontal .step-label {
  font-size: 0.6875rem;
  padding: 0;
  line-height: 1.2;
}
.workflow-progress--horizontal .step-chip.step-done .step-label {
  color: #059669;
}
.workflow-progress--horizontal .step-chip.step-current .step-label {
  color: #1d4ed8;
  font-weight: 600;
}
.workflow-progress--horizontal .step-chip.step-skipped .step-label {
  color: #9ca3af;
  text-decoration: line-through;
}
.step-sublabel-inline {
  font-size: 0.625rem;
  color: #6b7280;
  font-weight: 400;
}
.step-sep {
  font-size: 0.75rem;
  color: #cbd5e1;
  user-select: none;
  padding: 0 0.1rem;
}
</style>
