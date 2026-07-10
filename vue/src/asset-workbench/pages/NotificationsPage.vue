<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowRight, CheckCheck, MailOpen, RefreshCw } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

import { assetWorkbenchApi, type NotificationRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { useWorkbenchUnreadRefresh } from '@aw/app/useWorkbenchUnread'
import { chipClass } from '@aw/shared/format/status'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

type NotificationFilter = 'all' | 'unread' | 'read'

const filter = ref<NotificationFilter>('unread')
const router = useRouter()
const rows = ref<NotificationRow[]>([])
const nextCursor = ref('')
const notice = ref('')
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/notifications')
const refreshUnreadCount = useWorkbenchUnreadRefresh()

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
  await refreshUnreadCount?.()
}

async function markAllRead() {
  await assetWorkbenchApi.markAllNotificationsRead()
  rows.value = rows.value.map((row) => ({ ...row, is_read: true, read_at: row.read_at || new Date().toISOString() }))
  notice.value = '已标记全部通知为已读'
  await refreshUnreadCount?.()
}

function filterValue(value: NotificationFilter) {
  if (value === 'read') return true
  if (value === 'unread') return false
  return undefined
}

function notificationTitle(row: NotificationRow) {
  const payload = row.payload ?? {}
  if (typeof payload.title === 'string' && payload.title.trim()) return payload.title
  if (row.notification_type === 'asset_workbench_profile_incomplete') return '个人资料待补全'
  if (row.notification_type === 'asset_workbench_submission_created') return '收到新作品'
  if (row.notification_type === 'asset_workbench_qc_updated') return '作品检查结果已更新'
  if (row.notification_type === 'asset_workbench_settlement_updated') return '结算状态已更新'
  if (row.notification_type === 'asset_workbench_supplement_access') return '补录权限已更新'
  if (row.notification_type === 'asset_workbench_preview_failed') return '文件预览生成失败'
  if (row.notification_type === 'asset_workbench_batch_job_completed') return '批量任务已完成'
  if (row.notification_type === 'asset_workbench_batch_job_failed') return '批量任务未完成'
  return '资产工作台通知'
}

function notificationContent(row: NotificationRow) {
  const payload = row.payload ?? {}
  if (typeof payload.content === 'string' && payload.content.trim()) return payload.content
  if (typeof payload.message === 'string' && payload.message.trim()) return payload.message
  if (payload.reason === 'missing_pii' || payload.action === 'complete_profile') {
    return '请在个人中心补全联系、证件和收款资料。'
  }
  if (row.notification_type === 'asset_workbench_submission_created') {
    const uploader = typeof payload.uploader_name === 'string' && payload.uploader_name.trim() ? payload.uploader_name.trim() : '用户'
    const count = Number(payload.file_count || 0)
    return `${uploader} 已上传${count > 0 ? ` ${count} 个文件` : '新作品'}，请在上传总览中查看。`
  }
  if (row.notification_type === 'asset_workbench_qc_updated') {
    return payload.qc_status === 'needs_fix' ? '作品需要修改，请进入素材网盘查看检查结果。' : '作品检查结果已更新，请进入素材网盘查看。'
  }
  if (row.notification_type === 'asset_workbench_settlement_updated') {
    return '工资结算状态已更新，请进入结算页面查看。'
  }
  if (row.notification_type === 'asset_workbench_supplement_access') {
    return payload.enabled === false ? '补录入口已关闭，请进入结算页面查看。' : '补录入口已开放，请进入结算页面查看可补录月份。'
  }
  return '资产工作台中的相关状态已更新，请进入对应页面查看。'
}

function notificationTarget(row: NotificationRow) {
  if (row.notification_type === 'asset_workbench_profile_incomplete') return '/account'
  if (row.notification_type === 'asset_workbench_submission_created') return '/upload-overview'
  if (row.notification_type === 'asset_workbench_qc_updated' || row.notification_type === 'asset_workbench_preview_failed') return '/drive'
  if (row.notification_type === 'asset_workbench_settlement_updated' || row.notification_type === 'asset_workbench_supplement_access') return '/settlement'
  if (row.notification_type === 'asset_workbench_batch_job_completed' || row.notification_type === 'asset_workbench_batch_job_failed') return '/drive'
  return ''
}

async function openNotification(row: NotificationRow) {
  await markRead(row)
  const target = notificationTarget(row)
  if (target) await router.push(target)
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
        <p class="aw-eyebrow">消息中心</p>
        <h2>{{ pageLabel }}</h2>
        <p>{{ pageSubtitle }}集中在这里处理。</p>
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
            <div class="aw-inline-actions">
              <button v-if="notificationTarget(row)" class="aw-primary-button" type="button" @click="openNotification(row)">
                查看
                <ArrowRight :size="15" aria-hidden="true" />
              </button>
              <button class="aw-secondary-button" type="button" :disabled="row.is_read" @click="markRead(row)">
                <MailOpen :size="15" aria-hidden="true" />
                {{ row.is_read ? '已读' : '标为已读' }}
              </button>
            </div>
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
