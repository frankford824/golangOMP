<template>
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40">
    <div class="w-full max-w-md rounded-xl border border-[var(--v1-border)] bg-white p-4">
      <h3 class="text-sm font-semibold text-[var(--v1-text-primary)]">终止任务</h3>
      <textarea
        v-model="reason"
        class="mt-3 h-24 w-full rounded-lg border border-[var(--v1-border)] px-2 py-1.5 text-sm"
        placeholder="请输入终止原因（必填）"
      />
      <p v-if="errorText" class="mt-2 text-xs text-[var(--v1-danger)]">{{ errorText }}</p>
      <p v-if="suggestForceClose" class="mt-2 text-xs text-[var(--v1-text-secondary)]">
        当前任务存在已接单等限制，无法直接终止。可改用「强制终止」。
      </p>
      <p
        v-else-if="showDirectForce"
        class="mt-2 text-xs text-[var(--v1-text-secondary)]"
      >
        部门管理员可直接「强制终止」，将忽略接单等业务限制（仍会计入审计）。
      </p>
      <div class="mt-3 flex flex-wrap justify-end gap-2">
        <button
          type="button"
          class="rounded-md border border-[var(--v1-border)] px-3 py-1 text-xs"
          @click="$emit('close')"
        >
          取消
        </button>
        <button
          v-if="showDirectForce || suggestForceClose"
          type="button"
          class="rounded-md border border-red-200 bg-red-50 px-3 py-1 text-xs text-red-700"
          @click="$emit('force', reason)"
        >
          强制终止
        </button>
        <button
          type="button"
          class="rounded-md bg-amber-500 px-3 py-1 text-xs text-white"
          :disabled="Boolean(suggestForceClose)"
          @click="$emit('submit', reason)"
        >
          确认终止
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  errorText?: string
  /** 有模块已接单等 409 时，引导使用强制 */
  suggestForceClose?: boolean
  /** 部门管理员：在未 409 时也可选「强制终止」 */
  showDirectForce?: boolean
}>()
defineEmits<{ close: []; submit: [string]; force: [string] }>()

const reason = ref('')
</script>
