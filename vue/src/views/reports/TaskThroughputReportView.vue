<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">任务吞吐报表</h1>
    <p class="mt-1 text-xs text-[var(--v1-text-muted)]">统计窗口：{{ rangeLabel }}</p>
    <pre class="mt-3 overflow-auto rounded bg-[var(--v1-bg-surface-soft)] p-3 text-xs">{{ payload }}</pre>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { reportsApi } from '@/services/api/reportsApi'
import { getBeijingDateStringDaysAgo } from '@/utils/beijing-calendar'

const payload = ref<Record<string, unknown>>({})

const from = getBeijingDateStringDaysAgo(6)
const to = getBeijingDateStringDaysAgo(0)
const rangeLabel = computed(() => `${from} 至 ${to}`)

onMounted(async () => {
  const res = await reportsApi.l1Throughput({ from, to })
  payload.value = res.data as Record<string, unknown>
})
</script>
