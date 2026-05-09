<template>
  <div class="close-condition-panel">
    <div class="panel-header">
      <div class="panel-title-wrap">
        <span class="panel-title">结单条件检查</span>
        <span v-if="!isTerminal && task.workflowCanClose !== undefined" class="panel-sub">以下与后端 workflow 门禁一致</span>
      </div>
      <span
        class="close-status-badge"
        :class="isTerminal ? 'badge-terminal' : canClose ? 'badge-ready' : 'badge-not-ready'"
      >
        {{ badgeLabel }}
      </span>
    </div>
    <p v-if="isTerminal" class="terminal-copy">
      任务已结单或已完成，无需再次操作结单。
    </p>
    <ul v-else class="condition-list">
      <li
        v-for="condition in conditions"
        :key="condition.label"
        class="condition-item"
        :class="condition.passed ? 'passed' : 'failed'"
      >
        <span class="condition-icon">{{ condition.passed ? '✓' : '✗' }}</span>
        <span class="condition-label">{{ condition.label }}</span>
        <span v-if="!condition.passed && condition.hint" class="condition-hint">{{ condition.hint }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '@/domain/types/task'
import { checkTaskCompletion } from '@/domain/task-completion'
import { canCloseTaskForArchive, isTaskCloseFlowTerminal } from '@/domain/task-close-eligibility'
import { normalizeTaskType } from '@/domain/enums/task-type'
import { TaskTypeEnum } from '@/domain/enums/task-type'
import { workflowGateReasonLabelCn } from '@/domain/mappers/read-model-labels-cn'

const props = defineProps<{ task: Task }>()

const isTerminal = computed(() => isTaskCloseFlowTerminal(props.task))
const eligibility = computed(() => canCloseTaskForArchive(props.task))
const canClose = computed(() => !isTerminal.value && eligibility.value.allowed)
const badgeLabel = computed(() => {
  if (isTerminal.value) return '已结束'
  return canClose.value ? '可结单' : '未满足'
})
const isPurchase = computed(() => normalizeTaskType(props.task.taskType) === TaskTypeEnum.PURCHASE_TASK)

defineExpose({ canClose })

interface Condition {
  label: string
  passed: boolean
  hint?: string
}

/** 后端返回不可结单原因时优先展示，与 workflow.can_close 对齐 */
const conditions = computed((): Condition[] => {
  const t = props.task
  if (isTaskCloseFlowTerminal(t)) {
    return []
  }
  if (t.workflowCanClose === false && t.cannotCloseReasons && t.cannotCloseReasons.length > 0) {
    return t.cannotCloseReasons.map((r) => {
      const label = workflowGateReasonLabelCn(r.code, r.message)
      const code = (r.code ?? '').trim()
      const hint = code && label !== code ? code : undefined
      return { label, passed: false, hint }
    })
  }

  const list: Condition[] = []

  list.push({ label: '任务号已生成', passed: !!t.taskNo })
  list.push({ label: 'SKU 已绑定', passed: !!t.sku, hint: '请先绑定 SKU' })

  if (!isPurchase.value) {
    const designDone =
      t.designSubStatus === 'FINALIZED' || t.designSubStatus === 'APPROVED'
    list.push({
      label: '设计已终稿',
      passed: designDone,
      hint: '设计尚未终稿确认',
    })
  }

  const warehouseOk =
    t.warehouseSubStatus === 'RECEIVED' ||
    t.warehouseSubStatus === 'DONE' ||
    t.warehouseReceiveStatus === 'received' ||
    t.warehouseReceiveStatus === 'archived'
  list.push({
    label: '仓库已接收',
    passed: warehouseOk,
    hint: '仓库尚未确认接收',
  })

  const notReturned =
    t.warehouseSubStatus !== 'RETURNED' && t.warehouseReceiveStatus !== 'returned'
  list.push({
    label: '无仓库退回',
    passed: notReturned,
    hint: '仓库已退回，需处理后重新提交',
  })

  if (isPurchase.value) {
    list.push({
      label: '成本价已录入',
      passed: !!t.costPrice,
      hint: '采购任务必须录入成本价',
    })
  }

  if (t.workflowCanClose === false && list.every((c) => c.passed)) {
    const r = checkTaskCompletion(t)
    if (!r.canComplete && r.reasons.length) {
      return r.reasons.map((msg) => ({ label: msg, passed: false }))
    }
    if (t.missing_fields_summary_cn?.trim()) {
      return [{ label: t.missing_fields_summary_cn.trim(), passed: false }]
    }
  }

  return list
})
</script>

<style scoped>
.close-condition-panel {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 0.75rem;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}
.panel-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}
.panel-title {
  font-size: 0.8125rem;
  font-weight: 600;
  color: #374151;
}
.panel-sub {
  font-size: 0.625rem;
  font-weight: 500;
  color: #64748b;
}
.close-status-badge {
  font-size: 0.6875rem;
  font-weight: 500;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
}
.badge-ready { background: rgb(16 185 129 / 0.15); color: #059669; }
.badge-not-ready { background: #fef2f2; color: #dc2626; }
.badge-terminal { background: rgb(226 232 240 / 0.9); color: #475569; }
.terminal-copy {
  margin: 0 0 0.5rem;
  font-size: 0.75rem;
  line-height: 1.45;
  color: #64748b;
}
.condition-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.condition-item {
  display: flex;
  align-items: baseline;
  gap: 0.375rem;
  font-size: 0.75rem;
  line-height: 1.4;
}
.condition-icon { font-size: 0.6875rem; width: 0.875rem; flex-shrink: 0; }
.condition-label { color: #374151; }
.passed .condition-icon { color: #059669; }
.failed .condition-icon { color: #dc2626; }
.failed .condition-label { color: #b91c1c; }
.condition-hint { color: #9ca3af; font-size: 0.6875rem; }
</style>
