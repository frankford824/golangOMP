<template>
  <NSteps
    v-if="variant === 'horizontal'"
    class="workflow-progress workflow-progress--horizontal"
    :current="currentStep"
    status="process"
    size="small"
    content-placement="right"
  >
    <NStep
      v-for="step in steps"
      :key="step.key"
      :title="step.label"
      :description="step.description"
      :status="naiveStatus(step.state)"
    />
  </NSteps>

  <ol v-else class="workflow-progress workflow-progress--vertical" aria-label="任务进度">
    <li v-for="step in steps" :key="step.key" :class="`is-${step.state}`">
      <span class="marker" aria-hidden="true">{{ step.state === 'done' ? '✓' : '' }}</span>
      <span><strong>{{ step.label }}</strong><small v-if="step.description">{{ step.description }}</small></span>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NStep, NSteps } from 'naive-ui'

type StepState = 'done' | 'current' | 'pending' | 'skipped'
type Step = { key: string; label: string; description?: string; state: StepState }

const props = withDefaults(defineProps<{ task: { status?: string; task_status?: string; taskType?: string; task_type?: string; businessType?: string }; variant?: 'vertical' | 'horizontal' }>(), {
  variant: 'vertical',
})

const steps = computed<Step[]>(() => {
  const status = String(props.task.status || props.task.task_status || '')
  const taskType = String(props.task.taskType || props.task.task_type || props.task.businessType || '').toLowerCase()
  const completed = status === 'Completed' || status === 'Archived'
  const created = !['', 'Draft'].includes(status)

  if (taskType === 'sku_planning') {
    return [
      { key: 'create', label: '创建', description: '录入策划信息', state: created || completed ? 'done' : 'current' },
      { key: 'design', label: '设计', description: '不适用', state: 'skipped' },
      { key: 'audit', label: '审核', description: '不适用', state: 'skipped' },
      { key: 'done', label: '已结单', description: 'SKU 与信息已生成', state: completed ? 'done' : 'pending' },
    ]
  }

  if (taskType === 'retouch' || taskType === 'retouch_task') {
    return [
      { key: 'create', label: '创建', state: created || completed ? 'done' : 'current' },
      { key: 'design', label: '修图', description: '提交最终成品', state: completed ? 'done' : status === 'InProgress' ? 'current' : 'pending' },
      { key: 'audit', label: '审核', description: '不适用', state: 'skipped' },
      { key: 'done', label: '已结单', state: completed ? 'done' : 'pending' },
    ]
  }

  const designDone = ['PendingAudit', 'Completed', 'Archived'].includes(status)
  const designCurrent = ['Assigned', 'InProgress'].includes(status)
  const auditDone = completed
  return [
    { key: 'create', label: '创建', state: created || completed ? 'done' : 'current' },
    { key: 'design', label: '设计', state: designDone ? 'done' : designCurrent ? 'current' : 'pending' },
    { key: 'audit', label: '审核', state: auditDone ? 'done' : status === 'PendingAudit' ? 'current' : 'pending' },
    { key: 'done', label: '已结单', state: completed ? 'done' : 'pending' },
  ]
})

const currentStep = computed(() => {
  const current = steps.value.findIndex((step) => step.state === 'current')
  if (current >= 0) return current + 1
  const done = steps.value.reduce((last, step, index) => step.state === 'done' ? index : last, 0)
  return done + 1
})

function naiveStatus(state: StepState): 'process' | 'finish' | 'wait' {
  if (state === 'done') return 'finish'
  if (state === 'current') return 'process'
  return 'wait'
}
</script>

<style scoped>
.workflow-progress--horizontal{width:min(760px,100%)}.workflow-progress--vertical{list-style:none;margin:0;padding:0;display:grid;gap:0}.workflow-progress--vertical li{position:relative;display:grid;grid-template-columns:24px 1fr;gap:10px;min-height:54px;color:rgb(var(--yb-text-muted))}.workflow-progress--vertical li:not(:last-child)::after{content:"";position:absolute;left:10px;top:22px;bottom:0;width:2px;background:rgb(var(--yb-border))}.marker{position:relative;z-index:1;width:22px;height:22px;border:2px solid rgb(var(--yb-border));border-radius:50%;display:grid;place-items:center;background:rgb(var(--yb-surface));font-size:12px}.workflow-progress--vertical span:last-child{display:grid;gap:2px}.workflow-progress--vertical small{font-size:12px}.is-done .marker{border-color:rgb(var(--yb-success-strong));background:rgb(var(--yb-success-strong));color:rgb(var(--yb-text-inverse))}.workflow-progress--vertical .is-current{color:rgb(var(--yb-text))}.is-current .marker{border-color:rgb(var(--yb-brand));box-shadow:0 0 0 4px rgb(var(--yb-brand-soft))}.is-skipped{opacity:.55}
</style>
