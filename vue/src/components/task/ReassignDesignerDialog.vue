<template>
  <BaseModal
    :model-value="modelValue"
    title="重新指派设计师"
    :show-confirm="false"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template #default>
      <div v-if="step === 'form'" class="reassign-body space-y-4">
        <p class="intro-copy text-sm leading-relaxed">
          这是任务级调度动作：用于在设计阶段调整负责人。确认后将由
          <strong class="intro-copy-strong">新设计师</strong>继续负责该任务。
        </p>
        <div
          v-if="hasDesignOutputHint"
          class="text-sm text-amber-900 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2.5 space-y-1.5"
        >
          <p class="font-semibold m-0">风险提示</p>
          <p class="m-0">当前任务可能已存在设计稿/版本记录，请先与原设计师沟通确认后再重新指派。</p>
          <p class="m-0">重新指派后，新设计师将在现有任务基础上继续处理。</p>
        </div>

        <div class="field-block">
          <span class="field-label">当前设计师</span>
          <p class="field-readonly">{{ currentAssigneeName || '—' }}</p>
        </div>

        <div class="field-block">
          <BaseSelect
            v-model="reasonCode"
            class="reassign-select"
            label="转派原因"
            placeholder="请选择原因"
            :options="reasonOptions"
          />
        </div>

        <div class="field-block">
          <label class="field-label-text">原因说明</label>
          <textarea
            v-model="reasonNote"
            class="reason-textarea"
            rows="3"
            placeholder="补充说明（选「其他」时必填）"
          />
        </div>

        <div class="field-block">
          <BaseSelect
            v-model="selectedId"
            class="reassign-select"
            label="新设计师"
            :placeholder="loading ? '加载设计师列表...' : '请选择新设计师'"
            :disabled="loading"
            :options="designerSelectOptions"
            filterable
            filter-placeholder="输入姓名筛选"
            :error="!loading && !selectableDesigners.length ? '暂无可选设计师（已排除当前负责人）' : ''"
          />
        </div>

        <p v-if="formError" class="form-error">{{ formError }}</p>
      </div>

      <div v-else class="reassign-body space-y-3">
        <p v-if="pendingConfirm?.mode === 'clear'" class="confirm-copy-primary text-sm leading-relaxed">
          确认清空任务当前指派人并退回
          <strong>待指派</strong>
          状态吗？
        </p>
        <p v-else class="confirm-copy-primary text-sm leading-relaxed">
          确认将该任务从 <strong>{{ currentAssigneeName || '当前负责人' }}</strong> 重新指派给
          <strong>{{ pendingConfirm?.assigneeName }}</strong> 吗？
        </p>
        <p v-if="hasDesignOutputHint" class="text-sm text-amber-800 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
          当前任务可能已有设计产出，请确认已与原设计师沟通后再继续。重新指派后，新设计师将继续在现有任务基础上处理。
        </p>
        <p v-else-if="pendingConfirm?.mode === 'clear'" class="confirm-copy-secondary text-sm">
          清空后任务将回到待指派，需要重新指定设计负责人。
        </p>
        <p v-else class="confirm-copy-secondary text-sm">确认后由新设计师负责后续设计推进。</p>
        <p class="confirm-copy-secondary text-sm">
          转派原因：<span class="confirm-reason-label font-medium">{{ pendingConfirm?.reasonLabel }}</span>
          <template v-if="pendingConfirm?.reasonNote">
            · {{ pendingConfirm.reasonNote }}
          </template>
        </p>
      </div>
    </template>
    <template #footer>
      <footer class="flex-shrink-0 flex justify-end gap-2 px-5 py-4 border-t border-slate-100">
        <template v-if="step === 'form'">
          <BaseButton size="sm" variant="secondary" @click="close">取消</BaseButton>
          <BaseButton
            size="sm"
            variant="ghost"
            :disabled="loading || !currentAssigneeId"
            @click="goConfirmClear"
          >
            清空指派（退回待指派）
          </BaseButton>
          <BaseButton size="sm" variant="primary" :disabled="loading" @click="goConfirm">
            下一步：确认
          </BaseButton>
        </template>
        <template v-else>
          <BaseButton size="sm" variant="secondary" @click="step = 'form'">上一步</BaseButton>
          <BaseButton size="sm" variant="primary" @click="submitConfirm">
            {{ pendingConfirm?.mode === 'clear' ? '确认清空指派' : '确认重新指派' }}
          </BaseButton>
        </template>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import type { Designer } from '@/mock/designers'

const REASON_OTHER = 'other'

const reasonOptions = [
  { value: 'overload', label: '当前设计师任务过载' },
  { value: 'leave', label: '当前设计师请假/离岗' },
  { value: 'priority', label: '任务优先级调整' },
  { value: 'skill', label: '专业方向不匹配' },
  { value: 'lead_schedule', label: '设计组长调度' },
  { value: REASON_OTHER, label: '其他' },
]

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    designers: Designer[]
    loading?: boolean
    currentAssigneeId?: string | null
    currentAssigneeName?: string | null
    hasDesignOutputHint?: boolean
  }>(),
  {
    loading: false,
    hasDesignOutputHint: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [
    payload: {
      mode: 'reassign' | 'clear'
      assigneeId: string | null
      assigneeName: string | null
      reasonCode: string
      reasonLabel: string
      reasonNote: string
    },
  ]
}>()

const step = ref<'form' | 'confirm'>('form')
const reasonCode = ref<string>('')
const reasonNote = ref('')
const selectedId = ref<string | number>('')
const formError = ref('')

const selectableDesigners = computed(() => {
  const cur = props.currentAssigneeId != null ? String(props.currentAssigneeId) : ''
  return props.designers.filter((d) => String(d.id) !== cur)
})

const designerSelectOptions = computed(() =>
  selectableDesigners.value.map((d) => ({ value: d.id, label: d.name })),
)

const selectedDesigner = computed(() =>
  selectableDesigners.value.find((d) => d.id === String(selectedId.value)) ?? null,
)

const pendingConfirm = ref<{
  mode: 'reassign' | 'clear'
  assigneeId: string | null
  assigneeName: string | null
  reasonLabel: string
  reasonNote: string
} | null>(null)

function resetForm() {
  step.value = 'form'
  reasonCode.value = ''
  reasonNote.value = ''
  selectedId.value = ''
  formError.value = ''
  pendingConfirm.value = null
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) resetForm()
  },
)

function close() {
  emit('update:modelValue', false)
}

function goConfirm() {
  formError.value = ''
  if (!reasonCode.value) {
    formError.value = '请选择转派原因'
    return
  }
  if (reasonCode.value === REASON_OTHER && !reasonNote.value.trim()) {
    formError.value = '选择「其他」时请填写原因说明'
    return
  }
  if (!selectedDesigner.value) {
    formError.value = '请选择新设计师'
    return
  }
  const label =
    reasonOptions.find((o) => o.value === reasonCode.value)?.label ?? reasonCode.value
  pendingConfirm.value = {
    mode: 'reassign',
    assigneeId: selectedDesigner.value.id,
    assigneeName: selectedDesigner.value.name,
    reasonLabel: label,
    reasonNote: reasonNote.value.trim(),
  }
  step.value = 'confirm'
}

function goConfirmClear() {
  formError.value = ''
  if (!props.currentAssigneeId) {
    formError.value = '当前任务没有可清空的指派人'
    return
  }
  if (!reasonCode.value) {
    formError.value = '请选择转派原因'
    return
  }
  if (reasonCode.value === REASON_OTHER && !reasonNote.value.trim()) {
    formError.value = '选择「其他」时请填写原因说明'
    return
  }
  const label =
    reasonOptions.find((o) => o.value === reasonCode.value)?.label ?? reasonCode.value
  pendingConfirm.value = {
    mode: 'clear',
    assigneeId: null,
    assigneeName: null,
    reasonLabel: label,
    reasonNote: reasonNote.value.trim(),
  }
  step.value = 'confirm'
}

function submitConfirm() {
  const p = pendingConfirm.value
  if (!p) return
  emit('confirm', {
    mode: p.mode,
    assigneeId: p.assigneeId,
    assigneeName: p.assigneeName,
    reasonCode: reasonCode.value,
    reasonLabel: p.reasonLabel,
    reasonNote: p.reasonNote,
  })
  emit('update:modelValue', false)
}
</script>

<style scoped>
.reassign-body {
  max-width: 28rem;
}
.field-block {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.field-label,
.field-label-text {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.field-readonly {
  margin: 0;
  font-size: 0.875rem;
  color: #0f172a;
  padding: 0.5rem 0.75rem;
  background: #f8fafc;
  border-radius: 0.375rem;
  border: 1px solid #e2e8f0;
}
.reason-textarea {
  width: 100%;
  box-sizing: border-box;
  font-size: 0.875rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  color: #111827;
  resize: vertical;
  font-family: inherit;
}

.reason-textarea::placeholder {
  color: #9ca3af;
}

.reason-textarea:disabled {
  border-color: #d1d5db;
  background: #f3f4f6;
  color: #6b7280;
}
.reason-textarea:focus {
  outline: none;
  border-color: #93c5fd;
  box-shadow: 0 0 0 1px #bfdbfe;
}
.form-error {
  margin: 0;
  font-size: 0.8125rem;
  color: #b91c1c;
}

.intro-copy {
  color: #374151;
}

.intro-copy-strong {
  color: #111827;
}

.confirm-copy-primary {
  color: #111827;
}

.confirm-copy-secondary {
  color: #4b5563;
}

.confirm-reason-label {
  color: #111827;
}

.reassign-select :deep(label) {
  color: #6b7280;
}

.reassign-select :deep(.h-11) {
  border-color: #d1d5db;
  background: #ffffff;
  color: #111827;
}

.reassign-select :deep(.h-11 > button) {
  color: inherit;
}

.reassign-select :deep(.h-11 > button:disabled) {
  color: #6b7280 !important;
}

.reassign-select :deep(.h-11 .text-slate-500) {
  color: #6b7280 !important;
}

.reassign-select :deep(.h-11 .text-slate-400) {
  color: #9ca3af !important;
}

.reassign-select :deep(.cursor-not-allowed) {
  background: #f3f4f6 !important;
  color: #6b7280 !important;
  opacity: 1 !important;
}
</style>
