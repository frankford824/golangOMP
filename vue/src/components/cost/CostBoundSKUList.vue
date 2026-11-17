<template>
  <aside class="bound-skus" aria-label="方案关联SKU">
    <header>
      <h2>关联 SKU <small>{{ total }}</small></h2>
      <span class="current-cost-note" title="列表显示已记录的金额，方案调整不会即时改变这些金额。">已记录成本</span>
    </header>
    <form @submit.prevent="search">
      <input v-model.trim="keyword" aria-label="搜索 SKU、产品或款式" placeholder="搜索 SKU / 产品 / 款式" />
      <button type="submit" :disabled="loading || !group">搜索</button>
    </form>
    <div ref="listViewport" class="sku-table desktop-region-scroll" role="region" aria-label="关联 SKU 列表" tabindex="0" :aria-busy="loading">
      <p v-if="error" role="alert" class="sku-state">{{ error }} <button type="button" @click="load">重试</button></p>
      <p v-else-if="loading && !items.length" class="sku-state" role="status">正在读取…</p>
      <p v-else-if="!items.length" class="sku-state">{{ group ? '没有匹配的 SKU。' : '请选择方案。' }}</p>
      <table v-else>
        <colgroup><col /><col class="style-column" /><col class="cost-column" /></colgroup>
        <thead><tr><th scope="col">SKU / 产品</th><th scope="col">款式</th><th scope="col">成本</th></tr></thead>
        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td>
              <strong :title="item.sku_code">{{ item.sku_code }}</strong>
              <span :title="item.product_name || '未填写产品名称'">{{ item.product_name || '未填写产品名称' }}</span>
            </td>
            <td :title="item.style_code">{{ item.style_code || '—' }}</td>
            <td :title="formatCost(item.cost_price)">{{ formatCost(item.cost_price) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <footer>
      <button type="button" :disabled="page <= 1 || loading" @click="goToPage(page - 1)">上一页</button>
      <span>第 {{ page }} / {{ totalPages }} 页</span>
      <button type="button" :disabled="page >= totalPages || loading" @click="goToPage(page + 1)">下一页</button>
    </footer>
  </aside>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { costManagementApi } from '@/services/api/costManagementApi'

const props = defineProps<{ group: string; revision: number }>()
const pageSize = 20
const keyword = ref('')
const query = ref('')
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const error = ref('')
const listViewport = ref<HTMLElement | null>(null)
const items = ref<Awaited<ReturnType<typeof costManagementApi.listBoundSKUs>>['data']>([])
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
let request = 0

function formatCost(cost: number | null | undefined) {
  return cost == null ? '待确认' : `¥${cost.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

async function load() {
  const ticket = ++request
  error.value = ''
  if (!props.group) {
    items.value = []
    total.value = 0
    loading.value = false
    return
  }
  loading.value = true
  try {
    const data = await costManagementApi.listBoundSKUs({
      rule_group: props.group,
      keyword: query.value,
      page: page.value,
      page_size: pageSize,
    })
    if (ticket !== request) return
    items.value = data.data || []
    total.value = data.pagination?.total || 0
    if (listViewport.value) listViewport.value.scrollTop = 0
  } catch (cause) {
    if (ticket !== request) return
    items.value = []
    total.value = 0
    error.value = cause instanceof Error ? cause.message : '读取失败'
  } finally {
    if (ticket === request) loading.value = false
  }
}

function goToPage(next: number) {
  page.value = next
  void load()
}

function search() {
  query.value = keyword.value
  goToPage(1)
}

watch(() => [props.group, props.revision], () => {
  keyword.value = ''
  query.value = ''
  page.value = 1
  total.value = 0
  items.value = []
  void load()
}, { immediate: true })

onBeforeUnmount(() => { request += 1 })
</script>

<style scoped>
.bound-skus {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-width: 0;
  min-height: 0;
  padding: 0.85rem 0.9rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  overflow: auto;
}
.bound-skus header,
.bound-skus form,
.bound-skus footer {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 0.45rem;
}
.bound-skus h2 { margin: 0; font-size: 0.9375rem; }
.bound-skus h2 small,
.current-cost-note,
.bound-skus footer {
  color: rgb(var(--yb-text-muted));
  font-size: 0.75rem;
  font-weight: 500;
}
.bound-skus h2 small { margin-inline-start: 0.3rem; font-variant-numeric: tabular-nums; }
.bound-skus input,
.bound-skus button {
  min-height: 2.1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface));
  font-size: 0.8125rem;
}
.bound-skus input {
  flex: 1;
  min-width: 0;
  padding-inline: 0.55rem;
  color: rgb(var(--yb-text));
}
.bound-skus button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  padding: 0.15rem 0.6rem;
  color: rgb(var(--yb-brand));
  white-space: nowrap;
}
.bound-skus button:disabled { opacity: 0.48; cursor: default; }
.bound-skus footer { padding-top: 0.5rem; border-top: 1px solid rgb(var(--yb-border)); }
.sku-table { flex: 1; min-height: 0; overflow: auto; }
.sku-table table { width: 100%; border-collapse: separate; border-spacing: 0; table-layout: fixed; font-size: 0.8125rem; }
.style-column { width: 25%; }
.cost-column { width: 22%; }
.sku-table th { position: sticky; top: 0; z-index: 1; background: rgb(var(--yb-surface-soft)); color: rgb(var(--yb-text-muted)); font-weight: 600; }
.sku-table :is(th, td) {
  overflow: hidden;
  padding: 0.4rem 0.45rem;
  border-bottom: 1px solid rgb(var(--yb-border));
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sku-table :is(th, td):last-child { text-align: right; font-variant-numeric: tabular-nums; }
.sku-table td strong,
.sku-table td span { display: block; overflow: hidden; text-overflow: ellipsis; }
.sku-table td strong { color: rgb(var(--yb-brand)); }
.sku-table td span { margin-top: 0.12rem; color: rgb(var(--yb-text-muted)); font-size: 0.75rem; }
.sku-state { margin: 0.45rem; color: rgb(var(--yb-text-muted)); font-size: 0.8125rem; }
</style>
