<template>
  <section class="task-business-editor" aria-label="编辑任务信息">
    <header>
      <div>
        <p class="eyebrow">创建后可维护</p>
        <h3>编辑整张任务单</h3>
        <p>可修改需求、产品、规格与时效；任务编号、创建人、状态、指派、审核、成本规则和 ERP 强制同步保持受控。</p>
      </div>
      <span>保存后写入任务动态</span>
    </header>

    <form @submit.prevent="submit">
      <div class="field-grid">
        <label class="wide">
          <span>产品名称</span>
          <input v-model.trim="draft.productName" maxlength="255" required />
        </label>
        <label>
          <span>ERP 商品 / 款式编码</span>
          <input v-model.trim="draft.productIID" maxlength="100" placeholder="没有可留空" />
        </label>
        <label>
          <span>优先级</span>
          <select v-model="draft.priority">
            <option value="low">低</option>
            <option value="normal">普通</option>
            <option value="high">高</option>
            <option value="critical">紧急</option>
          </select>
        </label>
        <label>
          <span>截止时间</span>
          <input v-model="draft.deadlineAt" type="datetime-local" />
        </label>
        <label>
          <span>数量</span>
          <input v-model="draft.quantity" type="number" min="0" step="1" inputmode="numeric" />
        </label>
        <label class="wide">
          <span>{{ requirementLabel }}</span>
          <textarea v-model.trim="draft.requirement" maxlength="4000" rows="4" required />
        </label>
        <label class="wide">
          <span>运营备注</span>
          <textarea v-model="draft.note" maxlength="2000" rows="3" placeholder="可留空" />
        </label>
        <label>
          <span>规格</span>
          <input v-model.trim="draft.specText" maxlength="255" />
        </label>
        <label>
          <span>尺寸说明</span>
          <input v-model.trim="draft.sizeText" maxlength="255" />
        </label>
        <label>
          <span>材质</span>
          <input v-model.trim="draft.material" maxlength="255" />
        </label>
        <label>
          <span>工艺</span>
          <input v-model.trim="draft.craftText" maxlength="255" />
        </label>
        <label>
          <span>处理方式</span>
          <input v-model.trim="draft.process" maxlength="255" />
        </label>
        <label>
          <span>宽</span>
          <input v-model="draft.width" type="number" min="0" step="any" inputmode="decimal" />
        </label>
        <label>
          <span>高</span>
          <input v-model="draft.height" type="number" min="0" step="any" inputmode="decimal" />
        </label>
        <label>
          <span>面积</span>
          <input v-model="draft.area" type="number" min="0" step="any" inputmode="decimal" />
        </label>
      </div>

      <div v-if="error" class="editor-message error" role="alert">{{ error }}</div>
      <div v-if="saved" class="editor-message success" role="status">任务信息已保存。</div>
      <footer>
        <p>保存只更新业务信息，不改变当前流程节点和负责人。</p>
        <button type="submit" :disabled="submitting || !hasChanges">{{ submitting ? '保存中…' : '保存任务信息' }}</button>
      </footer>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { tasksApi } from '@/services/api/tasksApi'

interface TaskBusinessInfo extends Record<string, unknown> {
  product_name_snapshot?: string
  product_name?: string
  i_id?: string
  product_i_id?: string
  priority?: string
  deadline_at?: string
  due_at?: string
  design_requirement?: string
  change_request?: string
  requirement_description?: string
  description?: string
  note?: string
  operation_note?: string
  remark?: string
  spec_text?: string
  size_text?: string
  material?: string
  craft_text?: string
  process?: string
  quantity?: number | string | null
  width?: number | string | null
  height?: number | string | null
  area?: number | string | null
  task_type?: string
}

const props = defineProps<{
  taskId: number
  task: TaskBusinessInfo
}>()
const emit = defineEmits<{ saved: [] }>()

const draft = reactive({
  productName: '',
  productIID: '',
  priority: 'normal',
  deadlineAt: '',
  requirement: '',
  note: '',
  specText: '',
  sizeText: '',
  material: '',
  craftText: '',
  process: '',
  quantity: '',
  width: '',
  height: '',
  area: '',
})
const baseline = ref('')
const submitting = ref(false)
const error = ref('')
const saved = ref(false)

const requirementLabel = computed(() => {
  if (['retouch', 'retouch_task'].includes(String(props.task.task_type || ''))) return '修图要求'
  if (String(props.task.task_type || '') === 'sku_planning') return '策划说明'
  if (['regular_customization', 'customer_customization'].includes(String(props.task.task_type || ''))) return '定制需求'
  if (String(props.task.task_type || '') === 'original_product_development') return '修改要求'
  return '设计需求'
})
const hasChanges = computed(() => JSON.stringify(draft) !== baseline.value)

function text(...values: unknown[]) {
  const value = values.find((item) => typeof item === 'string' && item.trim())
  return typeof value === 'string' ? value : ''
}

function numberText(value: unknown) {
  return value == null || value === '' ? '' : String(value)
}

function localDateTime(value: unknown) {
  if (!value) return ''
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return ''
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function hydrate() {
  Object.assign(draft, {
    productName: text(props.task.product_name_snapshot, props.task.product_name),
    productIID: text(props.task.i_id, props.task.product_i_id),
    priority: normalizePriority(props.task.priority),
    deadlineAt: localDateTime(props.task.deadline_at || props.task.due_at),
    requirement: text(props.task.design_requirement, props.task.change_request, props.task.requirement_description, props.task.description),
    note: text(props.task.note, props.task.operation_note, props.task.remark),
    specText: text(props.task.spec_text),
    sizeText: text(props.task.size_text),
    material: text(props.task.material),
    craftText: text(props.task.craft_text),
    process: text(props.task.process),
    quantity: numberText(props.task.quantity),
    width: numberText(props.task.width),
    height: numberText(props.task.height),
    area: numberText(props.task.area),
  })
  baseline.value = JSON.stringify(draft)
  saved.value = false
  error.value = ''
}

function normalizePriority(value: unknown) {
  const raw = String(value || 'normal').toLowerCase()
  const aliases: Record<string, string> = { 低: 'low', 普通: 'normal', 高: 'high', 紧急: 'critical', urgent: 'critical' }
  return ['low', 'normal', 'high', 'critical'].includes(raw) ? raw : aliases[String(value)] || 'normal'
}

function optionalNumber(value: string) {
  if (!value.trim()) return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function deadlinePayload(value: string) {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

async function submit() {
  if (submitting.value || !hasChanges.value) return
  submitting.value = true
  error.value = ''
  saved.value = false
  try {
    const requirementField = String(props.task.task_type || '') === 'original_product_development'
      ? 'change_request'
      : 'design_requirement'
    const patch: Record<string, unknown> = {
      product_name: draft.productName,
      i_id: draft.productIID,
      priority: draft.priority,
      deadline_at: deadlinePayload(draft.deadlineAt),
      [requirementField]: draft.requirement,
      note: draft.note,
      spec_text: draft.specText,
      size_text: draft.sizeText,
      material: draft.material,
      craft_text: draft.craftText,
      process: draft.process,
      remark: '创建人编辑整张任务单',
    }
    for (const [key, value] of Object.entries({
      quantity: optionalNumber(draft.quantity),
      width: optionalNumber(draft.width),
      height: optionalNumber(draft.height),
      area: optionalNumber(draft.area),
    })) {
      if (value !== undefined) patch[key] = value
    }
    await tasksApi.patchBusinessInfo(String(props.taskId), patch)
    baseline.value = JSON.stringify(draft)
    saved.value = true
    emit('saved')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '任务信息保存失败，请稍后重试。'
  } finally {
    submitting.value = false
  }
}

watch(() => props.task, hydrate, { immediate: true })
</script>

<style scoped>
.task-business-editor{grid-column:1/-1;display:grid;gap:16px;padding:18px;border:1px solid rgb(var(--yb-brand-border));border-radius:15px;background:linear-gradient(145deg,rgb(var(--yb-brand-soft)),rgb(var(--yb-surface)) 60%)}
header{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}
header h3{margin:3px 0 5px;font-size:18px}
header p{max-width:760px;margin:0;color:rgb(var(--yb-text-muted));font-size:12px;line-height:1.6}
header>span{flex:0 0 auto;padding:6px 9px;border-radius:999px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-muted));font-size:11px;font-weight:700}
.eyebrow{color:rgb(var(--yb-brand));font-size:10px;font-weight:850;letter-spacing:.08em}
form{display:grid;gap:14px}.field-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:11px}
label{min-width:0;display:grid;align-content:start;gap:6px;color:rgb(var(--yb-text-body));font-size:12px;font-weight:700}.wide{grid-column:span 2}
input,textarea,select{width:100%;min-height:40px;box-sizing:border-box;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:9px 11px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font:inherit;font-weight:500;outline:none}
textarea{resize:vertical;line-height:1.55}input:focus,textarea:focus,select:focus{border-color:rgb(var(--yb-brand));box-shadow:0 0 0 3px rgb(var(--yb-brand-soft))}
footer{display:flex;align-items:center;justify-content:space-between;gap:14px;padding-top:2px}footer p{margin:0;color:rgb(var(--yb-text-muted));font-size:11px}
button{min-height:40px;border:0;border-radius:10px;padding:0 16px;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-weight:760;cursor:pointer}button:disabled{cursor:not-allowed;opacity:.5}
.editor-message{padding:10px 12px;border-radius:10px;font-size:12px}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.success{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}
@media(max-width:1000px){.field-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:620px){header,footer{align-items:stretch;flex-direction:column}.field-grid{grid-template-columns:1fr}.wide{grid-column:auto}footer button{width:100%}}
</style>
