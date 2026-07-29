/**
 * 页面职责：任务运营主页总览（看板布局）
 * 数据：useTasksStore 列表 + useAuditsStore 审计记录聚合；图表由当前列表统计。
 */
<template>
  <div
    class="dashboard-shell min-h-[100dvh] w-full min-w-0 max-w-full overflow-x-hidden bg-[rgb(var(--yb-bg-page))] text-[rgb(var(--yb-text))]"
  >
    <div v-if="error" class="dashboard-error mx-auto max-w-lg p-8 text-center">
      <p>{{ error }}</p>
      <BaseButton variant="primary" size="sm" @click="load()">重试</BaseButton>
    </div>
    <template v-else-if="hasBusinessAccess">
      <div
        class="board dashboard-workbench w-full min-w-0 space-y-4 pb-8 sm:space-y-5 md:space-y-6"
      >
        <!-- 顶栏 -->
        <header
          class="board-header yb-page-surface yb-page-header-row"
          data-page-header="dashboard"
        >
          <div class="board-header-content yb-page-heading-copy">
            <h1 class="board-header-title yb-page-title">
              任务运营主页总览
            </h1>
            <p class="board-header-subtitle yb-page-subtitle">
              先看今日是否正常，再判断队列积压、质量风险和下一步处理入口。
            </p>
          </div>
          <div class="board-header-aside" aria-live="polite">
            <span>30 秒自动刷新</span>
            <strong>{{ summary.todayPendingCount }}</strong>
            <small>设计待办</small>
            <small class="board-header-updated">更新于 {{ lastUpdatedLabel }}</small>
            <BaseButton
              type="button"
              variant="ghost"
              size="sm"
              :disabled="refreshing"
              @click="load(false)"
            >
              {{ refreshing ? '刷新中…' : '立即刷新' }}
            </BaseButton>
          </div>
        </header>

        <p v-if="refreshWarning" class="dashboard-refresh-warning" role="status">
          {{ refreshWarning }}；当前保留上次成功数据。
        </p>

        <!-- 快捷健康条（原「任务健康度」能力保留为单行） -->
        <div
          v-if="!loading"
          class="dashboard-health-strip flex flex-col gap-3 text-xs text-[rgb(var(--yb-text-muted))] sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-3 sm:gap-y-1 sm:text-sm"
        >
          <div class="dashboard-health-metrics flex flex-wrap items-center gap-2">
            <span class="dashboard-health-item dashboard-health-item--success">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">数据服务</span>
              <strong class="dashboard-health-value">{{ dataHealthLabel }}</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--load">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">今日结单</span>
              <strong class="dashboard-health-value">{{ summary.todayCompletedCount }}</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--quality">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">打回率</span>
              <strong class="dashboard-health-value">{{ kpiStats.rejectRateLabel }}</strong>
            </span>
            <span class="dashboard-health-item dashboard-health-item--danger">
              <span class="dashboard-health-dot" aria-hidden="true" />
              <span class="dashboard-health-label">已逾期</span>
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

        <!-- 任务经营指标与任务看板共用 task.view 权限。 -->
        <section
          v-if="can('task.view')"
          class="kpi kpi--quality grid grid-cols-1 gap-3 min-[400px]:grid-cols-2 lg:grid-cols-4 lg:gap-4"
        >
          <DashboardKpiCard
            title="本周完成率"
            :value="kpiStats.completedRateLabel"
            hint="本周新建且已结单 / 本周新建"
          />
          <DashboardKpiCard
            title="打回率"
            :value="kpiStats.rejectRateLabel"
            hint="本周审核打回 / 审核结论"
          />
          <DashboardKpiCard
            title="平均处理时长"
            :value="kpiStats.avgHoursLabel"
            :hint="kpiStats.avgHoursHint"
          />
          <DashboardKpiCard
            title="全局进行中任务"
            :value="kpiStats.pendingCount"
            hint="全局未完成、未取消任务"
            route="/tasks?operational_bucket=active_tasks"
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
            title="设计待办"
            :value="summary.todayPendingCount"
            hint="待指派、设计中、审核打回等任务"
            route="/tasks?operational_bucket=design_pending"
          />
          <DashboardKpiCard
            title="待审核"
            :value="summary.pendingAuditCount"
            hint="待审核任务"
            route="/tasks?operational_bucket=pending_audit"
          />
          <DashboardKpiCard
            title="需交班"
            :value="summary.handoverCount"
            hint="审核交班任务"
            route="/tasks?operational_bucket=handover"
          />
          <DashboardKpiCard
            title="今日新建"
            :value="summary.todayCreatedCount"
            route="/tasks?operational_bucket=today_created"
          />
        </section>

        <!-- 图表行：随屏宽单栏 → 双栏 -->
        <section
          class="dashboard-analysis-grid grid grid-cols-1 items-stretch gap-4 min-[1100px]:grid-cols-2 min-[1100px]:gap-5"
        >
          <article
            class="board-panel board-panel--trend flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-[rgb(var(--yb-surface))] p-4 ring-1 ring-[rgb(var(--yb-border)/0.6)] sm:gap-3 sm:p-5"
          >
            <div class="chart-panel-head">
              <div>
                <h2 class="dashboard-panel-heading m-0 text-[0.9rem] font-semibold sm:text-[0.95rem]">
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
            <p class="m-0 text-[0.7rem] text-[rgb(var(--yb-text-faint))]">近7日新建、完成与截止日分布</p>
          </article>
          <article
            class="board-panel board-panel--status flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-[rgb(var(--yb-surface))] p-4 ring-1 ring-[rgb(var(--yb-border)/0.6)] sm:gap-3 sm:p-5"
          >
            <div class="chart-panel-head">
              <div>
                <h2 class="dashboard-panel-heading m-0 text-[0.9rem] font-semibold sm:text-[0.95rem]">
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
            class="board-panel board-panel--tasks flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-[rgb(var(--yb-surface))] p-4 ring-1 ring-[rgb(var(--yb-border)/0.6)] sm:p-5"
          >
            <h2 class="dashboard-panel-heading m-0 mb-3 text-[0.9rem] font-semibold sm:mb-4 sm:text-[0.95rem]">近期任务明细</h2>
            <div v-if="loading" class="py-4">
              <StatusSkeleton :loading="true" :lines="4" class="!py-0" />
            </div>
            <div v-else class="min-w-0 overflow-x-auto -mx-1 px-1 sm:mx-0 sm:px-0">
              <DashboardTaskSnapshotTable :snapshots="overview?.recent_tasks ?? []" />
            </div>
          </article>
          <article
            class="board-panel board-panel--activity flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-[rgb(var(--yb-surface))] p-4 ring-1 ring-[rgb(var(--yb-border)/0.6)] sm:gap-3 sm:p-5"
          >
            <h2 class="dashboard-panel-heading m-0 text-[0.9rem] font-semibold sm:text-[0.95rem]">实时动态</h2>
            <p class="m-0 text-xs text-[rgb(var(--yb-text-faint))] sm:text-[0.7rem]">审核、交班与新建</p>
            <div
              class="min-h-[12rem] flex-1 overflow-auto rounded-lg bg-[rgb(var(--yb-surface-soft))] p-3 min-[1100px]:min-h-[16rem] sm:p-3 ring-1 ring-[rgb(var(--yb-border)/0.8)]"
            >
              <RecentEventStream
                :events="recentEvents"
                :loading="loading"
                :error="overview == null ? error : ''"
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
import { ref, onMounted, onBeforeUnmount, computed, defineAsyncComponent } from 'vue'
import { useRouter } from 'vue-router'
import type {
  DashboardSummary,
  RecentEvent,
  RiskItem,
  TaskOperationalOverview,
} from '@/types/dashboard'
import { tasksApi } from '@/services/api/tasksApi'
import StatusSkeleton from '@/components/common/StatusSkeleton.vue'
import DashboardKpiCard from '@/components/dashboard/DashboardKpiCard.vue'
import DashboardTaskSnapshotTable from '@/components/dashboard/DashboardTaskSnapshotTable.vue'
import RecentEventStream from '@/components/dashboard/RecentEventStream.vue'
import RiskListCard from '@/components/dashboard/RiskListCard.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import { usePermission } from '@/composables/usePermission'
import { formatDateBeijing } from '@/utils/date'
import { beijingDateKeyToShortLabel } from '@/utils/beijing-calendar'

const DashboardTrendChart = defineAsyncComponent(() => import('@/components/dashboard/DashboardTrendChart.vue'))

const router = useRouter()
const { can } = usePermission()
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
const refreshWarning = ref('')
const overview = ref<TaskOperationalOverview | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null
let lastSuccessfulRefreshAt = 0

const BUSINESS_ACTIONS = [
  'task.view',
  'task.create',
  'task.design.submit',
  'task.audit.decision',
] as const

const hasBusinessAccess = computed(() => BUSINESS_ACTIONS.some((a) => can(a)))

const lastUpdatedLabel = computed(() =>
  overview.value?.generated_at ? formatDateBeijing(overview.value.generated_at) : '尚未更新',
)
const dataHealthLabel = computed(() => overview.value?.health_status === 'ok' ? '正常' : '不可用')

const summary = computed<DashboardSummary>(() => {
  const counts = overview.value?.counts
  return {
    todayPendingCount: counts?.design_pending ?? 0,
    pendingAuditCount: counts?.pending_audit ?? 0,
    handoverCount: counts?.handover ?? 0,
    todayCompletedCount: counts?.today_completed ?? 0,
    todayCreatedCount: counts?.today_created ?? 0,
    overdueCount: counts?.overdue ?? 0,
  }
})

const trend7d = computed(() => {
  const points = overview.value?.trend ?? []
  const labels = points.map((point) => beijingDateKeyToShortLabel(point.date))
  const created = points.map((point) => point.created)
  const completed = points.map((point) => point.completed)
  const dueOnDay = points.map((point) => point.due)
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
  const activeBucketMeta = {
		design_ops: { tone: 'pending', hint: '设计、运营或打回后待推进' },
		audit: { tone: 'audit', hint: '审核队列待处理' },
		customization: { tone: 'customization', hint: '定制设计协同处理中' },
		blocked: { tone: 'danger', hint: '存在异常，需要人工处理' },
		completed: { tone: 'completed', hint: '已结单、已归档或已取消' },
  } as const
  const items = (overview.value?.status_distribution ?? []).flatMap((bucket) => {
    const meta = activeBucketMeta[bucket.key as keyof typeof activeBucketMeta]
    return meta ? [{ key: meta.tone, name: bucket.name, value: bucket.count, hint: meta.hint }] : []
  })
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
        ? `当前任务主要集中在「${lead.name}」，共 ${lead.value} 项`
        : '当前暂无任务',
    caption:
      total > 0
        ? withPercent.map((item) => `${item.name} ${item.percent}%`).join(' · ')
        : '暂无任务分布数据',
  }
})

const recentEvents = computed<RecentEvent[]>(() => {
  return (overview.value?.recent_events ?? []).map((event) => ({
    id: event.id,
    type: event.event_type,
    title: event.title,
    refId: String(event.task_id),
    refNo: event.task_no,
    actor: event.actor_name,
    at: formatDateBeijing(event.created_at),
    createdAtIso: event.created_at,
  }))
})

const risks = computed<RiskItem[]>(() => {
  const list: RiskItem[] = []
  const counts = overview.value?.counts
  if (!counts) return list
  if (counts.overdue > 0) {
    list.push({
      id: 'overdue-tasks',
      level: 'high',
      message: `${counts.overdue} 个进行中任务已经逾期`,
      route: '/tasks?operational_bucket=overdue',
    })
  }
  if (counts.due_today > 0) {
    list.push({
      id: 'due-today',
      level: 'medium',
      message: `${counts.due_today} 个任务今天截止`,
      route: '/tasks?operational_bucket=due_today',
    })
  }
  if (counts.customization_in_progress > 0) {
    list.push({
      id: 'customization-in-progress',
      level: 'medium',
      message: `${counts.customization_in_progress} 个定制任务处理中`,
      route: '/tasks?operational_bucket=customization_in_progress',
    })
  }
  return list
})

const kpiStats = computed(() => {
  const kpis = overview.value?.kpis
  const counts = overview.value?.counts
  const sampleCount = kpis?.average_processing_sample_count ?? 0
  const exactCount = kpis?.exact_completion_sample_count ?? 0
  const fallbackCount = kpis?.fallback_completion_sample_count ?? 0
  return {
    completedRateLabel: `${(kpis?.week_completion_rate ?? 0).toFixed(1)}%`,
    rejectRateLabel: `${(kpis?.week_reject_rate ?? 0).toFixed(1)}%`,
    avgHoursLabel: `${(kpis?.average_processing_hours ?? 0).toFixed(1)} 小时`,
    avgHoursHint:
      sampleCount > 0
        ? `本周结单平均耗时；事件 ${exactCount} 条，历史回退 ${fallbackCount} 条`
        : '本周暂无结单样本',
    pendingCount: counts?.active_tasks ?? 0,
  }
})

async function load(initial = overview.value == null) {
  if (refreshing.value) return
  if (initial) loading.value = true
  refreshing.value = true
  refreshWarning.value = ''
  try {
    const overviewResponse = await tasksApi.operationalOverview()
    const nextOverview = overviewResponse.data?.data
    if (!nextOverview) {
      throw new Error('运营主页统计响应缺少 data')
    }
    overview.value = nextOverview
    lastSuccessfulRefreshAt = Date.now()
    error.value = ''
  } catch (e) {
    const message = e instanceof Error ? e.message : '加载任务运营主页总览失败'
    if (overview.value == null) error.value = message
    else refreshWarning.value = message
  } finally {
    if (initial) loading.value = false
    refreshing.value = false
  }
}

function refreshWhenVisible() {
  if (document.visibilityState !== 'visible') return
  if (Date.now() - lastSuccessfulRefreshAt < 5000) return
  void load(false)
}

function startRefreshLoop() {
  refreshTimer = setInterval(() => {
    if (document.visibilityState === 'visible') void load(false)
  }, 30_000)
  document.addEventListener('visibilitychange', refreshWhenVisible)
  window.addEventListener('focus', refreshWhenVisible)
}

function stopRefreshLoop() {
  if (refreshTimer != null) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  document.removeEventListener('visibilitychange', refreshWhenVisible)
  window.removeEventListener('focus', refreshWhenVisible)
}

function goTaskPool() {
  void router.push('/tasks?tab=pool')
}

onMounted(() => {
  if (!hasBusinessAccess.value) {
    loading.value = false
    return
  }
  void load(true)
  startRefreshLoop()
})
onBeforeUnmount(stopRefreshLoop)
</script>

<style scoped>
.dashboard-error {
  background: rgb(var(--yb-danger-soft));
  border: 1px solid rgb(var(--yb-danger-border));
  border-radius: 1rem;
  color: rgb(var(--yb-danger-deep));
}
.dashboard-error p {
  margin: 0 0 1rem;
}
.dashboard-refresh-warning {
  margin: 0;
  border: 1px solid rgb(var(--yb-warning-border-soft));
  border-radius: 0.75rem;
  padding: 0.65rem 0.8rem;
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
  font-size: 0.8rem;
}
.kpi-skeleton {
  min-height: 5.5rem;
}
.dashboard-panel-heading {
  color: rgb(var(--yb-text-navy));
}
.board-panel {
  box-shadow:
    0 1px 2px rgb(var(--yb-shadow) / 0.05),
    0 0 0 1px rgb(var(--yb-shadow) / 0.04);
}

/* 无业务权限 — 浅色欢迎区 */
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
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
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
  background: rgb(var(--yb-brand-soft));
  border: 1px solid rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand));
  box-shadow: none;
}
.empty-icon {
  width: 52%;
  height: 52%;
  filter: none;
}
.empty-title {
  margin: 0;
  font-size: clamp(1.5rem, 2.4vw, 2rem);
  font-weight: 800;
  font-family: var(--yb-font-display);
  letter-spacing: -0.01em;
  color: rgb(var(--yb-text));
  text-shadow: none;
}
.empty-desc {
  margin: 0;
  max-width: 560px;
  font-size: clamp(0.9375rem, 1.2vw, 1rem);
  font-weight: 500;
  line-height: 1.7;
  color: rgb(var(--yb-text-muted));
}
.empty-muted {
  margin: 0;
  max-width: 560px;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.7;
  color: rgb(var(--yb-text-faint));
}
.empty-hint {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.42rem 0.95rem;
  border-radius: 9999px;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  font-size: 0.75rem;
  font-weight: 650;
  color: rgb(var(--yb-text-muted));
  box-shadow: none;
}
.empty-hint-dot {
  width: 6px;
  height: 6px;
  border-radius: 9999px;
  background: rgb(var(--yb-brand));
  box-shadow: none;
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
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  text-align: left;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
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
  opacity: 1;
}
.empty-step[data-accent='blue']::before {
  background: rgb(var(--yb-brand));
}
.empty-step[data-accent='violet']::before {
  background: rgb(var(--yb-purple));
}
.empty-step[data-accent='emerald']::before {
  background: rgb(var(--yb-success));
}
.empty-step:hover {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  box-shadow: 0 4px 12px rgb(var(--yb-shadow) / 0.06);
}
.empty-step-num {
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--yb-font-display);
  font-size: 0.8125rem;
  font-weight: 800;
}
.empty-step[data-accent='blue'] .empty-step-num {
  border: 1px solid rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  box-shadow: none;
}
.empty-step[data-accent='violet'] .empty-step-num {
  border: 1px solid rgb(var(--yb-purple-border));
  background: rgb(var(--yb-purple-soft));
  color: rgb(var(--yb-purple-text));
  box-shadow: none;
}
.empty-step[data-accent='emerald'] .empty-step-num {
  border: 1px solid rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-strong));
  box-shadow: none;
}
.empty-step-title {
  margin: 0;
  font-family: var(--yb-font-display);
  font-size: 0.9375rem;
  font-weight: 700;
  color: rgb(var(--yb-text));
}
.empty-step-desc {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.6;
  color: rgb(var(--yb-text-muted));
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

/* Light admin dashboard skin. Style-only. */
.dashboard-shell {
  background: transparent;
}

.board {
  width: min(100%, 1936px);
  max-width: min(100%, 1936px);
  color: rgb(var(--yb-text-body));
  position: relative;
  isolation: isolate;
}

.dashboard-workbench {
  padding-inline: 0;
}

.board::before,
.board::after {
  display: none;
}

.board-header {
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.25rem;
  min-height: clamp(5.25rem, 9vh, 6.5rem);
  padding: clamp(0.85rem, 1.4vw, 1.25rem) clamp(1.1rem, 2vw, 1.75rem);
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1.125rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
}

.board-header::before,
.board-header::after {
  display: none;
}

.board-header-content,
.board-header-aside {
  position: relative;
  z-index: 1;
}

.board-header-content {
  min-width: 0;
}

.board-header-title,
.board-header h1 {
  position: relative;
  z-index: 1;
  max-width: min(58rem, 100%);
  margin: 0;
  color: rgb(var(--yb-text));
  font-size: clamp(1.5rem, 2.1vw, 2rem);
  font-weight: 700;
  line-height: 1.2;
  text-shadow: none;
}

.board-header-subtitle {
  max-width: 46rem;
  margin: 0.4rem 0 0;
  color: rgb(var(--yb-text-faint));
  font-size: 0.8125rem;
  font-weight: 400;
  line-height: 1.45;
}

.board-header-aside {
  display: grid;
  min-width: 8.75rem;
  place-items: center;
  align-self: stretch;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.95rem;
  padding: 0.8rem 1rem;
  background: rgb(var(--yb-surface-soft));
  box-shadow: none;
}

.board-header-aside span,
.board-header-aside small {
  color: rgb(var(--yb-text-muted));
  font-size: 0.72rem;
  font-weight: 800;
}

.board-header-aside strong {
  color: rgb(var(--yb-text));
  font-family: var(--yb-font-data);
  font-size: clamp(2rem, 3vw, 3.1rem);
  line-height: 1;
}

.board-header-aside .board-header-updated {
  font-weight: 500;
  white-space: nowrap;
}

.board-header-aside :deep(button) {
  min-height: 1.75rem;
  padding-inline: 0.6rem;
  font-size: 0.7rem;
}

.dashboard-health-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 4rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.875rem;
  padding: 0.7rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
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
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.55rem;
  padding: 0.5rem 0.65rem;
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text-muted));
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
  background: rgb(var(--yb-brand));
  box-shadow: none;
}

.dashboard-health-item--success {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-soft));
}

.dashboard-health-item--success .dashboard-health-dot {
  background: rgb(var(--yb-success));
  box-shadow: none;
}

.dashboard-health-item--quality .dashboard-health-dot {
  background: rgb(var(--yb-success-bright));
  box-shadow: none;
}

.dashboard-health-item--load .dashboard-health-dot {
  background: rgb(var(--yb-brand));
}

.dashboard-health-item--danger {
  border-color: rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
}

.dashboard-health-item--danger .dashboard-health-dot {
  background: rgb(var(--yb-warning));
  box-shadow: none;
}

.dashboard-health-label {
  display: inline-block;
}

.dashboard-health-value,
.dashboard-health-strip strong {
  color: rgb(var(--yb-text));
  font-family: var(--yb-font-data);
  font-weight: 850;
}

.dashboard-health-item--danger .dashboard-health-value {
  color: rgb(var(--yb-warning-text));
}

.dashboard-health-strip :deep(button),
.dashboard-health-strip button {
  min-height: 2.65rem;
  flex: 0 0 auto;
  border-color: rgb(var(--yb-brand));
  border-radius: 0.55rem;
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
  box-shadow: 0 1px 2px rgb(var(--yb-brand) / 0.2);
}

.kpi {
  align-items: stretch;
}

.kpi--quality,
.kpi--queues {
  gap: 0.75rem;
}

.dashboard-analysis-grid {
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
}

.dashboard-bottom-grid {
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
}

.dashboard-risk-zone {
  border-radius: 0.875rem;
}

.board-panel,
.risk-card {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
}

/* KPI 八卡：浅色实底卡片 */
.kpi-card {
  position: relative;
  overflow: hidden;
  min-height: 7.85rem;
  border-radius: 0.875rem;
  padding: 1rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease,
    transform 0.18s ease;
}

.kpi-card::before {
  display: none;
}

.kpi-card:hover {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  box-shadow: 0 4px 12px rgb(var(--yb-shadow) / 0.08);
  transform: translateY(-1px);
}

@media (prefers-reduced-motion: reduce) {
  .kpi-card:hover {
    transform: none;
  }
}

.board-panel:hover,
.risk-card:hover {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
}

.board-panel,
.risk-card {
  border-radius: 0.875rem;
  padding: 1rem;
}

.board-panel--tasks,
.board-panel--activity {
  min-height: 19rem;
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
}

.board-panel--activity > div:last-child {
  border-radius: 0.75rem;
}

:deep(.kpi-card__label),
:deep(.kpi-card__hint),
.board-panel p,
:deep(.risk-empty),
:deep(.risk-loading) {
  color: rgb(var(--yb-text-muted));
}

:deep(.kpi-card__value) {
  color: rgb(var(--yb-text));
  font-family: var(--yb-font-data);
  font-size: clamp(1.75rem, 2vw, 2.4rem);
  text-shadow: none;
}

.board-panel h2,
:deep(.risk-title),
:deep(.task-table__head),
:deep(.task-table__row),
:deep(.stream-item) {
  color: rgb(var(--yb-text-body));
}

.board-panel :deep(.trend__chart),
.board-panel :deep(.pie__chart),
.board-panel :deep(.trend__skeleton),
.board-panel :deep(.pie__skeleton) {
  background: rgb(var(--yb-surface-soft));
  border-color: rgb(var(--yb-border));
}

.board-panel--trend,
.board-panel--status {
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border));
  min-height: 24rem;
}

.chart-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  border: 0;
  background: transparent;
}

.chart-panel-sub {
  margin: 0.3rem 0 0;
  color: rgb(var(--yb-text-muted));
  font-size: 0.74rem;
  line-height: 1.4;
}

.chart-panel-badge {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  min-height: 1.75rem;
  padding: 0.3rem 0.62rem;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 0.55rem;
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  font-size: 0.72rem;
  font-weight: 800;
}

.trend-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
  border: 0;
  background: transparent;
}

.trend-summary-card {
  min-width: 0;
  padding: 0.7rem 0.75rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-soft));
}

.trend-summary-label {
  display: block;
  color: rgb(var(--yb-text-muted));
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
  color: rgb(var(--yb-brand));
}

.trend-summary-card--completed .trend-summary-value {
  color: rgb(var(--yb-success-strong));
}

.trend-summary-card--due .trend-summary-value {
  color: rgb(var(--yb-danger));
}

.board-panel--trend :deep(.trend__chart),
.board-panel--trend :deep(.trend__skeleton) {
  min-height: 13rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.875rem;
  background: rgb(var(--yb-surface-soft));
}

.status-distribution-skeleton,
.status-distribution {
  display: flex;
  min-height: 13rem;
  flex-direction: column;
  gap: 0.75rem;
  border: 0;
  background: transparent;
}

.status-stack-bar {
  display: flex;
  width: 100%;
  min-height: 1.125rem;
  overflow: hidden;
  gap: 0.15rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 999px;
  background: rgb(var(--yb-surface-muted));
  box-shadow: none;
}

.status-stack-seg {
  min-width: 0;
  transition: opacity 0.16s ease;
}

.status-stack-seg--pending {
  background: rgb(var(--yb-status-pending));
}

.status-stack-seg--audit {
  background: rgb(var(--yb-success-border-bright));
}

.status-stack-seg--customization {
  background: rgb(var(--yb-warning));
}

.status-stack-seg--completed {
  background: rgb(var(--yb-success));
}

.status-stack-caption {
  margin: 0;
  color: rgb(var(--yb-text-muted));
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
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-soft));
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease;
}

.status-card:hover {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
}

.status-card-dot {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
}

.status-card--pending .status-card-dot {
  background: rgb(var(--yb-status-pending));
}

.status-card--audit .status-card-dot {
  background: rgb(var(--yb-success-border-bright));
}

.status-card--customization .status-card-dot {
  background: rgb(var(--yb-warning));
}

.status-card--completed .status-card-dot {
  background: rgb(var(--yb-success));
}

.status-card-main {
  min-width: 0;
}

.status-card-main strong,
.status-card-main small {
  display: block;
}

.status-card-main strong {
  color: rgb(var(--yb-text));
  font-size: 0.82rem;
  font-weight: 850;
}

.status-card-main small {
  margin-top: 0.12rem;
  overflow: hidden;
  color: rgb(var(--yb-text-muted));
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
  color: rgb(var(--yb-brand));
}

.status-card--audit .status-card-value {
  color: rgb(var(--yb-success-strong));
}

.status-card--customization .status-card-value {
  color: rgb(var(--yb-warning-text));
}

.status-card--completed .status-card-value {
  color: rgb(var(--yb-success-strong));
}

.board-panel :deep(.task-table__head) {
  color: rgb(var(--yb-text-muted));
  background: rgb(var(--yb-surface-soft));
  border-radius: 0.75rem 0.75rem 0 0;
  border-color: rgb(var(--yb-border));
}

.board-panel :deep(.task-table__row) {
  min-height: 3rem;
  color: rgb(var(--yb-text-body));
  border-color: rgb(var(--yb-border));
}

.board-panel :deep(.col-status) {
  color: rgb(var(--yb-text));
  font-weight: 800;
}

.board-panel--tasks :deep(.task-table) {
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  box-shadow: none;
}

.board-panel :deep(.task-table__row--alt),
.board-panel :deep(.task-table__row:not(.task-table__row--alt)) {
  background: rgb(var(--yb-surface));
}

.board-panel :deep(.task-table__row--alt) {
  background: rgb(var(--yb-surface-soft));
}

.board-panel :deep(.task-table__status--info) {
  color: rgb(var(--yb-brand));
}

.board-panel :deep(.task-table__status--warning) {
  color: rgb(var(--yb-warning-text));
}

.board-panel :deep(.task-table__status--success) {
  color: rgb(var(--yb-success-strong));
}

.board-panel :deep(.task-table__status--neutral) {
  color: rgb(var(--yb-text-muted));
}

.board-panel :deep(.task-table__row:hover),
.board-panel :deep(.stream-item.navigable:hover),
:deep(.risk-item.navigable:hover) {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text));
}

.board-panel :deep(.col-task),
.board-panel :deep(.col-owner),
.board-panel :deep(.col-due),
.board-panel :deep(.event-title),
.board-panel :deep(.event-actor),
:deep(.risk-message) {
  color: rgb(var(--yb-text-body));
}

.board-panel :deep(.event-time),
.board-panel :deep(.event-time--short) {
  color: rgb(var(--yb-text-faint));
}

.board-panel :deep(.stream-item) {
  border-color: rgb(var(--yb-border));
}

.board-panel--activity > div:last-child {
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
}

.board-panel :deep(.stream-list) {
  background: transparent;
}

.board-panel :deep(.stream-list--dashboard .stream-item) {
  margin-bottom: 0.25rem;
  padding: 0.58rem 0.55rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.65rem;
  background: rgb(var(--yb-surface));
  box-shadow: none;
}

.board-panel :deep(.stream-list--dashboard .stream-item:nth-child(even)) {
  background: rgb(var(--yb-surface-soft));
}

:deep(.risk-item) {
  border-radius: 0.65rem;
  padding: 0.58rem 0.55rem;
}

:deep(.event-ref--link),
:deep(.risk-ref--link) {
  color: rgb(var(--yb-brand));
}

@media (min-width: 1100px) {
  .dashboard-analysis-grid {
    grid-template-columns: minmax(0, 1.35fr) minmax(18rem, 0.75fr);
  }

  .dashboard-bottom-grid {
    grid-template-columns: minmax(0, 1.35fr) minmax(21rem, 0.72fr);
  }
}

@media (min-width: 1440px) {
  .dashboard-workbench {
    max-width: min(100%, 1760px);
  }

  .board-header {
    min-height: 6.5rem;
  }

  .dashboard-health-strip {
    align-items: center;
    flex-direction: row;
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
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .dashboard-analysis-grid {
    grid-template-columns: minmax(0, 1.7fr) minmax(22rem, 0.72fr);
  }

  .dashboard-bottom-grid {
    grid-template-columns: minmax(0, 1.55fr) minmax(25rem, 0.7fr);
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
