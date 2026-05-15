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
.fixed.inset-0 {
  background:
    radial-gradient(circle at 18% 5%, rgba(255, 45, 141, 0.22), transparent 28rem),
    radial-gradient(circle at 86% 0%, rgba(100, 210, 255, 0.16), transparent 30rem),
    rgba(0, 0, 0, 0.66) !important;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.fixed.inset-0 > div {
  border-color: rgba(148, 163, 184, 0.24) !important;
  background:
    linear-gradient(145deg, rgba(22, 31, 47, 0.96), rgba(9, 14, 23, 0.98)) !important;
  color: #dce7f7;
  box-shadow: 0 30px 70px -38px rgba(0, 0, 0, 0.95);
}

h3 {
  color: #f8fbff !important;
}

p {
  color: #b8c4d8 !important;
}

button {
  border-color: rgba(148, 163, 184, 0.24) !important;
  background: rgba(12, 18, 29, 0.74);
  color: #dce7f7 !important;
}

button:hover {
  border-color: rgba(125, 211, 252, 0.44) !important;
  background: rgba(24, 35, 52, 0.94);
}

button:last-child {
  border-color: rgba(255, 255, 255, 0.24) !important;
  background: linear-gradient(105deg, var(--yb-music-pink), var(--yb-music-purple), var(--yb-music-cyan));
  color: #fff !important;
}
</style>
