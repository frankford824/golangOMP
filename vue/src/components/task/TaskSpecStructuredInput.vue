<template>
  <div class="structured-spec">
    <label class="field-label">
      {{ label }}
      <span v-if="required" class="required">*</span>
    </label>

    <div class="spec-mode-tabs" role="group" aria-label="规格尺寸填写方式">
      <button
        v-for="opt in modeOptions"
        :key="opt.value"
        type="button"
        class="spec-mode-tab"
        :class="{ active: mode === opt.value }"
        @click="mode = opt.value"
      >
        {{ opt.label }}
      </button>
    </div>

    <div v-if="mode === 'size'" class="spec-size-grid">
      <input
        v-model="widthText"
        class="spec-native-input"
        type="number"
        min="0"
        step="0.01"
        inputmode="decimal"
        placeholder="宽"
      />
      <span class="spec-times">×</span>
      <input
        v-model="heightText"
        class="spec-native-input"
        type="number"
        min="0"
        step="0.01"
        inputmode="decimal"
        placeholder="高"
      />
      <select v-model="sizeUnit" class="spec-native-input spec-unit-select">
        <option v-for="opt in sizeUnitOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </select>
    </div>

    <div v-else-if="mode === 'area'" class="spec-area-grid">
      <input
        v-model="areaText"
        class="spec-native-input"
        type="number"
        min="0"
        step="0.001"
        inputmode="decimal"
        placeholder="面积"
      />
      <select v-model="areaUnit" class="spec-native-input spec-unit-select">
        <option v-for="opt in areaUnitOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </select>
    </div>

    <textarea
      v-else
      v-model="freeText"
      class="spec-native-textarea"
      :rows="2"
      placeholder="特殊尺寸或补充说明，例如 30*42cm（镂空18*18cm）"
    />

    <p class="spec-preview" :class="{ empty: !normalizedSpec }">
      {{ normalizedSpec ? `提交规格：${normalizedSpec}` : placeholder }}
    </p>
    <p v-if="hint" class="form-hint">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

type SpecMode = 'size' | 'area' | 'custom'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    placeholder?: string
    hint?: string
    required?: boolean
  }>(),
  {
    modelValue: '',
    label: '规格尺寸',
    placeholder: '请选择填写方式并输入数字',
    hint: '',
    required: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

const modeOptions: { value: SpecMode; label: string }[] = [
  { value: 'size', label: '宽 × 高' },
  { value: 'area', label: '面积' },
  { value: 'custom', label: '特殊说明' },
]

const sizeUnitOptions = [
  { value: 'cm', label: '厘米' },
  { value: 'm', label: '米' },
  { value: 'mm', label: '毫米' },
]

const areaUnitOptions = [
  { value: '平方米', label: '平方米' },
  { value: '平方厘米', label: '平方厘米' },
  { value: '平方毫米', label: '平方毫米' },
]

const mode = ref<SpecMode>('size')
const widthText = ref('')
const heightText = ref('')
const areaText = ref('')
const sizeUnit = ref('cm')
const areaUnit = ref('平方米')
const freeText = ref('')
const lastEmitted = ref('')

const normalizedSpec = computed(() => {
  if (mode.value === 'size') {
    const width = normalizePositiveNumber(widthText.value)
    const height = normalizePositiveNumber(heightText.value)
    if (!width || !height) return ''
    return `${width}*${height}${sizeUnit.value}`
  }
  if (mode.value === 'area') {
    const area = normalizePositiveNumber(areaText.value)
    if (!area) return ''
    return `${area}${areaUnit.value}`
  }
  return freeText.value.trim()
})

watch(
  () => props.modelValue,
  (value) => {
    const incoming = String(value ?? '').trim()
    if (incoming === lastEmitted.value) return
    hydrateFromText(incoming)
  },
  { immediate: true },
)

watch(normalizedSpec, (value) => {
  lastEmitted.value = value
  emit('update:modelValue', value)
})

function normalizePositiveNumber(value: string): string {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  const n = Number(raw)
  if (!Number.isFinite(n) || n <= 0) return ''
  return String(Number(n.toFixed(3)))
}

function hydrateFromText(value: string): void {
  if (!value) {
    mode.value = 'size'
    widthText.value = ''
    heightText.value = ''
    areaText.value = ''
    freeText.value = ''
    return
  }

  const sizeMatch = value.match(
    /^\s*([0-9]+(?:\.[0-9]+)?)\s*[x×*]\s*([0-9]+(?:\.[0-9]+)?)\s*(cm|m|mm|厘米|米|毫米)\s*$/i,
  )
  if (sizeMatch) {
    mode.value = 'size'
    widthText.value = sizeMatch[1] ?? ''
    heightText.value = sizeMatch[2] ?? ''
    sizeUnit.value = normalizeSizeUnit(sizeMatch[3])
    return
  }

  const areaMatch = value.match(
    /^\s*([0-9]+(?:\.[0-9]+)?)\s*(平方米|平方|平米|㎡|m2|m²|平方厘米|cm2|cm²|平方毫米|mm2|mm²)\s*$/i,
  )
  if (areaMatch) {
    mode.value = 'area'
    areaText.value = areaMatch[1] ?? ''
    areaUnit.value = normalizeAreaUnit(areaMatch[2])
    return
  }

  mode.value = 'custom'
  freeText.value = value
}

function normalizeSizeUnit(value: string | undefined): string {
  const unit = String(value ?? '').trim().toLowerCase()
  if (unit === 'm' || unit === '米') return 'm'
  if (unit === 'mm' || unit === '毫米') return 'mm'
  return 'cm'
}

function normalizeAreaUnit(value: string | undefined): string {
  const unit = String(value ?? '').trim().toLowerCase()
  if (unit === '平方厘米' || unit === 'cm2' || unit === 'cm²') return '平方厘米'
  if (unit === '平方毫米' || unit === 'mm2' || unit === 'mm²') return '平方毫米'
  return '平方米'
}
</script>

<style scoped>
.structured-spec {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.field-label {
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(var(--yb-text-body));
}

.required {
  color: rgb(var(--yb-danger-text));
}

.spec-mode-tabs {
  display: inline-flex;
  width: fit-content;
  max-width: 100%;
  gap: 0.25rem;
  padding: 0.2rem;
  border: 1px solid rgb(var(--yb-border-subtle));
  border-radius: 999px;
  background: rgb(var(--yb-surface-subtle));
}

.spec-mode-tab {
  border: 0;
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  background: transparent;
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.16s ease, color 0.16s ease, box-shadow 0.16s ease;
}

.spec-mode-tab:hover {
  color: rgb(var(--yb-brand-context-strong));
  background: rgb(var(--yb-surface-blue-control));
}

.spec-mode-tab.active {
  color: rgb(var(--yb-surface));
  background: linear-gradient(135deg, rgb(var(--yb-brand)), rgb(var(--yb-info-sky)));
  box-shadow: 0 6px 14px rgb(var(--yb-brand) / 0.22);
}

.spec-size-grid,
.spec-area-grid {
  display: grid;
  align-items: center;
  gap: 0.5rem;
}

.spec-size-grid {
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) minmax(6rem, 0.7fr);
}

.spec-area-grid {
  grid-template-columns: minmax(0, 1fr) minmax(7rem, 0.6fr);
}

.spec-times {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 1rem;
  font-weight: 800;
}

.spec-native-input,
.spec-native-textarea {
  width: 100%;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  font-size: 0.875rem;
  outline: none;
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.spec-native-input {
  height: 2.75rem;
  padding: 0 0.75rem;
}

.spec-native-textarea {
  min-height: 4.75rem;
  padding: 0.7rem 0.75rem;
  resize: vertical;
}

.spec-native-input:focus,
.spec-native-textarea:focus {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.spec-unit-select {
  cursor: pointer;
}

.spec-preview {
  min-height: 1.25rem;
  margin: 0;
  color: rgb(var(--yb-brand-strong));
  font-size: 0.75rem;
  font-weight: 700;
}

.spec-preview.empty,
.form-hint {
  color: rgb(var(--yb-text-muted));
}

.form-hint {
  margin: -0.1rem 0 0;
  font-size: 0.75rem;
}

@media (max-width: 560px) {
  .spec-size-grid {
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  }

  .spec-size-grid .spec-unit-select {
    grid-column: 1 / -1;
  }

  .spec-area-grid {
    grid-template-columns: 1fr;
  }
}
</style>
