/**
 * 页面职责：任务运营主页总览（看板布局）
 * 数据：useTasksStore 列表 + useAuditsStore 审计记录聚合；图表由当前列表统计。
 */
<template>
  <div
    class="min-h-[100dvh] w-full min-w-0 max-w-full overflow-x-hidden bg-stone-100 text-slate-900"
  >
    <div v-if="error" class="dashboard-error mx-auto max-w-lg p-8 text-center">
      <p>{{ error }}</p>
      <BaseButton variant="primary" size="sm" @click="load">重试</BaseButton>
    </div>
    <template v-else-if="hasBusinessAccess">
      <div
        class="board mx-auto w-full min-w-0 max-w-[min(100%,90rem)] space-y-4 px-3 pb-8 pt-4 sm:space-y-5 sm:px-4 sm:pt-5 md:space-y-6 md:px-6 md:pt-6 lg:px-8"
      >
        <!-- 顶栏 -->
        <header
          class="board-header rounded-xl bg-white p-4 shadow-sm shadow-slate-900/[0.04] ring-1 ring-slate-200/60 sm:p-5"
        >
          <h1 class="m-0 text-[1.25rem] font-semibold leading-tight text-[#0F172A] sm:text-[1.5rem] md:text-[1.625rem]">
            任务运营主页总览
          </h1>
        </header>

        <!-- 快捷健康条（原「任务健康度」能力保留为单行） -->
        <div
          v-if="!loading"
          class="flex flex-col gap-3 text-xs text-slate-500 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-3 sm:gap-y-1 sm:text-sm"
        >
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span>待仓库接收 <strong class="text-slate-800">{{ summary.pendingWarehouseReceiveCount }}</strong></span>
            <span class="hidden text-slate-300 sm:inline" aria-hidden="true">|</span>
            <span>定制处理中 <strong class="text-slate-800">{{ summary.pendingOutsourceReturnCount }}</strong></span>
            <span class="hidden text-slate-300 sm:inline" aria-hidden="true">|</span>
            <span>即将逾期 <strong class="font-semibold text-red-600">{{ summary.overdueCount }}</strong></span>
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
          class="kpi grid grid-cols-1 gap-3 min-[400px]:grid-cols-2 lg:grid-cols-4 lg:gap-4"
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
          class="kpi grid grid-cols-1 gap-3 min-[400px]:grid-cols-2 lg:grid-cols-4 lg:gap-4"
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
          class="grid grid-cols-1 items-stretch gap-4 min-[1100px]:grid-cols-2 min-[1100px]:gap-5"
        >
          <article
            class="board-panel flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
          >
            <h2 class="m-0 text-[0.9rem] font-semibold text-[#0F172A] sm:text-[0.95rem]">
              7日任务创建/完成趋势
            </h2>
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
            class="board-panel flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
          >
            <h2 class="m-0 text-[0.9rem] font-semibold text-[#0F172A] sm:text-[0.95rem]">
              任务状态分布
            </h2>
            <DashboardStatusPie :series="statusPieSeries" :loading="loading" />
            <p class="m-0 text-[0.7rem] text-slate-400">各处理阶段任务数量</p>
          </article>
        </section>

        <!-- 表格 + 动态 -->
        <section
          class="grid grid-cols-1 items-stretch gap-4 min-[1100px]:grid-cols-2 min-[1100px]:gap-5"
        >
          <article
            class="board-panel flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:p-5"
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
            class="board-panel flex min-h-0 min-w-0 flex-col gap-2 rounded-xl bg-white p-4 ring-1 ring-slate-200/60 sm:gap-3 sm:p-5"
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
        <section class="min-w-0">
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
const DashboardStatusPie = defineAsyncComponent(() => import('@/components/dashboard/DashboardStatusPie.vue'))

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

/* 无业务权限 */
.dashboard-empty {
  flex: 1 1 auto;
  width: 100%;
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(1.5rem, 3vw, 3rem);
  border-radius: 1.5rem;
  background:
    radial-gradient(circle at 15% 10%, rgba(219, 234, 254, 0.6), transparent 55%),
    radial-gradient(circle at 85% 20%, rgba(237, 233, 254, 0.55), transparent 55%),
    radial-gradient(circle at 50% 100%, rgba(252, 231, 243, 0.45), transparent 60%),
    rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow: 0 18px 56px -12px rgba(28, 25, 23, 0.08);
  text-align: center;
  overflow: hidden;
}
.empty-inner {
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
  background: linear-gradient(145deg, rgb(219 234 254) 0%, rgb(237 233 254) 100%);
  border: 1px solid rgba(255, 255, 255, 0.85);
  color: rgb(37 99 235);
  box-shadow: 0 10px 30px -10px rgba(37, 99, 235, 0.35);
}
.empty-icon {
  width: 52%;
  height: 52%;
}
.empty-title {
  margin: 0;
  font-size: clamp(1.5rem, 2.4vw, 2rem);
  font-weight: 800;
  font-family: Manrope, sans-serif;
  letter-spacing: -0.01em;
  color: rgb(15 23 42);
}
.empty-desc {
  margin: 0;
  max-width: 560px;
  font-size: clamp(0.9375rem, 1.2vw, 1rem);
  font-weight: 500;
  line-height: 1.7;
  color: rgb(71 85 105);
}
.empty-muted {
  margin: 0;
  max-width: 560px;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.7;
  color: rgb(148 163 184);
}
.empty-hint {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.9rem;
  border-radius: 9999px;
  background: rgba(241, 245, 249, 0.9);
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(100 116 139);
}
.empty-hint-dot {
  width: 6px;
  height: 6px;
  border-radius: 9999px;
  background: rgb(100 116 139);
}
.empty-steps {
  margin-top: 1rem;
  width: 100%;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}
.empty-step {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.1rem 1.1rem 1.25rem;
  border-radius: 0.9rem;
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid rgba(226, 232, 240, 0.7);
  text-align: left;
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
  background: rgb(219 234 254);
  color: rgb(29 78 216);
}
.empty-step[data-accent='violet'] .empty-step-num {
  background: rgb(237 233 254);
  color: rgb(109 40 217);
}
.empty-step[data-accent='emerald'] .empty-step-num {
  background: rgb(220 252 231);
  color: rgb(21 128 61);
}
.empty-step-title {
  margin: 0;
  font-family: Manrope, sans-serif;
  font-size: 0.9375rem;
  font-weight: 700;
  color: rgb(15 23 42);
}
.empty-step-desc {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.6;
  color: rgb(100 116 139);
}
@media (max-width: 720px) {
  .empty-steps {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
