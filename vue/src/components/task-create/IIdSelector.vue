<template>
  <div class="iid-selector">
    <BaseSelect
      :model-value="modelValue ?? ''"
      :label="label"
      :placeholder="placeholder"
      :options="selectOptions"
      :filterable="true"
      filter-placeholder="搜索款式编码"
      :disabled="disabled || loading"
      :clearable="true"
      :hint="hintText"
      @update:model-value="handleUpdate"
      @filter-change="handleFilterChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount } from 'vue'
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

const { loading, selectOptions, lastSourceMode, loadIids } = useErpIidOptions()
let searchTimer: ReturnType<typeof setTimeout> | undefined

const hintText = computed(() => {
  if (loading.value) return '正在搜索 ERP 款式编码...'
  if (lastSourceMode.value === 'fallback') return '默认展示本地预置 56 项，输入关键字后搜索 ERP'
  if (lastSourceMode.value === 'mixed') return '本地常用结果优先展示，ERP 搜索结果衔接显示在后'
  return '支持滚动浏览与关键字搜索'
})

function handleUpdate(value: string | number): void {
  const next = String(value ?? '').trim()
  emit('update:modelValue', next || undefined)
}

function handleFilterChange(query: string): void {
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    void loadIids(query)
  }, 300)
}

onBeforeUnmount(() => {
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
