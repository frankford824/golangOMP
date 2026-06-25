<template>
  <section class="security-page">
    <header class="security-hero">
      <div class="security-hero__icon">
        <ShieldCheck :size="24" aria-hidden="true" />
      </div>
      <div>
        <p>安全设置</p>
        <h1>修改密码</h1>
      </div>
    </header>

    <form class="security-panel" @submit.prevent="submit">
      <label class="password-field">
        <span>旧密码</span>
        <div class="password-input">
          <input v-model="oldPassword" :type="showOld ? 'text' : 'password'" autocomplete="current-password" />
          <button type="button" :aria-label="showOld ? '隐藏旧密码' : '显示旧密码'" @click="showOld = !showOld">
            <EyeOff v-if="showOld" :size="16" aria-hidden="true" />
            <Eye v-else :size="16" aria-hidden="true" />
          </button>
        </div>
      </label>

      <label class="password-field">
        <span>新密码</span>
        <div class="password-input">
          <input v-model="newPassword" :type="showNew ? 'text' : 'password'" autocomplete="new-password" />
          <button type="button" :aria-label="showNew ? '隐藏新密码' : '显示新密码'" @click="showNew = !showNew">
            <EyeOff v-if="showNew" :size="16" aria-hidden="true" />
            <Eye v-else :size="16" aria-hidden="true" />
          </button>
        </div>
      </label>

      <div class="password-strength" :class="`password-strength--${strengthLevel}`">
        <div class="password-strength__bar" aria-hidden="true">
          <span />
        </div>
        <strong>{{ strengthText }}</strong>
      </div>

      <div class="password-rules">
        <span v-for="rule in passwordRules" :key="rule.label" :class="{ 'password-rules__item--ok': rule.ok }">
          <CheckCircle2 :size="14" aria-hidden="true" />
          {{ rule.label }}
        </span>
      </div>

      <label class="password-field">
        <span>确认新密码</span>
        <div class="password-input">
          <input v-model="confirmPassword" :type="showConfirm ? 'text' : 'password'" autocomplete="new-password" />
          <button
            type="button"
            :aria-label="showConfirm ? '隐藏确认密码' : '显示确认密码'"
            @click="showConfirm = !showConfirm"
          >
            <EyeOff v-if="showConfirm" :size="16" aria-hidden="true" />
            <Eye v-else :size="16" aria-hidden="true" />
          </button>
        </div>
      </label>

      <p v-if="warningText" class="security-message security-message--error">{{ warningText }}</p>
      <p v-else-if="message" :class="messageClass">{{ message }}</p>

      <div class="security-actions">
        <button type="button" class="security-secondary" :disabled="submitting" @click="resetForm">清空</button>
        <button type="submit" class="security-primary" :disabled="submitting || !canSubmit">
          <KeyRound :size="15" aria-hidden="true" />
          {{ submitting ? '提交中' : '确认修改' }}
        </button>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckCircle2, Eye, EyeOff, KeyRound, ShieldCheck } from 'lucide-vue-next'
import { meApi } from '@/services/api/meApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const showOld = ref(false)
const showNew = ref(false)
const showConfirm = ref(false)
const submitting = ref(false)
const message = ref('')
const messageKind = ref<'success' | 'error' | 'muted'>('muted')

const passwordRules = computed(() => [
  { label: '至少 8 位', ok: newPassword.value.length >= 8 },
  { label: '包含字母', ok: /[A-Za-z]/.test(newPassword.value) },
  { label: '包含数字', ok: /\d/.test(newPassword.value) },
])

const passedRuleCount = computed(() => passwordRules.value.filter((rule) => rule.ok).length)
const strengthLevel = computed(() => {
  if (!newPassword.value) return 'empty'
  if (passedRuleCount.value <= 1) return 'weak'
  if (passedRuleCount.value === 2) return 'medium'
  return 'strong'
})
const strengthText = computed(() => {
  if (!newPassword.value) return '未输入新密码'
  if (strengthLevel.value === 'weak') return '强度偏弱'
  if (strengthLevel.value === 'medium') return '强度中等'
  return '强度良好'
})

const warningText = computed(() => {
  if (!newPassword.value && !confirmPassword.value) return ''
  if (newPassword.value && passedRuleCount.value < passwordRules.value.length) return '新密码需至少 8 位，并包含字母和数字'
  if (confirmPassword.value && confirmPassword.value !== newPassword.value) return '两次输入的新密码不一致'
  if (oldPassword.value && newPassword.value && oldPassword.value === newPassword.value) return '新密码不能与旧密码相同'
  return ''
})

const canSubmit = computed(
  () => !!oldPassword.value && !!newPassword.value && !!confirmPassword.value && !warningText.value,
)
const messageClass = computed(() => ({
  'security-message': true,
  'security-message--success': messageKind.value === 'success',
  'security-message--error': messageKind.value === 'error',
}))

function resetForm(): void {
  oldPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  message.value = ''
  messageKind.value = 'muted'
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return
  submitting.value = true
  message.value = ''
  messageKind.value = 'muted'
  try {
    await meApi.changePassword({
      old_password: oldPassword.value,
      new_password: newPassword.value,
      confirm: confirmPassword.value,
    })
    resetForm()
    message.value = '密码已修改'
    messageKind.value = 'success'
  } catch (error) {
    message.value = resolveApiUserMessage(error, { fallback: '密码修改失败' })
    messageKind.value = 'error'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.security-page {
  display: flex;
  max-width: 760px;
  flex-direction: column;
  gap: 16px;
}

.security-hero,
.security-panel {
  border: 1px solid var(--v1-border);
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 16px 36px -28px rgba(15, 23, 42, 0.28);
}

.security-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 20px;
}

.security-hero__icon {
  display: grid;
  width: 46px;
  height: 46px;
  place-items: center;
  border-radius: 14px;
  background: #eff6ff;
  color: #2563eb;
}

.security-hero p {
  margin: 0;
  font-size: 12px;
  font-weight: 800;
  color: #2563eb;
}

.security-hero h1 {
  margin: 3px 0 0;
  color: var(--v1-text-primary);
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0;
}

.security-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
}

.password-field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.password-field > span {
  color: #475569;
  font-size: 12px;
  font-weight: 800;
}

.password-input {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 40px;
  min-height: 42px;
  overflow: hidden;
  border: 1px solid #dbe3ef;
  border-radius: 9px;
  background: #fff;
}

.password-input:focus-within {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

.password-input input {
  min-width: 0;
  border: 0;
  padding: 0 12px;
  background: transparent;
  color: var(--v1-text-primary);
  font-size: 14px;
  outline: none;
}

.password-input button {
  display: grid;
  place-items: center;
  border: 0;
  border-left: 1px solid #e2e8f0;
  background: #f8fafc;
  color: #475569;
  cursor: pointer;
}

.password-input button:hover,
.password-input button:focus-visible {
  background: #eff6ff;
  color: #2563eb;
  outline: none;
}

.password-strength {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
}

.password-strength__bar {
  height: 7px;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.password-strength__bar span {
  display: block;
  width: 0;
  height: 100%;
  border-radius: inherit;
  background: #94a3b8;
  transition:
    width 0.2s ease,
    background-color 0.2s ease;
}

.password-strength strong {
  min-width: 72px;
  color: var(--v1-text-secondary);
  font-size: 12px;
  text-align: right;
}

.password-strength--weak .password-strength__bar span {
  width: 34%;
  background: #f97316;
}

.password-strength--medium .password-strength__bar span {
  width: 67%;
  background: #eab308;
}

.password-strength--strong .password-strength__bar span {
  width: 100%;
  background: #059669;
}

.password-rules {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.password-rules span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 999px;
  background: #f1f5f9;
  padding: 5px 9px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.password-rules__item--ok {
  background: #ecfdf5 !important;
  color: #047857 !important;
}

.security-message {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--v1-text-secondary);
}

.security-message--success {
  color: #047857;
}

.security-message--error {
  color: var(--v1-danger);
}

.security-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 4px;
}

.security-primary,
.security-secondary {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 8px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 800;
  cursor: pointer;
}

.security-primary {
  border: 1px solid #2563eb;
  background: #2563eb;
  color: #fff;
}

.security-secondary {
  border: 1px solid #dbe3ef;
  background: #fff;
  color: #475569;
}

.security-primary:disabled,
.security-secondary:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.security-primary:hover:not(:disabled),
.security-primary:focus-visible:not(:disabled) {
  border-color: #1d4ed8;
  background: #1d4ed8;
  outline: none;
}

.security-secondary:hover:not(:disabled),
.security-secondary:focus-visible:not(:disabled) {
  background: #f8fafc;
  outline: none;
}

@media (max-width: 640px) {
  .security-actions,
  .password-strength {
    grid-template-columns: 1fr;
  }

  .security-actions {
    flex-direction: column-reverse;
  }

  .security-primary,
  .security-secondary {
    width: 100%;
  }

  .password-strength strong {
    text-align: left;
  }
}
</style>
