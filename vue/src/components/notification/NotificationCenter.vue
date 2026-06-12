<template>
  <div v-if="open" class="notif-drawer-mask" @click.self="close">
    <aside class="notif-drawer">
      <header class="notif-drawer-head">
        <div>
          <div class="notif-drawer-eyebrow">
            <Bell class="h-4 w-4" />
            通知中心
          </div>
          <p>未读 {{ unreadCount }} 条 · 全部 {{ items.length }} 条</p>
        </div>
        <div class="notif-drawer-actions">
          <button type="button" class="notif-drawer-btn notif-drawer-btn--brand" :disabled="unreadCount === 0" @click="markAllRead">
            全部已读
          </button>
          <button type="button" class="notif-drawer-btn" @click="close">关闭</button>
        </div>
      </header>

      <div class="notif-drawer-segment">
        <button type="button" :class="{ 'is-active': filter === 'all' }" @click="filter = 'all'">全部</button>
        <button type="button" :class="{ 'is-active': filter === 'unread' }" @click="filter = 'unread'">未读</button>
      </div>

      <div v-if="filteredItems.length" class="notif-drawer-list">
        <article
          v-for="item in filteredItems"
          :key="item.id"
          class="notif-drawer-row"
          :class="{ 'is-read': isRead(item) }"
          @click="openNotification(item)"
        >
          <span class="notif-drawer-dot" />
          <div class="notif-drawer-body">
            <div class="notif-drawer-top">
              <h3>{{ displayTitle(item) }}</h3>
              <time>{{ displayTime(item.created_at) }}</time>
            </div>
            <p>{{ displayContent(item) }}</p>
            <div class="notif-drawer-foot">
              <span>{{ displayType(item) }}</span>
              <button
                v-if="!isRead(item)"
                type="button"
                class="notif-drawer-mini"
                @click.stop="markRead(item.id)"
              >
                标记已读
              </button>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="notif-drawer-empty">
        <Inbox class="h-7 w-7" />
        <p>{{ emptyText }}</p>
      </div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, Inbox } from 'lucide-vue-next'
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
const unreadCount = computed(() => notificationsStore.unreadCount)
const filteredItems = computed(() =>
  filter.value === 'unread' ? notificationsStore.unreadItems : items.value,
)
const emptyText = computed(() => (filter.value === 'unread' ? '暂无未读通知' : '暂无通知'))

function isRead(item: NotificationItem): boolean {
  return Boolean(item.read ?? item.is_read)
}

function notificationText(item: NotificationItem) {
  return formatNotification(item.notification_type, item.payload as Record<string, unknown> | undefined)
}

function displayTitle(item: NotificationItem): string {
  return item.title ?? notificationText(item).title
}

function displayContent(item: NotificationItem): string {
  return item.content ?? notificationText(item).content
}

function displayType(item: NotificationItem): string {
  if (item.notification_type === 'system_broadcast') return '系统广播'
  if (item.notification_type === 'task_assigned_to_me') return '任务分配'
  if (item.notification_type === 'task_pending_audit') return '待审核'
  if (item.notification_type === 'task_closed') return '已结单'
  if (item.notification_type === 'task_rejected') return '任务驳回'
  if (item.notification_type === 'task_cancelled') return '任务取消'
  return '系统通知'
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
  return formatDateTimeBeijing(createdAt) || createdAt
}

async function openNotification(item: NotificationItem): Promise<void> {
  if (!isRead(item)) {
    await notificationsStore.markRead(item.id).catch(() => undefined)
  }
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

onMounted(notificationsStore.load)
</script>

<style scoped>
.notif-drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 6000;
  background: rgba(15, 23, 42, 0.14);
  padding: 1.25rem;
}

.notif-drawer {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  width: min(100%, 29rem);
  max-height: calc(100vh - 2.5rem);
  margin-left: auto;
  overflow: hidden;
  border: 1px solid #e5e7eb;
  border-radius: 1rem;
  background: #fff;
  box-shadow: 0 24px 70px rgba(24, 24, 27, 0.18);
  padding: 1rem;
}

.notif-drawer-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.notif-drawer-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: #18181b;
  font-size: 0.95rem;
  font-weight: 950;
}

.notif-drawer-head p {
  margin: 0.3rem 0 0;
  color: #71717a;
  font-size: 0.75rem;
}

.notif-drawer-actions {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.45rem;
}

.notif-drawer-btn,
.notif-drawer-mini {
  border: 1px solid #e4e4e7;
  border-radius: 999px;
  background: #fff;
  color: #52525b;
  font-size: 0.7rem;
  font-weight: 900;
  line-height: 1;
}

.notif-drawer-btn {
  min-height: 2rem;
  padding: 0 0.7rem;
}

.notif-drawer-btn--brand {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
}

.notif-drawer-btn:disabled {
  cursor: not-allowed;
  opacity: 1;
  border-color: #e5e7eb;
  background: #f3f4f6;
  color: #9ca3af;
}

.notif-drawer-segment {
  display: inline-flex;
  width: fit-content;
  gap: 0.25rem;
  border: 1px solid #e4e4e7;
  border-radius: 999px;
  background: #f8fafc;
  padding: 0.25rem;
}

.notif-drawer-segment button {
  border: 0;
  border-radius: 999px;
  background: transparent;
  padding: 0.45rem 0.85rem;
  color: #71717a;
  font-size: 0.75rem;
  font-weight: 900;
}

.notif-drawer-segment button.is-active {
  background: #2563eb;
  color: #fff;
}

.notif-drawer-list {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  overflow: auto;
  padding-right: 0.2rem;
  min-height: 0;
}

.notif-drawer-row {
  display: grid;
  grid-template-columns: 0.5rem minmax(0, 1fr);
  gap: 0.65rem;
  border: 1px solid #dbeafe;
  border-radius: 0.85rem;
  background: linear-gradient(180deg, #eff6ff, rgba(255, 255, 255, 0.98));
  padding: 0.85rem;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    box-shadow 0.16s ease;
}

.notif-drawer-row.is-read {
  border-color: #e5e7eb;
  background: #fff;
}

.notif-drawer-row:hover {
  border-color: #bfdbfe;
  background: #f8fbff;
  box-shadow: 0 8px 20px -18px rgba(37, 99, 235, 0.35);
}

.notif-drawer-dot {
  width: 0.45rem;
  height: 0.45rem;
  margin-top: 0.35rem;
  border-radius: 999px;
  background: #ff2442;
}

.notif-drawer-row.is-read .notif-drawer-dot {
  background: #d4d4d8;
}

.notif-drawer-top {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}

.notif-drawer-top h3 {
  margin: 0;
  color: #18181b;
  font-size: 0.875rem;
  font-weight: 950;
}

.notif-drawer-top time {
  flex: 0 0 auto;
  color: #a1a1aa;
  font-size: 0.6875rem;
}

.notif-drawer-body p {
  margin: 0.3rem 0 0;
  color: #52525b;
  font-size: 0.78rem;
  line-height: 1.45;
}

.notif-drawer-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.65rem;
}

.notif-drawer-foot span {
  border-radius: 999px;
  background: #f4f4f5;
  padding: 0.2rem 0.5rem;
  color: #71717a;
  font-size: 0.65rem;
  font-weight: 900;
}

.notif-drawer-mini {
  padding: 0.35rem 0.6rem;
  color: #2563eb;
}

.notif-drawer-empty {
  display: grid;
  place-items: center;
  gap: 0.5rem;
  min-height: 18rem;
  border: 1px dashed #d4d4d8;
  border-radius: 0.85rem;
  color: #a1a1aa;
  text-align: center;
}

.notif-drawer-empty p {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 900;
}

@media (max-width: 520px) {
  .notif-drawer-mask {
    padding: 0.75rem;
  }

  .notif-drawer-head {
    flex-direction: column;
  }

  .notif-drawer-actions {
    width: 100%;
    justify-content: flex-end;
  }
}

</style>
