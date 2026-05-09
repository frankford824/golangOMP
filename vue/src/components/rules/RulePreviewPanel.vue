<template>
  <div class="preview-panel">
    <h4>规则预览</h4>
    <p v-if="!previewResult" class="hint">选择规则后点击「生成样例」</p>
    <div v-else class="preview-result">
      <span class="label">生成样例：</span>
      <span class="value">{{ previewResult }}</span>
    </div>
    <BaseButton
      size="sm"
      variant="primary"
      :disabled="!canGenerate"
      @click="generate"
    >
      生成样例
    </BaseButton>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { CodeRule } from '@/types'
import { useRulesStore } from '@/stores/rules'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{ rule: CodeRule | null }>()
const rulesStore = useRulesStore()

const previewResult = ref('')

const canGenerate = computed(() => {
  const r = props.rule
  if (!r) return false
  if (!r.prefix?.trim()) return false
  if (!r.dateFormat?.trim()) return false
  const n = r.sequenceDigits
  if (n < 1 || n > 6) return false
  return true
})

watch(() => props.rule, () => { previewResult.value = '' }, { immediate: true })

function generate() {
  if (!props.rule || !canGenerate.value) return
  if (props.rule.ruleType === 'sku') {
    previewResult.value = rulesStore.previewSkuWithRule(props.rule)
  } else {
    previewResult.value = rulesStore.previewCode(props.rule, {})
  }
}
</script>

<style scoped>
.preview-panel {
  padding: 1rem;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.preview-panel h4 {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.hint {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  color: #94a3b8;
}
.preview-result {
  margin-bottom: 0.5rem;
  font-size: 0.875rem;
}
.preview-result .label {
  color: #64748b;
}
.preview-result .value {
  font-weight: 600;
  color: #059669;
  margin-left: 0.25rem;
}
</style>
