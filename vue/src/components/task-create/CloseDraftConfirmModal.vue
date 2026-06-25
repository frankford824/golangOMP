<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[7200] flex items-center justify-center bg-slate-900/40">
      <div class="w-full max-w-sm rounded-xl border border-[var(--v1-border)] bg-white p-4">
        <h3 class="text-sm font-semibold text-[var(--v1-text-primary)]">表单未提交</h3>
        <p class="mt-2 text-xs text-[var(--v1-text-secondary)]">
          检测到已填写字段，是否保存为草稿？草稿最多保留 20 条，7 天自动清理。
        </p>
        <div class="mt-4 flex justify-end gap-2">
          <button
            type="button"
            class="rounded border border-[var(--v1-border)] px-3 py-1 text-xs text-[var(--v1-text-secondary)]"
            @click="$emit('cancel')"
          >
            取消
          </button>
          <button
            type="button"
            class="rounded border border-[var(--v1-border)] px-3 py-1 text-xs text-red-600"
            @click="$emit('discard')"
          >
            丢弃
          </button>
          <button
            type="button"
            class="rounded bg-[var(--v1-bg-primary)] px-3 py-1 text-xs text-white"
            @click="$emit('save')"
          >
            保存为草稿
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{ open: boolean }>()
defineEmits<{ save: []; discard: []; cancel: [] }>()
</script>

<style scoped>
/* Phase 4: light confirm dialog — align with BaseModal overlay/panel. Style-only. */
.fixed.inset-0 {
  background: rgba(15, 23, 42, 0.45) !important;
}

.fixed.inset-0 > div {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #111827;
  box-shadow: 0 10px 40px rgba(15, 23, 42, 0.12);
}

h3 {
  color: #111827 !important;
}

p {
  color: #6b7280 !important;
}

.fixed.inset-0 > div .flex button {
  border: 1px solid #d1d5db !important;
  background: #ffffff !important;
  color: #374151 !important;
  border-radius: 0.375rem;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease;
}

.fixed.inset-0 > div .flex button:hover {
  border-color: #9ca3af !important;
  background: #f9fafb !important;
  color: #111827 !important;
}

.fixed.inset-0 > div .flex button.text-red-600 {
  border-color: #fecaca !important;
  background: #fef2f2 !important;
  color: #b91c1c !important;
}

.fixed.inset-0 > div .flex button.text-red-600:hover {
  border-color: #fca5a5 !important;
  background: #fee2e2 !important;
  color: #991b1b !important;
}

.fixed.inset-0 > div .flex button:last-child {
  border-color: #2563eb !important;
  background: #2563eb !important;
  color: #ffffff !important;
}

.fixed.inset-0 > div .flex button:last-child:hover {
  border-color: #1d4ed8 !important;
  background: #1d4ed8 !important;
  color: #ffffff !important;
}
</style>
