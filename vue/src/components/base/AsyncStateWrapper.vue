<template>
  <template v-if="loading">
    <div class="bg-white rounded-2xl border border-slate-100 shadow-sm p-6">
      <slot name="skeleton">
        <div class="space-y-2">
          <BaseSkeleton
            v-for="i in skeletonRows"
            :key="i"
            width="100%"
            height="2.5rem"
          />
        </div>
      </slot>
    </div>
  </template>
  <template v-else-if="error">
    <div class="bg-white rounded-2xl border border-slate-100 shadow-sm p-6">
      <BaseErrorState :title="errorTitle ?? '加载失败'" :description="error" @retry="$emit('retry')" />
    </div>
  </template>
  <template v-else-if="empty">
    <div class="bg-white rounded-2xl border border-slate-100 shadow-sm p-6">
      <BaseEmptyState :title="emptyTitle ?? '暂无数据'" :description="emptyDescription">
        <slot name="empty-action" />
      </BaseEmptyState>
    </div>
  </template>
  <template v-else>
    <slot />
  </template>
</template>

<script setup lang="ts">
import BaseSkeleton from './BaseSkeleton.vue'
import BaseErrorState from './BaseErrorState.vue'
import BaseEmptyState from './BaseEmptyState.vue'

withDefaults(
  defineProps<{
    loading: boolean
    error?: string | null
    empty: boolean
    emptyTitle?: string
    emptyDescription?: string
    errorTitle?: string
    skeletonRows?: number
  }>(),
  { skeletonRows: 4 },
)

defineEmits<{ retry: [] }>()
</script>
