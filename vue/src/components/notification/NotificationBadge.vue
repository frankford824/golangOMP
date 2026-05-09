<template>
  <button
    type="button"
    class="relative inline-flex items-center rounded-full border border-neutral-300 bg-white px-3 py-1 text-xs font-bold text-neutral-900 shadow-sm transition-colors hover:bg-neutral-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-stone-400/35 focus-visible:ring-offset-2"
    @click="$emit('open')"
  >
    通知
    <span
      v-if="unreadCount > 0"
      class="absolute -right-1 -top-1 min-h-[1.125rem] min-w-[1.125rem] rounded-full bg-[var(--v1-xhs-brand)] px-1 text-center text-[10px] font-extrabold leading-[1.125rem] text-white"
    >
      {{ unreadCount > 99 ? '99+' : unreadCount }}
    </span>
  </button>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useWebSocket } from '@/composables/useWebSocket'
import { useNotificationsStore } from '@/stores/notifications.store'

defineEmits<{ open: [] }>()
const notificationsStore = useNotificationsStore()
const unreadCount = computed(() => notificationsStore.unreadCount)

useWebSocket({
  onMessage(event) {
    if (event.type === 'notification_arrived') {
      notificationsStore.applyUnreadCount(event.payload.unread_count)
    }
  },
  onFallbackPoll: notificationsStore.refreshUnreadCount,
})

onMounted(notificationsStore.refreshUnreadCount)
</script>
