<template>
  <div class="logs-management-view min-h-[100dvh]">
    <div class="page-header">
      <h2 class="page-title">日志管理</h2>
    </div>
    <div v-if="!canView" class="mt-6">
      <BaseEmptyState title="无查看权限" description="仅超级管理员或人事管理员可查看操作日志。" />
    </div>
    <template v-else>
      <div class="tabs mb-4">
        <button
          type="button"
          class="tab"
          :class="{ active: activeTab === 'operation' }"
          @click="activeTab = 'operation'"
        >
          操作日志
        </button>
        <button
          type="button"
          class="tab"
          :class="{ active: activeTab === 'permission' }"
          @click="activeTab = 'permission'"
        >
          权限日志
        </button>
        <!-- v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节，仅 Admin 可见 -->
        <button
          v-if="canViewServerLogs"
          type="button"
          class="tab"
          :class="{ active: activeTab === 'server' }"
          @click="activeTab = 'server'"
        >
          服务器日志
        </button>
      </div>

      <!-- 操作日志（GET /v1/operation-logs：任务 / 导出 / 集成调用聚合） -->
      <section v-show="activeTab === 'operation'" class="content-card">
        <h3 class="section-title">操作日志</h3>
        <div class="filter-row">
          <select v-model="opSource" class="filter-select">
            <option value="">全部来源</option>
            <option value="task_event">任务事件</option>
            <option value="export_event">导出任务</option>
            <option value="integration_call">集成调用</option>
          </select>
          <input
            v-model="opEventType"
            type="text"
            class="filter-input"
            placeholder="事件类型（可选，任务/导出）"
          />
          <BaseButton variant="primary" size="sm" @click="applyOperationFilters">查询</BaseButton>
        </div>
        <div v-if="opLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="opError" :title="opError" @retry="loadOperationLogs" />
        <BaseEmptyState
          v-else-if="!operationItems.length"
          title="暂无操作日志"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <table class="simple-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>来源</th>
                <th>摘要</th>
                <th>事件类型</th>
                <th>操作者</th>
                <th>引用</th>
                <th>状态</th>
                <th>载荷</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in operationItems" :key="r.log_id">
                <td>{{ formatAt(r.created_at) }}</td>
                <td>{{ opSourceLabel(r.source) }}</td>
                <td class="summary-cell">{{ r.summary || '—' }}</td>
                <td class="text-xs text-slate-800" :title="r.event_type || undefined">
                  {{ getOperationEventTypeLabel(r.event_type) }}
                </td>
                <td>{{ formatOperationActor(r) }}</td>
                <td class="ref-cell">
                  <span class="text-slate-600" :title="r.reference_type || undefined">{{ referenceTypeLabel(r.reference_type) }}</span>
                  <span class="font-mono text-xs block">{{ r.reference_id }}</span>
                </td>
                <td :title="r.status || undefined">{{ statusLabel(r.status) }}</td>
                <td class="json-cell-wrap">
                  <template v-if="hasPayload(r)">
                    <div class="json-cell-row">
                      <pre class="json-cell-preview">{{ jsonPreviewLine(r.payload) }}</pre>
                      <BaseButton
                        variant="secondary"
                        size="sm"
                        class="shrink-0"
                        @click="openJsonModal('载荷', r.payload)"
                      >
                        完整
                      </BaseButton>
                    </div>
                  </template>
                  <span v-else class="text-slate-400">—</span>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="opPage <= 1" @click="opPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ opPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="opPage >= opTotalPages" @click="opPage++">下一页</button>
          </div>
        </template>
      </section>

      <!-- 权限日志 -->
      <section v-show="activeTab === 'permission'" class="content-card">
        <h3 class="section-title">权限日志</h3>
        <div v-if="permLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="permError" :title="permError" @retry="loadPermissionLogs" />
        <BaseEmptyState
          v-else-if="!permissionItems.length"
          title="暂无权限日志"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <table class="simple-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>操作人</th>
                <th>目标用户</th>
                <th>动作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in permissionItems" :key="String(r.id)">
                <td>{{ formatAt(r.created_at) }}</td>
                <td>{{ formatPermActor(r) }}</td>
                <td>{{ formatPermTarget(r) }}</td>
                <td>{{ formatPermAction(r) }}</td>
              </tr>
            </tbody>
          </table>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="permPage <= 1" @click="permPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ permPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="permPage >= permTotalPages" @click="permPage++">下一页</button>
          </div>
        </template>
      </section>

      <!-- 服务器日志（v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节，仅 Admin 可见） -->
      <section v-show="activeTab === 'server' && canViewServerLogs" class="content-card">
        <div class="section-header">
          <h3 class="section-title">服务器日志</h3>
          <BaseButton variant="secondary" size="sm" @click="showCleanModal = true">清理旧日志</BaseButton>
        </div>
        <div class="filter-row">
          <select v-model="serverLevel" class="filter-select">
            <option value="">全部级别</option>
            <option value="info">信息</option>
            <option value="warn">警告</option>
            <option value="error">错误</option>
          </select>
          <input
            v-model="serverKeyword"
            type="text"
            class="filter-input"
            placeholder="关键词"
          />
          <input
            v-model="serverSince"
            type="datetime-local"
            class="filter-input"
            placeholder="起始时间"
          />
          <input
            v-model="serverUntil"
            type="datetime-local"
            class="filter-input"
            placeholder="截止时间"
          />
          <BaseButton variant="primary" size="sm" @click="applyServerFilters">查询</BaseButton>
        </div>
        <div v-if="serverLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="serverError" :title="serverError" @retry="loadServerLogs" />
        <BaseEmptyState
          v-else-if="!serverItems.length"
          title="暂无服务器日志"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <table class="simple-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>级别</th>
                <th>消息</th>
                <th>详情</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in serverItems" :key="r.id">
                <td>{{ formatAt(r.created_at) }}</td>
                <td><span class="level-badge" :class="'level-' + r.level" :title="r.level || undefined">{{ levelLabel(r.level) }}</span></td>
                <td class="msg-cell">{{ r.msg }}</td>
                <td class="json-cell-wrap">
                  <template v-if="hasServerDetails(r)">
                    <div class="json-cell-row">
                      <pre class="json-cell-preview">{{ jsonPreviewLine(normalizeServerDetails(r.details)) }}</pre>
                      <BaseButton
                        variant="secondary"
                        size="sm"
                        class="shrink-0"
                        @click="openJsonModal('详情', normalizeServerDetails(r.details))"
                      >
                        完整
                      </BaseButton>
                    </div>
                  </template>
                  <span v-else class="text-slate-400">—</span>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="serverPage <= 1" @click="serverPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ serverPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="serverPage >= serverTotalPages" @click="serverPage++">下一页</button>
          </div>
        </template>
      </section>

      <!-- 清理服务器日志弹窗（v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节） -->
      <BaseModal
        v-model="showCleanModal"
        title="清理旧服务器日志"
        :show-confirm="false"
        :custom-footer="true"
      >
        <div class="clean-form">
          <label class="clean-label">清理原因 <span class="required">*</span></label>
          <textarea
            v-model="cleanReason"
            class="clean-textarea"
            rows="3"
            placeholder="请输入清理原因（必填）"
          />
          <label class="clean-label">清理早于（小时）</label>
          <input
            v-model.number="cleanOlderThanHours"
            type="number"
            class="clean-input"
            min="1"
            placeholder="24"
          />
        </div>
        <template #footer>
          <BaseButton variant="secondary" size="sm" @click="showCleanModal = false">取消</BaseButton>
          <BaseButton variant="primary" size="sm" :disabled="!cleanReason.trim() || cleanLoading" @click="doCleanLogs">
            {{ cleanLoading ? '清理中…' : '确认清理' }}
          </BaseButton>
        </template>
      </BaseModal>

      <BaseModal
        v-model="showJsonModal"
        :title="jsonModalTitle"
        :show-confirm="false"
        :custom-footer="true"
      >
        <pre class="json-body">{{ jsonModalBody }}</pre>
        <template #footer>
          <BaseButton variant="primary" size="sm" @click="showJsonModal = false">关闭</BaseButton>
        </template>
      </BaseModal>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { logsApi } from '@/services/api/logsApi'
import type { OperationLogEntry, PermissionLog, ServerLog } from '@/services/apiTypes'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import { usePermissionsStore } from '@/stores/permissions'
import { usePermission } from '@/composables/usePermission'
import { beijingDateTimeLocalToISO, formatDateTimeBeijing } from '@/utils/date'
import { getOperationEventTypeLabel } from '@/utils/operation-event-type-labels'

const permissionsStore = usePermissionsStore()
const { can } = usePermission()
// v1.8 Round I：改走 action / menu 键，不再靠角色名兜底。
// `logs.view` 对应操作/权限日志；`logs.server.view` 对应服务器日志（SuperAdmin 专属）。
const canView = computed(
  () => can('logs.view') || permissionsStore.hasMenu('logs_center'),
)
const canViewServerLogs = computed(() => can('logs.server.view'))

const activeTab = ref<'operation' | 'permission' | 'server'>('operation')

const opPage = ref(1)
/** 程序内同步 opPage（如越界钳制）时跳过 watch，避免重复请求 */
const skipNextOpPageWatch = ref(false)
const opPageSize = 20
const opLoading = ref(false)
const opError = ref('')
const opData = ref<{ items: OperationLogEntry[]; total: number }>({ items: [], total: 0 })
const opSource = ref<'' | OperationLogEntry['source']>('')
const opEventType = ref('')

const permPage = ref(1)
const permPageSize = 20
const permLoading = ref(false)
const permError = ref('')
const permData = ref<{ items: PermissionLog[]; total: number }>({ items: [], total: 0 })

const operationItems = computed(() => opData.value.items)
const opTotalPages = computed(() => Math.max(1, Math.ceil(opData.value.total / opPageSize)))

const permissionItems = computed(() => permData.value.items)
const permTotalPages = computed(() => Math.max(1, Math.ceil(permData.value.total / permPageSize)))

// v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节 服务器日志
const serverPage = ref(1)
const serverPageSize = 20
const serverLoading = ref(false)
const serverError = ref('')
const serverData = ref<{ items: ServerLog[]; total: number }>({ items: [], total: 0 })
const serverLevel = ref<'info' | 'warn' | 'error' | ''>('')
const serverKeyword = ref('')
const serverSince = ref('')
const serverUntil = ref('')
const showCleanModal = ref(false)
const cleanReason = ref('')
const cleanOlderThanHours = ref(24)
const cleanLoading = ref(false)

const showJsonModal = ref(false)
const jsonModalTitle = ref('')
const jsonModalBody = ref('')

const serverItems = computed(() => serverData.value.items)
const serverTotalPages = computed(() => Math.max(1, Math.ceil(serverData.value.total / serverPageSize)))

function formatAt(iso: string) {
  return formatDateTimeBeijing(iso)
}

const OP_SOURCE_LABEL: Record<OperationLogEntry['source'], string> = {
  task_event: '任务',
  export_event: '导出',
  integration_call: '集成',
}

function opSourceLabel(s: OperationLogEntry['source']) {
  return OP_SOURCE_LABEL[s] ?? s
}

const ACTOR_TYPE_LABEL: Record<string, string> = {
  user: '用户',
  system: '系统',
  service: '服务',
  admin: '管理员',
  designer: '设计师',
  auditor: '审核人',
  api: '接口',
  job: '定时任务',
  integration: '集成',
}

const REFERENCE_TYPE_LABEL: Record<string, string> = {
  task: '任务',
  asset: '资产',
  customization_job: '定制任务',
  user: '用户',
  role: '角色',
  department: '部门',
  team: '团队',
  export: '导出',
  outsource: '外协单',
  warehouse: '仓库',
}

const LOG_LEVEL_LABEL: Record<string, string> = {
  trace: '追踪',
  debug: '调试',
  info: '信息',
  warn: '警告',
  warning: '警告',
  error: '错误',
  fatal: '致命',
}

const LOG_STATUS_LABEL: Record<string, string> = {
  success: '成功',
  ok: '成功',
  pass: '成功',
  failed: '失败',
  fail: '失败',
  error: '失败',
  pending: '处理中',
  running: '进行中',
  retrying: '重试中',
  skipped: '已跳过',
  ignored: '已忽略',
  timeout: '已超时',
  cancelled: '已取消',
}

function localizeEnum(dict: Record<string, string>, raw: unknown): string {
  if (raw === null || raw === undefined) return '—'
  const key = String(raw).trim()
  if (!key) return '—'
  return dict[key.toLowerCase()] ?? key
}

function actorTypeLabel(t: string | undefined | null): string {
  return localizeEnum(ACTOR_TYPE_LABEL, t)
}

function referenceTypeLabel(t: string | undefined | null): string {
  return localizeEnum(REFERENCE_TYPE_LABEL, t)
}

function levelLabel(t: string | undefined | null): string {
  return localizeEnum(LOG_LEVEL_LABEL, t)
}

function statusLabel(t: string | undefined | null): string {
  return localizeEnum(LOG_STATUS_LABEL, t)
}

function formatOperationActor(r: OperationLogEntry) {
  const t = actorTypeLabel(r.actor_type)
  if (r.actor_id != null && r.actor_id !== undefined) {
    return t !== '—' ? `${t} #${r.actor_id}` : String(r.actor_id)
  }
  return t
}

function hasPayload(r: OperationLogEntry) {
  const p = r.payload
  if (p === null || p === undefined) return false
  if (typeof p === 'object' && !Array.isArray(p)) return Object.keys(p as object).length > 0
  return true
}

function formatJsonPretty(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function jsonPreviewLine(value: unknown): string {
  try {
    let s: string
    if (typeof value === 'string') {
      try {
        s = JSON.stringify(JSON.parse(value))
      } catch {
        s = value
      }
    } else {
      s = JSON.stringify(value)
    }
    return s.length > 120 ? `${s.slice(0, 120)}…` : s
  } catch {
    return String(value)
  }
}

function openJsonModal(title: string, value: unknown) {
  jsonModalTitle.value = title
  jsonModalBody.value = formatJsonPretty(value)
  showJsonModal.value = true
}

function normalizeServerDetails(raw: ServerLog['details']): unknown {
  if (raw === null || raw === undefined) return null
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw)
    } catch {
      return raw
    }
  }
  return raw
}

function hasServerDetails(r: ServerLog): boolean {
  const d = r.details
  if (d === null || d === undefined) return false
  if (typeof d === 'string') return d.trim().length > 0
  if (typeof d === 'object') return Object.keys(d as object).length > 0
  return true
}

function formatPermActor(r: PermissionLog) {
  if (r.actor_username?.trim()) return r.actor_username
  if (r.actor_id != null) return `#${r.actor_id}`
  return '—'
}

function formatPermTarget(r: PermissionLog) {
  if (r.target_username?.trim()) return r.target_username
  if (r.target_user_id != null) return String(r.target_user_id)
  return '—'
}

const PERM_ACTION_LABEL: Record<string, string> = {
  'role.assign': '分配角色',
  'role.revoke': '撤销角色',
  'role.create': '创建角色',
  'role.update': '更新角色',
  'role.delete': '删除角色',
  'user.create': '创建用户',
  'user.update': '更新用户',
  'user.delete': '删除用户',
  'user.disable': '停用用户',
  'user.enable': '启用用户',
  'user.password_reset': '重置密码',
  'permission.grant': '授予权限',
  'permission.revoke': '撤销权限',
  'department.create': '创建部门',
  'department.update': '更新部门',
  'department.delete': '删除部门',
  'team.create': '创建团队',
  'team.update': '更新团队',
  'team.delete': '删除团队',
  login: '登录',
  logout: '退出登录',
}

function formatPermAction(r: PermissionLog) {
  const raw = r.action_type?.trim()
  if (raw) return PERM_ACTION_LABEL[raw.toLowerCase()] ?? raw
  const route = [r.method, r.route_path].filter(Boolean).join(' ')
  return route.trim() || '—'
}

function applyOperationFilters() {
  opPage.value = 1
  loadOperationLogs()
}

function parsePaginationTotal(pagination: { total?: unknown } | undefined): number | null {
  const t = pagination?.total
  if (typeof t === 'number' && Number.isFinite(t) && t >= 0) return Math.floor(t)
  if (typeof t === 'string' && t.trim() !== '') {
    const n = Number(t)
    if (Number.isFinite(n) && n >= 0) return Math.floor(n)
  }
  return null
}

function resolveOperationLogTotal(
  pagination: { total?: unknown } | undefined,
  items: OperationLogEntry[],
  pageRequested: number,
  pageSize: number,
  previousTotal: number,
): number {
  const parsed = parsePaginationTotal(pagination)
  if (parsed !== null) return parsed
  if (items.length > 0) {
    return Math.max(previousTotal, (pageRequested - 1) * pageSize + items.length)
  }
  if (pageRequested > 1) return previousTotal
  return 0
}

async function loadOperationLogs() {
  if (!canView.value) return
  opLoading.value = true
  opError.value = ''
  try {
    const buildParams = (page: number) => {
      const params: {
        page: number
        page_size: number
        source?: OperationLogEntry['source']
        event_type?: string
      } = { page, page_size: opPageSize }
      if (opSource.value) params.source = opSource.value
      const et = opEventType.value.trim()
      if (et) params.event_type = et
      return params
    }

    let pageToFetch = opPage.value
    let res = await logsApi.operationLogs(buildParams(pageToFetch))
    let body = res?.data as
      | { data?: OperationLogEntry[]; pagination?: { total?: unknown } }
      | undefined
    let items = Array.isArray(body?.data) ? body.data : []
    let total = resolveOperationLogTotal(
      body?.pagination,
      items,
      pageToFetch,
      opPageSize,
      opData.value.total,
    )
    let maxPage = Math.max(1, Math.ceil(total / opPageSize) || 1)

    if (pageToFetch > maxPage) {
      pageToFetch = maxPage
      res = await logsApi.operationLogs(buildParams(pageToFetch))
      body = res?.data as typeof body
      items = Array.isArray(body?.data) ? body.data : []
      total = resolveOperationLogTotal(
        body?.pagination,
        items,
        pageToFetch,
        opPageSize,
        total,
      )
      maxPage = Math.max(1, Math.ceil(total / opPageSize) || 1)
    }

    opData.value = { items, total }
    if (opPage.value !== pageToFetch) {
      skipNextOpPageWatch.value = true
      opPage.value = pageToFetch
    }
  } catch (e) {
    opError.value = e instanceof Error ? e.message : '加载操作日志失败'
  } finally {
    opLoading.value = false
  }
}

async function loadPermissionLogs() {
  if (!canView.value) return
  permLoading.value = true
  permError.value = ''
  try {
    const res = await logsApi.permissionLogs({ page: permPage.value, page_size: permPageSize })
    const httpBody = res?.data as
      | { data?: PermissionLog[]; pagination?: { total?: number } }
      | undefined
    const items = Array.isArray(httpBody?.data) ? httpBody.data : []
    const total =
      typeof httpBody?.pagination?.total === 'number' ? httpBody.pagination.total : items.length
    permData.value = { items, total }
  } catch (e) {
    permError.value = e instanceof Error ? e.message : '加载权限日志失败'
  } finally {
    permLoading.value = false
  }
}

async function loadServerLogs() {
  if (!canViewServerLogs.value) return
  serverLoading.value = true
  serverError.value = ''
  try {
    const params: Record<string, unknown> = {
      page: serverPage.value,
      page_size: serverPageSize,
    }
    if (serverLevel.value) params.level = serverLevel.value
    if (serverKeyword.value.trim()) params.keyword = serverKeyword.value.trim()
    if (serverSince.value) {
      const since = beijingDateTimeLocalToISO(serverSince.value)
      if (since) params.since = since
    }
    if (serverUntil.value) {
      const until = beijingDateTimeLocalToISO(serverUntil.value)
      if (until) params.until = until
    }
    const res = await logsApi.serverLogs(params)
    const body = res?.data
    const items = Array.isArray(body) ? body : (body?.data ?? body?.items ?? [])
    const total = typeof body?.pagination?.total === 'number' ? body.pagination.total : items.length
    serverData.value = { items, total }
  } catch (e) {
    serverError.value = e instanceof Error ? e.message : '加载服务器日志失败'
  } finally {
    serverLoading.value = false
  }
}

function applyServerFilters() {
  serverPage.value = 1
  loadServerLogs()
}

async function doCleanLogs() {
  if (!cleanReason.value.trim() || !canViewServerLogs.value) return
  cleanLoading.value = true
  try {
    await logsApi.serverLogsClean({
      reason: cleanReason.value.trim(),
      older_than_hours: cleanOlderThanHours.value || 24,
    })
    showCleanModal.value = false
    cleanReason.value = ''
    cleanOlderThanHours.value = 24
    await loadServerLogs()
  } catch (e) {
    serverError.value = e instanceof Error ? e.message : '清理失败'
  } finally {
    cleanLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'operation' && !opData.value.items.length && !opLoading.value) loadOperationLogs()
  if (tab === 'permission' && !permData.value.items.length && !permLoading.value) loadPermissionLogs()
  if (tab === 'server' && canViewServerLogs.value) loadServerLogs()
})
watch(opPage, () => {
  if (skipNextOpPageWatch.value) {
    skipNextOpPageWatch.value = false
    return
  }
  loadOperationLogs()
})
watch(permPage, loadPermissionLogs)
watch(serverPage, loadServerLogs)

onMounted(() => {
  if (canView.value && activeTab.value === 'operation') loadOperationLogs()
  if (canViewServerLogs.value && activeTab.value === 'server') loadServerLogs()
})
</script>

<style scoped>
.logs-management-view { padding: 0; }
.page-header { margin-bottom: 1rem; }
.page-title { margin: 0; font-size: 1.125rem; font-weight: 600; color: #0f172a; }
.tabs { display: flex; gap: 0.5rem; }
.tab { padding: 0.5rem 1rem; border-radius: 0.5rem; border: 1px solid #e2e8f0; background: #fff; cursor: pointer; font-size: 0.875rem; }
.tab.active { background: #0f172a; color: #fff; border-color: #0f172a; }
.content-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 0.75rem; padding: 1rem; }
.section-title { margin: 0 0 0.75rem; font-size: 0.875rem; font-weight: 600; color: #0f172a; }
.simple-table { width: 100%; border-collapse: collapse; font-size: 0.75rem; }
.simple-table th, .simple-table td { border: 1px solid #e2e8f0; padding: 0.25rem 0.5rem; text-align: left; }
.pager { margin-top: 0.75rem; display: flex; align-items: center; gap: 0.5rem; }
.pager-btn { padding: 0.25rem 0.75rem; font-size: 0.75rem; border-radius: 9999px; border: 1px solid #cbd5e1; background: #fff; }
.pager-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem; }
.filter-row { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 0.75rem; align-items: center; }
.filter-select, .filter-input { padding: 0.25rem 0.5rem; font-size: 0.75rem; border: 1px solid #e2e8f0; border-radius: 0.375rem; }
.filter-input { min-width: 140px; }
.level-badge { padding: 0.125rem 0.375rem; border-radius: 0.25rem; font-size: 0.7rem; font-weight: 500; }
.level-info { background: #dbeafe; color: #1d4ed8; }
.level-warn { background: #fef3c7; color: #b45309; }
.level-error { background: #fee2e2; color: #b91c1c; }
.msg-cell { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.summary-cell { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ref-cell { max-width: 140px; }
.json-cell-wrap { vertical-align: top; min-width: 12rem; max-width: 22rem; }
.json-cell-row { display: flex; align-items: flex-start; gap: 0.5rem; }
.json-cell-preview {
  margin: 0; flex: 1; min-width: 0; max-height: 4.5rem; overflow: hidden;
  font-size: 0.65rem; font-family: ui-monospace, monospace; color: #475569;
  white-space: pre-wrap; word-break: break-all;
}
.json-body {
  margin: 0; padding: 0.75rem; max-height: 70dvh; overflow: auto;
  font-size: 0.75rem; font-family: ui-monospace, monospace; line-height: 1.45;
  white-space: pre-wrap; word-break: break-all; background: #f8fafc; border-radius: 0.5rem;
}
.details-preview { font-size: 0.7rem; color: #64748b; }
.clean-form { display: flex; flex-direction: column; gap: 0.5rem; }
.clean-label { font-size: 0.875rem; font-weight: 500; color: #334155; }
.clean-label .required { color: #dc2626; }
.clean-textarea { width: 100%; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 0.375rem; font-size: 0.875rem; resize: vertical; }
.clean-input { width: 100%; max-width: 120px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 0.375rem; font-size: 0.875rem; }
</style>
