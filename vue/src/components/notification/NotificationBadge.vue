<template>
  <button
    type="button"
    class="topbar-icon"
    :aria-label="label"
    :title="label"
    @click="openNotifications"
  >
    <Bell :size="18" aria-hidden="true" />
    <span
      v-if="badgeCount > 0"
      class="absolute -right-1 -top-1 min-h-[1rem] min-w-[1rem] rounded-full bg-[rgb(var(--yb-danger-text))] px-0.5 text-center text-[0.625rem] font-bold leading-4 text-[rgb(var(--yb-text-inverse))]"
      aria-hidden="true"
    >
      {{ badgeCount > 99 ? '99+' : badgeCount }}
    </span>
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Bell } from 'lucide-vue-next'
import { oaEnabled, useOAAccess } from '@/oa/access'
import { useCostActivity } from '@/oa/costActivity'
import { expenseActivityActions, useExpenseActivity } from '@/oa/expenseActivity'
import { useNotificationsStore } from '@/stores/notifications.store'

const router = useRouter()
const emit = defineEmits<{ open: [] }>()
const notificationsStore = useNotificationsStore()
const unreadCount = computed(() => notificationsStore.unreadCount)
const { can, currentProfile } = useOAAccess()
const expenseActivity = useExpenseActivity()
const costActivity = useCostActivity()
const canUseExpenses = computed(() => expenseActivityActions.some(can))
const canUseCosts = computed(() => can('orders.view') || can('cost.request.handle'))
const expenseTodoCount = computed(() => canUseExpenses.value ? expenseActivity.todoCount ?? 0 : 0)
const expenseResultCount = computed(() => canUseExpenses.value ? expenseActivity.unreadResultCount : 0)
const costActivityCount = computed(() => canUseCosts.value ? costActivity.todoCount ?? 0 : 0)
const badgeCount = computed(() => unreadCount.value + expenseTodoCount.value + expenseResultCount.value + costActivityCount.value)
const label = computed(() => !oaEnabled ? (unreadCount.value ? `通知，${unreadCount.value} 条未读` : '通知') : [
  '通知',
  unreadCount.value ? `${unreadCount.value} 条系统未读` : '',
  expenseTodoCount.value ? `${expenseTodoCount.value} 项费用待办` : '',
  expenseResultCount.value ? `${expenseResultCount.value} 条费用结果未读` : '',
  costActivityCount.value ? `${costActivityCount.value} 项成本待办或回复` : '',
].filter(Boolean).join('，'))
let refreshTimer: ReturnType<typeof setInterval> | undefined

watch([canUseExpenses, () => currentProfile.value?.id], ([allowed]) => {
  expenseActivity.reset()
  if (allowed) void expenseActivity.refresh()
}, { immediate: true })
watch([canUseCosts, () => currentProfile.value?.id], ([allowed]) => {
  costActivity.reset()
  if (allowed) void costActivity.refresh()
}, { immediate: true })

function syncOAActivity() {
  if (canUseExpenses.value) void expenseActivity.refresh()
  if (canUseCosts.value) void costActivity.refresh()
}

async function openNotifications(): Promise<void> {
  if (!oaEnabled) { emit('open'); return }
  if (router.currentRoute.value.name === 'MeNotifications') return
  const expense = expenseTodoCount.value > 0 || expenseResultCount.value > 0
  const query = expense
    ? { section: 'expense', activity: expenseTodoCount.value ? undefined : 'results' }
    : costActivityCount.value ? { section: 'cost' } : {}
  await router.push({ name: 'MeNotifications', query })
}

function syncNotifications(): void {
  void (oaEnabled ? notificationsStore.refreshUnreadCount() : notificationsStore.load()).catch(() => undefined)
  syncOAActivity()
}

function handleVisibilityChange(): void {
  if (document.visibilityState === 'visible') syncNotifications()
}

onMounted(() => {
  syncNotifications()
  refreshTimer = setInterval(() => {
    if (document.visibilityState === 'visible') syncOAActivity()
  }, 60_000)
  window.addEventListener('focus', syncNotifications)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  clearInterval(refreshTimer)
  expenseActivity.reset()
  costActivity.reset()
  window.removeEventListener('focus', syncNotifications)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>
