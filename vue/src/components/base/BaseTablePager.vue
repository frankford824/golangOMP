<template>
  <div class="base-table-pager">
    <span class="base-table-pager__total">
      共 {{ total }} 条
      <template v-if="total > 0"> · {{ rangeLabel }}</template>
    </span>
    <div class="base-table-pager__actions">
      <label v-if="showPageSize" class="base-table-pager__size">
        每页
        <BaseSelect
          :model-value="pageSize"
          class="base-table-pager__select"
          :options="pageSizeSelectOptions"
          :disabled="loading"
          @update:model-value="updatePageSize"
        />
      </label>
      <BaseButton
        variant="ghost"
        size="sm"
        :disabled="loading || page <= 1"
        @click="goPage(1)"
      >
        首页
      </BaseButton>
      <BaseButton
        variant="ghost"
        size="sm"
        :disabled="loading || page <= 1"
        @click="goPage(page - 1)"
      >
        上一页
      </BaseButton>
      <span class="base-table-pager__page">第 {{ page }} / {{ totalPages }} 页</span>
      <BaseButton
        variant="ghost"
        size="sm"
        :disabled="loading || page >= totalPages"
        @click="goPage(page + 1)"
      >
        下一页
      </BaseButton>
      <BaseButton
        variant="ghost"
        size="sm"
        :disabled="loading || page >= totalPages"
        @click="goPage(totalPages)"
      >
        末页
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'

const props = withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    loading?: boolean
    showPageSize?: boolean
    pageSizeOptions?: number[]
  }>(),
  {
    loading: false,
    showPageSize: false,
    pageSizeOptions: () => [20, 50, 100],
  },
)

const emit = defineEmits<{
  'update:page': [number]
  'update:pageSize': [number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(Math.max(0, props.total) / Math.max(1, props.pageSize))))
const normalizedPage = computed(() => Math.min(Math.max(1, props.page), totalPages.value))
const rangeLabel = computed(() => {
  if (props.total <= 0) return '0-0'
  const start = (normalizedPage.value - 1) * props.pageSize + 1
  const end = Math.min(props.total, normalizedPage.value * props.pageSize)
  return `${start}-${end}`
})
const pageSizeSelectOptions = computed<BaseSelectOption[]>(() =>
  props.pageSizeOptions.map((size) => ({ value: size, label: String(size) })),
)

function goPage(next: number) {
  const target = Math.min(Math.max(1, next), totalPages.value)
  if (target === props.page) return
  emit('update:page', target)
}

function updatePageSize(value: string | number) {
  const next = Number(value)
  if (!Number.isFinite(next) || next <= 0 || next === props.pageSize) return
  emit('update:pageSize', Math.trunc(next))
}
</script>

<style scoped>
.base-table-pager {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--yb-border));
}

.base-table-pager__total,
.base-table-pager__page,
.base-table-pager__size {
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(var(--yb-text-muted-strong));
}

.base-table-pager__actions,
.base-table-pager__size {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.base-table-pager__select {
  min-width: 4.75rem;
}

@media (max-width: 640px) {
  .base-table-pager {
    align-items: stretch;
  }

  .base-table-pager__actions {
    width: 100%;
  }
}
</style>
