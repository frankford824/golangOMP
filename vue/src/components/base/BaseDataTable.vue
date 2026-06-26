<template>
  <div
    class="base-data-table"
    :class="{
      'base-data-table--compact': density === 'compact',
      'base-data-table--comfortable': density === 'comfortable',
    }"
    :aria-label="ariaLabel"
    data-base-data-table
  >
    <NDataTable
      :columns="columns"
      :data="data"
      :loading="loading"
      :row-key="rowKey"
      :row-class-name="rowClassName"
      :bordered="false"
      :bottom-bordered="false"
      :single-line="false"
      :pagination="false"
      :remote="remote"
      :scroll-x="scrollX"
      :max-height="maxHeight"
      :virtual-scroll="virtualScroll"
      :size="naiveSize"
    >
      <template #empty>
        <BaseEmptyState
          :title="emptyTitle"
          :description="emptyDescription"
        />
      </template>
    </NDataTable>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NDataTable, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'

type TableRow = any

const props = withDefaults(
  defineProps<{
    columns: DataTableColumns<TableRow>
    data: TableRow[]
    rowKey?: (row: TableRow) => DataTableRowKey
    rowClassName?: (row: TableRow, index: number) => string
    loading?: boolean
    remote?: boolean
    virtualScroll?: boolean
    scrollX?: number
    maxHeight?: number
    density?: 'compact' | 'comfortable'
    ariaLabel?: string
    emptyTitle?: string
    emptyDescription?: string
  }>(),
  {
    rowKey: undefined,
    rowClassName: undefined,
    loading: false,
    remote: true,
    virtualScroll: false,
    scrollX: 960,
    maxHeight: undefined,
    density: 'comfortable',
    ariaLabel: undefined,
    emptyTitle: '暂无数据',
    emptyDescription: '',
  },
)

const naiveSize = computed(() => (props.density === 'compact' ? 'small' : 'medium'))
</script>

<style scoped>
.base-data-table {
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
}

.base-data-table :deep(.n-data-table) {
  --n-font-size: 0.8125rem;
  --n-th-font-weight: 700;
  font-family: var(--yb-font-text);
}

.base-data-table :deep(.n-data-table-base-table) {
  background: rgb(var(--yb-surface));
}

.base-data-table :deep(.n-data-table-th) {
  border-color: rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-body));
  white-space: nowrap;
}

.base-data-table :deep(.n-data-table-td) {
  border-color: rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  vertical-align: top;
}

.base-data-table :deep(.n-data-table-tr:hover .n-data-table-td) {
  background: rgb(var(--yb-surface-soft));
}

.base-data-table :deep(.n-data-table-th__title) {
  color: rgb(var(--yb-text-body));
}

.base-data-table :deep(.n-data-table-empty) {
  padding: 0;
}

.base-data-table :deep(.n-data-table-loading) {
  background: rgb(var(--yb-surface) / 0.72);
}

.base-data-table :deep(.n-data-table-base-table-body),
.base-data-table :deep(.n-scrollbar-container) {
  scrollbar-color: rgb(var(--yb-border-strong)) rgb(var(--yb-surface-soft));
}

.base-data-table--compact :deep(.n-data-table-th),
.base-data-table--compact :deep(.n-data-table-td) {
  padding-block: 0.45rem;
}

.base-data-table--comfortable :deep(.n-data-table-th),
.base-data-table--comfortable :deep(.n-data-table-td) {
  padding-block: 0.6rem;
}
</style>
