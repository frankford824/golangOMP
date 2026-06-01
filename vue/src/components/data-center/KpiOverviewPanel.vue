<template>
  <section class="kpi-overview">
    <div class="panel-header">
      <div>
        <h3 class="panel-title">绩效概览</h3>
        <p class="panel-subtitle">{{ rangeLabel }} · 按真实业务动作统计</p>
      </div>
      <div class="panel-actions">
        <select v-model.number="rangeDays" class="range-select" @change="load">
          <option :value="7">近 7 天</option>
          <option :value="14">近 14 天</option>
          <option :value="30">近 30 天</option>
        </select>
        <BaseButton variant="secondary" size="sm" :loading="loading" @click="load">刷新</BaseButton>
      </div>
    </div>

    <BaseErrorState v-if="error" :title="error" @retry="load" />
    <template v-else>
      <div v-if="loading" class="metric-grid">
        <BaseSkeleton v-for="i in 8" :key="i" width="100%" height="4.75rem" />
      </div>
      <template v-else>
        <div class="metric-grid">
          <article v-for="metric in summaryMetrics" :key="metric.key" class="metric-card">
            <span>{{ metric.label }}</span>
            <strong>{{ metric.value }}</strong>
            <small>{{ metric.hint }}</small>
          </article>
        </div>

        <div v-if="reportCards.length" class="report-strip">
          <article v-for="card in reportCards" :key="card.key || card.title" class="report-card">
            <span>{{ reportCardTitle(card) }}</span>
            <strong>{{ card.value }}<small v-if="card.unit">{{ card.unit }}</small></strong>
          </article>
        </div>

        <BaseEmptyState
          v-if="!events.length"
          title="暂无绩效数据"
          description="当前时间范围内没有可统计的员工业务动作。"
        />

        <div v-else class="role-grid">
          <section class="role-card">
            <div class="role-header">
              <h4>运营人员</h4>
              <span>任务发起</span>
            </div>
            <div class="table-scroll">
              <table class="kpi-table">
                <thead>
                  <tr>
                    <th>用户姓名</th>
                    <th>任务单量</th>
                    <th>活跃天数</th>
                    <th>平均发起间隔</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in operatorRows" :key="row.key">
                    <td>
                      <strong>{{ row.name }}</strong>
                      <small>{{ orgText(row) }}</small>
                    </td>
                    <td>{{ row.taskCreates }}</td>
                    <td>{{ row.activeDays.size }}</td>
                    <td>{{ durationLabel(row.avgCreateIntervalMs) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="role-card">
            <div class="role-header">
              <h4>设计人员</h4>
              <span>接单与质量</span>
            </div>
            <div class="table-scroll">
              <table class="kpi-table">
                <thead>
                  <tr>
                    <th>用户姓名</th>
                    <th>接单</th>
                    <th>接单完成率</th>
                    <th>平均完成时间</th>
                    <th>打回率</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in designerRows" :key="row.key">
                    <td>
                      <strong>{{ row.name }}</strong>
                      <small>{{ orgText(row) }}</small>
                    </td>
                    <td>{{ row.designClaims }}</td>
                    <td>{{ percentLabel(row.designSubmits, row.designClaims) }}</td>
                    <td>{{ durationLabel(row.avgClaimToSubmitMs) }}</td>
                    <td :class="{ danger: row.designRejects > 0 }">
                      {{ percentLabel(row.designRejects, row.designSubmits) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="role-card">
            <div class="role-header">
              <h4>审核人员</h4>
              <span>审核效率</span>
            </div>
            <div class="table-scroll">
              <table class="kpi-table">
                <thead>
                  <tr>
                    <th>用户姓名</th>
                    <th>审核量</th>
                    <th>通过</th>
                    <th>打回</th>
                    <th>平均审核耗时</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in auditorRows" :key="row.key">
                    <td>
                      <strong>{{ row.name }}</strong>
                      <small>{{ orgText(row) }}</small>
                    </td>
                    <td>{{ row.auditDecisions }}</td>
                    <td>{{ row.auditApproves }}</td>
                    <td :class="{ danger: row.auditRejects > 0 }">{{ row.auditRejects }}</td>
                    <td>{{ durationLabel(row.avgAuditMs) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </template>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import { logsApi } from '@/services/api/logsApi'
import { reportsApi } from '@/services/api/reportsApi'
import { usersApi } from '@/services/api/usersApi'
import type { BackendUser, OperationLogEntry, WorkflowTraceEvent } from '@/services/apiTypes'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import { userAccountDisplay } from '@/domain/user-display'

interface L1Card {
  key?: string
  title?: string
  value?: string | number
  unit?: string
}

interface PaginationEnvelope<T> {
  data?: T[]
  pagination?: { total?: unknown }
}

interface UserDirectoryEntry {
  id: string
  username: string
  name: string
  department: string
  team: string
}

interface PersonStats {
  key: string
  name: string
  department: string
  team: string
  activeDays: Set<string>
  taskCreates: number
  createTimes: number[]
  avgCreateIntervalMs: number
  designClaims: number
  designSubmits: number
  designRejects: number
  claimToSubmitMs: number[]
  avgClaimToSubmitMs: number
  auditDecisions: number
  auditApproves: number
  auditRejects: number
  auditMs: number[]
  avgAuditMs: number
}

const permissionsStore = usePermissionsStore()
const { can } = usePermission()

const rangeDays = ref(7)
const loading = ref(false)
const error = ref('')
const events = ref<WorkflowTraceEvent[]>([])
const reportCards = ref<L1Card[]>([])
const traceTotal = ref(0)
const userDirectory = ref(new Map<string, UserDirectoryEntry>())
const userDirectoryByUsername = computed(() => {
  const next = new Map<string, UserDirectoryEntry>()
  for (const user of userDirectory.value.values()) {
    const username = user.username.trim().toLowerCase()
    if (username) next.set(username, user)
  }
  return next
})

const KPI_TASK_EVENT_TYPES = [
  'task.created',
  'task.batch_items_created',
  'task.assigned',
  'task.reassigned',
  'task.batch_assigned',
  'task.design.submitted',
  'task.audit.approved',
  'task.audit.rejected',
] as const

const REPORT_CARD_LABELS: Record<string, string> = {
  tasks_in_progress: '进行中任务',
  in_progress: '进行中任务',
  tasks_completed_today: '今日完成任务',
  completed_today: '今日完成任务',
  archived_total: '累计归档',
  archived: '累计归档',
  pool_waiting: '待接单任务',
  pool: '待接单任务',
}

const canLoadTrace = computed(() => can('logs.view') || permissionsStore.hasMenu('logs_center'))
const canLoadReports = computed(
  () =>
    permissionsStore.hasMenu('report_center') ||
    permissionsStore.hasMenu('kpi') ||
    permissionsStore.hasMenu('finance'),
)

const rangeStart = computed(() => {
  const d = new Date()
  d.setDate(d.getDate() - rangeDays.value + 1)
  d.setHours(0, 0, 0, 0)
  return d
})
const rangeEnd = computed(() => {
  const d = new Date()
  d.setHours(23, 59, 59, 999)
  return d
})
const rangeLabel = computed(() => `${dateOnly(rangeStart.value)} 至 ${dateOnly(rangeEnd.value)}`)

const people = computed(() => buildStats(events.value))
const operatorRows = computed(() =>
  people.value
    .filter((row) => row.taskCreates > 0 || looksLike(row, '运营'))
    .sort((a, b) => b.taskCreates - a.taskCreates || a.name.localeCompare(b.name, 'zh-Hans-CN'))
    .slice(0, 10),
)
const designerRows = computed(() =>
  people.value
    .filter((row) => row.designClaims > 0 || row.designSubmits > 0 || row.designRejects > 0 || looksLike(row, '设计') || looksLike(row, '美工'))
    .sort(
      (a, b) =>
        b.designSubmits - a.designSubmits ||
        b.designClaims - a.designClaims ||
        a.name.localeCompare(b.name, 'zh-Hans-CN'),
    )
    .slice(0, 10),
)
const auditorRows = computed(() =>
  people.value
    .filter((row) => row.auditDecisions > 0 || looksLike(row, '审核'))
    .sort((a, b) => b.auditDecisions - a.auditDecisions || a.name.localeCompare(b.name, 'zh-Hans-CN'))
    .slice(0, 10),
)

const summaryMetrics = computed(() => {
  const taskCreates = sum(people.value, (row) => row.taskCreates)
  const claims = sum(people.value, (row) => row.designClaims)
  const submits = sum(people.value, (row) => row.designSubmits)
  const designRejects = sum(people.value, (row) => row.designRejects)
  const auditDecisions = sum(people.value, (row) => row.auditDecisions)
  const auditRejects = sum(people.value, (row) => row.auditRejects)
  return [
    { key: 'kpi_events', label: '关键动作', value: events.value.length, hint: '创建 / 指派 / 提交 / 审核' },
    { key: 'task_creates', label: '运营任务单量', value: taskCreates, hint: '任务创建动作' },
    {
      key: 'avg_create_cycle',
      label: '任务发起周期',
      value: durationLabel(avg(people.value.map((row) => row.avgCreateIntervalMs).filter(Boolean))),
      hint: '同一运营连续发起间隔',
    },
    { key: 'design_claims', label: '设计接单', value: claims, hint: '接单动作' },
    { key: 'design_completion', label: '接单完成率', value: percentLabel(submits, claims), hint: '提交设计 / 接单' },
    {
      key: 'design_reject_rate',
      label: '设计打回率',
      value: percentLabel(designRejects, submits),
      hint: '审核打回 / 设计提交',
    },
    { key: 'audit_decisions', label: '审核处理量', value: auditDecisions, hint: '通过与打回' },
    {
      key: 'audit_reject_rate',
      label: '审核打回率',
      value: percentLabel(auditRejects, auditDecisions),
      hint: '打回 / 审核处理',
    },
    { key: 'trace_total', label: '链路事件', value: traceTotal.value, hint: '接口与页面行为参考' },
  ]
})

function dateOnly(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function eventAt(event: WorkflowTraceEvent): number {
  const raw = event.occurred_at || event.created_at
  const ms = raw ? new Date(raw).getTime() : 0
  return Number.isFinite(ms) ? ms : 0
}

function eventDay(event: WorkflowTraceEvent): string {
  const ms = eventAt(event)
  return ms > 0 ? dateOnly(new Date(ms)) : ''
}

function actorKey(event: WorkflowTraceEvent): string {
  if (event.actor_id) return `id:${event.actor_id}`
  return `name:${event.actor_username || 'unknown'}`
}

function actorName(event: WorkflowTraceEvent): string {
  const byId = event.actor_id ? userDirectory.value.get(String(event.actor_id)) : undefined
  if (byId?.name) return byId.name
  const username = String(event.actor_username ?? '').trim().toLowerCase()
  const byUsername = username ? userDirectoryByUsername.value.get(username) : undefined
  if (byUsername?.name) return byUsername.name
  const payloadName = readPayloadText(event.payload, [
    'actor_display_name',
    'actor_name',
    'display_name',
    'name',
    'operator_name',
    'creator_name',
    'designer_name',
    'auditor_name',
    'to_handler_name',
  ])
  return userAccountDisplay(payloadName, event.actor_username, event.actor_id ? `人员#${event.actor_id}` : '')
}

function eventSearchText(event: WorkflowTraceEvent): string {
  const fields = [
    event.event_source,
    event.event_type,
    event.action,
    event.route_method,
    event.route_path,
    event.route_full_path,
    event.page_url,
    event.page_name,
    event.component_id,
    event.module_key,
    event.resource_type,
  ]
  return fields.filter(Boolean).join(' ').toLowerCase()
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function readPayloadNumber(payload: unknown, keys: string[]): number {
  const record = asRecord(payload)
  for (const key of keys) {
    const raw = record[key]
    if (raw === null || raw === undefined || raw === '') continue
    const value = Number(raw)
    if (Number.isFinite(value) && value > 0) return value
  }
  return 0
}

function readPayloadText(payload: unknown, keys: string[]): string {
  const record = asRecord(payload)
  for (const key of keys) {
    const text = String(record[key] ?? '').trim()
    if (text && text !== 'null') return text
  }
  return ''
}

function hasAny(text: string, keywords: string[]): boolean {
  return keywords.some((keyword) => text.includes(keyword.toLowerCase()))
}

function isTaskCreate(event: WorkflowTraceEvent): boolean {
  const text = eventSearchText(event)
  const method = event.route_method?.toUpperCase()
  return (
    hasAny(text, ['create_task', 'task.create', 'task.created', 'task.batch_items_created', '创建任务', '发起任务']) ||
    (method === 'POST' && /\/v1\/tasks\/?$/.test(event.route_path || '')) ||
    (method === 'POST' && /\/tasks\/?$/.test(event.route_path || ''))
  )
}

function isDesignClaim(event: WorkflowTraceEvent): boolean {
  const text = eventSearchText(event)
  if (text.includes('task.audit.')) return false
  return hasAny(text, [
    'task.assigned',
    'task.reassigned',
    'task.batch_assigned',
    'claim',
    '接单',
    'assign_self',
    'design_claim',
    'customization_claim',
  ])
}

function isDesignSubmit(event: WorkflowTraceEvent): boolean {
  const text = eventSearchText(event)
  return hasAny(text, [
    'task.design.submitted',
    'design_submit',
    'submit_design',
    'design-submissions',
    'asset_upload',
    '上传成品',
    '提交设计',
    '提交审核',
  ])
}

function isAuditApprove(event: WorkflowTraceEvent): boolean {
  const text = eventSearchText(event)
  return hasAny(text, ['task.audit.approved', 'audit_approve', 'approve', 'pass', '审核通过', '通过审核'])
}

function isAuditReject(event: WorkflowTraceEvent): boolean {
  const text = eventSearchText(event)
  return hasAny(text, ['task.audit.rejected', 'audit_reject', 'reject', '打回', '驳回'])
}

function looksLike(row: PersonStats, keyword: string): boolean {
  return `${row.department} ${row.team}`.includes(keyword)
}

function emptyStats(event: WorkflowTraceEvent): PersonStats {
  return {
    key: actorKey(event),
    name: actorName(event),
    department: event.actor_department || '',
    team: event.actor_team || '',
    activeDays: new Set<string>(),
    taskCreates: 0,
    createTimes: [],
    avgCreateIntervalMs: 0,
    designClaims: 0,
    designSubmits: 0,
    designRejects: 0,
    claimToSubmitMs: [],
    avgClaimToSubmitMs: 0,
    auditDecisions: 0,
    auditApproves: 0,
    auditRejects: 0,
    auditMs: [],
    avgAuditMs: 0,
  }
}

function buildStats(source: WorkflowTraceEvent[]): PersonStats[] {
  const sorted = [...source].sort((a, b) => eventAt(a) - eventAt(b))
  const byActor = new Map<string, PersonStats>()
  const lastClaimByTask = new Map<string, { actor: string; at: number }>()
  const lastDesignSubmitByTask = new Map<string, { actor: string; at: number }>()

  for (const event of sorted) {
    const key = actorKey(event)
    const stat = byActor.get(key) ?? emptyStats(event)
    byActor.set(key, stat)
    const day = eventDay(event)
    if (day) stat.activeDays.add(day)
    const at = eventAt(event)
    const taskKey = event.task_id ? String(event.task_id) : event.resource_type === 'task' ? String(event.resource_id || '') : ''

    if (isTaskCreate(event)) {
      stat.taskCreates += 1
      if (at > 0) stat.createTimes.push(at)
    }
    if (isDesignClaim(event)) {
      stat.designClaims += 1
      if (taskKey && at > 0) lastClaimByTask.set(taskKey, { actor: key, at })
    }
    if (isDesignSubmit(event)) {
      stat.designSubmits += 1
      if (taskKey && at > 0) {
        const claim = lastClaimByTask.get(taskKey)
        if (claim && claim.actor === key && at > claim.at) stat.claimToSubmitMs.push(at - claim.at)
        lastDesignSubmitByTask.set(taskKey, { actor: key, at })
      }
    }
    if (isAuditApprove(event) || isAuditReject(event)) {
      stat.auditDecisions += 1
      if (isAuditApprove(event)) stat.auditApproves += 1
      if (isAuditReject(event)) {
        stat.auditRejects += 1
        if (taskKey) {
          const submit = lastDesignSubmitByTask.get(taskKey)
          if (submit) {
            const designer = byActor.get(submit.actor)
            if (designer) designer.designRejects += 1
          }
        }
      }
      if (taskKey && at > 0) {
        const submit = lastDesignSubmitByTask.get(taskKey)
        if (submit && at > submit.at) stat.auditMs.push(at - submit.at)
      }
    }
  }

  for (const stat of byActor.values()) {
    stat.avgCreateIntervalMs = avgIntervals(stat.createTimes)
    stat.avgClaimToSubmitMs = avg(stat.claimToSubmitMs)
    stat.avgAuditMs = avg(stat.auditMs)
  }
  return [...byActor.values()]
}

function avgIntervals(times: number[]): number {
  const sorted = [...times].sort((a, b) => a - b)
  const diffs: number[] = []
  for (let i = 1; i < sorted.length; i += 1) {
    const diff = sorted[i] - sorted[i - 1]
    if (diff > 0) diffs.push(diff)
  }
  return avg(diffs)
}

function avg(values: number[]): number {
  const clean = values.filter((value) => Number.isFinite(value) && value > 0)
  if (!clean.length) return 0
  return clean.reduce((sumValue, value) => sumValue + value, 0) / clean.length
}

function sum(rows: PersonStats[], getter: (row: PersonStats) => number): number {
  return rows.reduce((total, row) => total + getter(row), 0)
}

function durationLabel(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  const hours = ms / 3_600_000
  if (hours < 1) return `${Math.max(1, Math.round(hours * 60))} 分钟`
  if (hours < 24) return `${hours.toFixed(1)} 小时`
  return `${(hours / 24).toFixed(1)} 天`
}

function percentLabel(numerator: number, denominator: number): string {
  if (!denominator) return '—'
  return `${((numerator / denominator) * 100).toFixed(1)}%`
}

function orgText(row: PersonStats): string {
  return [row.department, row.team].filter(Boolean).join(' / ') || '未归属'
}

function parseTraceResponse(body: PaginationEnvelope<WorkflowTraceEvent> | WorkflowTraceEvent[] | undefined) {
  if (Array.isArray(body)) return { items: body, total: body.length }
  const items = Array.isArray(body?.data) ? body.data : []
  const totalRaw = body?.pagination?.total
  const total = typeof totalRaw === 'number' ? totalRaw : Number(totalRaw)
  return { items, total: Number.isFinite(total) ? total : items.length }
}

function parseOperationResponse(body: PaginationEnvelope<OperationLogEntry> | OperationLogEntry[] | undefined) {
  if (Array.isArray(body)) return { items: body, total: body.length }
  const items = Array.isArray(body?.data) ? body.data : []
  const totalRaw = body?.pagination?.total
  const total = typeof totalRaw === 'number' ? totalRaw : Number(totalRaw)
  return { items, total: Number.isFinite(total) ? total : items.length }
}

function parseUserList(body: PaginationEnvelope<BackendUser> | BackendUser[] | undefined): BackendUser[] {
  if (Array.isArray(body)) return body
  return Array.isArray(body?.data) ? body.data : []
}

function parseReportCards(body: { data?: L1Card[] } | L1Card[] | undefined): L1Card[] {
  const list = Array.isArray(body) ? body : body?.data
  return Array.isArray(list) ? list : []
}

function reportCardTitle(card: L1Card): string {
  const key = String(card.key ?? '').trim()
  const title = String(card.title ?? '').trim()
  const normalizedTitle = title.toLowerCase().replace(/\s+/g, '_')
  return REPORT_CARD_LABELS[key] || REPORT_CARD_LABELS[title] || REPORT_CARD_LABELS[normalizedTitle] || title || key || '指标'
}

function operationActorId(entry: OperationLogEntry): number | null {
  if (['task.assigned', 'task.reassigned', 'task.batch_assigned'].includes(entry.event_type)) {
    const assignedTo = readPayloadNumber(entry.payload, ['designer_id', 'to_handler_id', 'handler_id', 'assignee_id'])
    if (assignedTo > 0) return assignedTo
  }
  const payloadActor = readPayloadNumber(entry.payload, ['operator_id', 'creator_id', 'designer_id', 'auditor_id'])
  if (payloadActor > 0 && !entry.actor_id) return payloadActor
  return entry.actor_id ?? null
}

function userDirectoryEntry(id: number | null): UserDirectoryEntry | undefined {
  return id ? userDirectory.value.get(String(id)) : undefined
}

function operationToTrace(entry: OperationLogEntry): WorkflowTraceEvent | null {
  const createdAt = entry.created_at
  const at = createdAt ? new Date(createdAt).getTime() : 0
  if (!Number.isFinite(at) || at < rangeStart.value.getTime() || at > rangeEnd.value.getTime()) return null

  const actorId = operationActorId(entry)
  const user = userDirectoryEntry(actorId)
  const fallbackName = readPayloadText(entry.payload, ['designer_name', 'to_handler_name', 'creator_name', 'auditor_name'])
  const actorUsername = user?.name || entry.actor_username || fallbackName || (actorId ? `人员#${actorId}` : '')
  const taskID = Number(entry.reference_id)
  return {
    id: Number(entry.log_id) || at,
    event_id: `operation:${entry.source}:${entry.log_id}`,
    event_source: 'system',
    event_type: 'user_action',
    action: entry.event_type,
    actor_id: actorId,
    actor_username: actorUsername,
    actor_source: entry.actor_type,
    actor_department: user?.department || '',
    actor_team: user?.team || '',
    route_method: '',
    route_path: '',
    resource_type: entry.reference_type,
    resource_id: entry.reference_id,
    task_id: Number.isFinite(taskID) && taskID > 0 ? taskID : null,
    outcome: entry.status === 'failed' ? 'failed' : 'succeeded',
    payload: entry.payload,
    occurred_at: createdAt,
    created_at: createdAt,
  }
}

function dedupeEvents(source: WorkflowTraceEvent[]): WorkflowTraceEvent[] {
  const seen = new Set<string>()
  const out: WorkflowTraceEvent[] = []
  for (const event of source) {
    const key = event.event_id || `${event.action}:${event.task_id}:${event.actor_id}:${event.created_at}`
    if (seen.has(key)) continue
    seen.add(key)
    out.push(event)
  }
  return out.sort((a, b) => eventAt(b) - eventAt(a))
}

async function loadUserDirectory() {
  try {
    const res = await usersApi.list({ page: 1, page_size: 500 })
    const list = parseUserList(res.data as PaginationEnvelope<BackendUser> | BackendUser[])
    const next = new Map<string, UserDirectoryEntry>()
    for (const user of list) {
      const id = String(user.id ?? '').trim()
      if (!id) continue
      next.set(id, {
        id,
        username: String(user.username ?? '').trim(),
        name: userAccountDisplay(user.display_name, (user as { name?: unknown }).name, user.username, `用户#${id}`),
        department: String(user.department ?? '').trim(),
        team: String(user.team ?? '').trim(),
      })
    }
    userDirectory.value = next
  } catch {
    try {
      const res = await usersApi.getDesigners({ workflowLane: 'all' })
      const body = res.data as { data?: unknown } | unknown[]
      const list = Array.isArray(body) ? body : Array.isArray(body?.data) ? body.data : []
      const next = new Map<string, UserDirectoryEntry>()
      for (const raw of list) {
        const record = asRecord(raw)
        const id = String(record.id ?? '').trim()
        if (!id) continue
        next.set(id, {
          id,
          username: String(record.username ?? '').trim(),
          name: userAccountDisplay(record.display_name, record.name, record.username, `人员#${id}`),
          department: '',
          team: '',
        })
      }
      userDirectory.value = next
    } catch {
      userDirectory.value = new Map()
    }
  }
}

async function fetchTaskOperationEvents(): Promise<WorkflowTraceEvent[]> {
  const maxPages = rangeDays.value <= 7 ? 3 : rangeDays.value <= 14 ? 5 : 8
  const collected: WorkflowTraceEvent[] = []

  await Promise.all(
    KPI_TASK_EVENT_TYPES.map(async (eventType) => {
      for (let page = 1; page <= maxPages; page += 1) {
        const res = await logsApi.operationLogs({
          source: 'task_event',
          event_type: eventType,
          page,
          page_size: 100,
        })
        const parsed = parseOperationResponse(res.data as PaginationEnvelope<OperationLogEntry> | OperationLogEntry[])
        collected.push(...(parsed.items.map(operationToTrace).filter(Boolean) as WorkflowTraceEvent[]))

        const oldest = parsed.items.reduce((min, item) => {
          const ms = item.created_at ? new Date(item.created_at).getTime() : 0
          return ms > 0 ? Math.min(min, ms) : min
        }, Number.POSITIVE_INFINITY)
        if (parsed.items.length < 100 || oldest < rangeStart.value.getTime()) break
      }
    }),
  )

  return dedupeEvents(collected)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    if (!canLoadTrace.value && !canLoadReports.value) {
      events.value = []
      reportCards.value = []
      traceTotal.value = 0
      return
    }

    const jobs: Promise<void>[] = []
    if (canLoadTrace.value) {
      jobs.push(
        (async () => {
          await loadUserDirectory()
          const [taskEvents, traceRes] = await Promise.all([
            fetchTaskOperationEvents(),
            logsApi.traceEvents({
              actor_source: 'session_token',
              business_only: true,
              from: rangeStart.value.toISOString(),
              to: rangeEnd.value.toISOString(),
              page: 1,
              page_size: 100,
            }),
          ])
          const parsed = parseTraceResponse(traceRes.data as PaginationEnvelope<WorkflowTraceEvent> | WorkflowTraceEvent[])
          events.value = taskEvents.length ? taskEvents : parsed.items
          traceTotal.value = parsed.total
        })(),
      )
    }
    if (canLoadReports.value) {
      jobs.push(
        reportsApi
          .l1Cards()
          .then((res) => {
            reportCards.value = parseReportCards(res.data as { data?: L1Card[] } | L1Card[])
          })
          .catch(() => {
            reportCards.value = []
          }),
      )
    }

    await Promise.all(jobs)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载绩效概览失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.kpi-overview {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  color: #0f172a;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI",
    "PingFang SC", "Microsoft YaHei", sans-serif;
  letter-spacing: 0;
}
.panel-header,
.role-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.panel-title,
.role-header h4 {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 750;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}
.panel-subtitle,
.role-header span {
  margin: 0.2rem 0 0;
  font-size: 0.75rem;
  line-height: 1.35;
  color: #64748b;
  letter-spacing: 0;
}
.panel-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.range-select {
  height: 2rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.5rem;
  background: #fff;
  padding: 0 0.65rem;
  font-size: 0.75rem;
  color: #334155;
  letter-spacing: 0;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.25rem, 1fr));
  gap: 0.5rem;
}
.metric-card,
.report-card,
.role-card {
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #fff;
}
.metric-card {
  min-height: 4.75rem;
  padding: 0.7rem 0.75rem;
}
.metric-card span,
.report-card span {
  display: block;
  font-size: 0.71875rem;
  line-height: 1.3;
  color: #64748b;
  letter-spacing: 0;
}
.metric-card strong,
.report-card strong {
  display: block;
  margin-top: 0.25rem;
  font-size: 1.35rem;
  line-height: 1.1;
  color: #0f172a;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
}
.metric-card small,
.report-card small {
  display: block;
  margin-top: 0.28rem;
  font-size: 0.6875rem;
  line-height: 1.25;
  color: #94a3b8;
  letter-spacing: 0;
}
.report-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: 0.5rem;
}
.report-card {
  padding: 0.65rem 0.75rem;
  background: #f8fafc;
}
.role-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.65rem;
}
.role-card {
  min-width: 0;
  overflow: hidden;
}
.role-header {
  padding: 0.7rem 0.85rem;
  border-bottom: 1px solid #e2e8f0;
  background: #f8fafc;
}
.table-scroll {
  overflow-x: auto;
}
.kpi-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 42rem;
}
.kpi-table th,
.kpi-table td {
  padding: 0.58rem 0.75rem;
  border-bottom: 1px solid #edf2f7;
  text-align: left;
  font-size: 0.75rem;
  line-height: 1.35;
  color: #334155;
  white-space: nowrap;
  letter-spacing: 0;
}
.kpi-table th {
  background: #fff;
  color: #64748b;
  font-weight: 600;
}
.kpi-table td strong {
  display: block;
  color: #0f172a;
}
.kpi-table td small {
  display: block;
  margin-top: 0.15rem;
  font-size: 0.6875rem;
  color: #94a3b8;
}
.danger {
  color: #dc2626;
  font-weight: 700;
}
@media (min-width: 1180px) {
  .role-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .kpi-table {
    min-width: 34rem;
  }
}
@media (max-width: 720px) {
  .panel-header,
  .panel-actions {
    align-items: stretch;
    flex-direction: column;
  }
  .panel-actions {
    width: 100%;
  }
}
</style>
