<template>
  <section class="notif-page">
    <header class="notif-hero">
      <div>
        <div class="notif-eyebrow">
          <Bell class="h-4 w-4" />
          通知中心
        </div>
        <h1>系统消息与运营广播</h1>
        <p>共 {{ items.length }} 条，未读 {{ unreadCount }} 条。未点击过的通知会在下次登录后继续保留。</p>
      </div>
      <div class="notif-hero-actions">
        <div class="notif-segment">
          <button
            v-for="option in filterOptions"
            :key="option.value"
            type="button"
            :class="{ 'is-active': filter === option.value }"
            @click="filter = option.value"
          >
            {{ option.label }}
          </button>
        </div>
        <button
          type="button"
          class="notif-btn notif-btn--ghost"
          :disabled="unreadCount === 0"
          @click="markAllRead"
        >
          <CheckCircle2 class="h-4 w-4" />
          全部已读
        </button>
      </div>
    </header>

    <section v-if="canBroadcast" class="notif-broadcast">
      <div class="notif-section-head">
        <div>
          <div class="notif-eyebrow">
            <Megaphone class="h-4 w-4" />
            统一广播
          </div>
          <h2>发送通知到通知中心</h2>
        </div>
        <p>可选择全系统用户，或指定一个/多个用户。发送后写入对方通知中心，离线后下次登录仍可见。</p>
      </div>

      <div class="notif-broadcast-grid">
        <label class="notif-field">
          <span>标题</span>
          <input v-model.trim="broadcastTitle" maxlength="80" placeholder="例如：临时排班调整" />
        </label>

        <div class="notif-field">
          <span>接收范围</span>
          <div class="notif-choice">
            <button
              v-if="canBroadcastAll"
              type="button"
              :class="{ 'is-active': broadcastAudience === 'all' }"
              @click="broadcastAudience = 'all'"
            >
              <Users class="h-4 w-4" />
              全部用户
            </button>
            <button
              type="button"
              :class="{ 'is-active': broadcastAudience === 'users' }"
              @click="broadcastAudience = 'users'"
            >
              <UserPlus class="h-4 w-4" />
              指定用户
            </button>
          </div>
        </div>

        <label class="notif-field notif-field--wide">
          <span>通知详情</span>
          <textarea
            v-model.trim="broadcastContent"
            maxlength="1000"
            rows="4"
            placeholder="输入对方需要看到的完整通知内容"
          />
        </label>

        <div v-if="broadcastAudience === 'users'" class="notif-recipient-panel">
          <div class="notif-recipient-search">
            <Search class="h-4 w-4" />
            <input
              v-model.trim="userKeyword"
              placeholder="搜索姓名、账号、部门"
              @keyup.enter="searchUsers"
            />
            <button type="button" class="notif-btn notif-btn--ghost" :disabled="usersLoading" @click="searchUsers">
              搜索
            </button>
          </div>

          <div v-if="selectedUsers.length" class="notif-selected-users">
            <button v-for="user in selectedUsers" :key="user.id" type="button" @click="removeUser(user.id)">
              {{ displayUserName(user) }}
              <span>×</span>
            </button>
          </div>

          <div class="notif-user-options">
            <label v-for="user in userOptions" :key="user.id" class="notif-user-option">
              <input
                type="checkbox"
                :checked="selectedUserIds.includes(normalizeUserID(user.id))"
                @change="toggleUser(user)"
              />
              <span>
                <strong>{{ displayUserName(user) }}</strong>
                <small>{{ displayUserMeta(user) }}</small>
              </span>
            </label>
            <p v-if="!userOptions.length" class="notif-empty-inline">输入关键词搜索用户</p>
          </div>
        </div>
      </div>

      <footer class="notif-broadcast-footer">
        <p :class="{ 'is-error': broadcastMessageType === 'error' }">{{ broadcastMessage }}</p>
        <button type="button" class="notif-btn notif-btn--primary" :disabled="sendingBroadcast" @click="sendBroadcast">
          <Send class="h-4 w-4" />
          发送广播
        </button>
      </footer>
    </section>

    <div class="notif-layout">
      <section class="notif-list-panel">
        <div class="notif-section-head">
          <div>
            <h2>通知记录</h2>
            <p>按最新时间排序，点击通知可标记为已读；任务类通知会打开对应任务。</p>
          </div>
        </div>

        <div v-if="visibleItems.length" class="notif-list">
          <article
            v-for="item in visibleItems"
            :key="item.id"
            class="notif-row"
            :class="{ 'is-read': isRead(item) }"
            @click="openNotification(item)"
          >
            <div class="notif-row-icon">
              <Circle v-if="!isRead(item)" class="h-3 w-3" />
              <CheckCircle2 v-else class="h-4 w-4" />
            </div>
            <div class="notif-row-body">
              <div class="notif-row-top">
                <h3>{{ displayTitle(item) }}</h3>
                <time>{{ displayTime(item.created_at) }}</time>
              </div>
              <p>{{ displayContent(item) }}</p>
              <span>{{ displayType(item) }}</span>
            </div>
            <button
              v-if="!isRead(item)"
              type="button"
              class="notif-mini-btn"
              @click.stop="markRead(item.id)"
            >
              标为已读
            </button>
          </article>
        </div>

        <div v-else class="notif-empty">
          <Inbox class="h-8 w-8" />
          <p>{{ emptyText }}</p>
        </div>
      </section>

      <aside class="notif-summary">
        <div>
          <span>未读</span>
          <strong>{{ unreadCount }}</strong>
        </div>
        <div>
          <span>全部</span>
          <strong>{{ items.length }}</strong>
        </div>
        <div>
          <span>当前筛选</span>
          <strong>{{ visibleItems.length }}</strong>
        </div>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Bell,
  CheckCircle2,
  Circle,
  Inbox,
  Megaphone,
  Search,
  Send,
  UserPlus,
  Users,
} from 'lucide-vue-next'
import { formatNotification } from '@/domain/notification-text'
import { notificationsApi } from '@/services/api/notificationsApi'
import { usersApi } from '@/services/api/usersApi'
import { useNotificationsStore, type NotificationItem } from '@/stores/notifications.store'
import { usePermissionsStore } from '@/stores/permissions'
import { RoleEnum } from '@/types'
import { formatDateTimeBeijing, taskInstantMs } from '@/utils/date'

type FilterMode = 'all' | 'unread'
type AudienceMode = 'all' | 'users'
type MessageType = 'info' | 'error'

interface DirectoryUser {
  id: number | string
  username?: string
  display_name?: string
  name?: string
  department?: string
  team?: string
}

const router = useRouter()
const notificationsStore = useNotificationsStore()
const permissionsStore = usePermissionsStore()

const items = computed(() => notificationsStore.items)
const filter = ref<FilterMode>('all')
const broadcastAudience = ref<AudienceMode>('users')
const broadcastTitle = ref('')
const broadcastContent = ref('')
const userKeyword = ref('')
const userOptions = ref<DirectoryUser[]>([])
const selectedUserIds = ref<number[]>([])
const usersLoading = ref(false)
const sendingBroadcast = ref(false)
const broadcastMessage = ref('广播会写入通知中心，用户离线或重新登录后仍可看到未读记录。')
const broadcastMessageType = ref<MessageType>('info')

const filterOptions: Array<{ label: string; value: FilterMode }> = [
  { label: '全部', value: 'all' },
  { label: '未读', value: 'unread' },
]

const unreadCount = computed(() => notificationsStore.unreadCount)
const visibleItems = computed(() =>
  filter.value === 'unread' ? notificationsStore.unreadItems : items.value,
)
const emptyText = computed(() => (filter.value === 'unread' ? '暂无未读通知' : '暂无通知'))
const canBroadcast = computed(() =>
  permissionsStore.hasAnyRole([
    RoleEnum.SUPER_ADMIN,
    RoleEnum.HR_ADMIN,
    RoleEnum.DEPT_ADMIN,
    'SuperAdmin',
    'Admin',
    'HRAdmin',
    'DepartmentAdmin',
  ]),
)
const canBroadcastAll = computed(() =>
  permissionsStore.hasAnyRole([RoleEnum.SUPER_ADMIN, RoleEnum.HR_ADMIN, 'SuperAdmin', 'Admin', 'HRAdmin']),
)
const selectedUsers = computed(() => {
  const byID = new Map(userOptions.value.map((user) => [normalizeUserID(user.id), user]))
  return selectedUserIds.value.map((id) => byID.get(id) ?? ({ id, display_name: `用户 ${id}` } as DirectoryUser))
})

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

function normalizeUserID(value: number | string): number {
  const id = Number(value)
  return Number.isFinite(id) ? id : 0
}

function displayUserName(user: DirectoryUser): string {
  return String(user.display_name || user.name || user.username || `用户 ${user.id}`)
}

function displayUserMeta(user: DirectoryUser): string {
  return [user.department, user.team, user.username].filter(Boolean).join(' / ') || '系统用户'
}

function unwrapUsers(raw: unknown): DirectoryUser[] {
  const root = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  const data = root.data
  if (Array.isArray(data)) return data as DirectoryUser[]
  const body = data && typeof data === 'object' ? (data as Record<string, unknown>) : root
  const candidates = body.items ?? body.users ?? body.records ?? []
  return Array.isArray(candidates) ? (candidates as DirectoryUser[]) : []
}

function toggleUser(user: DirectoryUser): void {
  const id = normalizeUserID(user.id)
  if (id <= 0) return
  if (selectedUserIds.value.includes(id)) {
    selectedUserIds.value = selectedUserIds.value.filter((item) => item !== id)
    return
  }
  selectedUserIds.value = [...selectedUserIds.value, id]
}

function removeUser(id: number | string): void {
  const normalized = normalizeUserID(id)
  selectedUserIds.value = selectedUserIds.value.filter((item) => item !== normalized)
}

async function searchUsers(): Promise<void> {
  usersLoading.value = true
  try {
    const res = await usersApi.list({
      keyword: userKeyword.value || undefined,
      status: 'active',
      page: 1,
      page_size: 20,
    })
    userOptions.value = unwrapUsers(res.data).filter((user) => normalizeUserID(user.id) > 0)
  } catch (err) {
    broadcastMessageType.value = 'error'
    broadcastMessage.value = errorMessage(err, '用户搜索失败')
  } finally {
    usersLoading.value = false
  }
}

async function sendBroadcast(): Promise<void> {
  const title = broadcastTitle.value.trim()
  const content = broadcastContent.value.trim()
  if (!title || !content) {
    broadcastMessageType.value = 'error'
    broadcastMessage.value = '请填写通知标题和详情。'
    return
  }
  if (broadcastAudience.value === 'users' && !selectedUserIds.value.length) {
    broadcastMessageType.value = 'error'
    broadcastMessage.value = '请选择至少一个接收用户。'
    return
  }

  sendingBroadcast.value = true
  try {
    const res = await notificationsApi.broadcast({
      audience: broadcastAudience.value,
      user_ids: broadcastAudience.value === 'users' ? selectedUserIds.value : undefined,
      title,
      content,
    })
    const root = res.data && typeof res.data === 'object' ? (res.data as Record<string, unknown>) : {}
    const data = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
    const count = Number(data.notification_count ?? data.recipient_count ?? 0)
    broadcastMessageType.value = 'info'
    broadcastMessage.value = `已发送 ${Number.isFinite(count) && count > 0 ? count : '多'} 条通知。`
    broadcastTitle.value = ''
    broadcastContent.value = ''
    if (broadcastAudience.value === 'users') selectedUserIds.value = []
    await notificationsStore.load()
  } catch (err) {
    broadcastMessageType.value = 'error'
    broadcastMessage.value = errorMessage(err, '广播发送失败')
  } finally {
    sendingBroadcast.value = false
  }
}

async function openNotification(item: NotificationItem): Promise<void> {
  if (!isRead(item)) {
    await notificationsStore.markRead(item.id).catch(() => undefined)
  }
  const payload = (item.payload ?? {}) as Record<string, unknown>
  const taskId = Number(payload.task_id)
  if (Number.isFinite(taskId) && taskId > 0) {
    void router.push(`/tasks/${taskId}`)
  }
}

async function markRead(id: number): Promise<void> {
  await notificationsStore.markRead(id)
}

async function markAllRead(): Promise<void> {
  await notificationsStore.readAll()
}

function errorMessage(err: unknown, fallback: string): string {
  const root = err && typeof err === 'object' ? (err as Record<string, unknown>) : {}
  const response = root.response && typeof root.response === 'object' ? (root.response as Record<string, unknown>) : {}
  const data = response.data && typeof response.data === 'object' ? (response.data as Record<string, unknown>) : {}
  const message = data.message ?? data.error ?? root.message
  return typeof message === 'string' && message.trim() ? message : fallback
}

onMounted(() => {
  void notificationsStore.load()
  if (canBroadcast.value) {
    void searchUsers()
  }
})
</script>

<style scoped>
.notif-page {
  min-height: calc(100vh - 5rem);
  padding: 1.25rem;
  background: rgb(var(--yb-surface-neutral));
  color: rgb(var(--yb-text-zinc-strong));
}

.notif-hero,
.notif-broadcast,
.notif-list-panel,
.notif-summary {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 12px 34px rgb(var(--yb-text-zinc-strong) / 0.05);
}

.notif-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-radius: 0.875rem;
  padding: 1.25rem;
}

.notif-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: rgb(var(--yb-xhs-brand));
  font-size: 0.75rem;
  font-weight: 800;
}

.notif-hero h1,
.notif-section-head h2 {
  margin: 0.25rem 0 0;
  font-size: 1.125rem;
  line-height: 1.35;
  font-weight: 900;
}

.notif-hero p,
.notif-section-head p {
  margin: 0.35rem 0 0;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.8125rem;
}

.notif-hero-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.notif-segment,
.notif-choice {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 999px;
  background: rgb(var(--yb-surface-subtle));
  padding: 0.25rem;
}

.notif-segment button,
.notif-choice button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 2rem;
  border: 0;
  border-radius: 999px;
  background: transparent;
  padding: 0 0.75rem;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.75rem;
  font-weight: 800;
}

.notif-segment button.is-active,
.notif-choice button.is-active {
  background: rgb(var(--yb-text-zinc-strong));
  color: rgb(var(--yb-surface));
  box-shadow: 0 8px 20px rgb(var(--yb-text-zinc-strong) / 0.12);
}

.notif-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 2.25rem;
  border-radius: 999px;
  padding: 0 0.85rem;
  font-size: 0.75rem;
  font-weight: 900;
  transition:
    background 0.14s ease,
    border-color 0.14s ease,
    color 0.14s ease,
    box-shadow 0.14s ease;
}

.notif-btn:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}

.notif-btn--ghost {
  border: 1px solid rgb(var(--yb-border-zinc));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-zinc));
}

.notif-btn--ghost:hover:not(:disabled) {
  border-color: rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface-subtle));
}

.notif-btn--primary {
  border: 1px solid rgb(var(--yb-xhs-brand));
  background: rgb(var(--yb-xhs-brand));
  color: rgb(var(--yb-surface));
  box-shadow: 0 12px 24px rgb(var(--yb-xhs-brand) / 0.18);
}

.notif-broadcast {
  margin-top: 1rem;
  border-radius: 0.875rem;
  padding: 1.25rem;
}

.notif-section-head {
  display: flex;
  justify-content: space-between;
  gap: 1.25rem;
}

.notif-section-head > p {
  max-width: 30rem;
  text-align: right;
}

.notif-broadcast-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(18rem, 0.7fr);
  gap: 1rem;
  margin-top: 1rem;
}

.notif-field,
.notif-recipient-panel {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.notif-field--wide {
  grid-column: 1 / -1;
}

.notif-field span {
  color: rgb(var(--yb-text-zinc-muted));
  font-size: 0.75rem;
  font-weight: 900;
}

.notif-field input,
.notif-field textarea,
.notif-recipient-search input {
  width: 100%;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-row-even));
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 0.875rem;
  outline: none;
}

.notif-field input,
.notif-recipient-search input {
  height: 2.75rem;
  padding: 0 0.85rem;
}

.notif-field textarea {
  resize: vertical;
  min-height: 6rem;
  padding: 0.75rem 0.85rem;
}

.notif-field input:focus,
.notif-field textarea:focus,
.notif-recipient-search input:focus {
  border-color: rgb(var(--yb-xhs-brand));
  background: rgb(var(--yb-surface));
  box-shadow: 0 0 0 3px rgb(var(--yb-xhs-brand) / 0.1);
}

.notif-recipient-panel {
  grid-column: 1 / -1;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.875rem;
  background: rgb(var(--yb-surface-row-even));
  padding: 0.75rem;
}

.notif-recipient-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.notif-recipient-search svg {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-zinc-faint));
}

.notif-selected-users {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.notif-selected-users button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 1px solid rgb(var(--yb-xhs-brand) / 0.22);
  border-radius: 999px;
  background: rgb(var(--yb-xhs-brand) / 0.08);
  padding: 0.35rem 0.65rem;
  color: rgb(var(--yb-xhs-brand));
  font-size: 0.75rem;
  font-weight: 800;
}

.notif-user-options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr));
  gap: 0.5rem;
}

.notif-user-option {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  min-height: 4rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  padding: 0.75rem;
}

.notif-user-option input {
  margin-top: 0.2rem;
}

.notif-user-option span {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.2rem;
}

.notif-user-option strong {
  color: rgb(var(--yb-text-zinc-deep));
  font-size: 0.8125rem;
}

.notif-user-option small {
  overflow: hidden;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notif-empty-inline {
  grid-column: 1 / -1;
  margin: 0;
  border: 1px dashed rgb(var(--yb-border-zinc-strong));
  border-radius: 0.75rem;
  padding: 1rem;
  text-align: center;
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.75rem;
}

.notif-broadcast-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 1rem;
}

.notif-broadcast-footer p {
  margin: 0;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.8125rem;
}

.notif-broadcast-footer p.is-error {
  color: rgb(var(--yb-danger));
  font-weight: 800;
}

.notif-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 17rem;
  gap: 1rem;
  margin-top: 1rem;
}

.notif-list-panel,
.notif-summary {
  border-radius: 0.875rem;
  padding: 1.25rem;
}

.notif-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 1rem;
}

.notif-row {
  display: grid;
  grid-template-columns: 1.5rem minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: flex-start;
  border: 1px solid rgb(var(--yb-xhs-brand) / 0.16);
  border-radius: 0.875rem;
  background: linear-gradient(180deg, rgb(var(--yb-xhs-brand) / 0.06), rgb(var(--yb-surface) / 0.98));
  padding: 0.9rem;
  cursor: pointer;
}

.notif-row.is-read {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.notif-row-icon {
  display: flex;
  justify-content: center;
  padding-top: 0.25rem;
  color: rgb(var(--yb-xhs-brand));
}

.notif-row.is-read .notif-row-icon {
  color: rgb(var(--yb-text-zinc-faint));
}

.notif-row-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.notif-row h3 {
  margin: 0;
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 0.9375rem;
  font-weight: 900;
}

.notif-row p {
  margin: 0.25rem 0 0;
  color: rgb(var(--yb-text-zinc-muted));
  font-size: 0.8125rem;
  line-height: 1.5;
}

.notif-row time {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.75rem;
}

.notif-row span {
  display: inline-flex;
  width: fit-content;
  margin-top: 0.55rem;
  border-radius: 999px;
  background: rgb(var(--yb-surface-neutral-muted));
  padding: 0.2rem 0.5rem;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.6875rem;
  font-weight: 800;
}

.notif-mini-btn {
  border: 1px solid rgb(var(--yb-xhs-brand));
  border-radius: 999px;
  background: rgb(var(--yb-surface));
  padding: 0.35rem 0.65rem;
  color: rgb(var(--yb-xhs-brand));
  font-size: 0.6875rem;
  font-weight: 900;
}

.notif-empty {
  display: grid;
  place-items: center;
  gap: 0.6rem;
  margin-top: 1rem;
  min-height: 14rem;
  border: 1px dashed rgb(var(--yb-border-zinc-strong));
  border-radius: 0.875rem;
  color: rgb(var(--yb-text-zinc-faint));
}

.notif-summary {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  height: fit-content;
}

.notif-summary div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  padding: 0.9rem;
}

.notif-summary span {
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.75rem;
  font-weight: 800;
}

.notif-summary strong {
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 1.25rem;
  font-weight: 950;
}

@media (max-width: 920px) {
  .notif-hero,
  .notif-section-head,
  .notif-broadcast-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .notif-section-head > p {
    max-width: none;
    text-align: left;
  }

  .notif-broadcast-grid,
  .notif-layout {
    grid-template-columns: 1fr;
  }

  .notif-hero-actions,
  .notif-recipient-search {
    flex-wrap: wrap;
  }
}

/* Phase 5: light admin notification page skin. Style-only. */
.notif-page {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  background: transparent;
  color: rgb(var(--yb-text));
}

.notif-page::before {
  display: none;
}

.notif-hero,
.notif-broadcast,
.notif-list-panel,
.notif-summary {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.notif-hero,
.notif-broadcast,
.notif-list-panel {
  border-radius: 1.1rem;
}

.notif-summary {
  border-radius: 1rem;
}

.notif-eyebrow {
  color: rgb(var(--yb-text-muted));
}

.notif-hero h1,
.notif-section-head h2,
.notif-row h3 {
  color: rgb(var(--yb-text));
  letter-spacing: 0;
}

.notif-hero p,
.notif-section-head p,
.notif-broadcast-footer p,
.notif-row p {
  color: rgb(var(--yb-text-muted));
}

.notif-field span,
.notif-summary span,
.notif-row time,
.notif-user-option small {
  color: rgb(var(--yb-text-muted));
}

.notif-segment,
.notif-choice,
.notif-recipient-panel,
.notif-empty,
.notif-empty-inline {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.notif-segment,
.notif-choice {
  box-shadow: none;
}

.notif-segment button,
.notif-choice button {
  color: rgb(var(--yb-text-muted));
  transition:
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.notif-segment button.is-active,
.notif-choice button.is-active,
.notif-btn--primary {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
  box-shadow: none;
}

.notif-btn--ghost,
.notif-mini-btn {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
}

.notif-btn--ghost:hover:not(:disabled),
.notif-mini-btn:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.notif-btn:disabled {
  opacity: 0.48;
}

.notif-field input,
.notif-field textarea,
.notif-recipient-search input {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
}

.notif-field input::placeholder,
.notif-field textarea::placeholder,
.notif-recipient-search input::placeholder {
  color: rgb(var(--yb-text-faint));
}

.notif-field input:focus,
.notif-field textarea:focus,
.notif-recipient-search input:focus {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-surface));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.notif-recipient-search svg {
  color: rgb(var(--yb-text-muted));
}

.notif-selected-users button {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.notif-user-option {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.notif-user-option:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-surface-soft));
}

.notif-user-option strong {
  color: rgb(var(--yb-text));
}

.notif-broadcast-footer p.is-error {
  color: rgb(var(--yb-danger-text));
}

.notif-row {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    box-shadow 0.16s ease;
}

.notif-row:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  transform: none;
  box-shadow: 0 2px 8px rgb(var(--yb-shadow) / 0.06);
}

.notif-row.is-read {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.notif-row-icon {
  color: rgb(var(--yb-brand));
}

.notif-row.is-read .notif-row-icon {
  color: rgb(var(--yb-text-faint));
}

.notif-row span {
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.notif-summary div {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.notif-summary strong {
  color: rgb(var(--yb-text));
  font-family: var(--yb-font-data);
}

.notif-empty,
.notif-empty-inline {
  color: rgb(var(--yb-text-muted));
}

@media (prefers-reduced-motion: reduce), (prefers-reduced-transparency: reduce) {
  .notif-row,
  .notif-btn,
  .notif-mini-btn,
  .notif-segment button,
  .notif-choice button {
    transition: none;
    transform: none;
  }
}
</style>
