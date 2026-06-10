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
        <BaseButton
          variant="secondary"
          size="sm"
          :loading="aiAnalysisLoading"
          :disabled="loading || !events.length"
          @click="openAIAnalysis"
        >
          <Sparkles class="button-icon" aria-hidden="true" />
          AI 分析
        </BaseButton>
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

        <section v-if="managementPredictions.length" class="management-prediction-strip">
          <div class="management-prediction-head">
            <div>
              <span>预测提示</span>
              <strong>本周期管理关注点</strong>
            </div>
            <small>基于任务、资产、ERP 状态实时计算</small>
          </div>
          <div class="management-prediction-grid">
            <article
              v-for="item in managementPredictions"
              :key="item.id"
              class="management-prediction-card"
            >
              <span>{{ item.source || '系统提示' }}</span>
              <strong>{{ item.title }}</strong>
              <p v-if="item.detail">{{ item.detail }}</p>
            </article>
          </div>
        </section>

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
                    <th>最近登录</th>
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
                    <td>{{ shortDateTime(row.lastLoginAt) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="role-card">
            <div class="role-header">
              <h4>设计人员</h4>
              <span>派入 - 转出/终止 = 有效派入</span>
            </div>
            <div class="table-scroll">
              <table class="kpi-table kpi-table--design">
                <thead>
                  <tr>
                    <th>用户姓名</th>
                    <th>派入</th>
                    <th>转出/终止</th>
                    <th>当前在手</th>
                    <th>重点在手</th>
                    <th>本人提交</th>
                    <th>有效完成</th>
                    <th>按时</th>
                    <th>平均完成</th>
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
                    <td>{{ designExcludedClaims(row) }}</td>
                    <td>{{ row.designInHandClaims }}</td>
                    <td>{{ row.priorityInHandClaims }}</td>
                    <td>{{ row.designCompletedClaims }}</td>
                    <td>{{ percentLabel(row.designCompletedClaims, designEffectiveClaims(row)) }}</td>
                    <td>{{ percentLabel(row.designOnTimeCompletions, row.designDeadlineCompletions) }}</td>
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

    <BaseModal
      v-model="aiAnalysisOpen"
      title="绩效 AI 分析"
      :show-confirm="false"
      panel-class="max-w-6xl"
    >
      <section class="kpi-ai-modal">
        <div v-if="aiAnalysisLoading" class="kpi-ai-loading" role="status">
          <div class="kpi-ai-loading-dot" aria-hidden="true" />
          <div>
            <p class="kpi-ai-loading-title">正在生成分析</p>
            <p class="kpi-ai-loading-sub">系统正在读取本周期任务、人员、设计提交、审核与资产链路。</p>
          </div>
        </div>

        <div v-else-if="aiAnalysisError" class="kpi-ai-error">
          <p>{{ aiAnalysisError }}</p>
          <BaseButton size="sm" variant="primary" @click="loadAIAnalysis">重新生成</BaseButton>
        </div>

        <div v-else-if="aiAnalysis" class="kpi-ai-content">
          <header class="kpi-ai-hero">
            <span>AI 管理结论</span>
            <h3>{{ aiAnalysis.headline }}</h3>
            <p>{{ aiAnalysis.overview }}</p>
          </header>

          <div v-if="aiHighlights.length" class="kpi-ai-grid kpi-ai-grid--metrics">
            <article v-for="item in aiHighlights" :key="`${item.title}-${item.value || ''}`" class="kpi-ai-panel">
              <span>{{ item.title }}</span>
              <strong>{{ item.value || '—' }}</strong>
              <p>{{ item.note }}</p>
            </article>
          </div>

          <div class="kpi-ai-grid kpi-ai-grid--main">
            <article class="kpi-ai-panel">
              <h4>人员洞察</h4>
              <ul v-if="aiPeopleInsights.length" class="kpi-ai-list">
                <li v-for="item in aiPeopleInsights" :key="`${item.role || ''}-${item.name}-${item.metric || item.signal}`">
                  <strong>{{ item.name }}</strong>
                  <span>{{ [item.role, item.metric].filter(Boolean).join(' · ') }}</span>
                  <p>{{ item.signal }}</p>
                  <small v-if="item.action">{{ item.action }}</small>
                </li>
              </ul>
              <p v-else class="kpi-ai-muted">本周期暂无可展示的人员洞察。</p>
            </article>

            <article class="kpi-ai-panel">
              <h4>风险与动作</h4>
              <ul v-if="aiRisks.length" class="kpi-ai-list">
                <li v-for="risk in aiRisks" :key="`${risk.level || ''}-${risk.title}`">
                  <strong>{{ risk.title }}</strong>
                  <span>{{ riskLevelLabel(risk.level) }}</span>
                  <p>{{ risk.reason }}</p>
                </li>
              </ul>
              <div v-if="aiActions.length" class="kpi-ai-actions">
                <div v-for="action in aiActions" :key="`${action.owner || ''}-${action.action}`">
                  <span>{{ action.timing || '下一步' }}</span>
                  <strong>{{ action.owner || '相关负责人' }}</strong>
                  <p>{{ action.action }}</p>
                </div>
              </div>
            </article>
          </div>

          <article class="kpi-ai-panel">
            <h4>典型任务链路</h4>
            <div v-if="aiTaskSamples.length" class="kpi-ai-task-list">
              <section v-for="sample in aiTaskSamples" :key="sample.task_no" class="kpi-ai-task">
                <div>
                  <strong>{{ sample.task_no }}</strong>
                  <span>{{ [sample.task_type, sample.task_name].filter(Boolean).join(' · ') }}</span>
                </div>
                <ol>
                  <li v-for="line in sample.timeline || []" :key="line">{{ line }}</li>
                </ol>
                <p v-if="sample.observation">{{ sample.observation }}</p>
              </section>
            </div>
            <p v-else class="kpi-ai-muted">本周期暂无可展示的典型任务链路。</p>
          </article>

          <details class="kpi-ai-evidence">
            <summary>查看证据</summary>
            <ul v-if="aiEvidence.length">
              <li v-for="line in aiEvidence" :key="line">{{ line }}</li>
            </ul>
            <p v-else>系统暂无可展示证据。</p>
          </details>
        </div>
      </section>
      <template #footer>
        <footer class="kpi-ai-footer">
          <span v-if="aiAnalysis">{{ aiAnalysis.model || 'AI' }} · {{ confidenceLabel(aiAnalysis.confidence) }}</span>
          <div class="kpi-ai-footer-actions">
            <BaseButton size="sm" variant="secondary" :disabled="aiAnalysisLoading" @click="aiAnalysisOpen = false">关闭</BaseButton>
            <BaseButton size="sm" variant="primary" :loading="aiAnalysisLoading" :disabled="aiAnalysisLoading" @click="loadAIAnalysis">重新生成</BaseButton>
          </div>
        </footer>
      </template>
    </BaseModal>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import { logsApi } from '@/services/api/logsApi'
import { predictionsApi, type PredictionSuggestion } from '@/services/api/predictionsApi'
import { reportsApi } from '@/services/api/reportsApi'
import type { KpiAiAnalysisResponse, KpiTaskEvent } from '@/services/api/reportsApi'
import { usersApi } from '@/services/api/usersApi'
import type { BackendUser, OperationLogEntry, WorkflowTraceEvent } from '@/services/apiTypes'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import { userAccountDisplay } from '@/domain/user-display'
import {
  buildKpiOperationTraceEvent,
  summarizeKpiDesignLifecycle,
  shouldContinueUserDirectoryLoad,
  type KpiDesignLifecycleEvent,
  type KpiUserDirectoryEntry,
} from '@/domain/data-center-kpi'
import { Sparkles } from 'lucide-vue-next'

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

interface PersonStats {
  key: string
  name: string
  department: string
  team: string
  lastLoginAt?: string
  activeDays: Set<string>
  taskCreates: number
  createTimes: number[]
  avgCreateIntervalMs: number
  designClaims: number
  designSubmits: number
  designCompletedClaims: number
  designTransferredOut: number
  designClosedWithoutSubmit: number
  designInHandClaims: number
  designDeadlineCompletions: number
  designOnTimeCompletions: number
  priorityClaims: number
  priorityInHandClaims: number
  priorityScore: number
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
const aiAnalysisOpen = ref(false)
const aiAnalysisLoading = ref(false)
const aiAnalysisError = ref('')
const aiAnalysis = ref<KpiAiAnalysisResponse | null>(null)
const managementPredictions = ref<PredictionSuggestion[]>([])
const userDirectory = ref(new Map<string, KpiUserDirectoryEntry>())
const userDirectoryByUsername = computed(() => {
  const next = new Map<string, KpiUserDirectoryEntry>()
  for (const user of userDirectory.value.values()) {
    const username = user.username.trim().toLowerCase()
    if (username) next.set(username, user)
  }
  return next
})
const userDirectoryByDisplayName = computed(() => {
  const next = new Map<string, KpiUserDirectoryEntry>()
  for (const user of userDirectory.value.values()) {
    const key = normalizedPersonName(user.realName || user.name)
    if (key && !next.has(key)) next.set(key, user)
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
  'task.customization.reviewed',
  'task.warehouse.received',
  'task.warehouse.completed',
  'task.warehouse.rejected',
  'task.closed',
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
    .filter((row) => hasDesignActivity(row))
    .sort(
      (a, b) =>
        designRiskScore(b) - designRiskScore(a) ||
        b.priorityScore - a.priorityScore ||
        b.designClaims - a.designClaims ||
        designCompletionRate(b) - designCompletionRate(a) ||
        a.designRejects - b.designRejects ||
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
  const excludedClaims = sum(people.value, (row) => designExcludedClaims(row))
  const inHandClaims = sum(people.value, (row) => row.designInHandClaims)
  const effectiveClaims = sum(people.value, (row) => designEffectiveClaims(row))
  const completedClaims = sum(people.value, (row) => row.designCompletedClaims)
  const deadlineCompletions = sum(people.value, (row) => row.designDeadlineCompletions)
  const onTimeCompletions = sum(people.value, (row) => row.designOnTimeCompletions)
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
    { key: 'design_claims', label: '设计派入任务', value: claims, hint: '运营或主管指派给设计' },
    { key: 'design_excluded', label: '转出/终止任务', value: excludedClaims, hint: '后续改派或已离开设计阶段' },
    { key: 'design_in_hand', label: '设计当前在手', value: inHandClaims, hint: '仍需当前设计处理' },
    {
      key: 'design_completion',
      label: '设计有效完成率',
      value: percentLabel(completedClaims, effectiveClaims),
      hint: '本人提交 / 有效派入',
    },
    {
      key: 'design_on_time',
      label: '按时完成率',
      value: percentLabel(onTimeCompletions, deadlineCompletions),
      hint: '提交时间不晚于任务截止',
    },
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

const aiHighlights = computed(() => aiAnalysis.value?.highlights ?? [])
const aiPeopleInsights = computed(() => aiAnalysis.value?.people_insights ?? [])
const aiTaskSamples = computed(() => aiAnalysis.value?.task_samples ?? [])
const aiRisks = computed(() => aiAnalysis.value?.risks ?? [])
const aiActions = computed(() => aiAnalysis.value?.actions ?? [])
const aiEvidence = computed(() => aiAnalysis.value?.evidence ?? [])

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

function shortDateTime(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw) return '从未登录'
  const ms = new Date(raw).getTime()
  if (!Number.isFinite(ms) || ms <= 0) return '从未登录'
  const d = new Date(ms)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const currentYear = new Date().getFullYear()
  return y === currentYear ? `${m}/${day} ${hh}:${mm}` : `${y}/${m}/${day} ${hh}:${mm}`
}

function eventPriority(event: WorkflowTraceEvent): string {
  const value = readPayloadText(event.payload, ['task_priority', 'priority'])
  const normalized = value.toLowerCase()
  if (['critical', 'urgent', 'high', 'normal', 'medium', 'low'].includes(normalized)) return normalized
  return 'normal'
}

function priorityWeight(priority: string): number {
  switch (priority) {
    case 'critical':
    case 'urgent':
      return 4
    case 'high':
      return 3
    case 'normal':
    case 'medium':
      return 2
    case 'low':
      return 1
    default:
      return 2
  }
}

function eventDeadlineMs(event: WorkflowTraceEvent): number {
  const value = readPayloadText(event.payload, ['deadline_at', 'due_at'])
  if (!value) return 0
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) && ms > 0 ? ms : 0
}

function inactiveDesignAssignment(event: WorkflowTraceEvent): boolean {
  const status = readPayloadText(event.payload, ['task_status', 'status']).toLowerCase()
  if (!status) return false
  if (
    status.includes('audit') ||
    status.includes('warehouse') ||
    status.includes('completed') ||
    status.includes('cancelled') ||
    status.includes('canceled') ||
    status.includes('closed') ||
    status.includes('archived') ||
    status.includes('审核') ||
    status.includes('仓库') ||
    status.includes('完成') ||
    status.includes('取消') ||
    status.includes('关闭') ||
    status.includes('终止') ||
    status.includes('归档')
  ) {
    return true
  }
  return false
}

function normalizedPersonName(value: unknown): string {
  const text = String(value ?? '').trim().replace(/\s+/g, ' ')
  if (!text) return ''
  if (/^未知用户$/i.test(text)) return ''
  if (/^(用户|人员)\s*#?\d+$/i.test(text)) return ''
  if (/^#?\d+$/.test(text)) return ''
  if (/^session_actor\s*#?\d+$/i.test(text)) return ''
  return text.toLowerCase()
}

function userDirectoryDisplayName(user: KpiUserDirectoryEntry): string {
  return userAccountDisplay(user.realName, user.name, user.username, `用户#${user.id}`)
}

function payloadPersonName(event: WorkflowTraceEvent): string {
  return readPayloadText(event.payload, [
    'actor_real_name',
    'real_name',
    'employee_name',
    'staff_name',
    'operator_real_name',
    'creator_real_name',
    'designer_real_name',
    'auditor_real_name',
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
}

function resolveEventUser(event: WorkflowTraceEvent): KpiUserDirectoryEntry | undefined {
  const byId = event.actor_id ? userDirectory.value.get(String(event.actor_id)) : undefined
  if (byId) return byId
  const username = String(event.actor_username ?? '').trim().toLowerCase()
  const byUsername = username ? userDirectoryByUsername.value.get(username) : undefined
  if (byUsername) return byUsername
  const nameKey = normalizedPersonName(payloadPersonName(event) || event.actor_username)
  return nameKey ? userDirectoryByDisplayName.value.get(nameKey) : undefined
}

function actorKey(event: WorkflowTraceEvent): string {
  const user = resolveEventUser(event)
  const stableName = normalizedPersonName(user ? userDirectoryDisplayName(user) : actorName(event))
  if (stableName) return `person:${stableName}`
  const username = String(event.actor_username ?? '').trim().toLowerCase()
  if (username) return `username:${username}`
  if (event.actor_id) return `id:${event.actor_id}`
  return 'unknown'
}

function actorName(event: WorkflowTraceEvent): string {
  const user = resolveEventUser(event)
  if (user) return userDirectoryDisplayName(user)
  const payloadName = payloadPersonName(event)
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

function hasDesignActivity(row: PersonStats): boolean {
  return (
    row.designClaims > 0 ||
    row.designSubmits > 0 ||
    row.designRejects > 0 ||
    row.designTransferredOut > 0 ||
    row.designClosedWithoutSubmit > 0 ||
    row.designInHandClaims > 0
  )
}

function designExcludedClaims(row: PersonStats): number {
  return row.designTransferredOut + row.designClosedWithoutSubmit
}

function designEffectiveClaims(row: PersonStats): number {
  return Math.max(0, row.designClaims - designExcludedClaims(row))
}

function designCompletionRate(row: PersonStats): number {
  const effectiveClaims = designEffectiveClaims(row)
  return effectiveClaims > 0 ? row.designCompletedClaims / effectiveClaims : 0
}

function designRiskScore(row: PersonStats): number {
  const completionGap = designEffectiveClaims(row) - row.designCompletedClaims
  const lateGap = row.designDeadlineCompletions - row.designOnTimeCompletions
  return (
    row.priorityInHandClaims * 12 +
    row.designInHandClaims * 4 +
    Math.max(0, completionGap) * 2 +
    Math.max(0, lateGap) * 2 +
    row.designRejects
  )
}

function emptyStats(event: WorkflowTraceEvent): PersonStats {
  const user = resolveEventUser(event)
  return {
    key: actorKey(event),
    name: actorName(event),
    department: event.actor_department || user?.department || '',
    team: event.actor_team || user?.team || '',
    lastLoginAt: user?.lastLoginAt,
    activeDays: new Set<string>(),
    taskCreates: 0,
    createTimes: [],
    avgCreateIntervalMs: 0,
    designClaims: 0,
    designSubmits: 0,
    designCompletedClaims: 0,
    designTransferredOut: 0,
    designClosedWithoutSubmit: 0,
    designInHandClaims: 0,
    designDeadlineCompletions: 0,
    designOnTimeCompletions: 0,
    priorityClaims: 0,
    priorityInHandClaims: 0,
    priorityScore: 0,
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

function mergePersonMeta(stat: PersonStats, event: WorkflowTraceEvent) {
  const user = resolveEventUser(event)
  const nextName = actorName(event)
  if (normalizedPersonName(nextName) && stat.name !== nextName) {
    stat.name = nextName
  }
  const department = String(event.actor_department || user?.department || '').trim()
  const team = String(event.actor_team || user?.team || '').trim()
  if (!stat.department && department) stat.department = department
  if (!stat.team && team) stat.team = team
  if (!stat.lastLoginAt && user?.lastLoginAt) stat.lastLoginAt = user.lastLoginAt
}

function buildStats(source: WorkflowTraceEvent[]): PersonStats[] {
  const sorted = [...source].sort((a, b) => eventAt(a) - eventAt(b))
  const byActor = new Map<string, PersonStats>()
  const designLifecycleEvents: KpiDesignLifecycleEvent[] = []
  const designAssignmentActors = new Set<string>()
  const lastDesignSubmitByTask = new Map<string, { actor: string; at: number }>()

  for (const event of sorted) {
    const key = actorKey(event)
    const stat = byActor.get(key) ?? emptyStats(event)
    byActor.set(key, stat)
    mergePersonMeta(stat, event)
    const day = eventDay(event)
    if (day) stat.activeDays.add(day)
    const at = eventAt(event)
    const taskKey = event.task_id ? String(event.task_id) : event.resource_type === 'task' ? String(event.resource_id || '') : ''

    if (isTaskCreate(event)) {
      stat.taskCreates += 1
      if (at > 0) stat.createTimes.push(at)
    }
    if (isDesignClaim(event)) {
      const priority = eventPriority(event)
      designAssignmentActors.add(key)
      if (taskKey && at > 0) {
        designLifecycleEvents.push({
          taskKey,
          actorKey: key,
          at,
          deadlineMs: eventDeadlineMs(event),
          priorityWeight: priorityWeight(priority),
          inactiveWithoutSubmit: inactiveDesignAssignment(event),
          kind: 'assignment',
        })
      }
    }
    if (isDesignSubmit(event)) {
      if (taskKey && at > 0) {
        designLifecycleEvents.push({ taskKey, actorKey: key, at, kind: 'submission' })
        if (looksLike(stat, '设计') || designAssignmentActors.has(key)) {
          stat.designSubmits += 1
          lastDesignSubmitByTask.set(taskKey, { actor: key, at })
        }
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

  const lifecycleStats = summarizeKpiDesignLifecycle(designLifecycleEvents)
  for (const [key, lifecycle] of lifecycleStats.entries()) {
    const stat = byActor.get(key)
    if (!stat) continue
    stat.designClaims = lifecycle.designClaims
    stat.designCompletedClaims = lifecycle.designCompletedClaims
    stat.designTransferredOut = lifecycle.designTransferredOut
    stat.designClosedWithoutSubmit = lifecycle.designClosedWithoutSubmit
    stat.designInHandClaims = lifecycle.designInHandClaims
    stat.designDeadlineCompletions = lifecycle.designDeadlineCompletions
    stat.designOnTimeCompletions = lifecycle.designOnTimeCompletions
    stat.priorityClaims = lifecycle.priorityClaims
    stat.priorityInHandClaims = lifecycle.priorityInHandClaims
    stat.priorityScore = lifecycle.priorityScore
    stat.claimToSubmitMs = lifecycle.claimToSubmitMs
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

function parseKpiEventResponse(body: { data?: KpiTaskEvent[] } | KpiTaskEvent[] | undefined): KpiTaskEvent[] {
  if (Array.isArray(body)) return body
  return Array.isArray(body?.data) ? body.data : []
}

function parseUserList(body: PaginationEnvelope<BackendUser> | BackendUser[] | undefined): BackendUser[] {
  if (Array.isArray(body)) return body
  return Array.isArray(body?.data) ? body.data : []
}

function parseReportCards(body: { data?: L1Card[] } | L1Card[] | undefined): L1Card[] {
  const list = Array.isArray(body) ? body : body?.data
  return Array.isArray(list) ? list : []
}

function parseKpiAiAnalysis(body: { data?: KpiAiAnalysisResponse } | KpiAiAnalysisResponse | undefined): KpiAiAnalysisResponse | null {
  if (!body) return null
  const nested = (body as { data?: KpiAiAnalysisResponse }).data
  return nested ?? (body as KpiAiAnalysisResponse)
}

function reportCardTitle(card: L1Card): string {
  const key = String(card.key ?? '').trim()
  const title = String(card.title ?? '').trim()
  const normalizedTitle = title.toLowerCase().replace(/\s+/g, '_')
  return REPORT_CARD_LABELS[key] || REPORT_CARD_LABELS[title] || REPORT_CARD_LABELS[normalizedTitle] || title || key || '指标'
}

function confidenceLabel(value: unknown): string {
  const text = String(value ?? '').toLowerCase()
  if (text === 'high') return '可信度高'
  if (text === 'medium') return '可信度中'
  if (text === 'low') return '可信度低'
  return '可信度待判断'
}

function riskLevelLabel(value: unknown): string {
  const text = String(value ?? '').toLowerCase()
  if (text === 'high') return '高风险'
  if (text === 'medium') return '中风险'
  if (text === 'low') return '低风险'
  return '待观察'
}

function operationToTrace(entry: OperationLogEntry): WorkflowTraceEvent | null {
  return buildKpiOperationTraceEvent(entry, {
    rangeStartMs: rangeStart.value.getTime(),
    rangeEndMs: rangeEnd.value.getTime(),
    resolveUserById: (id) => (id ? userDirectory.value.get(String(id)) : undefined),
  })
}

function kpiReportEventToTrace(entry: KpiTaskEvent): WorkflowTraceEvent | null {
  const payload = mergeKpiEventPayload(entry)
  const operation: OperationLogEntry = {
    source: 'task_event',
    log_id: String(entry.id ?? ''),
    reference_type: 'task',
    reference_id: String(entry.task_id ?? ''),
    event_type: String(entry.event_type ?? ''),
    summary: 'Task workflow event',
    actor_id: entry.operator_id ?? null,
    actor_username: String(entry.operator_name ?? '').trim(),
    actor_type: 'session_actor',
    payload,
    created_at: String(entry.created_at ?? ''),
  }
  const trace = operationToTrace(operation)
  if (!trace) return null
  if (trace.actor_id === (entry.operator_id ?? null)) {
    if (!trace.actor_department && entry.operator_department) trace.actor_department = entry.operator_department
    if (!trace.actor_team && entry.operator_team) trace.actor_team = entry.operator_team
  }
  trace.sku_code = entry.sku_code || trace.sku_code
  return trace
}

function mergeKpiEventPayload(entry: KpiTaskEvent): Record<string, unknown> {
  const payload = { ...asRecord(entry.payload) }
  if (entry.priority && payload.priority === undefined) payload.priority = entry.priority
  if (entry.priority && payload.task_priority === undefined) payload.task_priority = entry.priority
  if (entry.deadline_at && payload.deadline_at === undefined) payload.deadline_at = entry.deadline_at
  if (entry.task_status && payload.task_status === undefined) payload.task_status = entry.task_status
  if (entry.task_no && payload.task_no === undefined) payload.task_no = entry.task_no
  if (entry.product_name && payload.product_name === undefined) payload.product_name = entry.product_name
  if (entry.task_type && payload.task_type === undefined) payload.task_type = entry.task_type
  return payload
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
    const next = new Map<string, KpiUserDirectoryEntry>()
    const pageSize = 100
    for (let page = 1; page <= 20; page += 1) {
      const res = await usersApi.list({ page, page_size: pageSize })
      const body = res.data as PaginationEnvelope<BackendUser> | BackendUser[]
      const list = parseUserList(body)
      for (const user of list) {
        const id = String(user.id ?? '').trim()
        if (!id) continue
        next.set(id, {
          id,
          username: String(user.username ?? '').trim(),
          realName: String(user.real_name ?? user.display_name ?? '').trim(),
          name: userAccountDisplay(user.real_name, user.name, user.display_name, user.username, `用户#${id}`),
          department: String(user.department ?? '').trim(),
          team: String(user.team ?? '').trim(),
          lastLoginAt: String(user.last_login_at ?? '').trim(),
        })
      }
      const totalRaw = Array.isArray(body) ? list.length : body?.pagination?.total
      const total = typeof totalRaw === 'number' ? totalRaw : Number(totalRaw)
      if (!shouldContinueUserDirectoryLoad({
        receivedCount: list.length,
        requestedPageSize: pageSize,
        totalLoaded: next.size,
        total,
      })) break
    }
    userDirectory.value = next
  } catch {
    try {
      const res = await usersApi.getDesigners({ workflowLane: 'all' })
      const body = res.data as { data?: unknown } | unknown[]
      const list = Array.isArray(body) ? body : Array.isArray(body?.data) ? body.data : []
      const next = new Map<string, KpiUserDirectoryEntry>()
      for (const raw of list) {
        const record = asRecord(raw)
        const id = String(record.id ?? '').trim()
        if (!id) continue
        next.set(id, {
          id,
          username: String(record.username ?? '').trim(),
          realName: String(record.real_name ?? '').trim(),
          name: userAccountDisplay(record.real_name, record.name, record.display_name, record.username, `人员#${id}`),
          department: '',
          team: '',
          lastLoginAt: String(record.last_login_at ?? '').trim(),
        })
      }
      userDirectory.value = next
    } catch {
      userDirectory.value = new Map()
    }
  }
}

async function openAIAnalysis() {
  aiAnalysisOpen.value = true
  if (!aiAnalysis.value && !aiAnalysisLoading.value) {
    await loadAIAnalysis()
  }
}

async function loadAIAnalysis() {
  aiAnalysisLoading.value = true
  aiAnalysisError.value = ''
  try {
    const res = await reportsApi.kpiAiAnalysis({
      from: dateOnly(rangeStart.value),
      to: dateOnly(rangeEnd.value),
    })
    const parsed = parseKpiAiAnalysis(res.data as { data?: KpiAiAnalysisResponse } | KpiAiAnalysisResponse)
    if (!parsed) {
      throw new Error('AI 分析暂未返回内容')
    }
    aiAnalysis.value = parsed
  } catch (e) {
    aiAnalysisError.value = e instanceof Error ? e.message : 'AI 分析生成失败，请稍后重试'
  } finally {
    aiAnalysisLoading.value = false
  }
}

async function fetchTaskOperationEvents(): Promise<WorkflowTraceEvent[]> {
  try {
    const res = await reportsApi.l1KpiEvents({
      from: dateOnly(rangeStart.value),
      to: dateOnly(rangeEnd.value),
      limit: 5000,
    })
    const parsed = parseKpiEventResponse(res.data as { data?: KpiTaskEvent[] } | KpiTaskEvent[])
    const reportEvents = parsed.map(kpiReportEventToTrace).filter(Boolean) as WorkflowTraceEvent[]
    if (reportEvents.length) return dedupeEvents(reportEvents)
  } catch {
    // Older backends do not expose the enriched report event feed; keep the existing log fallback.
  }

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
  aiAnalysis.value = null
  aiAnalysisError.value = ''
  managementPredictions.value = []
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
          // 人员绩效只按任务工作流事件统计；trace 仅用于链路覆盖量参考。
          events.value = taskEvents
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
      jobs.push(loadManagementPredictions())
    }

    await Promise.all(jobs)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载绩效概览失败'
  } finally {
    loading.value = false
  }
}

async function loadManagementPredictions(): Promise<void> {
  try {
    const bundle = await predictionsApi.management({
      from: dateOnly(rangeStart.value),
      to: dateOnly(rangeEnd.value),
      limit: 4,
    })
    managementPredictions.value = bundle.suggestions
  } catch {
    managementPredictions.value = []
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

.management-prediction-strip {
  display: grid;
  gap: 0.75rem;
  padding: 0.875rem;
  border: 1px solid #bfdbfe;
  border-radius: 0.75rem;
  background:
    linear-gradient(120deg, rgba(37, 99, 235, 0.08), rgba(14, 165, 233, 0.08), rgba(37, 99, 235, 0.08)),
    #f8fbff;
  background-size: 220% 100%;
  animation: kpi-stream-panel 8s linear infinite;
}

.management-prediction-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.management-prediction-head div {
  display: grid;
  gap: 0.125rem;
}

.management-prediction-head span {
  color: #2563eb;
  font-size: 0.72rem;
  font-weight: 800;
}

.management-prediction-head strong {
  color: #0f172a;
  font-size: 0.95rem;
}

.management-prediction-head small {
  color: #64748b;
  font-size: 0.72rem;
}

.management-prediction-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.625rem;
}

.management-prediction-card {
  position: relative;
  display: grid;
  gap: 0.25rem;
  min-height: 6rem;
  padding: 0.75rem;
  overflow: hidden;
  border: 1px solid #dbeafe;
  border-radius: 0.625rem;
  background: #ffffff;
  animation: kpi-card-enter 420ms ease both;
}

.management-prediction-card::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(110deg, transparent 0%, rgba(59, 130, 246, 0.12) 42%, transparent 72%);
  transform: translateX(-120%);
  transition: transform 650ms ease;
}

.management-prediction-card:hover::after {
  transform: translateX(120%);
}

.management-prediction-card span {
  color: #2563eb;
  font-size: 0.7rem;
  font-weight: 800;
}

.management-prediction-card strong {
  color: #111827;
  font-size: 0.875rem;
  line-height: 1.3;
}

.management-prediction-card p {
  margin: 0;
  color: #475569;
  font-size: 0.75rem;
  line-height: 1.4;
}

@keyframes kpi-stream-panel {
  from { background-position: 0% 50%; }
  to { background-position: 220% 50%; }
}

@keyframes kpi-card-enter {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .management-prediction-strip,
  .management-prediction-card {
    animation: none !important;
  }

  .management-prediction-card,
  .management-prediction-card::after {
    transition: none !important;
  }
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
.kpi-table--design {
  min-width: 56rem;
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
.kpi-table th:not(:first-child),
.kpi-table td:not(:first-child) {
  text-align: right;
}
.kpi-table th:first-child,
.kpi-table td:first-child {
  min-width: 9rem;
  max-width: 14rem;
}
.kpi-table td strong {
  display: block;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
}
.kpi-table td small {
  display: block;
  margin-top: 0.15rem;
  font-size: 0.6875rem;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
}
.danger {
  color: #dc2626;
  font-weight: 700;
}
.button-icon {
  width: 0.875rem;
  height: 0.875rem;
  margin-right: 0.35rem;
}
.kpi-ai-modal {
  color: #0f172a;
  letter-spacing: 0;
}
.kpi-ai-loading,
.kpi-ai-error {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  min-height: 9rem;
  padding: 1rem;
  border: 1px solid #dbeafe;
  border-radius: 0.5rem;
  background: #eff6ff;
}
.kpi-ai-error {
  justify-content: space-between;
  border-color: #fecaca;
  background: #fef2f2;
  color: #991b1b;
}
.kpi-ai-loading-dot {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
  background: #2563eb;
  box-shadow: 0 0 0 0 rgba(37, 99, 235, 0.32);
  animation: kpiPulse 1.25s ease-in-out infinite;
}
.kpi-ai-loading-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 750;
  color: #0f172a;
}
.kpi-ai-loading-sub {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: #64748b;
}
.kpi-ai-content {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.kpi-ai-hero {
  padding: 0.85rem 0.95rem;
  border: 1px solid #bfdbfe;
  border-radius: 0.5rem;
  background: #eff6ff;
}
.kpi-ai-hero span {
  font-size: 0.7rem;
  font-weight: 700;
  color: #2563eb;
}
.kpi-ai-hero h3 {
  margin: 0.25rem 0 0;
  font-size: 1.05rem;
  line-height: 1.35;
  font-weight: 800;
}
.kpi-ai-hero p,
.kpi-ai-panel p {
  margin: 0.35rem 0 0;
  line-height: 1.5;
  color: #475569;
}
.kpi-ai-grid {
  display: grid;
  gap: 0.65rem;
}
.kpi-ai-grid--metrics {
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
}
.kpi-ai-grid--main {
  grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
}
.kpi-ai-panel {
  min-width: 0;
  padding: 0.8rem 0.85rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #fff;
}
.kpi-ai-panel h4 {
  margin: 0 0 0.55rem;
  font-size: 0.875rem;
  font-weight: 800;
}
.kpi-ai-panel > span,
.kpi-ai-list span,
.kpi-ai-actions span,
.kpi-ai-task span {
  display: block;
  font-size: 0.7rem;
  line-height: 1.35;
  color: #64748b;
}
.kpi-ai-panel > strong {
  display: block;
  margin-top: 0.2rem;
  font-size: 1.15rem;
  line-height: 1.2;
}
.kpi-ai-list {
  display: grid;
  gap: 0.6rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
.kpi-ai-list li {
  padding-bottom: 0.6rem;
  border-bottom: 1px solid #edf2f7;
}
.kpi-ai-list li:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}
.kpi-ai-list strong,
.kpi-ai-actions strong,
.kpi-ai-task strong {
  display: block;
  font-size: 0.8125rem;
  color: #0f172a;
}
.kpi-ai-list small {
  display: block;
  margin-top: 0.3rem;
  color: #0f766e;
}
.kpi-ai-actions {
  display: grid;
  gap: 0.45rem;
  margin-top: 0.7rem;
}
.kpi-ai-actions div {
  padding: 0.6rem;
  border-radius: 0.5rem;
  background: #f8fafc;
}
.kpi-ai-task-list {
  display: grid;
  gap: 0.65rem;
}
.kpi-ai-task {
  padding: 0.65rem;
  border-radius: 0.5rem;
  background: #f8fafc;
}
.kpi-ai-task ol {
  margin: 0.5rem 0 0;
  padding-left: 1.15rem;
}
.kpi-ai-task li {
  margin: 0.18rem 0;
  color: #334155;
  line-height: 1.45;
}
.kpi-ai-muted {
  color: #94a3b8;
}
.kpi-ai-evidence {
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #f8fafc;
  padding: 0.7rem 0.85rem;
}
.kpi-ai-evidence summary {
  cursor: pointer;
  font-weight: 700;
}
.kpi-ai-evidence ul {
  margin: 0.55rem 0 0;
  padding-left: 1.1rem;
}
.kpi-ai-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
  color: #64748b;
  font-size: 0.75rem;
}
.kpi-ai-footer-actions {
  display: inline-flex;
  gap: 0.5rem;
}
@keyframes kpiPulse {
  0% {
    box-shadow: 0 0 0 0 rgba(37, 99, 235, 0.32);
  }
  70% {
    box-shadow: 0 0 0 0.55rem rgba(37, 99, 235, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(37, 99, 235, 0);
  }
}
@media (min-width: 1180px) {
  .role-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .kpi-table {
    min-width: 34rem;
  }
  .kpi-table--design {
    min-width: 60rem;
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
