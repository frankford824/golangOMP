<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getToken } from '@/services/http'
import OAEmpty from './OAEmpty.vue'
import { api, errorText, send } from '../api'
import { expenseActivityURL, useExpenseActivity, type ExpenseActivityItem, type ExpenseActivityPage } from '../expenseActivity'
import { feeDateTime } from '../feeDisplay'
import '../styles.css'

const props = defineProps<{ compact?: boolean; initialTab?: 'todos' | 'results'; initialPage?: number }>()
const emit = defineEmits<{ navigate: [] }>()
const headingId = useId()
const activity = useExpenseActivity()
const tab = ref<'todos' | 'results'>(props.initialTab ?? 'todos')
const router = useRouter()
const page = ref(props.initialPage || 1)
const data = ref<ExpenseActivityPage>()
const error = ref('')
const loading = ref(true)
const readError = ref('')
const reading = ref(new Set<number>())
const limit = props.compact ? 6 : 12
let request = 0
const count = computed(() => tab.value === 'todos' ? data.value?.todo_count ?? 0 : data.value?.result_count ?? 0)
const totalPages = computed(() => Math.max(1, Math.ceil(count.value / limit)))
const items = computed(() => tab.value === 'todos' ? data.value?.todos ?? [] : data.value?.items ?? [])
const resultLabels: Record<string, string> = {
  approved: '审批通过', returned: '退回修改', rejected: '申请已拒绝', paid: '付款已登记',
}
const actionLabels = { approve: '办理审批', pay: '登记付款', revise: '修改申请' }

async function load() {
  const current = ++request
  const token = getToken()
  loading.value = true
  error.value = ''
  const params = new URLSearchParams({ limit: String(limit), [tab.value === 'todos' ? 'todo_page' : 'result_page']: String(page.value) })
  try {
    const result = await api<ExpenseActivityPage>(expenseActivityURL + '?' + params)
    if (current !== request || token !== getToken()) return
    data.value = result
    if (page.value > totalPages.value) {
      if (!props.compact && router.currentRoute.value.path === '/me/notifications') {
        await router.replace(activityLocation(tab.value, totalPages.value))
        return
      }
      page.value = totalPages.value
      await load()
    }
  } catch (cause) {
    if (current === request && token === getToken()) {
      data.value = undefined
      error.value = errorText(cause)
    }
  } finally {
    if (current === request) loading.value = false
  }
}
function activityLocation(kind: 'todos' | 'results', number: number) {
  return { path: '/me/notifications', query: { section: 'expense', activity: kind === 'results' ? 'results' : undefined, expensePage: number > 1 ? String(number) : undefined } }
}
function documentLocation(item: ExpenseActivityItem) {
  const origin = !props.compact && router.currentRoute.value.path === '/me/notifications'
    ? router.currentRoute.value.fullPath : router.resolve(activityLocation(tab.value, 1)).fullPath
  return { path: '/fees/document', query: { id: item.payload.document_id, from: 'notifications',
    activity: tab.value === 'results' ? 'results' : undefined, expensePage: props.compact ? undefined : String(page.value), feeFrom: origin } }
}
function selectTab(value: typeof tab.value) {
  if (value === tab.value) return
  if (!props.compact && router.currentRoute.value.path === '/me/notifications') { void router.push(activityLocation(value, 1)); return }
  tab.value = value
  page.value = 1
  void load()
}
function turnPage(value: number) {
  if (!props.compact && router.currentRoute.value.path === '/me/notifications') { void router.push(activityLocation(tab.value, value)); return }
  page.value = value
  void load()
}
function refresh() {
  void load()
  void activity.refresh(true)
}
async function markRead(item: ExpenseActivityItem, navigating = false) {
  if (!item.unread || reading.value.has(item.id)) return
  const token = getToken()
  reading.value.add(item.id)
  readError.value = ''
  try {
    await send(expenseActivityURL + '/' + item.id + '/read', {})
    if (token === getToken()) activity.invalidate()
  } catch (cause) {
    if (!navigating && token === getToken()) readError.value = errorText(cause)
  } finally {
    reading.value.delete(item.id)
  }
}
function openItem(event: MouseEvent, item: ExpenseActivityItem) {
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return
  void markRead(item, true)
  emit('navigate')
}
onMounted(load)
onUnmounted(() => request++)
watch(() => activity.revision, load)
</script>

<template>
  <section class="expense-activity" :class="{ 'expense-activity--compact': compact }" :aria-labelledby="headingId">
    <header>
      <h2 :id="headingId">费用动态</h2>
      <button type="button" class="oa-button" :disabled="loading" @click="refresh">刷新费用动态</button>
    </header>
    <div class="expense-activity__tabs" role="group" aria-label="费用动态类型">
      <button type="button" :aria-pressed="tab === 'todos'" @click="selectTab('todos')">
        需要我处理 {{ data?.todo_count ?? '' }}
      </button>
      <button type="button" :aria-pressed="tab === 'results'" @click="selectTab('results')">
        处理结果 {{ data?.result_count ?? '' }}
        <span v-if="data?.unread_result_count"> · {{ data.unread_result_count }} 条未读</span>
      </button>
    </div>
    <p v-if="readError" role="alert">{{ readError }}</p>
    <p v-if="loading" role="status">正在加载费用动态…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><button type="button" class="oa-button" @click="refresh">重新加载</button></div>
    <ul v-else-if="items.length">
      <li v-for="item in items" :key="item.payload.kind === 'result' ? item.id : item.payload.document_id">
        <RouterLink :to="documentLocation(item)" @click="openItem($event, item)">
          <strong>{{ item.payload.title || '费用单据' }} <span v-if="item.unread" class="expense-unread">未读</span></strong>
          <span :class="['expense-status', 'oa-state-' + item.payload.state]">
            {{ item.payload.kind === 'result' ? (resultLabels[item.payload.state] || '状态更新') : actionLabels[item.payload.kind] }}
          </span>
          <time :datetime="item.created_at">{{ feeDateTime(item.created_at) }}</time>
        </RouterLink>
        <button v-if="item.unread" class="oa-button expense-mark-read" type="button" :disabled="reading.has(item.id)" @click="markRead(item)">标记已读</button>
      </li>
    </ul>
    <OAEmpty v-else kind="inbox" :title="tab === 'todos' ? '暂无需要处理的费用单据' : '暂无费用处理结果'" />
    <nav v-if="!loading && totalPages > 1" class="expense-activity__pager" aria-label="费用动态分页">
      <span>第 {{ page }} / {{ totalPages }} 页</span>
      <button type="button" class="oa-button" :disabled="page <= 1" @click="turnPage(page - 1)">上一页</button>
      <button type="button" class="oa-button" :disabled="page >= totalPages" @click="turnPage(page + 1)">下一页</button>
    </nav>
  </section>
</template>

<style scoped>
.expense-activity { padding: 16px; margin-bottom: 12px; border: 1px solid var(--oa-line); border-radius: 8px; background: var(--oa-surface); }
.expense-activity--compact { padding: 0; margin: 0; border: 0; }
.expense-activity header { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.expense-activity h2 { margin: 0; font-size: 16px; font-weight: 600; line-height: 1.4; }
.expense-activity ul { list-style: none; padding: 0; margin: 0; }
.expense-activity li + li { border-top: 1px solid rgb(var(--yb-border)); }
.expense-activity li:has(.expense-mark-read) { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 8px; }
.expense-activity .expense-unread { color: var(--oa-accent); font-weight: 600; }
.expense-mark-read { white-space: nowrap; }
.expense-activity a { display: grid; grid-template-columns: minmax(0, 1fr) max-content max-content; gap: 8px 16px; align-items: center; padding: 8px; min-height: 44px; border-radius: 6px; }
.expense-activity a:hover, .expense-activity a:focus-visible { background: rgb(var(--yb-surface-hover)); }
.expense-activity strong { min-width: 0; overflow-wrap: anywhere; }
.expense-activity p, .expense-activity span, .expense-activity time { color: rgb(var(--yb-text-muted)); font-size: 12px; }
.expense-activity .expense-status { padding: 4px 8px; border-radius: 4px; color: var(--oa-ink); font-weight: 600; }
.expense-activity time, .expense-activity__tabs button, .expense-activity__pager span { font-variant-numeric: tabular-nums; }
.expense-activity__tabs { display: flex; flex-wrap: wrap; gap: 8px; margin-block: 8px; border-bottom: 1px solid var(--oa-line); }
.expense-activity__tabs > button { min-height: 40px; padding-inline: 8px; font-size: 14px; border-bottom: 2px solid transparent; }
.expense-activity__tabs > button[aria-pressed='true'] { border-color: var(--oa-accent); color: var(--oa-accent); font-weight: 600; }
.expense-activity__pager { display: flex; align-items: center; justify-content: end; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.expense-activity--compact a { grid-template-columns: minmax(0, 1fr) max-content; }
.expense-activity--compact time { grid-column: 1 / -1; }
@media (max-width: 540px) {
  .expense-activity a { grid-template-columns: minmax(0, 1fr) max-content; }
  .expense-activity time { grid-column: 1 / -1; }
}
</style>
