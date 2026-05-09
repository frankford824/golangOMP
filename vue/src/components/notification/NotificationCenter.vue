<template>
  <div v-if="open" class="fixed inset-0 z-50 bg-slate-900/40 p-6" @click.self="close">
    <div
      class="notif-nc-shell ml-auto w-full max-w-md rounded-[1.375rem] border border-neutral-200/90 bg-white px-[1.125rem] py-4 shadow-[var(--v1-xhs-card-shadow)]"
    >
      <div class="mb-3 flex items-center justify-between gap-3">
        <h3 class="text-sm font-extrabold tracking-tight text-neutral-900">通知中心</h3>
        <div class="notif-nc-actions flex shrink-0 items-center gap-2">
          <button type="button" class="notif-nc-btn notif-nc-btn--coral" @click="markAllRead">全部已读</button>
          <button type="button" class="notif-nc-btn notif-nc-btn--mute" @click="close">关闭</button>
        </div>
      </div>
      <div
        class="notif-nc-segment mb-3 inline-flex items-center gap-1 rounded-full border border-neutral-200/90 bg-neutral-50/90 p-1 text-xs"
      >
        <button
          type="button"
          class="notif-nc-seg rounded-full border border-transparent px-2.5 py-1 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(255,36,66,0.35)] focus-visible:ring-offset-2"
          :class="filter === 'all' ? 'notif-nc-seg--on' : 'notif-nc-seg--off'"
          @click="filter = 'all'"
        >
          全部
        </button>
        <button
          type="button"
          class="notif-nc-seg rounded-full border border-transparent px-2.5 py-1 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(255,36,66,0.35)] focus-visible:ring-offset-2"
          :class="filter === 'unread' ? 'notif-nc-seg--on' : 'notif-nc-seg--off'"
          @click="filter = 'unread'"
        >
          未读
        </button>
      </div>
      <div v-if="filteredItems.length" class="space-y-2.5">
        <article
          v-for="item in filteredItems"
          :key="item.id"
          class="cursor-pointer rounded-[0.875rem] border px-3 py-2.5 text-sm transition-colors"
          :class="
            isRead(item)
              ? 'border-neutral-200/90 bg-white text-[var(--v1-text-muted)]'
              : 'notif-nc-row-unread border-[color:var(--v1-xhs-brand-border)] bg-[var(--v1-xhs-brand-soft)] text-neutral-900'
          "
          @click="openTask(item)"
        >
          <p>{{ displayTitle(item) }}</p>
          <p class="text-xs">{{ displayContent(item) }}</p>
          <p class="mt-1 text-[11px] text-[var(--v1-text-muted)]">{{ displayTime(item.created_at) }}</p>
          <div class="mt-2 flex justify-end gap-2">
            <button
              v-if="!isRead(item)"
              type="button"
              class="notif-nc-mini notif-nc-btn notif-nc-btn--mute"
              @click.stop="markRead(item.id)"
            >
              标记已读
            </button>
          </div>
        </article>
      </div>
      <div
        v-else
        class="rounded-[0.875rem] border border-solid border-neutral-200/95 bg-neutral-50/80 px-3 py-8 text-center text-sm font-semibold text-neutral-400"
      >
        {{ emptyText }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useWebSocket } from '@/composables/useWebSocket'
import { formatNotification } from '@/domain/notification-text'
import { useNotificationsStore, type NotificationItem } from '@/stores/notifications.store'
import { formatDateTimeBeijing, taskInstantMs } from '@/utils/date'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()
const router = useRouter()
const notificationsStore = useNotificationsStore()
const filter = ref<'all' | 'unread'>('all')
const open = computed(() => props.open)
const items = computed(() => notificationsStore.items)
const filteredItems = computed(() =>
  filter.value === 'unread' ? notificationsStore.unreadItems : items.value,
)
const emptyText = computed(() => (filter.value === 'unread' ? '暂无未读通知' : '暂无通知'))

function isRead(item: NotificationItem): boolean {
  return Boolean(item.read ?? item.is_read)
}

function displayTitle(item: NotificationItem): string {
  return formatNotification(item.notification_type, item.payload as Record<string, unknown> | undefined).title
}

function displayContent(item: NotificationItem): string {
  return formatNotification(item.notification_type, item.payload as Record<string, unknown> | undefined).content
}

function displayTime(createdAt: string | undefined): string {
  if (!createdAt) return '-'
  const diffMs = Date.now() - taskInstantMs(createdAt)
  if (Number.isFinite(diffMs) && diffMs >= 0) {
    const minute = 60_000
    const hour = 60 * minute
    const day = 24 * hour
    if (diffMs < hour) return `${Math.max(1, Math.floor(diffMs / minute))} 分钟前`
    if (diffMs < day) return `${Math.floor(diffMs / hour)} 小时前`
    if (diffMs < 7 * day) return `${Math.floor(diffMs / day)} 天前`
  }
  return formatDateTimeBeijing(createdAt)
}

function openTask(item: NotificationItem): void {
  const payload = (item.payload ?? {}) as Record<string, unknown>
  const taskId = Number(payload.task_id)
  if (!Number.isFinite(taskId) || taskId <= 0) return
  void router.push(`/tasks/${taskId}`)
  close()
}

function close(): void {
  emit('update:open', false)
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

<style scoped>
/* designs/xiaohongshu-review-v1.pen */
.notif-nc-btn {
  border-radius: 9999px;
  padding: 0.35rem 0.72rem;
  font-size: 0.6875rem;
  line-height: 1.35;
  font-weight: 700;
  letter-spacing: -0.01em;
  transition:
    border-color 0.12s ease,
    background-color 0.12s ease,
    box-shadow 0.12s ease;
}
.notif-nc-btn:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px #fff,
    0 0 0 4px rgba(255, 36, 66, 0.28);
}

.notif-nc-btn--mute {
  border: 1px solid #d4d4d8;
  background: #fff;
  color: #52525b;
}

.notif-nc-btn--mute:hover {
  border-color: #a1a1aa;
  background: #fafafa;
  color: #18181b;
}

.notif-nc-btn--coral {
  border: 1px solid var(--v1-xhs-brand);
  background: #fff;
  color: var(--v1-xhs-brand);
}

.notif-nc-btn--coral:hover {
  background: var(--v1-xhs-brand-soft);
  border-color: var(--v1-xhs-brand);
}

.notif-nc-seg--off {
  border-color: transparent;
  color: #71717a;
}

.notif-nc-seg--off:hover {
  color: #3f3f46;
  background: rgb(244 244 245 / 0.95);
}

.notif-nc-seg--on {
  border-color: var(--v1-xhs-brand);
  background: var(--v1-xhs-brand-soft);
  color: var(--v1-xhs-brand);
  font-weight: 800;
}

.notif-nc-mini {
  padding: 0.22rem 0.52rem;
  font-size: 0.625rem;
}

.notif-nc-row-unread {
  box-shadow: 0 1px 0 rgba(255, 36, 66, 0.05);
}
</style>
