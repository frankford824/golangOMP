/**
 * 页面职责：任务运营主页总览（看板布局）
 * 数据：useTasksStore 列表 + useAuditsStore 审计记录聚合；图表由当前列表统计。
 */
<template>
  <div
    class="dashboard-shell min-h-[100dvh] w-full min-w-0 max-w-full overflow-x-hidden bg-stone-100 text-slate-900"
  >
    <div v-if="error" class="dashboard-error mx-auto max-w-lg p-8 text-center">
      <p>{{ error }}</p>
      <BaseButton variant="primary" size="sm" @click="load">重试</BaseButton>
    </div>
    <template v-else-if="hasBusinessAccess">
      <div
        class="board dashboard-workbench mx-auto w-full min-w-0 max-w-[min(100%,90rem)] space-y-4 px-3 pb-8 pt-4 sm:space-y-5 sm:px-4 sm:pt-5 md:space-y-6 md:px-6 md:pt-6 lg:px-8"
      >
        <!-- 顶栏 -->
        <header
          class="board-header rounded-xl bg-white p-4 shadow-sm shadow-slate-900/[0.04] ring-1 ring-slate-200/60 sm:p-5"
        >
          <div class="board-header-content">
            <p class="board-header-kicker">运营工作台</p>
            <h1 class="m-0 text-[1.25rem] font-semibold leading-tight text-[#0F172A] sm:text-[1.5rem] md:text-[1.625rem]">
              任务运营主页总览
            </h1>
            <p class="board-header-subtitle">
              先看今日是否正常，再判断队列积压、质量风险和下一步处理入口。
            </p>
          </div>
          <div class="board-header-aside" aria-hidden="true">
            <span>实时</span>
            <strong>{{ summary.todayPendingCount }}</strong>
            <small>待处理</small>
          </div>
        </header>

        <!-- 快捷健康条（原「任务健康度」能力保留为单行） -->
        <div
          v-if="!loading"
          class="dashboard-health-strip flex flex-col gap-3 text-xs text-slate-500 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-3 sm:gap-y-1 sm:text-sm"
        >
          <div class="dashboard-health-metrics flex flex-wrap items-center gap-2">
            <span class="dashboard-health-item dashboard-health-item--success">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">系统健康</span>
              <strong class="dashboard-health-value">正常</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--load">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">待仓库接收</span>
              <strong class="dashboard-health-value">{{ summary.pendingWarehouseReceiveCount }}</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--quality">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">打回率</span>
              <strong class="dashboard-health-value">{{ kpiStats.rejectRateLabel }}</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--danger">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">即将逾期</span>
              <strong class="dashboard-health-value">{{ summary.overdueCount }}</strong>
            </span>
          </div>
          <BaseButton
            class="w-full shrink-0 sm:ml-1 sm:w-auto"
            variant="primary"
            size="sm"
            @click="goTaskPool"
          >
            查看未指派任务
          </BaseButton>
        </div>

        <!-- KPI 详情（需权限，沿用原统计口径） -->
        <section
          v-if="can('kpi.view')"
          class="kpi kpi--quality grid grid-cols-1 gap-3 min-[400px]:grid-cols-2 lg:grid-cols-4 lg:gap-4"
        >
          <DashboardKpiCard
            title="本周完成率"
            :value="kpiStats.completedRateLabel"
            hint="本周已完成任务 / 总任务"
          />
          <DashboardKpiCard
            title="打回率"
            :value="kpiStats.rejectRateLabel"
            hint="本周审核打回 / 总审核"
          />
          <DashboardKpiCard
            title="平均处理时长"
            :value="kpiStats.avgHoursLabel"
            hint="已完成任务平均耗时"
          />
          <DashboardKpiCard
            title="待处理任务数"
            :value="kpiStats.pendingCount"
            hint="按当前角色数据范围"
            route="/tasks"
          />
        </section>

        <!-- KPI 四卡 -->
        <div v-if="loading" class="kpi-skeleton">
          <StatusSkeleton :loading="true" :lines="2" class="!py-0" />
        </div>
        <section
          v-else
          class="kpi kpi--queues grid grid-cols-1 gap-3 min-[400px]:grid-cols-2 lg:grid-cols-4 lg:gap-4"
        >
          <DashboardKpiCard
            title="今日待处理"
            :value="summary.todayPendingCount"
            hint="需您处理的任务"
            route="/tasks"
          />
          <DashboardKpiCard
            title="待审核"
            :value="summary.pendingAuditCount"
            hint="待审核任务"
            route="/tasks?status=PendingAuditA,PendingAuditB"
          />
          <DashboardKpiCard
            title="需交班"
            :value="summary.handoverCount"
            hint="审核交班任务"
            route="/tasks?status=PendingAuditA,PendingAuditB"
          />
          <DashboardKpiCard
            title="今日新建"
            :value="summary.todayCreatedCount"
            route="/tasks"
          />
        </section>

        <!-- 图表行：随屏宽单栏 → 双栏 -->
        <section
          class="dashboard-analysis-grid grid grid-cols-1 items-stretch gap-4 min-[1100px]:grid-cols-2 min-[1100px]:gap-5"
        >
          <article
            class="board-panel board-panel--trend flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
          >
            <div class="chart-panel-head">
              <div>
                <h2 class="m-0 text-[0.9rem] font-semibold text-[#0F172A] sm:text-[0.95rem]">
                  7日任务创建 / 完成趋势
                </h2>
                <p class="chart-panel-sub">先看今日结论，再看近 7 日趋势</p>
              </div>
              <span class="chart-panel-badge">今日概览</span>
            </div>
            <div class="trend-summary-grid" aria-label="今日任务摘要">
              <div class="trend-summary-card trend-summary-card--created">
                <span class="trend-summary-label">今日新建</span>
                <strong class="trend-summary-value">{{ trendTodayStats.created }}</strong>
              </div>
              <div class="trend-summary-card trend-summary-card--completed">
                <span class="trend-summary-label">今日完成</span>
                <strong class="trend-summary-value">{{ trendTodayStats.completed }}</strong>
              </div>
              <div class="trend-summary-card trend-summary-card--due">
                <span class="trend-summary-label">当日截止</span>
                <strong class="trend-summary-value">{{ trendTodayStats.dueOnDay }}</strong>
              </div>
            </div>
            <DashboardTrendChart
              :labels="trend7d.labels"
              :created="trend7d.created"
              :completed="trend7d.completed"
              :due-on-day="trend7d.dueOnDay"
              :loading="loading"
            />
            <p class="m-0 text-[0.7rem] text-slate-400">近7日新建、完成与截止日分布</p>
          </article>
          <article
            class="board-panel board-panel--status flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
          >
            <div class="chart-panel-head">
              <div>
                <h2 class="m-0 text-[0.9rem] font-semibold text-[#0F172A] sm:text-[0.95rem]">
                  任务状态分布
                </h2>
                <p class="chart-panel-sub">{{ statusDistribution.conclusion }}</p>
              </div>
              <span class="chart-panel-badge">状态分布</span>
            </div>
            <div v-if="loading" class="status-distribution-skeleton">
              <StatusSkeleton :loading="true" :lines="4" class="!py-0" />
            </div>
            <div v-else class="status-distribution" aria-label="任务状态分布">
              <div class="status-stack-bar" aria-hidden="true">
                <span
                  v-for="item in statusDistribution.items"
                  :key="item.key"
                  class="status-stack-seg"
                  :class="`status-stack-seg--${item.key}`"
                  :style="{ flexGrow: item.value, flexBasis: `${item.percent}%` }"
                />
              </div>
              <p class="status-stack-caption">{{ statusDistribution.caption }}</p>
              <div class="status-cards">
                <article
                  v-for="item in statusDistribution.items"
                  :key="item.key"
                  class="status-card"
                  :class="`status-card--${item.key}`"
                >
                  <span class="status-card-dot" aria-hidden="true" />
                  <span class="status-card-main">
                    <strong>{{ item.name }}</strong>
                    <small>{{ item.hint }}</small>
                  </span>
                  <span class="status-card-value">{{ item.value }}</span>
                </article>
              </div>
            </div>
          </article>
        </section>

        <!-- 表格 + 动态 -->
        <section
          class="dashboard-bottom-grid grid grid-cols-1 items-stretch gap-4 min-[1100px]:grid-cols-2 min-[1100px]:gap-5"
        >
          <article
            class="board-panel board-panel--tasks flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:p-5"
          >
            <h2 class="m-0 mb-3 text-[0.9rem] font-semibold text-[#0F172A] sm:mb-4 sm:text-[0.95rem]">近期任务明细</h2>
            <div v-if="loading" class="py-4">
              <StatusSkeleton :loading="true" :lines="4" class="!py-0" />
            </div>
            <div v-else class="min-w-0 overflow-x-auto -mx-1 px-1 sm:mx-0 sm:px-0">
              <DashboardTaskSnapshotTable :tasks="snapshotTasks" />
            </div>
          </article>
          <article
            class="board-panel board-panel--activity flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
          >
            <h2 class="m-0 text-[0.9rem] font-semibold text-[#0F172A] sm:text-[0.95rem]">实时动态</h2>
            <p class="m-0 text-xs text-slate-400 sm:text-[0.7rem]">审核、交班与新建</p>
            <div
              class="min-h-[12rem] flex-1 overflow-auto rounded-lg bg-slate-50 p-3 min-[1100px]:min-h-[16rem] sm:p-3 ring-1 ring-slate-200/80"
            >
              <RecentEventStream
                :events="recentEvents"
                :loading="auditsStore.loading"
                :error="auditsStore.loadError"
                hide-title
                variant="dashboard"
              />
            </div>
          </article>
        </section>

        <!-- 风险提示 -->
        <section class="dashboard-risk-zone min-w-0">
          <RiskListCard :items="risks" :loading="loading" :error="error" />
        </section>
      </div>
    </template>
    <template v-else>
      <section class="dashboard-empty" role="status" aria-live="polite">
        <div class="empty-inner">
          <div class="empty-icon-ring">
            <svg
              class="empty-icon"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <polyline points="17 11 19 13 23 9" />
            </svg>
          </div>
          <h2 class="empty-title">欢迎登录</h2>
          <p class="empty-desc">您当前暂无业务权限，请联系所在部门的管理员为您分配角色。</p>
          <p class="empty-muted">分配后重新登录即可查看相应业务板块。</p>
          <div class="empty-hint">
            <span class="empty-hint-dot" aria-hidden="true" />
            <span>角色分配通常 30 分钟内生效</span>
          </div>
          <div class="empty-steps">
            <article class="empty-step" data-accent="blue">
              <div class="empty-step-num">1</div>
              <h3 class="empty-step-title">联系管理员</h3>
              <p class="empty-step-desc">告知所在部门管理员您需要的业务角色</p>
            </article>
            <article class="empty-step" data-accent="violet">
              <div class="empty-step-num">2</div>
              <h3 class="empty-step-title">等待角色分配</h3>
              <p class="empty-step-desc">管理员审批后系统将自动下发您的权限</p>
            </article>
            <article class="empty-step" data-accent="emerald">
              <div class="empty-step-num">3</div>
              <h3 class="empty-step-title">重新登录使用</h3>
              <p class="empty-step-desc">退出后重新登录即可解锁对应业务板块</p>
            </article>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, defineAsyncComponent } from 'vue'
import { useRouter } from 'vue-router'
import type { Task } from '@/domain/types/task'
import type { DashboardSummary, RecentEvent, RiskItem } from '@/types/dashboard'
import { useTasksStore } from '@/stores/tasks'
import { useAuditsStore } from '@/stores/audits'
import StatusSkeleton from '@/components/common/StatusSkeleton.vue'
import DashboardKpiCard from '@/components/dashboard/DashboardKpiCard.vue'
import DashboardTaskSnapshotTable from '@/components/dashboard/DashboardTaskSnapshotTable.vue'
import RecentEventStream from '@/components/dashboard/RecentEventStream.vue'
import RiskListCard from '@/components/dashboard/RiskListCard.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import {
  TODAY_PENDING_STATUSES,
  isInAuditQueue,
  isInCustomizationFlow,
  isDoneStatus,
  isPendingAuditB,
  canGoWarehouse,
  isCompletedOrArchived,
} from '@/domain/task-actions'
import { usePermission } from '@/composables/usePermission'
import {
  getBeijingDateString,
  formatDateBeijing,
  isOverdueByBeijingDay as checkOverdue,
  taskBeijingDateKey,
  taskInstantMs,
} from '@/utils/date'
import { getLastNBeijingDateKeys, beijingDateKeyToShortLabel } from '@/utils/beijing-calendar'

const DashboardTrendChart = defineAsyncComponent(() => import('@/components/dashboard/DashboardTrendChart.vue'))

const router = useRouter()
const tasksStore = useTasksStore()
const auditsStore = useAuditsStore()
const { can, canAccessTask } = usePermission()
const loading = ref(true)
const error = ref('')

const BUSINESS_ACTIONS = [
  'task.list',
  'task.create',
  'task.audit.claim',
  'warehouse.receive',
  'task.customization.submit',
  'design.review',
] as const

const canListTasks = computed(() => can('task.list'))
const hasBusinessAccess = computed(() => BUSINESS_ACTIONS.some((a) => can(a)))
const canAccessAuditLogs = computed(() => can('audit.view'))

function formatEventAt(iso: string): string {
  if (!iso) return ''
  return formatDateBeijing(iso)
}

const summary = computed<DashboardSummary>(() => {
  const list = tasksStore.list
  const todayStr = getBeijingDateString()

  const todayPendingCount = list.filter((t) =>
    (TODAY_PENDING_STATUSES as readonly string[]).includes(t.status),
  ).length
  const pendingAuditCount = list.filter((t) => isInAuditQueue(t)).length
  const handoverCount = list.filter((t) => isPendingAuditB(t)).length
  const pendingOutsourceReturnCount = list.filter((t) => isInCustomizationFlow(t)).length
  const pendingWarehouseReceiveCount = list.filter((t) => canGoWarehouse(t)).length
  const todayCreatedCount = list.filter(
    (t) => taskBeijingDateKey(t.createdAt) === todayStr,
  ).length
  const overdueCount = list.filter((t) => checkOverdue(t.dueAt, isDoneStatus(t))).length

  return {
    todayPendingCount,
    pendingAuditCount,
    handoverCount,
    pendingOutsourceReturnCount,
    pendingWarehouseReceiveCount,
    todayCreatedCount,
    overdueCount,
  }
})

const PIE_NAMES = ['待处理', '待审核', '定制协同', '待仓库', '已完成/关单'] as const

function statusPieSlice(t: Task): 0 | 1 | 2 | 3 | 4 {
  if (isCompletedOrArchived(t)) return 4
  if (isInAuditQueue(t)) return 1
  if (isInCustomizationFlow(t)) return 2
  if (canGoWarehouse(t)) return 3
  return 0
}

const statusPieSeries = computed(() => {
  const c: [number, number, number, number, number] = [0, 0, 0, 0, 0]
  for (const t of tasksStore.list) {
    c[statusPieSlice(t)] += 1
  }
  return PIE_NAMES.map((name, i) => ({ name, value: c[i] }))
})

const trend7d = computed(() => {
  const keys = getLastNBeijingDateKeys(7)
  const list = tasksStore.list
  const labels = keys.map(beijingDateKeyToShortLabel)
  const created = keys.map(
    (day) => list.filter((t) => taskBeijingDateKey(t.createdAt) === day).length,
  )
  const completed = keys.map(
    (day) =>
      list.filter(
        (t) => isCompletedOrArchived(t) && taskBeijingDateKey(t.updatedAt) === day,
      ).length,
  )
  const dueOnDay = keys.map(
    (day) => list.filter((t) => taskBeijingDateKey(t.dueAt) === day).length,
  )
  return { labels, created, completed, dueOnDay }
})

const trendTodayStats = computed(() => {
  const last = trend7d.value.labels.length - 1
  return {
    created: trend7d.value.created[last] ?? 0,
    completed: trend7d.value.completed[last] ?? 0,
    dueOnDay: trend7d.value.dueOnDay[last] ?? 0,
  }
})

const statusDistribution = computed(() => {
  const valueOf = (name: (typeof PIE_NAMES)[number]) =>
    statusPieSeries.value.find((item) => item.name === name)?.value ?? 0
  const items = [
    {
      key: 'pending',
      name: '待处理',
      value: valueOf('待处理'),
      hint: '设计 / 运营下一步动作',
    },
    {
      key: 'audit',
      name: '待审核',
      value: valueOf('待审核'),
      hint: '审核队列待处理',
    },
    {
      key: 'warehouse',
      name: '待仓库',
      value: valueOf('待仓库'),
      hint: '交付链路待接收',
    },
  ] as const
  const total = items.reduce((sum, item) => sum + item.value, 0)
  const withPercent = items.map((item) => ({
    ...item,
    percent: total > 0 ? Math.round((item.value / total) * 100) : 0,
  }))
  const lead = [...withPercent].sort((a, b) => b.value - a.value)[0]
  return {
    items: withPercent,
    total,
    conclusion:
      total > 0
        ? `当前积压主要集中在「${lead.name}」阶段，共 ${lead.value} 项`
        : '当前暂无明显积压',
    caption:
      total > 0
        ? withPercent.map((item) => `${item.name} ${item.percent}%`).join(' · ')
        : '待处理 0% · 待审核 0% · 待仓库 0%',
  }
})

const snapshotTasks = computed(() => {
  return [...tasksStore.list]
    .sort(
      (a, b) =>
        taskInstantMs(b.updatedAt) - taskInstantMs(a.updatedAt),
    )
    .slice(0, 8)
})

const RECENT_EVENT_LIMIT = 20

const recentEvents = computed<RecentEvent[]>(() => {
  const events: RecentEvent[] = []
  const taskById = (id: string) => tasksStore.getById(id)

  for (const r of auditsStore.records) {
    const task = taskById(r.taskId)
    const refNo = task?.taskNo ?? (r.taskId || '未知')
    const type =
      r.action === 'pass'
        ? 'audit_passed'
        : r.action === 'reject'
          ? 'audit_rejected'
          : 'handover'
    const title =
      type === 'audit_passed'
        ? '初审/复审通过'
        : type === 'audit_rejected'
          ? '审核打回'
          : '审核交班'
    events.push({
      id: `audit-${r.id}`,
      type,
      title,
      refId: task?.id ?? '',
      refNo,
      actor: r.auditorName,
      at: formatEventAt(r.createdAt),
    })
  }
  for (const h of auditsStore.handovers) {
    const task = taskById(h.taskId)
    events.push({
      id: `handover-${h.id}`,
      type: 'handover',
      title: '审核交班',
      refId: task?.id ?? '',
      refNo: task?.taskNo ?? (h.taskId || '未知'),
      actor: h.fromUserName,
      at: formatEventAt(h.createdAt),
    })
  }
  const todayStr = getBeijingDateString()
  for (const t of tasksStore.list) {
    if (taskBeijingDateKey(t.createdAt) !== todayStr) continue
    events.push({
      id: `task-created-${t.id}`,
      type: 'task_created',
      title: '新建任务',
      refId: t.id,
      refNo: t.taskNo,
      actor: t.requesterName,
      at: formatEventAt(t.createdAt),
    })
  }
  events.sort((a, b) => {
    const atA = a.at.replace(' ', 'T')
    const atB = b.at.replace(' ', 'T')
    return atB.localeCompare(atA)
  })
  return events.slice(0, RECENT_EVENT_LIMIT)
})

const risks = computed<RiskItem[]>(() => {
  const list: RiskItem[] = []
  for (const t of tasksStore.list) {
    if (!t.dueAt || isDoneStatus(t)) continue
    if (!checkOverdue(t.dueAt, false)) continue
    list.push({
      id: `overdue-${t.id}`,
      level: 'high',
      message: `任务 ${t.taskNo} 已逾期`,
      refId: t.id,
      refNo: t.taskNo,
    })
  }
  const pendingOutsource = tasksStore.list.filter((t) => isInCustomizationFlow(t)).length
  if (pendingOutsource > 0) {
    list.push({
      id: 'outsource-pending',
      level: 'medium',
      message: `${pendingOutsource} 个定制任务处理中`,
      route: '/tasks?task_category=customization',
    })
  }
  return list
})

const kpiStats = computed(() => {
  const now = Date.now()
  const weekMs = 7 * 24 * 60 * 60 * 1000

  const tasks = tasksStore.list
  const audits = auditsStore.records

  const thisWeekTasks = tasks.filter((t) => {
    const created = taskInstantMs(t.createdAt)
    return now - created <= weekMs && now >= created
  })

  const completedThisWeek = thisWeekTasks.filter((t) => t.status === 'Completed')
  const completedRate =
    thisWeekTasks.length > 0 ? (completedThisWeek.length / thisWeekTasks.length) * 100 : 0

  const thisWeekAudits = audits.filter((r) => {
    const ts = taskInstantMs(r.createdAt)
    return now - ts <= weekMs && now >= ts
  })
  const rejectCount = thisWeekAudits.filter((r) => r.action === 'reject').length
  const auditTotal = thisWeekAudits.length || 1
  const rejectRate = (rejectCount / auditTotal) * 100

  const durations: number[] = []
  for (const t of tasks) {
    if (!t.createdAt || !t.updatedAt) continue
    if (t.status !== 'Completed') continue
    const start = taskInstantMs(t.createdAt)
    const end = taskInstantMs(t.updatedAt)
    if (end > start) {
      durations.push((end - start) / (60 * 60 * 1000))
    }
  }
  const avgHours =
    durations.length > 0
      ? durations.reduce((sum, v) => sum + v, 0) / durations.length
      : 0

  const pendingForUser = tasks.filter(
    (t) => !isDoneStatus(t) && canAccessTask(t),
  ).length

  return {
    completedRateLabel: `${completedRate.toFixed(1)}%`,
    rejectRateLabel: `${rejectRate.toFixed(1)}%`,
    avgHoursLabel: `${avgHours.toFixed(1)} 小时`,
    pendingCount: pendingForUser,
  }
})

async function load() {
  loading.value = true
  error.value = ''
  auditsStore.loadError = ''

  try {
    if (canListTasks.value) {
      await tasksStore.loadTasks()
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载任务运营主页总览失败'
  } finally {
    loading.value = false
  }

  if (canAccessAuditLogs.value) {
    try {
      await auditsStore.loadAuditLogs()
    } catch (e) {
      auditsStore.loadError = e instanceof Error ? e.message : '加载审计日志失败'
    }
  }
}

function goTaskPool() {
  void router.push('/tasks?tab=pool')
}

onMounted(load)
</script>

<style scoped>
.dashboard-error {
  background: rgb(254 242 242);
  border: 1px solid rgb(254 202 202);
  border-radius: 1rem;
  color: rgb(153 27 27);
}
.dashboard-error p {
  margin: 0 0 1rem;
}
.kpi-skeleton {
  min-height: 5.5rem;
}
.board-panel {
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.05),
    0 0 0 1px rgba(15, 23, 42, 0.04);
}

/* 无业务权限 — dark glass 欢迎区 */
.dashboard-empty {
  flex: 1 1 auto;
  width: 100%;
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(1.5rem, 3vw, 3rem);
  border-radius: 1.5rem;
  position: relative;
  overflow: hidden;
  border: 1px solid var(--yb-music-border-strong);
  background:
    radial-gradient(circle at 14% 12%, rgba(100, 210, 255, 0.14), transparent 48%),
    radial-gradient(circle at 86% 18%, rgba(175, 82, 222, 0.11), transparent 50%),
    radial-gradient(circle at 50% 100%, rgba(100, 210, 255, 0.06), transparent 55%),
    linear-gradient(145deg, var(--yb-music-surface-strong), rgba(15, 21, 32, 0.94));
  backdrop-filter: blur(var(--yb-glass-blur));
  -webkit-backdrop-filter: blur(var(--yb-glass-blur));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--yb-glass-shadow);
  text-align: center;
}
.empty-inner {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 880px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;
}
.empty-icon-ring {
  width: clamp(72px, 8vw, 96px);
  height: clamp(72px, 8vw, 96px);
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    linear-gradient(145deg, rgba(100, 210, 255, 0.16), rgba(175, 82, 222, 0.12)),
    var(--yb-work-panel-2);
  border: 1px solid var(--yb-music-border-strong);
  color: var(--yb-music-cyan);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    0 10px 28px -14px rgba(100, 210, 255, 0.32);
}
.empty-icon {
  width: 52%;
  height: 52%;
  filter: drop-shadow(0 0 10px rgba(100, 210, 255, 0.35));
}
.empty-title {
  margin: 0;
  font-size: clamp(1.5rem, 2.4vw, 2rem);
  font-weight: 800;
  font-family: Manrope, sans-serif;
  letter-spacing: -0.01em;
  color: var(--yb-music-text);
  text-shadow: 0 12px 32px rgba(0, 0, 0, 0.35);
}
.empty-desc {
  margin: 0;
  max-width: 560px;
  font-size: clamp(0.9375rem, 1.2vw, 1rem);
  font-weight: 500;
  line-height: 1.7;
  color: var(--yb-music-muted);
}
.empty-muted {
  margin: 0;
  max-width: 560px;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.7;
  color: var(--yb-music-faint);
}
.empty-hint {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.42rem 0.95rem;
  border-radius: 9999px;
  border: 1px solid var(--yb-music-border);
  background: rgba(20, 29, 44, 0.72);
  font-size: 0.75rem;
  font-weight: 650;
  color: var(--yb-music-muted);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}
.empty-hint-dot {
  width: 6px;
  height: 6px;
  border-radius: 9999px;
  background: var(--yb-music-cyan);
  box-shadow: 0 0 8px rgba(100, 210, 255, 0.5);
}
.empty-steps {
  margin-top: 1rem;
  width: 100%;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}
.empty-step {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
  padding: 1.1rem 1.1rem 1.25rem;
  border-radius: 0.875rem;
  border: 1px solid var(--yb-music-border);
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.09), rgba(255, 255, 255, 0.03)),
    var(--yb-music-surface-soft);
  text-align: left;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 16px 36px -26px rgba(0, 0, 0, 0.68);
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    box-shadow 0.18s ease;
}
.empty-step::before {
  content: '';
  position: absolute;
  top: 0;
  left: 1rem;
  right: 1rem;
  height: 2px;
  border-radius: 0 0 2px 2px;
  opacity: 0.75;
}
.empty-step[data-accent='blue']::before {
  background: linear-gradient(90deg, transparent, rgba(100, 210, 255, 0.55), transparent);
}
.empty-step[data-accent='violet']::before {
  background: linear-gradient(90deg, transparent, rgba(175, 82, 222, 0.5), transparent);
}
.empty-step[data-accent='emerald']::before {
  background: linear-gradient(90deg, transparent, rgba(48, 209, 88, 0.48), transparent);
}
.empty-step:hover {
  border-color: rgba(255, 255, 255, 0.28);
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.05)),
    rgba(24, 34, 49, 0.92);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.09),
    0 20px 40px -24px rgba(0, 0, 0, 0.72);
}
.empty-step-num {
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: Manrope, sans-serif;
  font-size: 0.8125rem;
  font-weight: 800;
}
.empty-step[data-accent='blue'] .empty-step-num {
  border: 1px solid rgba(100, 210, 255, 0.35);
  background: rgba(37, 99, 235, 0.28);
  color: #93c5fd;
  box-shadow: 0 0 14px -4px rgba(100, 210, 255, 0.35);
}
.empty-step[data-accent='violet'] .empty-step-num {
  border: 1px solid rgba(167, 139, 250, 0.35);
  background: rgba(109, 40, 217, 0.28);
  color: #c4b5fd;
  box-shadow: 0 0 14px -4px rgba(167, 139, 250, 0.32);
}
.empty-step[data-accent='emerald'] .empty-step-num {
  border: 1px solid rgba(74, 222, 128, 0.35);
  background: rgba(21, 128, 61, 0.3);
  color: #86efac;
  box-shadow: 0 0 14px -4px rgba(48, 209, 88, 0.28);
}
.empty-step-title {
  margin: 0;
  font-family: Manrope, sans-serif;
  font-size: 0.9375rem;
  font-weight: 700;
  color: var(--yb-music-text-2);
}
.empty-step-desc {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.6;
  color: var(--yb-music-muted);
}
@media (max-width: 960px) and (min-width: 721px) {
  .empty-steps {
    gap: 0.75rem;
  }
  .empty-step {
    padding: 1rem 0.85rem 1.1rem;
  }
  .empty-step-title {
    font-size: 0.875rem;
  }
  .empty-step-desc {
    font-size: 0.78rem;
  }
}
@media (max-width: 720px) {
  .empty-steps {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (prefers-reduced-motion: reduce) {
  .empty-step {
    transition: none;
  }
}

/* Apple Music / iOS liquid glass dashboard skin. Style-only. */
:global(body) {
  background-color: #0a0d14;
}

.dashboard-shell {
  background:
    radial-gradient(circle at 8% 4%, rgba(175, 82, 222, 0.18), transparent 22rem),
    radial-gradient(circle at 92% 10%, rgba(100, 210, 255, 0.16), transparent 24rem),
    linear-gradient(125deg, #0a0d14 0%, #141a26 54%, #0b1520 100%) !important;
}

.board {
  width: min(100%, 1936px);
  max-width: min(100%, 1936px) !important;
  color: var(--yb-music-text-2);
  position: relative;
  isolation: isolate;
}

.dashboard-workbench {
  padding-left: clamp(1rem, 1.65vw, 2rem) !important;
  padding-right: clamp(1rem, 1.65vw, 2rem) !important;
}

.board::before,
.board::after {
  content: '';
  position: fixed;
  z-index: -1;
  pointer-events: none;
  filter: blur(42px);
}

.board::before {
  left: 5rem;
  top: 5rem;
  width: 28rem;
  height: 18rem;
  border-radius: 999px;
  background: rgba(175, 82, 222, 0.14);
}

.board::after {
  right: 4rem;
  top: 7rem;
  width: 26rem;
  height: 16rem;
  border-radius: 999px;
  background: rgba(100, 210, 255, 0.12);
}

.board-header {
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  min-height: clamp(8rem, 13vh, 9.25rem);
  padding: clamp(1rem, 1.8vw, 1.65rem) clamp(1.1rem, 2.2vw, 2rem) !important;
  border: 1px solid var(--yb-music-border-strong) !important;
  border-radius: 1.125rem !important;
  background:
    linear-gradient(90deg, rgba(255, 45, 141, 0.17), transparent 31%),
    radial-gradient(circle at 78% 22%, rgba(100, 210, 255, 0.2), transparent 18rem),
    linear-gradient(120deg, rgba(24, 31, 45, 0.96), rgba(15, 21, 32, 0.9)) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    0 34px 80px -48px rgba(0, 0, 0, 0.72) !important;
}

.board-header::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background:
    radial-gradient(circle at 16% 16%, rgba(255, 255, 255, 0.16), transparent 16rem),
    linear-gradient(110deg, transparent 0%, transparent 57%, rgba(255, 45, 85, 0.42) 57%, rgba(175, 82, 222, 0.38) 75%, rgba(100, 210, 255, 0.35) 100%);
  opacity: 0.8;
}

.board-header::after {
  content: '';
  position: absolute;
  right: 1.25rem;
  bottom: -4rem;
  width: 19rem;
  height: 10rem;
  border-radius: 999px;
  background: rgba(100, 210, 255, 0.14);
  filter: blur(18px);
}

.board-header-content,
.board-header-aside {
  position: relative;
  z-index: 1;
}

.board-header-content {
  min-width: 0;
}

.board-header-kicker {
  margin: 0 0 0.55rem;
  color: #64d2ff;
  font-size: 0.74rem;
  font-weight: 850;
  letter-spacing: 0.06em;
}

.board-header h1 {
  position: relative;
  z-index: 1;
  max-width: min(58rem, 100%);
  color: #fff !important;
  font-size: clamp(2.15rem, 3vw, 3.8rem) !important;
  font-weight: 900 !important;
  line-height: 1.04 !important;
  text-shadow: 0 18px 44px rgba(0, 0, 0, 0.36);
}

.board-header-subtitle {
  max-width: 46rem;
  margin: 0.7rem 0 0;
  color: #b8c2d6;
  font-size: 0.9rem;
  font-weight: 650;
  line-height: 1.6;
}

.board-header-aside {
  display: grid;
  min-width: 8.75rem;
  place-items: center;
  align-self: stretch;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 0.95rem;
  padding: 0.8rem 1rem;
  background: rgba(20, 29, 44, 0.44);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.board-header-aside span,
.board-header-aside small {
  color: #8fa0b8;
  font-size: 0.72rem;
  font-weight: 800;
}

.board-header-aside strong {
  color: #fff;
  font-family: var(--yb-font-data);
  font-size: clamp(2rem, 3vw, 3.1rem);
  line-height: 1;
}

.dashboard-health-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 4rem;
  border: 1px solid var(--yb-music-border);
  border-radius: 0.875rem;
  padding: 0.7rem;
  background:
    linear-gradient(90deg, rgba(19, 26, 39, 0.94), rgba(23, 31, 46, 0.84)),
    rgba(255, 255, 255, 0.045);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.07),
    0 22px 52px -44px rgba(0, 0, 0, 0.68);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

.dashboard-health-metrics {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.dashboard-health-item {
  display: inline-flex;
  flex: 1 1 0;
  align-items: center;
  min-width: 0;
  min-height: 2.65rem;
  gap: 0.42rem;
  border: 1px solid #2a3443;
  border-radius: 0.55rem;
  padding: 0.5rem 0.65rem;
  background: #1a2231;
  color: var(--yb-music-muted);
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.dashboard-health-item:first-child {
  flex-basis: 18rem;
  flex-grow: 0;
}

.dashboard-health-dot {
  display: inline-block;
  flex: 0 0 auto;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: var(--yb-music-cyan);
  box-shadow: 0 0 12px rgba(100, 210, 255, 0.6);
}

.dashboard-health-item--success {
  border-color: #244532;
  background: #15291f;
}

.dashboard-health-item--success .dashboard-health-dot {
  background: #30d158;
  box-shadow: 0 0 12px rgba(48, 209, 88, 0.55);
}

.dashboard-health-item--quality .dashboard-health-dot {
  background: #86efac;
  box-shadow: 0 0 12px rgba(134, 239, 172, 0.42);
}

.dashboard-health-item--load .dashboard-health-dot {
  background: #64d2ff;
}

.dashboard-health-item--danger {
  border-color: #5c3518;
  background: #352212;
}

.dashboard-health-item--danger .dashboard-health-dot {
  background: #ff9f0a;
  box-shadow: 0 0 12px rgba(255, 159, 10, 0.5);
}

.dashboard-health-label {
  display: inline-block;
}

.dashboard-health-value,
.dashboard-health-strip strong {
  color: #fff !important;
  font-family: var(--yb-font-data);
  font-weight: 850;
}

.dashboard-health-item--danger .dashboard-health-value {
  color: #ffd1df !important;
}

.dashboard-health-strip :deep(button),
.dashboard-health-strip button {
  min-height: 2.65rem;
  flex: 0 0 auto;
  border-color: rgba(147, 197, 253, 0.48) !important;
  border-radius: 0.55rem !important;
  background: rgba(37, 99, 235, 0.36) !important;
  color: #fff !important;
  box-shadow: 0 14px 30px -24px rgba(96, 165, 250, 0.9);
}

.kpi {
  align-items: stretch;
}

.kpi--quality,
.kpi--queues {
  gap: 0.75rem !important;
}

.dashboard-analysis-grid {
  grid-template-columns: minmax(0, 1fr) !important;
  gap: 1rem !important;
}

.dashboard-bottom-grid {
  grid-template-columns: minmax(0, 1fr) !important;
  gap: 1rem !important;
}

.dashboard-risk-zone {
  border-radius: 0.875rem;
}

.board-panel,
.risk-card {
  border: 1px solid var(--yb-music-border) !important;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.13), rgba(255, 255, 255, 0.064)) !important;
  color: var(--yb-music-text-2) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.07),
    0 18px 36px -30px rgba(0, 0, 0, 0.58) !important;
}

/* KPI 八卡：默认即「统一雾面面板」，避免浅色渐变 + 顶条伪元素在默认态形成横向碎层 */
.kpi-card {
  position: relative;
  overflow: hidden;
  min-height: 7.85rem;
  border-radius: 0.875rem !important;
  padding: 1rem !important;
  border: 1px solid rgba(255, 255, 255, 0.22) !important;
  background: rgba(255, 255, 255, 0.14) !important;
  color: var(--yb-music-text-2) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.09),
    0 18px 36px -30px rgba(0, 0, 0, 0.58) !important;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease,
    transform 0.18s ease;
}

.kpi-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 84% 12%, rgba(100, 210, 255, 0.14), transparent 8rem);
  opacity: 0.35;
  pointer-events: none;
}

.kpi-card:hover {
  border-color: rgba(255, 255, 255, 0.36) !important;
  background: rgba(255, 255, 255, 0.18) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    0 22px 44px -26px rgba(0, 0, 0, 0.62) !important;
  transform: translateY(-2px);
}

@media (prefers-reduced-motion: reduce) {
  .kpi-card:hover {
    transform: none;
  }
}

.board-panel:hover,
.risk-card:hover {
  border-color: rgba(255, 255, 255, 0.3) !important;
  background: rgba(255, 255, 255, 0.14) !important;
}

.board-panel,
.risk-card {
  border-radius: 0.875rem !important;
  padding: 1rem !important;
}

.board-panel--tasks,
.board-panel--activity {
  min-height: 19rem;
  border-color: rgba(196, 214, 246, 0.34) !important;
  background:
    linear-gradient(145deg, rgba(47, 57, 72, 0.88), rgba(30, 38, 52, 0.9)) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    inset 0 0 0 1px rgba(255, 255, 255, 0.035),
    0 20px 44px -34px rgba(0, 0, 0, 0.72) !important;
}

.board-panel--activity > div:last-child {
  border-radius: 0.75rem !important;
}

:deep(.kpi-card__label),
:deep(.kpi-card__hint),
.board-panel p,
:deep(.risk-empty),
:deep(.risk-loading) {
  color: var(--yb-music-muted) !important;
}

:deep(.kpi-card__value) {
  color: #fff !important;
  font-family: var(--yb-font-data) !important;
  font-size: clamp(1.75rem, 2vw, 2.4rem) !important;
  text-shadow: 0 0 24px rgba(100, 210, 255, 0.22);
}

.board-panel h2,
:deep(.risk-title),
:deep(.task-table__head),
:deep(.task-table__row),
:deep(.stream-item) {
  color: var(--yb-music-text-2) !important;
}

.board-panel :deep(.trend__chart),
.board-panel :deep(.pie__chart),
.board-panel :deep(.trend__skeleton),
.board-panel :deep(.pie__skeleton) {
  background: rgba(18, 25, 38, 0.58) !important;
  border-color: rgba(210, 226, 255, 0.15) !important;
}

.board-panel--trend,
.board-panel--status {
  background: linear-gradient(145deg, rgba(25, 32, 46, 0.97), rgba(16, 23, 35, 0.98)) !important;
  border-color: rgba(184, 200, 228, 0.24) !important;
  min-height: 24rem;
}

.chart-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  border: 0 !important;
  background: transparent !important;
}

.chart-panel-sub {
  margin: 0.3rem 0 0;
  color: #aeb9cc !important;
  font-size: 0.74rem;
  line-height: 1.4;
}

.chart-panel-badge {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  min-height: 1.75rem;
  padding: 0.3rem 0.62rem;
  border: 1px solid #3a4354;
  border-radius: 0.55rem;
  background: #1f2937;
  color: #93c5fd;
  font-size: 0.72rem;
  font-weight: 800;
}

.trend-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
  border: 0 !important;
  background: transparent !important;
}

.trend-summary-card {
  min-width: 0;
  padding: 0.7rem 0.75rem;
  border: 1px solid rgba(210, 226, 255, 0.14);
  border-radius: 0.75rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent),
    #151d2b;
}

.trend-summary-label {
  display: block;
  color: #8fa0b8;
  font-size: 0.72rem;
  font-weight: 800;
}

.trend-summary-value {
  display: block;
  margin-top: 0.2rem;
  font-family: var(--yb-font-data);
  font-size: clamp(1.25rem, 2.4vw, 1.75rem);
  font-weight: 800;
  line-height: 1;
}

.trend-summary-card--created .trend-summary-value {
  color: #93a9ff;
}

.trend-summary-card--completed .trend-summary-value {
  color: #86efac;
}

.trend-summary-card--due .trend-summary-value {
  color: #ff7a7a;
}

.board-panel--trend :deep(.trend__chart),
.board-panel--trend :deep(.trend__skeleton) {
  min-height: 13rem !important;
  border: 1px solid rgba(210, 226, 255, 0.14) !important;
  border-radius: 0.875rem !important;
  background:
    linear-gradient(180deg, rgba(19, 27, 40, 0.95), rgba(14, 20, 31, 0.95)) !important;
}

.status-distribution-skeleton,
.status-distribution {
  display: flex;
  min-height: 13rem;
  flex-direction: column;
  gap: 0.75rem;
  border: 0 !important;
  background: transparent !important;
}

.status-stack-bar {
  display: flex;
  width: 100%;
  min-height: 1.125rem;
  overflow: hidden;
  gap: 0.15rem;
  border: 1px solid rgba(210, 226, 255, 0.14);
  border-radius: 999px;
  background: #263349;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.03);
}

.status-stack-seg {
  min-width: 0;
  transition: opacity 0.16s ease;
}

.status-stack-seg--pending {
  background: #6b8cff;
}

.status-stack-seg--audit {
  background: #86efac;
}

.status-stack-seg--warehouse {
  background: #ffd166;
}

.status-stack-caption {
  margin: 0;
  color: #b8c2d6 !important;
  font-family: var(--yb-font-data);
  font-size: 0.74rem;
}

.status-cards {
  display: grid;
  gap: 0.5rem;
}

.status-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  min-height: 3.4rem;
  padding: 0.58rem 0.65rem;
  border: 1px solid rgba(210, 226, 255, 0.14);
  border-radius: 0.75rem;
  background: #151d2b;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease;
}

.status-card:hover {
  border-color: rgba(147, 197, 253, 0.42);
  background: #1c2637;
}

.status-card-dot {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
}

.status-card--pending .status-card-dot {
  background: #6b8cff;
}

.status-card--audit .status-card-dot {
  background: #86efac;
}

.status-card--warehouse .status-card-dot {
  background: #ffd166;
}

.status-card-main {
  min-width: 0;
}

.status-card-main strong,
.status-card-main small {
  display: block;
}

.status-card-main strong {
  color: #f8fafc;
  font-size: 0.82rem;
  font-weight: 850;
}

.status-card-main small {
  margin-top: 0.12rem;
  overflow: hidden;
  color: #8fa0b8;
  font-size: 0.7rem;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-card-value {
  font-family: var(--yb-font-data);
  font-size: 1.35rem;
  font-weight: 800;
  line-height: 1;
}

.status-card--pending .status-card-value {
  color: #93a9ff;
}

.status-card--audit .status-card-value {
  color: #86efac;
}

.status-card--warehouse .status-card-value {
  color: #ffd166;
}

.board-panel :deep(.task-table__head) {
  color: rgba(236, 244, 255, 0.88) !important;
  background: rgba(35, 45, 61, 0.78) !important;
  border-radius: 0.75rem 0.75rem 0 0;
  border-color: rgba(210, 226, 255, 0.14) !important;
}

.board-panel :deep(.task-table__row) {
  min-height: 3rem;
  color: #f5f8ff !important;
  border-color: rgba(210, 226, 255, 0.12) !important;
}

.board-panel :deep(.col-status) {
  color: #d8e6ff !important;
  font-weight: 800;
}

.board-panel--tasks :deep(.task-table) {
  overflow: hidden;
  border: 1px solid rgba(210, 226, 255, 0.22);
  border-radius: 0.75rem;
  background: rgba(12, 18, 28, 0.72) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.055);
}

.board-panel :deep(.task-table__row--alt),
.board-panel :deep(.task-table__row:not(.task-table__row--alt)) {
  background: rgba(20, 29, 43, 0.78) !important;
}

.board-panel :deep(.task-table__row--alt) {
  background: rgba(24, 34, 49, 0.84) !important;
}

.board-panel :deep(.task-table__status--info) {
  color: #7dd3fc !important;
}

.board-panel :deep(.task-table__status--warning) {
  color: #ffd166 !important;
}

.board-panel :deep(.task-table__status--success) {
  color: #86efac !important;
}

.board-panel :deep(.task-table__status--neutral) {
  color: #c6d3e8 !important;
}

.board-panel :deep(.task-table__row:hover),
.board-panel :deep(.stream-item.navigable:hover),
:deep(.risk-item.navigable:hover) {
  background: rgba(255, 255, 255, 0.105) !important;
  color: #fff !important;
}

.board-panel :deep(.col-task),
.board-panel :deep(.col-owner),
.board-panel :deep(.col-due),
.board-panel :deep(.event-title),
.board-panel :deep(.event-actor),
:deep(.risk-message) {
  color: var(--yb-music-text-2) !important;
}

.board-panel :deep(.event-time),
.board-panel :deep(.event-time--short) {
  color: rgba(220, 230, 255, 0.62) !important;
}

.board-panel :deep(.stream-item) {
  border-color: rgba(210, 226, 255, 0.12) !important;
}

.board-panel--activity > div:last-child {
  background: rgba(12, 18, 28, 0.62) !important;
  border: 1px solid rgba(210, 226, 255, 0.2) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.055),
    0 12px 28px -26px rgba(0, 0, 0, 0.72);
}

.board-panel :deep(.stream-list) {
  background: transparent !important;
}

.board-panel :deep(.stream-list--dashboard .stream-item) {
  margin-bottom: 0.25rem;
  padding: 0.58rem 0.55rem !important;
  border: 1px solid rgba(210, 226, 255, 0.13) !important;
  border-radius: 0.65rem !important;
  background: rgba(25, 36, 53, 0.9) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.045);
}

.board-panel :deep(.stream-list--dashboard .stream-item:nth-child(even)) {
  background: rgba(30, 42, 61, 0.92) !important;
}

:deep(.risk-item) {
  border-radius: 0.65rem;
  padding: 0.58rem 0.55rem !important;
}

:deep(.event-ref--link),
:deep(.risk-ref--link) {
  color: var(--yb-music-cyan) !important;
}

@media (min-width: 1100px) {
  .dashboard-analysis-grid {
    grid-template-columns: minmax(0, 1.35fr) minmax(18rem, 0.75fr) !important;
  }

  .dashboard-bottom-grid {
    grid-template-columns: minmax(0, 1.35fr) minmax(21rem, 0.72fr) !important;
  }
}

@media (min-width: 1440px) {
  .dashboard-workbench {
    max-width: min(100%, 1760px) !important;
  }

  .board-header {
    min-height: 10rem;
  }

  .dashboard-health-strip {
    align-items: center;
    flex-direction: row !important;
  }

  .dashboard-health-metrics {
    align-items: center;
    flex-direction: row;
  }

  .dashboard-health-item,
  .dashboard-health-item:first-child {
    flex: 1 1 0;
    flex-basis: auto;
    justify-content: flex-start;
  }

  .kpi--quality,
  .kpi--queues {
    grid-template-columns: repeat(4, minmax(0, 1fr)) !important;
  }

  .dashboard-analysis-grid {
    grid-template-columns: minmax(0, 1.7fr) minmax(22rem, 0.72fr) !important;
  }

  .dashboard-bottom-grid {
    grid-template-columns: minmax(0, 1.55fr) minmax(25rem, 0.7fr) !important;
  }

  .board-panel--trend,
  .board-panel--status {
    min-height: 24rem;
  }

  .board-panel--tasks,
  .board-panel--activity {
    min-height: 20rem;
  }
}

@media (max-width: 760px) {
  .board-header::before,
  .board-header::after {
    display: none;
  }

  .board-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .board-header h1 {
    max-width: 100%;
  }

  .board-header-aside {
    width: 100%;
    min-width: 0;
    grid-template-columns: auto auto auto;
    justify-content: start;
    gap: 0.55rem;
  }

  .dashboard-health-strip,
  .dashboard-health-metrics {
    align-items: stretch;
    flex-direction: column;
  }

  .dashboard-health-item:first-child {
    flex-basis: auto;
  }

  .trend-summary-grid {
    grid-template-columns: 1fr;
  }

  .chart-panel-head {
    flex-direction: column;
  }
}

@media (max-width: 1180px) {
  .board-header h1 {
    max-width: 100%;
  }
}
</style>
