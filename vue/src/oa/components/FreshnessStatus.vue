<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ freshness?: { source_committed_at?: string; lag_seconds?: number }; stale?: boolean }>()
const lag = computed(() => Number(props.freshness?.lag_seconds))
const formattedWatermark = computed(() => {
  const source = props.freshness?.source_committed_at
  if (!source) return ''
  if (/(?:Z|[+-]\d{2}:?\d{2})$/.test(source)) {
    const date = new Date(source)
    if (!Number.isNaN(date.getTime())) return new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', dateStyle: 'short', timeStyle: 'short' }).format(date)
  }
  return source.replace('T', ' ').slice(0, 16)
})
</script>

<template>
  <p v-if="freshness?.source_committed_at || stale || lag > 60" class="oa-watermark" role="status" :class="{ 'oa-watermark-warn': stale || lag > 60 }">
    <span v-if="freshness?.source_committed_at">数据截至 <time :datetime="freshness.source_committed_at">{{ formattedWatermark }}</time></span>
    <span v-if="lag > 600" class="oa-watermark-detail">同步已停，显示的是最后一次数据</span>
    <span v-else-if="lag > 60" class="oa-watermark-detail">同步较慢</span>
    <span v-if="stale" class="oa-watermark-detail">暂无法连接订单数据，显示上次查询成功的快照</span>
  </p>
</template>
