<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CheckCheck, MailOpen, RefreshCw } from 'lucide-vue-next'

import { assetWorkbenchApi, type NotificationRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { chipClass } from '@aw/shared/format/status'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

type NotificationFilter = 'all' | 'unread' | 'read'

const filter = ref<NotificationFilter>('unread')
const rows = ref<NotificationRow[]>([])
const nextCursor = ref('')
const notice = ref('')

const notificationsRequest = usePageRequest(
  () => assetWorkbenchApi.listNotifications({ limit: 30, is_read: filterValue(filter.value) }),
  { items: [], next_cursor: '' },
  '通知加载失败',
)
const loading = notificationsRequest.loading
const error = notificationsRequest.error
const unreadCount = computed(() => rows.value.filter((item) => !item.is_read).length)

async function loadNotifications() {
  notice.value = ''
  const result = await notificationsRequest.run()
  if (!result) return
  rows.value = result.items
  nextCursor.value = result.next_cursor || ''
}

async function loadMore() {
  if (!nextCursor.value || loading.value) return
  const result = await assetWorkbenchApi.listNotifications({
    limit: 30,
    cursor: nextCursor.value,
    is_read: filterValue(filter.value),
  })
  rows.value = rows.value.concat(result.items)
  nextCursor.value = result.next_cursor || ''
}

async function markRead(row: NotificationRow) {
  if (row.is_read) return
  await assetWorkbenchApi.markNotificationRead(row.id)
  row.is_read = true
  row.read_at = new Date().toISOString()
}

async function markAllRead() {
  await assetWorkbenchApi.markAllNotificationsRead()
  rows.value = rows.value.map((row) => ({ ...row, is_read: true, read_at: row.read_at || new Date().toISOString() }))
  notice.value = '已标记全部通知为已读'
}

function filterValue(value: NotificationFilter) {
  if (value === 'read') return true
  if (value === 'unread') return false
  return undefined
}

function notificationTitle(row: NotificationRow) {
  const payload = row.payload ?? {}
  if (typeof payload.title === 'string' && payload.title.trim()) return payload.title
  if (row.notification_type === 'system_broadcast') return '系统通知'
  if (row.notification_type === 'task_assigned_to_me') return '任务分配'
  if (row.notification_type === 'task_rejected') return '任务驳回'
  if (row.notification_type === 'task_pending_audit') return '待审核提醒'
  if (row.notification_type === 'task_closed') return '任务已结单'
  if (row.notification_type === 'task_sku_sync_failed') return 'SKU 同步异常'
  return row.notification_type
}

function notificationContent(row: NotificationRow) {
  const payload = row.payload ?? {}
  if (typeof payload.content === 'string' && payload.content.trim()) return payload.content
  if (typeof payload.message === 'string' && payload.message.trim()) return payload.message
  if (payload.reason === 'missing_pii' || payload.action === 'complete_profile') {
    return '请在个人中心补全联系、证件和收款资料。'
  }
  return '查看并处理这条待办。'
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

onMounted(() => {
  void loadNotifications()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">通知</p>
        <h2>通知与待办</h2>
        <p>资产工作台相关提醒、资料补录和系统通知集中在这里处理。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="loadNotifications">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
        <button class="aw-primary-button" type="button" :disabled="!rows.length" @click="markAllRead">
          <CheckCheck :size="16" aria-hidden="true" />
          全部已读
        </button>
      </div>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <select v-model="filter" aria-label="通知筛选" @change="loadNotifications()">
          <option value="unread">未读</option>
          <option value="all">全部</option>
          <option value="read">已读</option>
        </select>
        <span>{{ rows.length }} 条</span>
        <span class="aw-chip aw-chip--warn">未读 {{ unreadCount }}</span>
      </div>
      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <AsyncBoundary :loading="loading" :error="error" :empty="!rows.length" loading-label="正在加载通知" empty-label="暂无通知" @retry="loadNotifications">
        <div class="aw-compact-list">
          <article v-for="row in rows" :key="row.id" class="aw-compact-list__item">
            <div>
              <strong>{{ notificationTitle(row) }}</strong>
              <span>{{ notificationContent(row) }}</span>
              <span class="aw-copy">{{ formatDateTime(row.created_at) }}</span>
            </div>
            <button class="aw-secondary-button" type="button" :disabled="row.is_read" @click="markRead(row)">
              <MailOpen :size="15" aria-hidden="true" />
              {{ row.is_read ? '已读' : '标为已读' }}
            </button>
            <span :class="chipClass(row.is_read ? 'neutral' : 'warn')">{{ row.is_read ? '已读' : '未读' }}</span>
          </article>
        </div>
        <div v-if="nextCursor" class="aw-button-row">
          <button class="aw-secondary-button" type="button" @click="loadMore">加载更多</button>
        </div>
      </AsyncBoundary>
    </div>
  </section>
</template>
