<template>
  <Teleport to="body">
    <div v-if="open" class="close-draft-confirm-overlay fixed inset-0 z-[7200] flex items-center justify-center">
      <div
        class="close-draft-confirm-panel w-full max-w-sm rounded-xl border p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="close-draft-confirm-title"
      >
        <h3 id="close-draft-confirm-title" class="text-sm font-semibold">表单未提交</h3>
        <p class="mt-2 text-xs">
          检测到已填写字段，是否保存为草稿？草稿最多保留 20 条，7 天自动清理。
        </p>
        <div class="mt-4 flex justify-end gap-2">
          <button
            type="button"
            class="close-draft-confirm-action rounded px-3 py-1 text-xs"
            @click="$emit('cancel')"
          >
            取消
          </button>
          <button
            type="button"
            class="close-draft-confirm-action is-danger rounded px-3 py-1 text-xs"
            @click="$emit('discard')"
          >
            丢弃
          </button>
          <button
            type="button"
            class="close-draft-confirm-action is-primary rounded px-3 py-1 text-xs"
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
/* Phase 4: light confirm dialog - align with BaseModal overlay/panel. Style-only. */
.close-draft-confirm-overlay {
  background: rgb(var(--yb-shadow) / 0.45);
}

.close-draft-confirm-panel {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: 0 10px 40px rgb(var(--yb-shadow) / 0.12);
}

h3 {
  color: rgb(var(--yb-text));
}

p {
  color: rgb(var(--yb-text-muted));
}

.close-draft-confirm-action {
  border: 1px solid rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  border-radius: 0.375rem;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease;
}

.close-draft-confirm-action:hover {
  border-color: rgb(var(--yb-text-faint));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.close-draft-confirm-action.is-danger {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}

.close-draft-confirm-action.is-danger:hover {
  border-color: rgb(var(--yb-danger-border-hover));
  background: rgb(var(--yb-danger-soft-hover));
  color: rgb(var(--yb-danger-deep));
}

.close-draft-confirm-action.is-primary {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
}

.close-draft-confirm-action.is-primary:hover {
  border-color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-strong));
  color: rgb(var(--yb-text-inverse));
}
</style>
