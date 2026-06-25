/**
 * 组件职责：审核动作面板，提供通过/打回/意见填写 + 交班/转派按钮
 *
 * 核心业务规则（来自 Prompt.md）：
 *   - 审核支持交班/转派/接手
 *   - 危险动作需填写 reason + 二次确认
 *
 * 主要 Store：useAuditStore
 * 预留接口：POST /api/audits (mock)
 *
 * 当前状态：已迁移 Base 组件，按钮交互完整
 * 维护注意 / 风险点：
 *   - reason 必填 + 二次确认逻辑必须保留
 *   - 危险操作需记录审计日志
 */
<template>
  <div class="audit-action-panel">
    <h4>审核意见</h4>
    <p v-if="contextHint" class="audit-context-hint">{{ contextHint }}</p>
    <p class="audit-rule-hint">单次审核：打回必填问题分类/审核说明；通过备注选填</p>
    <div class="decision-mode">
      <span class="decision-label">处理结果</span>
      <div class="decision-seg">
        <button
          type="button"
          class="decision-seg-btn decision-seg-btn-reject"
          :class="{ 'decision-seg-btn-active': decisionMode === 'reject' }"
          @click="decisionMode = 'reject'"
        >
          打回
        </button>
        <button
          type="button"
          class="decision-seg-btn decision-seg-btn-pass"
          :class="{ 'decision-seg-btn-active': decisionMode === 'pass' }"
          @click="decisionMode = 'pass'"
        >
          通过
        </button>
      </div>
    </div>
    <div class="form-fields">
      <div class="field-block">
        <BaseSelect
          v-model="form.problemCategory"
          :label="decisionMode === 'reject' ? '问题分类 *' : '问题分类（选填）'"
          placeholder="选择"
          :options="problemOptions"
        />
      </div>
      <div class="field-block">
        <BaseTextarea
          v-model="form.comment"
          :label="decisionMode === 'reject' ? '审核说明 *' : '通过备注（选填）'"
          :rows="3"
          :placeholder="decisionMode === 'reject' ? '请填写打回原因与修改建议' : '通过时可补充说明（选填）'"
        />
      </div>
      <label class="checkbox-row">
        <input v-model="form.affectLaunch" type="checkbox" class="mr-2" />
        <span>影响上线</span>
      </label>
      <p
        v-if="decisionMode === 'pass'"
        class="pass-state-hint"
      >
        通过态默认不要求问题分类，可直接审核通过。
      </p>
    </div>
    <p v-if="disabledReason" class="disabled-reason">{{ disabledReason }}</p>
    <p v-if="integrationHint" class="disabled-reason">{{ integrationHint }}</p>
    <div class="action-buttons">
      <BaseButton variant="primary" size="sm" :disabled="actionsDisabled" @click="onPass">审核通过</BaseButton>
      <BaseButton size="sm" variant="secondary" :disabled="actionsDisabled" @click="onReject">打回修改</BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || handoverDisabled"
        :title="handoverDisabled ? integrationHint : ''"
        @click="$emit('handover')"
      >
        发起交班
      </BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || transferDisabled"
        :title="transferDisabled ? integrationHint : ''"
        @click="$emit('transfer')"
      >
        转交
      </BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || takeoverDisabled"
        :title="takeoverDisabled ? integrationHint : ''"
        @click="$emit('takeover')"
      >
        接手交班任务
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseButton from '@/components/base/BaseButton.vue'

withDefaults(defineProps<{
  actionsDisabled?: boolean
  disabledReason?: string
  handoverDisabled?: boolean
  transferDisabled?: boolean
  takeoverDisabled?: boolean
  integrationHint?: string
  contextHint?: string
}>(), {
  actionsDisabled: false,
  disabledReason: '',
  handoverDisabled: false,
  transferDisabled: false,
  takeoverDisabled: false,
  integrationHint: '',
  contextHint: '',
})

const problemOptions = [
  { value: 'copy', label: '文案错误' },
  { value: 'size', label: '尺寸不符' },
  { value: 'style', label: '风格偏差' },
  { value: 'other', label: '其他' },
]

const form = reactive({
  problemCategory: '',
  comment: '',
  affectLaunch: false,
})

const decisionMode = ref<'pass' | 'reject'>('reject')

const emit = defineEmits<{ pass: []; reject: []; handover: []; transfer: []; takeover: [] }>()

function onPass() {
  decisionMode.value = 'pass'
  emit('pass')
}

function onReject() {
  decisionMode.value = 'reject'
  emit('reject')
}

defineExpose({ form })
</script>

<style scoped>
.audit-rule-hint {
  margin: 0 0 0.5rem;
  font-size: 0.6875rem;
  color: rgb(var(--yb-text-muted-strong));
  line-height: 1.45;
}
.decision-mode {
  margin-bottom: 0.625rem;
}
.decision-label {
  display: block;
  margin-bottom: 0.35rem;
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-soft));
}
.decision-seg {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.45rem;
}
.decision-seg-btn {
  height: 1.9rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface-subtle));
  font-size: 0.6875rem;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, color 0.12s;
}
.decision-seg-btn-reject {
  color: rgb(var(--yb-danger-text));
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-wash));
}
.decision-seg-btn-pass {
  color: rgb(var(--yb-success-teal));
  border-color: rgb(var(--yb-success-border-soft));
  background: rgb(var(--yb-success-soft));
}
.decision-seg-btn-active.decision-seg-btn-reject {
  background: rgb(var(--yb-danger-soft-hover));
  border-color: rgb(var(--yb-danger-soft-red));
}
.decision-seg-btn-active.decision-seg-btn-pass {
  background: rgb(var(--yb-surface-success-strong));
  border-color: rgb(var(--yb-success-emerald-light));
}
.audit-context-hint {
  margin: 0 0 0.5rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  line-height: 1.45;
}
.audit-action-panel {
  padding: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 10px;
}
.audit-action-panel h4 {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  font-weight: 700;
  color: rgb(var(--yb-indigo-night));
}
.form-fields {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.field-block {
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface));
  border-radius: 0.625rem;
  padding: 0.45rem 0.5rem;
}
.checkbox-row {
  display: flex;
  align-items: center;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-slate));
  cursor: pointer;
}
.pass-state-hint {
  margin: 0;
  font-size: 0.6875rem;
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-ui-soft));
  border: 1px solid rgb(var(--yb-success-border-bright));
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  line-height: 1.4;
}
.disabled-reason {
  margin: 0.55rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-danger-text));
}
.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.75rem;
}
.action-buttons :deep(button) {
  min-width: 5rem;
}
</style>
