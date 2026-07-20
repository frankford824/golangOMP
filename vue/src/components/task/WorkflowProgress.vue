<template>
  <ol v-if="variant === 'horizontal'" class="workflow-progress workflow-progress--horizontal" aria-label="任务进度">
    <li v-for="(step, index) in steps" :key="step.key" :class="`is-${step.state}`" :aria-current="step.state === 'current' ? 'step' : undefined">
      <span class="step-marker" aria-hidden="true">
        <svg v-if="step.state === 'done'" viewBox="0 0 24 24"><path d="m5 12.5 4.2 4L19 7" /></svg>
        <span v-else>{{ index + 1 }}</span>
      </span>
      <span class="step-copy"><strong>{{ step.label }}</strong><small v-if="step.description">{{ step.description }}</small></span>
      <span v-if="index < steps.length - 1" class="step-connector" aria-hidden="true"><i /></span>
    </li>
  </ol>

  <ol v-else class="workflow-progress workflow-progress--vertical" aria-label="任务进度">
    <li v-for="step in steps" :key="step.key" :class="`is-${step.state}`">
      <span class="marker" aria-hidden="true">{{ step.state === 'done' ? '✓' : '' }}</span>
      <span><strong>{{ step.label }}</strong><small v-if="step.description">{{ step.description }}</small></span>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'

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

</script>

<style scoped>
.workflow-progress{list-style:none;margin:0;padding:0}.workflow-progress--horizontal{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));width:100%;min-width:650px}.workflow-progress--horizontal li{position:relative;display:grid;grid-template-columns:30px minmax(0,1fr);align-items:start;gap:10px;min-width:0;padding-right:34px;color:rgb(var(--yb-text-inverse)/.68)}.step-marker{position:relative;z-index:2;display:grid;width:28px;height:28px;place-items:center;border:1.5px solid rgb(var(--yb-text-inverse)/.42);border-radius:50%;background:rgb(var(--yb-text-night)/.26);color:rgb(var(--yb-text-inverse));font:800 12px var(--yb-font-data)}.step-marker svg{width:16px;height:16px;fill:none;stroke:currentColor;stroke-linecap:round;stroke-linejoin:round;stroke-width:2.2}.step-copy{display:grid;gap:2px;min-width:0;padding-top:1px}.step-copy strong{font-size:13px;font-weight:800;color:inherit}.step-copy small{overflow:hidden;color:inherit;font-size:10px;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.step-connector{position:absolute;z-index:0;top:13px;left:34px;right:6px;height:1px;overflow:hidden;background:rgb(var(--yb-text-inverse)/.22)}.step-connector i{position:absolute;inset:0;display:block;background:rgb(var(--yb-text-inverse)/.22)}.workflow-progress--horizontal .is-done{color:rgb(var(--yb-text-inverse))}.workflow-progress--horizontal .is-done .step-marker{border-color:rgb(var(--yb-success));background:rgb(var(--yb-success));box-shadow:0 0 0 4px rgb(var(--yb-success)/.12)}.workflow-progress--horizontal .is-done .step-connector{background:rgb(var(--yb-success)/.62)}.workflow-progress--horizontal .is-current{color:rgb(var(--yb-text-inverse))}.workflow-progress--horizontal .is-current .step-marker{border-color:rgb(var(--yb-brand-bright));background:rgb(var(--yb-brand));box-shadow:0 0 0 5px rgb(var(--yb-brand-bright)/.16)}.workflow-progress--horizontal .is-current .step-connector{background:rgb(var(--yb-text-inverse)/.18)}.workflow-progress--horizontal .is-current .step-connector i{width:32%;background:linear-gradient(90deg,transparent,rgb(var(--yb-brand-bright)),transparent);box-shadow:0 0 8px rgb(var(--yb-brand-bright)/.75)}.workflow-progress--horizontal .is-skipped{opacity:.5}.workflow-progress--vertical{display:grid;gap:0}.workflow-progress--vertical li{position:relative;display:grid;grid-template-columns:24px 1fr;gap:10px;min-height:54px;color:rgb(var(--yb-text-muted))}.workflow-progress--vertical li:not(:last-child)::after{content:"";position:absolute;left:10px;top:22px;bottom:0;width:2px;background:rgb(var(--yb-border))}.marker{position:relative;z-index:1;width:22px;height:22px;border:2px solid rgb(var(--yb-border));border-radius:50%;display:grid;place-items:center;background:rgb(var(--yb-surface));font-size:12px}.workflow-progress--vertical span:last-child{display:grid;gap:2px}.workflow-progress--vertical small{font-size:12px}.is-done .marker{border-color:rgb(var(--yb-success-strong));background:rgb(var(--yb-success-strong));color:rgb(var(--yb-text-inverse))}.workflow-progress--vertical .is-current{color:rgb(var(--yb-text))}.is-current .marker{border-color:rgb(var(--yb-brand));box-shadow:0 0 0 4px rgb(var(--yb-brand-soft))}.is-skipped{opacity:.55}@media(prefers-reduced-motion:no-preference){.workflow-progress--horizontal .is-current .step-marker{animation:current-step-breathe 2.8s ease-in-out infinite}.workflow-progress--horizontal .is-current .step-connector i{animation:current-step-flow 2.4s ease-in-out infinite}@keyframes current-step-breathe{50%{box-shadow:0 0 0 8px rgb(var(--yb-brand-bright)/.08)}}@keyframes current-step-flow{0%{transform:translateX(-120%)}100%{transform:translateX(350%)}}}
@media(max-width:760px){.workflow-progress--horizontal{min-width:0}.workflow-progress--horizontal li{grid-template-columns:22px minmax(0,1fr);gap:5px;padding-right:4px}.step-marker{width:22px;height:22px;font-size:10px}.step-marker svg{width:13px;height:13px}.step-copy{padding-top:2px}.step-copy strong{font-size:10px;line-height:1.25;white-space:nowrap}.step-copy small{display:none}.step-connector{top:10px;left:25px;right:1px}.workflow-progress--horizontal .is-done .step-marker{box-shadow:0 0 0 3px rgb(var(--yb-success)/.1)}.workflow-progress--horizontal .is-current .step-marker{box-shadow:0 0 0 3px rgb(var(--yb-brand-bright)/.14)}}
</style>
