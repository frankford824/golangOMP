<script setup lang="ts">
import { ref, watch } from 'vue'
import { oaRequestError } from '../requestErrors'

const copyStatus = ref('')
const traceField = ref<HTMLInputElement>()
watch(oaRequestError, () => { copyStatus.value = '' })

async function copyTrace() {
  const entry = oaRequestError.value
  if (!entry?.traceId) return
  try {
    await navigator.clipboard.writeText(entry.traceId)
    if (entry === oaRequestError.value) copyStatus.value = '已复制追踪号'
  } catch {
    if (entry !== oaRequestError.value) return
    traceField.value?.focus()
    traceField.value?.select()
    copyStatus.value = '已选中追踪号，请手动复制'
  }
}
</script>

<template>
  <aside v-if="oaRequestError" class="oa-request-error" aria-label="请求失败详情">
    <div role="status">
      <p>{{ oaRequestError.message }}</p>
      <p v-if="copyStatus" class="copy-status">{{ copyStatus }}</p>
    </div>
    <div class="trace-controls">
      <input ref="traceField" readonly aria-label="追踪号" :value="oaRequestError.traceId">
      <button type="button" @click="copyTrace">复制追踪号</button>
      <button type="button" @click="oaRequestError = undefined">关闭</button>
    </div>
  </aside>
</template>

<style scoped>
.oa-request-error {
  position: fixed;
  z-index: 3000;
  inset-inline-end: 16px;
  inset-block-end: 16px;
  width: min(416px, calc(100% - 32px));
  padding: 12px 16px;
  color: var(--oa-ink);
  background: var(--oa-surface);
  border: 1px solid var(--yb-line-structure);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgb(var(--yb-overlay-night) / .14);
}
.oa-request-error p { margin: 0 0 8px; overflow-wrap:anywhere; }
.copy-status { color: var(--oa-secondary); font-size: 13px; }
.trace-controls { display: flex; align-items:center; gap: 8px; flex-wrap: wrap; }
.trace-controls input {
  flex: 1 1 248px;
  min-width: 0;
  height: 32px;
  padding-inline: 8px;
  font: 13px var(--yb-font-mono);
  background: var(--oa-surface);
  border: 1px solid var(--yb-line-control);
  border-radius: 6px;
}
.trace-controls button { min-height: 32px; padding: 0 12px; color: var(--oa-accent); background: var(--oa-surface); border: 1px solid var(--yb-line-control); border-radius: 6px; cursor: pointer; }
.trace-controls button:hover { border-color: var(--oa-accent); }
</style>
