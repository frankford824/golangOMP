<template>
  <div class="iid-selector">
    <BaseSelect
      :model-value="modelValue ?? ''"
      :label="label"
      :placeholder="placeholder"
      :options="selectOptions"
      :filterable="true"
      :remote-filter="true"
      :empty-text="emptyText"
      :value-label="modelValue"
      filter-placeholder="搜索款式编码"
      :disabled="disabled"
      :clearable="true"
      :hint="hintText"
      :error="error"
      @update:model-value="handleUpdate"
      @filter-change="handleFilterChange"
    >
      <template #empty-actions>
        <button v-if="error" type="button" class="w-full px-3 py-2 text-sm text-[rgb(var(--yb-brand-strong))]" @click.stop="retry">重试查询</button>
      </template>
    </BaseSelect>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import { useErpIidOptions } from '@/composables/useErpIidOptions'

withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    placeholder?: string
    disabled?: boolean
  }>(),
  {
    label: '产品款式编码',
    placeholder: '搜索或选择款式编码',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string | undefined]
}>()

const { loading, selectOptions, error, hasLoaded, loadIids, reset } = useErpIidOptions()
const keyword = ref('')
const searchPending = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | undefined

const emptyText = computed(() => {
  if (loading.value || searchPending.value) return '正在查询 ERP 款式编码…'
  if (error.value) return '查询失败，请点击下方重试'
  return '未找到已同步的 ERP 款式，请核对编码或联系管理员补同步'
})
const hintText = computed(() => {
  if (loading.value || searchPending.value) return '正在查询 ERP 款式编码…'
  if (error.value) return ''
  if (hasLoaded.value && !selectOptions.value.length) return emptyText.value
  return '仅展示 ERP 已同步款式；可按款式编码、商品名称或 SKU 搜索'
})

function handleUpdate(value: string | number): void {
  const next = String(value ?? '').trim()
  emit('update:modelValue', next || undefined)
}

function handleFilterChange(query: string): void {
  keyword.value = query
  reset()
  searchPending.value = true
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    searchPending.value = false
    void loadIids(query)
  }, 300)
}

function retry() {
  void loadIids(keyword.value)
}

onMounted(() => { void loadIids() })
onBeforeUnmount(() => {
  reset()
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
})
</script>

<style scoped>
.iid-selector {
  min-width: 0;
}
</style>
