<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[4000] flex items-center justify-center bg-slate-900/40">
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
