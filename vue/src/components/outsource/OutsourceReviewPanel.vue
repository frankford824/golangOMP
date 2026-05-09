/**
 * 组件职责：定制回传审核面板，查看回传资产 + 执行复核通过/打回
 * 
 * 核心业务规则（来自 Prompt.md）：
 *   - 定制回传后由审核员B/复核员复核
 *   - 通过后进入仓库接收
 * 
 * 主要 Store：useOutsourceStore
 * 当前主链：外包创建由 POST /v1/tasks/{id}/outsource 触发，回传审核跟随任务审核链路
 * 
 * 当前状态：已迁移 Base 组件，面板交互完整
 * 维护注意 / 风险点：
 *   - 回传资产版本需对比 hash
 *   - 复核后状态变更需触发仓库接收
 */
<template>
  <div class="review-panel">
    <h4>回传复核</h4>
    <div class="form-fields">
      <label class="label">复核结果</label>
      <div class="radio-group">
        <label class="radio-item">
          <input v-model="result" type="radio" value="passed" />
          <span>通过</span>
        </label>
        <label class="radio-item">
          <input v-model="result" type="radio" value="rejected" />
          <span>打回定制</span>
        </label>
      </div>
      <BaseTextarea v-model="note" label="复核备注" :rows="3" placeholder="补充备注" />
      <BaseButton variant="primary" size="sm" :disabled="!order" @click="submit">提交复核</BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { OutsourceOrder } from '@/types'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{ order: OutsourceOrder | null }>()
const emit = defineEmits<{ submit: [{ result: 'passed' | 'rejected'; note: string }] }>()

const result = ref<'passed' | 'rejected'>('passed')
const note = ref('')

watch(() => props.order, () => { result.value = 'passed'; note.value = '' }, { immediate: true })

function submit() {
  if (!props.order) return
  emit('submit', { result: result.value, note: note.value })
}
</script>

<style scoped>
.review-panel {
  padding: 1rem;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.review-panel h4 {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.form-fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #334155;
}
.radio-group {
  display: flex;
  gap: 1rem;
}
.radio-item {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.875rem;
  cursor: pointer;
}
</style>
