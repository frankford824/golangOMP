<script setup lang="ts">
import OAEmpty from './OAEmpty.vue'
import { onMounted, ref } from 'vue'
import { api, errorText } from '../api'
import { feeDateTime, feeStatus } from '../feeDisplay'
import '../styles.css'

type Activity = { id: number; created_at: string; payload: { document_id: string; state: string; title: string } }
const items = ref<Activity[]>([])
const error = ref('')
const loading = ref(true)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await api<{ items: Activity[] }>('/oa-api/finance/notifications')
    items.value = result.items ?? []
  } catch (cause) { error.value = errorText(cause) }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <section class="expense-activity" aria-labelledby="expense-activity-title">
    <h2 id="expense-activity-title">费用动态</h2>
    <p v-if="loading" role="status">正在加载费用动态…</p>
    <div v-else-if="error"><p role="alert">{{ error }}</p><button type="button" @click="load">重新加载费用动态</button></div>
    <ul v-else-if="items.length">
      <li v-for="item in items" :key="item.id">
        <RouterLink :to="{ path: '/fees/document', query: { id: item.payload.document_id } }">
          <strong>{{ item.payload.title || '费用单据' }}</strong>
          <span :class="['expense-status', 'oa-state-' + item.payload.state]">{{ feeStatus(item.payload.state) }}</span>
          <time :datetime="item.created_at">{{ feeDateTime(item.created_at) }}</time>
        </RouterLink>
      </li>
    </ul>
    <OAEmpty v-else kind="inbox" title="暂无费用动态" description="单据经审批、退回或登记付款后，可在这里查看进展。" />
  </section>
</template>

<style scoped>
.expense-activity { padding: 16px; margin-bottom: 12px; border: 1px solid var(--oa-line); border-radius: 8px; background: var(--oa-surface); }
.expense-activity h2 { margin-bottom: 8px; font-size: 16px; font-weight: 600; line-height: 1.4; }
.expense-activity ul { list-style: none; padding: 0; margin: 0; }
.expense-activity li + li { border-top: 1px solid rgb(var(--yb-border)); }
.expense-activity a { display: grid; grid-template-columns: minmax(0, 1fr) max-content max-content; gap: 8px 16px; align-items: center; padding: 8px 0; min-height: 42px; }
.expense-activity strong { min-width: 0; overflow-wrap: anywhere; }
.expense-activity p, .expense-activity span, .expense-activity time { color: rgb(var(--yb-text-muted)); font-size: 12px; }
.expense-activity .expense-status { padding: 4px 8px; border-radius: 4px; color: var(--oa-ink); }
.expense-activity time { font-variant-numeric: tabular-nums; }
@media (max-width: 540px) {
  .expense-activity a { grid-template-columns: minmax(0, 1fr) max-content; }
  .expense-activity time { grid-column: 1 / -1; }
}
</style>
