<template>
  <div class="sku-preview-card">
    <h4 class="card-title">SKU 预览</h4>
    <p v-if="!hasEffectiveRule" class="hint danger">规则未配置，请联系管理员</p>
    <template v-else>
      <BaseSelect
        v-model="selectedRuleId"
        placeholder="选择产品编码规则"
        class="rule-select"
        :options="ruleOptions"
      />
      <BaseButton
        variant="primary"
        class="gen-btn"
        :disabled="!selectedRuleId"
        @click="generate"
      >
        预生成 SKU
      </BaseButton>
      <div v-if="previewSku" class="preview-result">
        <span class="label">生成结果：</span>
        <span class="value">{{ previewSku }}</span>
      </div>
      <p v-else class="hint">选择规则后点击「预生成 SKU」</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, withDefaults } from 'vue'
import { useRulesStore } from '@/stores/rules'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = withDefaults(
  defineProps<{
    modelValue: string | null
    /** 与当前选中的编码规则 id 同步，便于批量模式从模板预生成多 SKU */
    ruleId?: string | null
  }>(),
  { ruleId: null },
)
const emit = defineEmits<{
  'update:modelValue': [string | null]
  'update:ruleId': [string | null]
}>()

const rulesStore = useRulesStore()
const skuRules = computed(() => rulesStore.skuRules)
const hasEffectiveRule = computed(() => skuRules.value.length > 0)
const ruleOptions = computed(() => skuRules.value.map((r) => ({ value: r.id, label: r.name })))
const selectedRuleId = ref<string>(props.ruleId ?? '')
const previewSku = ref<string | null>(props.modelValue)

onMounted(() => rulesStore.loadRules())

watch(
  () => props.ruleId,
  (v) => {
    const next = v ?? ''
    if (next !== selectedRuleId.value) selectedRuleId.value = next
  },
  { immediate: true },
)
watch(selectedRuleId, (v) => emit('update:ruleId', v || null))
watch(() => props.modelValue, (v) => { previewSku.value = v }, { immediate: true })
watch(previewSku, (v) => emit('update:modelValue', v))

function generate() {
  if (!selectedRuleId.value) return
  const sku = rulesStore.generatePreviewSku(selectedRuleId.value)
  previewSku.value = sku
}
</script>

<style scoped>
.sku-preview-card {
  padding: 1rem;
  background: rgb(var(--yb-surface-subtle));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 8px;
}
.card-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}
.rule-select {
  margin-bottom: 0.75rem;
}
.gen-btn {
  margin-bottom: 0.75rem;
}
.preview-result {
  font-size: 0.875rem;
}
.preview-result .label {
  color: rgb(var(--yb-text-muted-strong));
}
.preview-result .value {
  font-weight: 600;
  color: rgb(var(--yb-success-emerald));
  margin-left: 0.25rem;
}
.hint {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-placeholder));
}
.hint.danger {
  color: rgb(var(--yb-danger));
}
</style>
