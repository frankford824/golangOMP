<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight, Bell } from 'lucide-vue-next'
import PageHead from '../components/PageHead.vue'
import CostEditor from '../components/CostEditor.vue'
import { api, assetURL, errorText, opsURL, request, send } from '../api'
import { can, currentProfile } from '../session'

const props = defineProps<{ mode: string }>()
const links: Record<string, { title: string; subtitle: string; items: { label: string; description: string; href: string }[] }> = {
  tasks: {
    title: '任务运营与设计交付',
    subtitle: '原任务的分配、设计交付、审核和结单在 OA 的任务办理区处理。',
    items: [
      { label: '任务中心', description: '打开任务列表、筛选和任务详情。', href: opsURL + '/tasks' },
      { label: '任务运营看板', description: '查看任务总览和工作进度。', href: opsURL + '/' },
      { label: '任务草稿', description: '继续办理本人已经保存的任务草稿。', href: opsURL + '/me/task-drafts' },
    ],
  },
  cost: {
    title: 'SKU 成本中心',
    subtitle: '成交核实成本在商品详情查看，当前维护价在 OA 的成本办理区更新。',
    items: [{ label: '成本规则与 SKU 维护', description: '查看和维护当前 SKU 的成本规则、价格和关联款式。', href: opsURL + '/cost-rules' }],
  },
  assets: {
    title: '资产交付与结算',
    subtitle: '进入资产工作台办理本人交付、质检和结算。',
    items: [
      { label: '我的工作台', description: '查看今日待办和个人交付进度。', href: assetURL + '/' },
      { label: '上传成品', description: '提交成品及附件。', href: assetURL + '/upload' },
      { label: '上传台账', description: '查看交付记录及质检反馈。', href: assetURL + '/upload-overview' },
      { label: '我的结算', description: '查看本人计件收入和确认批次。', href: assetURL + '/my-settlement' },
    ],
  },
}
const page = computed(() => links[props.mode])
const error = ref('')
const notice = ref('')
const busy = ref(false)
const password = ref({ old_password: '', new_password: '', password_confirmation: '' })

async function changePassword() {
  error.value = ''
  notice.value = ''
  if (password.value.new_password !== password.value.password_confirmation) {
    error.value = '两次输入的新密码不一致'
    return
  }
  busy.value = true
  try {
    await send('/v1/auth/password', password.value, 'PUT')
    notice.value = '密码已更新'
    password.value = { old_password: '', new_password: '', password_confirmation: '' }
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

type Notification = {
  id: number; notification_type?: string; type?: string; title?: string; content?: string; created_at: string; is_read: boolean
  payload?: { title?: string; content?: string; task_id?: string | number; expense_id?: string | number; fee_id?: string | number; document_id?: string | number; state?: string; site?: string }
}
const notifications = ref<Notification[]>([])
const nextCursor = ref('')
const unread = ref(false)
const expanded = ref<number>()

async function loadNotifications(append = false) {
  if (busy.value) return
  error.value = ''
  busy.value = true
  try {
    const query = new URLSearchParams({ limit: '30' })
    if (unread.value) query.set('is_read', 'false')
    if (append && nextCursor.value) query.set('cursor', nextCursor.value)
    const response = await request('/v1/me/notifications?' + query)
    const data: { data: Notification[]; next_cursor?: string } = await response.json()
    notifications.value = append ? [...notifications.value, ...(data.data ?? [])] : data.data ?? []
    nextCursor.value = data.next_cursor ?? ''
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

async function markRead(id?: number) {
  error.value = ''
  try {
    await api(id ? `/v1/me/notifications/${id}/read` : '/v1/me/notifications/read-all', { method: 'POST' })
    notifications.value = notifications.value.map(item => !id || item.id === id ? { ...item, is_read: true } : item)
  } catch (cause) { error.value = errorText(cause) }
}

function open(item: Notification) {
  expanded.value = expanded.value === item.id ? undefined : item.id
  if (!item.is_read) void markRead(item.id)
}

const notificationLabels: Record<string, string> = {
  task_design_ready: '设计已提交', task_assigned_to_me: '新的任务分配', task_rejected: '任务退回',
  task_pending_audit: '任务待审核', task_closed: '任务结单', task_cancelled: '任务取消',
  system_broadcast: '系统通知', oa_finance_document: '费用单据更新',
}
const isVisible = (item: Notification) => !unread.value || !item.is_read || expanded.value === item.id
onMounted(() => { if (props.mode === 'notifications') void loadNotifications() })
</script>

<template>
  <template v-if="page">
    <PageHead section="工作台" :title="page.title" :subtitle="page.subtitle" />
    <section class="oa-panel oa-link-list">
      <a v-for="item in page.items" :key="item.href" :href="item.href" target="_blank" rel="noopener noreferrer">
        <div><strong>{{ item.label }}</strong><p class="oa-muted">{{ item.description }}</p></div><ArrowUpRight :size="19" />
      </a>
    </section>
    <CostEditor v-if="mode === 'cost'" />
  </template>
  <template v-else-if="mode === 'account'">
    <PageHead section="个人事项" title="个人中心" :subtitle="currentProfile?.name" />
    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <div class="oa-split">
      <section class="oa-panel">
        <h2>账号与组织</h2>
        <dl class="oa-kv">
          <div><dt>姓名</dt><dd>{{ currentProfile?.name }}</dd></div>
          <div><dt>工号</dt><dd>{{ currentProfile?.employee_no == null ? '—' : String(currentProfile.employee_no).padStart(4, '0') }}</dd></div>
          <div><dt>部门</dt><dd>{{ currentProfile?.department || '—' }}</dd></div>
          <div><dt>团队</dt><dd>{{ currentProfile?.team || '—' }}</dd></div>
          <div><dt>管理权限</dt><dd>{{ currentProfile?.super_admin ? '超级管理员' : '按账号授权' }}</dd></div>
        </dl>
        <div class="oa-actions">
          <RouterLink v-if="can('orders.view')" class="oa-button" to="/products">查看本人业绩</RouterLink>
          <RouterLink class="oa-button" to="/fees/mine">我的申请</RouterLink>
          <a class="oa-button" :href="opsURL + '/me/task-drafts'" target="_blank" rel="noopener noreferrer">任务草稿 <ArrowUpRight :size="13" /></a>
        </div>
      </section>
      <section class="oa-panel">
        <h2>安全设置</h2>
        <form class="oa-form" @submit.prevent="changePassword">
          <label>当前密码<input v-model="password.old_password" type="password" autocomplete="current-password" required /></label>
          <label>新密码<input v-model="password.new_password" type="password" autocomplete="new-password" required /></label>
          <label>确认新密码<input v-model="password.password_confirmation" type="password" autocomplete="new-password" required /></label>
          <button class="oa-button-primary" :disabled="busy">修改密码</button>
        </form>
      </section>
    </div>
  </template>
  <template v-else>
    <PageHead section="个人事项" title="通知中心">
      <button :aria-pressed="!unread" @click="unread = false; loadNotifications()">全部</button>
      <button :aria-pressed="unread" @click="unread = true; loadNotifications()">未读</button>
      <button :disabled="busy || !notifications.some(item => !item.is_read)" @click="markRead()">全部已读</button>
    </PageHead>
    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <section class="oa-panel">
      <p v-if="busy && !notifications.length" class="oa-empty" role="status">正在读取通知…</p>
      <p v-else-if="!notifications.some(isVisible) && !error" class="oa-empty">暂无{{ unread ? '未读' : '' }}通知</p>
      <article v-for="item in notifications.filter(isVisible)" :key="item.id" class="oa-notification">
        <button :aria-expanded="expanded === item.id" @click="open(item)">
          <Bell :size="16" />
          <strong>{{ item.title || item.payload?.title || notificationLabels[item.notification_type || item.type || ''] || '业务通知' }}</strong>
          <time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time>
          <span v-if="!item.is_read" class="oa-pill">未读</span>
        </button>
        <div v-if="expanded === item.id" class="oa-notification-detail">
          <p>{{ item.content || item.payload?.content }}</p>
          <a v-if="item.payload?.task_id" :href="opsURL + '/tasks/' + encodeURIComponent(item.payload.task_id)" target="_blank" rel="noopener noreferrer">查看原任务 <ArrowUpRight :size="13" /></a>
          <RouterLink v-if="item.payload?.document_id || item.payload?.expense_id || item.payload?.fee_id" :to="{ path: '/fees/document', query: { id: item.payload?.document_id || item.payload?.expense_id || item.payload?.fee_id, site: item.payload?.site } }">查看费用单据</RouterLink>
        </div>
      </article>
      <button v-if="nextCursor" :disabled="busy" @click="loadNotifications(true)">加载更多</button>
    </section>
  </template>
</template>
