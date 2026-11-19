<script setup lang="ts">
import { computed } from 'vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import FeePersonSelect from './FeePersonSelect.vue'
import type { FeePerson } from '../feePeople'

type Field = { name: string; type: 'text' | 'number' | 'date' | 'select'; required: boolean; options?: string[] }
type Node = { name: string; mode: 'AND' | 'OR'; approvers: string[] }
type AmountRule = { min_cents: number; max_cents?: number; mode: 'AND' | 'OR'; approvers: string[] }
type Rules = {
  fields: string[]; required: string[]; nodes: string[]; extra: string
  field_definitions?: Field[]; approval_nodes?: Node[]; amount_rules?: AmountRule[]
  cc_after_approval?: number[]; cc_after_payment?: number[]
}

const props = defineProps<{ draft: Rules; known: Record<number, FeePerson>; legacyThreshold: string }>()
const emit = defineEmits<{ change: []; person: [person: FeePerson] }>()
const draft = computed(() => props.draft)
const editorKeys = new WeakMap<object, number>()
let editorKey = 0
function keyFor(row: object) {
  const existing = editorKeys.get(row)
  if (existing !== undefined) return existing
  const created = editorKey++
  editorKeys.set(row, created)
  return created
}
const fieldTypes = [{ value: 'text', label: '文本' }, { value: 'number', label: '数值' }, { value: 'date', label: '日期' }, { value: 'select', label: '单选' }]
const approvalModes = [{ value: 'AND', label: '会签 · 全部同意' }, { value: 'OR', label: '或签 · 一人同意' }]
const approvalSources = [{ value: 'manager', label: '申请人直属负责人' }, { value: 'fixed', label: '指定人员' }]
function setFieldType(field: Field, type: Field['type']) {
  field.type = type
  field.options = undefined
  emit('change')
}
const stages = [
  { key: 'cc_after_approval' as const, label: '审批完成后抄送' },
  { key: 'cc_after_payment' as const, label: '付款完成后抄送' },
]
const people = (reference: string) => {
  const id = Number(reference.slice(6))
  return id > 0 ? (props.known[id] ?? { id, name: '' }) : undefined
}
const person = (id: number) => props.known[id] ?? { id, name: '' }
const options = (value: string) => [...new Set(value.split(/\r?\n/).map(entry => entry.trim()).filter(Boolean))]
function addField() {
  draft.value.field_definitions ??= []
  draft.value.field_definitions.push({ name: '', type: 'text', required: false })
  emit('change')
}
function switchNodes() {
  draft.value.approval_nodes = draft.value.nodes.map(reference => ({ name: '', mode: 'AND', approvers: [reference] }))
  const threshold = amountInput(props.legacyThreshold, 0)
  if (threshold && draft.value.extra && !draft.value.amount_rules?.length) {
    draft.value.amount_rules = [{ min_cents: threshold, mode: 'AND', approvers: [draft.value.extra] }]
  }
  emit('change')
}
function addNode() {
  draft.value.approval_nodes?.push({ name: '', mode: 'AND', approvers: [''] })
  emit('change')
}
function moveNode(index: number, delta: number) {
  const nodes = draft.value.approval_nodes
  if (!nodes || index + delta < 0 || index + delta >= nodes.length) return
  const [node] = nodes.splice(index, 1)
  if (node) nodes.splice(index + delta, 0, node)
  emit('change')
}
function addRule() {
  draft.value.amount_rules ??= []
  draft.value.amount_rules.push({ min_cents: NaN, mode: 'AND', approvers: [''] })
  emit('change')
}
function amountValue(value?: number) {
  return value !== undefined && Number.isFinite(value) ? (value / 100).toFixed(2) : ''
}
function amountInput(value: string, empty: number | undefined) {
  const parts = /^(\d{1,10})(?:\.(\d{1,2}))?$/.exec(value.trim())
  return value.trim() === '' ? empty : parts ? Number(parts[1]) * 100 + Number((parts[2] || '').padEnd(2, '0')) : NaN
}
function selectPerson(approvers: string[], index: number, selected?: FeePerson) {
  if (selected) emit('person', selected)
  approvers[index] = selected ? 'fixed:' + selected.id : 'fixed:'
  emit('change')
}
function selectCC(stage: typeof stages[number]['key'], selected?: FeePerson, index?: number) {
  const ids = draft.value[stage] ??= []
  if (selected) emit('person', selected)
  if (index !== undefined) {
    if (!selected) ids.splice(index, 1)
    else if (!ids.includes(selected.id)) ids[index] = selected.id
  } else if (selected && !ids.includes(selected.id)) ids.push(selected.id)
  emit('change')
}
const reviewerGroups = computed(() => [
  ...(draft.value.approval_nodes ?? []).map((item, index) => ({ item, index, kind: 'node' as const, label: '第 ' + (index + 1) + ' 步' })),
  ...(draft.value.amount_rules ?? []).map((item, index) => ({ item, index, kind: 'rule' as const, label: '金额规则 ' + (index + 1) })),
])
</script>

<template>
  <fieldset class="oa-fieldset fee-template-rules">
    <legend>自定义字段</legend>
    <div v-for="(field, index) in draft.field_definitions" :key="keyFor(field)" class="fee-template-rule">
      <label>字段名称<input v-model.trim="field.name" maxlength="120" @input="emit('change')" /></label>
      <BaseSelect label="字段类型" :model-value="field.type" :options="fieldTypes" @update:model-value="setFieldType(field, $event as Field['type'])" />
      <label v-if="field.type === 'select'">选项<textarea :defaultValue="field.options?.join('\n')" rows="2" placeholder="每行一个选项" @input="field.options = options(($event.target as HTMLTextAreaElement).value); emit('change')" /></label>
      <div class="oa-actions"><label class="fee-rule-check"><input v-model="field.required" type="checkbox" @change="emit('change')" />必填</label>
        <button type="button" :aria-label="'删除字段 ' + (index + 1)" @click="draft.field_definitions?.splice(index, 1); emit('change')">删除字段</button>
      </div>
    </div>
    <button type="button" :disabled="(draft.field_definitions?.length ?? 0) >= 30" @click="addField">添加字段</button>
  </fieldset>

  <fieldset class="oa-fieldset fee-template-rules">
    <legend>多人审批与分段金额规则</legend>
    <button v-if="!draft.approval_nodes" type="button" @click="switchNodes">配置会签、或签与金额规则</button>
    <template v-else>
      <div v-for="({ item, index, kind, label }) in reviewerGroups" :key="keyFor(item)" class="fee-template-rule">
        <span v-if="kind === 'node' && index > 0" class="fee-rule-connector" aria-hidden="true">↓</span>
        <p class="fee-rule-title">{{ label }}</p>
        <label v-if="'name' in item">节点名称<input v-model.trim="item.name" maxlength="120" placeholder="如行政复核" @input="emit('change')" /></label>
        <template v-if="'min_cents' in item">
          <label>最低金额（元，含）<input :defaultValue="amountValue(item.min_cents)" inputmode="decimal" @input="item.min_cents = amountInput(($event.target as HTMLInputElement).value, NaN)!; emit('change')" /></label>
          <label>最高金额（元，含）<input :defaultValue="amountValue(item.max_cents)" inputmode="decimal" placeholder="留空表示不限" @input="item.max_cents = amountInput(($event.target as HTMLInputElement).value, undefined); emit('change')" /></label>
        </template>
        <BaseSelect label="审批方式" :model-value="item.mode" :options="approvalModes" @update:model-value="item.mode = $event as Node['mode']; emit('change')" />
        <div v-for="(approval, pos) in item.approvers" :key="pos" class="fee-reviewer-rule">
          <BaseSelect :label="label + ' 审批人 ' + (pos + 1)" :model-value="approval === 'manager' ? 'manager' : approval.startsWith('fixed:') ? 'fixed' : ''" :options="approvalSources" placeholder="选择审批人类型" @update:model-value="item.approvers[pos] = $event === 'fixed' ? 'fixed:' : String($event); emit('change')" />
          <FeePersonSelect v-if="approval.startsWith('fixed:')" :label="label + '指定审批人 ' + (pos + 1)" :model-value="people(approval)" @update:model-value="selectPerson(item.approvers, pos, $event)" />
          <button type="button" :disabled="item.approvers.length <= 1" @click="item.approvers.splice(pos, 1); emit('change')">删除审批人</button>
        </div>
        <div class="oa-actions">
          <button type="button" :disabled="item.approvers.length >= 12" @click="item.approvers.push(''); emit('change')">添加审批人</button>
          <template v-if="kind === 'node'">
            <button type="button" :disabled="index === 0" @click="moveNode(index, -1)">上移节点</button>
            <button type="button" :disabled="index + 1 >= (draft.approval_nodes?.length ?? 0)" @click="moveNode(index, 1)">下移节点</button>
            <button type="button" @click="draft.approval_nodes?.splice(index, 1); emit('change')">删除节点</button>
          </template>
          <button v-else type="button" @click="draft.amount_rules?.splice(index, 1); emit('change')">删除金额规则</button>
        </div>
      </div>
      <div class="oa-actions">
        <button type="button" :disabled="(draft.approval_nodes?.length ?? 0) >= 12" @click="addNode">添加多人审批节点</button>
        <button type="button" :disabled="(draft.amount_rules?.length ?? 0) >= 12" @click="addRule">添加金额规则</button>
      </div>
      <p class="oa-muted">金额规则按金额从低到高添加。命中区间时，会在常规审批节点之后追加一组审批人。</p>
    </template>
  </fieldset>

  <fieldset v-for="stage in stages" :key="stage.key" class="oa-fieldset fee-template-rules">
    <legend>{{ stage.label }}</legend>
    <div v-for="(id, index) in draft[stage.key]" :key="id" class="fee-reviewer-rule">
      <FeePersonSelect :label="stage.label + (index + 1)" :model-value="person(id)" @update:model-value="selectCC(stage.key, $event, index)" />
      <button type="button" @click="selectCC(stage.key, undefined, index)">移除</button>
    </div>
    <FeePersonSelect v-if="(draft[stage.key]?.length ?? 0) < 16" :key="draft[stage.key]?.length ?? 0" :label="'添加' + stage.label + '人员'" @update:model-value="selectCC(stage.key, $event)" />
  </fieldset>
</template>

<style scoped>
.fee-template-rules, .fee-template-rule, .fee-reviewer-rule { display: grid; gap: 10px; min-width: 0; }
.fee-template-rule { position: relative; border: 1px solid var(--oa-line); border-radius: 8px; padding: 12px; }
.fee-rule-title { justify-self: start; font-weight: 600; margin: 0; border-radius: 20px; padding: 3px 10px; background: rgb(var(--yb-surface-muted)); }
.fee-rule-connector { position: absolute; top: 0; inset-inline-start: 34px; transform: translateY(-60%); padding-inline: 4px; background: var(--oa-surface); color: var(--oa-secondary); }
.fee-reviewer-rule { border-block-start: 1px solid var(--oa-line); padding-block: 8px; }
.fee-template-rules :deep(button) { justify-self: start; }
.fee-template-rules :deep(.fee-rule-check) { display: inline-flex; align-items: center; gap: 5px; }
</style>
