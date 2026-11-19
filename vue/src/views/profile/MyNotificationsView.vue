<template>
  <section class="notif-page">
    <header v-if="!oaEnabled || section === 'system'" class="notif-hero" :class="{ 'notif-toolbar': oaEnabled }">
      <div>
        <h1 v-if="!oaEnabled">通知中心</h1>
        <p v-if="section === 'system'">{{ oaEnabled ? "本页" : "共" }} {{ visibleTotal }} 条 · {{ unreadCount }} 条未读</p>
      </div>
      <div v-if="section === 'system'" class="notif-hero-actions">
        <div class="notif-segment">
          <button
            v-for="option in filterOptions"
            :key="option.value"
            type="button"
            :class="{ 'is-active': filter === option.value }"
            :aria-pressed="filter === option.value"
            @click="filter = option.value"
          >
            {{ option.label }}
          </button>
        </div>
        <button
          type="button"
          class="notif-btn notif-btn--ghost"
          :disabled="unreadCount === 0 || markingRead"
          @click="markAllRead"
        >
          <CheckCircle2 class="h-4 w-4" />
          全部已读
        </button>
      </div>
    </header>

    <DesktopSlot v-if="oaEnabled && (showExpenseActivity || showCostActivity || currentProfile?.super_admin)" target="tabs">
      <nav class="notif-oa-segment" aria-label="通知分类">
        <RouterLink :to="{ name: 'MeNotifications' }" :aria-current="section === 'system' ? 'page' : undefined">系统通知</RouterLink>
        <RouterLink v-if="showExpenseActivity" :to="{ name: 'MeNotifications', query: { section: 'expense' } }"
          :aria-current="section === 'expense' ? 'page' : undefined">费用动态</RouterLink>
        <RouterLink v-if="showCostActivity" :to="{ name: 'MeNotifications', query: { section: 'cost' } }"
          :aria-current="section === 'cost' ? 'page' : undefined">成本复核</RouterLink>
        <RouterLink v-if="currentProfile?.super_admin" :to="{ name: 'MeNotifications', query: { section: 'data-alerts' } }"
          :aria-current="section === 'data-alerts' ? 'page' : undefined">数据告警</RouterLink>
      </nav>
    </DesktopSlot>
    <ExpenseNotifications v-if="section === 'expense'" :key="expenseTab + ':' + expensePage" :initial-tab="expenseTab" :initial-page="expensePage" />
    <CostRequestInbox v-else-if="section === 'cost'" :key="router.currentRoute.value.fullPath" />
    <DataAlerts v-else-if="section === 'data-alerts'" />
    <div v-else class="notif-layout">
      <section class="notif-list-panel" :aria-busy="oaEnabled ? pageLoading : undefined">
        <div class="notif-section-head">
          <div>
            <h2>通知记录</h2>
          </div>
        </div>

        <div v-if="oaEnabled && pageError" class="notif-empty" role="alert">
          <p>{{ pageError }}</p>
          <button type="button" class="notif-btn notif-btn--ghost" @click="reloadPage">重试</button>
        </div>
        <p v-else-if="oaEnabled && pageLoading" class="notif-empty" role="status">正在加载通知…</p>
        <div v-else-if="visibleItems.length" class="notif-list desktop-region-scroll" role="region" aria-label="通知记录" tabindex="0">
          <article
            v-for="item in visibleItems"
            :key="item.id"
            class="notif-row"
            :class="{ 'is-read': isRead(item) }"
            @click="!oaEnabled && openNotification(item)"
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
              <span v-if="!oaEnabled || displayType(item) !== displayTitle(item)" :class="oaEnabled && notificationTone(item)">{{ displayType(item) }}</span>
              <RouterLink
                v-if="oaEnabled && notificationTaskID(item)"
                v-slot="{ href }"
                :to="taskDetailLocation(notificationTaskID(item)!, route.fullPath)"
                custom
              >
                <a :href="href" class="notif-task-link" @click="openOATask($event, item)">查看任务</a>
              </RouterLink>
            </div>
            <button
              v-if="!isRead(item)"
              type="button"
              class="notif-mini-btn"
              :disabled="markingRead"
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
        <p v-if="oaEnabled && actionError" class="notif-action-error" role="alert">{{ actionError }}</p>
        <nav v-if="oaEnabled && (pageNumber > 1 || nextCursor)" class="notif-pagination" aria-label="系统通知分页">
          <button type="button" class="notif-btn notif-btn--ghost" :disabled="pageNumber === 1 || pageLoading || markingRead" @click="openPreviousPage">{{ previousPageLabel }}</button>
          <span aria-live="polite">第 {{ pageNumber }} 页</span>
          <button type="button" class="notif-btn notif-btn--ghost" :disabled="!nextCursor || pageLoading || markingRead" @click="openPage(pageNumber + 1, nextCursor)">下一页</button>
        </nav>
      </section>

      <aside class="notification-tools desktop-region-scroll" aria-label="通知设置与广播" tabindex="0">
        <nav v-if="canBroadcast" class="notification-tool-nav" aria-label="通知工具">
          <button type="button" :aria-pressed="toolPane==='settings'" @click="toolPane='settings'">通知设置</button>
          <button type="button" :aria-pressed="toolPane==='broadcast'" @click="toolPane='broadcast'">发广播</button>
        </nav>
        <section v-show="toolPane==='settings' || !canBroadcast" class="notif-webpush">
          <div class="notif-section-head">
            <div>
              <div class="notif-eyebrow">
                <BellRing class="h-4 w-4" />
                系统通知
              </div>
              <h2>Web Push 提醒</h2>
            </div>
            <p>{{ webPushStatusText }}</p>
          </div>
          <div class="notif-webpush-actions">
            <button
              v-if="!webPushActive"
              type="button"
              class="notif-btn notif-btn--primary"
              :disabled="webPushLoading || !webPushCanEnable"
              @click="enableWebPush"
            >
              <BellRing class="h-4 w-4" />
              开启系统通知
            </button>
            <button
              v-else
              type="button"
              class="notif-btn notif-btn--ghost"
              :disabled="webPushLoading"
              @click="disableWebPush"
            >
              关闭系统通知
            </button>
            <button
              type="button"
              class="notif-btn notif-btn--ghost"
              :disabled="webPushLoading || !webPushActive"
              @click="sendWebPushTest"
            >
              <Send class="h-4 w-4" />
              发送测试
            </button>
          </div>
          <p v-if="webPushMessage" class="notif-webpush-message" :class="{ 'is-error': webPushMessageType === 'error' }">
            {{ webPushMessage }}
          </p>
        </section>

        <section v-if="canBroadcast" v-show="toolPane==='broadcast'" class="notif-broadcast">
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
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onScopeDispose, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DesktopSlot from '@/oa/components/DesktopSlot.vue'
import { taskDetailLocation } from '@/oa/taskNavigation'
import {
  BellRing,
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
import type { TaskSKUSyncFailedPayload } from '@/services/v1Types'
import { oaEnabled, useOAAccess } from '@/oa/access'
import { expenseActivityActions } from '@/oa/expenseActivity'
import { notificationsApi } from '@/services/api/notificationsApi'
import { usersApi } from '@/services/api/usersApi'
import { useNotificationsStore, type NotificationItem } from '@/stores/notifications.store'
import { usePermissionsStore } from '@/stores/permissions'
import { useWebPushStore } from '@/stores/webPush.store'
import { PermissionEnum } from '@/types'
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
const route = useRoute()
const { can: canOA, currentProfile } = useOAAccess()
const showExpenseActivity = computed(() => expenseActivityActions.some(canOA))
const showCostActivity = computed(() => canOA('orders.view') || canOA('cost.request.handle'))
const section = computed(() => {
  const requested = route.query.section
  if (requested === 'data-alerts' && currentProfile.value?.super_admin) return 'data-alerts'
  if (requested === 'expense' && showExpenseActivity.value) return 'expense'
  if (requested === 'cost' && showCostActivity.value) return 'cost'
  return 'system'
})
const expensePage = computed(() => {
  const raw = Number(route.query.expensePage)
  return Number.isSafeInteger(raw) && raw >= 1 && raw <= 1_000_000 ? raw : 1
})
const expenseTab = computed(() => route.query.activity === 'results' ? 'results' : 'todos')
const ExpenseNotifications = defineAsyncComponent(() => import('@/oa/components/ExpenseNotifications.vue'))
const DataAlerts = defineAsyncComponent(() => import('@/oa/components/DataAlerts.vue'))
const CostRequestInbox = defineAsyncComponent(() => import('@/oa/components/CostRequestInbox.vue'))
const notificationsStore = useNotificationsStore()
const permissionsStore = usePermissionsStore()
const webPushStore = useWebPushStore()

const items = computed(() => notificationsStore.items)
const legacyFilter = ref<FilterMode>('all')
const filter = computed<FilterMode>({
  get: () => oaEnabled ? (route.query.filter === 'unread' ? 'unread' : 'all') : legacyFilter.value,
  set: (value) => {
    if (!oaEnabled) { legacyFilter.value = value; return }
    if (value !== filter.value) {
      void router.push({ name: 'MeNotifications', query: { filter: value === 'unread' ? 'unread' : undefined } })
    }
  },
})
const toolPane = ref<'settings' | 'broadcast'>('settings')
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
const webPushMessage = ref('')
const webPushMessageType = ref<MessageType>('info')

const filterOptions: Array<{ label: string; value: FilterMode }> = [
  { label: '全部', value: 'all' },
  { label: '未读', value: 'unread' },
]

const pageSize = 20
const pageItems = ref<NotificationItem[]>([])
const nextCursor = ref('')
const pageLoading = ref(false)
const pageError = ref('')
const actionError = ref('')
const markingRead = ref(false)
const pageCursor = computed(() => typeof route.query.cursor === 'string' ? route.query.cursor : '')
const pageNumber = computed(() => {
  const value = typeof route.query.page === 'string' ? Number(route.query.page) : 1
  return pageCursor.value && Number.isSafeInteger(value) && value > 1 ? value : 1
})
const activeCursor = computed(() => pageNumber.value > 1 ? pageCursor.value : '')
const cursorHistoryLimit = 48
const cursorHistoryKey = computed(() => {
  const id = currentProfile.value?.id
  return id ? 'oa-notification-cursors:' + id : undefined
})
const previousCursor = computed(() => {
  const value = route.query.previousCursor
  return typeof value === 'string' && value && value !== activeCursor.value
    ? value
    : readCursorHistory(pageNumber.value, activeCursor.value)
})
const previousPageLabel = computed(() => pageNumber.value > 2 && !previousCursor.value ? '回到首页' : '上一页')
let pageRequest: AbortController | undefined

const unreadCount = computed(() => notificationsStore.unreadCount)
const visibleItems = computed(() => {
  if (oaEnabled) return pageItems.value
  return filter.value === 'unread' ? notificationsStore.unreadItems : items.value
})
const visibleTotal = computed(() => oaEnabled ? pageItems.value.length : items.value.length)
const emptyText = computed(() => (filter.value === 'unread' ? '暂无未读通知' : '暂无通知'))
const canBroadcast = computed(() => permissionsStore.hasAction(PermissionEnum.SYSTEM_MANAGE))
const canBroadcastAll = canBroadcast
const selectedUsers = computed(() => {
  const byID = new Map(userOptions.value.map((user) => [normalizeUserID(user.id), user]))
  return selectedUserIds.value.map((id) => byID.get(id) ?? ({ id, display_name: `用户 ${id}` } as DirectoryUser))
})
const webPushActive = computed(() => webPushStore.active)
const webPushLoading = computed(() => webPushStore.loading)
const webPushCanEnable = computed(() => webPushStore.supported && webPushStore.serverEnabled)
const webPushStatusText = computed(() => {
  if (!webPushStore.supported) return '当前浏览器或当前访问环境不支持系统级通知。'
  if (!webPushStore.serverEnabled) return '服务端 Web Push 尚未启用。'
  if (webPushStore.permission === 'denied') return '浏览器已拒绝通知权限，需要在浏览器设置中重新允许。'
  if (webPushStore.active) return '已开启，浏览器关闭或后台时仍可收到高优先级提醒。'
  return '开启后，SKU同步失败等高优先级通知会进入系统通知。'
})

function isRead(item: NotificationItem): boolean {
  return Boolean(item.read ?? item.is_read)
}

function notificationText(item: NotificationItem) {
  return formatNotification(item.notification_type, item.payload as Record<string, unknown> | undefined)
}

function displayTitle(item: NotificationItem): string {
  if (oaEnabled && item.notification_type === 'task_sku_sync_failed') {
    const payload = item.payload as TaskSKUSyncFailedPayload | undefined
    const sku = payload?.failed_items?.[0]?.sku_code?.trim()
    const count = payload?.failed_count ?? payload?.failed_items?.length ?? 0
    if (sku) return count > 1 ? `SKU ${sku} 等 ${count} 项同步至 ERP 失败` : `SKU ${sku} 同步至 ERP 失败`
    if (payload?.task_no) return `任务 ${payload.task_no} 的 SKU 同步至 ERP 失败`
    return 'SKU 同步至 ERP 失败'
  }
  return item.title ?? notificationText(item).title
}

function displayContent(item: NotificationItem): string {
  if (oaEnabled && item.notification_type === 'task_sku_sync_failed') {
    const payload = item.payload as TaskSKUSyncFailedPayload | undefined
    const reason = [item.content, payload?.summary, ...(payload?.failed_items?.map((entry) => entry.error) ?? [])]
      .filter(Boolean).join(' ')
    if (/erp bridge request failed|timeout|deadline exceeded/i.test(reason)) {
      return '系统未能从 ERP 接口取得成功响应。请查看任务中的同步状态后再重试。'
    }
    return notificationTaskID(item)
      ? 'ERP 同步未完成。请打开任务查看 SKU 同步状态。'
      : 'ERP 同步未完成。请按 SKU 在成本规则或资产中心核对同步状态。'
  }
  return item.content ?? notificationText(item).content
}

function displayType(item: NotificationItem): string {
  if (oaEnabled && item.notification_type === 'task_sku_sync_failed') return 'ERP 同步异常'
  if (oaEnabled && item.notification_type === 'task_pending_audit') return '审批提醒'
  if (item.notification_type === 'system_broadcast') return '系统广播'
  if (item.notification_type === 'task_assigned_to_me') return '任务分配'
  if (item.notification_type === 'task_pending_audit') return '待审核'
  if (item.notification_type === 'task_design_ready') return '图已完成'
  if (item.notification_type === 'task_closed') return '已结单'
  if (item.notification_type === 'task_rejected') return '任务驳回'
  if (item.notification_type === 'task_cancelled') return '任务取消'
  if (item.notification_type === 'task_sku_sync_failed') return 'SKU同步失败'
  return '系统通知'
}

function notificationTone(item: NotificationItem): string {
  if (item.notification_type === 'task_sku_sync_failed') return 'notif-type-error'
  if (item.notification_type === 'task_rejected') return 'notif-type-warning'
  return 'notif-type-info'
}

function notificationTaskID(item: NotificationItem): number | undefined {
  const taskId = Number((item.payload as Record<string, unknown> | undefined)?.task_id)
  return Number.isSafeInteger(taskId) && taskId > 0 ? taskId : undefined
}

async function fetchPage(mode: FilterMode, cursor: string, signal: AbortSignal) {
  const { data: body } = await notificationsApi.list({
    limit: pageSize,
    is_read: mode === 'unread' ? false : undefined,
    cursor: cursor || undefined,
  }, signal)
  return {
    items: Array.isArray(body?.data) ? body.data as NotificationItem[] : [],
    next: typeof body?.next_cursor === 'string' ? body.next_cursor : '',
  }
}

function cursorHistory() {
  try {
    const key = cursorHistoryKey.value
    if (key) {
      const saved: unknown = JSON.parse(sessionStorage.getItem(key) ?? '{}')
      if (saved && typeof saved === 'object' && !Array.isArray(saved)) return saved as Record<string, string>
    }
  } catch {
    // Links carry one previous cursor even when browser storage is unavailable.
  }
  return {}
}

function cursorKey(page: number, cursor: string): string {
  return filter.value + ':' + page + ':' + cursor
}

function readCursorHistory(page: number, cursor: string): string | undefined {
  const previous = cursorHistory()[cursorKey(page, cursor)]
  return typeof previous === 'string' && previous !== cursor ? previous : undefined
}

function rememberPreviousCursor(page: number, current: string, previous: string): void {
  const key = cursorHistoryKey.value
  if (!key) return
  try {
    const history = cursorHistory()
    const entry = cursorKey(page, current)
    delete history[entry]
    history[entry] = previous
    sessionStorage.setItem(key, JSON.stringify(Object.fromEntries(Object.entries(history).slice(-cursorHistoryLimit))))
  } catch {
    // The previous cursor remains available through this page's URL.
  }
}

async function reloadPage(): Promise<void> {
  pageRequest?.abort()
  if (!oaEnabled || route.name !== 'MeNotifications' || section.value !== 'system') return

  const request = new AbortController()
  pageRequest = request
  const mode = filter.value
  const cursor = activeCursor.value
  pageLoading.value = true
  pageError.value = ''
  pageItems.value = []
  nextCursor.value = ''
  try {
    const result = await fetchPage(mode, cursor, request.signal)
    if (request.signal.aborted) return
    pageItems.value = result.items
    nextCursor.value = result.next
    if (result.next) rememberPreviousCursor(pageNumber.value + 1, result.next, cursor)
  } catch (err) {
    if (!request.signal.aborted) pageError.value = errorMessage(err, '通知加载失败')
  } finally {
    if (!request.signal.aborted) pageLoading.value = false
  }
}

function openPage(page: number, cursor = '') {
  const previous = page === pageNumber.value + 1 ? activeCursor.value : readCursorHistory(page, cursor)
  return router.push({
    name: 'MeNotifications',
    query: {
      filter: filter.value === 'unread' ? 'unread' : undefined,
      page: cursor ? String(page) : undefined,
      cursor: cursor || undefined,
      previousCursor: page > 2 ? previous || undefined : undefined,
    },
  })
}

function openPreviousPage() {
  if (pageNumber.value > 2 && previousCursor.value) {
    return openPage(pageNumber.value - 1, previousCursor.value)
  }
  return openPage(1)
}

async function openOATask(event: MouseEvent, item: NotificationItem): Promise<void> {
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  const taskId = notificationTaskID(item)
  if (!taskId || markingRead.value) return
  const source = route.fullPath
  if (!isRead(item)) {
    markingRead.value = true
    try {
      await notificationsApi.markRead(item.id)
      void refreshNotificationCount()
    } catch {
      // Task access is independent of marking the reminder read.
    } finally {
      markingRead.value = false
    }
  }
  if (route.fullPath === source) await router.push(taskDetailLocation(taskId, source))
}

async function refreshNotificationCount(): Promise<void> {
  await notificationsStore.refreshUnreadCount().catch(() => undefined)
}

function displayTime(createdAt: string | undefined): string {
  if (oaEnabled) return formatDateTimeBeijing(createdAt) || '—'
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
    if (oaEnabled) await Promise.all([reloadPage(), refreshNotificationCount()])
    else await notificationsStore.load()
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
  if (!oaEnabled) {
    await notificationsStore.markRead(id)
    return
  }
  await updateReadState(() => notificationsApi.markRead(id), '标为已读失败')
}

async function markAllRead(): Promise<void> {
  if (!oaEnabled) {
    await notificationsStore.readAll()
    return
  }
  await updateReadState(() => notificationsApi.readAll(), '全部标为已读失败')
}

async function updateReadState(update: () => Promise<unknown>, fallback: string): Promise<void> {
  if (markingRead.value) return
  markingRead.value = true
  actionError.value = ''
  try {
    await update()
    await Promise.all([reloadPage(), refreshNotificationCount()])
  } catch (err) {
    actionError.value = errorMessage(err, fallback)
  } finally {
    markingRead.value = false
  }
}

async function enableWebPush(): Promise<void> {
  webPushMessage.value = ''
  try {
    await webPushStore.enable()
    webPushMessageType.value = 'info'
    webPushMessage.value = '系统通知已开启。'
  } catch (err) {
    webPushMessageType.value = 'error'
    webPushMessage.value = webPushStore.errorMessage || errorMessage(err, '开启系统通知失败')
  }
}

async function disableWebPush(): Promise<void> {
  webPushMessage.value = ''
  try {
    await webPushStore.disable()
    webPushMessageType.value = 'info'
    webPushMessage.value = '系统通知已关闭。'
  } catch (err) {
    webPushMessageType.value = 'error'
    webPushMessage.value = errorMessage(err, '关闭系统通知失败')
  }
}

async function sendWebPushTest(): Promise<void> {
  webPushMessage.value = ''
  try {
    await webPushStore.sendTest()
    webPushMessageType.value = 'info'
    webPushMessage.value = '测试通知已发送，请留意系统通知。'
  } catch (err) {
    webPushMessageType.value = 'error'
    webPushMessage.value = webPushStore.errorMessage || errorMessage(err, '发送测试通知失败')
  }
}

function errorMessage(err: unknown, fallback: string): string {
  const root = err && typeof err === 'object' ? (err as Record<string, unknown>) : {}
  const response = root.response && typeof root.response === 'object' ? (root.response as Record<string, unknown>) : {}
  const data = response.data && typeof response.data === 'object' ? (response.data as Record<string, unknown>) : {}
  const message = data.message ?? data.error ?? root.message
  return typeof message === 'string' && message.trim() ? message : fallback
}

if (oaEnabled) {
  watch([() => route.name, section, filter, pageNumber, activeCursor], () => {
    actionError.value = ''
    void reloadPage()
  }, { immediate: true })
  const recentNotificationState = computed(() => notificationsStore.items
    .map((item) => item.id + ':' + isRead(item))
    .join(','))
  watch([unreadCount, recentNotificationState], () => {
    if (!markingRead.value) void reloadPage()
  })
  onScopeDispose(() => pageRequest?.abort())
}

onMounted(() => {
  if (oaEnabled) void refreshNotificationCount()
  else void notificationsStore.load()
  void webPushStore.refresh().catch(() => undefined)
  if (canBroadcast.value) {
    void searchUsers()
  }
})
</script>

<style scoped>
.notif-oa-segment { display: flex; flex-wrap: wrap; gap: 8px; align-items: stretch; }
.notif-oa-segment a { display: inline-flex; align-items: center; min-height: 40px; padding-inline: 12px; border-bottom: 2px solid transparent; }
.notif-oa-segment a[aria-current='page'] { border-color: var(--oa-accent); color: var(--oa-accent); font-weight: 600; }
.notif-page {
  min-height: calc(100vh - 5rem);
  padding: 1.25rem;
  background: rgb(var(--yb-surface-neutral));
  color: rgb(var(--yb-text-zinc-strong));
}

.notif-hero,
.notif-webpush,
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

.notif-webpush {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  margin-top: 1rem;
  border-radius: 0.875rem;
  padding: 1.25rem;
}

.notif-webpush-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.notif-webpush-message {
  margin: 0;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.8125rem;
  font-weight: 800;
}

.notif-webpush-message.is-error {
  color: rgb(var(--yb-danger));
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
.notification-tools {
  min-width: 0;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
}

.notification-tool-nav {
  display: flex;
  gap: 0.35rem;
  padding: 0.55rem 0.65rem 0;
  border-bottom: 1px solid rgb(var(--yb-border));
}

.notification-tool-nav button {
  min-height: 2rem;
  padding: 0 0.55rem;
  border-bottom: 2px solid transparent;
  color: rgb(var(--yb-text-muted));
  font-size: 0.8125rem;
}

.notification-tool-nav button[aria-pressed='true'] {
  border-bottom-color: rgb(var(--yb-brand));
  color: rgb(var(--yb-brand));
  font-weight: 700;
}

.notification-tools :is(.notif-webpush,.notif-broadcast) {
  padding: 0.85rem;
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

.notification-tools .notif-broadcast-grid { grid-template-columns: minmax(0,1fr); }
.notification-tools .notif-section-head { align-items: start; flex-direction: column; gap: 0.45rem; }
.notification-tools .notif-section-head > p { text-align: left; }
.notification-tools :is(.notif-webpush-actions,.notif-broadcast-footer,.notif-recipient-search) { flex-wrap: wrap; }
.notification-tools .notif-recipient-search input { min-width: 0; }

@media (min-width: 901px) {
  .notif-page {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    block-size: calc(100dvh - 6rem);
    min-block-size: 23rem;
  }
  .notif-hero { flex: 0 0 auto; padding: 0.85rem 1rem; }
  .notif-layout {
    flex: 1 1 auto;
    min-height: 0;
    grid-template-columns: minmax(0,1fr) minmax(17rem,23rem);
    align-items: stretch;
    gap: 0.7rem;
  }
  .notif-list-panel { display: flex; flex-direction: column; min-height: 0; }
  .notif-list-panel:not(:has(.notif-list)) { align-self: start; }
  .notif-list { flex: 1 1 auto; align-content: start; }
  .notif-empty { min-height: 7rem; align-content: center; }
  .notification-tools { align-self: start; max-block-size: 100%; }
}
html.oa-live-ui .notif-row { cursor: default; }
html.oa-live-ui .notif-row .notif-type-error { background: rgb(var(--yb-danger-soft)); color: rgb(var(--yb-danger-text)); }
html.oa-live-ui .notif-row .notif-type-warning { background: rgb(var(--yb-warning-soft)); color: rgb(var(--yb-warning-text)); }
.notif-pagination {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  padding-top: 0.75rem;
  font-size: 0.8125rem;
}
.notif-action-error { color: rgb(var(--yb-danger-text)); }
.notif-task-link { margin-left: 8px; font-size: 13px; color: var(--oa-primary, rgb(var(--yb-brand))); }

.notif-page .notif-toolbar { background:transparent; padding:0; margin:0; border:0; border-radius:0; box-shadow:none; min-height:32px; }
.notif-toolbar .notif-segment,.notif-toolbar .notif-segment button,.notif-toolbar .notif-btn { border-radius:6px; }
</style>
