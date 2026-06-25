<template>
  <button
    type="button"
    class="relative inline-flex items-center rounded-full border border-[rgb(var(--yb-border-strong))] bg-[rgb(var(--yb-surface))] px-3 py-1 text-xs font-bold text-[rgb(var(--yb-text))] shadow-sm transition-colors hover:bg-[rgb(var(--yb-surface-soft))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgb(var(--yb-brand)/0.35)] focus-visible:ring-offset-2"
    @click="openNotifications"
  >
    通知
    <span
      v-if="unreadCount > 0"
      class="absolute -right-1 -top-1 min-h-[1.125rem] min-w-[1.125rem] rounded-full bg-[var(--v1-xhs-brand)] px-1 text-center text-[10px] font-extrabold leading-[1.125rem] text-[rgb(var(--yb-text-inverse))]"
    >
      {{ unreadCount > 99 ? '99+' : unreadCount }}
    </span>
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useRealtimeStore } from '@/stores/realtime.store'

const emit = defineEmits<{ open: [] }>()
const notificationsStore = useNotificationsStore()
const realtimeStore = useRealtimeStore()
const unreadCount = computed(() => notificationsStore.unreadCount)

async function openNotifications(): Promise<void> {
  await realtimeStore.requestBrowserPermission().catch(() => undefined)
  emit('open')
}

function syncNotifications(): void {
  void notificationsStore.load()
}

function handleVisibilityChange(): void {
  if (document.visibilityState === 'visible') syncNotifications()
}

onMounted(() => {
  syncNotifications()
  window.addEventListener('focus', syncNotifications)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', syncNotifications)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>
