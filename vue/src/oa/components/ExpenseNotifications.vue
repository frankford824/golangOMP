<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorText } from '../api'

type Activity = { id: number; created_at: string; payload: { document_id: string; state: string; title: string } }
const items = ref<Activity[]>([])
const error = ref('')
const loading = ref(true)
const statuses: Record<string, string> = { waiting: '待审批', approved: '待付款', returned: '已退回', paid: '已登记付款', rejected: '已拒绝' }

onMounted(async () => {
  try {
    const result = await api<{ items: Activity[] }>('/oa-api/finance/notifications')
    items.value = result.items ?? []
  } catch (cause) { error.value = errorText(cause) }
  finally { loading.value = false }
})
</script>

<template>
  <section class="expense-activity" aria-labelledby="expense-activity-title">
    <h2 id="expense-activity-title">费用动态</h2>
    <p v-if="loading" role="status">正在加载费用动态…</p>
    <p v-else-if="error" role="alert">{{ error }}</p>
    <ul v-else-if="items.length">
      <li v-for="item in items" :key="item.id">
        <RouterLink :to="{ path: '/fees/document', query: { id: item.payload.document_id } }">
          <strong>{{ item.payload.title || '费用单据' }}</strong>
          <span>{{ statuses[item.payload.state] || '状态更新' }}</span>
          <time>{{ item.created_at }}</time>
        </RouterLink>
      </li>
    </ul>
    <p v-else>暂无费用动态</p>
  </section>
</template>

<style scoped>
.expense-activity { padding: 16px; margin-bottom: 12px; border: 1px solid rgb(var(--yb-border)); border-radius: 12px; background: rgb(var(--yb-surface)); }
.expense-activity h2 { margin-bottom: 8px; font-size: 15px; font-weight: 700; }
.expense-activity ul { list-style: none; padding: 0; margin: 0; }
.expense-activity li + li { border-top: 1px solid rgb(var(--yb-border)); }
.expense-activity a { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; padding: 8px 0; }
.expense-activity p, .expense-activity span, .expense-activity time { color: rgb(var(--yb-text-muted)); font-size: 12px; }
.expense-activity time { margin-left: auto; }
</style>
