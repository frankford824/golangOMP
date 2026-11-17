<template>
  <button
    type="button"
    class="topbar-icon"
    :aria-label="unreadCount ? `通知，${unreadCount} 条未读` : '通知'"
    title="通知"
    @click="openNotifications"
  >
    <Bell :size="18" aria-hidden="true" />
    <span
      v-if="unreadCount > 0"
      class="absolute -right-1 -top-1 min-h-[1rem] min-w-[1rem] rounded-full bg-[rgb(var(--yb-danger-text))] px-0.5 text-center text-[0.625rem] font-bold leading-4 text-[rgb(var(--yb-text-inverse))]"
      aria-hidden="true"
    >
      {{ unreadCount > 99 ? '99+' : unreadCount }}
    </span>
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { Bell } from 'lucide-vue-next'
import { useNotificationsStore } from '@/stores/notifications.store'

const emit = defineEmits<{ open: [] }>()
const notificationsStore = useNotificationsStore()
const unreadCount = computed(() => notificationsStore.unreadCount)

async function openNotifications(): Promise<void> {
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
