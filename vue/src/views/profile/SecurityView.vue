<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">安全设置</h1>
    <div class="mt-3 space-y-2">
      <input v-model="oldPassword" type="password" class="w-full rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm" placeholder="旧密码" />
      <input v-model="newPassword" type="password" class="w-full rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm" placeholder="新密码" />
      <input v-model="confirmPassword" type="password" class="w-full rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm" placeholder="确认新密码" />
      <p v-if="warningText" class="text-xs text-[var(--v1-danger)]">{{ warningText }}</p>
      <p v-if="message" class="text-xs text-[var(--v1-text-secondary)]">{{ message }}</p>
    </div>
    <div class="mt-3 flex justify-end">
      <button
        type="button"
        class="rounded bg-[var(--v1-bg-primary)] px-3 py-1 text-xs text-white disabled:opacity-50"
        :disabled="submitting"
        @click="submit"
      >
        {{ submitting ? '提交中...' : '修改密码' }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { meApi } from '@/services/api/meApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const message = ref('')

const warningText = computed(() => {
  if (!newPassword.value) return ''
  const valid = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/.test(newPassword.value)
  if (!valid) return '新密码需至少8位，且包含字母和数字'
  if (confirmPassword.value && confirmPassword.value !== newPassword.value) return '两次输入的新密码不一致'
  return ''
})

async function submit(): Promise<void> {
  if (warningText.value) return
  if (!oldPassword.value || !newPassword.value || !confirmPassword.value) {
    message.value = '请完整填写旧密码、新密码和确认新密码'
    return
  }
  submitting.value = true
  message.value = ''
  try {
    await meApi.changePassword({
      old_password: oldPassword.value,
      new_password: newPassword.value,
      password_confirmation: confirmPassword.value,
    })
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    message.value = '密码已修改'
  } catch (error) {
    message.value = resolveApiUserMessage(error, { fallback: '密码修改失败' })
  } finally {
    submitting.value = false
  }
}
</script>
