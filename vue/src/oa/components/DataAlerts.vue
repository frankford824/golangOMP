<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api, errorText } from '../api'
import OAEmpty from './OAEmpty.vue'

type Event = { seq: number; id: string; source: string; kind: string; title: string; message: string; at: string; event: 'firing' | 'repeat' | 'resolved'; severity: 'info' | 'warning' | 'critical' }
const items = ref<Event[]>([])
const count = ref<number>()
const busy = ref(false)
const error = ref('')
const stage = { firing: '触发', repeat: '持续提醒', resolved: '已恢复' }
const level = { info: '记录', warning: '警告', critical: '严重' }
let timer: number | undefined
async function refresh() {
  if (busy.value) return
  busy.value = true
  try {
    const value = await api<{ active_count: number; items: Event[] }>('/oa-api/data-alerts')
    items.value = value.items
    count.value = value.active_count
    error.value = ''
  } catch (e) { error.value = errorText(e) }
  finally { busy.value = false }
}
onMounted(() => {
  void refresh()
  timer = window.setInterval(() => { if (document.visibilityState === 'visible') void refresh() }, 30_000)
})
onUnmounted(() => window.clearInterval(timer))
</script>

<template>
  <section class="oa-page oa-panel" aria-label="订单数据告警">
    <div class="oa-actions"><h2>数据告警</h2><span v-if="count != null">{{ count }} 项未恢复</span><button type="button" :disabled="busy" @click="refresh">刷新告警</button></div>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-else-if="busy && count == null" role="status">正在读取告警…</p>
    <OAEmpty v-else-if="!items.length" compact kind="inbox" title="暂无数据告警" />
    <div v-if="items.length" class="oa-table-wrap">
      <table class="oa-table"><caption>最近 100 条事件 · 北京时间</caption>
        <thead><tr><th>事件时间</th><th>级别</th><th>状态</th><th>告警</th></tr></thead>
        <tbody><tr v-for="item in items" :key="item.seq"><td><time>{{ item.at }}</time></td><td>{{ level[item.severity] }}</td>
          <td>{{ stage[item.event] }}</td><td><strong>{{ item.title }}</strong><div>{{ item.message }}</div><small>{{ item.id }}</small></td></tr></tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.oa-actions { display: flex; align-items: center; gap: 12px; }
button { margin-inline-start: auto; }
caption { text-align: start; padding-block: 8px; color: var(--oa-secondary); }
td:last-child { white-space: normal; overflow-wrap: anywhere; }
time { white-space: nowrap; }
</style>
