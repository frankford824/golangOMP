<template>
  <section class="space-y-3">
    <header class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-[var(--v1-border)] bg-white px-4 py-3">
      <div class="space-y-1">
        <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">通知中心</h1>
        <p class="text-xs text-[var(--v1-text-secondary)]">
          共 {{ items.length }} 条 · 未读 {{ unreadCount }} 条
        </p>
      </div>
      <div class="flex items-center gap-2">
        <div class="inline-flex items-center rounded-full border border-[var(--v1-border)] bg-white p-1 text-xs">
          <button
            v-for="option in filterOptions"
            :key="option.value"
            type="button"
            class="rounded-full px-2 py-1"
            :class="
              filter === option.value
                ? 'bg-[var(--v1-bg-primary)] text-white'
                : 'text-[var(--v1-text-secondary)]'
            "
            @click="filter = option.value"
          >
            {{ option.label }}
          </button>
        </div>
        <button
          type="button"
          class="rounded-md border border-[var(--v1-border)] bg-white px-3 py-1 text-xs text-[var(--v1-text-secondary)]"
          :disabled="unreadCount === 0"
          @click="markAllRead"
        >
          全部已读
        </button>
      </div>
    </header>

    <div class="space-y-2">
      <article
        v-for="item in visibleItems"
        :key="item.id"
        class="rounded-xl border px-4 py-3"
        :class="
          isRead(item)
            ? 'border-[var(--v1-border)] bg-white text-[var(--v1-text-muted)]'
            : 'border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] text-[var(--v1-text-primary)]'
        "
      >
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1">
            <p class="text-sm font-medium">{{ item.title ?? item.type ?? '通知' }}</p>
            <p class="text-xs text-[var(--v1-text-secondary)]">{{ item.content ?? '-' }}</p>
            <p class="text-[11px] text-[var(--v1-text-muted)]">{{ displayTime(item.created_at) }}</p>
          </div>
          <button
            v-if="!isRead(item)"
            type="button"
            class="shrink-0 rounded border border-[var(--v1-border)] bg-white px-2 py-0.5 text-[11px] text-[var(--v1-bg-primary)]"
            @click="markRead(item.id)"
          >
            标为已读
          </button>
        </div>
      </article>
      <p v-if="visibleItems.length === 0" class="rounded-xl border border-dashed border-[var(--v1-border)] bg-white px-4 py-8 text-center text-xs text-[var(--v1-text-muted)]">
        暂无通知
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useWebSocket } from '@/composables/useWebSocket'
import { useNotificationsStore, type NotificationItem } from '@/stores/notifications.store'
import { formatDateTimeBeijing } from '@/utils/date'

const notificationsStore = useNotificationsStore()
const items = computed(() => notificationsStore.items)
const filter = ref<'all' | 'unread'>('all')

const filterOptions: Array<{ label: string; value: 'all' | 'unread' }> = [
  { label: '全部', value: 'all' },
  { label: '未读', value: 'unread' },
]

const unreadCount = computed(() => notificationsStore.unreadCount)

const visibleItems = computed(() =>
  filter.value === 'unread' ? notificationsStore.unreadItems : items.value,
)

function isRead(item: NotificationItem): boolean {
  return Boolean(item.read ?? item.is_read)
}

function displayTime(value: string | undefined): string {
  if (!value) return '-'
  return formatDateTimeBeijing(value) || value
}

async function markRead(id: number): Promise<void> {
  await notificationsStore.markRead(id)
}

async function markAllRead(): Promise<void> {
  await notificationsStore.readAll()
}

useWebSocket({
  onMessage(event) {
    if (event.type === 'notification_arrived') {
      notificationsStore.applyUnreadCount(event.payload.unread_count)
      void notificationsStore.load()
    }
  },
  onFallbackPoll: notificationsStore.refreshUnreadCount,
})

onMounted(notificationsStore.load)
</script>
